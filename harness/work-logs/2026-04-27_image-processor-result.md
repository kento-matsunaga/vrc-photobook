# PR23 image-processor 実装結果（2026-04-27）

## 概要

processing 状態の Image を取り出し、原本を decode → resize → JPEG 再エンコードし、
display / thumbnail variant を R2 に PUT、DB を available に確定して原本を削除する
**image-processor** を実装した。CLI バイナリ (`cmd/image-processor`) として API image
に同梱し、Cloud Run Jobs（後続 PR）で起動可能にする。

関連:
- [`docs/plan/m2-image-processor-plan.md`](../../docs/plan/m2-image-processor-plan.md)
- 直前: PR22.5 Frontend HEIC 除外（同 PR で実施）

## 実装内容

### 1. ドメイン / インフラ

| 変更 | 概要 |
|---|---|
| `backend/internal/image/domain/vo/storage_key/` | display / thumbnail の拡張子を `.webp` → **`.jpg`** に変更（plan §10）。OGP は `.png` 据え置き |
| `backend/internal/image/infrastructure/repository/rdb/queries/image.sql` | `ListProcessingImagesForUpdate` query を追加（`FOR UPDATE SKIP LOCKED LIMIT $1`、ORDER BY uploaded_at ASC） |
| `backend/internal/image/infrastructure/repository/rdb/image_repository.go` | `ListProcessingForUpdate(ctx, limit)` を追加（claim TX 内で呼ぶ） |
| `backend/internal/imageupload/infrastructure/r2/client.go` | `Client` interface に `GetObject` / `PutObject` / `ListObjects` を追加 |
| `backend/internal/imageupload/infrastructure/r2/aws_client.go` | 上記 3 メソッドを AWS SDK v2 で実装（NoSuchKey → `ErrObjectNotFound`） |
| `backend/internal/imageupload/tests/fake_r2_client.go` | `GetObjectFn` / `PutObjectFn` / `ListObjectsFn` を追加 |

### 2. image-processor 本体

| 配置 | 概要 |
|---|---|
| `backend/internal/imageprocessor/infrastructure/imaging/` | `image/jpeg` + `_ image/png` + `_ golang.org/x/image/webp` で decode、`disintegration/imaging` で Lanczos リサイズ、`image/jpeg` で APP1/EXIF/XMP/IPTC/ICC 含まない再エンコード。 `Decode` / `EncodeJPEG` を公開 |
| `backend/internal/imageprocessor/internal/usecase/process_image.go` | 単一 Image に対する一連のフロー（list → get → decode → encode → put display + thumbnail → DB MarkAvailable + AttachVariant×2 → delete original）。失敗パターンごとに `failure_reason` を区別して MarkFailed。`ErrorWithReason` で reason を呼び出し側に伝える |
| `backend/internal/imageprocessor/internal/usecase/process_pending.go` | claim TX で `FOR UPDATE SKIP LOCKED LIMIT 1` → 1 件取り出し → 重い処理 → finalize TX。ループで `MaxImages` 件まで処理 |
| `backend/internal/imageprocessor/wireup/` | `cmd` から呼ぶための `Runner` interface（`internal/usecase` を直接 import できないため再 export） |
| `backend/cmd/image-processor/main.go` | `--once` / `--all-pending` / `--dry-run` / `--max-images` / `--timeout` 付き CLI |
| `backend/Dockerfile` | golang:1.25-alpine に更新、`api` と `image-processor` の 2 バイナリをビルドして同 image に同梱 |

### 3. テスト（pure Go fake R2 + 実 DB）

| ファイル | 観点 |
|---|---|
| `imageprocessor/infrastructure/imaging/imaging_test.go` | jpeg/png/decode 失敗、resize 縮小/拡大なし、JPEG 出力に APP1/Exif/XMP/IPTC マーカーが含まれない（plan §6.2 受け入れ条件） |
| `imageprocessor/internal/usecase/process_image_test.go` | HEIC 短絡、ListObjects 0 件、GetObject NoSuchKey、decode 失敗、正常 JPEG（800×600）→ available + 2 variants、R2 PUT 失敗で processing 据え置き |

