package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	outboxdomain "vrcpb/backend/internal/outbox/domain"
	"vrcpb/backend/internal/outbox/domain/vo/aggregate_type"
	"vrcpb/backend/internal/outbox/domain/vo/event_type"
	outboxrdb "vrcpb/backend/internal/outbox/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	usagelimitaction "vrcpb/backend/internal/usagelimit/domain/vo/action"
	usagelimitscopehash "vrcpb/backend/internal/usagelimit/domain/vo/scope_hash"
	usagelimitscopetype "vrcpb/backend/internal/usagelimit/domain/vo/scope_type"
	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

// ogpSyncTimeout は publish commit 後の best-effort 同期生成に許容する最大時間。
//
// ADR-0007 §3 (2) / docs/plan/m2-ogp-sync-publish-plan.md STOP β: 95th 推定
// (warm ~430 ms) に対し十分なマージンを取りつつ、HTTP response を過度に遅らせない。
// 失敗 / timeout 時は pending 行を維持し worker fallback に委ねる。
const ogpSyncTimeout = 2500 * time.Millisecond

// ErrPublishConflict は publish 楽観ロック失敗（version 不一致 / status≠draft）。
var ErrPublishConflict = errors.New("publish conflict (version mismatch or not in draft state)")

// PR36: UsageLimit 連動。
var (
	// ErrPublishRateLimited は 1 時間 5 冊の publish 上限超過（HTTP 429）。
	ErrPublishRateLimited = errors.New("publish: rate limited")
	// ErrPublishRateLimiterUnavailable は UsageLimit Repository 失敗（fail-closed で 429）。
	ErrPublishRateLimiterUnavailable = errors.New("publish: rate limiter unavailable")
)

// PublishRateLimited は HTTP layer で 429 + Retry-After にマップする wrapper。
type PublishRateLimited struct {
	RetryAfterSeconds int
	Cause             error
}

// Error は error interface。
func (e *PublishRateLimited) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "publish: rate limited"
}

// Unwrap は errors.Is で Cause を判定できるようにする。
func (e *PublishRateLimited) Unwrap() error { return e.Cause }

// PublishFromDraftInput は publish の入力。
//
// PR36: RemoteIP / IPHashSalt は UsageLimit 連動用（業務知識 v4 §3.7 「同一作成元
// 1 時間 5 冊」）。空文字なら UsageLimit を skip。
//
// 2026-05-03 STOP α P0 v2: RightsAgreed は publish 時の権利・配慮確認同意フラグ
// （業務知識 v4 §3.1）。false / 不在の場合は early reject で domain.ErrRightsNotAgreed
// を返す。true の場合は同 TX 内で domain.WithRightsAgreed(Now) → CanPublish →
// repository.PublishFromDraft の順で実行し、同意も published_at と同時刻で
// DB に永続化する。
type PublishFromDraftInput struct {
	PhotobookID     photobook_id.PhotobookID
	ExpectedVersion int
	RightsAgreed    bool
	Now             time.Time
	RemoteIP        string
	IPHashSalt      string
}

// PublishFromDraftOutput は publish 結果。RawManageToken はログ禁止。
type PublishFromDraftOutput struct {
	Photobook       domain.Photobook
	RawManageToken  manage_url_token.ManageUrlToken
}

// MapPublishUsageErr は usagelimit エラーを publish 集約エラーに変換する。
// fail-closed: 両方の usage error を 429 wrapper にマップ。retry_after は最低 1 秒、
// Repository 失敗時は 60 秒の安全側既定。
func MapPublishUsageErr(err error, retryAfter int) error {
	switch {
	case errors.Is(err, usagelimitwireup.ErrRateLimited):
		if retryAfter < 1 {
			retryAfter = 1
		}
		return &PublishRateLimited{RetryAfterSeconds: retryAfter, Cause: ErrPublishRateLimited}
	case errors.Is(err, usagelimitwireup.ErrUsageRepositoryFailed):
		return &PublishRateLimited{RetryAfterSeconds: 60, Cause: ErrPublishRateLimiterUnavailable}
	default:
		return err
	}
}

