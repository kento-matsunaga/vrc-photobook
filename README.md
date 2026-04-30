# VRC PhotoBook

VRChat 向けの **ログイン不要** フォトブック作成サービス。
スマホファースト・管理 URL 方式・token → HttpOnly Cookie session で構成される。

> **本リポジトリは公開前の Private リポジトリです。** 設計・運用判断・実装方針が大量に
> 含まれます。ローンチ前の Public 化は予定していません（ローンチ判断時に再検討）。

## 現在地（2026-05-01）

- **M2 終盤、ローンチ前運用整備フェーズ**（業務知識 v4 のフロー実装は主要動線 + Moderation + Report + UsageLimit まで完了）
- Backend Cloud Run + `https://api.vrc-photobook.com`（HTTPS、Google Trust Services 証明書）/ Cloud SQL `vrcpb-api-verify` で稼働中
- Frontend Cloudflare Workers + `https://app.vrc-photobook.com`（Custom Domain）で稼働中
- 主要動線（作成 → 編集 → 公開 → 管理 URL 保存 → 公開 Viewer → OGP 自動生成 → 公開配信 + 通報受付 + 運営 hide/unhide + RateLimit）は本番経路で稼働
- Outbox 副作用 handler（`photobook.published` → OGP 生成 / `photobook.hidden` `photobook.unhidden` `report.submitted` no-op + log）は **Cloud Run Job 手動 execute** で運用中
- 未実装: LP / 利用規約 / Public repo 化 / 本番運用整備 / Email Provider 再選定（ADR-0006）/ Moderation 拡張（softDelete / restore / purge / reissueManageUrl）
- **詳細・現在地マーカー（新正典）**: [`docs/plan/vrc-photobook-final-roadmap.md`](./docs/plan/vrc-photobook-final-roadmap.md)

## 重要な運用ルール（必読）

1. **raw token / Cookie 値 / Secret / DATABASE_URL / DB password を**
   - ログ・README・issue・PR 説明・コミットメッセージ・チャットに **絶対に貼らない**
2. **dummy endpoint / 認証バイパス経路 / 固定 token を本番 router に作らない**
3. **実リソース操作（Cloud Run / Cloud SQL / Workers / DNS / Secret 登録 等）は**
   - 必ず計画書を先に作り、ユーザー承認を取ってから実施
   - 各ステップで `gcloud` / `dig` / `curl` で客観確認
4. **既存 spike リソース** (`vrcpb-spike-*`) は **削除しない**（切戻しの参照点として残置）
5. **Cookie / redirect / OGP / レスポンスヘッダ変更時** は
   [`.agents/rules/safari-verification.md`](./.agents/rules/safari-verification.md)
   に従い macOS Safari + iPhone Safari で必ず確認
6. **WSL シェル運用**:
   作業ディレクトリは repo root 固定。`cd <subdir>` 不使用、
   代わりに `go -C <dir>` / `docker -f <Dockerfile>` / `npm --prefix <dir>` を使う
   （[`.agents/rules/wsl-shell-rules.md`](./.agents/rules/wsl-shell-rules.md)）

## 主要ドキュメント

| 種別 | 場所 |
|---|---|
| **エージェント向けガイド** | [`CLAUDE.md`](./CLAUDE.md) |
| **業務知識（最上位の正典）** | [`docs/spec/vrc_photobook_business_knowledge_v4.md`](./docs/spec/vrc_photobook_business_knowledge_v4.md) |
| **アーキテクチャ決定** | [`docs/adr/`](./docs/adr/)（ADR-0001〜0005） |
| **集約設計** | [`docs/design/aggregates/`](./docs/design/aggregates/)（Photobook / Image / Report 等） |
| **認可機構** | [`docs/design/auth/`](./docs/design/auth/)（Session / upload-verification） |
| **M2 計画書群** | [`docs/plan/`](./docs/plan/) |
| **現在地ロードマップ（新正典）** | [`docs/plan/vrc-photobook-final-roadmap.md`](./docs/plan/vrc-photobook-final-roadmap.md) |
| **過去の作業ログ / 失敗事例** | [`harness/work-logs/`](./harness/work-logs/) / [`harness/failure-log/`](./harness/failure-log/) |
| **デザイン資産** | [`design/`](./design/)（mockups / design-system / concept-images） |

## ディレクトリ構成

```
vrc_photobook/
├── backend/        Go + chi + pgx + sqlc（Cloud Run / vrcpb-api）
├── frontend/       Next.js 15 + Tailwind + OpenNext（Cloudflare Workers）
├── docs/           業務知識・ADR・集約設計・計画書
├── design/         デザイン資産（mockups / design-system / concept-images）
├── harness/        品質スコア / 作業ログ / 失敗事例 / spike (M1 PoC、本実装に流用しない)
├── scripts/        Hooks / セットアップ
└── .agents/        エージェントルール / スキル / フック
    └── rules/      coding / testing / security-guard / wsl-shell-rules / safari-verification 等
```

## ローカル開発

### Backend（Go + PostgreSQL）

```sh
docker compose -f backend/docker-compose.yaml up -d postgres
DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable' \
  go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 \
  -dir migrations postgres "$DATABASE_URL" up

PORT=8080 APP_ENV=local \
DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable' \
  go -C backend run ./cmd/api
```

詳細: [`backend/README.md`](./backend/README.md)

### Frontend（Next.js dev）

```sh
NEXT_PUBLIC_BASE_URL=http://localhost:3000 \
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 \
  npm --prefix frontend run dev
```

詳細: [`frontend/README.md`](./frontend/README.md)

### テスト

```sh
go -C backend test ./...
npm --prefix frontend run test
npm --prefix frontend run typecheck
npm --prefix frontend run build
```

## main ブランチ運用（Free Private プランのため手動運用）

GitHub Free + Private プランでは branch protection / rulesets が利用できない
ため、以下を運用ルールで担保する:

- `main` への直 push は **避ける**（特に `git push --force` 禁止）
- 重要な変更は **feature ブランチ → PR → squash merge** を推奨
- 1 人開発のため小さな fix は main 直 push を許容するが、
  cross-cutting / migration / 実リソース操作系は必ず branch + PR

将来の選択肢:
- **GitHub Pro へアップグレード**（個人 $4/月）して branch protection を有効化
- ローンチ後に **Public 化** すれば Free プランでも branch protection が使える

## 関連リソース（運用情報、実値は別管理）

| リソース | 識別子 |
|---|---|
| ドメイン | `vrc-photobook.com`（Cloudflare Registrar、2026-04-26 取得、自動更新 ON） |
| GCP project | `project-1c310480-335c-4365-8a8` |
| Cloud Run service | `vrcpb-api`（asia-northeast1） |
| Artifact Registry | `vrcpb`（asia-northeast1） |
| Cloud SQL instance | `vrcpb-api-verify`（検証用名のまま本番相当に使用継続。本番化 / rename はローンチ前運用整備で再判断） |
| Workers project | `vrcpb-frontend`（OpenNext で deploy 済、`app.vrc-photobook.com` で公開） |

実 endpoint / DSN / Secret 値は本書および公開ドキュメントに **記載しない**。
運用情報は Cloud Console / Cloudflare Dashboard / Secret Manager で管理する。
