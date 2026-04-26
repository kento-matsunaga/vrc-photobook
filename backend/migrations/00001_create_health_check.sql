-- +goose Up
-- PR3 基盤確認用の最小テーブル。
-- 目的:
--   - goose の up / down が成立することの確認
--   - sqlc の最小 query が通ることの確認
-- 後続 PR で集約 DDL を追加する際に整理予定（M2 早期 implementation-bootstrap-plan §4）。

CREATE TABLE IF NOT EXISTS _health_check (
    id          INTEGER PRIMARY KEY,
    checked_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS _health_check;
