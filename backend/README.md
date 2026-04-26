# backend/

VRC PhotoBook の **本実装 Backend**。M2 以降の段階的 PR で機能を追加していく。

## 位置付け

- 本ディレクトリは **M2 本実装** であり、`harness/spike/backend/`（M1 PoC）とは別物。
- PoC コードを**直接コピペで流用しない**方針（[`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §11）。
- 本実装は [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../docs/spec/vrc_photobook_business_knowledge_v4.md) / ADR-0001〜0005 / 各集約ドメイン設計 / [`.agents/rules/domain-standard.md`](../.agents/rules/domain-standard.md) / [`.agents/rules/testing.md`](../.agents/rules/testing.md) に厳密に従う。

## 現在のスコープ（〜 PR6）

- HTTP server 起動（chi）
- `GET /health` → 200 `{"status":"ok"}`（PR1）
- `GET /readyz`（PR2 で枠、PR3 で pgx pool 状態に応じた分岐）:
  - `pool == nil`（`DATABASE_URL` 未設定） → 503 `{"status":"db_not_configured"}`
  - `pool.Ping(ctx)` 失敗 → 503 `{"status":"db_unreachable"}`
  - 成功 → 200 `{"status":"ready"}`
- `PORT` / `APP_ENV` / `DATABASE_URL` の最小読み込み（PR2-3 / `internal/config`）
- slog JSON logger（PR2 / `internal/shared/logging.go`）
- graceful shutdown（SIGINT / SIGTERM、10 秒 timeout）（PR2）
- pgx/v5 接続プール（PR3 / `internal/database/pool.go`、DSN 空時 nil 返し）
- goose migration 1 本（PR3 / `_health_check` 基盤確認用、後続 PR で集約 DDL に置換）
- sqlc base（PR3 / `internal/database/sqlcgen/`、最小 query `PingHealthCheck`）
- Dockerfile（PR3 / multi-stage / distroless static / nonroot、`cmd/api` のみ）
- docker-compose（PR3 で初版、PR6 でローカル開発用に動作確証 / `postgres:16-alpine` + api）

PR6 までで **未実装**:

- 各集約 DDL（`photobooks` / `images` / `image_variants` / `sessions` / `upload_verification_sessions` / `reports` / `usage_windows` / `manage_url_deliveries` / `moderation_actions` / `outbox_events` / `photobook_ogp_images` 等）は後続 PR
- 各集約 sqlc query / Repository / UseCase / Handler は後続 PR
- CORS / Origin / Auth middleware は後続 PR
- R2 / Turnstile / SendGrid は各集約段階
- Outbox / cmd/outbox-worker / cmd/ops は後続ステップ
- Cloud Run deploy / Cloud SQL（M2 早期 §F-2）

→ これらは [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §4 の順序で **PR7 以降**に順次追加する。

## ローカル開発フロー（PR6）

PR6 でのローカル開発の前提:

- `backend` （PostgreSQL + API）を docker-compose で起動する。**frontend は本 compose に含めない**（`npm --prefix frontend run dev` で別起動）
- Cloud SQL は使わない。Cloud Run / Cloud Run Jobs / Cloud Scheduler / Workers 実 deploy も行わない（独自ドメイン取得後の別 PR）
- R2 / Turnstile / SendGrid は **未接続**。各集約の実装 PR（Image / Photobook / ManageUrlDelivery 等）で個別に組み込む
- mailpit / 自動 migration entrypoint / cmd/outbox-worker / cmd/ops は本 PR には入れない

### A. DB なし（`go run` のみ、`/readyz` は 503 db_not_configured）

```sh
PORT=18083 APP_ENV=local go -C backend run ./cmd/api
curl -i http://localhost:18083/health   # 期待: 200 {"status":"ok"}
curl -i http://localhost:18083/readyz   # 期待: 503 {"status":"db_not_configured"}
```

### B. docker-compose で PostgreSQL + API を起動

```sh
# 起動（image をビルド + postgres healthy を待ってから api 起動）
docker compose -f backend/docker-compose.yaml up -d --build

# 状態確認
docker compose -f backend/docker-compose.yaml ps

# ヘルスチェック（migration 前でも /readyz は 200 ready）
curl -i http://localhost:8080/health   # 期待: 200 {"status":"ok"}
curl -i http://localhost:8080/readyz   # 期待: 200 {"status":"ready"}
```

> `/readyz` は PR3 時点では `pool.Ping(ctx)` のみを見ており、`_health_check` テーブル存在には依存しない。
> migration 前後で **どちらも 200 ready** が期待値（PR6 動作確証で確認済み）。テーブル単位の検証は集約実装と合わせて後続 PR で追加する。

### C. migration の適用

`goose` は別途インストール不要。`go run` 経由で実行する。

```sh
# DATABASE_URL は同じシェルで export してから実行する
# （DATABASE_URL=... go run ... "$DATABASE_URL" は外側シェルで $DATABASE_URL が空展開されるので NG）
export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'

go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 \
  -dir migrations postgres "$DATABASE_URL" up
```

migration 後は `goose_db_version` / `_health_check` テーブルが作成される。

### D. 停止 / 後片付け

```sh
# 停止のみ（volume は残す）
docker compose -f backend/docker-compose.yaml down

# volume も含めて完全削除（次回起動で migration をやり直したいとき）
docker compose -f backend/docker-compose.yaml down -v
```

> 過去事例: M1 PoC で残った古い volume が新しい credentials と衝突して SASL auth fail を起こした
> （`harness/failure-log/` 系）。本実装でも `docker compose down -v` でクリーンアップしてから再起動するのが安全。

### E. frontend は別起動

frontend は本 compose に含めず、別ターミナルで起動する（[`frontend/README.md`](../frontend/README.md) も参照）。

```sh
npm --prefix frontend run dev   # http://localhost:3000
```

理由:

- frontend は Next.js dev server / OpenNext build / Workers preview の切替が頻繁にあり、compose で固定すると開発体験が悪化する
- frontend のビルドコンテキストは backend と独立（依存関係 / 言語ランタイムが異なる）

### F. password 等の方針

- docker-compose の `POSTGRES_PASSWORD` 既定値 `vrcpb_local` は **ローカル開発専用の弱い既定値**。本番には絶対に流用しない
- 本番 DSN は Secret Manager から Cloud Run env vars 経由で注入する（M2 早期 §F-2 / 後続 PR）
- `.env` / `.env.local` は git 管理外、Docker build context にも `.dockerignore` で持ち込まない

### sqlc コード生成

```sh
# 公式バイナリ or `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0` で導入
sqlc -f backend/sqlc.yaml generate
```

PR3 の sqlc は `internal/database/sqlcgen/` に生成される（最小 query `PingHealthCheck` のみ、後続 PR で各集約に分割）。

### 終了 / ログ

`Ctrl+C`（SIGINT）または `SIGTERM` で 10 秒以内に graceful shutdown。
起動 / 停止 / DB 接続状況は slog JSON で stdout に出る:

```json
{"time":"...","level":"INFO","msg":"db pool configured","env":"local"}
{"time":"...","level":"INFO","msg":"server starting","env":"local","port":"8080"}
{"time":"...","level":"INFO","msg":"shutdown initiated","env":"local"}
{"time":"...","level":"INFO","msg":"shutdown complete","env":"local"}
```

`DATABASE_URL` が未設定なら `"db not configured; /readyz will return 503 db_not_configured"` が出る。

## 開発コマンド

```sh
go -C backend vet ./...
go -C backend build ./...
go -C backend test ./...
```

> WSL では `cd` は使わず `-C backend` で固定（[`.agents/rules/wsl-shell-rules.md`](../.agents/rules/wsl-shell-rules.md)）。

## ディレクトリ（PR6 時点）

```
backend/
├── go.mod / go.sum
├── .gitignore / .dockerignore
├── .env.example
├── README.md（本書）
├── Dockerfile                 # multi-stage / distroless / nonroot（PR3、cmd/api のみ）
├── docker-compose.yaml        # postgres + api（ローカル開発用）
├── sqlc.yaml                  # sqlc 設定（pgx/v5 出力）
├── migrations/
│   └── 00001_create_health_check.sql   # PR3 基盤確認用、後続 PR で集約 DDL を追加
├── cmd/
│   └── api/main.go            # HTTP server 起動 + graceful shutdown + pgx pool
└── internal/
    ├── config/
    │   └── config.go          # APP_ENV / PORT / DATABASE_URL（os.Getenv 最小実装）
    ├── database/
    │   ├── pool.go            # pgx/v5 pool（DSN 空時 nil 返し）
    │   ├── queries/health.sql # 最小 query
    │   └── sqlcgen/           # sqlc 生成物（コミット対象）
    ├── http/
    │   └── router.go          # chi router 組み立て + pool 受け渡し
    ├── health/
    │   └── handler.go         # /health / /readyz（pool 状態で分岐）
    └── shared/
        └── logging.go         # slog JSON logger（中央マスキングは後続 PR）
```

PR7 以降の構造拡張は [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §3 を参照。

## CI

PR1 で以下の最小 GitHub Actions を追加:

- `go vet ./...`
- `go build ./...`
- `go test ./...`

`golangci-lint` / `sqlc-check` / `goose-check` / Docker build の CI 化は **PR7 以降**で段階的に追加する。
PR6 では Docker build はローカル確認のみで、CI には載せていない。

## ヘルスチェックパスの方針

- **`/health`** を Cloud Run / 本番監視 / startup probe / liveness probe 用の正式パスとして採用。
- `/healthz` は採用しない（Cloud Run / Google Frontend が intercept する事象を M1 で確認、[`harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md`](../harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md)）。

## セキュリティ

- `.env` / `.env.local` は git 管理外（[`.gitignore`](./.gitignore)）。
- Secret 値は本ディレクトリには書かない。本番では Secret Manager 経由で注入。
- `.env.example` にはキー名と形式のみ記載。

## 関連ドキュメント

- [M2 実装ブートストラップ計画](../docs/plan/m2-implementation-bootstrap-plan.md)
- [プロジェクト全体ロードマップ](../harness/work-logs/2026-04-26_project-roadmap-overview.md)
- [業務知識 v4](../docs/spec/vrc_photobook_business_knowledge_v4.md)
- [ADR-0001 技術スタック](../docs/adr/0001-tech-stack.md)