// ComputeIPHashHexForTest は test 用に exported な hex 計算関数。
// PR36 commit 3.6 で追加した実 DB 副作用テストが usage_counters の pre-bucket を作る際に
// salt+ip から本番と同じ scope_hash を生成するために使う。
func ComputeIPHashHexForTest(salt, remoteIP string) string {
	return computeIPHashHex(salt, remoteIP)
}

// computeIPHashHex は salt + sha256(remoteIP) の hex を返す。生 IP は保存せず、
// 戻り値の hex も logs / chat に出さない（呼び出し側で redact）。
//
// 既存 `internal/report/internal/usecase.HashSourceIP` と同じアルゴリズム
// （salt + ":" + ip → sha256）を本 package 内で重複実装。本 PR36 では既存報告経路の
// HashSourceIP を再利用したいが、ImportCycle を避けるため photobook package に閉じた
// 局所実装にする（PR40 でユーティリティを共通化する余地）。
func computeIPHashHex(salt, remoteIP string) string {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte{':'})
	h.Write([]byte(remoteIP))
	return hex.EncodeToString(h.Sum(nil))
}

// PublishFromDraft は draft → published を実行する UseCase。
//
// **同一 TX 内で**:
//  1. photobook を取得・状態検証
//  2. public_url_slug 生成
//  3. ManageUrlToken / ManageUrlTokenHash 生成
//  4. photobookRepo.PublishFromDraft（status='draft' AND version=$expected で UPDATE）
//  5. session の RevokeAllDrafts（同 tx）
//  6. outbox_events に photobook.published event を INSERT（同 tx）
//
// 5 / 6 のいずれかが失敗した場合は photobook 側 UPDATE もロールバックされる
// （I-D7 / I-S9 整合 + Outbox 同 TX 不変条件）。
type PublishFromDraft struct {
	pool                 *pgxpool.Pool
	photobookRepoFactory PhotobookTxRepositoryFactory
	revokerFactory       DraftSessionRevokerFactory
	slugGen              SlugGenerator
	usage                *usagelimitwireup.Check // PR36: nil なら UsageLimit skip
	ogpEnsurer           OgpPendingEnsurerWithTx // M-2: nil なら OGP pending INSERT を skip（旧互換）
	ogpSync              OgpSyncGenerator        // M-2: nil なら commit 後 sync を skip（旧互換）
	logger               *slog.Logger            // nil なら slog.Default()
}

// NewPublishFromDraft は UseCase を組み立てる。
//
// PR36: usage が nil の場合 UsageLimit 連動を行わない（旧互換維持）。
// 本番では非 nil で渡す。
//
// M-2 (ADR-0007): ogpEnsurer / ogpSync が nil の場合は OGP 同期化を skip し、
// 旧 worker 経路（outbox + Scheduler）のみで OGP 行を作成する。test では nil を
// 渡して旧経路の挙動を確認できる。本番では wireup で非 nil を渡す。
// logger は slog 観測 (`event=ogp_sync_result`) 用。
func NewPublishFromDraft(
	pool *pgxpool.Pool,
	photobookRepoFactory PhotobookTxRepositoryFactory,
	revokerFactory DraftSessionRevokerFactory,
	slugGen SlugGenerator,
	usage *usagelimitwireup.Check,
	ogpEnsurer OgpPendingEnsurerWithTx,
	ogpSync OgpSyncGenerator,
	logger *slog.Logger,
) *PublishFromDraft {
	if logger == nil {
		logger = slog.Default()
	}
	return &PublishFromDraft{
		pool:                 pool,
		photobookRepoFactory: photobookRepoFactory,
		revokerFactory:       revokerFactory,
		slugGen:              slugGen,
		usage:                usage,
		ogpEnsurer:           ogpEnsurer,
		ogpSync:              ogpSync,
		logger:               logger,
	}
}

