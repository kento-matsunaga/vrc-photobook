-- PR35b: Report 集約の sqlc query。
--
-- 設計参照:
--   - docs/design/aggregates/report/データモデル設計.md §3 / §4
--   - docs/plan/m2-report-plan.md §5
--
-- セキュリティ:
--   - reporter_contact / detail / source_ip_hash は DB から読めるが、
--     呼び出し側（cmd/ops 出力ホワイトリスト）で表示制御する
--   - Outbox payload には reporter_contact / detail / source_ip_hash を入れない
--     （UseCase 側で payload を組み立てる時点で除外）
--   - source_ip_hash 完全値は外部応答 / log / chat に出さない（cmd/ops は先頭 4 byte のみ表示）

-- name: CreateReport :exec
-- 同一 TX 内（SubmitReport UseCase）で outbox_events INSERT と一緒に呼ばれる前提。
-- status='submitted' / submitted_at=$now を default で書く。
INSERT INTO reports (
    id,
    target_photobook_id,
    target_public_url_snapshot,
    target_title_snapshot,
    target_creator_display_name_snapshot,
    reason,
    detail,
    reporter_contact,
    status,
    submitted_at,
    source_ip_hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, 'submitted', $9, $10
);

-- name: GetReportByID :one
SELECT
    id, target_photobook_id,
    target_public_url_snapshot,
    target_title_snapshot,
    target_creator_display_name_snapshot,
    reason, detail, reporter_contact, status,
    submitted_at, reviewed_at, resolved_at, resolution_note,
    resolved_by_moderation_action_id, source_ip_hash
FROM reports
WHERE id = $1;

-- name: ListReports :many
-- cmd/ops report list 用。status / reason フィルタ + minor_safety_concern 優先。
-- minor_safety_concern を最優先で表示し、次に未対応（submitted）を新しい順、
-- 残りは submitted_at DESC。$1 status filter（''=ALL）/ $2 reason filter（''=ALL）
-- / $3 limit / $4 offset。
SELECT
    id, target_photobook_id,
    target_public_url_snapshot,
    target_title_snapshot,
    target_creator_display_name_snapshot,
    reason, detail, reporter_contact, status,
    submitted_at, reviewed_at, resolved_at, resolution_note,
    resolved_by_moderation_action_id, source_ip_hash
FROM reports
WHERE ($1::text = '' OR status = $1::text)
  AND ($2::text = '' OR reason = $2::text)
ORDER BY
    CASE WHEN reason = 'minor_safety_concern' AND status IN ('submitted','under_review') THEN 0
         WHEN status IN ('submitted','under_review') THEN 1
         ELSE 2
    END ASC,
    submitted_at DESC
LIMIT $3
OFFSET $4;

-- name: MarkReportResolvedActionTaken :execrows
-- Moderation hide --source-report-id 連動用。
-- 対象 status は submitted / under_review のみ（終端状態は再オープンしない、I4）。
-- 0 行 UPDATE は呼び出し側で「既に終端 / 不在」として error 化。
UPDATE reports
   SET status                            = 'resolved_action_taken',
       resolved_by_moderation_action_id  = sqlc.arg(moderation_action_id),
       resolved_at                       = sqlc.arg(resolved_at)
 WHERE id = sqlc.arg(id)
   AND status IN ('submitted', 'under_review');
