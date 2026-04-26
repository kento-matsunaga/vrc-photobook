-- PR18: ImageVariant テーブル。
--
-- 設計参照:
--   - docs/design/aggregates/image/データモデル設計.md §4
--   - docs/plan/m2-image-upload-plan.md §4.5
--
-- 重要な決定:
--   - image_id に FK ON DELETE CASCADE（image 行削除で variant も削除）
--   - (image_id, kind) UNIQUE で 1 image 1 種 1 行
--   - kind: original / display / thumbnail / ogp
--   - mime_type: image/jpeg / image/png / image/webp（HEIC は variant に出ない）
--   - storage_key は bucket 名を含めない（bucket 切替に耐える）

-- +goose Up
-- +goose StatementBegin
CREATE TABLE image_variants (
    id           uuid        NOT NULL,
    image_id     uuid        NOT NULL,
    kind         text        NOT NULL,
    storage_key  text        NOT NULL,
    width        int         NOT NULL,
    height       int         NOT NULL,
    byte_size    bigint      NOT NULL,
    mime_type    text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT image_variants_pk PRIMARY KEY (id),

    CONSTRAINT image_variants_image_id_fkey
        FOREIGN KEY (image_id)
        REFERENCES images (id)
        ON DELETE CASCADE,

    CONSTRAINT image_variants_kind_check
        CHECK (kind IN ('original', 'display', 'thumbnail', 'ogp')),

    CONSTRAINT image_variants_mime_type_check
        CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/webp')),

    CONSTRAINT image_variants_width_check
        CHECK (width >= 1),

    CONSTRAINT image_variants_height_check
        CHECK (height >= 1),

    CONSTRAINT image_variants_byte_size_check
        CHECK (byte_size >= 1)
);
-- +goose StatementEnd

-- (image_id, kind) UNIQUE: 1 image 1 種 1 行
CREATE UNIQUE INDEX image_variants_image_id_kind_uniq
    ON image_variants (image_id, kind);

-- storage_key 逆引き（運用保守）
CREATE INDEX image_variants_storage_key_idx
    ON image_variants (storage_key);

-- +goose Down
DROP TABLE image_variants;
