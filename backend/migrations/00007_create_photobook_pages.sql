-- PR19: Photobook の Page を表す photobook_pages テーブル。
--
-- 設計参照:
--   - docs/design/aggregates/photobook/データモデル設計.md §4
--   - docs/plan/m2-photobook-image-connection-plan.md §3.2
--
-- 重要な決定:
--   - photobook_id への FK は ON DELETE CASCADE（Photobook 削除で連鎖）
--   - display_order は 0 始まり連番（DB は uniqueness のみ、連続性はアプリ層保証）
--   - caption は length 0..200（CHECK）
--   - deleted_at は持たない（CASCADE 物理削除でライフサイクル管理）
--   - I1（page 最低 1 件）はアプリ層で保証

-- +goose Up
-- +goose StatementBegin
CREATE TABLE photobook_pages (
    id            uuid        NOT NULL,
    photobook_id  uuid        NOT NULL,
    display_order int         NOT NULL,
    caption       text        NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT photobook_pages_pk PRIMARY KEY (id),

    CONSTRAINT photobook_pages_photobook_id_fkey
        FOREIGN KEY (photobook_id)
        REFERENCES photobooks (id)
        ON DELETE CASCADE,

    CONSTRAINT photobook_pages_display_order_check
        CHECK (display_order >= 0),

    CONSTRAINT photobook_pages_caption_len_check
        CHECK (caption IS NULL OR char_length(caption) BETWEEN 0 AND 200)
);
-- +goose StatementEnd

CREATE UNIQUE INDEX photobook_pages_photobook_id_display_order_uniq
    ON photobook_pages (photobook_id, display_order);

CREATE INDEX photobook_pages_photobook_id_idx
    ON photobook_pages (photobook_id);

-- +goose Down
DROP TABLE photobook_pages;
