-- PR19: Photobook の Page / Photo / PageMeta / Cover に対する sqlc query 群。
--
-- 設計参照:
--   - docs/design/aggregates/photobook/データモデル設計.md
--   - docs/plan/m2-photobook-image-connection-plan.md §8.2
--
-- セキュリティ:
--   - photobook UPDATE はすべて WHERE version=$expected かつ status='draft' を持ち、
--     0 行 UPDATE をアプリ層で OptimisticLockConflict / ErrNotDraft として扱う

-- name: CreatePhotobookPage :exec
INSERT INTO photobook_pages (
    id, photobook_id, display_order, caption, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6
);

-- name: ListPhotobookPagesByPhotobookID :many
SELECT id, photobook_id, display_order, caption, created_at, updated_at
FROM photobook_pages
WHERE photobook_id = $1
ORDER BY display_order ASC;

-- name: FindPhotobookPageByID :one
SELECT id, photobook_id, display_order, caption, created_at, updated_at
FROM photobook_pages
WHERE id = $1;

-- name: CountPhotobookPagesByPhotobookID :one
SELECT COUNT(*)::int AS cnt
FROM photobook_pages
WHERE photobook_id = $1;

-- name: DeletePhotobookPage :execrows
DELETE FROM photobook_pages
WHERE id = $1
  AND photobook_id = $2;

-- name: UpdatePhotobookPageOrder :execrows
UPDATE photobook_pages
   SET display_order = $2,
       updated_at    = $3
 WHERE id = $1;

-- name: CreatePhotobookPhoto :exec
INSERT INTO photobook_photos (
    id, page_id, image_id, display_order, caption, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6
);

-- name: ListPhotobookPhotosByPageID :many
SELECT id, page_id, image_id, display_order, caption, created_at
FROM photobook_photos
WHERE page_id = $1
ORDER BY display_order ASC;

-- name: FindPhotobookPhotoByID :one
SELECT id, page_id, image_id, display_order, caption, created_at
FROM photobook_photos
WHERE id = $1;

-- name: CountPhotobookPhotosByPageID :one
SELECT COUNT(*)::int AS cnt
FROM photobook_photos
WHERE page_id = $1;

-- name: DeletePhotobookPhoto :execrows
DELETE FROM photobook_photos
WHERE id = $1
  AND page_id = $2;

-- name: UpdatePhotobookPhotoOrder :execrows
-- 注意: 単純な単一行 UPDATE。UNIQUE (page_id, display_order) と衝突する
-- new_order が既に他 photo に取られていると 23505 を返す。
-- MVP の Reorder は「新規 order が空いている」前提で運用する。
-- 二者間 swap や complex reorder は呼び出し側で：
--   1. 一時退避（例: 大きな offset 値 1000+ に一旦逃がす、CHECK display_order >= 0 のため）
--   2. 順次 UPDATE
-- のパターンを実装する。DEFERRABLE UNIQUE は MVP では採用しない（PR19 計画 / Audit）。
UPDATE photobook_photos
   SET display_order = $2
 WHERE id = $1;

-- BulkOffsetPhotoOrdersOnPage: PR27 reorder 一時退避用。
-- 同 page 内の全 photo の display_order を +1000 オフセットして UNIQUE 衝突を一時的に回避する。
-- 呼び出し側は同 TX 内で各 photo を新 display_order に書き戻す。
-- name: BulkOffsetPhotoOrdersOnPage :execrows
UPDATE photobook_photos
   SET display_order = display_order + 1000
 WHERE page_id = $1;

-- UpdatePhotobookPhotoCaption: PR27 photo caption 単独編集。
-- caption は VARCHAR/TEXT で NULL 許容（空 caption は NULL として保存）。
-- name: UpdatePhotobookPhotoCaption :execrows
UPDATE photobook_photos
   SET caption = $2
 WHERE id = $1;

