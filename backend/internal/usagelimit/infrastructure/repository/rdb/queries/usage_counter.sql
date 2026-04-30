-- PR36: UsageLimit / RateLimit 集約の sqlc query。
--
-- 設計参照:
--   - docs/plan/m2-usage-limit-plan.md §6 / §8
--   - migrations/00018_create_usage_counters.sql
--
-- セキュリティ:
--   - scope_hash 完全値を logs / chat / work-log に出さない（cmd/ops は redact 表示）
--   - 本 query で参照する scope_hash / IP / token は呼び出し側が VO 経由で渡す前提
--   - エラー時も raw scope_hash を error message に出さない（呼び出し側で redact）

-- name: UpsertAndIncrementCounter :one
-- INSERT ... ON CONFLICT DO UPDATE で race-free に atomic increment する。
-- 同 (scope_type, scope_hash, action, window_start) の行が無ければ count=1 で INSERT、
-- あれば既存 row の count を +1 して updated_at を now() に更新。
-- RETURNING で increment 後の count + limit_at_creation を返す。
--
-- 引数:
--   $1: scope_type
--   $2: scope_hash
--   $3: action
--   $4: window_start (時刻、UseCase が Window.StartFor(now) で算出)
--   $5: window_seconds
--   $6: limit_at_creation (INSERT 時点の閾値スナップショット、後の判定は呼び出し側が
--                          現行 limit と比較)
--   $7: expires_at (UseCase が window_end + retention_grace で算出)
--   $8: now (created_at / updated_at の値)
INSERT INTO usage_counters (
    scope_type,
    scope_hash,
    action,
    window_start,
    window_seconds,
    count,
    limit_at_creation,
    created_at,
    updated_at,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, 1, $6, $8, $8, $7
)
ON CONFLICT (scope_type, scope_hash, action, window_start)
DO UPDATE
   SET count      = usage_counters.count + 1,
       updated_at = $8
RETURNING
    count,
    limit_at_creation,
    window_start,
    window_seconds,
    expires_at;

-- name: GetUsageCounter :one
-- 指定 (scope_type, scope_hash, action, window_start) の現在 count を取得（読み取り）。
-- cmd/ops show 用。
SELECT
    scope_type,
    scope_hash,
    action,
    window_start,
    window_seconds,
    count,
    limit_at_creation,
    created_at,
    updated_at,
    expires_at
  FROM usage_counters
 WHERE scope_type = $1
   AND scope_hash = $2
   AND action     = $3
   AND window_start = $4;

-- name: ListUsageCountersByPrefix :many
-- scope_hash の prefix（先頭 N 文字）で検索。cmd/ops list 用、redact 検索を可能にする。
-- prefix は LIKE 'prefix%' で前方一致。最大 limit 件、created_at DESC で返す。
--
-- 引数:
--   $1: scope_type (空文字 '' なら全 scope_type)
--   $2: scope_hash_prefix (LIKE 'PREFIX%' 用、'%' は呼び出し側で付ける)
--   $3: action (空文字 '' なら全 action)
--   $4: limit
--   $5: offset
SELECT
    scope_type,
    scope_hash,
    action,
    window_start,
    window_seconds,
    count,
    limit_at_creation,
    created_at,
    updated_at,
    expires_at
  FROM usage_counters
 WHERE ($1::text = '' OR scope_type = $1)
   AND ($2::text = '' OR scope_hash LIKE $2)
   AND ($3::text = '' OR action     = $3)
 ORDER BY window_start DESC, scope_hash ASC
 LIMIT  $4
 OFFSET $5;

-- name: DeleteExpiredUsageCounters :execrows
-- 期限切れ行を削除する。本 PR36 MVP では cmd/ops cleanup --execute は実装しないため、
-- 主に Backend test のセットアップ / runbook に直接記載する手動 SQL の cleanup が
-- 利用先になる（PR36 計画書 §11）。
DELETE FROM usage_counters
 WHERE expires_at < $1;
