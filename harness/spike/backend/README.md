# M1 Spike: Backend PoC

> **目的**: M1 スパイク検証計画 [`docs/plan/m1-spike-plan.md`](../../../docs/plan/m1-spike-plan.md) の優先順位 3 に対応する最小 PoC。
>
> Go 1.24+（PoC は 1.23 で動作確認）/ chi / pgx / sqlc / goose / Cloud Run + PostgreSQL の最小構成が成立するかを確認する。本実装には流用しない。

---

## 重要な前提

- **本実装ディレクトリ `backend/` は触らない**。本 PoC は `harness/spike/backend/` に閉じる。
- **PoC コードを本実装に流用しない**。M2 の本実装は `domain-standard.md` のディレクトリ構造で別途書き直す。
- **秘密情報・実値・APIキー・DB パスワードをコミットしない**。`.env` 系は `.gitignore` 対象。`.env.example` のみコミット。
- **token / Cookie / presigned URL / DB エラーメッセージ詳細をログ・レスポンスに出さない**設計（漏洩抑止）。
- 実装は粗くてよい。ただし検証手順とその結果記入欄は明確にする。

---

## 構成

```
harness/spike/backend/
├── README.md                         # 本書
├── go.mod / go.sum                   # Go モジュール（PoC 用）
├── .gitignore
├── .env.example                      # 環境変数キーのサンプル（実値ではない）
├── Dockerfile                        # multi-stage / distroless / nonroot
├── docker-compose.yaml               # ローカル PostgreSQL（実環境ではない）
├── sqlc.yaml                          # sqlc 設定
├── cmd/
│   └── api/main.go                   # chi サーバ起動
├── internal/
│   ├── config/config.go              # 環境変数読み込み（標準 os.Getenv のみ）
│   ├── db/
│   │   ├── pool.go                   # pgx の最小プール
│   │   ├── queries/test_alive.sql    # sqlc 生成元クエリ
│   │   └── sqlcgen/                  # sqlc 生成物（コミット対象、再生成可）
│   ├── health/handler.go             # /healthz, /readyz
│   └── sandbox/db_ping.go            # /sandbox/db-ping
└── migrations/
    └── 00001_create_test_alive.sql   # goose migration（最小、PoC 専用テーブル）
```

### 実装したエンドポイント

| メソッド | パス | 用途 | DB 接続 |
|---------|-----|------|:-------:|
| GET | `/healthz` | Cloud Run startup / liveness probe 用。プロセス自体の生存のみ返す | 不要 |
| GET | `/readyz` | DB 接続込みの readiness。pgx プール `Ping` で判定 | 必要 |
| GET | `/sandbox/db-ping` | `SELECT now()` 実行結果を JSON で返す PoC 用検証エンドポイント | 必要 |

レスポンスに **DB エラーメッセージや token 値は含めない**。サーバ側ログでのみ追跡する設計。

### 採用したライブラリ

| 種別 | 採用 | バージョン |
|------|------|-----------|
| HTTP ルーター | `github.com/go-chi/chi/v5` | v5.1.0 |
| chi middleware | RequestID / RealIP / Recoverer / Timeout | （chi 標準） |
| DB ドライバ・プール | `github.com/jackc/pgx/v5/pgxpool` | v5.7.1 |
| Migration | `github.com/pressly/goose/v3` (CLI) | v3.22.0（go run 経由で実行） |
| Code generation (SQL → Go) | `sqlc` (CLI) | v1.30.0 |
| ロガー | 標準 `log/slog`（JSON ハンドラ） | Go 1.21+ 標準 |

ORM は採用しない（ADR-0001 §coding-rules: 明示的 > 暗黙的、any/interface{} 禁止と整合）。

### Go バージョン

PoC ローカル環境: **Go 1.23.2**。ADR-0001 で本実装は Go 1.24+ と確定しているが、PoC は 1.23 で動作確認した。M2 本実装着手前に 1.24 へアップデートする。`go.mod` の `go 1.23` を `go 1.24` に書き換えれば良いだけのため、本 PoC では阻害要因にならない。

