// PR19: PhotobookRepository に追加する page / photo / cover 操作メソッド。
//
// 設計参照:
//   - docs/design/aggregates/photobook/データモデル設計.md
//   - docs/plan/m2-photobook-image-connection-plan.md §8.3
//
// セキュリティ:
//   - すべての photobooks UPDATE は WHERE version=$expected かつ status='draft'
//   - 0 行 UPDATE は ErrOptimisticLockConflict / ErrNotDraft に変換
//   - photobook_photos INSERT 前に Image の owner / status を FOR UPDATE で検証
package rdb

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/page_id"
	"vrcpb/backend/internal/photobook/domain/vo/photo_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb/sqlcgen"
)

// 追加のビジネス例外。
var (
	ErrImageNotAttachable = errors.New("image is not attachable (owner mismatch / not available / deleted)")
	ErrNotDraft           = errors.New("photobook is not in draft state")
	ErrPageNotFound       = errors.New("page not found")
	ErrPhotoNotFound      = errors.New("photo not found")
)

// === Page operations ===

// AddPage は draft Photobook の version+1 と新 Page INSERT を実行する。
//
// version 不一致 / status!=draft の場合 ErrOptimisticLockConflict を返す。
func (r *PhotobookRepository) AddPage(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	page domain.Page,
	expectedVersion int,
	now time.Time,
) error {
	if !page.PhotobookID().Equal(photobookID) {
		return errors.New("AddPage: page.photobook_id mismatch")
	}
	if err := r.bumpVersion(ctx, photobookID, expectedVersion, now); err != nil {
		return err
	}
	return r.q.CreatePhotobookPage(ctx, sqlcgen.CreatePhotobookPageParams{
		ID:           pgtype.UUID{Bytes: page.ID().UUID(), Valid: true},
		PhotobookID:  pgtype.UUID{Bytes: photobookID.UUID(), Valid: true},
		DisplayOrder: int32(page.DisplayOrder().Int()),
		Caption:      captionPtr(page.Caption()),
		CreatedAt:    pgtype.Timestamptz{Time: page.CreatedAt(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: page.UpdatedAt(), Valid: true},
	})
}

// RemovePage は draft Photobook の version+1 と Page DELETE（CASCADE で photos / metas）。
func (r *PhotobookRepository) RemovePage(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	id page_id.PageID,
	expectedVersion int,
	now time.Time,
) error {
	if err := r.bumpVersion(ctx, photobookID, expectedVersion, now); err != nil {
		return err
	}
	rows, err := r.q.DeletePhotobookPage(ctx, sqlcgen.DeletePhotobookPageParams{
		ID:          pgtype.UUID{Bytes: id.UUID(), Valid: true},
		PhotobookID: pgtype.UUID{Bytes: photobookID.UUID(), Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrPageNotFound
	}
	return nil
}

// ListPagesByPhotobookID は display_order ASC 順で Page 一覧を返す。
func (r *PhotobookRepository) ListPagesByPhotobookID(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
) ([]domain.Page, error) {
	rows, err := r.q.ListPhotobookPagesByPhotobookID(ctx,
		pgtype.UUID{Bytes: photobookID.UUID(), Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Page, 0, len(rows))
	for _, row := range rows {
		p, err := pageFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// CountPagesByPhotobookID は Page 総数を返す。
func (r *PhotobookRepository) CountPagesByPhotobookID(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
) (int, error) {
	n, err := r.q.CountPhotobookPagesByPhotobookID(ctx,
		pgtype.UUID{Bytes: photobookID.UUID(), Valid: true})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// === Photo operations ===

// AddPhoto は version+1 / Image owner+status 検証 / Photo INSERT を実行する。
//
// 同 TX 内で次を実施:
//  1. photobooks の version+1 + status=draft 検証
//  2. images の owner+status FOR UPDATE 検証
//  3. photobook_photos INSERT
//
// owner / status 検証で 0 行なら ErrImageNotAttachable。
func (r *PhotobookRepository) AddPhoto(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	pageID page_id.PageID,
	photo domain.Photo,
	expectedVersion int,
	now time.Time,
) error {
	if !photo.PageID().Equal(pageID) {
		return errors.New("AddPhoto: photo.page_id mismatch")
	}
	if err := r.bumpVersion(ctx, photobookID, expectedVersion, now); err != nil {
		return err
	}
	// Image owner + status 検証
	_, err := r.q.FindAvailableImageForPhotobook(ctx, sqlcgen.FindAvailableImageForPhotobookParams{
		ID:               pgtype.UUID{Bytes: photo.ImageID().UUID(), Valid: true},
		OwnerPhotobookID: pgtype.UUID{Bytes: photobookID.UUID(), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrImageNotAttachable
		}
		return err
	}
	return r.q.CreatePhotobookPhoto(ctx, sqlcgen.CreatePhotobookPhotoParams{
		ID:           pgtype.UUID{Bytes: photo.ID().UUID(), Valid: true},
		PageID:       pgtype.UUID{Bytes: pageID.UUID(), Valid: true},
		ImageID:      pgtype.UUID{Bytes: photo.ImageID().UUID(), Valid: true},
		DisplayOrder: int32(photo.DisplayOrder().Int()),
		Caption:      captionPtr(photo.Caption()),
		CreatedAt:    pgtype.Timestamptz{Time: photo.CreatedAt(), Valid: true},
	})
}

// RemovePhoto は version+1 / Photo DELETE。
func (r *PhotobookRepository) RemovePhoto(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	pageID page_id.PageID,
	id photo_id.PhotoID,
	expectedVersion int,
	now time.Time,
) error {
	if err := r.bumpVersion(ctx, photobookID, expectedVersion, now); err != nil {
		return err
	}
	rows, err := r.q.DeletePhotobookPhoto(ctx, sqlcgen.DeletePhotobookPhotoParams{
		ID:     pgtype.UUID{Bytes: id.UUID(), Valid: true},
		PageID: pgtype.UUID{Bytes: pageID.UUID(), Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrPhotoNotFound
	}
	return nil
}

// ReorderPhoto は Photo の display_order を更新する。version+1 も同時に行う。
func (r *PhotobookRepository) ReorderPhoto(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	id photo_id.PhotoID,
	newOrder display_order.DisplayOrder,
	expectedVersion int,
	now time.Time,
) error {
	if err := r.bumpVersion(ctx, photobookID, expectedVersion, now); err != nil {
		return err
	}
	rows, err := r.q.UpdatePhotobookPhotoOrder(ctx, sqlcgen.UpdatePhotobookPhotoOrderParams{
		ID:           pgtype.UUID{Bytes: id.UUID(), Valid: true},
		DisplayOrder: int32(newOrder.Int()),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrPhotoNotFound
	}
	return nil
}

// ListPhotosByPageID は display_order ASC で Photo 一覧を返す。
func (r *PhotobookRepository) ListPhotosByPageID(
	ctx context.Context,
	pageID page_id.PageID,
) ([]domain.Photo, error) {
	rows, err := r.q.ListPhotobookPhotosByPageID(ctx,
		pgtype.UUID{Bytes: pageID.UUID(), Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Photo, 0, len(rows))
	for _, row := range rows {
		ph, err := photoFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, ph)
	}
	return out, nil
}

// CountPhotosByPageID は Page 内 Photo 総数を返す。
func (r *PhotobookRepository) CountPhotosByPageID(
	ctx context.Context,
	pageID page_id.PageID,
) (int, error) {
	n, err := r.q.CountPhotobookPhotosByPageID(ctx,
		pgtype.UUID{Bytes: pageID.UUID(), Valid: true})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// === Cover operations ===

// SetCoverImage は cover_image_id を更新する。Image owner+status を同 TX 内で検証する。
func (r *PhotobookRepository) SetCoverImage(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	imgID image_id.ImageID,
	expectedVersion int,
	now time.Time,
) error {
	// Image owner + status 検証
	if _, err := r.q.FindAvailableImageForPhotobook(ctx, sqlcgen.FindAvailableImageForPhotobookParams{
		ID:               pgtype.UUID{Bytes: imgID.UUID(), Valid: true},
		OwnerPhotobookID: pgtype.UUID{Bytes: photobookID.UUID(), Valid: true},
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrImageNotAttachable
		}
		return err
	}
	rows, err := r.q.SetPhotobookCoverImage(ctx, sqlcgen.SetPhotobookCoverImageParams{
		ID:           pgtype.UUID{Bytes: photobookID.UUID(), Valid: true},
		CoverImageID: pgtype.UUID{Bytes: imgID.UUID(), Valid: true},
		Version:      int32(expectedVersion),
		UpdatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOptimisticLockConflict
	}
	return nil
}

// ClearCoverImage は cover_image_id を NULL にする（draft + version 一致のとき）。
func (r *PhotobookRepository) ClearCoverImage(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	expectedVersion int,
	now time.Time,
) error {
	rows, err := r.q.ClearPhotobookCoverImage(ctx, sqlcgen.ClearPhotobookCoverImageParams{
		ID:        pgtype.UUID{Bytes: photobookID.UUID(), Valid: true},
		Version:   int32(expectedVersion),
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOptimisticLockConflict
	}
	return nil
}

// === PageMeta operations ===

// UpsertPageMeta は photobook_page_metas を upsert する。version は変更しない
// （※ 必要なら呼び出し側で BumpVersion を別途呼ぶ）。
func (r *PhotobookRepository) UpsertPageMeta(
	ctx context.Context,
	meta domain.PageMeta,
) error {
	cast := meta.Cast()
	var castParam []string
	if len(cast) > 0 {
		castParam = cast
	}
	return r.q.UpsertPhotobookPageMeta(ctx, sqlcgen.UpsertPhotobookPageMetaParams{
		PageID:       pgtype.UUID{Bytes: meta.PageID().UUID(), Valid: true},
		World:        meta.World(),
		CastList:     castParam,
		Photographer: meta.Photographer(),
		Note:         meta.Note(),
		EventDate:    timePtrToPgDate(meta.EventDate()),
		CreatedAt:    pgtype.Timestamptz{Time: meta.CreatedAt(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: meta.UpdatedAt(), Valid: true},
	})
}

// FindPageMetaByPageID は PageMeta を取得する。
func (r *PhotobookRepository) FindPageMetaByPageID(
	ctx context.Context,
	pageID page_id.PageID,
) (domain.PageMeta, error) {
	row, err := r.q.FindPhotobookPageMetaByPageID(ctx,
		pgtype.UUID{Bytes: pageID.UUID(), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PageMeta{}, ErrNotFound
		}
		return domain.PageMeta{}, err
	}
	return pageMetaFromRow(row), nil
}

// === helpers ===

// bumpVersion は photobooks の version+1 + status=draft 検証を実行する。
//
// 0 行 UPDATE は ErrOptimisticLockConflict（draft 以外 / version 不一致 を区別しない）。
func (r *PhotobookRepository) bumpVersion(
	ctx context.Context,
	id photobook_id.PhotobookID,
	expectedVersion int,
	now time.Time,
) error {
	rows, err := r.q.BumpPhotobookVersionForDraft(ctx, sqlcgen.BumpPhotobookVersionForDraftParams{
		ID:        pgtype.UUID{Bytes: id.UUID(), Valid: true},
		Version:   int32(expectedVersion),
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOptimisticLockConflict
	}
	return nil
}

func captionPtr(c *caption.Caption) *string {
	if c == nil {
		return nil
	}
	s := c.String()
	return &s
}

func pageFromRow(row sqlcgen.PhotobookPage) (domain.Page, error) {
	pid, err := page_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return domain.Page{}, err
	}
	pbid, err := photobook_id.FromUUID(row.PhotobookID.Bytes)
	if err != nil {
		return domain.Page{}, err
	}
	order, err := display_order.New(int(row.DisplayOrder))
	if err != nil {
		return domain.Page{}, err
	}
	var capPtr *caption.Caption
	if row.Caption != nil {
		c, err := caption.New(*row.Caption)
		if err != nil {
			return domain.Page{}, err
		}
		capPtr = &c
	}
	return domain.RestorePage(domain.RestorePageParams{
		ID:           pid,
		PhotobookID:  pbid,
		DisplayOrder: order,
		Caption:      capPtr,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}), nil
}

func photoFromRow(row sqlcgen.PhotobookPhoto) (domain.Photo, error) {
	id, err := photo_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return domain.Photo{}, err
	}
	pid, err := page_id.FromUUID(row.PageID.Bytes)
	if err != nil {
		return domain.Photo{}, err
	}
	iid, err := image_id.FromUUID(row.ImageID.Bytes)
	if err != nil {
		return domain.Photo{}, err
	}
	order, err := display_order.New(int(row.DisplayOrder))
	if err != nil {
		return domain.Photo{}, err
	}
	var capPtr *caption.Caption
	if row.Caption != nil {
		c, err := caption.New(*row.Caption)
		if err != nil {
			return domain.Photo{}, err
		}
		capPtr = &c
	}
	return domain.RestorePhoto(domain.RestorePhotoParams{
		ID:           id,
		PageID:       pid,
		ImageID:      iid,
		DisplayOrder: order,
		Caption:      capPtr,
		CreatedAt:    row.CreatedAt.Time,
	}), nil
}

func pageMetaFromRow(row sqlcgen.PhotobookPageMeta) domain.PageMeta {
	pid, _ := page_id.FromUUID(row.PageID.Bytes)
	var eventDate *time.Time
	if row.EventDate.Valid {
		t := row.EventDate.Time
		eventDate = &t
	}
	return domain.RestorePageMeta(domain.RestorePageMetaParams{
		PageID:       pid,
		World:        row.World,
		Cast:         row.CastList,
		Photographer: row.Photographer,
		Note:         row.Note,
		EventDate:    eventDate,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	})
}

func timePtrToPgDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{Valid: false}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// suppress unused import warning（image_id は exported 型として使用されているが、
// import 整列時に保護するため）
var _ = uuid.Nil
