-- M1 spike: upload_verification_sessions の最小 sqlc クエリ。
-- 本実装では auth/upload-verification 集約 Repository として整備する。

-- name: CreateUploadVerificationSession :one
INSERT INTO upload_verification_sessions (
    id,
    session_token_hash,
    photobook_id,
    allowed_intent_count,
    used_intent_count,
    expires_at
) VALUES (
    $1, $2, $3, $4, 0, $5
)
RETURNING id, photobook_id, allowed_intent_count, used_intent_count,
          expires_at, created_at, revoked_at;

-- name: GetUploadVerificationSessionByHash :one
SELECT id, session_token_hash, photobook_id, allowed_intent_count,
       used_intent_count, expires_at, created_at, revoked_at
FROM upload_verification_sessions
WHERE session_token_hash = $1;

-- name: ConsumeUploadVerificationIntent :one
-- アトミック消費: hash 一致 + photobook_id 一致 + 有効期限 + revoked なし + 残数あり
-- すべてを満たすときのみ used_intent_count を +1 し、その後の値を返す。
-- 0 行返却（pgx.ErrNoRows）のとき呼び出し側で「拒否（exhausted/expired/revoked/not_found のいずれか）」と判定する。
UPDATE upload_verification_sessions
SET used_intent_count = used_intent_count + 1
WHERE session_token_hash = $1
  AND photobook_id = $2
  AND revoked_at IS NULL
  AND expires_at > now()
  AND used_intent_count < allowed_intent_count
RETURNING used_intent_count, allowed_intent_count;
