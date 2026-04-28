-- PR35b: reports（通報レコード、状態遷移あり）。
--
-- 設計参照:
--   - docs/design/aggregates/report/データモデル設計.md §3 / §4 / §5
--   - docs/design/aggregates/report/ドメイン設計.md
--   - docs/plan/m2-report-plan.md §4 / §15 ユーザー判断 #2
--   - 業務知識 v4 §3.6 / §3.7 / §7.2 / §7.3 / §7.4
--
-- 重要な決定:
--   - target_photobook_id / resolved_by_moderation_action_id に **FK 制約は付けない**
--     （v4 §3.6 P0-11: Photobook purge 後も通報証跡を残すため）
--   - target_*_snapshot 3 カラムを必須化（Photobook 物理削除後の文脈復元）
--   - reason CHECK は 6 種（minor_safety_concern を含む、v4 P0-10）
--   - status CHECK は 5 種（submitted / under_review / resolved_action_taken /
--     resolved_no_action / dismissed）
--   - source_ip_hash は bytea NULL（生 IP は保存しない、ソルト + sha256、UsageLimit と
--     同ソルトポリシー、計画書 §4.4 / §15 ユーザー判断 #3）
--   - reporter_contact ≤ 200 char、detail ≤ 2000 char
--   - 90 日後 NULL 化 reconciler は PR33e / PR39 系で別途実装

-- +goose Up
-- +goose StatementBegin
CREATE TABLE reports (
    id                                       uuid        NOT NULL DEFAULT gen_random_uuid(),
    target_photobook_id                      uuid        NOT NULL,
    target_public_url_snapshot               text        NOT NULL,
    target_title_snapshot                    text        NOT NULL,
    target_creator_display_name_snapshot     text        NULL,
    reason                                   text        NOT NULL,
    detail                                   text        NULL,
    reporter_contact                         text        NULL,
    status                                   text        NOT NULL DEFAULT 'submitted',
    submitted_at                             timestamptz NOT NULL DEFAULT now(),
    reviewed_at                              timestamptz NULL,
    resolved_at                              timestamptz NULL,
    resolution_note                          text        NULL,
    resolved_by_moderation_action_id         uuid        NULL,
    source_ip_hash                           bytea       NULL,

    CONSTRAINT reports_pk PRIMARY KEY (id),

    CONSTRAINT reports_reason_check
        CHECK (reason IN (
            'subject_removal_request',
            'unauthorized_repost',
            'sensitive_flag_missing',
            'harassment_or_doxxing',
            'minor_safety_concern',
            'other'
        )),

    CONSTRAINT reports_status_check
        CHECK (status IN (
            'submitted',
            'under_review',
            'resolved_action_taken',
            'resolved_no_action',
            'dismissed'
        )),

    -- snapshot は Photobook purge 後の証跡保持に必須。空文字も拒否。
    CONSTRAINT reports_target_public_url_snapshot_len_check
        CHECK (char_length(target_public_url_snapshot) BETWEEN 1 AND 100),
    CONSTRAINT reports_target_title_snapshot_len_check
        CHECK (char_length(target_title_snapshot) BETWEEN 1 AND 200),
    CONSTRAINT reports_target_creator_display_name_snapshot_len_check
        CHECK (target_creator_display_name_snapshot IS NULL OR char_length(target_creator_display_name_snapshot) <= 100),

    CONSTRAINT reports_detail_len_check
        CHECK (detail IS NULL OR char_length(detail) <= 2000),

    CONSTRAINT reports_reporter_contact_len_check
        CHECK (reporter_contact IS NULL OR char_length(reporter_contact) <= 200),

    -- resolved_by_moderation_action_id は status='resolved_action_taken' のときのみ非 NULL（v4 I8）
    CONSTRAINT reports_resolved_action_consistency_check
        CHECK (
            (status = 'resolved_action_taken' OR resolved_by_moderation_action_id IS NULL)
        ),

    -- resolved_at は終端状態（resolved_*, dismissed）のときのみ非 NULL
    CONSTRAINT reports_resolved_at_consistency_check
        CHECK (
            (status IN ('resolved_action_taken','resolved_no_action','dismissed'))
            OR resolved_at IS NULL
        )
);
-- +goose StatementEnd

-- 設計書 §4 の INDEX 6 種をそのまま採用。
-- Photobook 別の通報集計
CREATE INDEX reports_target_photobook_idx
    ON reports (target_photobook_id);

-- 未対応通報の最新一覧
CREATE INDEX reports_status_submitted_at_idx
    ON reports (status, submitted_at DESC);

-- カテゴリ別の集計
CREATE INDEX reports_reason_submitted_at_idx
    ON reports (reason, submitted_at DESC);

-- 未成年関連通報の最優先抽出（v4 §3.6 / §7.4）
CREATE INDEX reports_minor_safety_priority_idx
    ON reports (submitted_at)
    WHERE reason = 'minor_safety_concern' AND status IN ('submitted', 'under_review');

-- 同一 IP からの大量通報検出（UsageLimit と連携、v4 P1-4）
CREATE INDEX reports_source_ip_hash_idx
    ON reports (source_ip_hash, submitted_at)
    WHERE source_ip_hash IS NOT NULL;

-- Moderation 処置起因の通報逆引き
CREATE INDEX reports_resolved_by_moderation_action_idx
    ON reports (resolved_by_moderation_action_id)
    WHERE resolved_by_moderation_action_id IS NOT NULL;

-- +goose Down
DROP TABLE reports;