// Execute は WithTx 内で publish + RevokeAllDrafts を原子的に実行する。
func (u *PublishFromDraft) Execute(
	ctx context.Context,
	in PublishFromDraftInput,
) (PublishFromDraftOutput, error) {
	// 2026-05-03 STOP α P0 v2: 権利・配慮確認同意 (rights_agreed) を必須化。
	// false の場合は UsageLimit / TX 開始の前に early reject（無駄なリソース消費を避ける）。
	// 業務知識 v4 §3.1: 公開前の権利・配慮確認は必須、同意日時も記録する。
	if !in.RightsAgreed {
		return PublishFromDraftOutput{}, domain.ErrRightsNotAgreed
	}

	// PR36: UsageLimit 連動（業務知識 v4 §3.7 同一作成元 1 時間 5 冊）。
	// salt 未設定 / RemoteIP 空 / usage nil ならいずれも skip し、Turnstile に依存。
	if u.usage != nil && in.IPHashSalt != "" && in.RemoteIP != "" {
		ipHashHex := computeIPHashHex(in.IPHashSalt, in.RemoteIP)
		ipScope, perr := usagelimitscopehash.Parse(ipHashHex)
		if perr != nil {
			return PublishFromDraftOutput{}, fmt.Errorf("scope_hash ip: %w", perr)
		}
		out, uerr := u.usage.Execute(ctx, usagelimitwireup.CheckInput{
			ScopeType:          usagelimitscopetype.SourceIPHash(),
			ScopeHash:          ipScope,
			Action:             usagelimitaction.PublishFromDraft(),
			Now:                in.Now,
			WindowSeconds:      3600,
			Limit:              5,
			RetentionGraceSecs: 86400,
		})
		if uerr != nil {
			return PublishFromDraftOutput{}, MapPublishUsageErr(uerr, out.RetryAfterSeconds)
		}
	}

	publicSlug, err := u.slugGen.Generate(ctx)
	if err != nil {
		return PublishFromDraftOutput{}, fmt.Errorf("slug gen: %w", err)
	}
	rawManage, err := manage_url_token.Generate()
	if err != nil {
		return PublishFromDraftOutput{}, fmt.Errorf("manage token gen: %w", err)
	}
	manageHash := manage_url_token_hash.Of(rawManage)

	var resultPB domain.Photobook
	err = database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := u.photobookRepoFactory(tx)
		revoker := u.revokerFactory(tx)

		pb, err := repo.FindByID(ctx, in.PhotobookID)
		if err != nil {
			if errors.Is(err, rdb.ErrNotFound) {
				return ErrPublishConflict
			}
			return err
		}
		// 2026-05-03 STOP α P0 v2: domain に同意を反映してから CanPublish。
		// 同意のみ残って publish 失敗 / publish のみ通って同意未保存 を回避するため、
		// repository.PublishFromDraft の SQL も rights_agreed=true / rights_agreed_at=$4
		// を同 UPDATE で書く（同 TX、同行）。
		pb = pb.WithRightsAgreed(in.Now)
		if err := pb.CanPublish(); err != nil {
			return err
		}
		if pb.Version() != in.ExpectedVersion {
			return ErrPublishConflict
		}
		// domain 側の状態遷移（DB UPDATE は次の repo 呼び出し）
		next, err := pb.Publish(publicSlug, manageHash, in.Now)
		if err != nil {
			return err
		}
		if err := repo.PublishFromDraft(ctx, pb.ID(), publicSlug, manageHash, in.Now, in.ExpectedVersion); err != nil {
			if errors.Is(err, rdb.ErrOptimisticLockConflict) {
				return ErrPublishConflict
			}
			return err
		}
		if _, err := revoker.RevokeAllDrafts(ctx, pb.ID()); err != nil {
			return fmt.Errorf("revoke all drafts: %w", err)
		}
		// PR30: PhotobookPublished event を同一 TX で Outbox に INSERT。
		// payload は worker（PR31）が DB 再 fetch できる最小値のみ含める。
		// raw manage token / draft token / Cookie / presigned URL は入れない。
		var coverIDStr *string
		if cid := next.CoverImageID(); cid != nil {
			s := cid.String()
			coverIDStr = &s
		}
		ev, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Photobook(),
			AggregateID:   next.ID().UUID(),
			EventType:     event_type.PhotobookPublished(),
			Payload: outboxdomain.PhotobookPublishedPayload{
				EventVersion: outboxdomain.EventVersion,
				OccurredAt:   in.Now.UTC(),
				PhotobookID:  next.ID().String(),
				Slug:         publicSlug.String(),
				Visibility:   next.Visibility().String(),
				Type:         next.Type().String(),
				CoverImageID: coverIDStr,
			},
			Now: in.Now.UTC(),
		})
		if err != nil {
			return fmt.Errorf("build photobook.published event: %w", err)
		}
		if err := outboxrdb.NewOutboxRepository(tx).Create(ctx, ev); err != nil {
			return fmt.Errorf("outbox create photobook.published: %w", err)
		}
		// M-2 (ADR-0007 §3 (1)): photobook_ogp_images の pending 行を同一 TX で冪等 INSERT。
		// publish と pending 行の存在を atomic に揃え、1st X crawler hit 時に row が無く
		// `/og/default.png` redirect される事故を防ぐ。
		// SQL 側 ON CONFLICT DO NOTHING で worker 側 CreatePending との race を吸収する。
		if u.ogpEnsurer != nil {
			if err := u.ogpEnsurer.EnsurePendingWithTx(ctx, tx, pb.ID(), in.Now); err != nil {
				return fmt.Errorf("ogp ensure pending: %w", err)
			}
		}
		resultPB = next
		return nil
	})
	if err != nil {
		return PublishFromDraftOutput{}, err
	}

	// M-2 (ADR-0007 §3 (2)): commit 後 best-effort 同期生成。
	// timeout / 失敗時も publish 自体は成功扱い（pending 行は維持され worker fallback）。
	// 観測: slog `event=ogp_sync_result` で outcome / duration_ms を必ず出す。
	u.runOgpSync(ctx, resultPB.ID(), in.Now)

	return PublishFromDraftOutput{Photobook: resultPB, RawManageToken: rawManage}, nil
}

