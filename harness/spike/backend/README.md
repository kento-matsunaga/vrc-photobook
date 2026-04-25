# M1 Spike: Backend PoC

> **目的**: M1 スパイク検証計画 [`docs/plan/m1-spike-plan.md`](../../../docs/plan/m1-spike-plan.md) の優先順位 **3 + 4 + 5** に対応する最小 PoC。
>
> Go 1.24+ / chi / pgx / sqlc / goose / Cloud Run + PostgreSQL の最小構成（優先順位 3）、Cloudflare R2 への接続 / presigned PUT URL 発行 / HeadObject 確認（優先順位 4）、Cloudflare Turnstile siteverify と `upload_verification_sessions` のアトミック消費（優先順位 5）が成立するかを確認する。本実装には流用しない。
>
> **HEIC 変換 / 画像処理ワーカーは本 PoC に含めない**。これらは別 PoC（または M3〜M6）で扱う。

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
│   ├── api/main.go                   # chi サーバ起動
│   └── outbox-worker/main.go         # Outbox ワーカー CLI（M1 priority 7、Cloud Run Jobs 想定）
├── internal/
│   ├── config/config.go              # 環境変数読み込み（標準 os.Getenv のみ）
│   ├── db/
│   │   ├── pool.go                   # pgx の最小プール
│   │   ├── queries/                  # sqlc 生成元クエリ（test_alive / upload_verification / outbox）
│   │   └── sqlcgen/                  # sqlc 生成物（コミット対象、再生成可）
│   ├── health/handler.go             # /healthz, /readyz
│   ├── turnstile/client.go           # Cloudflare Turnstile siteverify クライアント（mock モード対応）
│   └── sandbox/                      # PoC 用の sandbox エンドポイント群
│       ├── db_ping.go                # /sandbox/db-ping
│       ├── r2_handlers.go            # /sandbox/r2-* （優先順位 4）
│       ├── integration_handlers.go   # /sandbox/session-check / origin-check
│       ├── turnstile_handlers.go     # /sandbox/turnstile/verify, /sandbox/upload-intent/consume
│       └── outbox_handlers.go        # /sandbox/outbox/* （優先順位 7）
├── migrations/
│   ├── 00001_create_test_alive.sql                       # goose migration（最小、PoC 専用テーブル）
│   ├── 00002_create_upload_verification_sessions.sql     # upload_verification_sessions
│   └── 00003_create_outbox_events.sql                    # outbox_events（横断 Outbox）
└── scripts/
    ├── turnstile-consume-race.sh     # 100 並列 consume レース検証（優先順位 5）
    └── outbox-process-once.sh        # outbox-worker --once / --retry-failed のラッパー（優先順位 7）
