// Package domain (Photo).
//
// Photobook 集約の子エンティティ。Page の子。
//
// 設計参照:
//   - docs/design/aggregates/photobook/データモデル設計.md §6
//   - docs/design/aggregates/photobook/ドメイン設計.md §3.3
package domain

import (
	"errors"
	"time"

	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
)

// Photo の不変条件エラー。
var (
	ErrInvalidPhotoState = errors.New("invalid photo state")
)

// Photo は photobook_photos 行を表すエンティティ。
//
// owner_photobook_id 整合は Image 集約側で保証されるため、本構造体は image_id を保持
// するのみ。配置先 Page の photobook_id と Image の owner_photobook_id が一致するか
// は Repository / UseCase 層で検証する。
type Photo struct {
	id           photo_id.PhotoID
	pageID       page_id.PageID
	imageID      image_id.ImageID
	displayOrder display_order.DisplayOrder
	photoCaption *caption.Caption
	createdAt    time.Time
}

// NewPhotoParams は新規 Photo 作成の引数。
type NewPhotoParams struct {
	ID           photo_id.PhotoID
	PageID       page_id.PageID
	ImageID      image_id.ImageID
	DisplayOrder display_order.DisplayOrder
	Caption      *caption.Caption
	Now          time.Time
}

// NewPhoto は新規 Photo を組み立てる。
func NewPhoto(p NewPhotoParams) (Photo, error) {
	if p.Now.IsZero() {
		return Photo{}, ErrInvalidPhotoState
	}
	return Photo{
		id:           p.ID,
		pageID:       p.PageID,
		imageID:      p.ImageID,
		displayOrder: p.DisplayOrder,
		photoCaption: p.Caption,
		createdAt:    p.Now,
	}, nil
}

// RestorePhotoParams は DB から復元する引数。
type RestorePhotoParams struct {
	ID           photo_id.PhotoID
	PageID       page_id.PageID
	ImageID      image_id.ImageID
	DisplayOrder display_order.DisplayOrder
	Caption      *caption.Caption
	CreatedAt    time.Time
}

// RestorePhoto は DB row を Photo に復元する。
func RestorePhoto(p RestorePhotoParams) Photo {
	return Photo{
		id:           p.ID,
		pageID:       p.PageID,
		imageID:      p.ImageID,
		displayOrder: p.DisplayOrder,
		photoCaption: p.Caption,
		createdAt:    p.CreatedAt,
	}
}

// アクセサ。
func (p Photo) ID() photo_id.PhotoID                       { return p.id }
func (p Photo) PageID() page_id.PageID                     { return p.pageID }
func (p Photo) ImageID() image_id.ImageID                  { return p.imageID }
func (p Photo) DisplayOrder() display_order.DisplayOrder   { return p.displayOrder }
func (p Photo) Caption() *caption.Caption                  { return p.photoCaption }
func (p Photo) CreatedAt() time.Time                       { return p.createdAt }

// Reorder は display_order を更新した新インスタンスを返す。
func (p Photo) Reorder(newOrder display_order.DisplayOrder) Photo {
	out := p
	out.displayOrder = newOrder
	return out
}
