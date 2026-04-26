-- PR20: Upload Verification Session の sqlc query 群。
--
-- 設計参照:
--   - docs/adr/0005-image-upload-flow.md §upload_verification_session
--   - docs/plan/m2-upload-verification-plan.md §3 / §7
--
-- セキュリティ:
--   - すべての consume は単一 UPDATE で atomic に実行（FOR UPDATE 不要）
--   - 0 行影響時の理由は外に漏らさない（ErrUploadVerificationFailed で一括）

-- name: CreateUploadVerificationSession :exec
INSERT INTO upload_verification_sessions (
    id,
    photobook_id,
    session_token_hash,
    allowed_intent_count,
    used_intent_count,
    expires_at,
    created_at
) VALUES (
    $1, $2, $3, $4, 0, $5, $6
);

-- name: FindUploadVerificationSessionByID :one
SELECT id, photobook_id, session_token_hash, allowed_intent_count,
       used_intent_count, expires_at, created_at, revoked_at
FROM upload_verification_sessions
WHERE id = $1;

-- name: FindUploadVerificationSessionByTokenHash :one
SELECT id, photobook_id, session_token_hash, allowed_intent_count,
       used_intent_count, expires_at, created_at, revoked_at
FROM upload_verification_sessions
WHERE session_token_hash = $1;

-- name: ConsumeUploadVerificationSession :one
-- atomic consume: 単一 UPDATE で row-level lock を取り、
-- used_intent_count < allowed_intent_count かつ未期限切れ・未 revoke のみ +1。
-- 0 行影響は呼び出し側で ErrUploadVerificationFailed として扱う。
UPDATE upload_verification_sessions
   SET used_intent_count = used_intent_count + 1
 WHERE session_token_hash = $1
   AND photobook_id       = $2
   AND used_intent_count  < allowed_intent_count
   AND expires_at         > now()
   AND revoked_at         IS NULL
RETURNING id, used_intent_count, allowed_intent_count;

-- name: RevokeUploadVerificationSession :execrows
UPDATE upload_verification_sessions
   SET revoked_at = $2
 WHERE id         = $1
   AND revoked_at IS NULL;

-- name: CountActiveUploadVerificationSessionsByPhotobookID :one
SELECT COUNT(*)::int AS cnt
FROM upload_verification_sessions
WHERE photobook_id = $1
  AND expires_at   > now()
  AND revoked_at   IS NULL;
