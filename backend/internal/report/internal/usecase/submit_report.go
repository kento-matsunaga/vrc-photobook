package usecase

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	outboxdomain "vrcpb/backend/internal/outbox/domain"
	"vrcpb/backend/internal/outbox/domain/vo/aggregate_type"
	"vrcpb/backend/internal/outbox/domain/vo/event_type"
	outboxrdb "vrcpb/backend/internal/outbox/infrastructure/repository/rdb"
	photobookdomain "vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/report/domain/entity"
	"vrcpb/backend/internal/report/domain/vo/report_detail"
	"vrcpb/backend/internal/report/domain/vo/report_id"
	"vrcpb/backend/internal/report/domain/vo/report_reason"
	"vrcpb/backend/internal/report/domain/vo/reporter_contact"
	"vrcpb/backend/internal/report/domain/vo/target_snapshot"
	reportrdb "vrcpb/backend/internal/report/infrastructure/repository/rdb"
	"vrcpb/backend/internal/turnstile"
	"vrcpb/backend/internal/usagelimit"
	usagelimitaction "vrcpb/backend/internal/usagelimit/domain/vo/action"
	usagelimitscopehash "vrcpb/backend/internal/usagelimit/domain/vo/scope_hash"
	usagelimitscopetype "vrcpb/backend/internal/usagelimit/domain/vo/scope_type"
	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

// SubmitReportInput は SubmitReport UseCase の入力。
//
// セキュリティ:
//   - RemoteIP 生値は呼び出し側 → 本 UseCase 内の HashSourceIP で hash 化に使うのみ。
//     DB / log / outbox payload に保存しない
//   - TurnstileToken / RemoteIP は使い終わったら破棄
type SubmitReportInput struct {
	Slug            string
	Reason          report_reason.ReportReason
	Detail          report_detail.ReportDetail
	ReporterContact reporter_contact.ReporterContact
	TurnstileToken  string
	RemoteIP        string // 呼び出し側で Cf-Connecting-Ip / X-Forwarded-For 末尾を取得
	Now             time.Time
}

// SubmitReportOutput は UseCase の戻り値。
type SubmitReportOutput struct {
	ReportID report_id.ReportID
}

// mapUsageErr は usagelimit UseCase のエラーを report 集約のエラーに変換する。
// fail-closed: ErrUsageRepositoryFailed もしくは ErrRateLimited はいずれも HTTP 429 にマップ。
// retryAfter は最低 1 秒、Repository 失敗時は 60 秒の安全側既定。
func mapUsageErr(err error, retryAfter int) error {
	switch {
	case errors.Is(err, usagelimitwireup.ErrRateLimited):
		if retryAfter < 1 {
			retryAfter = 1
		}
		return &RateLimited{RetryAfterSeconds: retryAfter, Cause: ErrRateLimited}
	case errors.Is(err, usagelimitwireup.ErrUsageRepositoryFailed):
		return &RateLimited{RetryAfterSeconds: 60, Cause: ErrRateLimiterUnavailable}
	default:
		return err
	}
}

// SubmitReport は通報を受け付けて DB + Outbox に同一 TX で書き込む UseCase。
//
// 処理:
//   1. Turnstile siteverify
//   2. slug → photobook 解決（FindAnyBySlug）+ 公開対象判定（assessReportEligibility:
//      status=published AND hidden_by_operator=false AND visibility != private）
//   3. snapshot 確保（slug / title / creator_display_name）
//   4. source_ip_hash 算出（salt + sha256）
//   5. 同一 TX で reports INSERT + outbox_events INSERT
//
// 公開対象判定（draft / private / hidden / deleted / purged）の理由を **外部に区別なく
// 漏らさない**ために、すべて ErrTargetNotEligibleForReport（HTTP 404）に集約する。
// 設計判断: docs/plan/post-pr36-submit-report-visibility-decision.md（案 B、unlisted も許可）
type SubmitReport struct {
	pool              *pgxpool.Pool
	turnstileVerifier turnstile.Verifier
	turnstileHostname string
	turnstileAction   string
	ipHashSalt        string // 空なら ErrSaltNotConfigured
	usage             *usagelimitwireup.Check
}

// NewSubmitReport は UseCase を組み立てる。
//
// salt が空文字でも組み立て自体は成功する（main.go の起動順序で env 未注入 / Cloud Run
// secretKeyRef 反映漏れがあっても起動継続するため）が、Execute は ErrSaltNotConfigured
// を即返す。
//
// usage が nil の場合 UsageLimit 連動を行わない（PR36 commit 3 以前の互換維持用）。
// 本番では非 nil で渡す。
func NewSubmitReport(
	pool *pgxpool.Pool,
	verifier turnstile.Verifier,
	turnstileHostname string,
	turnstileAction string,
	ipHashSalt string,
	usage *usagelimitwireup.Check,
) *SubmitReport {
	return &SubmitReport{
		pool:              pool,
		turnstileVerifier: verifier,
		turnstileHostname: turnstileHostname,
		turnstileAction:   turnstileAction,
		ipHashSalt:        ipHashSalt,
		usage:             usage,
	}
}

