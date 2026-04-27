# M2 image-processor 実装計画（PR23 候補）

> 作成日: 2026-04-27
> 位置付け: PR22（Frontend upload UI）完了後、processing 状態の Image を処理して
> display / thumbnail variant を生成し available に進める **image-processor** を実装する
> フェーズの入口。実装コードはまだ書かない。
>
> 上流参照（必読）:
> - [`docs/adr/0005-image-upload-flow.md`](../adr/0005-image-upload-flow.md) §image-processor 時の本検証
> - [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §3.10
> - [`docs/design/aggregates/image/ドメイン設計.md`](../design/aggregates/image/ドメイン設計.md)
> - [`docs/design/aggregates/image/データモデル設計.md`](../design/aggregates/image/データモデル設計.md)
> - [`docs/plan/m2-image-upload-plan.md`](./m2-image-upload-plan.md)
> - [`docs/plan/m2-r2-presigned-url-plan.md`](./m2-r2-presigned-url-plan.md)
> - [`docs/plan/m2-frontend-upload-ui-plan.md`](./m2-frontend-upload-ui-plan.md)
> - [`harness/work-logs/2026-04-27_frontend-upload-ui-result.md`](../../harness/work-logs/2026-04-27_frontend-upload-ui-result.md)
> - [`backend/Dockerfile`](../../backend/Dockerfile) (現状: distroless static + CGO_ENABLED=0)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
> - [`.agents/rules/testing.md`](../../.agents/rules/testing.md)

---

## 0. 本計画書の使い方

- 設計の正典は ADR-0005 + image データモデル設計。本書はそれを **PR23 でどこまで切り出すか** を整理する。
- HEIC / libheif / cgo / Dockerfile が技術的な分岐点。まず §4 / §5 を読んで判断方針を確定させる。
- §17 のユーザー判断事項に答えてもらってから PR23 実装に着手する。

---

## 1. 目的

- **`processing` 状態の Image を処理して `available` に進める**。
- R2 から original object を取得 → decode → resize → encode → R2 に display / thumbnail variant
  を PUT → DB に variants を INSERT + Image を MarkAvailable。
- EXIF / XMP / IPTC を除去（**re-encode による暗黙除去**で十分）。
- 失敗時は image データモデル §3.0 の **failure_reason 12 種**に従って `failed` に遷移。
- PR22 で R2 cleanup 済 + DB 上に残る processing image（PR22 work-log §「PR23 着手時」）は
  `MarkFailed(object_not_found)` で整理。
- 将来の Outbox / Cloud Run Jobs / 公開 Viewer への接続を見据えて **PR23 では最小範囲に絞る**。

---

## 2. PR23 の対象範囲

### 対象（PR23 で実装する）

- 新パッケージ `backend/internal/imageprocessor/`
  - `domain/`: 処理パイプラインの VO（`variant_size` 等）
  - `infrastructure/imaging/`: image decode / resize / encode（pure Go）
  - `infrastructure/r2/`: GetObject / PutObject / DeleteObject（既存 imageupload/r2 を共有 or 拡張）
  - `internal/usecase/process_image.go`: 単一 Image を processing → available / failed に遷移
  - `internal/usecase/process_pending.go`: ListProcessingImages を順に処理（CLI 駆動）
- `cmd/image-processor/main.go`: CLI ツール
  - `--once <image_id>` で 1 件処理
  - `--all-pending` で processing 状態の全 Image を処理
  - `--dry-run` で DB / R2 に副作用を出さず動作確認
- DB:
  - `MarkImageAvailable` SQL は PR18 既存を流用、`AttachVariant` も既存
  - 必要に応じて `ListProcessingImages` SQL を追加
- tests:
  - imaging unit (JPEG / PNG / WebP の decode / resize / encode / EXIF 除去確認)
  - process_image UseCase（fake R2 client）
  - cmd/image-processor の dry-run test
- Outbox: **event 名 / payload 案だけ計画書に記述**（実装は PR25）

### 対象外（PR23 では実装しない）

- HEIC 本対応（libheif + cgo、§4 で「PR23 では unsupported_format」を推奨）
- 公開 Viewer での画像表示（display variant の URL 配信）
- OGP 生成
- moderation UI / Report
- Outbox events / table（PR25）
- Cloud Run Jobs / Scheduler 本番運用（CLI から手動実行 or 別 PR）
- SendGrid
- Public repo 化
- Cloud SQL 削除 / spike 削除

---

## 3. 処理方式の選択肢

| 案 | 仕組み | 利点 | 欠点 |
|---|---|---|---|
| A: complete-upload 内で同期処理 | Frontend が complete を呼んだら同 request 内で処理 | UX シンプル | Cloud Run timeout（60s 既定）/ HEIC 変換負荷 / Safari upload 後の待ち時間 / TX 中の R2 操作で巻き戻し困難 |
| B: Cloud Run Jobs + Outbox-worker | 非同期で堅牢 | ADR-0005 設計通り | PR23 単体で実装範囲が広い、Outbox 本実装も連動 |
| **C（推奨）: PR23 は CLI / admin command** | image-processor を独立 UseCase + cmd/image-processor として実装、ローカル実行 + 将来 Cloud Run Jobs から呼べる形 | API と疎結合 / PR23 範囲を絞れる / Outbox 不要 | 完全自動化までは PR25 を待つ |

→ **案 C 推奨**。PR23 では次のフローで動かす:

1. ユーザー（or 運用者）が手元で `image-processor --all-pending` を実行
2. または Cloud Run Jobs / Cloud Scheduler を後続 PR で接続（PR25）
3. PR22 で残存している processing 4 件は PR23 完了直後に `--all-pending` で `MarkFailed(object_not_found)` 整理

---

## 4. HEIC 対応方針（最重要）

### 4.1 選択肢

| 案 | HEIC を | runtime | 利点 | 欠点 |
|---|---|---|---|---|
| **案 H1（推奨）: PR23 では unsupported_format** | failed として処理（`failure_reason='unsupported_format'`） | 現状の distroless static + CGO_ENABLED=0 を維持 | Dockerfile / cgo 工事不要 / PR23 範囲が小さい / iPhone Safari の HEIC は Frontend で blocking 表示してもらう or 別 PR で対応 | iPhone Safari ユーザーが HEIC アップロード時に failed になる UX |
| 案 H2: libheif + cgo + Cloud Run image 拡張 | JPG/WebP に変換 | distroless static → debian/slim or distroless cc に変更、`CGO_ENABLED=1` | ADR-0005 設計通り、本格対応 | image build 工事大、cgo の build 時間 / image size 増（推定 +50-100MB）、libheif の脆弱性追従 |
| 案 H3: libvips（C ライブラリ）+ govips bindings | JPG/WebP に変換 | 同上 | 高速 / 高機能 | 同上 + libvips 追跡コスト |

### 4.2 推奨: **案 H1（PR23 では unsupported）**

理由:
- **PR23 を Backend 内 pure Go で完結させる**ことで、Cloud Run image build / cgo 切替を別 PR に分離
- Frontend (PR22) は現状 HEIC を accept しているが、Backend で `failed(unsupported_format)` になる
  → ユーザー UX は「アップロードはできたが処理に失敗」に表示される
- iPhone Safari ユーザーが完全に詰まらないよう、**Frontend に HEIC accept を一時的に外す（PR22 修正）or
  注意文を出す**を併せて検討（§17 Q3）

### 4.3 案 H1 採用時の Backend 挙動

```
1. R2 GetObject で original を取得
2. magic number で実形式判定（先頭 12 byte で JPEG / PNG / WebP / HEIC を識別）
3. HEIC なら即 MarkFailed(unsupported_format) + R2 original を削除（or 残置で PR25 cleanup）
4. JPEG / PNG / WebP のみ処理続行
```

### 4.4 PR25 以降での HEIC 本対応（参考）

- 別 PR で `cmd/image-processor` 専用 image を debian-slim ベースで作る
- libheif を apt で追加、cgo bindings (例: `github.com/strukturag/libheif`) を import
- API image (`vrcpb-api`) は distroless static のまま
- Cloud Run Jobs / Cloud Scheduler で processor 専用 image を起動

---

## 5. Dockerfile / runtime 影響

### 5.1 現状

```
backend/Dockerfile:
  build:    golang:1.24-alpine（CGO_ENABLED=0）
  runtime:  gcr.io/distroless/static-debian12:nonroot
  size:     12-25 MB
  binary:   /usr/local/bin/api（cmd/api のみ）
```

### 5.2 PR23 で必要な変更（案 H1 採用時）

**API image に image-processor binary を同梱**:

```dockerfile
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/image-processor ./cmd/image-processor

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/image-processor /usr/local/bin/image-processor
```

→ image size +5MB 程度（pure Go）。distroless static / CGO_ENABLED=0 維持。

### 5.3 Cloud Run service と Cloud Run Jobs

- `vrcpb-api`（既存 service）: `cmd/api` を起動
- 将来の `vrcpb-image-processor`（Cloud Run Jobs、PR25）: 同じ image を起動引数 `image-processor --all-pending` で実行
- PR23 では Cloud Run Jobs を作らず、ローカル / SSH 経由 / Cloud SQL Auth Proxy 経由で手動実行

---

## 6. 画像ライブラリ方針

### 6.1 選択肢（pure Go 限定）

| ライブラリ | 用途 | 状況 |
|---|---|---|
| `image/jpeg` (Go 標準) | JPEG decode / encode | OK |
| `image/png` (Go 標準) | PNG decode / encode | OK |
| `golang.org/x/image/webp` | WebP decode | OK（read-only） |
| `github.com/chai2010/webp` | WebP encode（cgo 不要 fork あり） | 要検証、cgo フォークが多い |
| `github.com/HugoSmits86/nativewebp` | WebP encode pure Go | 比較的新しい、評価必要 |
| `github.com/disintegration/imaging` | resize / crop / filter | 安定、pure Go |

### 6.2 推奨

- **decode**: 標準 `image/jpeg` + `image/png` + `golang.org/x/image/webp`
- **resize**: `github.com/disintegration/imaging`（Lanczos）
- **encode**:
  - display: **JPEG**（quality=85、最も互換性高い）
  - thumbnail: **JPEG**（quality=80）
  - WebP encode は pure Go の選択肢が薄いため PR23 では JPEG 統一を推奨
- **EXIF/XMP/IPTC 除去**: re-encode（decode → resize → JPEG encode）で**自動的に除去**される
  - JPEG encoder が APP1 / APP13 / EXIF segment を出さない設計
  - 確認: encode 後の binary に APP1 / EXIF marker が残っていないか test で grep

### 6.3 image データモデル §4 との整合

`image_variants.mime_type` CHECK は `image/jpeg` / `image/png` / `image/webp`。JPEG 統一でも
CHECK は通る。将来 WebP encode を追加するときに mime_type を変えるだけで対応可。

---

## 7. variant 仕様

### 7.1 サイズ

| variant | 長辺 | 用途 |
|---|---|---|
| `display` | **1600px** | 公開 Viewer / 編集画面プレビュー |
| `thumbnail` | **480px** | 編集画面 grid / OGP（後続 PR） |

長辺基準で aspect ratio 維持。元画像が長辺 1600px 未満なら拡大せず元サイズで encode（PR23 推奨）。

### 7.2 format

PR23: 全 variant **JPEG**（§6.2 通り）。

### 7.3 PR18 「original 保持しない」方針との整合

ADR-0005 / image データモデル v4 U9: **MVP では original variant を保持しない**。

PR23 の処理:
1. R2 GetObject で original 取得
2. display / thumbnail を encode
3. R2 PutObject で display / thumbnail PUT
4. DB に image_variants 行 (display + thumbnail) INSERT + Image MarkAvailable
5. **R2 で original prefix の object を DeleteObject**（清掃）

`image_variants` table に `original` 行は作らない。

### 7.4 metadata_stripped_at の更新

re-encode 完了時刻を `metadata_stripped_at` に記録。`Image.MarkAvailable` UseCase 引数で渡す
（PR18 既存）。

---

## 8. R2 object 方針

### 8.1 入出力 key

入力（PR21 で PUT 済）:
```
photobooks/{photobook_id}/images/{image_id}/original/{random}.{ext}
```

出力:
```
photobooks/{photobook_id}/images/{image_id}/display/{random}.jpg
photobooks/{photobook_id}/images/{image_id}/thumbnail/{random}.jpg
```

`storage_key.GenerateForVariant(pid, iid, kind=display)` は PR18 で実装済（拡張子 webp 固定）。
PR23 で **拡張子を引数で渡せるように修正**するか、JPEG 用の新ヘルパを追加する（§17 Q5）。

### 8.2 入力 object 削除

成功時:
- DB transaction 完了後に R2 で `photobooks/{pid}/images/{iid}/original/` prefix を ListObjectsV2 + DeleteObject
- 失敗（DeleteObject エラー）でも DB は available のまま放置（orphan original は Reconcile で cleanup、PR25）

失敗時:
- decode_failed / encode_failed → original を残置（手動調査用）+ MarkFailed
- unsupported_format → original を削除（HEIC で容量取らない）+ MarkFailed
- object_not_found → R2 PUT が無いケースなので何もしない + MarkFailed

### 8.3 storage_key のログ禁止（継続）

`security-guard.md` に従い storage_key を logs に出さない。CLI 出力は `image_id` までに留める。

---

## 9. Image 状態遷移

PR23 で扱う遷移:

```
processing → available     (display + thumbnail variant 生成成功)
processing → failed        (各種エラー)
available  → noop          (idempotent: 既に available なら処理しない)
failed     → noop           (idempotent: 既に failed なら処理しない)
```

failure_reason マッピング（PR18 の 12 種、image データモデル §3.0）:

| エラー条件 | failure_reason |
|---|---|
| R2 GetObject で 404 | `object_not_found` |
| 実 size > 10MB | `file_too_large` |
| 申告 size と HeadObject size 不一致 | `size_mismatch` |
| magic number が JPEG/PNG/WebP/HEIC 以外 | `unsupported_format` |
| HEIC（PR23 案 H1） | `unsupported_format` |
| SVG | `svg_not_allowed` |
| アニメーション WebP / APNG | `animated_image_not_allowed` |
| 寸法 8192px 超 / 40MP 超 | `dimensions_too_large` |
| decode 失敗 / decompression bomb | `decode_failed` |
| EXIF 除去失敗 | `exif_strip_failed` |
| HEIC 変換失敗（PR25 以降） | `heic_conversion_failed` |
| display / thumbnail 生成失敗 | `variant_generation_failed` |
| 上記分類できない内部エラー | `unknown` |

PR23 で実装するのは、JPEG/PNG/WebP の処理 + HEIC を unsupported に倒すパターン。

---

## 10. DB / sqlc / repository 追加

### 10.1 既存で十分なもの

PR18 で実装済:
- `Image.MarkProcessing` / `MarkAvailable` / `MarkFailed`
- `ImageRepository.MarkProcessing` / `MarkAvailable` / `MarkFailed` / `AttachVariant`

### 10.2 追加が必要なもの

```sql
-- name: ListProcessingImagesForUpdate :many
SELECT id, owner_photobook_id, usage_kind, source_format, ...
FROM images
WHERE status = 'processing'
ORDER BY uploaded_at ASC
LIMIT $1;
```

PR23 で `ListProcessingImagesForUpdate` を sqlc query として追加。

### 10.3 Transaction 設計

**1 image あたりの TX**:

```
Tx Begin
  Image.MarkAvailable (SQL UPDATE)
  AttachVariant(display) (SQL INSERT)
  AttachVariant(thumbnail) (SQL INSERT)
Tx Commit
```

**TX 外**:
- R2 GetObject（読み取り、無害）
- R2 PutObject(display) / PutObject(thumbnail)（書き込み、TX 前 or TX 内のどちら？）
- R2 DeleteObject(original)（TX commit 後）

### 10.4 R2 PUT vs DB TX の順序

選択肢:

| 案 | 順序 | 障害時 |
|---|---|---|
| **案 R1（推奨）** | 1) R2 PUT(display) → 2) R2 PUT(thumbnail) → 3) DB TX → 4) R2 DELETE(original) | (3) 失敗で R2 に variant が残るが orphan、Reconcile で cleanup |
| 案 R2 | DB TX → R2 PUT | DB は available 状態で R2 に variant 無い「壊れた状態」になる、最悪 |
| 案 R3 | R2 PUT + DB TX を「2-phase」相当で再試行 | 複雑、PR23 範囲超過 |

