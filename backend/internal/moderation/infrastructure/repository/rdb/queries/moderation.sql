-- PR34b: Moderation Action 集約の sqlc query。
--
-- 設計参照:
--   - docs/design/aggregates/moderation/データモデル設計.md §3 / §5 / §8
--   - docs/plan/m2-moderation-ops-plan.md §5
--
-- セキュリティ:
--   - detail に個人情報を書かない運用ガイド（runbook）。
--     Repository は単純に書き込むだけで sanitize しない（書き手側の責務）。
--   - actor_label は VO 側で正規表現バリデーション済（個人情報を含めない設計）。
--
-- 重要:
--   - moderation_actions は **append-only**。本 file に UPDATE / DELETE は書かない。

-- name: CreateModerationAction :exec
-- 同一 TX 内（hide/unhide UseCase 経由）で photobooks UPDATE と outbox_events INSERT
-- と一緒に呼び出される前提。
INSERT INTO moderation_actions (
    id,
    kind,
    target_photobook_id,
    source_report_id,
    actor_label,
    reason,
    detail,
    correlation_id,
    executed_at,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $9
);

-- name: ListModerationActionsByPhotobook :many
-- 特定 photobook の操作履歴（最新順）。GetPhotobookForOps の直近 ≤ N 件取得に使う。
-- INDEX moderation_actions_target_executed_idx を使う。
SELECT
    id,
    kind,
    target_photobook_id,
    source_report_id,
    actor_label,
    reason,
    detail,
    correlation_id,
    executed_at,
    created_at
FROM moderation_actions
WHERE target_photobook_id = $1
ORDER BY executed_at DESC
LIMIT $2;
