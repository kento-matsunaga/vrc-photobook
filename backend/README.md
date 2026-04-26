# backend/

VRC PhotoBook の **本実装 Backend**。M2 以降の段階的 PR で機能を追加していく。

## 位置付け

- 本ディレクトリは **M2 本実装** であり、`harness/spike/backend/`（M1 PoC）とは別物。
- PoC コードを**直接コピペで流用しない**方針（[`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §11）。
- 本実装は [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../docs/spec/vrc_photobook_business_knowledge_v4.md) / ADR-0001〜0005 / 各集約ドメイン設計 / [`.agents/rules/domain-standard.md`](../.agents/rules/domain-standard.md) / [`.agents/rules/testing.md`](../.agents/rules/testing.md) に厳密に従う。

## 現在のスコープ（〜 PR9b）

- HTTP server 起動（chi）
- `GET /health` → 200 `{"status":"ok"}`（PR1）
- `GET /readyz`（PR2 で枠、PR3 で pgx pool 状態に応じた分岐）:
  - `pool == nil`（`DATABASE_URL` 未設定） → 503 `{"status":"db_not_configured"}`
  - `pool.Ping(ctx)` 失敗 → 503 `{"status":"db_unreachable"}`
  - 成功 → 200 `{"status":"ready"}`
- `PORT` / `APP_ENV` / `DATABASE_URL` の最小読み込み（PR2-3 / `internal/config`）
- slog JSON logger（PR2 / `internal/shared/logging.go`、PR7 で禁止フィールド方針を拡充）
- graceful shutdown（SIGINT / SIGTERM、10 秒 timeout）（PR2）
- pgx/v5 接続プール（PR3 / `internal/database/pool.go`、DSN 空時 nil 返し）
- goose migration 2 本（PR3 `_health_check`、PR7 `sessions`）
- sqlc 集約別分割（PR3 `internal/database/sqlcgen/` ＋ PR7 `internal/auth/session/.../sqlcgen/`）
- Dockerfile（PR3 / multi-stage / distroless static / nonroot、`cmd/api` のみ）
- docker-compose（PR3 で初版、PR6 でローカル開発用に動作確証 / `postgres:16-alpine` + api）
- **Session 認可機構の単体（PR7）**:
  - `internal/auth/session/domain/`（VO + `Session` エンティティ）
  - `internal/auth/session/cookie/`（Cookie policy、Set-Cookie ヘッダ生成器）
  - `internal/auth/session/infrastructure/repository/rdb/`（sqlc 生成物 + Repository + marshaller + tests Builder）
  - draft / manage 汎用 1 本テーブル `sessions`、`session_type` 分岐
  - SessionToken は 32B `crypto/rand` + base64url、DB 保存は SHA-256 32B のみ
- **Session UseCase + middleware 枠（PR8）**:
  - `internal/auth/session/internal/usecase/`: `IssueDraftSession` / `IssueManageSession` /
    `ValidateSession` / `TouchSession` / `RevokeSession` / `RevokeAllDrafts` /
    `RevokeAllManageByTokenVersion`
  - `internal/auth/session/internal/usecase/tests/fake_repository.go`: in-memory fake repository
  - `internal/auth/session/middleware/`: `RequireDraftSession` / `RequireManageSession` /
    `SessionFromContext`（chi 互換 middleware、Cookie 名→raw token→hash→Repository 照合）
  - **本番 router からは未接続**。dummy token で動く公開 endpoint は作らない。token 交換
    HTTP endpoint は PR9c（Photobook aggregate 接続後）で追加する
- **Photobook 集約 domain + repository（PR9a）**:
  - `internal/photobook/domain/`: VO（PhotobookID / PhotobookStatus / PhotobookType /
    PhotobookLayout / OpeningStyle / Visibility / Slug / DraftEditToken / DraftEditTokenHash /
    ManageUrlToken / ManageUrlTokenHash / ManageUrlTokenVersion）+ Photobook entity
  - `internal/photobook/infrastructure/repository/rdb/`: sqlc 生成物 + Repository + marshaller
  - migration: `00003_create_photobooks.sql` + `00004_add_photobooks_fk_to_sessions.sql`
- **Photobook UseCase + 同一 TX 接続（PR9b）**:
  - `internal/database/tx.go`: `WithTx` ヘルパ（pgx pool → Tx → fn(tx) → Commit/Rollback）
  - `internal/photobook/internal/usecase/`: 6 UseCase
    - `CreateDraftPhotobook`（draft 作成 + raw DraftEditToken 返却）
    - `TouchDraft`（draft_expires_at 延長）
    - `ExchangeDraftTokenForSession`（raw draft token 検証 → IssueDraftSession、touchDraft は呼ばない）
    - `ExchangeManageTokenForSession`(`raw manage token 検証 → IssueManageSession`、token_version_at_issue snapshot)
    - `PublishFromDraft`（**WithTx 内**で repository.PublishFromDraft + RevokeAllDrafts、
      session revoke 失敗時に photobook UPDATE もロールバック）
    - `ReissueManageUrl`（**WithTx 内**で repository.ReissueManageUrl + RevokeAllManageByTokenVersion、
      同様にロールバック）
  - `internal/auth/session/sessionintegration/`: session 配下の facade。Tx 起点で
    Session UseCase / Repository を呼び出すための薄い wrapper（Go internal ルール対応）
  - `internal/photobook/infrastructure/session_adapter/`: Photobook 側 ports interface
    （`DraftSessionIssuer` / `ManageSessionIssuer` / `DraftSessionRevokerFactory` /
    `ManageSessionRevokerFactory` / `PhotobookTxRepositoryFactory`）の本番実装
  - **HTTP endpoint / 本番 router 接続 / Set-Cookie は PR9c**（dummy 経由は引き続き作らない）
  - Outbox INSERT は本 PR では行わない（Outbox 本実装は別 PR）
  - raw `draft_edit_token` / `manage_url_token` / `session_token` / hash・Cookie 全体は
    ログ・diff・テストログ・レスポンスに出さない

PR9b までで **未実装**:

- 各集約 DDL（`photobooks` / `images` / `image_variants` / `sessions` / `upload_verification_sessions` / `reports` / `usage_windows` / `manage_url_deliveries` / `moderation_actions` / `outbox_events` / `photobook_ogp_images` 等）は後続 PR
- 各集約 sqlc query / Repository / UseCase / Handler は後続 PR
- CORS / Origin / Auth middleware は後続 PR
- R2 / Turnstile / SendGrid は各集約段階
- Outbox / cmd/outbox-worker / cmd/ops は後続ステップ
- Cloud Run deploy / Cloud SQL（M2 早期 §F-2）

→ これらは [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §4 / [`docs/plan/m2-photobook-session-integration-plan.md`](../docs/plan/m2-photobook-session-integration-plan.md) §13 の順序で **PR9c 以降**に順次追加する（PR9c: HTTP endpoint 接続 + 本番 router 接続）。

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

migration 後は以下のテーブルが作成される:

- `goose_db_version`（goose 内部）
- `_health_check`（PR3、基盤確認用）
- `sessions`（PR7、draft / manage 汎用 1 本、`session_type` 分岐）
- `photobooks`（PR9a、draft / published / deleted / purged 状態モデル）
- `sessions.photobook_id` への FK（PR9a の `00004` で追加）

Session / Photobook の repository test を実行する場合は up 後に同じ `DATABASE_URL` を export した状態で:

```sh
go -C backend test ./internal/auth/session/infrastructure/repository/rdb/...
go -C backend test ./internal/photobook/infrastructure/repository/rdb/...
```

`DATABASE_URL` が空のときは repository test は `t.Skip` でスキップされる（CI / DB 無し環境向け）。

> 既存ローカル DB に古い `sessions` 行が残った状態で `00004`（FK 追加）を up すると、
> `photobooks(id)` に存在しない `photobook_id` を持つ session 行が引っかかって失敗します。
> ローカルでは `docker compose down -v` でクリーンアップしてから up し直すか、
> `TRUNCATE TABLE sessions` を入れてから 00004 を流してください。

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

## ディレクトリ（PR7 時点）

```
backend/
├── go.mod / go.sum
├── .gitignore / .dockerignore
├── .env.example
├── README.md（本書）
├── Dockerfile                 # multi-stage / distroless / nonroot（PR3、cmd/api のみ）
├── docker-compose.yaml        # postgres + api（ローカル開発用、PR6 で動作確証）
├── sqlc.yaml                  # 集約別 sqlc 設定（PR3 + PR7 + PR9a で 3 セット）
├── migrations/
│   ├── 00001_create_health_check.sql                # PR3 基盤確認用
│   ├── 00002_create_sessions.sql                    # PR7 Session 認可機構
│   ├── 00003_create_photobooks.sql                  # PR9a Photobook 集約
│   └── 00004_add_photobooks_fk_to_sessions.sql      # PR9a sessions → photobooks FK
├── cmd/
│   └── api/main.go
└── internal/
    ├── photobook/                                  # PR9a
    │   ├── domain/                                  # VO + Photobook entity + tests Builder
    │   │   ├── photobook.go
    │   │   ├── tests/photobook_builder.go
    │   │   └── vo/                                  # 12 種の VO
    │   └── infrastructure/repository/rdb/           # sqlc + Repository + marshaller
    │       ├── photobook_repository.go              # CreateDraft / FindBy* / Touch / Publish / Reissue
    │       ├── marshaller/
    │       ├── queries/photobook.sql
    │       └── sqlcgen/
    ├── auth/
    │   └── session/                                # PR7 + PR8
    │       ├── cookie/                             # Cookie policy（HttpOnly / Secure / SameSite=Strict）
    │       ├── domain/                             # PR7: VO + Session エンティティ + tests Builder
    │       ├── infrastructure/repository/rdb/      # PR7: sqlc + Repository + marshaller
    │       ├── internal/                           # PR8（本サブツリー外からは import 不可）
    │       │   └── usecase/
    │       │       ├── issue_draft_session.go      # raw token 生成 + Create
    │       │       ├── issue_manage_session.go
    │       │       ├── validate_session.go         # ErrSessionInvalid を一律返す（情報漏洩抑止）
    │       │       ├── touch_session.go
    │       │       ├── revoke_session.go           # Revoke / RevokeAllDrafts / RevokeAllManageByTokenVersion
    │       │       ├── repository.go               # SessionRepository interface
    │       │       └── tests/fake_repository.go    # in-memory 実装（usecase / middleware テスト用）
    │       └── middleware/                         # PR8 RequireDraftSession / RequireManageSession / SessionFromContext
    │                                               #     （本 router からは未接続、PR9 で接続予定）
    ├── config/
    ├── database/                                   # PR3 の基盤
    ├── health/
    ├── http/
    └── shared/logging.go                           # 禁止フィールド方針（PR7 で session 関連を追記）
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