-- name: UpsertPhotobookPageMeta :exec
INSERT INTO photobook_page_metas (
    page_id, world, cast_list, photographer, note, event_date, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (page_id) DO UPDATE
SET
    world        = EXCLUDED.world,
    cast_list    = EXCLUDED.cast_list,
    photographer = EXCLUDED.photographer,
    note         = EXCLUDED.note,
    event_date   = EXCLUDED.event_date,
    updated_at   = EXCLUDED.updated_at;

-- name: FindPhotobookPageMetaByPageID :one
SELECT page_id, world, cast_list, photographer, note, event_date, created_at, updated_at
FROM photobook_page_metas
WHERE page_id = $1;

-- name: SetPhotobookCoverImage :execrows
UPDATE photobooks
   SET cover_image_id = $2,
       version        = version + 1,
       updated_at     = $4
 WHERE id      = $1
   AND status  = 'draft'
   AND version = $3;

-- name: ClearPhotobookCoverImage :execrows
UPDATE photobooks
   SET cover_image_id = NULL,
       version        = version + 1,
       updated_at     = $3
 WHERE id      = $1
   AND status  = 'draft'
   AND version = $2;

-- name: BumpPhotobookVersionForDraft :execrows
UPDATE photobooks
   SET version    = version + 1,
       updated_at = $3
 WHERE id      = $1
   AND status  = 'draft'
   AND version = $2;

-- name: FindAvailableImageForPhotobook :one
-- Image が「対象 photobook 所有 + status=available + 未削除」を満たすかを返す。
-- 戻り値が無ければ ErrNoRows を呼び出し側で ErrImageNotAttachable に変換する。
SELECT id
FROM images
WHERE id                = $1
  AND owner_photobook_id = $2
  AND status            = 'available'
  AND deleted_at        IS NULL
FOR UPDATE;

-- name: ListAvailableUnattachedImageIDsByPhotobook :many
-- /prepare/attach-images（plan v2 §3.4 / §5）で「photobook の available + 未 attach な
-- image」を bulk 取得する。
-- ・status='available' のみ（uploading / processing / failed / deleted / purged は対象外）
-- ・既に photobook_photos に attach 済の image は NOT EXISTS で除外
--   （photobook_photos には photobook_id 直接 column が無いため photobook_pages 経由で JOIN）
-- ・並び順は uploaded_at ASC（attach 順序の決定論性を担保）
SELECT i.id
FROM images i
WHERE i.owner_photobook_id = $1
  AND i.status            = 'available'
  AND i.deleted_at        IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM photobook_photos pp
      JOIN photobook_pages pg ON pp.page_id = pg.id
      WHERE pg.photobook_id = $1
        AND pp.image_id     = i.id
  )
ORDER BY i.uploaded_at ASC;

-- ============================================================================
-- STOP P-1: page split / merge / move primitives (m2-edit Phase A)
-- ----------------------------------------------------------------------------
-- 計画: docs/plan/m2-edit-page-split-and-preview-plan.md §2.3 / §2.4
-- 方針: 薄い primitive に保つ。draft check / version+1 / 30 page 上限 / reason mapping
--       はすべて UseCase 層 (STOP P-2) で扱う。本 query は SQL レベルの ownership 検証
--       (page は photobook_id 直 column、photo は photobook_pages 経由 JOIN) のみ行う。
-- ============================================================================

-- UpdatePhotobookPageCaption: page caption の単独編集 (P-1)。
-- caption は VARCHAR/TEXT で NULL 許容。len validation は domain.NewCaption に委譲。
-- WHERE 句で photobook_id 所属を検証 → 別 photobook の page を誤更新しない。
-- 0 行 → ErrPageNotFound (Repository 層で変換)。
-- name: UpdatePhotobookPageCaption :execrows
UPDATE photobook_pages
   SET caption    = $2,
       updated_at = $3
 WHERE id           = $1
   AND photobook_id = $4;

-- BulkOffsetPagesInPhotobook: page reorder / split / merge の +1000 escape primitive (P-1)。
-- photo 版 BulkOffsetPhotoOrdersOnPage と同じ思想で、UNIQUE (photobook_id, display_order)
-- との衝突を一時的に回避するため、対象 photobook の **全 page** の display_order を +1000
-- する。呼び出し側 (UseCase) は同 TX 内で各 page を新しい display_order に書き戻す。
-- updated_at は呼び出し側で渡す ($now)。
-- name: BulkOffsetPagesInPhotobook :execrows
UPDATE photobook_pages
   SET display_order = display_order + 1000,
       updated_at    = $2
 WHERE photobook_id = $1;

-- UpdatePhotobookPhotoPageAndOrder: photo の page_id + display_order を同時更新 (P-1)。
-- 単純な単一行 UPDATE。UNIQUE (page_id, display_order) と衝突する new_order が target_page
-- に既に取られていれば 23505 を返すため、呼び出し側 (UseCase) は事前に
-- BulkOffsetPhotoOrdersOnPage(target_page) で escape してから呼ぶ。
-- ownership (photo / target page が同 photobook 配下か) は Repository 層で
-- FindPhotobookPhotoWithPhotobookID + FindPhotobookPageByID により検証する。
-- 0 行 → ErrPhotoNotFound (Repository 層で変換)。
-- name: UpdatePhotobookPhotoPageAndOrder :execrows
UPDATE photobook_photos
   SET page_id       = $2,
       display_order = $3
 WHERE id = $1;

-- FindPhotobookPhotoWithPhotobookID: photo + 所属 photobook_id を JOIN で同時取得 (P-1)。
-- ownership 検証用。row 不在は呼び出し側で ErrPhotoNotFound に変換。
-- (caption / image_id / display_order も合わせて返すため、FindPhotobookPhotoByID の
--  上位互換として move ロジックで利用しやすい)
-- name: FindPhotobookPhotoWithPhotobookID :one
SELECT pp.id            AS id,
       pp.page_id       AS page_id,
       pp.image_id      AS image_id,
       pp.display_order AS display_order,
       pp.caption       AS caption,
       pp.created_at    AS created_at,
       pg.photobook_id  AS photobook_id
FROM photobook_photos pp
JOIN photobook_pages  pg ON pg.id = pp.page_id
WHERE pp.id = $1;
