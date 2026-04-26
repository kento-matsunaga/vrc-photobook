-- PR19: photobooks.cover_image_id に images(id) への FK を追加。
--
-- 設計参照:
--   - docs/design/aggregates/photobook/データモデル設計.md §3.2
--   - docs/plan/m2-photobook-image-connection-plan.md §3.5
--
-- 重要な決定:
--   - ON DELETE SET NULL: Image 集約側の孤児 GC と整合。Image 削除で Photobook を
--     巻き込まない。通常運用では owner_photobook_id 縛りで Photobook 削除時に
--     連鎖削除されるため、SET NULL に到達するのは異常系のみ。
--   - PR9a で photobooks テーブル作成時に cover_image_id 列は uuid NULL で既に追加済。
--     本 migration は FK 制約のみを追加する。

-- +goose Up
ALTER TABLE photobooks
    ADD CONSTRAINT photobooks_cover_image_id_fkey
    FOREIGN KEY (cover_image_id)
    REFERENCES images (id)
    ON DELETE SET NULL;

-- +goose Down
ALTER TABLE photobooks
    DROP CONSTRAINT photobooks_cover_image_id_fkey;
