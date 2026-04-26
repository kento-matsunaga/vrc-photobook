-- PR7: Session 認可機構の単体テーブル（draft / manage 汎用 1 本）。
--
-- 設計参照:
--   - docs/design/auth/session/データモデル設計.md §3
--   - docs/adr/0003-frontend-token-session-flow.md
--   - docs/plan/m2-session-auth-implementation-plan.md §4 / §8
--
-- 重要な決定:
--   - photobook_id への FK は本 PR では張らない。Photobook aggregate は PR9 で実装するため、
--     PR9 で ALTER TABLE ... ADD CONSTRAINT で後付けする。
--   - session_token_hash は SHA-256（32 バイト固定）、raw token は保存しない（ADR-0003）。
--   - draft の token_version_at_issue は CHECK で 0 強制（I-S5）。

-- +goose Up
-- +goose StatementBegin
CREATE TABLE sessions (
    id                       uuid        NOT NULL,
    session_token_hash       bytea       NOT NULL,
    session_type             text        NOT NULL,
    photobook_id             uuid        NOT NULL,
    token_version_at_issue   int         NOT NULL DEFAULT 0,
    expires_at               timestamptz NOT NULL,
    created_at               timestamptz NOT NULL DEFAULT now(),
    last_used_at             timestamptz,
    revoked_at               timestamptz,

    CONSTRAINT sessions_pk PRIMARY KEY (id),

    CONSTRAINT sessions_session_type_check
        CHECK (session_type IN ('draft', 'manage')),

    CONSTRAINT sessions_session_token_hash_len_check
        CHECK (octet_length(session_token_hash) = 32),

    CONSTRAINT sessions_expires_after_created_check
        CHECK (expires_at > created_at),

    CONSTRAINT sessions_last_used_in_range_check
        CHECK (
            last_used_at IS NULL
            OR (last_used_at >= created_at AND last_used_at <= expires_at)
        ),

    CONSTRAINT sessions_revoked_after_created_check
        CHECK (revoked_at IS NULL OR revoked_at >= created_at),

    CONSTRAINT sessions_token_version_nonneg_check
        CHECK (token_version_at_issue >= 0),

    CONSTRAINT sessions_draft_token_version_zero_check
        CHECK (session_type <> 'draft' OR token_version_at_issue = 0)
);
-- +goose StatementEnd

CREATE UNIQUE INDEX sessions_session_token_hash_uniq
    ON sessions (session_token_hash);

CREATE INDEX sessions_photobook_type_revoked_idx
    ON sessions (photobook_id, session_type, revoked_at);

CREATE INDEX sessions_photobook_type_version_active_idx
    ON sessions (photobook_id, session_type, token_version_at_issue)
    WHERE revoked_at IS NULL;

CREATE INDEX sessions_expires_active_idx
    ON sessions (expires_at)
    WHERE revoked_at IS NULL;

CREATE INDEX sessions_revoked_idx
    ON sessions (revoked_at)
    WHERE revoked_at IS NOT NULL;

-- +goose Down
DROP TABLE sessions;