→ **案 R1 推奨**。TX 失敗時の R2 orphan は許容（Reconcile が PR25 で対応）。

---

## 11. Outbox 連携

PR23 では **実装しない**（PR25 に分離）。本書には event 名 / payload 案だけ記述:

| Event 名 | 発火タイミング | payload |
|---|---|---|
| `ImageBecameAvailable` | image-processor で MarkAvailable 成功 | `{image_id, photobook_id, available_at, variants: [display, thumbnail]}` |
| `ImageFailed` | MarkFailed 成功 | `{image_id, photobook_id, failure_reason, failed_at}` |
| `PhotobookOgpRegenerationRequested` | available image が cover_image に該当 | `{photobook_id}`（OGP 生成は別集約、PR24） |

PR23 では UseCase 戻り値で event 構造体を返す形にしておき、PR25 で Outbox INSERT を追加するだけで済むよう
**event payload を return value に整形**しておく。

---

## 12. API / command 方針

### 12.1 PR23 で追加する command

`backend/cmd/image-processor/main.go`:

```
Usage:
  image-processor --once <image_id>      1 件処理
  image-processor --all-pending          processing 状態の全 image を処理
  image-processor --dry-run [...]        DB / R2 に副作用を出さず動作確認
  image-processor --max-images <n>       1 回の実行で処理する上限（既定 50）
  image-processor --timeout <duration>   1 image あたりの timeout（既定 60s）
```

