# M2 実装ブートストラップ計画 — frontend/ と backend/ の骨組み

> **位置付け**: M2 早期 §F-4 優先度 D（本実装移植）の入口計画書。本書は計画であって本実装ではない。記載された手順は実行前に本書をレビューし、ユーザーの承認後に着手する。
>
> **作成日**: 2026-04-26
>
> **上流**:
> - [`harness/work-logs/2026-04-26_project-roadmap-overview.md`](../../harness/work-logs/2026-04-26_project-roadmap-overview.md) §F-4
> - [`docs/plan/m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md)（U2 解消、ドメイン購入は延期）
> - [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md)（ディレクトリ構造の規範）
> - [`.agents/rules/testing.md`](../../.agents/rules/testing.md)（テーブル駆動 + Builder + description 必須）
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
> - [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md)（業務知識正本）
> - [`docs/design/aggregates/`](../design/aggregates/)（6 集約のドメイン設計）
> - [`docs/design/cross-cutting/`](../design/cross-cutting/)（Outbox / Reconcile / OGP）
> - [`docs/design/auth/`](../design/auth/)（session / upload-verification）
>
> **重要前提**:
> - **本書ではコードを書かない / リソースを作らない**
> - **PoC コード（`harness/spike/`）はそのまま流用しない**（粗いコードを本実装に持ち込まない原則）
> - **ドメイン購入は引き続き延期**（`m2-domain-candidate-research.md` §9.2）
> - **Cloud SQL / Cloud Run Jobs / Cloud Scheduler / SendGrid 実送信 / Turnstile 本番 widget は本書段階では作らない**
> - **AWS SES 採用不可**（ADR-0004）
> - **Safari / iPhone Safari は常に検証対象**（`.agents/rules/safari-verification.md`）

---

## 1. 目的

| 目的 | 内容 |
|---|---|
| M2 本実装の入口を定義 | どの順序で何を作るかの一次合意を形成 |
| `backend/` と `frontend/` の初期構造を決める | `domain-standard.md` 構造に従い、PoC とは別の骨組みを設計 |
| **PoC コードをそのまま流用しない**方針を明記 | `harness/spike/` は参照のみ（学び・失敗ログを糧にする）|
| `domain-standard.md` / `testing.md` / `security-guard.md` を本実装の基準にする | ルール違反コミットを最初から発生させない |
| ドメイン購入前にローカルでしっかり動く骨格を作る | 独自ドメイン取得時にスムーズに切替できる土台を準備 |

---

## 2. 前提

### 2.1 状態前提（2026-04-26 時点）

- M1 は完了承認済（`harness/work-logs/2026-04-26_m1-completion-judgment.md`）
- **ドメインは `vrc-photobook.com` で確定・購入済**（2026-04-26 後段、`m2-domain-candidate-research.md` §9.5）。
  当初は `vrcphotobook.com`（ハイフン無し）が第一候補だったが、ハイフン入りで購入が確定した。以後の実装は `vrc-photobook.com` を正とする
- ドメイン取得後の DNS / Workers Custom Domain / Cloud Run Domain Mapping 設定は別 PR で実施（本書段階では未実施）
- 本実装は `frontend/`（既存ディレクトリ、`.gitkeep` のみ）/ `backend/`（未作成）に新規作成
- `harness/spike/` は **参照のみ**、コピペ流用しない

### 2.2 着手範囲の制約

- 本書段階では **コードを書かない / リソースを作らない**
- Cloud SQL / Cloud Run Jobs / Cloud Scheduler は作らない
- SendGrid 実送信 PoC / Turnstile 本番 widget は作らない
- ドメイン購入 / Cloudflare DNS / Workers Custom Domain / Cloud Run Domain Mapping は実施しない
- AWS SES は採用しない

### 2.3 検証対象ブラウザ（M2 でも継続）

- macOS Safari 最新（必須）
- iPhone Safari 最新（必須）
- Chrome 最新（ベースライン）

---

## 3. backend/ 初期構造案

`.agents/rules/domain-standard.md` に従い、Go module ルートに以下の構造を提案する。

```
backend/
├── go.mod
├── go.sum
├── Dockerfile                          # multi-stage / distroless / nonroot
├── docker-compose.yaml                 # ローカル開発用（PostgreSQL + backend + 必要なら mailpit）
├── .dockerignore
├── .env.example                        # キー名のサンプルのみ、値は書かない
├── .gitignore
├── sqlc.yaml
├── README.md
├── cmd/
│   ├── api/main.go                     # HTTP API サーバ（chi）
│   ├── outbox-worker/main.go           # Outbox 配送ワーカー（Cloud Run Jobs 想定）
│   └── ops/main.go                     # 運営 CLI（hide / unhide / restore / purge / reissue_manage_url 等、ADR-0002）
├── migrations/                         # goose
│   └── 00001_*.sql ...
└── internal/
    ├── config/                         # 環境変数読み込み（os.Getenv のみ、ライブラリ依存なし）
    ├── http/                           # chi ルーター組み立て / OpenAPI 風ルート定義
    ├── middleware/                     # CORS / Origin / RequestID / Recoverer / Auth / RateLimit
    ├── database/                       # pgx pool / トランザクション境界 / Tx 受け渡し
    ├── outbox/                         # Outbox 横断機構（dispatcher / handlers ルーティング）
    ├── reconcile/                      # 自動 reconciler 4 種 + 手動 cmd/ops/reconcile/ の共通基盤
    ├── platform/
    │   ├── r2/                         # Cloudflare R2（aws-sdk-go-v2 wrapper）
    │   ├── email/                      # EmailSender ポート（SendGrid 実装は infra に置くが共通インターフェイス）
    │   └── turnstile/                  # Turnstile siteverify クライアント
    ├── shared/                         # 共通 VO（UUIDv7 / セッショントークン生成 / hash）/ 共通エラー型
    ├── auth/                           # 認可機構（session / upload-verification、集約ではない、docs/design/auth/ 準拠）
    │   ├── session/
    │   │   ├── domain/                 # session の VO / 操作仕様
    │   │   ├── infrastructure/         # repository（sessions テーブル）
    │   │   └── internal/usecase/       # ExchangeTokenForSession 等
    │   └── upload-verification/
    │       ├── domain/
    │       ├── infrastructure/
    │       └── internal/usecase/
    └── modules/                        # 各集約（domain-standard.md §構造）
        ├── photobook/
        │   ├── domain/
        │   │   ├── entity/             # Photobook / Page / Photo / Cover / DraftEditToken / ManageUrlToken
        │   │   ├── vo/                 # PhotobookId / Slug / TitleVo / Visibility 等
        │   │   └── service/            # ドメインサービス（必要時のみ）
        │   ├── infrastructure/
        │   │   ├── repository/rdb/     # pgx + sqlc 生成コード経由
        │   │   ├── query/rdb/          # CQRS Query 側（一覧・閲覧用）
        │   │   ├── marshaller/         # ドメイン ↔ DB 変換
        │   │   ├── tests/              # テスト用 Builder
        │   │   └── mock/               # interface mock（必要時）
        │   └── internal/
        │       ├── usecase/            # CreateDraftPhotobook / PublishPhotobook 等（コマンド & クエリ）
        │       └── controller/         # API handler（chi handler、薄く保つ）
        ├── image/
        │   └── ... ( 同構造 )
        ├── report/
        │   └── ...
        ├── usage-limit/
        │   └── ...
        ├── manage-url-delivery/
        │   └── ...
        └── moderation/
            └── ...
```

### 3.1 各ディレクトリの役割

| ディレクトリ | 役割 |
|---|---|
| `cmd/api` | HTTP API サーバの起動エントリ（chi router を組み立てて listen） |
| `cmd/outbox-worker` | Outbox 配送ワーカー（`--once` / `--retry-failed`）。Cloud Run Jobs から起動 |
| `cmd/ops` | 運営 CLI（ADR-0002）。サブコマンド（`hide` / `unhide` / `restore` / `purge` / `reissue_manage_url` / `resolve_report` / `list_reports` / `list_moderation_actions` / `reconcile/*`）|
| `internal/config` | 環境変数読み込み（標準 `os.Getenv`、ライブラリ依存なし）|
| `internal/http` | chi router 組み立て、ルートグループ、CORS / Origin 設定 |
| `internal/middleware` | RequestID / RealIP / Recoverer / Timeout（chi 標準） + 自前の Auth / Origin / RateLimit |
| `internal/database` | pgx pool 管理、Tx 関数（`WithTx(ctx, fn)`）|
| `internal/outbox` | Outbox dispatcher（`event_type` → handler ルーティング）/ ハンドラ集約 |
| `internal/reconcile` | 自動 reconciler 4 種（`outbox_failed_retry` / `draft_expired` / `stale_ogp_enqueue` / `delivery_expired_to_permanent`）共通基盤 |
| `internal/platform/r2` | R2 client（aws-sdk-go-v2、HeadBucket / PresignPut / HeadObject）|
| `internal/platform/email` | `EmailSender` interface（domain）+ `infra/sendgrid_sender.go` / `infra/mailgun_sender.go`（M2 早期 SendGrid 採用）|
| `internal/platform/turnstile` | siteverify client（M1 PoC で確立した実装を本実装版に書き直し）|
| `internal/shared` | UUIDv7 生成 / 256bit 乱数 + base64url + SHA-256 / 共通エラー型 / 構造化ログヘルパ |
| `internal/auth/session` | draft / manage session 認可機構（集約ではない、`docs/design/auth/session/` 準拠）|
| `internal/auth/upload-verification` | Turnstile セッション化（同上、`docs/design/auth/upload-verification/`）|
| `internal/modules/{module}/` | 各集約の domain / infrastructure / internal（`domain-standard.md` 厳守）|
| `migrations/` | goose migration（v3.22）。集約の DDL を順次追加 |
| `sqlc.yaml` | sqlc v1.30 + pgx/v5 出力設定 |

### 3.2 sqlc / goose の運用

- `sqlc.yaml` で `pgx/v5` 出力を採用
- DDL は `migrations/` に goose で書く（`-- +goose Up/Down`）
- Query は各集約の `infrastructure/repository/rdb/queries/*.sql` に書き、sqlc で各集約 `infrastructure/repository/rdb/sqlcgen/` 配下にコード生成
- 本実装では **集約ごとに sqlcgen ディレクトリを分ける**（M1 PoC では 1 箇所にまとまっていたが、集約境界を保つため M2 で分離）

---

## 4. backend/ 実装順序

ローカル開発で一気通貫を確認できる順序。Cloud SQL は作らず、ローカル PostgreSQL（docker-compose）で進める。

| # | ステップ | 完了条件 |
|---|---|---|
| 1 | **Go module / lint / test / build の最小セット** | `go.mod` 作成、`go vet` / `go build` / `go test ./...` が空ツリーで通る、`golangci-lint` 設定（M1 では未導入、M2 で入れる）|
| 2 | **config / logger / graceful shutdown** | `internal/config`（os.Getenv 最小実装）/ `slog` JSON ハンドラ / `signal.NotifyContext` + `srv.Shutdown(ctx)` |
| 3 | **`/health` / `/readyz`** | `/health` 200（Cloud Run 公式ヘルスチェック、M1 学習済）/ `/readyz` は pgx pool ping、pool nil 時 503 |
| 4 | **database 接続** | `internal/database/pool.go`、DSN 空時 `nil, nil` 返す（M1 で確立した DB なし起動の互換）|
| 5 | **goose migrations** | `_test_alive` 風の最小 migration、本格集約 DDL は §6 で順次追加 |
| 6 | **sqlc base** | `sqlc.yaml`、最小 query 1 本で生成成立確認 |
| 7 | **Session auth 基盤** | `internal/auth/session/`、`sessions` テーブル、token → session 交換、Cookie Domain 対応設計（実値は ENV で）|
| 8 | **Photobook aggregate** | `domain-standard.md` 厳守、Photobook entity / VO / 不変条件 / `CreateDraftPhotobook` / `PublishPhotobook` / `PhotobookSoftDeleted` 等、Outbox 同一 TX |
| 9 | **Image aggregate + R2 連携** | `images` / `image_variants` テーブル、upload-intent / complete UseCase、R2 PresignPut / HeadObject、`ImageIngestionRequested` Outbox |
| 10 | **Upload verification + Turnstile** | `upload_verification_sessions` テーブル、`internal/auth/upload-verification/`、Turnstile siteverify、原子消費 SQL（M1 学習済）|
| 11 | **Outbox 横断機構** | `outbox_events` テーブル、`internal/outbox/dispatcher/`、`cmd/outbox-worker --once / --retry-failed` |
| 12 | **ManageUrlDelivery + EmailSender** | `manage_url_deliveries` / `manage_url_delivery_attempts` テーブル、`EmailSender` interface、`infra/sendgrid_sender.go`（実送信は M2 早期 §F-3 別タスクで API Key 後）|
| 13 | **Report / Moderation** | `reports` / `moderation_actions` テーブル、Moderation 同一 TX（Photobook + ModerationAction + Report 状態 + Outbox）|
| 14 | **Reconcile scripts** | 自動 reconciler 4 種 / 手動 `scripts/ops/reconcile/*.sh` + `cmd/ops/reconcile/` |
| 15 | **`cmd/ops`** | 運営 CLI、ADR-0002 準拠（`--dry-run` デフォルト、`--operator` 必須、参照系は `--execute` 不要）|

### 4.1 ローカル DB 戦略

- **第一案（推奨）**: docker-compose で `postgres:16-alpine` を立てる（M1 PoC と同じ運用）
- 第二案: Compute Engine 上の単一 PostgreSQL（M1 計画書 §12 で代替案として挙げた構成、不要）
- **Cloud SQL は M2 早期 §F-2 優先度 B でのみ短時間起動**（本書段階では作らない）

---

## 5. frontend/ 初期構造案

Next.js 15 App Router + OpenNext + Tailwind を前提に、以下の構造を提案する。

```
frontend/
├── package.json
├── package-lock.json
├── tsconfig.json
├── next.config.mjs                     # middleware 一本化方針、headers() に X-Robots-Tag を書かない（M1 学習済）
├── open-next.config.ts                 # 最小設定（incremental cache 無し）
├── wrangler.jsonc                      # Workers + Static Assets binding、name = vrcpb（仮）
├── middleware.ts                       # X-Robots-Tag / Referrer-Policy 出し分け
├── tailwind.config.ts
├── postcss.config.mjs
├── .env.local.example
├── .env.production.example             # NEXT_PUBLIC_API_BASE_URL / NEXT_PUBLIC_BASE_URL のサンプル
├── .gitignore                          # .env / .env.local / .env.production を除外
├── README.md
├── public/
│   ├── og-default.png                  # OGP デフォルト画像
│   └── ...
├── styles/
│   └── globals.css                     # Tailwind base / components / utilities
├── lib/                                # 共通関数（fetch wrapper / zod スキーマ / Cookie util）
│   ├── api-client.ts                   # Backend API client（NEXT_PUBLIC_API_BASE_URL 経由）
│   ├── schemas/                        # zod スキーマ（Frontend / Backend で共有予定の型）
│   └── cookies.ts                      # Cookie 操作ヘルパ（HttpOnly Cookie は読み書き禁止、存在確認のみ）
├── components/                         # 共通 UI コンポーネント（プレゼンテーション層）
│   ├── ui/                             # ボタン / 入力 / カード等の汎用
│   └── layout/                         # ヘッダ / フッタ / ナビ
├── features/                           # 機能ごとの実装（Atomic Design とは別、業務単位）
│   ├── photobook-create/               # 作成画面のロジック + UI
│   ├── photobook-edit/                 # 編集画面
│   ├── photobook-view/                 # 公開閲覧
│   ├── photobook-manage/               # 管理画面
│   ├── photobook-report/               # 通報フォーム
│   └── upload/                         # 画像アップロード（Turnstile widget 統合 / presigned PUT）
└── app/                                # App Router
    ├── layout.tsx                      # metadataBase（NEXT_PUBLIC_BASE_URL 経由、M1 学習済）/ robots: noindex
    ├── (public)/                       # ルートグループ：公開・閲覧系
    │   ├── page.tsx                    # トップ / LP（v4 §3.9）
    │   └── p/[slug]/page.tsx           # 公開フォトブック閲覧（v4 §3.3）
    ├── (draft)/                        # ルートグループ：作成者向け（draft）
    │   ├── draft/[token]/route.ts      # token → session 交換 → /edit/{photobook_id} redirect
    │   └── edit/[photobook_id]/page.tsx # draft 編集
    ├── (manage)/                       # ルートグループ：管理 URL 経由
    │   ├── manage/token/[token]/route.ts # token → session 交換 → /manage/{photobook_id} redirect
    │   └── manage/[photobook_id]/page.tsx # 管理画面
    ├── (report)/                       # ルートグループ：通報フォーム
    │   └── report/page.tsx
    └── api/                            # Route Handler
        └── （MVP では原則使わない方針：Backend が API を持つ。Frontend Route Handler は token 受けの薄いハンドラのみ）
```

### 5.1 Route Handler の方針（重要）

- **Backend が API を持つ**ため、Frontend の `app/api/*` Route Handler は **原則使わない**
- 例外として **token 受け（`/draft/[token]` / `/manage/token/[token]`）の Route Handler** だけは Frontend 側に置く
  - 理由: token を Backend に渡してから Set-Cookie するより、Frontend の Route Handler 内で Backend を呼んで Cookie を発行する方が、独自ドメイン下での Cookie Domain 設定が単純
  - ただし、token の **検証ロジックは Backend へ HTTP 委譲**する（Frontend に hash 検証ロジックを書かない）

### 5.2 features/ vs components/

- `features/`: 機能単位の実装（API 呼び出し / 状態管理 / ページ固有 UI）。例 `features/photobook-edit/EditForm.tsx`
- `components/`: 機能横断で再利用可能な UI（純粋プレゼンテーション）。例 `components/ui/Button.tsx`
- Atomic Design は採用しない（過剰な抽象化を避ける）

---

## 6. frontend/ 実装順序

| # | ステップ | 完了条件 |
|---|---|---|
| 1 | **Next.js 15 / Tailwind / OpenNext 初期構築** | `npm create next-app`（手動セットアップでも可）+ Tailwind + `@opennextjs/cloudflare` 導入、`wrangler.jsonc` 最小セット |
| 2 | **metadataBase / noindex / Referrer-Policy / X-Robots-Tag** | `app/layout.tsx` で `metadataBase` 設定（M1 学習済）、`middleware.ts` で `X-Robots-Tag` / `Referrer-Policy` 出し分け、`next.config.mjs` には書かない |
| 3 | **draft / manage token redirect ルート** | `(draft)/draft/[token]/route.ts` / `(manage)/manage/token/[token]/route.ts`、Backend へ token 検証委譲、Cookie 発行 + 302 redirect |
| 4 | **edit / manage / public view の画面骨格** | `(draft)/edit/[photobook_id]/page.tsx` / `(manage)/manage/[photobook_id]/page.tsx` / `(public)/p/[slug]/page.tsx`、SSR + Cookie session 検証、Server Component から Backend へ fetch |
| 5 | **画像アップロード UI** | `features/upload/`、Backend `/api/photobooks/{id}/images/upload-intent` → R2 直接 PUT → `/api/images/{id}/complete`、進捗表示、Content-Length 一致（M1 学習済）|
| 6 | **Turnstile widget 差し込み準備** | `features/upload/TurnstileWidget.tsx`、本番 widget hostname 確定後（M2 早期 §F-3）にプロダクション siteKey に差し替えやすい構造。M2 早期段階では公式テスト siteKey で動作 |
| 7 | **API client** | `lib/api-client.ts`、`fetch` の薄いラッパー、credentials / CORS 設定、エラー型変換、リトライポリシー |
| 8 | **エラーハンドリング** | `app/error.tsx` / `app/not-found.tsx`、Cookie 切れ時の再入場 URL 案内画面（ADR-0003 §決定 §session 期限切れ時）|
| 9 | **Safari / iPhone Safari 表示確認** | `.agents/rules/safari-verification.md` 必須項目、ローカル / 開発デプロイで実機確認 |
| 10 | **Workers deploy 設定** | `wrangler.jsonc` を本実装名に確定、`cf:build` / `cf:deploy` script、Workers Custom Domain は **ドメイン購入後**に追加 |

---

## 7. Docker / docker-compose

ローカル開発で M2 全期間を支える構成案。

```yaml
# backend/docker-compose.yaml （案）
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: vrcpb
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}    # .env.local から
      POSTGRES_DB: vrcpb
    ports: ["5432:5432"]
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U vrcpb"]
      interval: 5s

  api:
    build:
      context: .
      dockerfile: Dockerfile
    environment:
      DATABASE_URL: postgres://vrcpb:${POSTGRES_PASSWORD}@postgres:5432/vrcpb?sslmode=disable
      APP_ENV: local
      # R2_*, ALLOWED_ORIGINS, TURNSTILE_SECRET_KEY は .env.local から（実値はコミットしない）
    ports: ["8080:8080"]
    depends_on:
      postgres: { condition: service_healthy }

  # mailpit はメール検証時のみ追加（M2 早期 §F-3 SendGrid 準備のローカル代替）
  # mailpit:
  #   image: axllent/mailpit
  #   ports: ["1025:1025", "8025:8025"]

volumes:
  pgdata:
```

### 7.1 R2 のローカル戦略

- **第一案（推奨）**: **実 Cloudflare R2 を使う**（M1 PoC で確立した運用）
  - 理由: M1 で短期 R2 API Token + 専用バケット運用が安定して機能していた
  - 利点: localstack / minio との API 差分（とくに署名挙動）に悩まされない、`Content-Length` SignedHeaders 等の挙動が本物と一致
- 第二案: localstack / minio をローカルで立てる
  - 利点: オフライン開発可能、コスト 0
  - 欠点: 署名挙動の差分、HEIC / 大容量画像で挙動差が出ると本番再現コストが嵩む
  - **採用しない**（MVP では実 R2 で問題ないため）

### 7.2 mailpit の扱い

- M2 早期 §F-3 SendGrid 実送信 PoC 着手時に `mailpit` をローカル代替として導入候補
- それまでは `EmailSender` interface の `FakeSender`（メモリ内記録のみ）でテスト
- mailpit は SMTP 受信を Web UI で見られるため、本番に近いローカル検証ができる

---

## 8. CI / 品質ゲート

### 8.1 CI（GitHub Actions、M2 で導入）

| ジョブ | 内容 | 失敗時の扱い |
|---|---|---|
| `backend-vet` | `go vet ./...` | ブロック |
| `backend-build` | `go build ./...` | ブロック |
| `backend-test` | `go test ./...`（testcontainers-postgres）| ブロック |
| `backend-lint` | `golangci-lint run` | ブロック |
| `sqlc-check` | `sqlc generate` 後 git diff が空 | ブロック（生成漏れ検出）|
| `goose-check` | migration の up→down→up が成立 | ブロック |
| `frontend-build` | `npm run build` | ブロック |
| `frontend-lint` | `npm run lint` | ブロック |
| `frontend-typecheck` | `tsc --noEmit` | ブロック |
| `frontend-format` | `prettier --check` | ブロック |
| `secret-grep` | `git diff` 内に Secret パターンが無いか | ブロック |
| `safari-check` | **手動ゲート**（PR description のチェックボックス）| 警告 |
| `failure-log-check` | failure-log の整合性 | 警告 |

### 8.2 PR テンプレート（M2 で整備）

`.github/pull_request_template.md`（仮）に以下のチェックボックスを入れる:

- [ ] `go test ./...` / `npm test` ともに pass
- [ ] `domain-standard.md` 構造に違反していない
- [ ] テストは `testing.md` のテーブル駆動 + Builder + description で書いた
- [ ] Secret / Cookie / token / presigned URL がログ・diff・コミットメッセージに出ていない
- [ ] Safari / iPhone Safari の確認が必要な変更か。必要ならどこで確認したか記載
- [ ] `harness/QUALITY_SCORE.md` を更新したか
- [ ] 失敗があれば `harness/failure-log/` に記録したか

### 8.3 自動承認（M2 中盤以降）

- `@claude frontreview` / `@claude deepreview` の AI レビューを併用（CLAUDE.md の §AI レビュー）
- 自動承認は `frontreview` の判定が clean な場合のみ

---

## 9. テスト方針（`.agents/rules/testing.md` 準拠）

### 9.1 必須パターン

- **テーブル駆動テスト**: `tests := []struct { name, description string; ... }{...}`、フラットな `t.Run()` 列挙禁止
- **`description` 必須**: Given / When / Then を 1 行で書く
- **Builder パターン**: メソッドテストの前提構築は集約ごとの Builder を使う、`t` を保持しない、`Build(t)` で受け取る
- **コンストラクタテストは直接構築**: `New*()` 関数のテストでは Builder を使わない
- **テストファイル内ヘルパー禁止 / fixture.go 禁止**: 暗黙のテストデータを作らない

### 9.2 テスト階層

| レイヤー | テスト内容 | DB | 配置 |
|---|---|---|---|
| ドメインモデル | コンストラクタ / ビジネスメソッド / 境界値 | 不要 | `internal/modules/{module}/domain/entity/*_test.go` |
| VO | コンストラクタ / 等価性 / 不変性 | 不要 | `internal/modules/{module}/domain/vo/*/*_test.go` |
| ユースケース | ビジネスフロー | mock 可（DB なしで Repository mock）| `internal/modules/{module}/internal/usecase/*_test.go` |
| リポジトリ | データ永続化・取得 | 必要（testcontainers-postgres）| `internal/modules/{module}/infrastructure/repository/rdb/*_test.go` |
| コントローラー（API）| API 統合 | depends（UseCase は実物）| `internal/modules/{module}/internal/controller/*_test.go` |
| Frontend ユニット | components / features | 不要 | `*.test.tsx`（Vitest）|
| Frontend E2E | 主要シナリオ | depends | `tests/e2e/`（Playwright、MVP 後半で最小限）|

### 9.3 PoC との違い

- M1 PoC は **意図的にテストを書かない方針**（`harness/spike/backend/README.md` §既知の制限）
- 本実装は **テストを書く**。テストなしの PR は CI でブロックする運用
- テストの目安カバレッジ: ドメイン層 90%+ / UseCase 80%+ / Controller 70%+ / Frontend 60%+（厳密ではなく目安）

---

## 10. セキュリティ方針（`.agents/rules/security-guard.md` / ADR-0003 / ADR-0005 準拠）

### 10.1 raw token / Secret 管理

- **raw token は DB に保存しない**（SHA-256 のみ保存、bytea 32B、M1 で確立）
- **raw token をログに出さない**: 構造化ログの禁止フィールドに `authorization` / `cookie` / `draft_edit_token` / `manage_url_token` / `session_token` を登録、マスク処理を中央化
- **presigned URL をログに出さない**: 構造化ログ / Sentry / APM すべてで scrub
- **`storage_key` も慎重に扱う**: パス推測抑止
- **`recipient_email` は 24h 後 NULL 化**（ManageUrlDelivery、ADR-0004）
- **管理 URL 再発行時に過去 `recipient_email` を再利用しない**（ADR-0004）

### 10.2 Cookie 属性

- **HttpOnly / Secure / SameSite=Strict / Path=/**（ADR-0003）
- **Domain は独自ドメイン取得後に `.<domain>`** を設定（M2 早期 §F-1）
- 期限: draft 7 日 / manage 24h〜7日

### 10.3 Safari 検証必須

`.agents/rules/safari-verification.md` の必須項目:
- Cookie 発行ロジック / redirect / OGP / レスポンスヘッダ / モバイル UI 変更時に **macOS Safari + iPhone Safari** で必ず確認
- Chrome / Edge のみで完了とすることは禁止

### 10.4 Secret Manager 前提

- すべての Secret は **GCP Secret Manager** 経由で Cloud Run / Cloud Run Jobs に注入（M1 で確立）
- ローカルは `.env.local`（git ignore）
- **`.env.local` は Docker build context に入れない**（`.dockerignore` で除外、M1 で確立）

### 10.5 検証ルール

- 実行者 ID は **コンテキストから取得**（リクエストパラメータからの取得禁止、`security-guard.md`）
- マルチテナントデータアクセスは **テナントスコープガード（RLS）** 必須
- 認可チェックは **ユースケース実行前に必ず実施**
- 認証バイパスのテストヘルパー禁止（テストでも認証フローを通す）
- シークレットのハードコード禁止

---

## 11. PoC から本実装へ持ち込むもの / 持ち込まないもの

### 11.1 持ち込むもの（学び・知見・記録）

| 項目 | 反映先 |
|---|---|
| 技術判断（chi / pgx / sqlc / goose / OpenNext / R2 SDK 等）| ADR-0001 で確定済 |
| 検証結果（Cloud Run Domain Mapping / `/health` 採用 / `metadataBase` 必要 / X-Robots-Tag 重複問題 等）| ADR / 計画書 / failure-log で記録済 |
| 失敗ログ 6 件 | `harness/failure-log/2026-04-25_*.md` 〜 `2026-04-26_*.md` |
| API 挙動の学び（`Content-Length` SignedHeaders / GFE intercept / NEXT_PUBLIC build inline 等）| failure-log + roadmap §J |
| Cloud Run / Workers / R2 / Turnstile / Outbox の設計知見 | M1 計画書 / 完了判定表 / 集約ドメイン設計の§M1 検証結果 |
| 5 つの failure-log + ハーネス補強で確立した運用ルール | `.agents/rules/wsl-shell-rules.md` 等 |

### 11.2 持ち込まないもの（コード本体）

| 項目 | 理由 |
|---|---|
| `harness/spike/backend/cmd/`、`internal/sandbox/*` | sandbox endpoint は本実装の handler とは責務 / 構造が違う |
| `harness/spike/backend/Dockerfile` | M1 の最小 Dockerfile を本実装で書き直す（domain-standard.md 構造に合わせて） |
| 仮 token（`sample-draft-token` / `sample-manage-token`）| 本実装は実 token 検証 |
| sample page（`/p/sample-slug` 等）| 本実装はクエリ生成された Photobook を表示 |
| M1 専用の暫定実装（mock turnstile mode、DB なし起動の handler 分岐 等）| 本実装は本番運用前提、mock は test 内のみ |
| DB なし起動の過剰な分岐 | 本実装は DB 接続前提（`/readyz` で 503 を返す前提）|
| `harness/spike/frontend/app/integration/backend-check/page.tsx` | 検証用ページ、本実装には不要 |

### 11.3 PoC コードを直接コピーしないルール

- 本実装ファイルを書くときは **PoC を「読む」ことはあるが「コピペしない」**
- 必要なら PoC コードをトリミングして再構成、テストを書きながら写経する形
- PR レビューで「PoC コピペ」を検出したら差し戻しの運用を CI / レビュアーで合意

---

## 12. 最初の実装 PR 候補

### PR1: backend skeleton

| 項目 | 内容 |
|---|---|
| 作成物 | `backend/go.mod` / `backend/.gitignore` / `backend/.env.example` / `backend/README.md` / `backend/cmd/api/main.go`（最小、`/health` だけ）|
| 完了条件 | `go vet ./...` / `go build ./...` / `go test ./...` 成功、`/health` 200 をローカル `go run` で確認 |
| 範囲外 | DB 接続 / R2 / 認証 / API 機能 |

### PR2: backend config / logger / health

| 項目 | 内容 |
|---|---|
| 作成物 | `backend/internal/config/config.go` / `backend/internal/shared/logging.go`（slog wrapper）/ `backend/cmd/api/main.go` の graceful shutdown / `/readyz` |
| 完了条件 | `slog` JSON ログ確認、SIGTERM で 10 秒以内終了、`/readyz` が pool nil で 503 |

### PR3: backend database / migrations / sqlc base

| 項目 | 内容 |
|---|---|
| 作成物 | `backend/internal/database/pool.go` / `backend/migrations/00001_*.sql`（最小）/ `backend/sqlc.yaml` / `backend/Dockerfile`（最小、後で 2 バイナリ化）/ `backend/docker-compose.yaml` |
| 完了条件 | `goose up` 成功、`sqlc generate` 成功、`docker compose up -d` で PostgreSQL 起動 + backend `/readyz` 200 |

### PR4: frontend skeleton

| 項目 | 内容 |
|---|---|
| 作成物 | `frontend/package.json` / `frontend/next.config.mjs` / `frontend/tsconfig.json` / `frontend/tailwind.config.ts` / `frontend/postcss.config.mjs` / `frontend/app/layout.tsx` / `frontend/app/page.tsx`（最小トップ）/ `frontend/.gitignore` / `frontend/README.md` |
| 完了条件 | `npm install` / `npm run build` / `npm run dev` 成功、トップページが Tailwind スタイル付きで表示 |

### PR5: frontend security headers / middleware

| 項目 | 内容 |
|---|---|
| 作成物 | `frontend/middleware.ts`（X-Robots-Tag / Referrer-Policy 出し分け）/ `frontend/app/layout.tsx` の `metadataBase` / `frontend/wrangler.jsonc`（最小）/ `frontend/open-next.config.ts` |
| 完了条件 | `npm run cf:build` / `cf:preview` 成功、curl で X-Robots-Tag が 1 回だけ出る、metadataBase が `.env.production` で渡せる |

### PR6: docker-compose local dev

| 項目 | 内容 |
|---|---|
| 作成物 | リポジトリルートまたは `backend/` 配下の `docker-compose.yaml` 確定 / `.env.local.example` 整備 / README にローカル起動手順 |
| 完了条件 | `docker compose up -d` で PostgreSQL + backend が起動、frontend は `npm run dev` で別途起動、両者が同一マシンで動く |

### PR の進行順序

1. **PR1 → PR2 → PR3** を順次（backend 先行、Skeleton + Config + DB）
2. **並行で PR4 → PR5** を進めても良い（frontend 先行）
3. **PR6** は PR3 / PR5 完了後（Backend と Frontend の両方が動くようになってから）
4. これ以降は `internal/auth/session/` → 各集約（Photobook → Image → ...）の順で PR を増やす

---

## 13. ユーザー判断事項

| # | 判断項目 | 推奨 / 提案 |
|---|---|---|
| 1 | **backend から先に作るか、frontend から先に作るか** | **backend 先行**（PR1 → PR2 → PR3）。理由: API 仕様を確定させてから Frontend が叩く構造の方が手戻りが少ない |
| 2 | **docker-compose を先に整えるか** | PR3（database 着手時）に同時整備 |
| 3 | **local PostgreSQL を使うか** | docker-compose で `postgres:16-alpine` を使う（Cloud SQL は M2 早期 §F-2 まで作らない）|
| 4 | **R2 は実 Cloudflare を使うか** | **実 Cloudflare R2 を使う**（§7.1 推奨）。M1 PoC と同じ短期 API Token 運用 |
| 5 | **SendGrid / Turnstile はいつ入れるか** | SendGrid: M2 中盤、ManageUrlDelivery 着手時（§4 ステップ 12）。Turnstile: M2 早期、Image upload-intent 着手時（§4 ステップ 9〜10）。本番 widget は **ドメイン購入後**（M2 早期 §F-3）|
| 6 | **ドメイン購入の再判断タイミング** | 以下のいずれかで本書 §6 PR5 / `m2-domain-candidate-research.md` §9.3 を参照: <br> - PR1〜PR5 完了後（本実装骨格が確定） <br> - PR で `internal/auth/session/` 着手時（Cookie Domain 設定が必要に） <br> - SendGrid 実送信 PoC 着手時（送信ドメイン認証が必要に） |
| 7 | **CI を最初の PR に入れるか** | PR1 と同時に最小 CI（`go vet` / `go build` / `go test`）を入れる。frontend CI は PR4 で追加。golangci-lint や PR テンプレートは PR2〜3 で段階的に拡張 |
| 8 | **PoC コードは消すか残すか** | 本実装が稼働するまで `harness/spike/` は **残す**（参照価値あり）。MVP リリース前に「PoC 削除」or「PoC アーカイブ移動」の判断を再度行う |

---

## 14. 関連ドキュメント

- 上流ルール: [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md) / [`.agents/rules/testing.md`](../../.agents/rules/testing.md) / [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`.agents/rules/feedback-loop.md`](../../.agents/rules/feedback-loop.md)
- 業務知識: [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md)
- ADR: [`docs/adr/`](../adr/)（0001〜0005）
- 集約: [`docs/design/aggregates/README.md`](../design/aggregates/README.md)
- 横断: [`docs/design/cross-cutting/`](../design/cross-cutting/)
- 認可: [`docs/design/auth/README.md`](../design/auth/README.md)
- ロードマップ: [`harness/work-logs/2026-04-26_project-roadmap-overview.md`](../../harness/work-logs/2026-04-26_project-roadmap-overview.md) §F-4
- M1 完了判定: [`harness/work-logs/2026-04-26_m1-completion-judgment.md`](../../harness/work-logs/2026-04-26_m1-completion-judgment.md)
- ドメイン: [`docs/plan/m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md) / [`docs/plan/m2-domain-candidate-research.md`](./m2-domain-candidate-research.md)

## 15. 履歴

| 日付 | 変更 |
|---|---|
| 2026-04-26 | 初版作成。M2 本実装ブートストラップ計画として、`backend/` / `frontend/` 初期構造案、実装順序、Docker、CI、テスト、セキュリティ、PoC との分離方針、最初の 6 PR 候補、ユーザー判断事項 8 項目を整理。本書段階ではコード作成・実リソース操作は行わない |
