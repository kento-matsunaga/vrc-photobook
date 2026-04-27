// Package wireup は outbox-worker の構成要素を組み立てる facade。
//
// `internal/outbox/internal/usecase` は outbox 配下からのみ import 可能なため、
// `cmd/outbox-worker` は本パッケージ経由で Worker / HandlerRegistry を受け取る。
package wireup

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
	"vrcpb/backend/internal/outbox/internal/usecase/handlers"
	"vrcpb/backend/internal/outbox/domain/vo/event_type"
)

// RunInput / RunOutput は CLI から渡す薄い表現。
type RunInput struct {
	MaxEvents int
	DryRun    bool
}

type RunOutput struct {
	Picked      int
	Processed   int
	FailedRetry int
	Dead        int
	Skipped     int
}

// Runner は cmd/outbox-worker の起動エントリ。
type Runner interface {
	Run(ctx context.Context, in RunInput) (RunOutput, error)
	ReleaseStaleLocks(ctx context.Context, timeout time.Duration) (int64, error)
}

// runnerImpl は内部 Worker を保持する。
type runnerImpl struct {
	pool   *pgxpool.Pool
	worker *outboxusecase.Worker
	logger *slog.Logger
}

// Config は Runner 組み立て時の設定。
type Config struct {
	WorkerID    string
	MaxAttempts int
	Backoff     time.Duration
	MaxBackoff  time.Duration
}

// NewRunner は HandlerRegistry に現状の 3 種 handler を登録した Worker を組み立てる。
//
// 後続で event 種が増えたら、本関数の Register 呼び出しに追加し、同時に
// migrations で event_type CHECK を緩める。
func NewRunner(pool *pgxpool.Pool, cfg Config, logger *slog.Logger) Runner {
	if logger == nil {
		logger = slog.Default()
	}
	registry := outboxusecase.NewHandlerRegistry()
	registry.Register(event_type.PhotobookPublished().String(), handlers.NewPhotobookPublishedHandler(logger))
	registry.Register(event_type.ImageBecameAvailable().String(), handlers.NewImageBecameAvailableHandler(logger))
	registry.Register(event_type.ImageFailed().String(), handlers.NewImageFailedHandler(logger))

	worker := outboxusecase.NewWorker(pool, registry, outboxusecase.WorkerConfig{
		WorkerID:    cfg.WorkerID,
		MaxAttempts: cfg.MaxAttempts,
		Backoff:     cfg.Backoff,
		MaxBackoff:  cfg.MaxBackoff,
	}, logger)

	return &runnerImpl{pool: pool, worker: worker, logger: logger}
}

// Run は plan §15 想定の挙動で 1 batch を処理する。
func (r *runnerImpl) Run(ctx context.Context, in RunInput) (RunOutput, error) {
	out, err := r.worker.Run(ctx, outboxusecase.RunInput{
		MaxEvents: in.MaxEvents,
		Now:       nowFn(),
		DryRun:    in.DryRun,
	})
	return RunOutput{
		Picked:      out.Picked,
		Processed:   out.Processed,
		FailedRetry: out.FailedRetry,
		Dead:        out.Dead,
		Skipped:     out.Skipped,
	}, err
}

// ReleaseStaleLocks は processing で stuck した行を pending に戻す。
func (r *runnerImpl) ReleaseStaleLocks(ctx context.Context, timeout time.Duration) (int64, error) {
	return outboxusecase.ReleaseStaleLocks(ctx, r.pool, nowFn(), timeout, r.logger)
}

// nowFn は test で差し替え可能にする間接層。
var nowFn = func() time.Time { return time.Now().UTC() }
