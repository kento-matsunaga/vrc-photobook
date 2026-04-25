# backend/

VRC PhotoBook の **本実装 Backend**。M2 以降の段階的 PR で機能を追加していく。

## 位置付け

- 本ディレクトリは **M2 本実装** であり、`harness/spike/backend/`（M1 PoC）とは別物。
- PoC コードを**直接コピペで流用しない**方針（[`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §11）。
- 本実装は [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../docs/spec/vrc_photobook_business_knowledge_v4.md) / ADR-0001〜0005 / 各集約ドメイン設計 / [`.agents/rules/domain-standard.md`](../.agents/rules/domain-standard.md) / [`.agents/rules/testing.md`](../.agents/rules/testing.md) に厳密に従う。

## 現在のスコープ（PR1）

PR1 では **最小骨格**のみ:

- HTTP server 起動（chi）
- `GET /health` → 200 `{"status":"ok"}`
- `PORT` 環境変数（未設定時 8080）

PR1 で **未実装**:

- `/readyz` / config 本格実装 / logger（slog）/ graceful shutdown
- DB 接続 / goose / sqlc
- Dockerfile / docker-compose
- CORS / Origin / Auth / Outbox
- R2 / Turnstile / SendGrid
- Cloud Run deploy / Cloud SQL

→ これらは [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §4 の順序で **PR2 以降**に順次追加する。

## ローカル起動

```sh
go -C backend run ./cmd/api
# 別ターミナル:
curl -i http://localhost:8080/health
# 期待: HTTP 200, body {"status":"ok"}
```

ポート変更:

```sh
PORT=8090 go -C backend run ./cmd/api
```

## 開発コマンド

```sh
go -C backend vet ./...
go -C backend build ./...
go -C backend test ./...
```

> WSL では `cd` は使わず `-C backend` で固定（[`.agents/rules/wsl-shell-rules.md`](../.agents/rules/wsl-shell-rules.md)）。

## ディレクトリ（PR1 時点）

```
backend/
├── go.mod
├── .gitignore
├── .env.example
├── README.md（本書）
├── cmd/
│   └── api/main.go            # HTTP server 起動エントリ
└── internal/
    ├── http/                  # chi router 組み立て
    │   └── router.go
    └── health/                # ヘルスチェック
        └── handler.go
```

PR2 以降の構造拡張は [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §3 を参照。

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
