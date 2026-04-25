-- M1 spike: outbox_events の最小 sqlc クエリ。
-- 本実装では outbox 横断モジュールの Repository として再整備する。

-- name: CreateOutboxEvent :one
-- 集約の状態変更と同一 TX で呼び出す前提（PoC では sandbox 経由で単独 INSERT）。
INSERT INTO outbox_events (
    id,
    event_type,
    aggregate_type,
    aggregate_id,
    payload,
    status,
    attempts,
    next_attempt_at
) VALUES (
    $1, $2, $3, $4, $5, 'pending', 0, now()
)
RETURNING id, event_type, aggregate_type, aggregate_id, status,
          attempts, next_attempt_at, created_at;

-- name: ClaimPendingOutboxEvents :many
-- pending かつ next_attempt_at 到達済のイベントを最大 $1 件取得して
-- アトミックに status='processing' に遷移する。
-- FOR UPDATE SKIP LOCKED によって複数ワーカーが同じ行を取得しない。
WITH claimed AS (
    SELECT id
    FROM outbox_events
    WHERE status = 'pending'
      AND next_attempt_at <= now()
    ORDER BY next_attempt_at, created_at
    FOR UPDATE SKIP LOCKED
    LIMIT $1
)
UPDATE outbox_events o
SET status     = 'processing',
    attempts   = attempts + 1,
    locked_at  = now()
FROM claimed
WHERE o.id = claimed.id
RETURNING o.id, o.event_type, o.aggregate_type, o.aggregate_id,
          o.payload, o.attempts, o.created_at;

-- name: MarkOutboxProcessed :exec
-- ハンドラ成功時、processed に遷移する。
UPDATE outbox_events
SET status       = 'processed',
    processed_at = now(),
    last_error   = NULL,
    locked_at    = NULL
WHERE id = $1
  AND status = 'processing';

-- name: MarkOutboxFailed :exec
-- ハンドラ失敗時、status='failed' に遷移する（PoC では指数バックオフを導入せず
-- ターミナル failed に集約。本実装では attempts に応じて pending 戻し or failed を選ぶ）。
UPDATE outbox_events
SET status     = 'failed',
    last_error = $2,
    locked_at  = NULL
WHERE id = $1
  AND status = 'processing';

-- name: RetryFailedOutboxEvents :execrows
-- 自動 reconciler `outbox_failed_retry` の最小実装（cross-cutting/reconcile-scripts.md §3.7.2）。
-- failed 状態のイベントを pending に戻し、再度 worker が拾えるようにする。
-- 戻り値は影響行数（再投入したイベント数）。
UPDATE outbox_events
SET status          = 'pending',
    next_attempt_at = now(),
    locked_at       = NULL
WHERE status = 'failed';

-- name: ListOutboxEvents :many
-- 検証用の最小一覧。payload / last_error はクライアントへ返さない（情報量を絞る）。
SELECT id, event_type, aggregate_type, aggregate_id, status,
       attempts, created_at, processed_at
FROM outbox_events
ORDER BY created_at DESC
LIMIT $1;

-- name: CountOutboxEventsByStatus :many
-- 検証用の status 別集計。一覧と組み合わせて使う。
SELECT status, COUNT(*) AS count
FROM outbox_events
GROUP BY status
ORDER BY status;

-- name: ResetOutboxForTest :exec
-- PoC 検証用: outbox_events を全件削除する。
-- 本実装には流用しない（運用 API は outbox_failed.sh などの reconcile を使う）。
DELETE FROM outbox_events;