env 経由:
- `DATABASE_URL`（Cloud SQL Auth Proxy 経由）
- `R2_*`（PR21 と同じ）

### 12.2 HTTP endpoint は追加しない

ADR-0005 §image-processor 通り、processor は非同期の独立実行体。HTTP endpoint で Public 化すると
権限管理 / DDoS / 順序制御が複雑化するため避ける。

### 12.3 PR23 完了直後の運用

PR22 で残存している processing 4 件:
1. `cmd/image-processor --all-pending` を 1 回実行
2. 既に R2 cleanup 済（PR22 work-log）なので、4 件とも `MarkFailed(object_not_found)` で整理
3. その後の運用 image は PR25 で Cloud Run Jobs に乗せる

---

## 13. Test 方針

### 13.1 PR23 で書くテスト

**imaging unit test（pure Go、fixture image を `testdata/`）**:
- JPEG decode → resize 1600px → JPEG encode が成立
- PNG decode → resize → JPEG encode 成立
- WebP decode → resize → JPEG encode 成立
- 出力 binary に EXIF marker (`APP1` / `Exif` literal) が含まれない
- 寸法 8192px の画像も処理可能、8193px は dimensions_too_large
- アニメーション WebP は animated_image_not_allowed
- 1x1 px 画像も無事処理（境界）