```

### 実装したエンドポイント

#### 基盤系（優先順位 3）

| メソッド | パス | 用途 | DB 接続 |
|---------|-----|------|:-------:|
| GET | `/healthz` | Cloud Run startup / liveness probe 用。プロセス自体の生存のみ返す | 不要 |
| GET | `/readyz` | DB 接続込みの readiness。pgx プール `Ping` で判定 | 必要 |
| GET | `/sandbox/db-ping` | `SELECT now()` 実行結果を JSON で返す PoC 用検証エンドポイント | 必要 |

#### R2 系（優先順位 4）

| メソッド | パス | 用途 | R2 設定 |
|---------|-----|------|:------:|
| GET | `/sandbox/r2-headbucket` | R2 への接続確認（`HeadBucket`）。成功時 `{"status":"ok"}` | 必要 |
| GET | `/sandbox/r2-list` | バケット内オブジェクトを最大 5 件列挙（key + size のみ） | 必要 |
| POST | `/sandbox/r2-presign-put` | presigned PUT URL を 15 分有効で発行。filename / content_type / byte_size を受け取り、storage_key はサーバ生成 | 必要 |
| GET | `/sandbox/r2-headobject?key=...` | R2 オブジェクトの存在 / ContentLength / ContentType / ETag を返す | 必要 |

**R2 設定が未注入のとき**、R2 系エンドポイントはすべて 503 `{"error":"r2_not_configured"}` を返す（`/healthz` は影響なく 200 を返す）。

#### Turnstile + upload_verification_session 系（優先順位 5）

| メソッド | パス | 用途 | DB 接続 |
|---------|-----|------|:-------:|
| POST | `/sandbox/turnstile/verify` | Turnstile siteverify を呼び、成功時に `upload_verification_session` を発行（30 分 / 20 intent 上限）。raw token はレスポンスのみ、DB は SHA-256 ハッシュのみ保存 | 必要 |
| POST | `/sandbox/upload-intent/consume` | アトミック条件 UPDATE で upload intent を 1 消費。残数枯渇 / 期限切れ / revoked / hash 不一致 / photobook_id 不一致のいずれも 403 `consume_rejected` に集約 | 必要 |

**Turnstile が mock モード**（`TURNSTILE_SECRET_KEY` 空）のとき、起動ログに `running in MOCK mode (PoC only)` の WARN を出す。mock 規則:

| token | mock の判定 |
|------|-----------|
| 空文字列 | 400 `turnstile_token_required`（事前バリデーション） |
| `MOCK_FAIL` を含む | 403 `turnstile_rejected`（強制失敗） |
| その他 | 200 + セッション発行 |

**実 Cloudflare 検証** には公開サンドボックス secret を使う:

- 必ず success: `1x0000000000000000000000000000000AA`
- 必ず failure: `2x0000000000000000000000000000000AA`

#### Outbox + 自動 reconciler 系（優先順位 7）

| メソッド | パス | 用途 | DB 接続 |
|---------|-----|------|:-------:|
| POST | `/sandbox/outbox/enqueue` | 検証用 Outbox イベントを単発 INSERT。本実装では集約の状態変更と同一 TX で呼ばれる（cross-cutting/outbox.md §2）| 必要 |
| POST | `/sandbox/outbox/process-once?limit=N` | pending を最大 N 件 claim（`FOR UPDATE SKIP LOCKED`）して mock ハンドラで処理。`ImageIngestionRequested` → processed、`ForceFail` を含む event_type → failed | 必要 |
| POST | `/sandbox/outbox/retry-failed` | 自動 reconciler `outbox_failed_retry`（reconcile-scripts.md §3.7.2）の最小実装。failed 状態を pending に戻して再投入 | 必要 |
| GET | `/sandbox/outbox/list?limit=N` | 最近のイベント一覧 + status 別件数。**payload / last_error はクライアントへ返さない** | 必要 |
| POST | `/sandbox/outbox/reset` | PoC 検証用に outbox_events を全件削除。本実装には流用しない | 必要 |

CLI / shell ラッパー:

- `go run ./cmd/outbox-worker --once --limit 50`：1 回だけ pending を処理して終了
- `go run ./cmd/outbox-worker --retry-failed`：failed → pending 再投入のみ
- `./scripts/outbox-process-once.sh`：上記 CLI のラッパー（`OUTBOX_LIMIT` / `OUTBOX_RETRY_FAILED=1` で挙動切替、`.env.local` を自動読込）

セキュリティ方針:

- `payload` 全文はレスポンスにも一覧 API にも出さない（情報漏えい抑止）
- `last_error` はサーバ側 slog のみに残し、クライアントには `consume_rejected` 同様の集約カテゴリで返す
- presigned URL / Secret / token を `payload` に入れないこと（呼び出し側の責務、本実装の ApplicationService で担保）

#### バリデーションルール（R2 presign-put）

| 条件 | レスポンス |
|------|-----------|
| `filename` が空 | 400 `filename_required` |
| `byte_size` ≤ 0 | 400 `byte_size_invalid` |
| `byte_size` > 10MB（10485760） | 400 `file_too_large` |
| `content_type` が許可リスト外（`image/svg+xml`, `image/gif` など） | 400 `unsupported_format` |

**許可 content_type**: `image/jpeg` / `image/png` / `image/webp` / `image/heic` / `image/heif`

#### バリデーションルール（R2 headobject）

| 条件 | レスポンス |
|------|-----------|
| `key` クエリ未指定 | 400 `key_required` |
| `key` が `photobooks/` で始まらない | 400 `key_prefix_invalid` |
| `key` に `../` を含む | 400 `key_traversal_forbidden` |

#### storage_key 命名規則

ADR-0005 の規則に従う:

```
photobooks/{photobook_id}/images/{image_id}/original/{random}.{ext}
```

PoC では `photobook_id` は固定 UUID `00000000-0000-0000-0000-000000000001`、`image_id` はリクエストごとに新規ランダム UUID、`{random}` は 12 バイトの暗号論的乱数を base64url 化（パディングなし）。

レスポンスに `upload_url` / `storage_key` / `expires_in_seconds` を返すが、**サーバ側ログには上記いずれも出さない**設計（`grep` で漏洩確認済み）。

レスポンス全般に **DB / R2 のエラー詳細・認証情報・presigned URL は含めない**。サーバ側ログにも raw 値を残さない。

### 採用したライブラリ

| 種別 | 採用 | バージョン |
|------|------|-----------|
| HTTP ルーター | `github.com/go-chi/chi/v5` | v5.1.0 |
| chi middleware | RequestID / RealIP / Recoverer / Timeout | （chi 標準） |
| DB ドライバ・プール | `github.com/jackc/pgx/v5/pgxpool` | v5.7.1 |
| Migration | `github.com/pressly/goose/v3` (CLI) | v3.22.0（go run 経由で実行） |
| Code generation (SQL → Go) | `sqlc` (CLI) | v1.30.0 |
| **AWS SDK (R2 用)** | `github.com/aws/aws-sdk-go-v2/service/s3` | v1.100.0（S3 互換、R2 で利用） |
| **AWS SDK config / credentials** | `aws-sdk-go-v2/config` v1.32.16 / `aws-sdk-go-v2/credentials` v1.19.15 | — |
| ロガー | 標準 `log/slog`（JSON ハンドラ） | Go 1.21+ 標準 |

ORM は採用しない（ADR-0001 §coding-rules: 明示的 > 暗黙的、any/interface{} 禁止と整合）。

### Go バージョン

`go.mod` は **`go 1.24`**（aws-sdk-go-v2 v1.41 系が要求）。ADR-0001 §採用技術 表の「Go 1.24+」方針と一致。

ローカル環境の `go` コマンドが 1.23.x の場合でも、`GOTOOLCHAIN=auto`（デフォルト）の挙動で 1.24 toolchain が自動取得される。`go env GOTOOLCHAIN` で確認可能。

Dockerfile も `golang:1.24-alpine` を使う（R2 PoC 拡張時に `1.23` から更新）。

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
docker image rm vrcpb-spike-backend:latest vrcpb-spike-backend:r2 2>/dev/null
```

---

## R2 接続 PoC 検証手順（M1 priority 4）

**重要**: 実 R2 接続は **ユーザー側で実施**。Claude Code 側は実 R2 認証情報を扱わず、コード作成・ローカルビルド・バリデーション挙動の確認に留める。

### Cloudflare 側の事前準備

1. **M1 検証用バケットを作成**
   - Cloudflare Dashboard → R2 → Create bucket
   - 名前例: `vrcpb-spike`（本実装用バケットとは分離する）
   - リージョンは特に指定不要（auto）
2. **M1 検証用 API トークンを発行**
   - R2 → Manage R2 API Tokens → Create API token
   - 権限: **Object Read & Write**
   - 対象バケット: 上記 `vrcpb-spike` のみに制限
   - 有効期限: M1 完了予定までの短期（1〜2 週間目安）
3. 発行された `Access Key ID` / `Secret Access Key` / `Endpoint URL` を控える
4. `.env.local`（git ignore 対象）に値を書き込む

### .env.local の例

`.env.local` は **git にコミットしない**。実値はリポジトリ外で管理する。

