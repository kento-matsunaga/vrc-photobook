// Package wireup は Report UseCase の facade（cmd/ops / Backend HTTP layer 用）。
//
// `internal/report/internal/usecase` は report サブツリーからのみ import 可能なため、
// 入出力型 / sentinel エラーを re-export する。
package wireup

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/report/internal/usecase"
	"vrcpb/backend/internal/turnstile"
)

// 入出力型 re-export。
type (
	SubmitReportInput        = usecase.SubmitReportInput
	SubmitReportOutput       = usecase.SubmitReportOutput
	GetReportForOpsInput     = usecase.GetReportForOpsInput
	GetReportForOpsOutput    = usecase.GetReportForOpsOutput
	ListReportsForOpsInput   = usecase.ListReportsForOpsInput
	ListReportsForOpsOutput  = usecase.ListReportsForOpsOutput
)

// エラー sentinel re-export。
var (
	ErrTargetNotEligibleForReport  = usecase.ErrTargetNotEligibleForReport
	ErrTurnstileTokenMissing       = usecase.ErrTurnstileTokenMissing
	ErrTurnstileVerificationFailed = usecase.ErrTurnstileVerificationFailed
	ErrTurnstileUnavailable        = usecase.ErrTurnstileUnavailable
	ErrInvalidSlug                 = usecase.ErrInvalidSlug
	ErrSaltNotConfigured           = usecase.ErrSaltNotConfigured
	ErrReportNotFound              = usecase.ErrReportNotFound
)

// Handlers は cmd/ops / Backend HTTP layer が使う facade。
type Handlers struct {
	submit      *usecase.SubmitReport
	getForOps   *usecase.GetReportForOps
	listForOps  *usecase.ListReportsForOps
}

// Config は BuildHandlers の依存。
type Config struct {
	TurnstileVerifier turnstile.Verifier
	TurnstileHostname string
	TurnstileAction   string // 既定 "report-submit"
	IPHashSalt        string // REPORT_IP_HASH_SALT_V1
}

// BuildHandlers は pool / Config から Report UseCase 群を組み立てる。
//
// pool が nil なら nil を返す（呼び出し側で endpoint / cmd 自体を出さない判断）。
// Turnstile / salt が空でも組み立てるが、SubmitReport 実行時に明示エラーで返す。
func BuildHandlers(pool *pgxpool.Pool, cfg Config, logger *slog.Logger) *Handlers {
	if logger == nil {
		logger = slog.Default()
	}
	if pool == nil {
		return nil
	}
	return &Handlers{
		submit:     usecase.NewSubmitReport(pool, cfg.TurnstileVerifier, cfg.TurnstileHostname, cfg.TurnstileAction, cfg.IPHashSalt),
		getForOps:  usecase.NewGetReportForOps(pool),
		listForOps: usecase.NewListReportsForOps(pool),
	}
}

// Submit は SubmitReport を実行する。
func (h *Handlers) Submit(ctx context.Context, in SubmitReportInput) (SubmitReportOutput, error) {
	return h.submit.Execute(ctx, in)
}

// Show は GetReportForOps を実行する。
func (h *Handlers) Show(ctx context.Context, in GetReportForOpsInput) (GetReportForOpsOutput, error) {
	return h.getForOps.Execute(ctx, in)
}

// List は ListReportsForOps を実行する。
func (h *Handlers) List(ctx context.Context, in ListReportsForOpsInput) (ListReportsForOpsOutput, error) {
	return h.listForOps.Execute(ctx, in)
}