**fake R2 test**:
- GetObject success → bytes
- GetObject 404 → ErrObjectNotFound
- PutObject success
- DeleteObject success / not found

**ProcessImage UseCase test（実 DB + fake R2）**:
- 正常: processing → available + display / thumbnail variant が image_variants に INSERT される
- HEIC: unsupported_format で MarkFailed
- 寸法超過: dimensions_too_large で MarkFailed
- decode 失敗: decode_failed で MarkFailed
- 既に available の image は noop（idempotent）
- 既に failed の image は noop
- R2 GetObject 404 → object_not_found で MarkFailed

**cmd/image-processor test**:
- `--dry-run` で DB / R2 に副作用を出さない（mock 経由）
- `--once <image_id>` で 1 件処理
- `--all-pending` で複数件処理（max-images 上限を超えない）

**Secret/token 漏洩 grep**:
- 既存 grep 設定を継承

### 13.2 PR23 で書かないテスト

- 実 R2 への E2E（手動 1 回確認のみ、PR23 完了直前）
- HEIC 変換 success path（PR25 で libheif 採用後）
- Cloud Run Jobs / Scheduler 実環境動作（PR25）

---

## 14. Security / privacy

- EXIF / GPS / シリアル / PC username / 撮影日時を JPEG re-encode で除去（output に APP1 segment 無し）
- decode timeout 60s（decompression bomb 対策）
- max dimensions 8192px / 40MP（PR18 既存 CHECK 維持）
- max bytes 10MB（HeadObject で再確認）
- SVG 拒否継続（magic number で識別）
- file name は logs / image_variants 列に保存しない
- storage_key / presigned URL / R2 credentials を logs に出さない
- decode panic は recover で捕捉して `decode_failed` として扱う
- malicious image（巨大 chunk / 異常 marker）は decode 関数の error を信頼

