-- M1 spike: upload_verification_sessions（Turnstile セッション化、ADR-0005 / 付録C P0-17/18）
-- 本実装では auth/upload-verification 集約として整備する（PoC では同等の最小スキーマ）。

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS upload_verification_sessions (
    id                       uuid        PRIMARY KEY,
    session_token_hash       bytea       NOT NULL UNIQUE,
    photobook_id             uuid        NOT NULL,
    allowed_intent_count     int         NOT NULL DEFAULT 20 CHECK (allowed_intent_count > 0),
    used_intent_count        int         NOT NULL DEFAULT 0  CHECK (used_intent_count >= 0),
    expires_at               timestamptz NOT NULL,
    created_at               timestamptz NOT NULL DEFAULT now(),
    revoked_at               timestamptz,
    CHECK (used_intent_count <= allowed_intent_count),
    CHECK (expires_at > created_at),
    CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS upload_verification_active_idx
    ON upload_verification_sessions (photobook_id, expires_at)
    WHERE revoked_at IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS upload_verification_expired_gc_idx
    ON upload_verification_sessions (expires_at)
    WHERE revoked_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS upload_verification_sessions;
-- +goose StatementEnd