// runOgpSync は publish commit 後の best-effort sync を起動する。
//
// ogpSync が nil の場合は何もしない（旧互換）。
// timeout は ogpSyncTimeout (2.5s)。outcome / duration_ms を slog で必ず出力する。
// 内部 panic / OGP UC error は呼び出し側 (HTTP handler) には伝播させない（best-effort）。
//
// 注意: 親 ctx が既に cancel されている場合でも sync を試みるため、parent ctx を
// 引き継ぐが timeout のみ 2.5s で制約する。HTTP response を返した直後に worker が
// goroutine を即座に reap しないよう、同期実行 (caller blocking) で実装する。
func (u *PublishFromDraft) runOgpSync(ctx context.Context, photobookID photobook_id.PhotobookID, now time.Time) {
	if u.ogpSync == nil {
		return
	}
	syncCtx, cancel := context.WithTimeout(ctx, ogpSyncTimeout)
	defer cancel()
	start := time.Now()
	outcome := u.ogpSync.GenerateSync(syncCtx, photobookID, now)
	durMs := time.Since(start).Milliseconds()

	// logger は本来 NewPublishFromDraft で nil チェック済だが、unit test 等で直接
	// 構築されるケースを想定し defensive に default へ倒す。
	logger := u.logger
	if logger == nil {
		logger = slog.Default()
	}
	attrs := []any{
		slog.String("event", "ogp_sync_result"),
		slog.String("outcome", string(outcome)),
		slog.Int64("duration_ms", durMs),
		slog.String("photobook_id", photobookID.String()),
	}
	if outcome == OgpSyncOutcomeSuccess {
		logger.InfoContext(ctx, "ogp sync after publish", attrs...)
	} else {
		// 失敗時も WarnContext のみ。publish 自体は成功扱いのため Error は使わない。
		logger.WarnContext(ctx, "ogp sync after publish (non-success)", attrs...)
	}
}
