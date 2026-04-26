// Package domain (Page / Photo / PageMeta).
//
// Photobook 集約の子エンティティ。集約ルートは Photobook。
// 本ファイルは domain ロジックのみ。DB UPDATE は Repository / UseCase の責務。
//
// 設計参照:
//   - docs/design/aggregates/photobook/データモデル設計.md §4
//   - docs/design/aggregates/photobook/ドメイン設計.md §3.2
package domain

import (
	"errors"
	"time"

	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// 不変条件・状態遷移エラー。
var (
	ErrInvalidPageState = errors.New("invalid page state")
)

// Page は photobook_pages 行を表すエンティティ。
type Page struct {
	id            page_id.PageID
	photobookID   photobook_id.PhotobookID
	displayOrder  display_order.DisplayOrder
	pageCaption   *caption.Caption
	createdAt     time.Time
	updatedAt     time.Time
}

// NewPageParams は新規 Page 作成の引数。
type NewPageParams struct {
	ID           page_id.PageID
	PhotobookID  photobook_id.PhotobookID
	DisplayOrder display_order.DisplayOrder
	Caption      *caption.Caption
	Now          time.Time
}

// NewPage は新規 Page を組み立てる。
func NewPage(p NewPageParams) (Page, error) {
	if p.Now.IsZero() {
		return Page{}, ErrInvalidPageState
	}
	return Page{
		id:           p.ID,
		photobookID:  p.PhotobookID,
		displayOrder: p.DisplayOrder,
		pageCaption:  p.Caption,
		createdAt:    p.Now,
		updatedAt:    p.Now,
	}, nil
}

// RestorePageParams は DB から復元する引数。
type RestorePageParams struct {
	ID           page_id.PageID
	PhotobookID  photobook_id.PhotobookID
	DisplayOrder display_order.DisplayOrder
	Caption      *caption.Caption
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RestorePage は DB row を Page に復元する。
func RestorePage(p RestorePageParams) Page {
	return Page{
		id:           p.ID,
		photobookID:  p.PhotobookID,
		displayOrder: p.DisplayOrder,
		pageCaption:  p.Caption,
		createdAt:    p.CreatedAt,
		updatedAt:    p.UpdatedAt,
	}
}

// アクセサ。
func (p Page) ID() page_id.PageID                          { return p.id }
func (p Page) PhotobookID() photobook_id.PhotobookID       { return p.photobookID }
func (p Page) DisplayOrder() display_order.DisplayOrder    { return p.displayOrder }
func (p Page) Caption() *caption.Caption                   { return p.pageCaption }
func (p Page) CreatedAt() time.Time                        { return p.createdAt }
func (p Page) UpdatedAt() time.Time                        { return p.updatedAt }

// Reorder は display_order を更新した新インスタンスを返す。
func (p Page) Reorder(newOrder display_order.DisplayOrder, now time.Time) Page {
	out := p
	out.displayOrder = newOrder
	out.updatedAt = now
	return out
}