```
APP_ENV=local
PORT=8090
DATABASE_URL=postgres://vrcpb_spike:spike_local_password@localhost:5432/vrcpb_spike?sslmode=disable

R2_ACCOUNT_ID=<your-cloudflare-account-id>
R2_ACCESS_KEY_ID=<m1-spike-token-access-key-id>
R2_SECRET_ACCESS_KEY=<m1-spike-token-secret>
R2_BUCKET_NAME=vrcpb-spike
R2_ENDPOINT=https://<your-cloudflare-account-id>.r2.cloudflarestorage.com
```

### サーバ起動（R2 設定込み）

```sh
set -a; . ./.env.local; set +a
go build -C harness/spike/backend -o /tmp/spike-api ./cmd/api
/tmp/spike-api
```

ログに `r2 not configured` が出ない（または出る）ことで R2 設定の注入を確認できる。

### curl 検証手順

#### A. HeadBucket（接続確認）

```sh
curl -sS http://localhost:8090/sandbox/r2-headbucket
# 期待: 200 {"status":"ok"}
```

#### B. ListObjects（バケット内一覧、最大 5 件）

```sh
curl -sS http://localhost:8090/sandbox/r2-list
# 期待: 200 {"count":N,"objects":[{"key":"...","size":...}]}
```

#### C. presigned PUT URL 発行

```sh
RESP=$(curl -sS -X POST http://localhost:8090/sandbox/r2-presign-put \
  -H 'Content-Type: application/json' \
  -d '{"filename":"sample.png","content_type":"image/png","byte_size":12345}')
echo "$RESP" | jq .
# 期待: 200 {"upload_url":"https://...","storage_key":"photobooks/.../original/....png","expires_in_seconds":900}

UPLOAD_URL=$(echo "$RESP" | jq -r '.upload_url')
STORAGE_KEY=$(echo "$RESP" | jq -r '.storage_key')
```

#### D. R2 へ直接 PUT（小さなテストファイルで）

```sh
echo "M1 spike test content" > /tmp/test-png.bin
curl -sS -i -X PUT \
  -H "Content-Type: image/png" \
  --data-binary @/tmp/test-png.bin \
  "$UPLOAD_URL"
# 期待: 200 OK（R2 が対象オブジェクトを書き込み完了）
```

実画像でなくてもよい（PoC では「PUT が通るか」だけが目的）。

#### E. HeadObject（complete 相当の存在確認）

```sh
curl -sS "http://localhost:8090/sandbox/r2-headobject?key=$STORAGE_KEY"
# 期待: 200 {"content_length":...,"content_type":"image/png","etag":"\"...\""}
```

#### F. 10MB 超過時の挙動確認

```sh
curl -sS -X POST http://localhost:8090/sandbox/r2-presign-put \
  -H 'Content-Type: application/json' \
  -d '{"filename":"big.jpg","content_type":"image/jpeg","byte_size":11000000}'
# 期待: 400 {"error":"file_too_large"}
```

#### G. SVG / GIF 拒否確認

```sh
curl -sS -X POST http://localhost:8090/sandbox/r2-presign-put \
  -H 'Content-Type: application/json' \
  -d '{"filename":"x.svg","content_type":"image/svg+xml","byte_size":1000}'
# 期待: 400 {"error":"unsupported_format"}
```

#### H. path traversal 拒否確認

```sh
curl -sS 'http://localhost:8090/sandbox/r2-headobject?key=photobooks/../etc/passwd'
# 期待: 400 {"error":"key_traversal_forbidden"}
```

### 検証結果記入欄（R2 接続検証担当者が記入）

| 項目 | 結果 | 備考 |
|------|:---:|------|
| Cloudflare R2 バケット作成 | ✅ | 名前: `vrcpb-spike`（2026-04-25 ユーザーが Dashboard で作成） |
| API トークン発行（Object Read & Write、対象バケット限定） | ✅ | 短期有効（1〜2 週間）。Secret は `.env.local` 経由のみ、Claude Code は値を表示しない |
| `/sandbox/r2-headbucket` 200 OK | ✅ | `{"status":"ok"}` |
| `/sandbox/r2-list` 200 OK | ✅ | 初回 `count=0` |
| `/sandbox/r2-presign-put` 200 OK | ✅ | upload_url 519 bytes、`X-Amz-Algorithm` / `X-Amz-Credential` / `X-Amz-Signature` / `X-Amz-Expires` を含む。expires_in_seconds=900 |
| R2 への `curl -X PUT` 成功（200 / 204） | ✅ | 1024 bytes 合致で `200 OK`（最初は byte_size=1024 宣言に対し 34 bytes 送信し `403 SignatureDoesNotMatch` になったため、実装の Content-Length 署名挙動を確認しサイズ一致で再 PUT 成功） |
| `/sandbox/r2-headobject` で content_length 一致 | ✅ | `content_length=1024 / content_type=image/png / etag` 取得 |
| 10MB 超過 → 400 file_too_large | ✅ | byte_size=11000000 で再現 |
| SVG → 400 unsupported_format | ✅ | image/svg+xml で再現 |
| GIF → 400 unsupported_format | ✅ | image/gif で再現 |
| path traversal → 400 key_traversal_forbidden | ✅ | `photobooks/../etc/passwd` で再現 |
| key prefix invalid → 400 key_prefix_invalid | ✅ | `evil/x.txt` で再現 |
| 存在しない key（prefix valid） → 502 r2_headobject_failed | ✅ | ADR-0005 の方針通り「分類キーのみ返す」実装（404 ではなく 502 系で集約） |
| byte_size=0 → 400 byte_size_invalid | ✅ | |
| filename 空 → 400 filename_required | ✅ | |
| サーバ slog に presigned URL / storage_key / Secret が出ていない | ✅ | `grep -E 'X-Amz-Signature\|X-Amz-Credential\|X-Amz-Algorithm\|presigned\|access[_-]?key\|signing[_-]?key\|Authorization:'` で 0 ヒット。`R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_ENDPOINT` / `storage_key` の各値で `grep -F` を取った count もすべて 0。ログ全体は 3 行のみ（起動ログのみ） |
| Cloudflare 側ログ（R2 監査ログ等）にトークンが平文で残っていない | （未確認） | 必要に応じて Cloudflare Dashboard で別途確認 |
| Cloud Run 東京 ↔ R2 のレイテンシ計測 | （未） | 後続 deploy 検証で実施 |

