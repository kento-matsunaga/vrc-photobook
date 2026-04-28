# vrc_photobook

VRChat 向けフォトブックサービス。**スマホファースト / ログイン不要 / 管理 URL 方式**。
ハーネスエンジニアリング（Spec → Implement → Verify → Feedback）で開発する。

## 現在地（2026-04-28）

**M2 終盤、ローンチ前運用整備フェーズ**。Photobook の **作成 → 編集 → 公開 → 管理 URL 保存
→ 公開 Viewer → OGP 自動生成 → 公開配信** までの主要動線が本番経路で稼働中
（PR12〜PR33d 完了）。

主要動線で実装済の機能:

- 独自ドメイン（`api.vrc-photobook.com` / `app.vrc-photobook.com`）+ HttpOnly Cookie session
- Image upload pipeline（Turnstile siteverify + presigned PUT + image-processor で
  display/thumbnail variant 生成）
- 公開 Viewer (`/p/[slug]`) + 管理ページ (`/manage/[photobookId]`) + 編集 UI 本格化
  （photo grid / caption / reorder / cover / publish settings）
- Publish flow + Complete 画面（manage URL を 1 度だけ表示する MVP 方式）
- Backend deploy 自動化（Cloud Build manual submit）+ Outbox table + 同一 TX INSERT
- outbox-worker（CLI）+ image 同梱 + Cloud Run Job `vrcpb-outbox-worker`（**手動 execute**）
- OGP 独立管理（`photobook_ogp_images` + Cloudflare Workers proxy + R2 binding、
  R2 public OFF を維持）+ `photobook.published` outbox handler 連携

未実装（公開ローンチまでに必要）:

- Moderation / `cmd/ops`（hide / unhide / softDelete / restore / purge / reissueManageUrl）
  → **次の PR34**
- Report 集約（通報受付 + 運営対応） → PR35
- UsageLimit 集約（公開数制限 / abuse 抑止） → PR36
- LP / `/terms` / `/privacy` / `/about` → PR37
- Public repo 化判断 + 履歴 secret scan → PR38
- 本番運用整備（Cloud SQL 本番化 / Budget Alert / Error Reporting） → PR39
- ローンチ前チェック + spike 削除 + Cloud Build trigger 化 → PR40
- Email Provider 再選定 + ManageUrlDelivery 集約（ADR-0006 で MVP 必須から外した、
  個人契約可能 Provider 確定後に再開） → PR32c 以降
- Cloud Scheduler 作成（outbox-worker 自動回し）→ 当面は手動 Job execute、PR33e で要否判断
- Reconcile（OGP / R2 orphan）→ PR33e（任意）
- HEIC 本対応（libheif + cgo）→ 任意

> **新しい PR / サイクルに着手する前に必ず最初に確認するロードマップ（新正典）**:
> [`docs/plan/vrc-photobook-final-roadmap.md`](./docs/plan/vrc-photobook-final-roadmap.md)
>
> PR 番号体系・現在地マーカー・各 PR の必須項目・実リソース操作・Safari 確認要否・
> 課金判断ポイントは本書に集約する。M2 前半（PR12〜PR23）の進行記録は
> [`harness/work-logs/2026-04-27_post-deploy-final-roadmap.md`](./harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)
> に archive 済（PR24 以降は新正典を優先）。

過去のロードマップ（M1 完了時点）: [`harness/work-logs/2026-04-26_project-roadmap-overview.md`](./harness/work-logs/2026-04-26_project-roadmap-overview.md)

> Backend deploy は **Cloud Build manual trigger** を経由する（PR29）。
> 手順は [`docs/runbook/backend-deploy.md`](./docs/runbook/backend-deploy.md)。
> 旧手動 `docker build` → `gcloud run services update` 経路は緊急時のみ。

## 一目で見るリンクハブ

| 何を知りたいか | 参照先 |
|---|---|
| 業務知識（最上位の正典、矛盾時はこれが正） | [`docs/spec/vrc_photobook_business_knowledge_v4.md`](./docs/spec/vrc_photobook_business_knowledge_v4.md) |
| 技術スタック / 認可 / 画像 / メールの確定事項 | [`docs/adr/`](./docs/adr/) (ADR-0001〜0006) |
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
- [`pr-closeout.md`](./.agents/rules/pr-closeout.md) — **PR 完了前に必ず確認**: コメント整合チェック / 先送り事項のロードマップ記録 / 古い PR 番号コメントの削除

## PR 完了前の必須チェック

すべての PR / 作業サイクルの完了報告を出す前に [`pr-closeout.md`](./.agents/rules/pr-closeout.md) に従い:

1. `bash scripts/check-stale-comments.sh` で stale コメント候補を一覧化
2. 各ヒットを §3 の 4 区分（修正 / 状態ベース TODO で残す / 過去経緯として残す / 生成元を直す）に分類
3. 先送り事項は新正典ロードマップ等に記録（「いつ・どの PR 以降で再検討するか」を明記）
4. 完了報告に §6 のチェックリストを含める（コメント整合 / 残した TODO / 先送り記録 / generated 反映 / Secret grep）

> 古い PR 番号コメント（「PR8 では未接続」「後続 PR で実装」等）は劣化が早いため新規に書かない。
> 状態ベース表現（「未実装」「ADR-0006 後続」「MVP 範囲外」等）を使う。

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
| `frontend/`, `backend/` | 本実装（M2 で稼働中。Backend は Cloud Run、Frontend は Cloudflare Workers 経由で公開済）|
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