// Execute は同一 TX で通報を保存する。
func (u *SubmitReport) Execute(ctx context.Context, in SubmitReportInput) (SubmitReportOutput, error) {
	if u.ipHashSalt == "" {
		return SubmitReportOutput{}, ErrSaltNotConfigured
	}
	// L4: 多層防御 Turnstile ガード（`.agents/rules/turnstile-defensive-guard.md`）。
	// 空白のみのトークンを Cloudflare siteverify に投げない。
	if strings.TrimSpace(in.TurnstileToken) == "" {
		return SubmitReportOutput{}, ErrTurnstileTokenMissing
	}

	// 1) Turnstile siteverify
	_, err := u.turnstileVerifier.Verify(ctx, turnstile.VerifyInput{
		Token:    in.TurnstileToken,
		RemoteIP: in.RemoteIP,
		Action:   u.turnstileAction,
		Hostname: u.turnstileHostname,
	})
	if err != nil {
		if errors.Is(err, turnstile.ErrUnavailable) {
			return SubmitReportOutput{}, ErrTurnstileUnavailable
		}
		// ErrVerificationFailed / hostname / action / stale をまとめて failed 扱い
		return SubmitReportOutput{}, ErrTurnstileVerificationFailed
	}

	// 2) slug → photobook 解決
	parsedSlug, err := slug.Parse(in.Slug)
	if err != nil {
		return SubmitReportOutput{}, ErrInvalidSlug
	}
	photobookRepo := photobookrdb.NewPhotobookRepository(u.pool)
	pb, err := photobookRepo.FindAnyBySlug(ctx, parsedSlug)
	if err != nil {
		if errors.Is(err, photobookrdb.ErrNotFound) {
			return SubmitReportOutput{}, ErrTargetNotEligibleForReport
		}
		return SubmitReportOutput{}, fmt.Errorf("find photobook by slug: %w", err)
	}
	// 公開対象判定: status=published AND hidden_by_operator=false AND visibility != private
	// 詳細は assessReportEligibility 参照。判定理由は外部に区別なく漏らさず
	// ErrTargetNotEligibleForReport（HTTP 404）に集約する。
	if err := assessReportEligibility(pb); err != nil {
		return SubmitReportOutput{}, err
	}

	// 3) snapshot 確保
	pbSlug := pb.PublicUrlSlug()
	if pbSlug == nil {
		// published だが slug が nil の異常系
		return SubmitReportOutput{}, ErrTargetNotEligibleForReport
	}
	creator := pb.CreatorDisplayName()
	var creatorPtr *string
	if creator != "" {
		creatorPtr = &creator
	}
	snap, err := target_snapshot.New(pbSlug.String(), pb.Title(), creatorPtr)
	if err != nil {
		return SubmitReportOutput{}, fmt.Errorf("build target snapshot: %w", err)
	}

	// 4) source_ip_hash 算出（生 IP は DB に保存せず）
	var ipHash []byte
	if in.RemoteIP != "" {
		ipHash = HashSourceIP(SaltVersionV1, u.ipHashSalt, in.RemoteIP)
	}

	// 4.5) UsageLimit 連動（PR36 commit 3 + 3.5）。
	// 業務知識 v4 §3.7 / PR36 計画書 §4.2 / §5.2 / §6.4 に従い 2 本のレートリミット。
	//
	// **どちらも scope_type='source_ip_hash' で統一**（v4 「UsageLimit と Report で IP
	// ハッシュソルト共有」と整合）。狭い制限の側は、IP × photobook の **複合 scope_hash**
	// を 1 軸に圧縮して同じ source_ip_hash 軸 bucket に詰める設計。scope_type は
	// 「主観点のラベル」として一貫した意味（source_ip_hash 軸の集計）を持たせる。
	//
	//   1. 「同一 IP × 同一 photobook」の 5 分 3 件:
	//      scope_type='source_ip_hash' / scope_hash=sha256(ip_hash || ":" || pid)
	//   2. 「同一 IP 全体」の 1 時間 20 件:
	//      scope_type='source_ip_hash' / scope_hash=ip_hash hex
	//
	// MVP 仕様: 1 を consume 後に 2 で deny されると「片方だけ count が進む」副作用がある。
	// PR36 計画書 §11 / §17 で許容済（CheckOnly + Consume 分離は後続検討）。
	// ip 取得不能時（RemoteIP 空 / hash 失敗）は UsageLimit を skip し、Turnstile に依存する。
	if u.usage != nil && len(ipHash) > 0 {
		ipHashHex := hex.EncodeToString(ipHash)
		pidUUID := pb.ID().UUID()
		pidHex := hex.EncodeToString(pidUUID[:])
		composedHash := usagelimit.ComposeIPHashAndPhotobookID(ipHashHex, pidHex)

		// 1: 同一 IP × 同一 photobook の 5 分 3 件
		composedScope, err := usagelimitscopehash.Parse(composedHash)
		if err != nil {
			return SubmitReportOutput{}, fmt.Errorf("scope_hash compose: %w", err)
		}
		ipScope, err := usagelimitscopehash.Parse(ipHashHex)
		if err != nil {
			return SubmitReportOutput{}, fmt.Errorf("scope_hash ip: %w", err)
		}
		out1, err := u.usage.Execute(ctx, usagelimitwireup.CheckInput{
			ScopeType:          usagelimitscopetype.SourceIPHash(),
			ScopeHash:          composedScope,
			Action:             usagelimitaction.ReportSubmit(),
			Now:                in.Now,
			WindowSeconds:      300,
			Limit:              3,
			RetentionGraceSecs: 86400,
		})
		if err != nil {
			return SubmitReportOutput{}, mapUsageErr(err, out1.RetryAfterSeconds)
		}
		// 2: 同一 IP 全体の 1 時間 20 件
		out2, err := u.usage.Execute(ctx, usagelimitwireup.CheckInput{
			ScopeType:          usagelimitscopetype.SourceIPHash(),
			ScopeHash:          ipScope,
			Action:             usagelimitaction.ReportSubmit(),
			Now:                in.Now,
			WindowSeconds:      3600,
			Limit:              20,
			RetentionGraceSecs: 86400,
		})
		if err != nil {
			return SubmitReportOutput{}, mapUsageErr(err, out2.RetryAfterSeconds)
		}
	}

	// 5) report id 生成
	rid, err := report_id.New()
	if err != nil {
		return SubmitReportOutput{}, fmt.Errorf("report_id gen: %w", err)
	}

	// 6) 同一 TX で reports INSERT + outbox INSERT
	rep, err := entity.NewSubmitted(entity.NewSubmittedParams{
		ID:                rid,
		TargetPhotobookID: pb.ID(),
		TargetSnapshot:    snap,
		Reason:            in.Reason,
		Detail:            in.Detail,
		ReporterContact:   in.ReporterContact,
		SubmittedAt:       in.Now,
		SourceIPHash:      ipHash,
	})
	if err != nil {
		return SubmitReportOutput{}, fmt.Errorf("build report entity: %w", err)
	}

	err = database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		reportRepo := reportrdb.NewReportRepository(tx)
		outboxRepo := outboxrdb.NewOutboxRepository(tx)

		if err := reportRepo.Create(ctx, rep); err != nil {
			return fmt.Errorf("reports insert: %w", err)
		}

		// Outbox payload には reporter_contact / detail / source_ip_hash を入れない
		ev, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Report(),
			AggregateID:   rid.UUID(),
			EventType:     event_type.ReportSubmitted(),
			Payload: outboxdomain.ReportSubmittedPayload{
				EventVersion:      outboxdomain.EventVersion,
				OccurredAt:        in.Now.UTC(),
				ReportID:          rid.String(),
				TargetPhotobookID: pb.ID().String(),
				Reason:            in.Reason.String(),
				HasContact:        in.ReporterContact.Present(),
			},
			Now: in.Now.UTC(),
		})
		if err != nil {
			return fmt.Errorf("build report.submitted event: %w", err)
		}
		if err := outboxRepo.Create(ctx, ev); err != nil {
			return fmt.Errorf("outbox create report.submitted: %w", err)
		}
		return nil
	})
	if err != nil {
		return SubmitReportOutput{}, err
	}
	return SubmitReportOutput{ReportID: rid}, nil
}

// assessReportEligibility は通報対象になり得るかを判定する。
//
// 受入条件:
//   - status = published
//   - hidden_by_operator = false
//   - visibility != private（public / unlisted を許可）
//
// それ以外はすべて ErrTargetNotEligibleForReport を返す。判定理由（draft / private /
// hidden / deleted / purged）の区別は外部に漏らさない（敵対者対策、`get_public_photobook.go`
// の既存ポリシーと整合）。
//
// 設計判断: docs/plan/post-pr36-submit-report-visibility-decision.md（案 B 採用）。
// 業務知識 v4 §3.6「閲覧者は通報できる」の自然な解釈と整合し、公開 Viewer
// (`assessPublicVisibility`) の visibility 判定（`!= private`）と同じ受入軸を採用する。
func assessReportEligibility(pb photobookdomain.Photobook) error {
	if !pb.Status().IsPublished() {
		return ErrTargetNotEligibleForReport
	}
	if pb.Visibility().Equal(visibility.Private()) {
		return ErrTargetNotEligibleForReport
	}
	if pb.HiddenByOperator() {
		return ErrTargetNotEligibleForReport
	}
	return nil
}
