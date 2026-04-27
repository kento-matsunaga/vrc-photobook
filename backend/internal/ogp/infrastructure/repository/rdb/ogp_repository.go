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
