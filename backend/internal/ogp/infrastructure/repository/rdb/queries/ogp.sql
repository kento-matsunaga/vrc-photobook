-- PR33b: photobook_ogp_images の sqlc query。
--
-- 設計参照:
--   - docs/plan/m2-ogp-generation-plan.md §6 / §9
--   - docs/design/cross-cutting/ogp-generation.md §3 / §4
--
-- セキュリティ:
--   - failure_reason は呼び出し側 VO で sanitize 済を渡す（200 char 上限、
--     危険語 redact、ogp_failure_reason VO 参照）

-- name: FindOgpByPhotobookID :one
SELECT
    id, photobook_id, status, image_id, version,
    generated_at, failed_at, failure_reason,
    created_at, updated_at
FROM photobook_ogp_images
WHERE photobook_id = $1;

-- name: CreatePendingOgp :exec
-- 新規 pending row を 1 行 INSERT。photobook_id UNIQUE 違反は呼び出し側で扱う。
INSERT INTO photobook_ogp_images (
    id, photobook_id, status, version,
    created_at, updated_at
) VALUES (
    $1, $2, 'pending', 1,
    $3, $3
);

-- name: MarkOgpGenerated :exec
-- pending / stale → generated。image_id / generated_at 必須。
UPDATE photobook_ogp_images
SET
    status        = 'generated',
    image_id      = $2,
    generated_at  = $3,
    updated_at    = $3,
    failed_at     = NULL,
    failure_reason = NULL
WHERE id = $1
  AND status IN ('pending', 'stale', 'failed');

-- name: MarkOgpFailed :exec
-- pending / stale → failed。failed_at / failure_reason 必須（呼び出し側で sanitize）。
UPDATE photobook_ogp_images
SET
    status         = 'failed',
    failed_at      = $2,
    failure_reason = $3,
    updated_at     = $2
WHERE id = $1
  AND status IN ('pending', 'stale');

-- name: MarkOgpStale :exec
-- generated / failed → stale + version++（Photobook 更新時）。
UPDATE photobook_ogp_images
SET
    status     = 'stale',
    version    = version + 1,
    updated_at = $2
WHERE id = $1
  AND status IN ('generated', 'failed');

-- name: ListPendingOgp :many
-- pickup query（status='pending' / 'stale' / 'failed'）。
-- updated_at 古い順で limit。FOR UPDATE SKIP LOCKED は CLI single-runner 想定では
-- 任意（PR33d で worker 連携時に再評価）。
SELECT
    id, photobook_id, status, image_id, version,
    generated_at, failed_at, failure_reason,
    created_at, updated_at
FROM photobook_ogp_images
WHERE status IN ('pending', 'stale', 'failed')
ORDER BY updated_at ASC
LIMIT $1;

-- ----------------------------------------------------------------------------
-- PR33c: OGP 生成完了化のための images / image_variants INSERT。
-- ----------------------------------------------------------------------------
--
-- 採用方針:
--   - OGP は domain.image の通常フローを通さず、ogp package が直接書き込む
--     （source は backend 生成 PNG で uploading→processing→available の遷移を持たない）
--   - 必須列を一発で埋めて status='available' で INSERT する
--   - image_variants には kind='ogp' で 1 行だけ INSERT（display/thumbnail 等は不要）

-- name: CreateOgpImageRecord :exec
-- usage_kind='ogp', status='available' で images に 1 行作成する（generated 化用）。
-- すべての NOT NULL（images_status_columns_consistency_check 'available' 経路）を満たす。
INSERT INTO images (
    id, owner_photobook_id, usage_kind,
    source_format, normalized_format,
    original_width, original_height, original_byte_size,
    metadata_stripped_at, status, uploaded_at, available_at,
    created_at, updated_at
) VALUES (
    $1, $2, 'ogp',
    'png', 'jpg',
    $3, $4, $5,
    $6, 'available', $6, $6,
    $6, $6
);

-- name: CreateOgpImageVariant :exec
-- image_variants に kind='ogp' / mime_type='image/png' で 1 行作成する。
-- (image_id, kind) UNIQUE 制約で同一 image に対する再投入は失敗する。
INSERT INTO image_variants (
    id, image_id, kind, storage_key,
    width, height, byte_size, mime_type, created_at
) VALUES (
    $1, $2, 'ogp', $3,
    $4, $5, $6, 'image/png', $7
);

-- name: GetOgpDeliveryByPhotobookID :one
-- 公開 OGP 配信 lookup 用：photobook_ogp_images + photobook 状態 + image_variants(kind='ogp')
-- を JOIN して、Workers proxy が必要な (status, version, storage_key) を返す。
--
-- 配信判定:
--   - photobook が published / visibility='public' / hidden_by_operator=false で **無い**場合
--     → 呼び出し側で fallback（status を「公開不可」として扱う）
--   - status='generated' かつ image_id != NULL かつ image_variants(kind='ogp') が存在
--     → storage_key を返す
--   - それ以外は storage_key NULL
SELECT
    o.status::text       AS ogp_status,
    o.version            AS ogp_version,
    p.status::text       AS photobook_status,
    p.visibility::text   AS photobook_visibility,
    p.hidden_by_operator,
    v.storage_key        AS ogp_storage_key
FROM photobook_ogp_images o
INNER JOIN photobooks p ON p.id = o.photobook_id
LEFT JOIN image_variants v
    ON v.image_id = o.image_id
   AND v.kind = 'ogp'
WHERE o.photobook_id = $1;

-- name: EnsureCreatedPendingOgp :exec
-- M-2 OGP 同期化 (STOP β): publish 同 TX で pending 行を冪等 INSERT する。
-- photobook_id UNIQUE 違反は ON CONFLICT DO NOTHING で吸収（既存 row があれば no-op）。
-- worker 側 `CreatePendingOgp` 経路は existence check 後の strict INSERT を残し、
-- 本 query は publish UC からの冪等 ensure 専用とする。
INSERT INTO photobook_ogp_images (
    id, photobook_id, status, version,
    created_at, updated_at
) VALUES (
    $1, $2, 'pending', 1,
    $3, $3
)
ON CONFLICT (photobook_id) DO NOTHING;
