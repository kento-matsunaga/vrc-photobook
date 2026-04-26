-- PR20: Upload Verification Session テーブル。
--
-- 設計参照:
--   - docs/adr/0005-image-upload-flow.md §upload_verification_session の保存先
--   - docs/plan/m2-upload-verification-plan.md §3
--
-- 重要な決定:
--   - photobook_id への FK は ON DELETE CASCADE（photobook 削除で連鎖）
--   - session_token_hash は SHA-256 32B 固定
--   - allowed_intent_count default 20 / used_intent_count default 0（ADR-0005）
--   - 30 分 TTL は domain 側の既定で expires_at として保存
--   - revoked_at は明示 revoke 用（nullable）
--   - 失敗理由を区別しない atomic consume を可能にする CHECK 群
--   - sessions テーブルへの統合は Phase 2 検討（MVP は分離）

-- +goose Up
-- +goose StatementBegin
CREATE TABLE upload_verification_sessions (
    id                    uuid        NOT NULL,
    photobook_id          uuid        NOT NULL,
    session_token_hash    bytea       NOT NULL,
    allowed_intent_count  int         NOT NULL DEFAULT 20,
    used_intent_count     int         NOT NULL DEFAULT 0,
    expires_at            timestamptz NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    revoked_at            timestamptz NULL,

    CONSTRAINT upload_verification_sessions_pk PRIMARY KEY (id),

    CONSTRAINT upload_verification_sessions_photobook_id_fkey
        FOREIGN KEY (photobook_id)
        REFERENCES photobooks (id)
        ON DELETE CASCADE,

    CONSTRAINT upload_verification_sessions_token_hash_len_check
        CHECK (octet_length(session_token_hash) = 32),

    CONSTRAINT upload_verification_sessions_allowed_positive_check
        CHECK (allowed_intent_count > 0),

    CONSTRAINT upload_verification_sessions_used_nonneg_check
        CHECK (used_intent_count >= 0),

    CONSTRAINT upload_verification_sessions_used_le_allowed_check
        CHECK (used_intent_count <= allowed_intent_count),

    CONSTRAINT upload_verification_sessions_expires_after_created_check
        CHECK (expires_at > created_at)
);
-- +goose StatementEnd

-- token hash 一意 (検索 + 重複防止)
CREATE UNIQUE INDEX upload_verification_sessions_token_hash_uniq
    ON upload_verification_sessions (session_token_hash);

-- photobook 配下の active session 検索 / cleanup
CREATE INDEX upload_verification_sessions_photobook_expires_idx
    ON upload_verification_sessions (photobook_id, expires_at);

-- expires_at による cleanup batch 用（active のみを対象）
CREATE INDEX upload_verification_sessions_expires_at_active_idx
    ON upload_verification_sessions (expires_at)
    WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE upload_verification_sessions;
