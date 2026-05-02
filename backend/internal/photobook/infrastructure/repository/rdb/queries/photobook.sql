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

-- UpdatePhotobookSettings: PR27 編集 UI から settings 一括 PATCH。
--
-- title / description / type / layout / opening_style / visibility / cover_title を
-- まとめて更新する。version+1 を含めて status='draft' AND version=$expected で OCC。
-- 0 行影響は ErrOptimisticLockConflict。
-- name: UpdatePhotobookSettings :execrows
UPDATE photobooks
   SET type           = $2,
       title          = $3,
       description    = $4,
       layout         = $5,
       opening_style  = $6,
       visibility     = $7,
       cover_title    = $8,
       updated_at     = $9,
       version        = version + 1
 WHERE id      = $1
   AND status  = 'draft'
   AND version = $10;

-- name: TouchDraftPhotobook :execrows
UPDATE photobooks
   SET draft_expires_at = $2,
       updated_at       = now(),
       version          = version + 1
 WHERE id      = $1
   AND status  = 'draft'
   AND version = $3;

-- name: PublishPhotobookFromDraft :execrows
-- 2026-05-03 STOP α P0 v2: 同 TX で rights_agreed=true / rights_agreed_at=$4 を保存。
-- /edit publish UI の同意チェック → UseCase が pb.WithRightsAgreed(now) で domain を更新 →
-- 本 SQL で永続化、までを 1 つの operation として扱う（業務知識 v4 §3.1）。
-- 同意のみ残って publish 失敗 / publish のみ通って同意未保存の状態を作らない。
UPDATE photobooks
   SET status                   = 'published',
       public_url_slug          = $2,
       manage_url_token_hash    = $3,
       manage_url_token_version = 0,
       draft_edit_token_hash    = NULL,
       draft_expires_at         = NULL,
       published_at             = $4,
       rights_agreed            = true,
       rights_agreed_at         = $4,
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

-- name: SetPhotobookHiddenByOperator :execrows
-- PR34b: 運営による hide / unhide 操作。
-- 計画書 §5.6 / ユーザー判断 #5 で version は上げない方針。published のみ対象。
-- 0 行更新は呼び出し側で「対象が published でない or 既に目的状態」として扱う。
UPDATE photobooks
   SET hidden_by_operator = $2,
       updated_at         = $3
 WHERE id                 = $1
   AND status             = 'published'
   AND hidden_by_operator = $4;

-- name: ListHiddenPhotobooksForOps :many
-- PR34b: hidden_by_operator=true な published photobook の一覧（cmd/ops list-hidden 用）。
-- raw token / hash 系は返さない（呼び出し側で出さなくてよい列のみ select）。
SELECT
    id,
    public_url_slug,
    title,
    creator_display_name,
    visibility,
    status,
    version,
    published_at,
    updated_at
FROM photobooks
WHERE hidden_by_operator = true
ORDER BY updated_at DESC
LIMIT $1
OFFSET $2;

-- name: GetPhotobookForOps :one
-- PR34b: cmd/ops show 用。raw token / hash 系は返さない。
SELECT
    id,
    type,
    title,
    description,
    layout,
    opening_style,
    visibility,
    creator_display_name,
    creator_x_id,
    cover_title,
    public_url_slug,
    status,
    hidden_by_operator,
    version,
    published_at,
    created_at,
    updated_at
FROM photobooks
WHERE id = $1;
