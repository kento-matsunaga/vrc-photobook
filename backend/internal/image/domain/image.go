// Package domain は Image 集約のドメインモデルを提供する。
//
// 設計参照:
//   - docs/design/aggregates/image/ドメイン設計.md
//   - docs/design/aggregates/image/データモデル設計.md
//   - docs/adr/0005-image-upload-flow.md
//
// PR18 段階で扱う状態:
//   uploading → processing → available
//                         └→ failed
//   uploading → failed
//   available  → deleted → purged
//   failed     → deleted → purged
//
// 副作用（DB UPDATE / R2 / Outbox）は本ファイルには持ち込まない。entity は不変条件を
// 守った新インスタンスを返すのみ。
//
// 不変条件（CHECK 制約と一致）:
//   - status='available'  のとき normalized_format / dimensions / byte_size /
//     metadata_stripped_at が必須
//   - status='failed'     のとき failure_reason が必須
//   - status IN ('deleted','purged') のとき deleted_at が必須
//   - owner_photobook_id は生成時不変
//   - variants は (image_id, kind) で重複しない
package domain

import (
	"errors"
	"time"

	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/failure_reason"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/image_format"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/image/domain/vo/image_status"
	"vrcpb/backend/internal/image/domain/vo/image_usage_kind"
	"vrcpb/backend/internal/image/domain/vo/normalized_format"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// 不変条件・状態遷移エラー。
var (
	ErrInvalidStateForRestore   = errors.New("invalid state combination for image restore")
	ErrInvalidVariant           = errors.New("invalid image variant")
	ErrDuplicateVariantKind     = errors.New("variant kind already attached")
	ErrIllegalStatusTransition  = errors.New("illegal image status transition")
	ErrAvailableMissingFields   = errors.New("status=available requires normalized_format / dimensions / byte_size / metadata_stripped_at")
	ErrFailedMissingReason      = errors.New("status=failed requires failure_reason")
	ErrDeletedMissingDeletedAt  = errors.New("status=deleted/purged requires deleted_at")
	ErrAttachOnNotAvailable     = errors.New("only available image can be attached to photobook")
)

// Image は集約ルート。
type Image struct {
	id                  image_id.ImageID
	ownerPhotobookID    photobook_id.PhotobookID
	usageKind           image_usage_kind.ImageUsageKind
	sourceFormat        image_format.ImageFormat
	normalizedFormat    *normalized_format.NormalizedFormat
	originalDimensions  *image_dimensions.ImageDimensions
	originalByteSize    *byte_size.ByteSize
	metadataStrippedAt  *time.Time
	status              image_status.ImageStatus
	uploadedAt          time.Time
	availableAt         *time.Time
	failedAt            *time.Time
	failureReason       *failure_reason.FailureReason
	deletedAt           *time.Time
	createdAt           time.Time
	updatedAt           time.Time
	variants            []ImageVariant
}

// NewUploadingImageParams は upload-intent 受領直後の Image 構築引数。
type NewUploadingImageParams struct {
	ID               image_id.ImageID
	OwnerPhotobookID photobook_id.PhotobookID
	UsageKind        image_usage_kind.ImageUsageKind
	SourceFormat     image_format.ImageFormat
	Now              time.Time
}

// NewUploadingImage は status=uploading の新規 Image を組み立てる。
//
// upload-intent 段階で確定する情報のみを引数に取り、寸法 / size / 正規化形式 / variant
// は後続（complete / image-processor）で確定する。
func NewUploadingImage(p NewUploadingImageParams) (Image, error) {
	if p.Now.IsZero() {
		return Image{}, ErrInvalidStateForRestore
	}
	return Image{
		id:               p.ID,
		ownerPhotobookID: p.OwnerPhotobookID,
		usageKind:        p.UsageKind,
		sourceFormat:     p.SourceFormat,
		status:           image_status.Uploading(),
		uploadedAt:       p.Now,
		createdAt:        p.Now,
		updatedAt:        p.Now,
	}, nil
}

// RestoreImageParams は DB から取り出した行をドメインに復元する引数。
type RestoreImageParams struct {
	ID                 image_id.ImageID
	OwnerPhotobookID   photobook_id.PhotobookID
	UsageKind          image_usage_kind.ImageUsageKind
	SourceFormat       image_format.ImageFormat
	NormalizedFormat   *normalized_format.NormalizedFormat
	OriginalDimensions *image_dimensions.ImageDimensions
	OriginalByteSize   *byte_size.ByteSize
	MetadataStrippedAt *time.Time
	Status             image_status.ImageStatus
	UploadedAt         time.Time
	AvailableAt        *time.Time
	FailedAt           *time.Time
	FailureReason      *failure_reason.FailureReason
	DeletedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Variants           []ImageVariant
}

// RestoreImage は DB row をドメインに復元する。
//
// status と各列の整合は CHECK で DB 側に強制されているが、ドメイン側でも再検証する
// （CHECK 漏れや不正テストデータからの保護）。
func RestoreImage(p RestoreImageParams) (Image, error) {
	switch {
	case p.Status.IsAvailable(),
		(p.Status.IsDeleted() && p.AvailableAt != nil),
		(p.Status.IsPurged() && p.AvailableAt != nil):
		if p.NormalizedFormat == nil || p.OriginalDimensions == nil ||
			p.OriginalByteSize == nil || p.MetadataStrippedAt == nil {
			return Image{}, ErrAvailableMissingFields
		}
	}
	if p.Status.IsFailed() && p.FailureReason == nil {
		return Image{}, ErrFailedMissingReason
	}
	if (p.Status.IsDeleted() || p.Status.IsPurged()) && p.DeletedAt == nil {
		return Image{}, ErrDeletedMissingDeletedAt
	}
	img := Image{
		id:                 p.ID,
		ownerPhotobookID:   p.OwnerPhotobookID,
		usageKind:          p.UsageKind,
		sourceFormat:       p.SourceFormat,
		normalizedFormat:   p.NormalizedFormat,
		originalDimensions: p.OriginalDimensions,
		originalByteSize:   p.OriginalByteSize,
		metadataStrippedAt: p.MetadataStrippedAt,
		status:             p.Status,
		uploadedAt:         p.UploadedAt,
		availableAt:        p.AvailableAt,
		failedAt:           p.FailedAt,
		failureReason:      p.FailureReason,
		deletedAt:          p.DeletedAt,
		createdAt:          p.CreatedAt,
		updatedAt:          p.UpdatedAt,
	}
	for _, v := range p.Variants {
		if v.imageID.UUID() != p.ID.UUID() {
			return Image{}, ErrInvalidVariant
		}
	}
	img.variants = append(img.variants, p.Variants...)
	return img, nil
}

// MarkProcessing は uploading → processing への遷移結果を返す（不変、新インスタンス）。
func (i Image) MarkProcessing(now time.Time) (Image, error) {
	if !i.status.IsUploading() {
		return Image{}, ErrIllegalStatusTransition
	}
	out := i.shallowCopy()
	out.status = image_status.Processing()
	out.updatedAt = now
	return out, nil
}

// MarkAvailableParams は available 遷移時に確定する情報。
type MarkAvailableParams struct {
	NormalizedFormat   normalized_format.NormalizedFormat
	OriginalDimensions image_dimensions.ImageDimensions
	OriginalByteSize   byte_size.ByteSize
	MetadataStrippedAt time.Time
	Now                time.Time
}

// MarkAvailable は processing → available への遷移結果を返す。
//
// 必須情報がそろわない場合はエラー。Variant は別途 AttachVariant で追加する。
func (i Image) MarkAvailable(p MarkAvailableParams) (Image, error) {
	if !i.status.IsProcessing() {
		return Image{}, ErrIllegalStatusTransition
	}
	if p.MetadataStrippedAt.IsZero() || p.Now.IsZero() {
		return Image{}, ErrAvailableMissingFields
	}
	out := i.shallowCopy()
	nf := p.NormalizedFormat
	od := p.OriginalDimensions
	ob := p.OriginalByteSize
	ms := p.MetadataStrippedAt
	out.normalizedFormat = &nf
	out.originalDimensions = &od
	out.originalByteSize = &ob
	out.metadataStrippedAt = &ms
	at := p.Now
	out.availableAt = &at
	out.status = image_status.Available()
	out.updatedAt = p.Now
	return out, nil
}

// MarkFailed は uploading / processing → failed への遷移結果を返す。
func (i Image) MarkFailed(reason failure_reason.FailureReason, now time.Time) (Image, error) {
	if !(i.status.IsUploading() || i.status.IsProcessing()) {
		return Image{}, ErrIllegalStatusTransition
	}
	out := i.shallowCopy()
	r := reason
	out.failureReason = &r
	at := now
	out.failedAt = &at
	out.status = image_status.Failed()
	out.updatedAt = now
	return out, nil
}

// MarkDeleted は available / failed → deleted への遷移結果を返す。
func (i Image) MarkDeleted(now time.Time) (Image, error) {
	if !(i.status.IsAvailable() || i.status.IsFailed()) {
		return Image{}, ErrIllegalStatusTransition
	}
	out := i.shallowCopy()
	at := now
	out.deletedAt = &at
	out.status = image_status.Deleted()
	out.updatedAt = now
	return out, nil
}

// AttachVariant は新しい variant を追加した Image を返す。
//
// 同 kind がすでに存在する場合は ErrDuplicateVariantKind。
// variant の image_id が本 Image の id と一致することを確認する。
func (i Image) AttachVariant(v ImageVariant) (Image, error) {
	if v.imageID.UUID() != i.id.UUID() {
		return Image{}, ErrInvalidVariant
	}
	for _, existing := range i.variants {
		if existing.kind.Equal(v.kind) {
			return Image{}, ErrDuplicateVariantKind
		}
	}
	out := i.shallowCopy()
	out.variants = append([]ImageVariant{}, i.variants...)
	out.variants = append(out.variants, v)
	return out, nil
}

// CanAttachToPhotobook は Image を Photobook に attach 可能か判定する。
//
// uploading / processing / failed / deleted / purged は不可。available のみ可。
func (i Image) CanAttachToPhotobook() bool {
	return i.status.IsAvailable()
}

// アクセサ。
func (i Image) ID() image_id.ImageID                                       { return i.id }
func (i Image) OwnerPhotobookID() photobook_id.PhotobookID                 { return i.ownerPhotobookID }
func (i Image) UsageKind() image_usage_kind.ImageUsageKind                 { return i.usageKind }
func (i Image) SourceFormat() image_format.ImageFormat                     { return i.sourceFormat }
func (i Image) NormalizedFormat() *normalized_format.NormalizedFormat      { return i.normalizedFormat }
func (i Image) OriginalDimensions() *image_dimensions.ImageDimensions      { return i.originalDimensions }
func (i Image) OriginalByteSize() *byte_size.ByteSize                      { return i.originalByteSize }
func (i Image) MetadataStrippedAt() *time.Time                             { return clonePtrTime(i.metadataStrippedAt) }
func (i Image) Status() image_status.ImageStatus                           { return i.status }
func (i Image) UploadedAt() time.Time                                      { return i.uploadedAt }
func (i Image) AvailableAt() *time.Time                                    { return clonePtrTime(i.availableAt) }
func (i Image) FailedAt() *time.Time                                       { return clonePtrTime(i.failedAt) }
func (i Image) FailureReason() *failure_reason.FailureReason               { return i.failureReason }
func (i Image) DeletedAt() *time.Time                                      { return clonePtrTime(i.deletedAt) }
func (i Image) CreatedAt() time.Time                                       { return i.createdAt }
func (i Image) UpdatedAt() time.Time                                       { return i.updatedAt }

// Variants は variants の copy slice を返す（不変保証）。
func (i Image) Variants() []ImageVariant {
	out := make([]ImageVariant, len(i.variants))
	copy(out, i.variants)
	return out
}

// VariantByKind は kind に一致する variant を返す。なければ false。
func (i Image) VariantByKind(kind variant_kind.VariantKind) (ImageVariant, bool) {
	for _, v := range i.variants {
		if v.kind.Equal(kind) {
			return v, true
		}
	}
	return ImageVariant{}, false
}

// 状態判定。
func (i Image) IsUploading() bool  { return i.status.IsUploading() }
func (i Image) IsProcessing() bool { return i.status.IsProcessing() }
func (i Image) IsAvailable() bool  { return i.status.IsAvailable() }
func (i Image) IsFailed() bool     { return i.status.IsFailed() }
func (i Image) IsDeleted() bool    { return i.status.IsDeleted() }
func (i Image) IsPurged() bool     { return i.status.IsPurged() }

// shallowCopy は variants slice の所有権を新インスタンスに委ねるための浅いコピー。
//
// MarkProcessing / MarkAvailable / MarkFailed / MarkDeleted は variants 内容を
// 変更しないため、slice header の共有でも安全。AttachVariant のみが新 slice を作る。
func (i Image) shallowCopy() Image {
	return i
}

func clonePtrTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}
