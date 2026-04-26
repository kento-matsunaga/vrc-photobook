-- PR19: Photobook Page の任意メタ情報を表す photobook_page_metas テーブル。
--
-- 設計参照:
--   - docs/design/aggregates/photobook/データモデル設計.md §5
--
-- 重要な決定:
--   - page_id を PK とする（Page 1 つに 0..1 で所属）
--   - cast は PostgreSQL text[]（業務知識 v4 §4）
--   - comment カラムは禁止（v4 §2.7 非採用用語）。note を使う
--   - photobook_id 経由で Photobook 削除時は CASCADE で削除される（page → meta 連鎖）

-- +goose Up
-- +goose StatementBegin
CREATE TABLE photobook_page_metas (
    page_id       uuid        NOT NULL,
    world         text        NULL,
    cast_list     text[]      NULL,
    photographer  text        NULL,
    note          text        NULL,
    event_date    date        NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT photobook_page_metas_pk PRIMARY KEY (page_id),

    CONSTRAINT photobook_page_metas_page_id_fkey
        FOREIGN KEY (page_id)
        REFERENCES photobook_pages (id)
        ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
DROP TABLE photobook_page_metas;