### 検証完了後の対応

検証完了後にユーザー側で以下を実施する:

1. R2 にアップロードされたテストオブジェクト（1024 バイト 1 件、`storage_key` は API レスポンスで返るランダム UUID 配下）を Cloudflare Dashboard → R2 → `vrcpb-spike` から削除
2. M1 検証用に発行した R2 API Token を Cloudflare Dashboard → R2 → Manage R2 API Tokens で **Revoke**（短期 TTL でも明示的に無効化）
3. M2 本実装では別バケット・別トークンを発行する

### 本 PoC で扱わないもの（明示）

- HEIC / libheif / libde265 の cgo コンテナ構築
- 画像処理ワーカー（image-processor）
- variant 生成（display / thumbnail / OGP）
- EXIF / XMP / IPTC 除去

これらは別 PoC または M3〜M6 本実装で扱う（ADR-0005 §未解決事項参照）。

---

## Turnstile + upload_verification_session PoC 検証手順（M1 priority 5）

### 設計の要点

- raw token は **レスポンスのみ** に返し、DB には **SHA-256（32 バイト bytea）** のみを永続化
- 拒否分類はクライアントへ「**意味が読めない単一カテゴリ**」（`consume_rejected`）に集約。理由は `slog` のサーバ側ログでのみ追跡（`reason=no_rows` 等）
- `upload_verification_sessions` の `ConsumeUploadVerificationIntent` は **単一 SQL UPDATE で原子消費**:
  - `session_token_hash = $1`
  - `photobook_id = $2`
  - `revoked_at IS NULL`
  - `expires_at > now()`
  - `used_intent_count < allowed_intent_count`
  - 0 行返却 = `pgx.ErrNoRows` → 403 で集約
- mock モードは `TURNSTILE_SECRET_KEY` 空のときに自動で有効化。本 PoC のローカル検証専用

### 検証マトリクス（2026-04-25 完了）

#### A. mock モード起動

```sh
DATABASE_URL='postgres://vrcpb_spike:spike_local_password@localhost:5432/vrcpb_spike?sslmode=disable' \
PORT=8090 APP_ENV=local \
ALLOWED_ORIGINS='http://localhost:8787,http://localhost:3000' \
TURNSTILE_SECRET_KEY= \
UPLOAD_VERIFICATION_INTENT_LIMIT=20 UPLOAD_VERIFICATION_TTL=30m \
go -C harness/spike/backend run ./cmd/api
```

起動ログに `running in MOCK mode (PoC only)` の WARN が出ることを確認。

#### B. Turnstile verify バリデーション

```sh
# 空 token → 400 turnstile_token_required
curl -s -X POST http://localhost:8090/sandbox/turnstile/verify \
  -H 'Content-Type: application/json' \
  -d '{"turnstile_token":"","photobook_id":"11111111-1111-1111-1111-111111111111"}'

# MOCK_FAIL → 403 turnstile_rejected
curl -s -X POST http://localhost:8090/sandbox/turnstile/verify \
  -H 'Content-Type: application/json' \
  -d '{"turnstile_token":"x_MOCK_FAIL_y","photobook_id":"11111111-1111-1111-1111-111111111111"}'

# UUID 不正 → 400 photobook_id_invalid
curl -s -X POST http://localhost:8090/sandbox/turnstile/verify \
  -H 'Content-Type: application/json' \
  -d '{"turnstile_token":"DUMMY_OK","photobook_id":"not-a-uuid"}'

# mock 成功 → 200 + verification_session_token
curl -s -X POST http://localhost:8090/sandbox/turnstile/verify \
  -H 'Content-Type: application/json' \
  -d '{"turnstile_token":"DUMMY_OK","photobook_id":"22222222-2222-2222-2222-222222222222"}'
```

#### C. 20 回上限（逐次消費 → 21 回目 403）

`B` で得た token を使って `/sandbox/upload-intent/consume` を 21 回叩く。1〜20 回目は `200` で `remaining` が 19 → 0 と下る。21 回目は `403 consume_rejected`。

#### D. 100 並列 race 検証

```sh
./harness/spike/backend/scripts/turnstile-consume-race.sh http://localhost:8090
# 期待:
#   HTTP 200 (consumed): 20
#   HTTP 403 (rejected): 80
#   その他            : 0
#   PASS: success=20 / forbidden=80
```

スクリプトは新しいセッションを発行し、100 並列で `consume` を叩いた結果を集計する。本 PoC で **100 並列でも `success_count == 20` を満たす**ことを確認した（PostgreSQL Read Committed の単一行 UPDATE による原子消費）。

#### E. 拒否カテゴリの混合

| 操作 | 期待 |
|------|------|
| 別 photobook_id で発行 → 元 photobook_id で consume | 403 `consume_rejected` |
| token を改ざん（base64 文字を別の値に） | 403 `consume_rejected` |
| 既に 20 回消費済みのセッションで再 consume | 403 `consume_rejected` |

すべての拒否は **同じ分類キー** で返るため、攻撃者は「どの条件で落ちたか」を判別できない。

#### F. 実 Cloudflare サンドボックスキー検証

```sh
# always-pass: 任意 token でも 200
TURNSTILE_SECRET_KEY=1x0000000000000000000000000000000AA ./run.sh
curl -s -X POST .../sandbox/turnstile/verify -d '{"turnstile_token":"any","photobook_id":"..."}'
# → 200 + verification_session_token

# always-fail: 任意 token で 403
TURNSTILE_SECRET_KEY=2x0000000000000000000000000000000AA ./run.sh
curl -s -X POST .../sandbox/turnstile/verify -d '{"turnstile_token":"any","photobook_id":"..."}'
# → 403 turnstile_rejected
# サーバ側ログに error_codes:["invalid-input-response"] が出る（クライアントには返さない）
```

