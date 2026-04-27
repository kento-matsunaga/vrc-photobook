// Package rdb は Image 集約の RDB Repository を提供する。
//
// 設計参照:
//   - docs/design/aggregates/image/データモデル設計.md
//   - docs/plan/m2-image-upload-plan.md §17
//
// 公開する操作（PR18 範囲）:
//   - CreateUploading: status=uploading の Image を INSERT
//   - FindByID: id 一致の Image + variants を取得
//   - ListActiveByPhotobookID: photobook 配下の deleted_at IS NULL を一覧
//   - MarkProcessing / MarkAvailable / MarkFailed / MarkDeleted: 状態遷移
//   - AttachVariant: ImageVariant を 1 行 INSERT（(image_id, kind) UNIQUE 衝突は ErrDuplicateVariantKind）
//   - ListVariantsByImageID: variant 一覧（運用 / テスト用）
//
// セキュリティ:
//   - 楽観ロックは photobook と異なり PR18 では未導入（Image は version 列を持たない）。
//     代わりに UPDATE 時の WHERE で「現在 status が遷移元」を条件に含める方針で安全側に倒す。
//   - 0 行 UPDATE は ErrConflict として返す。
package rdb

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/image/domain"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/image/infrastructure/repository/rdb/marshaller"
	"vrcpb/backend/internal/image/infrastructure/repository/rdb/sqlcgen"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// ビジネス例外。
var (
	ErrNotFound            = errors.New("image not found")
	ErrConflict            = errors.New("image state conflict (status mismatch)")
	ErrDuplicateVariantKind = errors.New("variant kind already exists for image")
)

// ImageRepository は images / image_variants への永続化を提供する。
type ImageRepository struct {
	q *sqlcgen.Queries
}

// NewImageRepository は pgx pool または tx（sqlcgen.DBTX を満たすもの）から Repository を作る。
func NewImageRepository(db sqlcgen.DBTX) *ImageRepository {
	return &ImageRepository{q: sqlcgen.New(db)}
}

// CreateUploading は status=uploading の Image を INSERT する。
//
// 業務上の必須事項（owner_photobook_id の存在）は FK で担保される。owner が無い場合は
// pgx 経由で 23503 (foreign_key_violation) が返るため、呼び出し側で捕捉すれば良い。
func (r *ImageRepository) CreateUploading(ctx context.Context, img domain.Image) error {
	params, err := marshaller.ToCreateImageParams(img)
	if err != nil {
		return err
	}
	return r.q.CreateImage(ctx, params)
}

// FindByID は id 一致の Image を返す。variant は同時に取得して entity に attach する。
func (r *ImageRepository) FindByID(ctx context.Context, id image_id.ImageID) (domain.Image, error) {
	row, err := r.q.FindImageByID(ctx, pgtype.UUID{Bytes: id.UUID(), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Image{}, ErrNotFound
		}
		return domain.Image{}, err
	}
	img, err := marshaller.FromImageRow(row)
	if err != nil {
		return domain.Image{}, err
	}
	return r.attachVariants(ctx, img)
}