---

## ローカル検証手順

### 前提

- Go 1.23+（推奨 1.24+）
- Docker + Docker Compose
- `sqlc` CLI（任意、PoC では go run 経由でも代替可）
- PostgreSQL クライアント（任意、`docker exec ... psql` で代替可）

### 1. 依存解決

```sh
go mod tidy -C harness/spike/backend
```

### 2. ローカル PostgreSQL 起動

```sh
docker compose -f harness/spike/backend/docker-compose.yaml up -d
```

`vrcpb_spike` ユーザー / `vrcpb_spike` データベース / port 5432。サンプル値は `.env.example` 参照。

### 3. Migration 実行（goose）

```sh
go run -C harness/spike/backend \
  github.com/pressly/goose/v3/cmd/goose@v3.22.0 \
  -dir migrations \
  postgres \
  'postgres://vrcpb_spike:spike_local_password@localhost:5432/vrcpb_spike?sslmode=disable' \
  up
```

`_test_alive` テーブルが作成される（PoC 専用）。

### 4. sqlc コード生成

```sh
sqlc generate -f harness/spike/backend/sqlc.yaml
```

`internal/db/sqlcgen/{db.go, models.go, test_alive.sql.go}` が生成される。生成物はコミット対象。

### 5. ビルド + 起動

```sh
go build -C harness/spike/backend -o /tmp/spike-api ./cmd/api

DATABASE_URL='postgres://vrcpb_spike:spike_local_password@localhost:5432/vrcpb_spike?sslmode=disable' \
PORT=8090 APP_ENV=local /tmp/spike-api
```

注意: ホスト環境で 8080 が他サービスに使われている場合、`PORT=8090` 等で別ポートに切り替える。Cloud Run では `PORT` 環境変数が自動注入される。

### 6. エンドポイント検証

```sh
curl -sS -i http://localhost:8090/healthz
curl -sS -i http://localhost:8090/readyz
curl -sS -i http://localhost:8090/sandbox/db-ping
```

### 7. Docker build

```sh
docker build -t vrcpb-spike-backend:latest harness/spike/backend
```

### 8. Docker container 動作確認（compose ネットワーク経由で DB 接続）

```sh
docker run -d --rm --name spike-api-test \
  --network backend_default \
  -e DATABASE_URL='postgres://vrcpb_spike:spike_local_password@postgres:5432/vrcpb_spike?sslmode=disable' \
  -e PORT=8080 \
  -p 8091:8080 \
  vrcpb-spike-backend:latest

curl -sS http://localhost:8091/healthz
curl -sS http://localhost:8091/readyz
curl -sS http://localhost:8091/sandbox/db-ping

docker stop spike-api-test
```

### 9. クリーンアップ

```sh
docker compose -f harness/spike/backend/docker-compose.yaml down -v
docker image rm vrcpb-spike-backend:latest
```

---

## 検証結果（2026-04-25 CLI 検証）

