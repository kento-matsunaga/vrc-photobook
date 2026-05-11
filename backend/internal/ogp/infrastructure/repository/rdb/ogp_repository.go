// Package rdb は photobook_ogp_images の RDB Repository。
//
// 設計参照:
//   - docs/plan/m2-ogp-generation-plan.md §6 / §9
//   - docs/design/cross-cutting/ogp-generation.md §3 / §4
package rdb

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	imageid "vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/ogp/domain"
	"vrcpb/backend/internal/ogp/domain/vo/ogp_failure_reason"
	"vrcpb/backend/internal/ogp/domain/vo/ogp_status"
	"vrcpb/backend/internal/ogp/domain/vo/ogp_version"
	"vrcpb/backend/internal/ogp/infrastructure/repository/rdb/sqlcgen"
	photobookid "vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// ErrNotFound は対象 photobook の OGP row が存在しないとき。
var ErrNotFound = errors.New("ogp image not found")

// OgpRepository は photobook_ogp_images への永続化を提供する。
type OgpRepository struct {
	q *sqlcgen.Queries
}

// NewOgpRepository は pool / Tx から Repository を作る。
func NewOgpRepository(db sqlcgen.DBTX) *OgpRepository {
	return &OgpRepository{q: sqlcgen.New(db)}
}

// CreatePending は新規 pending row を INSERT する。
//
// photobook_id UNIQUE 違反は呼び出し側で確認（FindByPhotobookID で先に存在チェックを推奨）。
func (r *OgpRepository) CreatePending(ctx context.Context, ev domain.OgpImage) error {
	return r.q.CreatePendingOgp(ctx, sqlcgen.CreatePendingOgpParams{
		ID:          pgtype.UUID{Bytes: ev.ID(), Valid: true},
		PhotobookID: pgtype.UUID{Bytes: ev.PhotobookID().UUID(), Valid: true},
		CreatedAt:   pgtype.Timestamptz{Time: ev.CreatedAt(), Valid: true},
	})
}

// EnsureCreatedPending は photobook_id を key にした冪等 INSERT を実行する。
//
// M-2 OGP 同期化 (STOP β、ADR-0007): publish UC が WithTx 内で pending 行を先行
// INSERT する用途。photobook_id UNIQUE 違反は SQL 側 ON CONFLICT DO NOTHING で吸収し、
// 既存 row があっても error にしない（worker 側の `CreatePending` を経た後に再度
// publish 経路が走るケース等を含む冪等動作）。
//
// 呼び出し側は内部で uuid v7 / domain.NewPending を実行し、両者が race しても
// SQL 側 UNIQUE constraint で先勝ち row が残るため安全。
func (r *OgpRepository) EnsureCreatedPending(
	ctx context.Context,
	photobookID photobookid.PhotobookID,
	now time.Time,
) error {
	pending, err := domain.NewPending(domain.NewPendingParams{
		PhotobookID: photobookID,
		Now:         now,
	})
	if err != nil {
		return err
	}
	return r.q.EnsureCreatedPendingOgp(ctx, sqlcgen.EnsureCreatedPendingOgpParams{
		ID:          pgtype.UUID{Bytes: pending.ID(), Valid: true},
		PhotobookID: pgtype.UUID{Bytes: photobookID.UUID(), Valid: true},
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	})
}

// FindByPhotobookID は photobook_id から row を取得する（無ければ ErrNotFound）。
func (r *OgpRepository) FindByPhotobookID(ctx context.Context, pid photobookid.PhotobookID) (domain.OgpImage, error) {
	row, err := r.q.FindOgpByPhotobookID(ctx, pgtype.UUID{Bytes: pid.UUID(), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OgpImage{}, ErrNotFound
		}
		return domain.OgpImage{}, err
	}
	return toDomain(row)
}

// MarkGenerated は status='generated' に遷移させる。image_id / generated_at 必須。
func (r *OgpRepository) MarkGenerated(ctx context.Context, ev domain.OgpImage) error {
	imgID := ev.ImageID()
	if imgID == nil {
		return domain.ErrImageIDRequiredForGenerated
	}
	gAt := ev.GeneratedAt()
	if gAt == nil {
		return domain.ErrGeneratedAtRequired
	}
	return r.q.MarkOgpGenerated(ctx, sqlcgen.MarkOgpGeneratedParams{
		ID:          pgtype.UUID{Bytes: ev.ID(), Valid: true},
		ImageID:     pgtype.UUID{Bytes: imgID.UUID(), Valid: true},
		GeneratedAt: pgtype.Timestamptz{Time: *gAt, Valid: true},
	})
}

// MarkFailed は status='failed' に遷移させる。failure_reason は VO で sanitize 済。
func (r *OgpRepository) MarkFailed(ctx context.Context, ev domain.OgpImage) error {
	fAt := ev.FailedAt()
	if fAt == nil {
		return domain.ErrFailedAtRequired
	}
	reason := ev.FailureReason().String()
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	return r.q.MarkOgpFailed(ctx, sqlcgen.MarkOgpFailedParams{
		ID:            pgtype.UUID{Bytes: ev.ID(), Valid: true},
		FailedAt:      pgtype.Timestamptz{Time: *fAt, Valid: true},
		FailureReason: reasonPtr,
	})
}

// MarkStale は status='stale' + version++ に遷移させる。
func (r *OgpRepository) MarkStale(ctx context.Context, id uuid.UUID, now func() pgtype.Timestamptz) error {
	return r.q.MarkOgpStale(ctx, sqlcgen.MarkOgpStaleParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		UpdatedAt: now(),
	})
}

