# backend/

VRC PhotoBook の **本実装 Backend**。M2 以降の段階的 PR で機能を追加していく。

## 位置付け

- 本ディレクトリは **M2 本実装** であり、`harness/spike/backend/`（M1 PoC）とは別物。
- PoC コードを**直接コピペで流用しない**方針（[`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §11）。
- 本実装は [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../docs/spec/vrc_photobook_business_knowledge_v4.md) / ADR-0001〜0005 / 各集約ドメイン設計 / [`.agents/rules/domain-standard.md`](../.agents/rules/domain-standard.md) / [`.agents/rules/testing.md`](../.agents/rules/testing.md) に厳密に従う。

## 現在のスコープ（〜 PR2）

- HTTP server 起動（chi）
- `GET /health` → 200 `{"status":"ok"}`（PR1）
- `GET /readyz` → 503 `{"status":"db_not_configured"}`（PR2、DB 未実装のため固定。PR3 で pgx pool 状態に応じて 200/503 を返すハンドラに置き換える）
- `PORT` / `APP_ENV` 環境変数の最小読み込み（PR2 / `internal/config`）
- slog JSON logger（PR2 / `internal/shared/logging.go`）
- graceful shutdown（SIGINT / SIGTERM、10 秒 timeout）（PR2）

PR2 までで **未実装**:

- DB 接続 / goose / sqlc（PR3）
- Dockerfile / docker-compose（PR3 / PR6）
- CORS / Origin / Auth middleware（PR4 以降の auth ステップ）
- R2 / Turnstile / SendGrid（各集約段階）
- Outbox / cmd/outbox-worker / cmd/ops（後続ステップ）
- Cloud Run deploy / Cloud SQL（M2 早期 §F-2 / 本実装デプロイ計画）

→ これらは [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §4 の順序で **PR3 以降**に順次追加する。

## ローカル起動

```sh
go -C backend run ./cmd/api
# 別ターミナル:
curl -i http://localhost:8080/health   # 期待: 200 {"status":"ok"}
curl -i http://localhost:8080/readyz   # 期待: 503 {"status":"db_not_configured"}（PR2 時点）
```

ポート変更:

```sh
PORT=8090 APP_ENV=local go -C backend run ./cmd/api
```

終了は `Ctrl+C`（SIGINT）または `SIGTERM`。10 秒以内に graceful shutdown する。
起動 / 停止ログは slog JSON で stdout に出る:

```json
{"time":"...","level":"INFO","msg":"server starting","env":"local","port":"8080"}
{"time":"...","level":"INFO","msg":"shutdown initiated","env":"local"}
{"time":"...","level":"INFO","msg":"shutdown complete","env":"local"}
```

## 開発コマンド

```sh
go -C backend vet ./...
go -C backend build ./...
go -C backend test ./...
```

> WSL では `cd` は使わず `-C backend` で固定（[`.agents/rules/wsl-shell-rules.md`](../.agents/rules/wsl-shell-rules.md)）。

## ディレクトリ（PR2 時点）

```
backend/
├── go.mod
├── go.sum
├── .gitignore
├── .env.example
├── README.md（本書）
├── cmd/
│   └── api/main.go            # HTTP server 起動 + graceful shutdown
└── internal/
    ├── config/
    │   └── config.go          # 環境変数読み込み（os.Getenv 最小実装）
    ├── http/
    │   └── router.go          # chi router 組み立て
    ├── health/
    │   └── handler.go         # /health / /readyz
    └── shared/
        └── logging.go         # slog JSON logger（中央マスキングは PR3 以降）
```

PR3 以降の構造拡張は [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §3 を参照。

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
