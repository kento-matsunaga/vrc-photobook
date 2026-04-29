package usecase

import (
	"context"
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
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/report/domain/entity"
	"vrcpb/backend/internal/report/domain/vo/report_detail"
	"vrcpb/backend/internal/report/domain/vo/report_id"
	"vrcpb/backend/internal/report/domain/vo/report_reason"
	"vrcpb/backend/internal/report/domain/vo/reporter_contact"
	"vrcpb/backend/internal/report/domain/vo/target_snapshot"
	reportrdb "vrcpb/backend/internal/report/infrastructure/repository/rdb"
	"vrcpb/backend/internal/turnstile"
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

// SubmitReport は通報を受け付けて DB + Outbox に同一 TX で書き込む UseCase。
//
// 処理:
//   1. Turnstile siteverify
//   2. slug → photobook 解決（FindAnyBySlug）+ 公開対象判定（published+visibility=public+hidden=false）
//   3. snapshot 確保（slug / title / creator_display_name）
//   4. source_ip_hash 算出（salt + sha256）
//   5. 同一 TX で reports INSERT + outbox_events INSERT
//
// 公開対象判定（draft / private / hidden / deleted / purged）の理由を **外部に区別なく
// 漏らさない**ために、すべて ErrTargetNotEligibleForReport（HTTP 404）に集約する。
type SubmitReport struct {
	pool             *pgxpool.Pool
	turnstileVerifier turnstile.Verifier
	turnstileHostname string
	turnstileAction   string
	ipHashSalt        string // 空なら ErrSaltNotConfigured
}

// NewSubmitReport は UseCase を組み立てる。
//
// salt が空文字でも組み立て自体は成功する（main.go の起動順序で env 未注入 / Cloud Run
// secretKeyRef 反映漏れがあっても起動継続するため）が、Execute は ErrSaltNotConfigured
// を即返す。
func NewSubmitReport(
	pool *pgxpool.Pool,
	verifier turnstile.Verifier,
	turnstileHostname string,
	turnstileAction string,
	ipHashSalt string,
) *SubmitReport {
	return &SubmitReport{
		pool:              pool,
		turnstileVerifier: verifier,
		turnstileHostname: turnstileHostname,
		turnstileAction:   turnstileAction,
		ipHashSalt:        ipHashSalt,
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
	// 公開対象判定: published + visibility=public + hidden=false
	if !pb.Status().IsPublished() {
		return SubmitReportOutput{}, ErrTargetNotEligibleForReport
	}
	if pb.Visibility().String() != "public" {
		return SubmitReportOutput{}, ErrTargetNotEligibleForReport
	}
	if pb.HiddenByOperator() {
		return SubmitReportOutput{}, ErrTargetNotEligibleForReport
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
