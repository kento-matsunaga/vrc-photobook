// Package marshaller はドメインの Image / ImageVariant と sqlc 生成物 row の相互変換を担う。
//
// 設計方針（.agents/rules/domain-standard.md §インフラ）:
//   - VO ↔ プリミティブ変換は本パッケージに閉じる
//   - DB エラーはそのまま返し、ビジネス例外への変換は repository が担う
package marshaller

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/image/domain"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/failure_reason"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/image_format"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/image/domain/vo/image_status"
	"vrcpb/backend/internal/image/domain/vo/image_usage_kind"
	"vrcpb/backend/internal/image/domain/vo/mime_type"
	"vrcpb/backend/internal/image/domain/vo/normalized_format"
	"vrcpb/backend/internal/image/domain/vo/storage_key"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	"vrcpb/backend/internal/image/infrastructure/repository/rdb/sqlcgen"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// ErrInvalidRow は DB から取得した row が VO に変換できないとき。
var ErrInvalidRow = errors.New("invalid image row from db")

// ToCreateImageParams は uploading の Image を sqlc CreateImageParams に変換する。
func ToCreateImageParams(img domain.Image) (sqlcgen.CreateImageParams, error) {
	if !img.IsUploading() {
		return sqlcgen.CreateImageParams{}, errors.New("ToCreateImageParams expects status=uploading")
	}
	return sqlcgen.CreateImageParams{
		ID:               uuidToPg(img.ID().UUID()),
		OwnerPhotobookID: uuidToPg(img.OwnerPhotobookID().UUID()),
		UsageKind:        img.UsageKind().String(),
		SourceFormat:     img.SourceFormat().String(),
		UploadedAt:       timeToPg(img.UploadedAt()),
		CreatedAt:        timeToPg(img.CreatedAt()),
		UpdatedAt:        timeToPg(img.UpdatedAt()),
	}, nil
}

// FromImageRow は sqlcgen.Image をドメインに復元する（variant 無し）。
//
// variants は別途 ListImageVariantsByImageID で取得し、AttachVariant で組み立てる。
func FromImageRow(row sqlcgen.Image) (domain.Image, error) {
	if !row.ID.Valid {
		return domain.Image{}, ErrInvalidRow
	}
	id, err := image_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return domain.Image{}, err
	}
	if !row.OwnerPhotobookID.Valid {
		return domain.Image{}, ErrInvalidRow
	}
	owner, err := photobook_id.FromUUID(row.OwnerPhotobookID.Bytes)
	if err != nil {
		return domain.Image{}, err
	}
	usage, err := image_usage_kind.Parse(row.UsageKind)
	if err != nil {
		return domain.Image{}, err
	}
	srcFmt, err := image_format.Parse(row.SourceFormat)
	if err != nil {
		return domain.Image{}, err
	}
	status, err := image_status.Parse(row.Status)
	if err != nil {
		return domain.Image{}, err
	}

	var nf *normalized_format.NormalizedFormat
	if row.NormalizedFormat != nil {
		v, err := normalized_format.Parse(*row.NormalizedFormat)
		if err != nil {
			return domain.Image{}, err
		}
		nf = &v
	}
	var dims *image_dimensions.ImageDimensions
	if row.OriginalWidth != nil && row.OriginalHeight != nil {
		v, err := image_dimensions.New(int(*row.OriginalWidth), int(*row.OriginalHeight))
		if err != nil {
			return domain.Image{}, err
		}
		dims = &v
	}
	var bs *byte_size.ByteSize
	if row.OriginalByteSize != nil {
		v, err := byte_size.New(*row.OriginalByteSize)
		if err != nil {
			return domain.Image{}, err
		}
		bs = &v
	}
	var reason *failure_reason.FailureReason
	if row.FailureReason != nil {
		v, err := failure_reason.Parse(*row.FailureReason)
		if err != nil {
			return domain.Image{}, err
		}
		reason = &v
	}

	return domain.RestoreImage(domain.RestoreImageParams{
		ID:                 id,
		OwnerPhotobookID:   owner,
		UsageKind:          usage,
		SourceFormat:       srcFmt,
		NormalizedFormat:   nf,
		OriginalDimensions: dims,
		OriginalByteSize:   bs,
		MetadataStrippedAt: pgToTimePtr(row.MetadataStrippedAt),
		Status:             status,
		UploadedAt:         row.UploadedAt.Time,
		AvailableAt:        pgToTimePtr(row.AvailableAt),
		FailedAt:           pgToTimePtr(row.FailedAt),
		FailureReason:      reason,
		DeletedAt:          pgToTimePtr(row.DeletedAt),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	})
}

// ToCreateImageVariantParams は ImageVariant を sqlc params に変換する。
//
// id（variant 行 PK）はここで新規発行する。Image 集約 entity は variant の id を
// 持たないため、永続化時点で確定させる方針。
func ToCreateImageVariantParams(v domain.ImageVariant) (sqlcgen.CreateImageVariantParams, error) {
	if v.StorageKey().IsZero() {
		return sqlcgen.CreateImageVariantParams{}, ErrInvalidRow
	}
	rowID, err := uuid.NewV7()
	if err != nil {
		return sqlcgen.CreateImageVariantParams{}, err
	}
	return sqlcgen.CreateImageVariantParams{
		ID:         pgtype.UUID{Bytes: rowID, Valid: true},
		ImageID:    uuidToPg(v.ImageID().UUID()),
		Kind:       v.Kind().String(),
		StorageKey: v.StorageKey().String(),
		Width:      int32(v.Dimensions().Width()),
		Height:     int32(v.Dimensions().Height()),
		ByteSize:   v.ByteSize().Int64(),
		MimeType:   v.MimeType().String(),
		CreatedAt:  timeToPg(v.CreatedAt()),
	}, nil
}

// FromImageVariantRow は sqlc row をドメイン ImageVariant に復元する。
func FromImageVariantRow(row sqlcgen.ImageVariant) (domain.ImageVariant, error) {
	if !row.ImageID.Valid {
		return domain.ImageVariant{}, ErrInvalidRow
	}
	imgID, err := image_id.FromUUID(row.ImageID.Bytes)
	if err != nil {
		return domain.ImageVariant{}, err
	}
	kind, err := variant_kind.Parse(row.Kind)
	if err != nil {
		return domain.ImageVariant{}, err
	}
	key, err := storage_key.Parse(row.StorageKey)
	if err != nil {
		return domain.ImageVariant{}, err
	}
	dims, err := image_dimensions.New(int(row.Width), int(row.Height))
	if err != nil {
		return domain.ImageVariant{}, err
	}
	bs, err := byte_size.New(row.ByteSize)
	if err != nil {
		return domain.ImageVariant{}, err
	}
	mt, err := mime_type.Parse(row.MimeType)
	if err != nil {
		return domain.ImageVariant{}, err
	}
	return domain.NewImageVariant(domain.NewImageVariantParams{
		ImageID:    imgID,
		Kind:       kind,
		StorageKey: key,
		Dimensions: dims,
		ByteSize:   bs,
		MimeType:   mt,
		CreatedAt:  row.CreatedAt.Time,
	})
}

// === helpers ===

func uuidToPg(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func timeToPg(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func pgToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	x := t.Time
	return &x
}