---

## 15. PR23 実装範囲（明確化）

### PR23 で実装する

- `backend/internal/imageprocessor/` 新パッケージ（domain / infrastructure / internal/usecase）
- `backend/internal/imageprocessor/infrastructure/imaging/`: pure Go decode / resize / encode
- `backend/internal/imageprocessor/internal/usecase/process_image.go`: 1 image 処理
- `backend/internal/imageprocessor/internal/usecase/process_pending.go`: 一括処理
- R2 client: 既存 `internal/imageupload/infrastructure/r2/` を共有 or `imageprocessor/infrastructure/r2/` に拡張（GetObject 追加が必要）
- `backend/cmd/image-processor/main.go`: CLI ツール
- sqlc: `ListProcessingImagesForUpdate` 追加
- `image/domain/vo/storage_key`: JPEG 拡張子対応の helper（or 既存修正）
- `backend/Dockerfile`: image-processor binary 同梱
- testdata/ に最小 fixture image（JPEG / PNG / WebP、各 1KB-10KB の小サイズ）
- tests
- 作業ログ + PR25 への引き継ぎメモ

### PR23 で実装しない

- HEIC 本対応（libheif + cgo）
- 公開 Viewer での画像表示
- OGP 生成
- moderation UI / Report
- Outbox events 本実装（event payload 案だけ記述）
- Cloud Run Jobs / Scheduler 本番運用
- SendGrid
- Public repo 化
- Cloud SQL 削除 / spike 削除

