-- M1 spike: outbox_events（横断 Outbox / 自動 reconciler PoC、付録C P0-13/15/27/28/29）
-- 本実装では cross-cutting/outbox.md の正式スキーマで再整備する（PoC では最小カラムのみ）。
--
-- 設計上の対応:
--   - cross-cutting/outbox.md §3.1 のカラム定義に整合
--   - PoC では「retry_count → attempts」「next_retry_at → next_attempt_at」「failure_reason → last_error」
--     とユーザー指示の命名を採用（本実装では設計書命名に揃える）

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS outbox_events (
    id               uuid        PRIMARY KEY,
    event_type       text        NOT NULL,
    aggregate_type   text        NOT NULL,
    aggregate_id     uuid        NOT NULL,
    payload          jsonb       NOT NULL DEFAULT '{}'::jsonb,
    status           text        NOT NULL DEFAULT 'pending',
    attempts         int         NOT NULL DEFAULT 0,
    next_attempt_at  timestamptz NOT NULL DEFAULT now(),
    last_error       text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    processed_at     timestamptz,
    locked_at        timestamptz,
    CHECK (status IN ('pending', 'processing', 'processed', 'failed')),
    CHECK (attempts >= 0),
    CHECK (event_type <> ''),
    CHECK (aggregate_type <> ''),
    CHECK (
        (status = 'processed' AND processed_at IS NOT NULL)
        OR (status <> 'processed' AND processed_at IS NULL)
    )
);
-- +goose StatementEnd

-- ピック対象（pending かつ next_attempt_at 到達済）の高速抽出。
-- FOR UPDATE SKIP LOCKED と組み合わせて並列ワーカーが衝突しない設計。
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS outbox_events_pending_idx
    ON outbox_events (next_attempt_at, created_at)
    WHERE status = 'pending';
-- +goose StatementEnd

-- failed 抽出（reconcile / 監視用）。
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS outbox_events_failed_idx
    ON outbox_events (created_at DESC)
    WHERE status = 'failed';
-- +goose StatementEnd

-- 集約別履歴参照（イベントトレース / 障害調査）。
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS outbox_events_aggregate_idx
    ON outbox_events (aggregate_type, aggregate_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS outbox_events;
-- +goose StatementEnd