## 実環境チェック

### Backend ローカル

```
go -C backend build ./...                  # 通る
go -C backend vet ./...                    # 通る
DATABASE_URL=... go test ./internal/imageprocessor/...  # 全件 pass（imaging 4 + UseCase 6）
```

実 DB（postgres:16-alpine）を上げて `process_image_test.go` 6 ケース全て pass。
正常系 JPEG ケースで `images.status` が `available`、`image_variants` に display + thumbnail
の 2 行が入ることを確認。HEIC ケースで `failure_reason='unsupported_format'` を確認。

### Frontend ローカル

```
npm --prefix frontend run typecheck   # 通る
npm --prefix frontend test            # 44 件全て pass
npm --prefix frontend run build       # build 成功
```

## 既知のスコープ外（PR23 では実施しない）

PR23 計画書 §2 の「対象外」と整合。本 PR では作らない／触らない。

- **libheif / cgo / debian-slim**: HEIC は `unsupported_format` で失敗扱い。PR22.5 で
  Frontend がアップロード前に弾く（多層防御）
- **Cloud Run Jobs / Cloud Scheduler 定義（job spec / IAM / 起動時 secret 注入）**:
  PR23 計画書 §2 / §3 で **対象外**と明記されている。本 PR では `cmd/image-processor`
  CLI バイナリを image に同梱するところまでに留める。Jobs / Scheduler は別 PR で接続
- **Outbox INSERT**: `ImageBecameAvailable` イベントは本 PR で扱わず（Reconcile / OGP は別 PR）
- **HTTP processor endpoint**: 公開エンドポイントは作らない（CLI のみ）
- **R2 orphan Reconcile**: PR25 で対応。display/thumbnail PUT 成功 → DB 失敗のときは
  R2 に orphan が残るが、次回 retry で新しい variant が作られる
- **Backend Go バージョン bump**: 本 PR で go.mod / Dockerfile の Go バージョンは
  **1.24 維持**。`go get` 副作用で一時的に `go 1.25.0` / `golang.org/x/image v0.39.0` /
  `golang.org/x/text v0.36.0` まで上がっていたが、PR23 計画書 §4.2 の「distroless static
  + CGO_ENABLED=0 維持 / Dockerfile 工事不要」方針に沿うため、`golang.org/x/image v0.30.0` /
  `golang.org/x/text v0.29.0` / `golang.org/x/sync v0.17.0` に固定して baseline と整合させた

## 設計判断（plan §17 確定）

| Q | 採用 | 理由 |
|---|---|---|
| Q3 | image-processor を API image に同梱 | 別 image を作るより docker build / push が単純 |
| Q4 | 標準 + golang.org/x/image/webp + disintegration/imaging | pure Go、cgo 不要、CGO_ENABLED=0 維持 |
| Q5 | display / thumbnail 共に **JPEG** | 互換性最優先、APP1 を出さないので EXIF 自動除去 |
| Q6 | display=1600 / thumbnail=480 | Viewer 互換 |
| Q8 | R2 PUT → DB TX → R2 DELETE(original) | DB は信頼源、PUT 失敗は retry、R2 orphan は Reconcile cleanup |
| Q9 | single-worker CLI | 並列 worker は claim 用 status / claimed_at 列を追加してから（次 PR 以降） |
| Q10 | CLI（cmd/image-processor） | Cloud Run Jobs 親和性。ただし PR23 では **CLI まで**で、Jobs 定義は別 PR |

## 実環境 cleanup 結果（2026-04-27 17:34 JST 実行）

PR23 計画書 §2 / §3 に従い、Cloud Run Jobs / Scheduler は作らず、Cloud SQL Auth Proxy
+ ローカル CLI で既存の processing 画像を整理した。

### 手順