#### G. 漏洩確認（grep）

```sh
grep -E 'TURNSTILE_SECRET|0000000AA|turnstile_token=|verification_session_token|raw_token' /tmp/spike-api*.log
# → 期待: 0 ヒット（"secret not configured" / "secret configured" の status だけ）
```

DB 側でも raw token 由来の文字列（`DUMMY_*` / `MOCK_FAIL` / `tampered_*`）が `session_token_hash` に含まれないことを確認:

```sh
docker exec backend-postgres-1 psql -U vrcpb_spike -d vrcpb_spike -c \
  "SELECT count(*) FROM upload_verification_sessions WHERE encode(session_token_hash, 'escape') ILIKE '%DUMMY%';"
# → 0
```

`session_token_hash` は **常に 32 バイト**、`encode(..., 'hex')` で 64 桁の hex 文字列のみ（SHA-256 出力長）。

---

## Outbox + 自動 reconciler 起動基盤 PoC 検証手順（M1 priority 7）

### 設計の要点

- **`outbox_events` テーブル**: `id / event_type / aggregate_type / aggregate_id / payload / status / attempts / next_attempt_at / last_error / created_at / processed_at / locked_at`。CHECK 制約で `status` 4 種 + `attempts >= 0` + `processed_at` の status 整合
- **アトミック claim**: 単一 SQL で `WHERE status='pending' AND next_attempt_at <= now() FOR UPDATE SKIP LOCKED LIMIT $1` を CTE で実行し、即座に `status='processing', attempts++, locked_at=now()` に UPDATE。複数ワーカーが並列で叩いても同じ行を取得しない
- **mock ハンドラルール**:
  - `event_type` が `"ForceFail"` を含む → MarkOutboxFailed（PoC では指数バックオフを導入せず terminal failed に集約）
  - それ以外（例: `ImageIngestionRequested`）→ MarkOutboxProcessed
- **自動 reconciler `outbox_failed_retry`**: 単一 UPDATE で `status='failed' → 'pending', next_attempt_at=now(), locked_at=NULL` を一括反映。reconcile-scripts.md §3.7.2 の最小実装として位置付け

### 検証マトリクス（2026-04-25 完了）

#### A. enqueue / list / バリデーション

```sh
# 5 件正常 + 2 件 ForceFail を enqueue
for i in 1 2 3 4 5; do
  curl -s -X POST http://localhost:8090/sandbox/outbox/enqueue \
    -H 'Content-Type: application/json' \
    -d '{"event_type":"ImageIngestionRequested","aggregate_type":"image","aggregate_id":"00000000-0000-0000-0000-00000000000'"$i"'"}'
done

# list → by_status: pending=7
curl -s http://localhost:8090/sandbox/outbox/list

# バリデーション失敗 3 種: event_type 空 / aggregate_type 空 / aggregate_id 不正
curl -s -X POST http://localhost:8090/sandbox/outbox/enqueue -H 'Content-Type: application/json' -d '{"event_type":"","aggregate_type":"image","aggregate_id":"00000000-0000-0000-0000-000000000001"}'
# → 400 event_type_required
```

#### B. process-once / retry-failed / 再 process-once

```sh
curl -s -X POST 'http://localhost:8090/sandbox/outbox/process-once?limit=10'
# → {"claimed":7,"processed":5,"failed":2,"event_ids":[...]}

curl -s -X POST http://localhost:8090/sandbox/outbox/retry-failed
# → {"requeued":2}

curl -s -X POST 'http://localhost:8090/sandbox/outbox/process-once?limit=10'
# → {"claimed":2,"processed":0,"failed":2}
# 最終: processed=5, failed=2 (attempts=2)、processed 側は attempts=1 で processed_at NOT NULL
```

#### C. 二重処理防止（2 プロセス並列 process-once）

```sh
# 30 件 enqueue 後、2 並列で process-once（各 limit=30）
( curl -s -X POST 'http://localhost:8090/sandbox/outbox/process-once?limit=30' > /tmp/p1.json ) &
( curl -s -X POST 'http://localhost:8090/sandbox/outbox/process-once?limit=30' > /tmp/p2.json ) &
wait

# 期待: 2 プロセスの event_ids に overlap=0、最終 by_status processed=30
```

`SKIP LOCKED` により先に SELECT したプロセスが行をロック、もう一方は同じ行をスキップする。今回の検証ではプロセス #2 が先に 30 件すべてを取得し、#1 は 0 件 claim となった（タイミング依存だが「重複処理が起きない」保証は不変）。

#### D. CLI と shell ラッパー

```sh
# 直接 CLI
go run ./cmd/outbox-worker --once --limit 50
go run ./cmd/outbox-worker --retry-failed

# shell ラッパー（.env.local を自動 dot-source）
./scripts/outbox-process-once.sh
OUTBOX_LIMIT=10 ./scripts/outbox-process-once.sh
OUTBOX_RETRY_FAILED=1 ./scripts/outbox-process-once.sh
```

CLI / shell ラッパーともに sandbox API と同等の処理フロー。Cloud Run Jobs + Cloud Scheduler から起動する想定（cross-cutting/reconcile-scripts.md §3.7.5、U11）。

### 自動 reconciler / 手動 reconcile の整理

cross-cutting/reconcile-scripts.md §3.0 / §3.7 に整合。MVP では以下を 2 系統に分けて運用する。

#### 自動 reconciler（cron 起動、無人実行）

| reconciler | 頻度（推奨） | 本 PoC での扱い |
|-----------|------------|--------------|
| `outbox_failed_retry` | 5 分 / 回 | **本 PoC で最小実装**（`POST /sandbox/outbox/retry-failed` / `outbox-worker --retry-failed`）|
| `draft_expired` | 1 時間 / 回 | 後続（M6）|
| `stale_ogp_enqueue` | 30 分 / 回 | 後続（M6）|
| `delivery_expired_to_permanent` | 1 時間 / 回 | 後続（M6、ManageUrlDelivery 集約整備後）|

