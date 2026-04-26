-- PR18: Image 集約テーブル。
--
-- 設計参照:
--   - docs/design/aggregates/image/データモデル設計.md §3
--   - docs/design/aggregates/image/ドメイン設計.md §4
--   - docs/adr/0005-image-upload-flow.md
--   - docs/plan/m2-image-upload-plan.md §4
--
-- 重要な決定:
--   - owner_photobook_id への FK は ON DELETE RESTRICT（明示削除フローを通すため）
--   - status: uploading / processing / available / failed / deleted / purged の 6 値（rejected は採用しない）
--   - failure_reason は 12 種固定（CHECK 制約で値域固定）
--   - status='available' のとき normalized_format / 寸法 / size / metadata_stripped_at が必須
--   - status='failed' のとき failure_reason 必須
--   - status='deleted'|'purged' のとき deleted_at 必須
--   - 寸法 1..=8192 / size 1..=10485760
--   - image_variants は別 migration（00006）

-- +goose Up
-- +goose StatementBegin
CREATE TABLE images (
    id                    uuid        NOT NULL,
    owner_photobook_id    uuid        NOT NULL,
    usage_kind            text        NOT NULL,
    source_format         text        NOT NULL,
    normalized_format     text        NULL,
    original_width        int         NULL,
    original_height       int         NULL,
    original_byte_size    bigint      NULL,
    metadata_stripped_at  timestamptz NULL,
    status                text        NOT NULL DEFAULT 'uploading',
    uploaded_at           timestamptz NOT NULL DEFAULT now(),
    available_at          timestamptz NULL,
    failed_at             timestamptz NULL,
    failure_reason        text        NULL,
    deleted_at            timestamptz NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT images_pk PRIMARY KEY (id),

    CONSTRAINT images_owner_photobook_id_fkey
        FOREIGN KEY (owner_photobook_id)
        REFERENCES photobooks (id)
        ON DELETE RESTRICT,

    CONSTRAINT images_usage_kind_check
        CHECK (usage_kind IN ('photo', 'cover', 'ogp')),

    CONSTRAINT images_source_format_check
        CHECK (source_format IN ('jpg', 'png', 'webp', 'heic')),

    CONSTRAINT images_normalized_format_check
        CHECK (normalized_format IS NULL OR normalized_format IN ('jpg', 'webp')),

    CONSTRAINT images_status_check
        CHECK (status IN ('uploading', 'processing', 'available', 'failed', 'deleted', 'purged')),

    CONSTRAINT images_original_width_check
        CHECK (original_width IS NULL OR (original_width BETWEEN 1 AND 8192)),

    CONSTRAINT images_original_height_check
        CHECK (original_height IS NULL OR (original_height BETWEEN 1 AND 8192)),

    CONSTRAINT images_original_byte_size_check
        CHECK (original_byte_size IS NULL OR (original_byte_size BETWEEN 1 AND 10485760)),

    CONSTRAINT images_failure_reason_check
        CHECK (
            failure_reason IS NULL OR failure_reason IN (
                'file_too_large',
                'size_mismatch',
                'unsupported_format',
                'svg_not_allowed',
                'animated_image_not_allowed',
                'dimensions_too_large',
                'decode_failed',
                'exif_strip_failed',
                'heic_conversion_failed',
                'variant_generation_failed',
                'object_not_found',
                'unknown'
            )
        ),

    -- 状態整合性:
    -- available / deleted / purged のとき、normalized_format / 寸法 / size / metadata_stripped_at が必須
    -- failed のとき failure_reason 必須
    -- deleted / purged のとき deleted_at 必須
    CONSTRAINT images_status_columns_consistency_check
        CHECK (
            CASE status
                WHEN 'uploading' THEN
                    failure_reason IS NULL
                    AND failed_at IS NULL
                    AND deleted_at IS NULL
                WHEN 'processing' THEN
                    failure_reason IS NULL
                    AND failed_at IS NULL
                    AND deleted_at IS NULL
                WHEN 'available' THEN
                    normalized_format IS NOT NULL
                    AND original_width IS NOT NULL
                    AND original_height IS NOT NULL
                    AND original_byte_size IS NOT NULL
                    AND metadata_stripped_at IS NOT NULL
                    AND available_at IS NOT NULL
                    AND failure_reason IS NULL
                    AND failed_at IS NULL
                    AND deleted_at IS NULL
                WHEN 'failed' THEN
                    failure_reason IS NOT NULL
                    AND failed_at IS NOT NULL
                    AND deleted_at IS NULL
                WHEN 'deleted' THEN
                    deleted_at IS NOT NULL
                WHEN 'purged' THEN
                    deleted_at IS NOT NULL
                ELSE TRUE
            END
        )
);
-- +goose StatementEnd

-- 検索性能 (image データモデル §3.1)
CREATE INDEX images_owner_photobook_id_idx
    ON images (owner_photobook_id);

CREATE INDEX images_owner_photobook_id_usage_kind_idx
    ON images (owner_photobook_id, usage_kind);

CREATE INDEX images_status_deleted_at_idx
    ON images (status, deleted_at);

CREATE INDEX images_status_failed_at_idx
    ON images (status, failed_at)
    WHERE status = 'failed';

CREATE INDEX images_status_available_at_idx
    ON images (status, available_at)
    WHERE status = 'available';

-- +goose Down
DROP TABLE images;
