-- PR18: Image 集約の sqlc query 群。
--
-- 設計参照:
--   - docs/design/aggregates/image/データモデル設計.md
--   - docs/plan/m2-image-upload-plan.md §4
--
-- セキュリティ:
--   - storage_key / failure_reason などのログ出力には security-guard.md に従う
--   - 削除済 (deleted_at IS NOT NULL) を含めるかどうかは呼び出し側の責任

-- name: CreateImage :exec
INSERT INTO images (
    id,
    owner_photobook_id,
    usage_kind,
    source_format,
    status,
    uploaded_at,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, 'uploading', $5, $6, $7
);

-- name: FindImageByID :one
SELECT
    id, owner_photobook_id, usage_kind, source_format,
    normalized_format, original_width, original_height,
    original_byte_size, metadata_stripped_at,
    status, uploaded_at, available_at, failed_at,
    failure_reason, deleted_at, created_at, updated_at
FROM images
WHERE id = $1;

-- name: ListActiveImagesByPhotobookID :many
SELECT
    id, owner_photobook_id, usage_kind, source_format,
    normalized_format, original_width, original_height,
    original_byte_size, metadata_stripped_at,
    status, uploaded_at, available_at, failed_at,
    failure_reason, deleted_at, created_at, updated_at
FROM images
WHERE owner_photobook_id = $1
  AND deleted_at IS NULL
ORDER BY uploaded_at ASC;

-- ListProcessingImagesForUpdate
--
-- M2 image-processor: status='processing' の Image を最大 N 件 claim する。
--
-- FOR UPDATE SKIP LOCKED により、複数 worker が並列で動いても同じ row を二重 claim しない。
-- 呼び出し側は短い TX 内で claim → return → 別 TX で重い処理（GetObject / 画像処理 / PUT）
-- → 別 TX で finalize（MarkAvailable / MarkFailed）するパターンを取る。
--
-- 並び順は uploaded_at ASC（古い順、stuck 検出時に最も困る順）。
-- name: ListProcessingImagesForUpdate :many
SELECT
    id, owner_photobook_id, usage_kind, source_format,
    normalized_format, original_width, original_height,
    original_byte_size, metadata_stripped_at,
    status, uploaded_at, available_at, failed_at,
    failure_reason, deleted_at, created_at, updated_at
FROM images
WHERE status = 'processing'
  AND deleted_at IS NULL
ORDER BY uploaded_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: UpdateImageStatusProcessing :execrows
UPDATE images
   SET status     = 'processing',
       updated_at = $2
 WHERE id = $1
   AND status = 'uploading';

-- name: UpdateImageStatusAvailable :execrows
UPDATE images
   SET status               = 'available',
       normalized_format    = $2,
       original_width       = $3,
       original_height      = $4,
       original_byte_size   = $5,
       metadata_stripped_at = $6,
       available_at         = $7,
       updated_at           = $8
 WHERE id     = $1
   AND status = 'processing';

-- name: UpdateImageStatusFailed :execrows
UPDATE images
   SET status         = 'failed',
       failure_reason = $2,
       failed_at      = $3,
       updated_at     = $4
 WHERE id     = $1
   AND status IN ('uploading', 'processing');

-- name: MarkImageDeleted :execrows
UPDATE images
   SET status     = 'deleted',
       deleted_at = $2,
       updated_at = $3
 WHERE id     = $1
   AND status IN ('available', 'failed');

-- name: CreateImageVariant :exec
INSERT INTO image_variants (
    id,
    image_id,
    kind,
    storage_key,
    width,
    height,
    byte_size,
    mime_type,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
);

-- name: ListImageVariantsByImageID :many
SELECT
    id, image_id, kind, storage_key,
    width, height, byte_size, mime_type, created_at
FROM image_variants
WHERE image_id = $1
ORDER BY kind ASC;