#### 手動 `scripts/ops/reconcile/`（運営判断、`--dry-run` デフォルト）

| script | 用途 | 本 PoC での扱い |
|--------|-----|--------------|
| `image_references.sh` | 孤児 Image の検出と修復 | 後続（M6）|
| `outbox_failed.sh` | failed Outbox の手動再投入（個別判断） | 後続（M6）|
| `ogp_stale.sh` | stale OGP の手動再生成 | 後続（M6）|
| `draft_expired.sh` | 期限切れ draft の手動 GC | 後続（M6）|
| `photobook_image_integrity.sh` | Photobook ↔ Image 整合性監査 | 後続（M6）|
| `cdn_cache_force_purge.sh` | CDN キャッシュ強制パージ | 後続（M6）|

本 PoC では **`outbox_failed_retry` の最小挙動のみ実装**し、他は README / M1 計画に「後続作業」として整理した。

### 残る未確認事項

- **多重起動防止**: 2 プロセス並列で SKIP LOCKED が有効に働くことは確認済みだが、Cloud Run Jobs が **1 回の cron 起動で 2 つの Job が走る**ケース（スケジューラ重複）における advisory lock の必要性は未検証。本実装では DB advisory lock or Job スケジューラ側の排他制御を導入する想定（reconcile-scripts.md §3.7.6）
- **指数バックオフ**: PoC では `failed` に集約しているが、本実装では `attempts` に応じて pending 戻し or failed を選ぶ（cross-cutting/outbox.md §6.3）
- **保持期間とクリーンアップ**: `processed=30 日 / failed=無期限 / processing で 1 時間滞留 → pending に戻す`（outbox.md §8）は本 PoC では未実装
- **Cloud Run Jobs + Cloud Scheduler の実環境起動**: M1 残作業（U11）

---

