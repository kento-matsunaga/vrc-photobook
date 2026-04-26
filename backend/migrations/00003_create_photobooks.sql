-- PR9a: Photobook 集約テーブル。
--
-- 設計参照:
--   - docs/design/aggregates/photobook/データモデル設計.md §3
--   - docs/plan/m2-photobook-session-integration-plan.md §4 / §7
--
-- 重要な決定:
--   - cover_image_id への FK は本 PR では張らない（Image 集約は PR11 以降）
--   - pages / photos / page_metas は本 PR では作らない（Image 集約と一緒に追加）
--   - outbox_events は本 PR では作らない（Outbox 本実装は別 PR）
--   - status: draft / published / deleted / purged の 4 値を CHECK で許可するが、
--     UseCase は draft / published のみ（softDelete / restore / purge は後続 PR）
--   - draft / published 整合性は I-D1 / I-D2 / I-D6 を CASE で表現
--   - token hash は SHA-256 32B 固定（DB 側で octet_length チェック）

-- +goose Up
-- +goose StatementBegin
CREATE TABLE photobooks (
    id                       uuid        NOT NULL,
    type                     text        NOT NULL,
    title                    text        NOT NULL,
    description              text        NULL,
    layout                   text        NOT NULL,
    opening_style            text        NOT NULL,
    visibility               text        NOT NULL DEFAULT 'unlisted',
    sensitive                boolean     NOT NULL DEFAULT false,
    rights_agreed            boolean     NOT NULL DEFAULT false,
    rights_agreed_at         timestamptz NULL,
    creator_display_name     text        NOT NULL,
    creator_x_id             text        NULL,
    cover_title              text        NULL,
    cover_image_id           uuid        NULL,
    public_url_slug          text        NULL,
    manage_url_token_hash    bytea       NULL,
    manage_url_token_version int         NOT NULL DEFAULT 0,
    draft_edit_token_hash    bytea       NULL,
    draft_expires_at         timestamptz NULL,
    status                   text        NOT NULL DEFAULT 'draft',
    hidden_by_operator       boolean     NOT NULL DEFAULT false,
    version                  int         NOT NULL DEFAULT 0,
    published_at             timestamptz NULL,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    deleted_at               timestamptz NULL,

    CONSTRAINT photobooks_pk PRIMARY KEY (id),

    CONSTRAINT photobooks_status_check
        CHECK (status IN ('draft', 'published', 'deleted', 'purged')),

    CONSTRAINT photobooks_visibility_check
        CHECK (visibility IN ('public', 'unlisted', 'private')),

    CONSTRAINT photobooks_type_check
        CHECK (type IN ('event', 'daily', 'portfolio', 'avatar', 'world', 'memory', 'free')),

    CONSTRAINT photobooks_layout_check
        CHECK (layout IN ('simple', 'magazine', 'card', 'large')),

    CONSTRAINT photobooks_opening_style_check
        CHECK (opening_style IN ('light', 'cover_first_view')),

    CONSTRAINT photobooks_version_nonneg_check
        CHECK (version >= 0),

    CONSTRAINT photobooks_manage_url_token_version_nonneg_check
        CHECK (manage_url_token_version >= 0),

    CONSTRAINT photobooks_draft_edit_token_hash_len_check
        CHECK (draft_edit_token_hash IS NULL OR octet_length(draft_edit_token_hash) = 32),

    CONSTRAINT photobooks_manage_url_token_hash_len_check
        CHECK (manage_url_token_hash IS NULL OR octet_length(manage_url_token_hash) = 32),

    -- 状態整合性（I-D1 / I-D2 / I-D6 / I7）
    -- draft:     draft_edit_token_hash IS NOT NULL AND draft_expires_at IS NOT NULL
    --            AND public_url_slug IS NULL AND manage_url_token_hash IS NULL
    -- published: public_url_slug IS NOT NULL AND manage_url_token_hash IS NOT NULL
    --            AND draft_edit_token_hash IS NULL AND draft_expires_at IS NULL
    --            AND published_at IS NOT NULL
    -- deleted:   published と同条件 + deleted_at IS NOT NULL
    -- purged:    本書では制約しない（後続 PR で詰める）
    CONSTRAINT photobooks_status_columns_consistency_check
        CHECK (
            CASE status
                WHEN 'draft' THEN
                    draft_edit_token_hash IS NOT NULL
                    AND draft_expires_at IS NOT NULL
                    AND public_url_slug IS NULL
                    AND manage_url_token_hash IS NULL
                    AND published_at IS NULL
                    AND deleted_at IS NULL
                WHEN 'published' THEN
                    draft_edit_token_hash IS NULL
                    AND draft_expires_at IS NULL
                    AND public_url_slug IS NOT NULL
                    AND manage_url_token_hash IS NOT NULL
                    AND published_at IS NOT NULL
                    AND deleted_at IS NULL
                WHEN 'deleted' THEN
                    draft_edit_token_hash IS NULL
                    AND draft_expires_at IS NULL
                    AND public_url_slug IS NOT NULL
                    AND manage_url_token_hash IS NOT NULL
                    AND published_at IS NOT NULL
                    AND deleted_at IS NOT NULL
                ELSE TRUE
            END
        )
);
-- +goose StatementEnd

-- 部分 UNIQUE: Slug 復元ルール（published / deleted のみで unique、purged で解放、draft では NULL）
CREATE UNIQUE INDEX photobooks_public_url_slug_uniq
    ON photobooks (public_url_slug)
    WHERE status IN ('published', 'deleted');

-- 部分 UNIQUE: 同一 token hash は最大 1 件（NULL は除外）
CREATE UNIQUE INDEX photobooks_manage_url_token_hash_uniq
    ON photobooks (manage_url_token_hash)
    WHERE manage_url_token_hash IS NOT NULL;

CREATE UNIQUE INDEX photobooks_draft_edit_token_hash_uniq
    ON photobooks (draft_edit_token_hash)
    WHERE draft_edit_token_hash IS NOT NULL;

-- 期限切れ draft 検出（Reconcile 用）
CREATE INDEX photobooks_draft_expires_idx
    ON photobooks (draft_expires_at)
    WHERE status = 'draft';

-- 論理削除 / 物理削除バッチ用
CREATE INDEX photobooks_status_deleted_at_idx
    ON photobooks (status, deleted_at);

-- +goose Down
DROP TABLE photobooks;
