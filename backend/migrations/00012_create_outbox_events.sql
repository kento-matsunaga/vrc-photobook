-- PR30: Outbox events table。
--
-- 設計参照:
--   - docs/plan/m2-outbox-plan.md
--   - docs/design/cross-cutting/outbox.md（v4 業務正典）
--   - docs/adr/0006-email-provider-and-manage-url-delivery.md
--     （Email Provider 依存 event は本 PR では追加しない）
--
-- 重要な決定:
--   - 集約の状態変更と同一 TX で INSERT する Transactional Outbox パターン
--   - PR30 で実体投入する event_type は 3 種に絞る:
--     photobook.published / image.became_available / image.failed
--     後続 PR で event_type CHECK を migration で緩める
--   - status は pending / processing / processed / failed / dead の 5 値
--     PR30 では pending 作成のみ実施、worker（PR31）が他 status へ遷移
--   - aggregate_type は将来拡張余地として 5 種を許可
--   - locked_at / locked_by は PR31 worker が SKIP LOCKED の代替として使う
--     stuck row 検出（reset by ReleaseStaleLocks）に使う
--   - last_error は PR31 worker が sanitize して書き込む（200 char 上限、
--     DATABASE_URL / R2_*= / token パターンを redact、ADR-0006 後続でも維持）
--   - payload は jsonb。token / Cookie / presigned URL / storage_key 完全値 /
--     R2 credentials / DATABASE_URL / email address は入れない（plan §5.5）

-- +goose Up
-- +goose StatementBegin
CREATE TABLE outbox_events (
    id              uuid        NOT NULL DEFAULT gen_random_uuid(),
    aggregate_type  text        NOT NULL,
    aggregate_id    uuid        NOT NULL,
    event_type      text        NOT NULL,
    payload         jsonb       NOT NULL DEFAULT '{}'::jsonb,
    status          text        NOT NULL DEFAULT 'pending',
    available_at    timestamptz NOT NULL DEFAULT now(),
    attempts        int         NOT NULL DEFAULT 0,
    last_error      text        NULL,
    locked_at       timestamptz NULL,
    locked_by       text        NULL,
    processed_at    timestamptz NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT outbox_events_pk PRIMARY KEY (id),

    CONSTRAINT outbox_events_aggregate_type_check
        CHECK (aggregate_type IN (
            'photobook',
            'image',
            'report',
            'moderation',
            'manage_url_delivery'
        )),

    -- PR30 では event_type を 3 種に絞り、誤投入を CHECK で防ぐ。
    -- 後続 PR で event を追加する都度、migration で CHECK を緩める。
    CONSTRAINT outbox_events_event_type_check
        CHECK (event_type IN (
            'photobook.published',
            'image.became_available',
            'image.failed'
        )),

    CONSTRAINT outbox_events_status_check
        CHECK (status IN ('pending', 'processing', 'processed', 'failed', 'dead')),

    CONSTRAINT outbox_events_attempts_check
        CHECK (attempts >= 0),

    -- last_error は worker が sanitize して書く前提で 200 char 上限を強制
    CONSTRAINT outbox_events_last_error_len_check
        CHECK (last_error IS NULL OR char_length(last_error) <= 200),

    -- payload は object（top-level array / scalar を禁止、worker の dispatch を単純化）
    CONSTRAINT outbox_events_payload_object_check
        CHECK (jsonb_typeof(payload) = 'object'),

    -- 状態と関連列の整合
    CONSTRAINT outbox_events_status_columns_consistency_check
        CHECK (
            CASE status
                WHEN 'pending' THEN
                    locked_at IS NULL
                    AND locked_by IS NULL
                    AND processed_at IS NULL
                WHEN 'processing' THEN
                    locked_at IS NOT NULL
                    AND locked_by IS NOT NULL
                    AND processed_at IS NULL
                WHEN 'processed' THEN
                    processed_at IS NOT NULL
                WHEN 'failed' THEN
                    locked_at IS NULL
                    AND locked_by IS NULL
                WHEN 'dead' THEN
                    last_error IS NOT NULL
                ELSE TRUE
            END
        )
);
-- +goose StatementEnd

-- worker pick 用（pending / failed の available_at >= now() を順次ピック）
-- PR31 で FOR UPDATE SKIP LOCKED と組み合わせる。
CREATE INDEX outbox_events_pickup_idx
    ON outbox_events (status, available_at)
    WHERE status IN ('pending', 'failed');

-- 集約別履歴（運用調査 / debug 用）
CREATE INDEX outbox_events_aggregate_idx
    ON outbox_events (aggregate_type, aggregate_id, created_at DESC);

-- 種別 + status の集計
CREATE INDEX outbox_events_event_type_status_idx
    ON outbox_events (event_type, status);

-- failed / dead の Reconcile 抽出
CREATE INDEX outbox_events_failed_idx
    ON outbox_events (status, processed_at)
    WHERE status IN ('failed', 'dead');

-- locked_at で stuck 検出（PR31 worker が ReleaseStaleLocks で使う）
CREATE INDEX outbox_events_locked_at_idx
    ON outbox_events (locked_at)
    WHERE locked_at IS NOT NULL;

-- +goose Down
DROP TABLE outbox_events;