---

## 16. Cloud SQL 残置/一時削除判断

### 16.1 PR23 計画書完了時点での判断材料

- PR23 実装にすぐ進むなら残置（Repository test + 実環境 1 回確認に DB 必要）
- 数日空くなら一時削除
- 累計（PR17 完了から本書まで）: ~12 時間 / ~¥28
- R2 object 増加は cleanup 戦略で抑制（PR23 完了時に `--all-pending` で清掃）

### 16.2 推奨

**残置継続**（PR23 実装に連続着手予定）。
次回判断タイミング: 「PR23 実装 PR の完了時 or 2 日後」の早い方。

---

## 17. ユーザー判断事項（PR23 着手前に確認）

| # | 判断対象 | 推奨案 | 代替案 | 影響 |
|---|---|---|---|---|
| Q1 | HEIC を PR23 でどう扱うか | **`unsupported_format` で fail**（案 H1、pure Go 維持） | libheif + cgo（案 H2） | distroless static / Dockerfile 維持 |
| Q2 | iPhone Safari ユーザーの HEIC 体験 | **Frontend で HEIC を accept から外す**（PR22 一時修正） or 注意文 | accept 含めたまま Backend で fail させる | UX |
| Q3 | image-processor を API image に同梱 | **同梱**（同 image・別 binary） | 別 image で build | Dockerfile 工事 1 回で済む |
| Q4 | 画像ライブラリ | **標準 + golang.org/x/image/webp + disintegration/imaging** | govips / vips（cgo） | pure Go 維持 |
| Q5 | display / thumbnail format | **両方 JPEG**（互換性最優先） | display=WebP / thumbnail=JPEG | Frontend / Viewer 互換 |
| Q6 | display / thumbnail サイズ | **display=1600px / thumbnail=480px**（長辺基準） | 別サイズ | Viewer 互換 |
| Q7 | original の保存 | **PR23 完了時に削除**（v4 U9） | 残す | Storage コスト / 設計通り |
| Q8 | R2 PUT vs DB TX 順序 | **R2 PUT → DB TX → R2 DELETE(original)** | DB TX → R2 PUT | 障害時 orphan を許容、Reconcile で cleanup |
| Q9 | Outbox を PR23 で入れるか | **入れない**（PR25 に分離、event payload 案だけ計画書に記述） | PR23 で入れる | スコープ |
| Q10 | image-processor の起動形態 | **CLI（cmd/image-processor）**、ローカル / Cloud Run Jobs 兼用 | HTTP endpoint | 設計簡素化 |
| Q11 | Cloud Run Jobs を PR23 で作るか | **作らない**（PR25 で別途）。PR23 では手動 / SSH 経由実行 | 作る | スコープ |
| Q12 | 既存 processing 4 件の処理 | **PR23 完了直後に `--all-pending` 実行**で `MarkFailed(object_not_found)` 整理 | 残置 | DB 整合 |
| Q13 | timeout / limits | **decode 60s / max-images 50 / dimensions 8192px** | 別値 | Cloud Run Jobs request timeout 内 |
| Q14 | testdata fixture | **小サイズ（1KB-10KB）の JPEG / PNG / WebP を testdata/ に commit** | 大サイズ / fixtures 別 repo | repo size 微増 |
| Q15 | EXIF 除去確認 | **JPEG 出力に APP1 / Exif marker が含まれないこと**を test で grep | バイナリ比較 | 必要十分 |
| Q16 | Cloud SQL 残置 | **残置継続**（PR23 連続着手） | 一時削除 | 推奨 |
| Q17 | Public repo 化 | **PR23 完了後でも保留**（PR24 OGP / PR25 Outbox / 1 週間 Secret 安定の後） | 公開 | 推奨 |

Q1〜Q17 への回答後、PR23 実装に進む。

---

## 18. 関連

- [ADR-0005 画像アップロード方式](../adr/0005-image-upload-flow.md)
- [Image ドメイン設計](../design/aggregates/image/ドメイン設計.md)
- [Image データモデル設計](../design/aggregates/image/データモデル設計.md)
- [PR18 Image aggregate 計画](./m2-image-upload-plan.md)
- [PR19 Photobook ↔ Image 連携](./m2-photobook-image-connection-plan.md)
- [PR20 Upload Verification 計画](./m2-upload-verification-plan.md)
- [PR21 R2 + presigned URL 計画](./m2-r2-presigned-url-plan.md)
- [PR22 Frontend upload UI 計画](./m2-frontend-upload-ui-plan.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
- [`.agents/rules/testing.md`](../../.agents/rules/testing.md)
- [`docs/security/public-repo-checklist.md`](../security/public-repo-checklist.md)
