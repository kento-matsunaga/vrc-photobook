-- PR34b: outbox_events.event_type CHECK を緩めて photobook.hidden / photobook.unhidden を許容。
--
-- 設計参照:
--   - docs/plan/m2-moderation-ops-plan.md §7.4 / ユーザー判断事項 #9
--   - docs/design/aggregates/moderation/ドメイン設計.md §7
--
-- 重要な決定:
--   - moderation 同 TX INSERT で使う（worker handler は no-op + log）
--   - PR30 で 3 種に絞ったまま維持し、本 migration で 5 種に拡張
--   - PR34b 範囲では soft_delete / restore / purge / reissue_manage_url 系 event は追加しない
--     （後続 PR で UseCase が増えたタイミングで migration を増やす）

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
            'image.failed'
        ));

-- +goose Down
ALTER TABLE outbox_events
    DROP CONSTRAINT outbox_events_event_type_check;

ALTER TABLE outbox_events
    ADD CONSTRAINT outbox_events_event_type_check
        CHECK (event_type IN (
            'photobook.published',
            'image.became_available',
            'image.failed'
        ));
