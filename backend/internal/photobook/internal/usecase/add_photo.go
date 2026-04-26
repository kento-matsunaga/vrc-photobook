package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// AddPhotoInput は AddPhoto の入力。
type AddPhotoInput struct {
	PhotobookID     photobook_id.PhotobookID
	PageID          page_id.PageID
	ImageID         image_id.ImageID
	ExpectedVersion int
	Caption         *caption.Caption
	Now             time.Time
}

// AddPhotoOutput は新規 Photo。
type AddPhotoOutput struct {
	Photo domain.Photo
}

// AddPhoto は Page に新しい Photo を末尾追加する UseCase。
//
// 同一 TX 内で:
//  1. 既存 Photo 数を取得（20 上限）
//  2. photobooks.version+1 (status=draft 確認)
//  3. images の owner+status FOR UPDATE 検証
//  4. photobook_photos INSERT
//
// 失敗パス:
//   - ErrPhotoLimitExceeded（20 超）
//   - ErrOptimisticLockConflict / ErrNotDraft（photobooks UPDATE 0 行）
//   - ErrImageNotAttachable（owner 違反 / status != available / deleted）
type AddPhoto struct {
	pool *pgxpool.Pool
}

// NewAddPhoto は UseCase を組み立てる。
func NewAddPhoto(pool *pgxpool.Pool) *AddPhoto {
	return &AddPhoto{pool: pool}
}

// Execute は新規 Photo を末尾追加する。
func (u *AddPhoto) Execute(ctx context.Context, in AddPhotoInput) (AddPhotoOutput, error) {
	var out AddPhotoOutput
	err := database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)
		count, err := repo.CountPhotosByPageID(ctx, in.PageID)
		if err != nil {
			return fmt.Errorf("count photos: %w", err)
		}
		if count >= domain.MaxPhotosPerPage {
			return domain.ErrPhotoLimitExceeded
		}
		newOrder, err := display_order.New(count)
		if err != nil {
			return err
		}
		newID, err := photo_id.New()
		if err != nil {
			return err
		}
		photo, err := domain.NewPhoto(domain.NewPhotoParams{
			ID:           newID,
			PageID:       in.PageID,
			ImageID:      in.ImageID,
			DisplayOrder: newOrder,
			Caption:      in.Caption,
			Now:          in.Now,
		})
		if err != nil {
			return err
		}
		if err := repo.AddPhoto(ctx, in.PhotobookID, in.PageID, photo, in.ExpectedVersion, in.Now); err != nil {
			return err
		}
		out = AddPhotoOutput{Photo: photo}
		return nil
	})
	if err != nil {
		return AddPhotoOutput{}, err
	}
	return out, nil
}
