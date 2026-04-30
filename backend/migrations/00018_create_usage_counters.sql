-- PR36: usage_counters（UsageLimit / RateLimit 用 fixed window バケット）。
--
-- 設計参照:
--   - docs/plan/m2-usage-limit-plan.md §6 / §18 ユーザー判断（A〜L 確定済）
--   - docs/spec/vrc_photobook_business_knowledge_v4.md §3.7（同一作成元 1 時間 5 冊）
--   - .agents/rules/turnstile-defensive-guard.md（Turnstile L0-L4 と直列に配置）
--
-- 重要な決定:
--   - 単一テーブル + fixed window 方式。Redis 等は導入しない（PR36 計画書 §6）。
--   - PRIMARY KEY (scope_type, scope_hash, action, window_start) で同窓内 1 行を保証。
--     INSERT ... ON CONFLICT DO UPDATE で race-free に increment（PR36 計画書 §6.3）。
--   - scope_hash は hex 文字列（source_ip_hash は salt+sha256 → 32 byte → 64 hex char）。
--     IP 生値は保存しない（業務知識 v4 §3.7 / .agents/rules/security-guard.md）。
--   - REPORT_IP_HASH_SALT_V1 を流用（PR36 計画書 §7 案 A、業務知識 v4
--     「UsageLimit と Report で IP ハッシュソルトを共有」）。
--   - limit_at_creation は INSERT 時点の閾値スナップショット。後日閾値変更しても
--     過去履歴を歪めないため保持する。判定は UseCase が現在閾値を参照して行う。
--   - retention は MVP 24h grace。期限切れ行は手動 cleanup SQL で削除（PR36 計画書 §11）。
--   - 自動 cleanup（Cloud Run Job / Scheduler）は本 PR 範囲外（後続 PR）。
--   - Outbox event は不要（同期 response 完結、PR36 計画書 §12）。
--
-- セキュリティ:
--   - scope_hash 完全値を logs / chat / work-log に出さない運用ガイド
--     （cmd/ops 出力は先頭 8 文字 prefix のみ）
--   - raw IP / raw token / Cookie / manage URL / storage_key は本テーブルに保存しない

-- +goose Up
-- +goose StatementBegin
CREATE TABLE usage_counters (
    scope_type        text        NOT NULL,
    scope_hash        text        NOT NULL,
    action            text        NOT NULL,
    window_start      timestamptz NOT NULL,
    window_seconds    integer     NOT NULL,
    count             integer     NOT NULL DEFAULT 0,
    limit_at_creation integer     NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    expires_at        timestamptz NOT NULL,

    CONSTRAINT usage_counters_pk
        PRIMARY KEY (scope_type, scope_hash, action, window_start),

    -- 計画書 §6.2 で限定した 4 種以外を許容しない（誤代入防止）
    CONSTRAINT usage_counters_scope_type_check
        CHECK (scope_type IN (
            'source_ip_hash',
            'draft_session_id',
            'manage_session_id',
            'photobook_id'
        )),

    -- 計画書 §4.2 で対象を絞った 3 種以外を許容しない（誤 action 投入防止）
    CONSTRAINT usage_counters_action_check
        CHECK (action IN (
            'report.submit',
            'upload_verification.issue',
            'publish.from_draft'
        )),

    CONSTRAINT usage_counters_window_seconds_check
        CHECK (window_seconds > 0),

    CONSTRAINT usage_counters_count_check
        CHECK (count >= 0),

    CONSTRAINT usage_counters_limit_at_creation_check
        CHECK (limit_at_creation > 0),

    -- expires_at は window_start + window_seconds 以降。retention grace 24h 以上を期待
    CONSTRAINT usage_counters_expires_at_after_window_check
        CHECK (expires_at > window_start)
);

-- 手動 cleanup / 期限切れスキャン用
CREATE INDEX usage_counters_expires_at_idx
    ON usage_counters (expires_at);

-- 運営調査（最近の高頻度 action を action × time order で見る）用
CREATE INDEX usage_counters_action_window_start_idx
    ON usage_counters (action, window_start DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS usage_counters;
-- +goose StatementEnd
