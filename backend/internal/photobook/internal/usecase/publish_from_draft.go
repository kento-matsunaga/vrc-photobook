package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
type PublishFromDraftInput struct {
	PhotobookID     photobook_id.PhotobookID
	ExpectedVersion int
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
}

// NewPublishFromDraft は UseCase を組み立てる。
//
// PR36: usage が nil の場合 UsageLimit 連動を行わない（旧互換維持）。
// 本番では非 nil で渡す。
func NewPublishFromDraft(
	pool *pgxpool.Pool,
	photobookRepoFactory PhotobookTxRepositoryFactory,
	revokerFactory DraftSessionRevokerFactory,
	slugGen SlugGenerator,
	usage *usagelimitwireup.Check,
) *PublishFromDraft {
	return &PublishFromDraft{
		pool:                 pool,
		photobookRepoFactory: photobookRepoFactory,
		revokerFactory:       revokerFactory,
		slugGen:              slugGen,
		usage:                usage,
	}
}

// Execute は WithTx 内で publish + RevokeAllDrafts を原子的に実行する。
func (u *PublishFromDraft) Execute(
	ctx context.Context,
	in PublishFromDraftInput,
) (PublishFromDraftOutput, error) {
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
		resultPB = next
		return nil
	})
	if err != nil {
		return PublishFromDraftOutput{}, err
	}
	return PublishFromDraftOutput{Photobook: resultPB, RawManageToken: rawManage}, nil
}
