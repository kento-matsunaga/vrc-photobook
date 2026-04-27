-- PR9a: Photobook 集約の sqlc query 群。
--
-- 設計参照:
--   - docs/design/aggregates/photobook/データモデル設計.md
--   - docs/plan/m2-photobook-session-integration-plan.md §8
--
-- セキュリティ:
--   - すべての UPDATE は version = $expectedVersion を WHERE に含める（楽観ロック）
--   - 0 行 UPDATE はアプリ層で OptimisticLockConflict として扱う
--   - draft 検索は draft_expires_at > now() を条件に含め、期限切れを抜け出させない

-- name: CreateDraftPhotobook :exec
INSERT INTO photobooks (
    id,
    type,
    title,
    description,
    layout,
    opening_style,
    visibility,
    sensitive,
    rights_agreed,
    rights_agreed_at,
    creator_display_name,
    creator_x_id,
    cover_title,
    cover_image_id,
    public_url_slug,
    manage_url_token_hash,
    manage_url_token_version,
    draft_edit_token_hash,
    draft_expires_at,
    status,
    hidden_by_operator,
    version,
    published_at,
    created_at,
    updated_at,
    deleted_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, NULL, NULL, $15, $16, $17,
    'draft', false, $18, NULL, $19, $20, NULL
);

-- name: FindPhotobookByID :one
SELECT
    id, type, title, description, layout, opening_style, visibility,
    sensitive, rights_agreed, rights_agreed_at, creator_display_name,
    creator_x_id, cover_title, cover_image_id, public_url_slug,
    manage_url_token_hash, manage_url_token_version, draft_edit_token_hash,
    draft_expires_at, status, hidden_by_operator, version, published_at,
    created_at, updated_at, deleted_at
FROM photobooks
WHERE id = $1;

-- name: FindPhotobookByDraftEditTokenHash :one
SELECT
    id, type, title, description, layout, opening_style, visibility,
    sensitive, rights_agreed, rights_agreed_at, creator_display_name,
    creator_x_id, cover_title, cover_image_id, public_url_slug,
    manage_url_token_hash, manage_url_token_version, draft_edit_token_hash,
    draft_expires_at, status, hidden_by_operator, version, published_at,
    created_at, updated_at, deleted_at
FROM photobooks
WHERE draft_edit_token_hash = $1
  AND status = 'draft'
  AND draft_expires_at > now();

-- name: FindPhotobookByManageUrlTokenHash :one
SELECT
    id, type, title, description, layout, opening_style, visibility,
    sensitive, rights_agreed, rights_agreed_at, creator_display_name,
    creator_x_id, cover_title, cover_image_id, public_url_slug,
    manage_url_token_hash, manage_url_token_version, draft_edit_token_hash,
    draft_expires_at, status, hidden_by_operator, version, published_at,
    created_at, updated_at, deleted_at
FROM photobooks
WHERE manage_url_token_hash = $1
  AND status IN ('published', 'deleted');

-- name: FindPhotobookBySlug :one
SELECT
    id, type, title, description, layout, opening_style, visibility,
    sensitive, rights_agreed, rights_agreed_at, creator_display_name,
    creator_x_id, cover_title, cover_image_id, public_url_slug,
    manage_url_token_hash, manage_url_token_version, draft_edit_token_hash,
    draft_expires_at, status, hidden_by_operator, version, published_at,
    created_at, updated_at, deleted_at
FROM photobooks
WHERE public_url_slug = $1
  AND status = 'published'
  AND hidden_by_operator = false;

-- FindPhotobookBySlugAny: slug 一致のみで status / hidden_by_operator を判別しない。
-- PR25a 公開 Viewer は status / hidden / visibility を usecase 側で判定して
-- 200 / 410 / 404 を分岐するため、より permissive な query を別に置く。
-- name: FindPhotobookBySlugAny :one
SELECT
    id, type, title, description, layout, opening_style, visibility,
    sensitive, rights_agreed, rights_agreed_at, creator_display_name,
    creator_x_id, cover_title, cover_image_id, public_url_slug,
    manage_url_token_hash, manage_url_token_version, draft_edit_token_hash,
    draft_expires_at, status, hidden_by_operator, version, published_at,
    created_at, updated_at, deleted_at
FROM photobooks
WHERE public_url_slug = $1;

-- name: TouchDraftPhotobook :execrows
UPDATE photobooks
   SET draft_expires_at = $2,
       updated_at       = now(),
       version          = version + 1
 WHERE id      = $1
   AND status  = 'draft'
   AND version = $3;

-- name: PublishPhotobookFromDraft :execrows
UPDATE photobooks
   SET status                   = 'published',
       public_url_slug          = $2,
       manage_url_token_hash    = $3,
       manage_url_token_version = 0,
       draft_edit_token_hash    = NULL,
       draft_expires_at         = NULL,
       published_at             = $4,
       updated_at               = $4,
       version                  = version + 1
 WHERE id      = $1
   AND status  = 'draft'
   AND version = $5;

-- name: ReissuePhotobookManageUrl :execrows
UPDATE photobooks
   SET manage_url_token_hash    = $2,
       manage_url_token_version = manage_url_token_version + 1,
       updated_at               = now(),
       version                  = version + 1
 WHERE id      = $1
   AND status  IN ('published', 'deleted')
   AND version = $3;
