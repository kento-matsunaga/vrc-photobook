-- PR35b: outbox_events.event_type CHECK を緩めて report.submitted を許容。
--
-- 設計参照:
--   - docs/plan/m2-report-plan.md §12 / §15 ユーザー判断 #10
--   - docs/design/aggregates/report/ドメイン設計.md §7
--   - docs/design/cross-cutting/outbox.md §4.2
--
-- 重要な決定:
--   - SubmitReport UseCase が同一 TX で reports INSERT + outbox_events INSERT を行う
--   - worker handler は no-op + structured log（minor_safety_concern は Warn 以上）
--   - Email / Slack 通知は ADR-0006 後続（PR32c 以降）
--   - 既存 5 種（photobook.published / photobook.hidden / photobook.unhidden /
--     image.became_available / image.failed）に report.submitted を追加して 6 種に

-- +goose Up
ALTER TABLE outbox_events
    DROP CONSTRAINT outbox_events_event_type_check;

ALTER TABLE outbox_events
    ADD CONSTRAINT outbox_events_event_type_check
        CHECK (event_type IN (
            'photobook.published',
            'photobook.hidden',
            'photobook.unhidden',
            'image.became_available',
            'image.failed',
            'report.submitted'
        ));

-- +goose Down
ALTER TABLE outbox_events
    DROP CONSTRAINT outbox_events_event_type_check;

ALTER TABLE outbox_events
    ADD CONSTRAINT outbox_events_event_type_check
        CHECK (event_type IN (
            'photobook.published',
            'photobook.hidden',
            'photobook.unhidden',
            'image.became_available',
            'image.failed'
        ));
