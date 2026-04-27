// PR27 編集 UI 本格化のための追加 UseCase 群。
//
// 設計参照:
//   - docs/plan/m2-frontend-edit-ui-fullspec-plan.md §4 / §5
//   - .agents/rules/domain-standard.md §集約子テーブルと親 version OCC
//
// 公開する UseCase:
//   - UpdatePhotoCaption: photo caption 単独編集（version+1 と同一 TX）
//   - BulkReorderPhotosOnPage: 同 page 内の photo を一括再配置（一時退避方式）
//   - UpdatePhotobookSettings: draft Photobook の settings 一括 PATCH
//
// セキュリティ:
//   - すべて draft session middleware 通過後に呼ばれる前提
//   - 0 行 UPDATE は ErrOptimisticLockConflict / ErrPhotoNotFound に集約
//   - 失敗詳細を外部に漏らさない
package usecase

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// === UpdatePhotoCaption ===

// UpdatePhotoCaptionInput は photo caption 編集の入力。
type UpdatePhotoCaptionInput struct {
	PhotobookID     photobook_id.PhotobookID
	PhotoID         photo_id.PhotoID
	Caption         *caption.Caption // nil = caption をクリア
	ExpectedVersion int
	Now             time.Time
}

// UpdatePhotoCaption は photo caption 単独編集の UseCase。
type UpdatePhotoCaption struct{ pool *pgxpool.Pool }

// NewUpdatePhotoCaption は UseCase を組み立てる。
func NewUpdatePhotoCaption(pool *pgxpool.Pool) *UpdatePhotoCaption {
	return &UpdatePhotoCaption{pool: pool}
}

// Execute は version+1 + caption UPDATE を同一 TX で実行する。
func (u *UpdatePhotoCaption) Execute(ctx context.Context, in UpdatePhotoCaptionInput) error {
	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)
		return repo.UpdatePhotoCaption(ctx, in.PhotobookID, in.PhotoID, in.Caption, in.ExpectedVersion, in.Now)
	})
}

// === BulkReorderPhotosOnPage ===

// PhotoOrderItem は新しい display_order の指定。
type PhotoOrderItem struct {
	PhotoID  photo_id.PhotoID
	NewOrder display_order.DisplayOrder
}

// BulkReorderPhotosOnPageInput は一括 reorder の入力。
type BulkReorderPhotosOnPageInput struct {
	PhotobookID     photobook_id.PhotobookID
	PageID          page_id.PageID
	Assignments     []PhotoOrderItem
	ExpectedVersion int
	Now             time.Time
}

// BulkReorderPhotosOnPage は同 page 内 photo の display_order 一括再配置。
type BulkReorderPhotosOnPage struct{ pool *pgxpool.Pool }

// NewBulkReorderPhotosOnPage は UseCase を組み立てる。
func NewBulkReorderPhotosOnPage(pool *pgxpool.Pool) *BulkReorderPhotosOnPage {
	return &BulkReorderPhotosOnPage{pool: pool}
}

// Execute は一時退避（+1000）→ 順次 UPDATE で UNIQUE 衝突を回避しつつ再配置する。
func (u *BulkReorderPhotosOnPage) Execute(ctx context.Context, in BulkReorderPhotosOnPageInput) error {
	return database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := photobookrdb.NewPhotobookRepository(tx)
		assigns := make([]photobookrdb.PhotoOrderAssignment, 0, len(in.Assignments))
		for _, a := range in.Assignments {
			assigns = append(assigns, photobookrdb.PhotoOrderAssignment{
				PhotoID:  a.PhotoID,
				NewOrder: a.NewOrder,
			})
		}
		return repo.BulkReorderPhotosOnPage(ctx, in.PhotobookID, in.PageID, assigns, in.ExpectedVersion, in.Now)
	})
}

// === UpdatePhotobookSettings ===

// UpdatePhotobookSettingsInput は settings 一括 PATCH の入力。
//
// すべて文字列レベルで受ける（VO への parse は UseCase 内で実施し、不正値は
// ErrInvalidSettings に集約）。これにより HTTP layer は単純な JSON decode で済む。
type UpdatePhotobookSettingsInput struct {
	PhotobookID     photobook_id.PhotobookID
	Type            string
	Title           string
	Description     *string
	Layout          string
	OpeningStyle    string
	Visibility      string
	CoverTitle      *string
	ExpectedVersion int
	Now             time.Time
}

// UpdatePhotobookSettings は draft Photobook の settings 一括 PATCH UseCase。
type UpdatePhotobookSettings struct{ pool *pgxpool.Pool }

// NewUpdatePhotobookSettings は UseCase を組み立てる。
func NewUpdatePhotobookSettings(pool *pgxpool.Pool) *UpdatePhotobookSettings {
	return &UpdatePhotobookSettings{pool: pool}
}

// Execute は VO 検証 + settings UPDATE を実行する。
//
// title / description は呼び出し側で長さ検証済の前提。VO への parse は最低限のもののみ
// 実施（type / layout / opening_style / visibility の enum 検証）。
func (u *UpdatePhotobookSettings) Execute(ctx context.Context, in UpdatePhotobookSettingsInput) error {
	repo := photobookrdb.NewPhotobookRepository(u.pool)
	return repo.UpdateSettings(ctx, in.PhotobookID, photobookrdb.PhotobookSettings{
		Type:         in.Type,
		Title:        in.Title,
		Description:  in.Description,
		Layout:       in.Layout,
		OpeningStyle: in.OpeningStyle,
		Visibility:   in.Visibility,
		CoverTitle:   in.CoverTitle,
	}, in.ExpectedVersion, in.Now)
}