// CreateOgpImageAndVariant は OGP 完了化用に images + image_variants を 1 行ずつ
// INSERT する。呼び出し側 TX で MarkGenerated と組で実行することを想定。
//
// images 行は usage_kind='ogp' / status='available' / source_format='png' /
// normalized_format='jpg'（CHECK 制約に合わせる、実体は PNG）/ 1200×630 固定。
// image_variants 行は kind='ogp' / mime_type='image/png'。
func (r *OgpRepository) CreateOgpImageAndVariant(
	ctx context.Context,
	imageID uuid.UUID,
	photobookID uuid.UUID,
	variantID uuid.UUID,
	storageKey string,
	width int,
	height int,
	byteSize int64,
	now time.Time,
) error {
	if err := r.q.CreateOgpImageRecord(ctx, sqlcgen.CreateOgpImageRecordParams{
		ID:               pgtype.UUID{Bytes: imageID, Valid: true},
		OwnerPhotobookID: pgtype.UUID{Bytes: photobookID, Valid: true},
		OriginalWidth:    int32Ptr(int32(width)),
		OriginalHeight:   int32Ptr(int32(height)),
		OriginalByteSize: int64Ptr(byteSize),
		MetadataStrippedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		return err
	}
	return r.q.CreateOgpImageVariant(ctx, sqlcgen.CreateOgpImageVariantParams{
		ID:         pgtype.UUID{Bytes: variantID, Valid: true},
		ImageID:    pgtype.UUID{Bytes: imageID, Valid: true},
		StorageKey: storageKey,
		Width:      int32(width),
		Height:     int32(height),
		ByteSize:   byteSize,
		CreatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
	})
}

func int32Ptr(v int32) *int32 { return &v }
func int64Ptr(v int64) *int64 { return &v }

// OgpDelivery は public OGP lookup endpoint / Workers proxy が必要とする値。
// chat / log には storage_key 完全値を出さず、Workers binding 経由で R2 GET にのみ使う。
type OgpDelivery struct {
	OgpStatus           string
	OgpVersion          int
	PhotobookStatus     string
	PhotobookVisibility string
	HiddenByOperator    bool
	StorageKey          string // status != 'generated' の場合 ""
}

// GetDeliveryByPhotobookID は photobook_ogp_images + photobooks + image_variants
// を JOIN して Workers proxy が必要な情報を返す。
//
// 行が存在しない場合は ErrNotFound。
func (r *OgpRepository) GetDeliveryByPhotobookID(ctx context.Context, pid uuid.UUID) (OgpDelivery, error) {
	row, err := r.q.GetOgpDeliveryByPhotobookID(ctx, pgtype.UUID{Bytes: pid, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OgpDelivery{}, ErrNotFound
		}
		return OgpDelivery{}, err
	}
	out := OgpDelivery{
		OgpStatus:           row.OgpStatus,
		OgpVersion:          int(row.OgpVersion),
		PhotobookStatus:     row.PhotobookStatus,
		PhotobookVisibility: row.PhotobookVisibility,
		HiddenByOperator:    row.HiddenByOperator,
	}
	if row.OgpStorageKey != nil {
		out.StorageKey = *row.OgpStorageKey
	}
	return out, nil
}

// ListPending は pending / stale / failed の行を limit 件取り出す。
func (r *OgpRepository) ListPending(ctx context.Context, limit int) ([]domain.OgpImage, error) {
	rows, err := r.q.ListPendingOgp(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.OgpImage, 0, len(rows))
	for _, row := range rows {
		ev, err := toDomain(row)
		if err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, nil
}

// toDomain は sqlcgen 表現を domain.OgpImage に変換する。
func toDomain(row sqlcgen.PhotobookOgpImage) (domain.OgpImage, error) {
	pid, err := photobookid.FromUUID(row.PhotobookID.Bytes)
	if err != nil {
		return domain.OgpImage{}, err
	}
	st, err := ogp_status.Parse(row.Status)
	if err != nil {
		return domain.OgpImage{}, err
	}
	ver, err := ogp_version.New(int(row.Version))
	if err != nil {
		return domain.OgpImage{}, err
	}
	var imgIDPtr *imageid.ImageID
	if row.ImageID.Valid {
		v, err := imageid.FromUUID(row.ImageID.Bytes)
		if err != nil {
			return domain.OgpImage{}, err
		}
		imgIDPtr = &v
	}
	var genAt *time.Time
	if row.GeneratedAt.Valid {
		t := row.GeneratedAt.Time
		genAt = &t
	}
	var failAt *time.Time
	if row.FailedAt.Valid {
		t := row.FailedAt.Time
		failAt = &t
	}
	var reason ogp_failure_reason.OgpFailureReason
	if row.FailureReason != nil {
		reason, err = ogp_failure_reason.FromTrustedString(*row.FailureReason)
		if err != nil {
			return domain.OgpImage{}, err
		}
	}
	createdAt := row.CreatedAt.Time
	updatedAt := row.UpdatedAt.Time
	return domain.Restore(domain.RestoreParams{
		ID:            row.ID.Bytes,
		PhotobookID:   pid,
		Status:        st,
		ImageID:       imgIDPtr,
		Version:       ver,
		GeneratedAt:   genAt,
		FailedAt:      failAt,
		FailureReason: reason,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	})
}