// ListProcessingForUpdate は status='processing' の Image を最大 limit 件 claim する。
//
// FOR UPDATE SKIP LOCKED により並列 worker が同じ row を二重 claim しない。
// **必ず TX 内で呼び出すこと**（pool 直で呼ぶと lock がすぐ解放される）。
// variant は attach しない（claim 段階では不要、処理本体で使うのは id / source_format のみ）。
func (r *ImageRepository) ListProcessingForUpdate(
	ctx context.Context,
	limit int,
) ([]domain.Image, error) {
	rows, err := r.q.ListProcessingImagesForUpdate(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Image, 0, len(rows))
	for _, row := range rows {
		img, err := marshaller.FromImageRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, nil
}

// ListActiveByPhotobookID は deleted_at IS NULL の Image を発行順で返す（variant 込み）。
func (r *ImageRepository) ListActiveByPhotobookID(
	ctx context.Context,
	pid photobook_id.PhotobookID,
) ([]domain.Image, error) {
	rows, err := r.q.ListActiveImagesByPhotobookID(ctx, pgtype.UUID{Bytes: pid.UUID(), Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Image, 0, len(rows))
	for _, row := range rows {
		img, err := marshaller.FromImageRow(row)
		if err != nil {
			return nil, err
		}
		img, err = r.attachVariants(ctx, img)
		if err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, nil
}

// MarkProcessing は uploading → processing の UPDATE を実行する。
func (r *ImageRepository) MarkProcessing(ctx context.Context, img domain.Image) error {
	rows, err := r.q.UpdateImageStatusProcessing(ctx, sqlcgen.UpdateImageStatusProcessingParams{
		ID:        pgtype.UUID{Bytes: img.ID().UUID(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: img.UpdatedAt(), Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrConflict
	}
	return nil
}

// MarkAvailable は processing → available の UPDATE を実行する。
//
// img は MarkAvailable 適用済の domain.Image を期待する。
func (r *ImageRepository) MarkAvailable(ctx context.Context, img domain.Image) error {
	if !img.IsAvailable() {
		return errors.New("MarkAvailable expects status=available image")
	}
	if img.NormalizedFormat() == nil || img.OriginalDimensions() == nil ||
		img.OriginalByteSize() == nil || img.MetadataStrippedAt() == nil ||
		img.AvailableAt() == nil {
		return errors.New("MarkAvailable requires all available fields")
	}
	nf := img.NormalizedFormat().String()
	w := int32(img.OriginalDimensions().Width())
	h := int32(img.OriginalDimensions().Height())
	bs := img.OriginalByteSize().Int64()
	stripped := *img.MetadataStrippedAt()
	availAt := *img.AvailableAt()
	rows, err := r.q.UpdateImageStatusAvailable(ctx, sqlcgen.UpdateImageStatusAvailableParams{
		ID:                 pgtype.UUID{Bytes: img.ID().UUID(), Valid: true},
		NormalizedFormat:   &nf,
		OriginalWidth:      &w,
		OriginalHeight:     &h,
		OriginalByteSize:   &bs,
		MetadataStrippedAt: pgtype.Timestamptz{Time: stripped, Valid: true},
		AvailableAt:        pgtype.Timestamptz{Time: availAt, Valid: true},
		UpdatedAt:          pgtype.Timestamptz{Time: img.UpdatedAt(), Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrConflict
	}
	return nil
}

// MarkFailed は uploading|processing → failed の UPDATE を実行する。
func (r *ImageRepository) MarkFailed(ctx context.Context, img domain.Image) error {
	if !img.IsFailed() || img.FailureReason() == nil || img.FailedAt() == nil {
		return errors.New("MarkFailed requires status=failed + failure_reason + failed_at")
	}
	reason := img.FailureReason().String()
	failedAt := *img.FailedAt()
	rows, err := r.q.UpdateImageStatusFailed(ctx, sqlcgen.UpdateImageStatusFailedParams{
		ID:            pgtype.UUID{Bytes: img.ID().UUID(), Valid: true},
		FailureReason: &reason,
		FailedAt:      pgtype.Timestamptz{Time: failedAt, Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: img.UpdatedAt(), Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrConflict
	}
	return nil
}

// MarkDeleted は available|failed → deleted の UPDATE を実行する。
func (r *ImageRepository) MarkDeleted(ctx context.Context, img domain.Image) error {
	if !img.IsDeleted() || img.DeletedAt() == nil {
		return errors.New("MarkDeleted requires status=deleted + deleted_at")
	}
	deletedAt := *img.DeletedAt()
	rows, err := r.q.MarkImageDeleted(ctx, sqlcgen.MarkImageDeletedParams{
		ID:        pgtype.UUID{Bytes: img.ID().UUID(), Valid: true},
		DeletedAt: pgtype.Timestamptz{Time: deletedAt, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: img.UpdatedAt(), Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrConflict
	}
	return nil
}

// AttachVariant は ImageVariant を 1 行 INSERT する。
//
// (image_id, kind) UNIQUE 違反は ErrDuplicateVariantKind に変換する。
// その他の DB エラーはそのまま返す。
func (r *ImageRepository) AttachVariant(ctx context.Context, v domain.ImageVariant) error {
	params, err := marshaller.ToCreateImageVariantParams(v)
	if err != nil {
		return err
	}
	if err := r.q.CreateImageVariant(ctx, params); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateVariantKind
		}
		return err
	}
	return nil
}

// ListVariantsByImageID は variant 一覧を返す。
func (r *ImageRepository) ListVariantsByImageID(
	ctx context.Context,
	id image_id.ImageID,
) ([]domain.ImageVariant, error) {
	rows, err := r.q.ListImageVariantsByImageID(ctx, pgtype.UUID{Bytes: id.UUID(), Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ImageVariant, 0, len(rows))
	for _, row := range rows {
		v, err := marshaller.FromImageVariantRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *ImageRepository) attachVariants(ctx context.Context, img domain.Image) (domain.Image, error) {
	variants, err := r.ListVariantsByImageID(ctx, img.ID())
	if err != nil {
		return domain.Image{}, err
	}
	for _, v := range variants {
		img, err = img.AttachVariant(v)
		if err != nil {
			return domain.Image{}, err
		}
	}
	return img, nil
}
