# backend/

VRC PhotoBook の **本実装 Backend**。M2 以降の段階的 PR で機能を追加していく。

## 位置付け

- 本ディレクトリは **M2 本実装** であり、`harness/spike/backend/`（M1 PoC）とは別物。
- PoC コードを**直接コピペで流用しない**方針（[`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §11）。
- 本実装は [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../docs/spec/vrc_photobook_business_knowledge_v4.md) / ADR-0001〜0005 / 各集約ドメイン設計 / [`.agents/rules/domain-standard.md`](../.agents/rules/domain-standard.md) / [`.agents/rules/testing.md`](../.agents/rules/testing.md) に厳密に従う。

## 現在のスコープ（〜 PR3）

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
- docker-compose（PR3 / `postgres:16-alpine` + api）

PR3 までで **未実装**:

- 各集約 DDL（`photobooks` / `images` / `image_variants` / `sessions` / `upload_verification_sessions` / `reports` / `usage_windows` / `manage_url_deliveries` / `moderation_actions` / `outbox_events` / `photobook_ogp_images` 等）は後続 PR
- 各集約 sqlc query / Repository / UseCase / Handler は後続 PR
- CORS / Origin / Auth middleware は後続 PR
- R2 / Turnstile / SendGrid は各集約段階
- Outbox / cmd/outbox-worker / cmd/ops は後続ステップ
- Cloud Run deploy / Cloud SQL（M2 早期 §F-2）

→ これらは [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §4 の順序で **PR4 以降**に順次追加する。

## ローカル起動

### A. DB なし（`go run` のみ、`/readyz` は 503 db_not_configured）

```sh
PORT=18083 APP_ENV=local go -C backend run ./cmd/api
curl -i http://localhost:18083/health   # 期待: 200 {"status":"ok"}
curl -i http://localhost:18083/readyz   # 期待: 503 {"status":"db_not_configured"}
```

### B. docker-compose で PostgreSQL + API を起動（`/readyz` 200 ready）

```sh
# 起動
docker compose -f backend/docker-compose.yaml up -d

# migration を適用（goose は go run 経由で実行、別途インストール不要）
DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable' \
  go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 \
  -dir migrations postgres "$DATABASE_URL" up

# ヘルスチェック
curl -i http://localhost:8080/health   # 期待: 200 {"status":"ok"}
curl -i http://localhost:8080/readyz   # 期待: 200 {"status":"ready"}

# 後片付け（ボリューム含めて完全削除）
docker compose -f backend/docker-compose.yaml down -v
```

> docker-compose の `POSTGRES_PASSWORD` は `vrcpb_local`（**ローカル開発用の弱い既定値**）。
> 本番には絶対に流用しない。本番 DSN は Secret Manager から Cloud Run env vars 経由で注入する。

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

## ディレクトリ（PR3 時点）

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

PR4 以降の構造拡張は [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §3 を参照。

## CI

PR1 で以下の最小 GitHub Actions を追加:

- `go vet ./...`
- `go build ./...`
- `go test ./...`

`golangci-lint` / `sqlc-check` / `goose-check` / frontend CI は **PR2 以降**で段階的に追加する。

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
