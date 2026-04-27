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

-- ----------------------------------------------------------------------------
-- worker 用 query（PR31 で追加）。
-- ----------------------------------------------------------------------------
--
-- 並列 worker 安全性:
--   ListPendingForUpdate は FOR UPDATE SKIP LOCKED で他 worker が見ている行を
--   避けて取り出す。short TX で SELECT → UPDATE→ COMMIT し、lock を即解放したあとに
--   handler を実行する。handler 中の long-running operation は DB lock を保持しない。
--
-- status 遷移は worker 内でのみ実施:
--   pending → processing （MarkProcessingByIDs）
--   processing → processed （MarkProcessed）
--   processing → failed / dead （MarkFailed / MarkDead）
--   processing → pending （ReleaseStaleLocks: locked_at が古いものを救出）
--   failed → pending（pickup の status IN ('pending','failed') で自動 retry 候補）

-- name: ListPendingOutboxEventsForUpdate :many
-- pickup query。status='pending' / 'failed' のうち available_at <= $1（worker 起動時刻）
-- のものを古い順で $2 件まで取り出す。FOR UPDATE SKIP LOCKED で並列 worker と
-- 衝突しない。
SELECT
    id, aggregate_type, aggregate_id, event_type, payload,
    status, available_at, attempts, last_error,
    locked_at, locked_by, processed_at, created_at, updated_at
FROM outbox_events
WHERE status IN ('pending', 'failed')
  AND available_at <= $1
ORDER BY available_at ASC, created_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxProcessingByIDs :exec
-- claim TX で複数行をまとめて processing に遷移させる。
-- locked_by は worker 識別子（hostname-pid-... 等）。
UPDATE outbox_events
SET
    status     = 'processing',
    locked_at  = $2,
    locked_by  = $3,
    updated_at = $2
WHERE id = ANY($1::uuid[])
  AND status IN ('pending', 'failed');

-- name: MarkOutboxProcessed :exec
-- handler 成功時。processed_at NOT NULL（CHECK 制約）。
UPDATE outbox_events
SET
    status       = 'processed',
    processed_at = $2,
    updated_at   = $2,
    locked_at    = NULL,
    locked_by    = NULL,
    last_error   = NULL
WHERE id = $1
  AND status = 'processing';

-- name: MarkOutboxFailedRetry :exec
-- handler 失敗 + retry 余地あり。status='failed' に戻し、available_at を後ろ倒し。
-- attempts++、last_error を sanitize 済の文字列で保存（200 char 上限は呼び出し側で担保）。
UPDATE outbox_events
SET
    status       = 'failed',
    attempts     = attempts + 1,
    last_error   = $2,
    available_at = $3,
    updated_at   = $4,
    locked_at    = NULL,
    locked_by    = NULL
WHERE id = $1
  AND status = 'processing';

-- name: MarkOutboxDead :exec
-- handler 失敗 + retry 上限到達。status='dead' に固定。available_at は触らない。
UPDATE outbox_events
SET
    status     = 'dead',
    attempts   = attempts + 1,
    last_error = $2,
    updated_at = $3,
    locked_at  = NULL,
    locked_by  = NULL
WHERE id = $1
  AND status = 'processing';

-- name: ReleaseStaleOutboxLocks :execrows
-- worker crash 等で processing のまま残った行を pending に戻す。
-- threshold は呼び出し側が指定（now - timeout）。
-- 戻り値は影響行数（exec が rows affected を返す）。
UPDATE outbox_events
SET
    status     = 'pending',
    locked_at  = NULL,
    locked_by  = NULL,
    updated_at = $2
WHERE status = 'processing'
  AND locked_at IS NOT NULL
  AND locked_at < $1;

-- name: FindOutboxEventByID :one
-- test / inspector 用。
SELECT
    id, aggregate_type, aggregate_id, event_type, payload,
    status, available_at, attempts, last_error,
    locked_at, locked_by, processed_at, created_at, updated_at
FROM outbox_events
WHERE id = $1;
