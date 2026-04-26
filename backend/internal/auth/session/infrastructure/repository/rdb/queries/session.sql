-- PR7: Session 認可機構の sqlc query 群。
--
-- 設計参照:
--   - docs/design/auth/session/データモデル設計.md
--   - docs/plan/m2-session-auth-implementation-plan.md §9
--
-- セキュリティ:
--   - すべての SELECT は revoked_at IS NULL AND expires_at > now() を条件に含める
--     （期限切れ / revoke 済 session を取り出してしまう経路を作らない）
--   - DELETE / UPDATE は WHERE 条件を必ず明示。アプリ層からの任意 SQL は許可しない

-- name: CreateSession :exec
INSERT INTO sessions (
    id,
    session_token_hash,
    session_type,
    photobook_id,
    token_version_at_issue,
    expires_at,
    created_at,
    last_used_at,
    revoked_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, NULL, NULL
);

-- name: FindActiveSessionByHash :one
SELECT
    id,
    session_token_hash,
    session_type,
    photobook_id,
    token_version_at_issue,
    expires_at,
    created_at,
    last_used_at,
    revoked_at
FROM sessions
WHERE session_token_hash = $1
  AND session_type       = $2
  AND photobook_id       = $3
  AND revoked_at IS NULL
  AND expires_at > now();

-- name: TouchSession :execrows
UPDATE sessions
   SET last_used_at = now()
 WHERE id = $1
   AND revoked_at IS NULL
   AND expires_at > now();

-- name: RevokeSessionByID :execrows
UPDATE sessions
   SET revoked_at = now()
 WHERE id = $1
   AND revoked_at IS NULL;

-- name: RevokeAllDraftsByPhotobook :execrows
UPDATE sessions
   SET revoked_at = now()
 WHERE photobook_id = $1
   AND session_type = 'draft'
   AND revoked_at IS NULL;

-- name: RevokeAllManageByTokenVersion :execrows
UPDATE sessions
   SET revoked_at = now()
 WHERE photobook_id           = $1
   AND session_type           = 'manage'
   AND token_version_at_issue <= $2
   AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions
 WHERE (revoked_at IS NOT NULL AND revoked_at < now() - interval '30 days')
    OR (expires_at < now() - interval '7 days');
