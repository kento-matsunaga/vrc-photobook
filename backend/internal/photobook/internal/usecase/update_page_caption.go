// STOP P-2: page caption 単独編集 UseCase。
//
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §3.4.1 / §11
// API spec:
//
//	PATCH /api/photobooks/{id}/pages/{pageId}/caption
//	{ "caption": string|null, "expected_version": number } -> { "version": number }
//
// 仕様:
//   - draft only / expected_version 必須
//   - 1 TX で BumpVersion + UpdatePageCaption
//   - caption length validation は domain.NewCaption 側で行う前提 (handler が VO 化)
//   - page が photobook 配下でない場合は ErrPageNotFound (Repository 層で SQL レベルに検出)
package usecase

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// UpdatePageCaptionInput は page caption 編集の入力。
type UpdatePageCaptionInput struct {
	PhotobookID     photobook_id.PhotobookID
	PageID          page_id.PageID
	Caption         *caption.Caption // nil = caption をクリア
	ExpectedVersion int
	Now             time.Time
}

// UpdatePageCaption は draft Photobook の page caption を単独編集する UseCase。
type UpdatePageCaption struct {
	pool *pgxpool.Pool
}

// NewUpdatePageCaption は UseCase を組み立てる。
func NewUpdatePageCaption(pool *pgxpool.Pool) *UpdatePageCaption {
	return &UpdatePageCaption{pool: pool}
}

// Execute は version+1 + page caption UPDATE を 1 TX で実行する。
//
// エラー mapping:
//   - photobookrdb.ErrOptimisticLockConflict: version 不一致 / status!=draft
//   - photobookrdb.ErrPageNotFound: pageID が photobookID 配下にない / 不存在
func (u *UpdatePageCaption) Execute(ctx context.Context, in UpdatePageCaptionInput) error {
	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)
		if err := repo.BumpVersion(ctx, in.PhotobookID, in.ExpectedVersion, in.Now); err != nil {
			return err
		}
		return repo.UpdatePageCaption(ctx, in.PhotobookID, in.PageID, in.Caption, in.Now)
	})
}
