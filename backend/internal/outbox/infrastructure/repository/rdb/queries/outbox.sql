-- PR30: Outbox events の sqlc query。
--
-- 設計参照:
--   - docs/plan/m2-outbox-plan.md §7
--   - docs/design/cross-cutting/outbox.md
--
-- セキュリティ:
--   - payload に Secret / token / Cookie / presigned URL / storage_key 完全値を入れない
--     （application 層の VO で担保、Repository は jsonb をそのまま受ける）
--   - last_error は CHECK 制約で 200 char 上限。PR31 worker が sanitize して書く

-- name: CreateOutboxEvent :exec
-- 同一 TX で集約状態更新と一緒に呼び出される前提。
-- status='pending' / available_at=now() / attempts=0 を default で書く。
INSERT INTO outbox_events (
    id,
    aggregate_type,
    aggregate_id,
    event_type,
    payload,
    status,
    available_at,
    attempts,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    'pending', $6, 0, $7, $7
);

-- worker 用 query（PR31 で追加予定）:
--   - ListPendingOutboxEventsForUpdate（FOR UPDATE SKIP LOCKED）
--   - MarkOutboxProcessing（locked_at / locked_by 設定）
--   - MarkOutboxProcessed（status='processed' + processed_at）
--   - MarkOutboxFailed（attempts++ + status='failed' / 'dead' + last_error）
--   - BumpOutboxRetry（available_at = now() + backoff）
--   - ReleaseStaleLocks（locked_at < threshold で status='pending' に戻す）
--
-- PR30 では CreateOutboxEvent のみで完結する。
