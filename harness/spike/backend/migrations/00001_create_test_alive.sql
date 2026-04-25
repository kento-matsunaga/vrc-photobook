-- M1 spike: 最小 migration の動作確認用テーブル。
-- 本実装で利用するテーブルではない（PoC 専用）。

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS _test_alive (
    id         BIGSERIAL    PRIMARY KEY,
    note       TEXT         NOT NULL DEFAULT 'M1 spike alive',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS _test_alive;
-- +goose StatementEnd