1. `cloud-sql-proxy v2.18.0` をローカルに用意（gcloud component 不可だったので
   storage.googleapis.com から直 download / 配置先 `~/bin/cloud-sql-proxy`）
2. `cloud-sql-proxy --port 15432 project-...:asia-northeast1:vrcpb-api-verify` を background 起動
3. `gcloud secrets versions access latest --secret=...` で `DATABASE_URL` /
   `R2_*` を **値を chat / log に出さないまま** env 変数に inject。`set -x` 不使用
4. `DATABASE_URL` の `?host=/cloudsql/...` 部分を sed で剥がし、`@/` を
   `@127.0.0.1:15432/` に置換（unix socket → TCP）
5. `--dry-run --max-images=50` で対象 1 件と確認（dry-run は処理対象 1 件目のみ表示する仕様、
   詳細は §dry-run 仕様参照）
6. `--all-pending --max-images=50 --timeout=5m` で本実行

### 結果

| 集計 | 値 |
|---|---|
| picked | 5 |
| success | 0 |
| failed | 5（全て `failure_reason='object_not_found'`、PR22 で R2 cleanup 済のため期待通り）|

5 件の対象（4 photobook 配下）は全て `processing` → `failed` に整理。PR22 での
R2 cleanup と矛盾しない結果（DB だけが processing で残っていた状態 → 解消）。

> 着手前の work-log 推測では「4 件」と記録していたが、実 DB は 5 件だった。
> 古い処理待ちが 1 件多かったのは driver 単位（PR21 / PR22 検証中の中断 image）の
> 想定範囲内なので、今回の cleanup で吸収。

### dry-run の仕様（PR23 で見つかった挙動）

`--dry-run` は **状態を変えない**ため、`ListProcessingImagesForUpdate` の
`ORDER BY uploaded_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED` は毎回先頭の同じ row を返す
（claim TX commit 時に lock 解放、status は processing のまま）。
PR23 では memo (`seen` map) で同じ id の 2 度目を検知して break することにより、
無限ループを防ぐ。dry-run 時の picked 数は「次に live 処理した場合に **最初に処理される
1 件**」を表す。全件を列挙したい場合は live 実行（または `psql` で直接 count）。
multi-worker 化と claim 用 column 追加を別 PR で扱う際に再設計予定。

### Secret 漏洩なし（確認）

stdout / stderr / log を grep して以下が含まれないことを確認:
- `DATABASE_URL` 値 / `R2_ACCESS_KEY_ID` 値 / `R2_SECRET_ACCESS_KEY` 値 /
  `R2_BUCKET_NAME` 値 / `R2_ENDPOINT` 値 / `R2_ACCOUNT_ID` 値
- 完全な `storage_key` 値（log には `image_id` / `photobook_id` のみ）
- raw token / Cookie 値

## 次にやること

1. backend image をビルドして Artifact Registry に push（api / image-processor 同梱）
2. Cloud Run の API revision 更新（image-processor は同 image だが API 起動には影響なし）
3. PR23 commit + push（**Cloud Run Jobs は本 PR には含めない、PR24 以降で対応**）

> **Cloud Run Jobs / Scheduler は次 PR（PR24 以降）の作業**。
> 本 PR では CLI を image に同梱するまで。Cloud Run Jobs spec / IAM service account /
> Scheduler trigger は計画書段階から作っていない。
>
> 既存の processing 画像 cleanup は **本 PR 完了前にローカル CLI で実施済**（上記 §結果）。

## セキュリティ確認

- log には `image_id` / `photobook_id` / `failure_reason` / `source_format` /
  `variant_count` / `processing_duration_ms` のみ出力
- `storage_key` / `presigned URL` / `R2 credentials` / `Cookie` / `DATABASE_URL` /
  ファイル内容は一切 log に出さない（plan §10B.2）
- HEIC は Frontend で除外済（PR22.5）+ Backend で短絡 MarkFailed（多層防御）
