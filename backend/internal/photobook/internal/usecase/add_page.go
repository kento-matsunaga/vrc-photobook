package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// AddPageInput は AddPage UseCase の入力。
type AddPageInput struct {
	PhotobookID     photobook_id.PhotobookID
	ExpectedVersion int
	Caption         *caption.Caption // nil 可（メタ未指定）
	Now             time.Time
}

// AddPageOutput は新規 Page。
type AddPageOutput struct {
	Page domain.Page
}

// AddPage は draft Photobook に新しい Page を末尾追加する UseCase。
//
// 同一 TX 内で:
//  1. 既存 Page 数を取得（30 上限チェック）
//  2. photobooks.version+1 (status=draft 確認)
//  3. photobook_pages INSERT
//
// expected_version 不一致 / status!=draft なら ErrOptimisticLockConflict（Repository
// 側 0 行 UPDATE 由来）。30 件超過時は domain.ErrPageLimitExceeded。
type AddPage struct {
	pool *pgxpool.Pool
}

// NewAddPage は UseCase を組み立てる。
func NewAddPage(pool *pgxpool.Pool) *AddPage {
	return &AddPage{pool: pool}
}

// Execute は新規 Page を末尾追加する。
func (u *AddPage) Execute(ctx context.Context, in AddPageInput) (AddPageOutput, error) {
	var out AddPageOutput
	err := database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)
		count, err := repo.CountPagesByPhotobookID(ctx, in.PhotobookID)
		if err != nil {
			return fmt.Errorf("count pages: %w", err)
		}
		if count >= domain.MaxPagesPerPhotobook {
			return domain.ErrPageLimitExceeded
		}
		newOrder, err := display_order.New(count) // 末尾追加
		if err != nil {
			return err
		}
		newID, err := page_id.New()
		if err != nil {
			return err
		}
		page, err := domain.NewPage(domain.NewPageParams{
			ID:           newID,
			PhotobookID:  in.PhotobookID,
			DisplayOrder: newOrder,
			Caption:      in.Caption,
			Now:          in.Now,
		})
		if err != nil {
			return err
		}
		if err := repo.AddPage(ctx, in.PhotobookID, page, in.ExpectedVersion, in.Now); err != nil {
			return err
		}
		out = AddPageOutput{Page: page}
		return nil
	})
	if err != nil {
		return AddPageOutput{}, err
	}
	return out, nil
}
