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

// === bulk attach helpers (m2-prepare-resilience-and-throughput, plan v2 §3.4 / §5) ===
//
// AttachAvailableImages usecase が 1 TX 内で N image を bulk attach する際に使う 4 method。
// 既存 AddPage / AddPhoto は内部で version bump を都度実行するため、bulk loop 内で呼ぶと
// 複数回 bump されてしまう（atomicity は保てるが意味的に奇妙）。本 helper 群は版を bump
// せず INSERT のみに専念し、loop 終了後に BumpVersion を **1 度だけ**呼ぶ設計を可能にする。
//
// すべて DBTX (pgx.Tx 経由) で呼び出す前提。pool 直で呼ぶ用途は無い。

// ListAvailableUnattachedImageIDs は photobook の available + 未 attach な image_id を返す。
//
// SQL は queries/photobook_pages.sql#ListAvailableUnattachedImageIDsByPhotobook 参照。
// status='available' 以外（uploading / processing / failed / deleted / purged）は除外され、
// photobook_photos に既に attach 済の image も NOT EXISTS で除外される。
func (r *PhotobookRepository) ListAvailableUnattachedImageIDs(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
) ([]image_id.ImageID, error) {
	rows, err := r.q.ListAvailableUnattachedImageIDsByPhotobook(ctx,
		pgtype.UUID{Bytes: photobookID.UUID(), Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]image_id.ImageID, 0, len(rows))
	for _, row := range rows {
		id, err := image_id.FromUUID(row.Bytes)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// CreatePageInTx は photobook_pages INSERT のみを実行する（version bump せず）。
//
// 呼び出し側の TX 内で N 回呼んだ後、最後に BumpVersion を 1 度だけ呼ぶ設計。
// 単発 page 作成には既存 AddPage を使うこと（version bump 込み）。
func (r *PhotobookRepository) CreatePageInTx(
	ctx context.Context,
	page domain.Page,
) error {
	return r.q.CreatePhotobookPage(ctx, sqlcgen.CreatePhotobookPageParams{
		ID:           pgtype.UUID{Bytes: page.ID().UUID(), Valid: true},
		PhotobookID:  pgtype.UUID{Bytes: page.PhotobookID().UUID(), Valid: true},
		DisplayOrder: int32(page.DisplayOrder().Int()),
		Caption:      captionPtr(page.Caption()),
		CreatedAt:    pgtype.Timestamptz{Time: page.CreatedAt(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: page.UpdatedAt(), Valid: true},
	})
}

// CreatePhotoInTx は photobook_photos INSERT のみを実行する（version bump せず）。
//
// 呼び出し側で image owner / status を保証している前提（AttachAvailableImages では
// ListAvailableUnattachedImageIDs で available + owner=photobook を SQL で保証済み）。
// 単発 photo attach には既存 AddPhoto を使うこと（owner+status FOR UPDATE 検証 + version
// bump 込み）。
func (r *PhotobookRepository) CreatePhotoInTx(
	ctx context.Context,
	photo domain.Photo,
) error {
	return r.q.CreatePhotobookPhoto(ctx, sqlcgen.CreatePhotobookPhotoParams{
		ID:           pgtype.UUID{Bytes: photo.ID().UUID(), Valid: true},
		PageID:       pgtype.UUID{Bytes: photo.PageID().UUID(), Valid: true},
		ImageID:      pgtype.UUID{Bytes: photo.ImageID().UUID(), Valid: true},
		DisplayOrder: int32(photo.DisplayOrder().Int()),
		Caption:      captionPtr(photo.Caption()),
		CreatedAt:    pgtype.Timestamptz{Time: photo.CreatedAt(), Valid: true},
	})
}

// BumpVersion は photobooks の version+1 + status=draft 検証を実行する（public）。
//
// AttachAvailableImages usecase の TX 末尾で 1 度だけ呼ぶ用途。挙動は private bumpVersion
// と同一（0 行 UPDATE → ErrOptimisticLockConflict）。
func (r *PhotobookRepository) BumpVersion(
	ctx context.Context,
	id photobook_id.PhotobookID,
	expectedVersion int,
	now time.Time,
) error {
	return r.bumpVersion(ctx, id, expectedVersion, now)
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

// === PR27 新規 ===

// UpdatePhotoCaption は Photo の caption を単独で更新する（version+1 と同 TX）。
//
// caption が nil なら NULL を保存する。0 行 UPDATE は ErrPhotoNotFound。
func (r *PhotobookRepository) UpdatePhotoCaption(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	id photo_id.PhotoID,
	c *caption.Caption,
	expectedVersion int,
	now time.Time,
) error {
	if err := r.bumpVersion(ctx, photobookID, expectedVersion, now); err != nil {
		return err
	}
	rows, err := r.q.UpdatePhotobookPhotoCaption(ctx, sqlcgen.UpdatePhotobookPhotoCaptionParams{
		ID:      pgtype.UUID{Bytes: id.UUID(), Valid: true},
		Caption: captionPtr(c),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrPhotoNotFound
	}
	return nil
}

// PhotoOrderAssignment は BulkReorderPhotosOnPage の引数の 1 件分。
type PhotoOrderAssignment struct {
	PhotoID  photo_id.PhotoID
	NewOrder display_order.DisplayOrder
}

// BulkReorderPhotosOnPage は同 page 内の photo 群の display_order を一括で再配置する。
//
// アルゴリズム（PR27 計画書 §5.4 方式 A）:
//  1. photobooks.version+1（status=draft AND version=$expected で OCC）
//  2. 同 page の全 photo の display_order を +1000 して一時退避（UNIQUE 衝突回避）
//  3. assignments の各 (photo_id, new_order) を順次 UPDATE
//  4. すべて同 TX 内で実行
//
// 0 行 / 個数不一致は ErrPhotoNotFound に集約。
func (r *PhotobookRepository) BulkReorderPhotosOnPage(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	pageID page_id.PageID,
	assignments []PhotoOrderAssignment,
	expectedVersion int,
	now time.Time,
) error {
	if err := r.bumpVersion(ctx, photobookID, expectedVersion, now); err != nil {
		return err
	}
	if _, err := r.q.BulkOffsetPhotoOrdersOnPage(ctx,
		pgtype.UUID{Bytes: pageID.UUID(), Valid: true}); err != nil {
		return err
	}
	for _, a := range assignments {
		rows, err := r.q.UpdatePhotobookPhotoOrder(ctx, sqlcgen.UpdatePhotobookPhotoOrderParams{
			ID:           pgtype.UUID{Bytes: a.PhotoID.UUID(), Valid: true},
			DisplayOrder: int32(a.NewOrder.Int()),
		})
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrPhotoNotFound
		}
	}
	return nil
}

// PhotobookSettings は UpdateSettings の入力値（VO 化済み）。
type PhotobookSettings struct {
	Type         string
	Title        string
	Description  *string
	Layout       string
	OpeningStyle string
	Visibility   string
	CoverTitle   *string
}

// UpdateSettings は draft Photobook の settings 一括 PATCH。
//
// 0 行 UPDATE は ErrOptimisticLockConflict（draft 以外 / version 不一致 を区別しない）。
func (r *PhotobookRepository) UpdateSettings(
	ctx context.Context,
	id photobook_id.PhotobookID,
	s PhotobookSettings,
	expectedVersion int,
	now time.Time,
) error {
	rows, err := r.q.UpdatePhotobookSettings(ctx, sqlcgen.UpdatePhotobookSettingsParams{
		ID:           pgtype.UUID{Bytes: id.UUID(), Valid: true},
		Type:         s.Type,
		Title:        s.Title,
		Description:  s.Description,
		Layout:       s.Layout,
		OpeningStyle: s.OpeningStyle,
		Visibility:   s.Visibility,
		CoverTitle:   s.CoverTitle,
		UpdatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		Version:      int32(expectedVersion),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOptimisticLockConflict
	}
	return nil
}

// ============================================================================
// STOP P-1: page split / merge / move primitives (m2-edit Phase A)
// ----------------------------------------------------------------------------
// 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §2.4 / §11
// 方針: 薄い primitive。draft check / version+1 / 30 page 上限 / reason mapping は
//       UseCase 層 (STOP P-2) で扱う。本層は SQL 単発 + 必要最低限の ownership 検証のみ。
//       既存 BulkReorderPhotosOnPage / UpdatePhotoCaption が内部で bumpVersion を
//       呼ぶのと違い、本 group は **bumpVersion を呼ばない** (UseCase が 1 TX 内で
//       一度だけ呼ぶ)。
// ============================================================================

// UpdatePageCaption は page の caption を更新する (薄い primitive、bumpVersion なし)。
//
// caption が nil なら NULL を保存する。
// WHERE 句で photobook_id 所属を SQL レベルで検証 → 別 photobook の page は更新されない。
// 0 行 UPDATE は ErrPageNotFound。
//
// UseCase 責務:
//   - bumpVersion (OCC + draft check)
//   - caption length validation (domain.NewCaption に委譲)
//   - photobook_id と pageID の整合
func (r *PhotobookRepository) UpdatePageCaption(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	pageID page_id.PageID,
	c *caption.Caption,
	now time.Time,
) error {
	rows, err := r.q.UpdatePhotobookPageCaption(ctx, sqlcgen.UpdatePhotobookPageCaptionParams{
		ID:          pgtype.UUID{Bytes: pageID.UUID(), Valid: true},
		Caption:     captionPtr(c),
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
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

// BulkOffsetPagesInPhotobook は photobook 内全 page の display_order を +1000 する
// (page reorder / split / merge の escape primitive、bumpVersion なし)。
//
// UNIQUE (photobook_id, display_order) との衝突を一時回避するため、UseCase は同 TX 内で
// 各 page を新しい display_order に書き戻す (UpdatePhotobookPageOrder を順次呼ぶ)。
//
// 1 page も無い photobook での呼出は no-op (rows=0、エラーにしない)。
func (r *PhotobookRepository) BulkOffsetPagesInPhotobook(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	now time.Time,
) error {
	_, err := r.q.BulkOffsetPagesInPhotobook(ctx, sqlcgen.BulkOffsetPagesInPhotobookParams{
		PhotobookID: pgtype.UUID{Bytes: photobookID.UUID(), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	})
	return err
}

// UpdatePhotoPageAndOrder は photo を別 page (or 同 page) の指定 display_order に移動する
// (薄い primitive、bumpVersion なし)。
//
// ownership 検証 (photo + target page が photobookID 配下か) を Repository 内で実施する。
// SQL UPDATE 自体は単純な WHERE id=$1 のため、UseCase 側で escape (BulkOffsetPhotoOrders
// OnPage) を行ってから本 method を呼ぶこと (target page で UNIQUE 衝突しない位置を空ける)。
//
// 失敗パターン:
//   - photo_id 不存在 → ErrPhotoNotFound
//   - photo の現 page が photobookID 配下でない → ErrPhotoNotFound (ownership は外に出さない)
//   - target_page_id 不存在 / photobookID 配下でない → ErrPageNotFound
//   - SQL UPDATE で UNIQUE 衝突 → 23505 (pg error をそのまま伝播、UseCase で escape を
//     入れていないと発生する programmer error)
func (r *PhotobookRepository) UpdatePhotoPageAndOrder(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	photoID photo_id.PhotoID,
	targetPageID page_id.PageID,
	newOrder display_order.DisplayOrder,
) error {
	// 1. photo の現状取得 + 所属 photobook 検証
	photoRow, err := r.q.FindPhotobookPhotoWithPhotobookID(ctx,
		pgtype.UUID{Bytes: photoID.UUID(), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPhotoNotFound
		}
		return err
	}
	if photoRow.PhotobookID.Bytes != photobookID.UUID() {
		return ErrPhotoNotFound
	}

	// 2. target page の所属 photobook 検証
	pageRow, err := r.q.FindPhotobookPageByID(ctx,
		pgtype.UUID{Bytes: targetPageID.UUID(), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPageNotFound
		}
		return err
	}
	if pageRow.PhotobookID.Bytes != photobookID.UUID() {
		return ErrPageNotFound
	}

	// 3. UPDATE (SQL は単純な WHERE id=$1、UNIQUE 衝突は UseCase の escape 責務)
	rows, err := r.q.UpdatePhotobookPhotoPageAndOrder(ctx, sqlcgen.UpdatePhotobookPhotoPageAndOrderParams{
		ID:           pgtype.UUID{Bytes: photoID.UUID(), Valid: true},
		PageID:       pgtype.UUID{Bytes: targetPageID.UUID(), Valid: true},
		DisplayOrder: int32(newOrder.Int()),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		// 1. の検証通過後に photo が消えた race。defensive。
		return ErrPhotoNotFound
	}
	return nil
}

// FindPhotoWithPhotobookID は photo を photo_id で取得し、所属 photobook_id とともに返す。
//
// move ロジック / split ロジックで「ある photo がどの page に属しているか + その page が
// どの photobook に属しているか」を 1 query で確認するために使う。
// row 不在は ErrPhotoNotFound。
func (r *PhotobookRepository) FindPhotoWithPhotobookID(
	ctx context.Context,
	photoID photo_id.PhotoID,
) (domain.Photo, photobook_id.PhotobookID, error) {
	row, err := r.q.FindPhotobookPhotoWithPhotobookID(ctx,
		pgtype.UUID{Bytes: photoID.UUID(), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Photo{}, photobook_id.PhotobookID{}, ErrPhotoNotFound
		}
		return domain.Photo{}, photobook_id.PhotobookID{}, err
	}
	pid, err := photo_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return domain.Photo{}, photobook_id.PhotobookID{}, err
	}
	pageID, err := page_id.FromUUID(row.PageID.Bytes)
	if err != nil {
		return domain.Photo{}, photobook_id.PhotobookID{}, err
	}
	imgID, err := image_id.FromUUID(row.ImageID.Bytes)
	if err != nil {
		return domain.Photo{}, photobook_id.PhotobookID{}, err
	}
	order, err := display_order.New(int(row.DisplayOrder))
	if err != nil {
		return domain.Photo{}, photobook_id.PhotobookID{}, err
	}
	var capPtr *caption.Caption
	if row.Caption != nil {
		c, err := caption.New(*row.Caption)
		if err != nil {
			return domain.Photo{}, photobook_id.PhotobookID{}, err
		}
		capPtr = &c
	}
	pbid, err := photobook_id.FromUUID(row.PhotobookID.Bytes)
	if err != nil {
		return domain.Photo{}, photobook_id.PhotobookID{}, err
	}
	photo := domain.RestorePhoto(domain.RestorePhotoParams{
		ID:           pid,
		PageID:       pageID,
		ImageID:      imgID,
		DisplayOrder: order,
		Caption:      capPtr,
		CreatedAt:    row.CreatedAt.Time,
	})
	return photo, pbid, nil
}