| 項目 | 結果 | 備考 |
|------|:---:|------|
| `go mod tidy` | ✅ | chi v5.1.0 / pgx v5.7.1 + transitive deps が解決 |
| `go vet ./...` | ✅ | エラーなし |
| `go test ./...` | ✅ | テストファイルゼロ（PoC のため）。実装に問題なし |
| `go build -o /tmp/spike-api ./cmd/api` | ✅ | 約 15MB の x86_64 ELF バイナリ |
| `docker compose up -d`（PostgreSQL 16-alpine） | ✅ | healthcheck で `pg_isready` 通過 |
| `goose ... up` | ✅ | `00001_create_test_alive.sql` migration 適用、5.9ms |
| `sqlc generate` | ✅ | 3 ファイル（`db.go`, `models.go`, `test_alive.sql.go`）生成 |
| ローカル直起動 + `/healthz` | ✅ | `200 OK {"status":"ok"}` |
| ローカル直起動 + `/readyz`（DB 接続あり） | ✅ | `200 OK {"status":"ready"}` |
| ローカル直起動 + `/sandbox/db-ping` | ✅ | `SELECT now()` の結果を JSON 返却 |
| 未存在ルート（404） | ✅ | chi の標準 404 ハンドラ |
| Graceful shutdown（SIGINT） | ✅ | `shutdown initiated` → `shutdown complete` |
| `docker build` | ✅ | multi-stage / distroless / nonroot で成功 |
| **イメージサイズ** | ✅ | **12.4MB**（distroless static-debian12 ベース、Cloud Run 互換） |
| Docker container + DB 接続（compose ネットワーク経由） | ✅ | `/healthz` / `/readyz` / `/sandbox/db-ping` すべて成功 |
| ログ漏洩確認（slog JSON で token / cookie / DSN を出していない） | ✅ | 出力は `port` / `env` / `error.error` のみ。DSN・パスワードは含まれない |

---

## Cloud Run へ載せるための環境変数（M2 本実装で整備）

`.env.example` に詳細記載。本 PoC で実際に使ったキー:

| 環境変数 | M1 PoC | 本実装での扱い |
|---------|:-----:|---------------|
| `PORT` | ローカル 8090 / container 8080 | Cloud Run が自動注入（変更不要） |
| `APP_ENV` | `local` / `container` | `local` / `staging` / `production` |
| `DATABASE_URL` | docker compose の DSN | **Secret Manager 経由で注入**（Cloud SQL Auth Proxy or 直接 DSN） |

**M2 本実装で追加が必要な環境変数（PoC では未使用）**:

| 環境変数 | 用途 | 取得元 |
|---------|------|-------|
| `R2_ACCOUNT_ID` | Cloudflare R2 接続 | Cloudflare ダッシュボード |
| `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` | R2 API 認証 | R2 トークン発行 → Secret Manager |
| `R2_BUCKET_NAME` | R2 バケット名 | 環境別 |
| `R2_ENDPOINT` | R2 エンドポイント URL | `https://<R2_ACCOUNT_ID>.r2.cloudflarestorage.com` |
| `IP_HASH_SALT_V1` 等 | IP ハッシュソルト（v4 §3.7） | Secret Manager |
| `IP_HASH_SALT_CURRENT_VERSION` | 現在のソルトバージョン | 設定値（例: `1`） |
| `SESSION_TOKEN_HASH_PEPPER`（検討中） | session token hash の追加 pepper（必要なら） | Secret Manager |

すべての Secret は **Cloud Run 環境変数経由で Secret Manager から注入**。コードベース・コミットには絶対に含めない（ADR-0001 / ADR-0002 / ADR-0005 各セキュリティ方針と整合）。

---

## Cloud Run へ進めるうえでの未確認事項（M1 残作業）

PoC として CLI 検証は完了したが、以下は実環境が必要なため未確認:

### Cloud Run デプロイ動作

- [ ] `gcloud run deploy` で実際にデプロイ
- [ ] Cloud Run のコールドスタート時間計測（ADR-0001 §結果デメリットで言及）
- [ ] Cloud SQL Auth Proxy 経由 / 直接接続のレイテンシ計測
- [ ] Cloud Logging に slog JSON が正しくパースされるか確認（severity マッピング）
- [ ] Cloud Run の SIGTERM → graceful shutdown が 10 秒内で完了するか

### R2 接続検証（M1 優先順位 4 で別途扱う）

- [ ] aws-sdk-go-v2 で R2 へ HeadBucket / ListObjects
- [ ] presigned URL 発行（PUT、15分）
- [ ] Cloud Run 東京 ↔ R2 のレイテンシ計測（ADR-0001 §クロスクラウド注意）
- [ ] HEIC 変換用 libheif 同梱コンテナの構築（ADR-0005 / 未解決事項）

