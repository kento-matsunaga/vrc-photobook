# vrc_photobook

VRChat 向けフォトブックサービス。**スマホファースト / ログイン不要 / 管理 URL 方式**。
ハーネスエンジニアリング（Spec → Implement → Verify → Feedback）で開発する。

## 現在地（2026-04-27）

**M2 中盤**。PR12〜PR23 完了（独自ドメイン / Cookie session / Image upload pipeline /
image-processor / display + thumbnail variant 生成）。次は PR24（公開 Viewer / 管理ページ計画書）。

> **新しい PR / サイクルに着手する前に必ず最初に確認するロードマップ（新正典）**:
> [`docs/plan/vrc-photobook-final-roadmap.md`](./docs/plan/vrc-photobook-final-roadmap.md)
>
> PR 番号体系・現在地マーカー・各 PR の必須項目・実リソース操作・Safari 確認要否・
> 課金判断ポイントは本書に集約する。旧ロードマップ
> [`harness/work-logs/2026-04-27_post-deploy-final-roadmap.md`](./harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)
> は PR12〜PR23 の進行記録としてのみ参照（PR24 以降は新正典を優先）。

過去のロードマップ（M1 完了時点）: [`harness/work-logs/2026-04-26_project-roadmap-overview.md`](./harness/work-logs/2026-04-26_project-roadmap-overview.md)

> Backend deploy は **Cloud Build manual trigger** を経由する（PR29）。
> 手順は [`docs/runbook/backend-deploy.md`](./docs/runbook/backend-deploy.md)。
> 旧手動 `docker build` → `gcloud run services update` 経路は緊急時のみ。

## 一目で見るリンクハブ

| 何を知りたいか | 参照先 |
|---|---|
| 業務知識（最上位の正典、矛盾時はこれが正） | [`docs/spec/vrc_photobook_business_knowledge_v4.md`](./docs/spec/vrc_photobook_business_knowledge_v4.md) |
| 技術スタック / 認可 / 画像 / メールの確定事項 | [`docs/adr/`](./docs/adr/) (ADR-0001〜0005) |
| 集約一覧（Photobook / Image / Report / UsageLimit / ManageUrlDelivery / Moderation） | [`docs/design/aggregates/README.md`](./docs/design/aggregates/README.md) |
| 横断設計（Outbox / Reconcile / OGP） | [`docs/design/cross-cutting/`](./docs/design/cross-cutting/) |
| 認可機構（Cookie session / upload-verification） | [`docs/design/auth/README.md`](./docs/design/auth/README.md) |
| v3→v4 変更点と P0/P1 達成チェック | [`docs/design/v4-change-summary.md`](./docs/design/v4-change-summary.md) |
| M1 計画 / 実環境デプロイ計画 / 費用ガード | [`docs/plan/`](./docs/plan/) |
| M1 完了判定 / 検証ログ | [`harness/work-logs/`](./harness/work-logs/) |
| 過去の失敗事例（再発させない）| [`harness/failure-log/`](./harness/failure-log/) |
| 品質スコア | [`harness/QUALITY_SCORE.md`](./harness/QUALITY_SCORE.md) |
| コード ↔ docs 対応 | [`docs/ディレクトリマッピング.md`](./docs/ディレクトリマッピング.md) |

## 守るべきルール（必読、`.agents/rules/`）

- [`coding-rules.md`](./.agents/rules/coding-rules.md) — 明示的 > 暗黙的、`any` / `interface{}` 禁止
- [`domain-standard.md`](./.agents/rules/domain-standard.md) — 集約 / VO / Repository のディレクトリ構造
- [`testing.md`](./.agents/rules/testing.md) — テーブル駆動 + Builder + `description` 必須
- [`security-guard.md`](./.agents/rules/security-guard.md) — Secret / Cookie / 認可
- [`safari-verification.md`](./.agents/rules/safari-verification.md) — Cookie / redirect / OGP / ヘッダ変更時の Safari 必須確認
- [`feedback-loop.md`](./.agents/rules/feedback-loop.md) — すべての失敗を `harness/failure-log/` に記録
- [`wsl-shell-rules.md`](./.agents/rules/wsl-shell-rules.md) — `cd` 不使用 / `-C` / `-f` / 絶対パス、sudo は対話シェルで

## コアコンセプト

```
Spec → Implement → Verify → Feedback
```

> **すべての失敗は再発を防止するルールまたはスキルになる。**

詳細手順: [`.agents/rules/feedback-loop.md`](./.agents/rules/feedback-loop.md)

## ディレクトリ概略

| 場所 | 内容 |
|---|---|
| `docs/` | 業務知識 v4 / ADR / 集約設計 / 横断設計 / M1 計画 |
| `.agents/` | AI ルール / スキル / フック（`.claude/` はシンボリックリンク）|
| `harness/` | 品質スコア / failure-log / work-logs / `spike/`（M1 PoC、**本実装に流用しない**）|
| `frontend/`, `backend/` | 本実装。**M2 以降に着手**（`backend/` は未作成）|
| `design/` | Figma / デザインシステム / モックアップ |
| `scripts/` | hooks / セットアップ |

## AI レビュー

| コマンド | モード | 用途 |
|---|---|---|
| PR 自動 | Standard | コードレビュー |
| `@claude deepreview` | Comprehensive | 最大 9 サブエージェント並列 |
| `@claude frontreview` | Frontend | フロントエンド 4 観点 + 自動承認判定 |
| `@claude {質問}` | Assist | 調査・修正・質問回答 |

サブエージェント定義: `.claude/agents/review/`
