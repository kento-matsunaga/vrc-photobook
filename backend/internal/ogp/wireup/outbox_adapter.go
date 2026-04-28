// outbox_adapter: outbox handler から OGP 生成を呼び出すための adapter。
//
// 公開する型 / sentinel は `internal/outbox/contract` で定義され、本 adapter は
// それを実装する。outbox/handlers（internal package）への直接 import を避け、
// boundary を保つ。
//
// 失敗マッピング:
//   - ogpusecase.ErrNotPublished → contract.ErrNotPublishedSkippable
//     （outbox worker が processed 扱いに倒す permanent skip）
//   - その他 error はそのまま伝播（outbox worker が attempts++ で retry / dead に振り分け）
package wireup

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/ogp/infrastructure/renderer"
	ogpusecase "vrcpb/backend/internal/ogp/internal/usecase"
	"vrcpb/backend/internal/outbox/contract"
	photobookid "vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// OutboxOgpAdapter は contract.OgpGenerator を実装する。
type OutboxOgpAdapter struct {
	uc *ogpusecase.GenerateOgpForPhotobook
}

// BuildOutboxOgpAdapter は adapter を組み立てる（renderer / fetcher / UseCase を内包）。
func BuildOutboxOgpAdapter(
	pool *pgxpool.Pool,
	r2Client r2.Client,
	bucketName string,
	logger *slog.Logger,
) (contract.OgpGenerator, error) {
	rdr, err := renderer.New()
	if err != nil {
		return nil, err
	}
	fetcher := ogpusecase.NewPhotobookFetcherFromRdb(pool)
	uc := ogpusecase.NewGenerateOgpForPhotobook(pool, fetcher, r2Client, rdr, bucketName, logger)
	return &OutboxOgpAdapter{uc: uc}, nil
}

// GenerateForPhotobook は OGP UseCase を呼び出し、結果 / error を contract 形式に変換する。
func (a *OutboxOgpAdapter) GenerateForPhotobook(
	ctx context.Context,
	photobookID uuid.UUID,
	now time.Time,
) (contract.OgpGenerateResult, error) {
	pid, err := photobookid.FromUUID(photobookID)
	if err != nil {
		return contract.OgpGenerateResult{}, err
	}
	res, err := a.uc.Execute(ctx, ogpusecase.GenerateOgpInput{
		PhotobookID: pid,
		Now:         now,
	})
	if err != nil {
		// ErrNotPublished は contract の永続 skip sentinel に変換
		if errors.Is(err, ogpusecase.ErrNotPublished) {
			return contract.OgpGenerateResult{}, contract.ErrNotPublishedSkippable
		}
		// ErrPhotobookNotFound 等は worker の retry / dead 経路に乗せる
		// （MaxAttempts 5 で dead に倒れる）
		return contract.OgpGenerateResult{}, err
	}
	return contract.OgpGenerateResult{
		OgpImageID: res.OgpImageID,
		Generated:  res.Generated,
	}, nil
}