### Frontend / Backend 結合検証（次工程）

- [ ] Backend を `*.run.app` にデプロイ
- [ ] Frontend PoC（OpenNext、`harness/spike/frontend/`）から Backend に Cookie 付きで API 呼び出し
- [ ] **Cookie Domain 属性の決定**（U2、ADR-0003）: 異なるホスト構成で Cookie 共有のためどう設定するか
- [ ] Origin ヘッダ検証 + CORS 設定の動作確認
- [ ] CSRF 対策（SameSite=Strict + Origin 検証）の実機動作確認

---

## ADR / M1計画 へのフィードバック

本 PoC で発見した点で、ADR / M1計画 / 業務知識へ反映が必要そうな項目:

1. **Go バージョン**: ローカル開発環境で Go 1.23.2 が動作確認済み。ADR-0001 §採用技術 で `Go 1.24+` を維持しつつ「PoC は 1.23 で動作確認、本実装着手時に 1.24 へ移行」とメモを追加してよい
2. **Cloud Run Dockerfile 構成のテンプレート**: distroless static + nonroot + multi-stage で **12.4MB** に収まることを確認。M2 本実装の Dockerfile はこのテンプレートをベースに `domain-standard.md` 構造に合わせて整備
3. **Graceful shutdown 実装パターン**: `signal.NotifyContext` + `srv.Shutdown(ctx)` の最小実装を確立。本実装でもこのパターンを踏襲
4. **Migration ツール選定**: `goose v3.22.0` で `-- +goose Up/Down` 注釈付き SQL が問題なく動作することを確認。`sqlc v1.30.0` の `pgx/v5` 出力も成立。ADR-0001 §採用技術 で確定済みのライブラリで問題なし
5. **404 ハンドラ**: chi 標準で十分。明示的な 404 JSON レスポンスは M2 で middleware 化する想定

これらは別コミットで反映予定。

---

## 既知の制限・未検証事項

- **テストコードはゼロ**（PoC のため）。本実装では `domain-standard.md` / `testing.md` 準拠でテーブル駆動テスト + Builder パターンを必須化する
- **認証 / 認可・session 検証ミドルウェア**は本 PoC に含まない（ADR-0003 / Frontend PoC で別検証）
- **R2 / Turnstile / Outbox / Email Provider** は本 PoC に含まない（M1 優先順位 4 以降の別 PoC で扱う）
- **ログ構造化**: slog JSON のフィールド設計は最小限（`time`, `level`, `msg`, 任意の attr）。本実装では request_id / photobook_id / actor_label 等のフィールド標準化を行う
- **メトリクス・トレーシング**は未導入（OpenTelemetry / Cloud Trace は M6 想定）
- **CORS / Origin 検証 middleware** は未実装（Frontend PoC との結合検証時に追加する）

---

## トラブルシューティング

### `bind: address already in use`

ホスト環境で 8080 が他のサービスに使われている可能性。`PORT=8090` 等で別ポートに切り替える。本 PoC 環境では Apache + WordPress が 8080 で動いていたため、検証は **8090** で実施した。

### goose で `dial unix /tmp/.s.PGSQL.5432` エラー

引数の DSN がシェル展開されず libpq のフォールバック動作になっている可能性。`DATABASE_URL` を **直接シングルクォートで埋める**形にする（README 手順 3 参照）。`DATABASE_URL=... goose ... "$DATABASE_URL"` のように同行で先に環境変数を設定すると、その行内では展開されない。

### Cloud Run コンテナ動作確認

ローカル `docker run` で動くなら Cloud Run でも同等動作する想定。ただし起動順序（startup probe → liveness probe → traffic）と SIGTERM ハンドリングは Cloud Run 上で別途確認する。

---

## ライセンス / 取扱い

本 PoC は内部検証のみを目的とする。外部公開・本実装流用は禁止。
