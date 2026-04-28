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

	"vrcpb/backend/internal/outbox/contract"
	"vrcpb/backend/internal/outbox/domain/vo/event_type"
	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
	"vrcpb/backend/internal/outbox/internal/usecase/handlers"
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
//
// PR33d 以降:
//   - OgpGenerator が nil の場合、photobook.published handler は **登録されない**
//     （OGP 機能未配線の環境で worker を動かしても event を消費しない、安全側）
type Config struct {
	WorkerID     string
	MaxAttempts  int
	Backoff      time.Duration
	MaxBackoff   time.Duration
	OgpGenerator contract.OgpGenerator
}

// NewRunner は HandlerRegistry に handler を登録した Worker を組み立てる。
//
// 後続で event 種が増えたら、本関数の Register 呼び出しに追加し、同時に
// migrations で event_type CHECK を緩める。
//
// PR33d: photobook.published は副作用 handler（OGP 生成）に切り替わった。
// cfg.OgpGenerator が nil の場合は handler 自体を登録せず、event はそのまま
// pending / failed に滞留する（pending event を意図せず processed に進めないため）。
func NewRunner(pool *pgxpool.Pool, cfg Config, logger *slog.Logger) Runner {
	if logger == nil {
		logger = slog.Default()
	}
	registry := outboxusecase.NewHandlerRegistry()
	if cfg.OgpGenerator != nil {
		registry.Register(
			event_type.PhotobookPublished().String(),
			handlers.NewPhotobookPublishedHandler(cfg.OgpGenerator, logger),
		)
	}
	registry.Register(event_type.ImageBecameAvailable().String(), handlers.NewImageBecameAvailableHandler(logger))
	registry.Register(event_type.ImageFailed().String(), handlers.NewImageFailedHandler(logger))
	// PR34b: moderation hide/unhide event の no-op handler。
	// 副作用（CDN purge / OGP cache invalidation）は後続 PR で追加する。
	registry.Register(event_type.PhotobookHidden().String(), handlers.NewPhotobookHiddenHandler(logger))
	registry.Register(event_type.PhotobookUnhidden().String(), handlers.NewPhotobookUnhiddenHandler(logger))

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
