// Package rdb は Photobook 集約の RDB Repository を提供する。
//
// 設計参照:
//   - docs/design/aggregates/photobook/データモデル設計.md
//   - docs/plan/m2-photobook-session-integration-plan.md §8
//
// セキュリティ:
//   - すべての UPDATE は version = $expectedVersion を含む（楽観ロック、I-V1〜V3）
//   - 0 行 UPDATE は ErrOptimisticLockConflict として返す
//   - raw SQL を直接実行できる経路は本パッケージ外には公開しない
package rdb

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb/marshaller"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb/sqlcgen"
)

// ビジネス例外。
var (
	ErrNotFound               = errors.New("photobook not found")
	ErrOptimisticLockConflict = errors.New("photobook optimistic lock conflict")
)

// PhotobookRepository は photobooks テーブルへの永続化を提供する。
type PhotobookRepository struct {
	q *sqlcgen.Queries
}

// NewPhotobookRepository は pgx pool または tx（sqlcgen.DBTX を満たすもの）から Repository を作る。
func NewPhotobookRepository(db sqlcgen.DBTX) *PhotobookRepository {
	return &PhotobookRepository{q: sqlcgen.New(db)}
}

// CreateDraft は draft Photobook を INSERT する。
func (r *PhotobookRepository) CreateDraft(ctx context.Context, p domain.Photobook) error {
	params, err := marshaller.ToCreateParams(p)
	if err != nil {
		return err
	}
	return r.q.CreateDraftPhotobook(ctx, params)
}

// FindByID は id 一致の Photobook を返す。該当なし時は ErrNotFound。
func (r *PhotobookRepository) FindByID(ctx context.Context, id photobook_id.PhotobookID) (domain.Photobook, error) {
	row, err := r.q.FindPhotobookByID(ctx, pgtype.UUID{Bytes: id.UUID(), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Photobook{}, ErrNotFound
		}
		return domain.Photobook{}, err
	}
	return marshaller.FromRow(row)
}

// FindByDraftEditTokenHash は status=draft かつ未期限切れの draft を返す。
//
// SQL 自体が `draft_expires_at > now()` を含むため、期限切れは ErrNotFound。
func (r *PhotobookRepository) FindByDraftEditTokenHash(
	ctx context.Context,
	hash draft_edit_token_hash.DraftEditTokenHash,
) (domain.Photobook, error) {
	row, err := r.q.FindPhotobookByDraftEditTokenHash(ctx, hash.Bytes())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Photobook{}, ErrNotFound
		}
		return domain.Photobook{}, err
	}
	return marshaller.FromRow(row)
}

// FindByManageUrlTokenHash は status IN (published, deleted) の Photobook を返す。
func (r *PhotobookRepository) FindByManageUrlTokenHash(
	ctx context.Context,
	hash manage_url_token_hash.ManageUrlTokenHash,
) (domain.Photobook, error) {
	row, err := r.q.FindPhotobookByManageUrlTokenHash(ctx, hash.Bytes())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Photobook{}, ErrNotFound
		}
		return domain.Photobook{}, err
	}
	return marshaller.FromRow(row)
}

// FindBySlug は status=published かつ hidden_by_operator=false の Photobook を返す。
func (r *PhotobookRepository) FindBySlug(ctx context.Context, s slug.Slug) (domain.Photobook, error) {
	val := s.String()
	row, err := r.q.FindPhotobookBySlug(ctx, &val)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Photobook{}, ErrNotFound
		}
		return domain.Photobook{}, err
	}
	return marshaller.FromRow(row)
}

// FindAnyBySlug は status / hidden_by_operator を問わず slug 一致の Photobook を返す。
//
// PR25a 公開 Viewer の usecase が 200 / 410 / 404 を分岐するために使う
// （plan §3.2 / m2-public-viewer-and-manage-plan.md）。
// 0 行は ErrNotFound を返す。
func (r *PhotobookRepository) FindAnyBySlug(ctx context.Context, s slug.Slug) (domain.Photobook, error) {
	val := s.String()
	row, err := r.q.FindPhotobookBySlugAny(ctx, &val)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Photobook{}, ErrNotFound
		}
		return domain.Photobook{}, err
	}
	return marshaller.FromRow(row)
}

// TouchDraft は draft_expires_at を newExpiresAt に延長する。
// 0 行 UPDATE（version 不一致 / status≠draft）は ErrOptimisticLockConflict。
func (r *PhotobookRepository) TouchDraft(
	ctx context.Context,
	id photobook_id.PhotobookID,
	newExpiresAt time.Time,
	expectedVersion int,
) error {
	rows, err := r.q.TouchDraftPhotobook(ctx, sqlcgen.TouchDraftPhotobookParams{
		ID:             pgtype.UUID{Bytes: id.UUID(), Valid: true},
		DraftExpiresAt: pgtype.Timestamptz{Time: newExpiresAt, Valid: true},
		Version:        int32(expectedVersion),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOptimisticLockConflict
	}
	return nil
}

// PublishFromDraft は draft → published を実行する UPDATE。
//
// SQL は status='draft' AND version=$expectedVersion を要求する。
// 0 行 UPDATE は ErrOptimisticLockConflict（draft でない / version 不一致）。
func (r *PhotobookRepository) PublishFromDraft(
	ctx context.Context,
	id photobook_id.PhotobookID,
	publicSlug slug.Slug,
	manageHash manage_url_token_hash.ManageUrlTokenHash,
	publishedAt time.Time,
	expectedVersion int,
) error {
	slugStr := publicSlug.String()
	rows, err := r.q.PublishPhotobookFromDraft(ctx, sqlcgen.PublishPhotobookFromDraftParams{
		ID:                 pgtype.UUID{Bytes: id.UUID(), Valid: true},
		PublicUrlSlug:      &slugStr,
		ManageUrlTokenHash: manageHash.Bytes(),
		PublishedAt:        pgtype.Timestamptz{Time: publishedAt, Valid: true},
		Version:            int32(expectedVersion),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOptimisticLockConflict
	}
	return nil
}

// ReissueManageUrl は manage_url_token を新しい hash に置換する UPDATE。
//
// SQL は status IN ('published','deleted') AND version=$expectedVersion を要求する。
// 0 行 UPDATE は ErrOptimisticLockConflict。
func (r *PhotobookRepository) ReissueManageUrl(
	ctx context.Context,
	id photobook_id.PhotobookID,
	newManageHash manage_url_token_hash.ManageUrlTokenHash,
	expectedVersion int,
) error {
	rows, err := r.q.ReissuePhotobookManageUrl(ctx, sqlcgen.ReissuePhotobookManageUrlParams{
		ID:                 pgtype.UUID{Bytes: id.UUID(), Valid: true},
		ManageUrlTokenHash: newManageHash.Bytes(),
		Version:            int32(expectedVersion),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOptimisticLockConflict
	}
	return nil
}
