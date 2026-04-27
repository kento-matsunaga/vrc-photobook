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
