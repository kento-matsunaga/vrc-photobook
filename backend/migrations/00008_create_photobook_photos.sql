-- PR19: Photobook の Photo を表す photobook_photos テーブル。
--
-- 設計参照:
--   - docs/design/aggregates/photobook/データモデル設計.md §6
--   - docs/plan/m2-photobook-image-connection-plan.md §3.3
--
-- 重要な決定:
--   - page_id への FK は ON DELETE CASCADE（Page 削除で連鎖）
--   - image_id への FK は ON DELETE RESTRICT（誤参照削除を防ぐ多層防御、設計 §6 / Image 集約所有モデル）
--   - display_order は Page 内 0 始まり連番（DB は uniqueness のみ、連続性はアプリ層保証）
--   - caption は length 0..200（CHECK）
--   - I2（page 最低 1 photo）は draft 中は緩め / published 時のみ強制（PR19 計画 Q4）
--   - owner_photobook_id 整合（image.owner_photobook_id == page.photobook_id）は
--     Repository / UseCase 層で担保（DB trigger は使わない、PR19 計画 Q11）

-- +goose Up
-- +goose StatementBegin
CREATE TABLE photobook_photos (
    id            uuid        NOT NULL,
    page_id       uuid        NOT NULL,
    image_id      uuid        NOT NULL,
    display_order int         NOT NULL,
    caption       text        NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT photobook_photos_pk PRIMARY KEY (id),

    CONSTRAINT photobook_photos_page_id_fkey
        FOREIGN KEY (page_id)
        REFERENCES photobook_pages (id)
        ON DELETE CASCADE,

    CONSTRAINT photobook_photos_image_id_fkey
        FOREIGN KEY (image_id)
        REFERENCES images (id)
        ON DELETE RESTRICT,

    CONSTRAINT photobook_photos_display_order_check
        CHECK (display_order >= 0),

    CONSTRAINT photobook_photos_caption_len_check
        CHECK (caption IS NULL OR char_length(caption) BETWEEN 0 AND 200)
);
-- +goose StatementEnd

CREATE UNIQUE INDEX photobook_photos_page_id_display_order_uniq
    ON photobook_photos (page_id, display_order);

CREATE INDEX photobook_photos_image_id_idx
    ON photobook_photos (image_id);

-- +goose Down
DROP TABLE photobook_photos;
