-- PR33b: photobook_ogp_images。OGP 画像の状態管理（実体は Image 集約に格納）。
--
-- 設計参照:
--   - docs/design/cross-cutting/ogp-generation.md §3
--   - docs/plan/m2-ogp-generation-plan.md §6
--
-- 重要な決定:
--   - 1 photobook につき 1 row（photobook_id UNIQUE）
--   - status: pending / generated / failed / fallback / stale
--   - image_id は images.id への FK ON DELETE SET NULL（生成失敗時の不整合に耐える）
--   - Photobook 物理削除（purge）時は CASCADE で削除
--   - failure_reason は CHECK で 200 char 上限（worker / renderer 側で sanitize して書く）
--   - public 配信は別経路（Cloudflare Workers proxy、PR33c）。本 migration では DB のみ

-- +goose Up
-- +goose StatementBegin
CREATE TABLE photobook_ogp_images (
    id              uuid        NOT NULL DEFAULT gen_random_uuid(),
    photobook_id    uuid        NOT NULL,
    status          text        NOT NULL DEFAULT 'pending',
    image_id        uuid        NULL,
    version         int         NOT NULL DEFAULT 1,
    generated_at    timestamptz NULL,
    failed_at       timestamptz NULL,
    failure_reason  text        NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT photobook_ogp_images_pk PRIMARY KEY (id),

    CONSTRAINT photobook_ogp_images_photobook_unique UNIQUE (photobook_id),

    CONSTRAINT photobook_ogp_images_status_check
        CHECK (status IN ('pending', 'generated', 'failed', 'fallback', 'stale')),

    CONSTRAINT photobook_ogp_images_version_check
        CHECK (version >= 1),

    -- failure_reason は worker / renderer が sanitize して書く前提で 200 char 上限
    CONSTRAINT photobook_ogp_images_failure_reason_len_check
        CHECK (failure_reason IS NULL OR char_length(failure_reason) <= 200),

    -- 状態と関連列の整合（最低限）
    CONSTRAINT photobook_ogp_images_status_columns_consistency_check
        CHECK (
            CASE status
                WHEN 'generated' THEN
                    image_id IS NOT NULL AND generated_at IS NOT NULL
                WHEN 'failed' THEN
                    failed_at IS NOT NULL
                ELSE TRUE
            END
        ),

    CONSTRAINT photobook_ogp_images_photobook_fk
        FOREIGN KEY (photobook_id) REFERENCES photobooks(id) ON DELETE CASCADE,

    CONSTRAINT photobook_ogp_images_image_fk
        FOREIGN KEY (image_id) REFERENCES images(id) ON DELETE SET NULL
);
-- +goose StatementEnd

-- Reconcile / バックフィル抽出用（status='stale' / 'failed' を updated_at 古い順）
CREATE INDEX photobook_ogp_images_status_updated_idx
    ON photobook_ogp_images (status, updated_at);

-- +goose Down
DROP TABLE photobook_ogp_images;
