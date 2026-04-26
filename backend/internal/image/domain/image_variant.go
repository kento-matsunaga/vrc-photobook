// Package domain は Image 集約のドメインモデルを提供する。
//
// 設計参照:
//   - docs/design/aggregates/image/ドメイン設計.md
//   - docs/design/aggregates/image/データモデル設計.md
package domain

import (
	"time"

	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/image/domain/vo/mime_type"
	"vrcpb/backend/internal/image/domain/vo/storage_key"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
)

// ImageVariant は派生画像のメタデータ。
//
// 不変条件は値オブジェクト側で保証されるため、本構造体は値の入れ物として振る舞う。
// `(image_id, kind)` の UNIQUE は DB 側および Image entity の AttachVariant で担保。
type ImageVariant struct {
	imageID    image_id.ImageID
	kind       variant_kind.VariantKind
	storageKey storage_key.StorageKey
	dimensions image_dimensions.ImageDimensions
	byteSize   byte_size.ByteSize
	mimeType   mime_type.MimeType
	createdAt  time.Time
}

// NewImageVariantParams は ImageVariant 構築の引数。
type NewImageVariantParams struct {
	ImageID    image_id.ImageID
	Kind       variant_kind.VariantKind
	StorageKey storage_key.StorageKey
	Dimensions image_dimensions.ImageDimensions
	ByteSize   byte_size.ByteSize
	MimeType   mime_type.MimeType
	CreatedAt  time.Time
}

// NewImageVariant は新規 ImageVariant を組み立てる。
//
// VO 側で値域は保証されているため、ここでは StorageKey / CreatedAt の有効性のみを確認する。
func NewImageVariant(p NewImageVariantParams) (ImageVariant, error) {
	if p.StorageKey.IsZero() {
		return ImageVariant{}, ErrInvalidVariant
	}
	if p.CreatedAt.IsZero() {
		return ImageVariant{}, ErrInvalidVariant
	}
	return ImageVariant{
		imageID:    p.ImageID,
		kind:       p.Kind,
		storageKey: p.StorageKey,
		dimensions: p.Dimensions,
		byteSize:   p.ByteSize,
		mimeType:   p.MimeType,
		createdAt:  p.CreatedAt,
	}, nil
}

// アクセサ。
func (v ImageVariant) ImageID() image_id.ImageID                  { return v.imageID }
func (v ImageVariant) Kind() variant_kind.VariantKind             { return v.kind }
func (v ImageVariant) StorageKey() storage_key.StorageKey         { return v.storageKey }
func (v ImageVariant) Dimensions() image_dimensions.ImageDimensions { return v.dimensions }
func (v ImageVariant) ByteSize() byte_size.ByteSize               { return v.byteSize }
func (v ImageVariant) MimeType() mime_type.MimeType               { return v.mimeType }
func (v ImageVariant) CreatedAt() time.Time                       { return v.createdAt }