## 検証結果（2026-04-25 CLI 検証、優先順位 3 + R2 拡張後）

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
| **R2 拡張後の `go vet` / `go build` / `go test`** | ✅ | aws-sdk-go-v2 v1.41 系を `go 1.24` で解決、サイズ 21.6MB |
| **R2 未設定時に既存エンドポイント維持** | ✅ | `/healthz` 200、`/readyz` `db_not_configured`、R2 系すべて 503 `r2_not_configured` |
| **R2 未設定時のサーバ起動ログ** | ✅ | `r2 not configured; r2 sandbox endpoints will return 503` の INFO のみ。Secret は出ない |
| **R2 設定時の R2 sandbox バリデーション全パターン** | ✅ | `file_too_large` / `unsupported_format`(SVG/GIF) / `filename_required` / `byte_size_invalid` / `key_required` / `key_prefix_invalid` / `key_traversal_forbidden` すべて期待通り |
| **R2 設定時の `/sandbox/r2-presign-put` 正常応答** | ✅ | `upload_url` / `storage_key`（`photobooks/<UUID>/images/<UUID>/original/<base64url>.png`） / `expires_in_seconds: 900` |
| **R2 拡張後の Docker build** | ✅ | `golang:1.24-alpine` → `gcr.io/distroless/static-debian12:nonroot`、約 17MB |
| **R2 拡張後のログ漏洩確認**（dummy_key / dummy_secret / X-Amz-Signature / presigned URL がログに出ていない） | ✅ | `grep` で漏洩なしを確認 |
| **Turnstile 拡張後の `go vet` / `go build` / `go test`** | ✅ | `google/uuid` v1.6.0 を追加。エラーなし、テスト 0 件のまま |
| **mock モード起動ログ** | ✅ | `running in MOCK mode (PoC only)` WARN 出力。secret は出ない |
| **siteverify 設定モード起動ログ** | ✅ | `siteverify will be called` INFO 出力。secret は出ない |
| **`/sandbox/turnstile/verify` バリデーション** | ✅ | empty / MOCK_FAIL / 不正 UUID / 正常パス すべて期待通り |
| **`upload_verification_sessions` テーブル定義** | ✅ | CHECK 制約（used≤allowed / expires>created / revoked≥created）と `(photobook_id, expires_at) WHERE revoked_at IS NULL` 部分インデックスが適用 |
| **DB に raw token を保存していない** | ✅ | `session_token_hash` は 32 バイト bytea、SHA-256 出力長と一致。`DUMMY_*` / `MOCK_FAIL` 等の生文字列は 0 件 |
| **20 回上限（逐次）** | ✅ | 1〜20 回目 200 / 21 回目 403 `consume_rejected` |
| **100 並列 race（mock + DUMMY token）** | ✅ | `success=20 / forbidden=80 / その他=0`（`scripts/turnstile-consume-race.sh` PASS） |
| **photobook_id 不一致 / 改ざん token** | ✅ | いずれも 403 `consume_rejected`（拒否カテゴリを集約） |
| **実 Cloudflare always-pass secret（`1x0000000000000000000000000000000AA`）** | ✅ | 200 + verification_session_token を返す。実 siteverify が呼ばれていることを `siteverify will be called` で確認 |
| **実 Cloudflare always-fail secret（`2x0000000000000000000000000000000AA`）** | ✅ | 403 `turnstile_rejected`。サーバ側 INFO ログのみ `error_codes:["invalid-input-response"]` を残し、クライアントへは出さない |
| **Turnstile / verification_session token / secret のログ漏洩確認** | ✅ | `grep -E 'TURNSTILE_SECRET\|0000000AA\|verification_session_token\|DUMMY_OK\|MOCK_FAIL\|tampered'` で 0 ヒット |
| **Wrangler CLI 経由の R2 疎通検証**（M1 当初予定） | ⚠️ 中断 | Wrangler 4.82.2 / 4.85.0 とも `--scopes-list` に `r2:*` が無く、OAuth 経由では R2 操作トークンが取得できない（Wrangler 側仕様）。Cloudflare Dashboard 上で R2 は有効・バケット 2 個稼働を確認済み。本検証は本来の目的（Go backend → R2 実接続）でカバー |
| **R2 S3 互換 API 実接続: HeadBucket / ListObjects** | ✅ | 2026-04-25 ユーザー発行の短期 API Token + `vrcpb-spike` バケットで成立。`r2-headbucket=200 / r2-list=200 (count=0)` |
| **R2 S3 互換 API 実接続: presigned PUT 発行 + R2 への PUT + HeadObject** | ✅ | presign 200 / R2 PUT 200（1024 bytes 合致）/ HeadObject 200（`content_length=1024 / content_type=image/png / etag`）。最初は宣言サイズと実 PUT サイズの不一致で R2 が `SignatureDoesNotMatch` を返すことを確認し、aws-sdk-go-v2 の Content-Length 署名挙動を実機で把握 |
| **R2 S3 互換 API: バリデーション 8 ケース** | ✅ | 10MB+ / SVG / GIF / path traversal / prefix invalid / 存在しない key / byte_size=0 / filename 空、すべて期待通り |
| **R2 S3 API 実接続のログ漏洩確認** | ✅ | `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_ENDPOINT` / `storage_key` の各値で `grep -F` count = 0。`X-Amz-*` / `presigned` / `access_key` 等の一般パターンも 0。ログ 3 行のみ（起動ログ） |
| **Outbox migration `00003_create_outbox_events.sql`** | ✅ | goose で 33ms で適用。CHECK 制約（status / attempts / event_type / aggregate_type / processed_at の整合）と `(status, next_attempt_at) WHERE status='pending'` 部分インデックスを含む |
| **Outbox sqlc generate**（`Create / ClaimPending / MarkProcessed / MarkFailed / RetryFailed / List / CountByStatus / ResetForTest`） | ✅ | `internal/db/sqlcgen/outbox.sql.go` を生成、`go vet` / `go build` / `go test` 全て pass |
| **enqueue → list → process-once → list（基本フロー）** | ✅ | 5 件 ImageIngestionRequested + 2 件 ForceFail を enqueue → process-once で `claimed=7 / processed=5 / failed=2`、最終 list の by_status は `processed=5, failed=2`、processed には `processed_at` が NOT NULL でセット |
| **attempts のインクリメント** | ✅ | claim ごとに `attempts +1`。ForceFail を 1 回 process → retry-failed → 再 process した結果 `attempts=2`、ImageIngestionRequested は 1 回処理で `attempts=1` |
| **`ForceFail` → failed → `outbox_failed_retry` で再 pending → 再 process** | ✅ | retry-failed で `requeued=2`、その直後の by_status は `pending=2 / processed=5`。再 process-once で再度 `failed=2`（mock ハンドラが ForceFail を強制失敗する設計どおり） |
| **二重処理防止（2 プロセス並列 process-once）** | ✅ | 30 件 enqueue 後に 2 並列で process-once（各 limit=30）。プロセス間の `event_ids` の **overlap=0**、最終 by_status `processed=30`。PostgreSQL の `FOR UPDATE SKIP LOCKED` が同じ行を複数ワーカーに渡さないことを実機確認 |
| **CLI `go run ./cmd/outbox-worker --once`** | ✅ | sandbox API と同等のハンドラルールで動作（claimed=5 / processed=3 / failed=2 を slog INFO で出力） |
| **CLI `--retry-failed`** | ✅ | failed → pending 再投入で `requeued=N` を slog INFO で出力 |
| **shell ラッパー `scripts/outbox-process-once.sh`** | ✅ | `.env.local` を自動 dot-source、`OUTBOX_LIMIT` / `OUTBOX_RETRY_FAILED` で挙動切替。Secret は echo / printenv しない |
| **既存エンドポイントへの regression** | ✅ | `/healthz` / `/readyz` / `/sandbox/db-ping` / `/sandbox/r2-headbucket` / `/sandbox/r2-list` / `/sandbox/turnstile/verify` (mock) / `/sandbox/origin-check` (許可・拒否) すべて 200 / 403 を期待通り返却 |
| **Outbox 関連のログ漏洩確認** | ✅ | `payload` / Secret / 一般パターン（`X-Amz-*` / `presigned` 等）すべて 0 ヒット。Outbox 正常系は無音、エラー時のみ slog.Warn で `last_error` を残す設計 |

実 R2 接続（実 Cloudflare アカウント経由）は **ユーザー側で実施**する。手順は本書「R2 接続 PoC 検証手順」セクション参照。

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

## Frontend / Backend 結合検証手順（M1）

`harness/spike/frontend/` と本 PoC を組み合わせた結合検証。CORS / Cookie / Origin チェックの基盤動作を確認する。

### Backend 起動

```sh
PORT=8090 APP_ENV=local \
ALLOWED_ORIGINS='http://localhost:8787,http://localhost:3000' \
DATABASE_URL='postgres://vrcpb_spike:spike_local_password@localhost:5432/vrcpb_spike?sslmode=disable' \
/tmp/spike-api
```

`ALLOWED_ORIGINS` が未設定の場合、CORS ヘッダは付かず `/sandbox/origin-check` は常に `403 origin_not_allowed`（Origin が空なら `403 origin_required`）。

### Frontend 起動

```sh
# frontend 側で一度だけ
cd harness/spike/frontend
echo "NEXT_PUBLIC_API_BASE_URL=http://localhost:8090" > .env.local

# OpenNext 経由で wrangler dev（推奨）
npm run cf:build
npm run cf:preview
# → http://localhost:8787
```

### 結合検証ページ

`http://localhost:8787/integration/backend-check`

ボタンを押すごとに Backend へ fetch する:

- `GET /healthz`（credentials なし）
- `GET /sandbox/session-check`（credentials: include / omit 比較）
- `POST /sandbox/origin-check`（credentials: include）

