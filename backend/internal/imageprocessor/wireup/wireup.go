// Package wireup は image-processor 構成要素の組み立てを公開する。
//
// `internal/usecase` は imageprocessor 内部からのみ import 可能なため、
// cmd/image-processor は wireup 経由で Runner（Execute メソッドだけを公開する interface）
// を取得する。
package wireup

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	processorusecase "vrcpb/backend/internal/imageprocessor/internal/usecase"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
)

// RunInput は Runner.Run の引数。
type RunInput struct {
	MaxImages int
	DryRun    bool
}

// RunOutput は Runner.Run の結果。
type RunOutput struct {
	Picked  int
	Success int
	Failed  int
}

// Runner は image-processor の起動エントリ。
type Runner interface {
	Run(ctx context.Context, in RunInput) (RunOutput, error)
}

// runnerImpl は ProcessPending を内部に持つ。
type runnerImpl struct {
	pp *processorusecase.ProcessPending
}

// NewRunner は ProcessPending UseCase を組み立てて Runner として返す。
func NewRunner(pool *pgxpool.Pool, r2Client r2.Client, logger *slog.Logger) Runner {
	return &runnerImpl{pp: processorusecase.NewProcessPending(pool, r2Client, logger)}
}

// Run は plan §15.1 の挙動で Image を処理する。
func (r *runnerImpl) Run(ctx context.Context, in RunInput) (RunOutput, error) {
	out, err := r.pp.Execute(ctx, processorusecase.ProcessPendingInput{
		MaxImages: in.MaxImages,
		Now:       nowFn(),
		DryRun:    in.DryRun,
	})
	return RunOutput{Picked: out.Picked, Success: out.Success, Failed: out.Failed}, err
}

// nowFn は test で差し替え可能にする間接層（package 内 var）。
var nowFn = defaultNow
