// PR19 残りの編集系 UseCase: RemovePage / RemovePhoto / ReorderPhoto /
// SetCoverImage / ClearCoverImage / UpsertPageMeta。
//
// いずれも `*pgxpool.Pool` を受け、必要なら同一 TX で実行する。
package usecase

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// === RemovePage ===

type RemovePageInput struct {
	PhotobookID     photobook_id.PhotobookID
	PageID          page_id.PageID
	ExpectedVersion int
	Now             time.Time
}

type RemovePage struct{ pool *pgxpool.Pool }

func NewRemovePage(pool *pgxpool.Pool) *RemovePage { return &RemovePage{pool: pool} }

// Execute は Page を削除する（CASCADE で photos / page_metas も連鎖削除）。
func (u *RemovePage) Execute(ctx context.Context, in RemovePageInput) error {
	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)
		return repo.RemovePage(ctx, in.PhotobookID, in.PageID, in.ExpectedVersion, in.Now)
	})
}

// === RemovePhoto ===

type RemovePhotoInput struct {
	PhotobookID     photobook_id.PhotobookID
	PageID          page_id.PageID
	PhotoID         photo_id.PhotoID
	ExpectedVersion int
	Now             time.Time
}

type RemovePhoto struct{ pool *pgxpool.Pool }

func NewRemovePhoto(pool *pgxpool.Pool) *RemovePhoto { return &RemovePhoto{pool: pool} }

func (u *RemovePhoto) Execute(ctx context.Context, in RemovePhotoInput) error {
	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)
		return repo.RemovePhoto(ctx, in.PhotobookID, in.PageID, in.PhotoID, in.ExpectedVersion, in.Now)
	})
}

// === ReorderPhoto ===

type ReorderPhotoInput struct {
	PhotobookID     photobook_id.PhotobookID
	PhotoID         photo_id.PhotoID
	NewOrder        display_order.DisplayOrder
	ExpectedVersion int
	Now             time.Time
}

type ReorderPhoto struct{ pool *pgxpool.Pool }

func NewReorderPhoto(pool *pgxpool.Pool) *ReorderPhoto { return &ReorderPhoto{pool: pool} }

func (u *ReorderPhoto) Execute(ctx context.Context, in ReorderPhotoInput) error {
	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)
		return repo.ReorderPhoto(ctx, in.PhotobookID, in.PhotoID, in.NewOrder, in.ExpectedVersion, in.Now)
	})
}

// === SetCoverImage ===

type SetCoverImageInput struct {
	PhotobookID     photobook_id.PhotobookID
	ImageID         image_id.ImageID
	ExpectedVersion int
	Now             time.Time
}

type SetCoverImage struct{ pool *pgxpool.Pool }

func NewSetCoverImage(pool *pgxpool.Pool) *SetCoverImage { return &SetCoverImage{pool: pool} }

func (u *SetCoverImage) Execute(ctx context.Context, in SetCoverImageInput) error {
	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)
		return repo.SetCoverImage(ctx, in.PhotobookID, in.ImageID, in.ExpectedVersion, in.Now)
	})
}

// === ClearCoverImage ===

type ClearCoverImageInput struct {
	PhotobookID     photobook_id.PhotobookID
	ExpectedVersion int
	Now             time.Time
}

type ClearCoverImage struct{ pool *pgxpool.Pool }

func NewClearCoverImage(pool *pgxpool.Pool) *ClearCoverImage { return &ClearCoverImage{pool: pool} }

func (u *ClearCoverImage) Execute(ctx context.Context, in ClearCoverImageInput) error {
	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)
		return repo.ClearCoverImage(ctx, in.PhotobookID, in.ExpectedVersion, in.Now)
	})
}

// === UpsertPageMeta ===

type UpsertPageMetaInput struct {
	Meta domain.PageMeta
}

type UpsertPageMeta struct{ pool *pgxpool.Pool }

func NewUpsertPageMeta(pool *pgxpool.Pool) *UpsertPageMeta { return &UpsertPageMeta{pool: pool} }

func (u *UpsertPageMeta) Execute(ctx context.Context, in UpsertPageMetaInput) error {
	repo := photobookrdb.NewPhotobookRepository(u.pool)
	return repo.UpsertPageMeta(ctx, in.Meta)
}
