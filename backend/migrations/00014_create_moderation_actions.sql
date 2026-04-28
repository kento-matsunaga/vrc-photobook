-- PR34b: moderation_actions（運営アクション履歴、append-only）。
--
-- 設計参照:
--   - docs/design/aggregates/moderation/データモデル設計.md §3 / §5 / §6
--   - docs/design/aggregates/moderation/ドメイン設計.md
--   - docs/plan/m2-moderation-ops-plan.md §4 案 B / ユーザー判断事項 #1〜#3
--   - 業務知識 v4 §5.4 / §6.19 / §7.3 / §7.4
--
-- 重要な決定:
--   - append-only。アプリ層で UPDATE / DELETE 文を発行しない
--     （DB trigger による強制は採用しない、計画書 §4.2 ユーザー判断 #2）
--   - target_photobook_id / source_report_id に **FK 制約は付けない**
--     （v4 設計書 §4。purge 後も監査証跡として残す必要があるため）
--   - kind / reason CHECK は v4 設計書通り 6 種 / 9 種を最初から受け入れる
--     （MVP 運用は hide / unhide のみだが、後続 PR で migration 不要にする）
--   - actor_label は個人情報を含まない運営内識別子（v4 §4.3 の正規表現はアプリ層 VO で強制）

-- +goose Up
-- +goose StatementBegin
CREATE TABLE moderation_actions (
    id                    uuid        NOT NULL DEFAULT gen_random_uuid(),
    kind                  text        NOT NULL,
    target_photobook_id   uuid        NOT NULL,
    source_report_id      uuid        NULL,
    actor_label           text        NOT NULL,
    reason                text        NOT NULL,
    detail                text        NULL,
    correlation_id        uuid        NULL,
    executed_at           timestamptz NOT NULL DEFAULT now(),
    created_at            timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT moderation_actions_pk PRIMARY KEY (id),

    CONSTRAINT moderation_actions_kind_check
        CHECK (kind IN (
            'hide',
            'unhide',
            'soft_delete',
            'restore',
            'purge',
            'reissue_manage_url'
        )),

    CONSTRAINT moderation_actions_reason_check
        CHECK (reason IN (
            'report_based_harassment',
            'report_based_unauthorized_repost',
            'report_based_sensitive_violation',
            'report_based_minor_related',
            'report_based_subject_removal',
            'rights_claim',
            'creator_request_manage_url_reissue',
            'erroneous_action_correction',
            'policy_violation_other'
        )),

    -- detail は内部参照用、≤ 2000 char
    CONSTRAINT moderation_actions_detail_len_check
        CHECK (detail IS NULL OR char_length(detail) <= 2000),

    -- actor_label 長さガード（VO の正規表現はアプリ層、ここは下限 / 上限のみ）
    CONSTRAINT moderation_actions_actor_label_len_check
        CHECK (char_length(actor_label) BETWEEN 3 AND 64)
);
-- +goose StatementEnd

-- 設計書 §5 の INDEX 6 種をそのまま採用。
-- 特定 photobook の操作履歴（最新順）。
CREATE INDEX moderation_actions_target_executed_idx
    ON moderation_actions (target_photobook_id, executed_at DESC);

-- 通報からの逆引き。
CREATE INDEX moderation_actions_source_report_idx
    ON moderation_actions (source_report_id)
    WHERE source_report_id IS NOT NULL;

-- アクション種別別の集計。
CREATE INDEX moderation_actions_kind_executed_idx
    ON moderation_actions (kind, executed_at DESC);

-- 運営の活動ログ。
CREATE INDEX moderation_actions_actor_executed_idx
    ON moderation_actions (actor_label, executed_at DESC);

-- 理由別の集計（report_based_minor_related の頻度監視等）。
CREATE INDEX moderation_actions_reason_executed_idx
    ON moderation_actions (reason, executed_at DESC);

-- ペア参照（hide ↔ unhide 等）。
CREATE INDEX moderation_actions_correlation_idx
    ON moderation_actions (correlation_id)
    WHERE correlation_id IS NOT NULL;

-- +goose Down
DROP TABLE moderation_actions;
