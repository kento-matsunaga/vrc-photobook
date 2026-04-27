// process_pending: 複数 Image の逐次処理。
//
// 設計参照:
//   - docs/plan/m2-image-processor-plan.md §10.7 / §10.8
//
// PR23 では single-worker CLI を前提に、claim TX 内で
// `SELECT ... FOR UPDATE SKIP LOCKED LIMIT 1` で 1 件取り出し、即 commit して lock を解放する。
// 並列 worker は将来追加（plan §17 Q9）。
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	imagedomain "vrcpb/backend/internal/image/domain"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
)

// ProcessPendingInput は バッチ処理の引数。
type ProcessPendingInput struct {
	MaxImages int       // 1 回の起動で処理する最大枚数（CLI 制御）。0 の場合は無制限ではなく 1 件。
	Now       time.Time // 起点時刻。各画像の MarkAvailable / MarkFailed に使われる。
	DryRun    bool      // true なら claim 結果を log に出すだけで処理しない。
}

// ProcessPendingOutput は集約サマリ。
type ProcessPendingOutput struct {
	Picked  int
	Success int
	Failed  int
}

// ProcessPending は status=processing の Image を順に処理する。
type ProcessPending struct {
	pool         *pgxpool.Pool
	r2Client     r2.Client
	processImage *ProcessImage
	logger       *slog.Logger
}

// NewProcessPending は ProcessImage を内部に持つ batch processor を組み立てる。
func NewProcessPending(pool *pgxpool.Pool, r2Client r2.Client, logger *slog.Logger) *ProcessPending {
	if logger == nil {
		logger = slog.Default()
	}
	return &ProcessPending{
		pool:         pool,
		r2Client:     r2Client,
		processImage: NewProcessImage(pool, r2Client, logger),
		logger:       logger,
	}
}

// Execute は MaxImages 件まで処理する。
//
// 1 件ずつ:
//   - 短い claim TX で 1 件取り出し（FOR UPDATE SKIP LOCKED）
//   - lock 解放後に重い処理（GetObject / decode / encode / PutObject）
//   - 別 TX で MarkAvailable + AttachVariant
//
// race の可能性: claim TX 内で他 worker が同じ row に到達することはない（FOR UPDATE
// SKIP LOCKED）。一方 claim TX commit 後に同じ row を別 worker が再度取り出す可能性は
// あるが、PR23 では single-worker 前提のため考慮外。multi-worker は plan §17 Q9 で再検討。
func (u *ProcessPending) Execute(ctx context.Context, in ProcessPendingInput) (ProcessPendingOutput, error) {
	max := in.MaxImages
	if max <= 0 {
		max = 1
	}
	out := ProcessPendingOutput{}

	// dry-run では Image の状態を変えないため、claim TX commit 後に同じ row が
	// 再取得される。同じ id を 2 回出さないよう memo する。
	seen := map[string]struct{}{}

	for i := 0; i < max; i++ {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		picked, err := u.claimOne(ctx)
		if err != nil {
			return out, err
		}
		if picked == nil {
			// もう処理対象なし。
			break
		}
		idKey := picked.ID().String()
		if _, dup := seen[idKey]; dup {
			// 同じ id を再び引いた = 残りは全て seen 済 = 処理対象なし。
			break
		}
		seen[idKey] = struct{}{}
		out.Picked++

		if in.DryRun {
			u.logger.InfoContext(ctx, "dry-run: would process",
				slog.String("image_id", idKey),
				slog.String("photobook_id", picked.OwnerPhotobookID().String()),
				slog.String("source_format", picked.SourceFormat().String()),
			)
			continue
		}

		res, err := u.processImage.Execute(ctx, ProcessImageInput{
			ImageID: picked.ID(),
			Now:     in.Now,
		})
		switch {
		case err == nil && res.Status == "available":
			out.Success++
		case errors.Is(err, ErrProcessFailed):
			// 想定済みの failure（MarkFailed 完了済）
			out.Failed++
		case errors.Is(err, ErrImageNotProcessing):
			// race: 他 worker が完了済（PR23 では起きない想定だが defensive）
			u.logger.WarnContext(ctx, "skipped: not in processing state",
				slog.String("image_id", picked.ID().String()),
			)
		default:
			// R2 / DB 系の retryable error。ログに残して次の画像へ進む。
			u.logger.ErrorContext(ctx, "process image failed (will retry next run)",
				slog.String("image_id", picked.ID().String()),
				slog.String("error", err.Error()),
			)
		}
	}
	return out, nil
}

// claimOne は短い TX 内で 1 件取り出す。lock は commit で解放される。
//
// PR23 single-worker 前提のため、commit 後 lock 解放されても他 worker と競合しない。
// multi-worker 化時は claim 用の status / claimed_at 列を追加する必要がある（plan §17 Q9）。
func (u *ProcessPending) claimOne(ctx context.Context) (*imagedomain.Image, error) {
	var picked *imagedomain.Image
	if err := database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := imagerdb.NewImageRepository(tx)
		images, err := repo.ListProcessingForUpdate(ctx, 1)
		if err != nil {
			return fmt.Errorf("list processing: %w", err)
		}
		if len(images) == 0 {
			return nil
		}
		v := images[0]
		picked = &v
		return nil
	}); err != nil {
		return nil, err
	}
	return picked, nil
}

