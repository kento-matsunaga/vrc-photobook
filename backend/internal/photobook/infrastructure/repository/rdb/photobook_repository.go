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

// SetHiddenByOperator は status='published' な photobook の hidden_by_operator を切り替える。
// 計画書 §5.6 / ユーザー判断 #5: version は上げない（編集 OCC を壊さない）。
//
// 戻り値:
//   - true: 1 行更新（状態が変わった）
//   - false: 0 行（status 違い or 既に target 状態。冪等扱い / 上位で分岐）
//   - error: DB エラー
//
// expectCurrentHidden を渡すことで、二重実行や状態誤認を防ぐ:
//   - hide 操作 → expectCurrentHidden=false, target=true
//   - unhide 操作 → expectCurrentHidden=true, target=false
func (r *PhotobookRepository) SetHiddenByOperator(
	ctx context.Context,
	id photobook_id.PhotobookID,
	target bool,
	expectCurrentHidden bool,
	now time.Time,
) (bool, error) {
	rows, err := r.q.SetPhotobookHiddenByOperator(ctx, sqlcgen.SetPhotobookHiddenByOperatorParams{
		ID:                 pgtype.UUID{Bytes: id.UUID(), Valid: true},
		HiddenByOperator:   target,
		UpdatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
		HiddenByOperator_2: expectCurrentHidden,
	})
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}

// OpsView は cmd/ops 表示用のスリムビュー（raw token / hash 系を含めない）。
type OpsView struct {
	ID                 photobook_id.PhotobookID
	Type               string
	Title              string
	Description        *string
	Layout             string
	OpeningStyle       string
	Visibility         string
	CreatorDisplayName string
	CreatorXID         *string
	CoverTitle         *string
	PublicURLSlug      *string
	Status             string
	HiddenByOperator   bool
	Version            int
	PublishedAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// GetForOps は cmd/ops show 用のスリムビューを返す。raw token / hash 系は出さない。
// 該当なしは ErrNotFound。
func (r *PhotobookRepository) GetForOps(ctx context.Context, id photobook_id.PhotobookID) (OpsView, error) {
	row, err := r.q.GetPhotobookForOps(ctx, pgtype.UUID{Bytes: id.UUID(), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OpsView{}, ErrNotFound
		}
		return OpsView{}, err
	}
	pid, err := photobook_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return OpsView{}, err
	}
	v := OpsView{
		ID:                 pid,
		Type:               row.Type,
		Title:              row.Title,
		Description:        row.Description,
		Layout:             row.Layout,
		OpeningStyle:       row.OpeningStyle,
		Visibility:         row.Visibility,
		CreatorDisplayName: row.CreatorDisplayName,
		CreatorXID:         row.CreatorXID,
		CoverTitle:         row.CoverTitle,
		PublicURLSlug:      row.PublicUrlSlug,
		Status:             row.Status,
		HiddenByOperator:   row.HiddenByOperator,
		Version:            int(row.Version),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}
	if row.PublishedAt.Valid {
		t := row.PublishedAt.Time
		v.PublishedAt = &t
	}
	return v, nil
}

// OpsHiddenSummary は list-hidden の 1 行分の要約。
type OpsHiddenSummary struct {
	ID                 photobook_id.PhotobookID
	PublicURLSlug      *string
	Title              string
	CreatorDisplayName string
	Visibility         string
	Status             string
	Version            int
	PublishedAt        *time.Time
	UpdatedAt          time.Time // hide 直近時刻の近似（status update 時刻）
}

// ListHiddenForOps は hidden_by_operator=true な photobook を最新更新順で返す。
func (r *PhotobookRepository) ListHiddenForOps(ctx context.Context, limit, offset int32) ([]OpsHiddenSummary, error) {
	rows, err := r.q.ListHiddenPhotobooksForOps(ctx, sqlcgen.ListHiddenPhotobooksForOpsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]OpsHiddenSummary, 0, len(rows))
	for _, row := range rows {
		pid, err := photobook_id.FromUUID(row.ID.Bytes)
		if err != nil {
			return nil, err
		}
		s := OpsHiddenSummary{
			ID:                 pid,
			PublicURLSlug:      row.PublicUrlSlug,
			Title:              row.Title,
			CreatorDisplayName: row.CreatorDisplayName,
			Visibility:         row.Visibility,
			Status:             row.Status,
			Version:            int(row.Version),
			UpdatedAt:          row.UpdatedAt.Time,
		}
		if row.PublishedAt.Valid {
			t := row.PublishedAt.Time
			s.PublishedAt = &t
		}
		out = append(out, s)
	}
	return out, nil
}
