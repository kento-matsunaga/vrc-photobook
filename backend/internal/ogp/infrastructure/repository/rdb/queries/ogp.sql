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