### 期待される CORS 挙動（curl で事前確認可能）

| 検証 | 期待結果 |
|------|---------|
| OPTIONS preflight from `http://localhost:8787` | 204 + `Access-Control-Allow-Origin: http://localhost:8787` + `Access-Control-Allow-Credentials: true` + `Vary: Origin` |
| OPTIONS preflight from `http://evil.example` | 204 だが `Access-Control-Allow-Origin` 無し（ブラウザが弾く） |
| GET from 許可 Origin | `Access-Control-Allow-Origin` を反射、`Allow-Credentials: true` |
| GET from 許可外 Origin | CORS ヘッダ無し（ブラウザは応答を破棄） |
| POST `/sandbox/origin-check` from 許可 Origin | 200 `{"origin_allowed":true}` |
| POST `/sandbox/origin-check` from 拒否 Origin | 403 `{"error":"origin_not_allowed"}` |
| POST `/sandbox/origin-check` Origin ヘッダ無し | 403 `{"error":"origin_required"}` |

### Cookie 引き渡しの限界（重要）

ローカル `localhost:8787` (Frontend) と `localhost:8090` (Backend) は**ホスト名が同じだがポートが違う = ブラウザ仕様で別オリジン**。Set-Cookie 時に Domain 未指定の Cookie は **発行ホスト + 同一ポート**にしか付かないため:

- Frontend で発行した `vrcpb_draft_*` / `vrcpb_manage_*` Cookie は、Frontend ホストにのみ付く
- Backend `/sandbox/session-check` を `credentials: "include"` で叩いても、Backend ホストに該当 Cookie は無いため `false` / `false`

これは設計失敗ではなく **ローカル分離オリジンの仕様**。実環境では:

- 案A: Frontend / Backend を共通親ドメイン下に配置（例: `app.example.com` / `api.example.com`、Cookie Domain `.example.com`）
- 案B: Backend を Frontend と同一オリジン経由でプロキシ（Cloudflare Worker ルート、または同一ホスト内のリバプロ）
- 案C: Frontend が Backend を `/api/*` パスで吸収（同一オリジン化）

選択は U2 として ADR-0003 で継続検討。実環境デプロイ後の結合検証で確定する。

curl レベルでは `-H 'Cookie: vrcpb_draft_sample-photobook-id=...'` を直接渡すことで Backend の `/sandbox/session-check` 動作確認は可能（本書「検証結果」セクション参照）。

### Safari で追加確認すべき項目

- Web Inspector → Storage → Cookies で `localhost:8787` ホストに `vrcpb_draft_*` / `vrcpb_manage_*` が付いている
- `/integration/backend-check` で各ボタンを押し、Network タブで以下を確認:
  - preflight が出ている（POST 系）
  - レスポンスヘッダ `Access-Control-Allow-Origin: http://localhost:8787` が反射されている
  - `credentials: "include"` で Cookie ヘッダが Backend リクエストに付いていない（別オリジンだから）
- ITP がクロスサイト Cookie として扱う挙動の確認（実環境で再確認）

### ローカル HTTP では確認できない項目（実環境で再確認）

- `Secure` 属性付き Cookie の引き渡し（HTTP 経由では送られない）
- 共通親ドメイン下での Cookie Domain 動作（実 DNS 必要）
- Cloudflare Workers + Cloud Run の異なるホスト構成下での CORS / preflight
- Safari ITP の長期影響（24h / 7 日）

---

## 次工程（M1 残作業）

`docs/plan/m1-spike-plan.md` §13.0 に従い、本 Backend PoC の次は以下:

1. ~~**R2 Wrangler 実操作疎通検証**~~ ⚠️ **中断**（2026-04-25）
   - Wrangler 4.82.2 / 4.85.0 の `wrangler login --scopes-list` に R2 系スコープが無く、OAuth 経由では R2 操作トークンを取得できないことが判明
   - Cloudflare Dashboard 上で R2 が有効・既存バケット稼働中を目視確認しているため、Wrangler 不要と判断し本来の目的である Go backend 経由の実接続検証へ前倒し
2. ~~**R2 S3 API + presigned URL 実接続検証**~~ ✅ **完了**（2026-04-25）
   - ユーザーが Cloudflare Dashboard で `vrcpb-spike` バケット作成 + 短期 R2 API Token 発行 + `.env.local` に手入力
   - Claude Code は Secret 値を一切表示せず、`/sandbox/r2-headbucket` / `r2-list` / `r2-presign-put` / R2 PUT / `r2-headobject` / 失敗系 8 ケース / ログ漏洩 grep を実施
   - 後片付けはユーザー側で「テストオブジェクトの削除」「R2 API Token の Revoke」を実施
3. ~~**Outbox / 自動 reconciler 起動基盤 PoC**~~ ✅ **完了**（2026-04-25、M1 優先順位 7）
   - `outbox_events` migration + sqlc + sandbox API + CLI worker + shell ラッパー
   - enqueue → process → processed / failed → retry-failed → 再 process の流れと、2 プロセス並列下での `FOR UPDATE SKIP LOCKED` による二重処理防止を実機確認
4. **Email Provider 選定**（ADR-0004 Proposed → Accepted、M1 優先順位 8）← 次工程候補
5. **Cloud Run + Cloudflare Workers 実環境デプロイ**
   - Cookie Domain（U2、ADR-0003）、CORS、Origin、SameSite=Strict をエンドツーエンドで確認
   - Cloud Run 東京 ↔ R2 のレイテンシ計測
   - **Cloud Run Jobs + Cloud Scheduler から `outbox-worker --once` を起動できることの実機確認**（U11）
6. **Turnstile 本番 widget 発行**（Workers の hostname 確定後）
   - Cloudflare Dashboard → Turnstile → Add widget で発行。本番 secret は `.env.local` または Secret Manager のみで扱う（チャット・ログに貼らない）

---

## ライセンス / 取扱い

本 PoC は内部検証のみを目的とする。外部公開・本実装流用は禁止。
