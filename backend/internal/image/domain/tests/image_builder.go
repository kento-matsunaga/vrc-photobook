// Package tests は Image ドメインのテスト用 Builder を提供する。
//
// 方針（.agents/rules/testing.md §Builder パターン）:
//   - Builder は t を保持しない（Build(t) で受け取る）
//   - メソッドテストの前提条件構築に使う
//   - コンストラクタテスト（NewUploadingImage / RestoreImage）では使わず、引数を直接渡す
package tests

import (
	"testing"
	"time"

	"vrcpb/backend/internal/image/domain"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/image_format"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/image/domain/vo/image_usage_kind"
	"vrcpb/backend/internal/image/domain/vo/mime_type"
	"vrcpb/backend/internal/image/domain/vo/storage_key"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// ImageBuilder は status=uploading の Image を組み立てる Builder。
type ImageBuilder struct {
	id               *image_id.ImageID
	ownerPhotobookID *photobook_id.PhotobookID
	usageKind        image_usage_kind.ImageUsageKind
	sourceFormat     image_format.ImageFormat
	now              time.Time
}

// NewImageBuilder は既定値の Builder を返す。
//
// 既定値:
//   - usage_kind=photo
//   - source_format=jpg
//   - now=2026-04-27 12:00:00 UTC
//
// id / owner_photobook_id は呼び出しごとに新規発行される（明示しない場合）。
func NewImageBuilder() *ImageBuilder {
	return &ImageBuilder{
		usageKind:    image_usage_kind.Photo(),
		sourceFormat: image_format.Jpg(),
		now:          time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC),
	}
}

// WithID は image_id を上書きする。
func (b *ImageBuilder) WithID(id image_id.ImageID) *ImageBuilder {
	b.id = &id
	return b
}

// WithOwnerPhotobookID は owner_photobook_id を上書きする。
func (b *ImageBuilder) WithOwnerPhotobookID(pid photobook_id.PhotobookID) *ImageBuilder {
	b.ownerPhotobookID = &pid
	return b
}

// WithUsageKind は usage_kind を上書きする。
func (b *ImageBuilder) WithUsageKind(k image_usage_kind.ImageUsageKind) *ImageBuilder {
	b.usageKind = k
	return b
}

// WithSourceFormat は source_format を上書きする。
func (b *ImageBuilder) WithSourceFormat(f image_format.ImageFormat) *ImageBuilder {
	b.sourceFormat = f
	return b
}

// WithNow は now を上書きする。
func (b *ImageBuilder) WithNow(t time.Time) *ImageBuilder {
	b.now = t
	return b
}

// Build は status=uploading の Image を生成する。
func (b *ImageBuilder) Build(t *testing.T) domain.Image {
	t.Helper()
	id := b.id
	if id == nil {
		v, err := image_id.New()
		if err != nil {
			t.Fatalf("image_id.New: %v", err)
		}
		id = &v
	}
	owner := b.ownerPhotobookID
	if owner == nil {
		v, err := photobook_id.New()
		if err != nil {
			t.Fatalf("photobook_id.New: %v", err)
		}
		owner = &v
	}
	img, err := domain.NewUploadingImage(domain.NewUploadingImageParams{
		ID:               *id,
		OwnerPhotobookID: *owner,
		UsageKind:        b.usageKind,
		SourceFormat:     b.sourceFormat,
		Now:              b.now,
	})
	if err != nil {
		t.Fatalf("NewUploadingImage: %v", err)
	}
	return img
}

// MakeDisplayVariant は Image に紐づく display variant を組み立てるヘルパ。
func MakeDisplayVariant(t *testing.T, img domain.Image) domain.ImageVariant {
	t.Helper()
	key, err := storage_key.GenerateForVariant(img.OwnerPhotobookID(), img.ID(), variant_kind.Display())
	if err != nil {
		t.Fatalf("storage_key.GenerateForVariant: %v", err)
	}
	dims, err := image_dimensions.New(1024, 768)
	if err != nil {
		t.Fatalf("image_dimensions.New: %v", err)
	}
	bs, err := byte_size.New(123_456)
	if err != nil {
		t.Fatalf("byte_size.New: %v", err)
	}
	v, err := domain.NewImageVariant(domain.NewImageVariantParams{
		ImageID:    img.ID(),
		Kind:       variant_kind.Display(),
		StorageKey: key,
		Dimensions: dims,
		ByteSize:   bs,
		MimeType:   mime_type.Webp(),
		CreatedAt:  img.CreatedAt(),
	})
	if err != nil {
		t.Fatalf("NewImageVariant: %v", err)
	}
	return v
}
