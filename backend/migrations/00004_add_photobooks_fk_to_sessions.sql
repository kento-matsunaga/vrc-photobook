-- PR9a: sessions.photobook_id に photobooks(id) への FK を追加。
--
-- PR7 で意図的に保留した FK を、photobooks table が存在する本 PR で追加する。
--
-- 設計参照:
--   - docs/plan/m2-session-auth-implementation-plan.md §8.3
--   - docs/plan/m2-photobook-session-integration-plan.md §7.2
--
-- 注意:
--   - 既存 sessions 行が photobooks に存在しない photobook_id を持っている場合、FK 追加で失敗する
--   - ローカル開発で session が残っている場合は `docker compose down -v` でクリーンアップしてから up すること
--     （README §C-D 参照）

-- +goose Up
ALTER TABLE sessions
    ADD CONSTRAINT sessions_photobook_id_fkey
    FOREIGN KEY (photobook_id)
    REFERENCES photobooks (id)
    ON DELETE CASCADE;

-- +goose Down
ALTER TABLE sessions
    DROP CONSTRAINT sessions_photobook_id_fkey;
