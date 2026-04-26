# M2 R2 + presigned URL 実装計画（PR21 候補）

> 作成日: 2026-04-27
> 位置付け: PR20（Upload Verification / Turnstile foundation）完了後、画像の実体保存先
> として Cloudflare R2 を使い、presigned PUT URL 発行 + complete-upload による
> R2 HeadObject 確認を実装するフェーズの入口。実装コードはまだ書かない。
>
> 上流参照（必読）:
> - [`docs/adr/0005-image-upload-flow.md`](../adr/0005-image-upload-flow.md)
> - [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §2.8 / §3.10
> - [`docs/plan/m2-image-upload-plan.md`](./m2-image-upload-plan.md)
> - [`docs/plan/m2-photobook-image-connection-plan.md`](./m2-photobook-image-connection-plan.md)
> - [`docs/plan/m2-upload-verification-plan.md`](./m2-upload-verification-plan.md)
> - [`docs/design/aggregates/image/ドメイン設計.md`](../design/aggregates/image/ドメイン設計.md)
> - [`docs/design/aggregates/image/データモデル設計.md`](../design/aggregates/image/データモデル設計.md)
> - [`harness/spike/backend/README.md`](../../harness/spike/backend/README.md) M1 R2 PoC 実証結果
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
> - [`.agents/rules/wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)

---

## 0. 本計画書の使い方

- 設計の正典は `docs/adr/0005-image-upload-flow.md`。本書はそれを **PR21 として
  どこまで切り出すか + ユーザー手動操作の停止ポイントをどこに置くか** を整理する。
- 本 PR から **初めて Cloudflare R2 / Secret Manager / Cloud Run Secret 注入の
  実リソース**が絡む。手動操作の停止ポイントを §13 で明確化する。
- §16 のユーザー判断事項に答えてもらってから PR21 実装に着手する。

---

## 1. 目的

- Cloudflare R2 bucket に画像 binary を直接アップロードできるようにする。
- Backend が presigned PUT URL を発行（15 分有効、ADR-0005）。
- complete-upload で R2 HeadObject を確認、Image を `uploading` → `processing` に進める。
- PR20 の Upload Verification consume を upload-intent と統合する。
- PR22 Frontend upload UI / PR23 image-processor の前提を作る。
- M1 spike の `vrcpb-spike` bucket は流用しない（既存 spike Secret も別管理）。

---

## 2. PR21 の対象範囲

### 対象（PR21 で実装する）

- Cloudflare R2 bucket / API token / CORS 作成計画書（実操作はユーザー手動 §13）
- GCP Secret Manager 登録計画（実操作はユーザー手動 §13）
- Cloud Run revision 更新計画（実操作 §13）
- Backend:
  - R2 client interface (`R2Client` / `Presigner`)
  - AWS SDK for Go v2 S3 client 実装（Cloudflare R2 endpoint）
  - Fake R2 client（test 用）
  - UseCase: `IssueUploadIntent`（Upload Verification consume + Image row 作成 +
    presigned URL 発行）
  - UseCase: `CompleteUpload`（HeadObject + Image MarkProcessing）
  - HTTP handler 2 本: `POST /api/photobooks/{id}/images/upload-intent` /
    `POST /api/photobooks/{id}/images/{imageId}/complete`
  - draft session middleware を流用（既存）
  - Backend 側 CORS 拡張（必要に応じて）
- tests:
  - R2 fake client での upload-intent / complete UseCase
  - HTTP handler integration test（実 DB + fake R2）
  - 失敗パス（content-type / size / verification 失敗 / object not found / 別 photobook 等）

### 対象外（PR21 では実装しない）

- Frontend upload UI / Turnstile widget 表示 / drag-and-drop / progress
- image-processor 本体（EXIF 除去 / variant 生成 / HEIC 変換 / available 化）
- Outbox events（PR23 で発火統合）
- SendGrid
- moderation UI
- Safari / iPhone Safari 実機検証（widget 無しのため PR22 で）
- R2 lifecycle 設定（MVP では未設定）
- 既存 M1 spike R2 / Cloud Run / Cloud SQL の削除
- Real Cloudflare R2 への staging 経由 E2E 自動テスト（手動の curl + 1 回確認は §11）

---

## 3. R2 bucket / credentials 方針

### 3.1 R2 bucket

| 項目 | 値 | 備考 |
|---|---|---|
| 名前 | **`vrcpb-images`** | M1 spike の `vrcpb-spike` と区別 |
| location | `Asia-Pacific (APAC)` 自動選択 | Cloudflare 側で決定 |
| public access | **OFF** | 配信は署名 URL or Workers 経由（M6 確定） |
| versioning | **OFF**（MVP） | 削除即時、復旧は Reconcile / 監査ログで |
| lifecycle | **未設定**（MVP） | orphan は app 側 Reconcile で削除（PR23 / PR24） |

### 3.2 R2 API token / S3 credentials

- token type: **R2 API Token**（Cloudflare Dashboard → R2 → Manage R2 API Tokens）
- permissions: **Object Read & Write**
- bucket scope: **`vrcpb-images` のみ**（他 bucket には触れない）
- TTL: **無期限**（MVP、後日 90 日 rotate を検討）
- 取得する値:
  - **Access Key ID**
  - **Secret Access Key**
  - **Account ID**（endpoint 構築に必要）
  - **Endpoint**: `https://<account_id>.r2.cloudflarestorage.com`

### 3.3 Secret Manager 名前空間

M1 spike `harness/spike/backend/.env.example` の `R2_*` は spike 用。本実装では
**新規 Secret 名で分離**:

| Secret 名 | 用途 | 値 |
|---|---|---|
| `R2_ACCOUNT_ID` | endpoint 構築 | Cloudflare Dashboard 取得値 |
| `R2_ACCESS_KEY_ID` | S3 v4 署名 | 同 |
| `R2_SECRET_ACCESS_KEY` | S3 v4 署名 | 同 |
| `R2_BUCKET_NAME` | 操作対象 bucket 固定 | `vrcpb-images`（環境変数でも可） |
| `R2_ENDPOINT` | endpoint 固定 | `https://<account_id>.r2.cloudflarestorage.com` |

**Secret 値はチャット / コミットメッセージ / 作業ログに貼らない**。Cloud Run revision
更新は対話シェルで `--update-secrets=R2_ACCESS_KEY_ID=R2_ACCESS_KEY_ID:latest` 等。

M1 spike 用 `R2_*` は spike Cloud Run service が参照中のため、上書きしない。本実装は
別 Secret 名で衝突回避（**Secret 名前空間としては同じ `R2_*` だが、Cloud Run service
が `vrcpb-api` / `vrcpb-spike-api` で別 service なので、Cloud Run env 注入レベルでは
分離できる**）。

→ **推奨**: M1 spike は `R2_*` をそのまま使い続け、本実装の `vrcpb-api` には新規
Secret として `R2_*` を別 version で登録する。spike service には注入しない。

### 3.4 R2 CORS（PR21 後半でユーザー手動）

```json
[
  {
    "AllowedOrigins": ["https://app.vrc-photobook.com"],
    "AllowedMethods": ["PUT", "HEAD", "GET"],
    "AllowedHeaders": ["Content-Type", "Content-Length", "Authorization"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 3600
  }
]
```

- Frontend が `app.vrc-photobook.com` から R2 endpoint に直接 PUT
- preflight OPTIONS を許可（Safari / iPhone Safari の挙動差は PR22 確認）
- HEAD / GET も許可（display variant の直 GET / 後続の表示用、ただし MVP では Workers
  経由配信を予定、CORS は将来用に許容）

---

## 4. R2 client 設計

### 4.1 推奨: AWS SDK for Go v2

Cloudflare R2 は S3 互換 API。M1 spike で実証済（`harness/spike/backend/`）。

```go
// R2Client は upload-intent / complete-upload で使う最小機能。
type R2Client interface {
    PresignPutObject(ctx context.Context, in PresignPutInput) (PresignPutOutput, error)
    HeadObject(ctx context.Context, key string) (HeadObjectOutput, error)
    DeleteObject(ctx context.Context, key string) error // 後続 PR で使用予定
}

type PresignPutInput struct {
    StorageKey   string
    ContentType  string
    ContentLength int64  // 申告値、ADR-0005 §Content-Length / aws-sdk-go-v2 仕様で必須
    ExpiresIn    time.Duration
}

type PresignPutOutput struct {
    URL       string
    Headers   map[string]string  // 必須 SignedHeaders（Content-Type / Content-Length 等）
    ExpiresAt time.Time
}

type HeadObjectOutput struct {
    Exists       bool
    ContentLength int64
    ContentType  string
    ETag         string
}
```

### 4.2 Content-Length の扱い（M1 spike 実証結果）

M1 spike で確認済（ADR-0005 §M1 PoC 結果）:

> aws-sdk-go-v2 の presign は Content-Length を SignedHeaders に含めるため、
> 宣言サイズ（PutObjectInput.ContentLength）と実 PUT 時の body サイズが一致しないと
> R2 が 403 SignatureDoesNotMatch を返す

→ presigned URL 発行時に Content-Length を必ず固定し、Frontend は同じサイズで PUT する
ように strict に縛る。Content-Length 不一致時は R2 側で 403。

### 4.3 checksum

- MVP では **使わない**（推奨案）。MVP の防御は HeadObject の `ContentLength` /
  `ContentType` で十分。
- 将来 image-processor で `sha256` を計算 / `images.checksum_sha256` 列に保存する案も
  あるが、現状 schema には未追加（image データモデル §3 で `checksum_sha256` が
  optional）。PR23 image-processor で追加検討。

### 4.4 fake client

`backend/internal/imageupload/tests/fake_r2_client.go` で `R2Client` interface を
struct field 差し替え式で実装。テストでは `HeadObject` / `PresignPutObject` の戻り値を
任意に設定。

---

## 5. StorageKey / object key 方針

### 5.1 命名規則（PR18 の StorageKey VO と整合）

ADR-0005 §storage_key 通り:

```
photobooks/{photobook_id}/images/{image_id}/original/{random}.{ext}
photobooks/{photobook_id}/images/{image_id}/display/{random}.webp
photobooks/{photobook_id}/images/{image_id}/thumbnail/{random}.webp
photobooks/{photobook_id}/ogp/{ogp_id}/{random}.png
```

PR18 で `image/domain/vo/storage_key` に `GenerateForVariant` / `GenerateForOriginal` /
`GenerateForOgp` を実装済。PR21 ではそれを利用する。

### 5.2 upload 直後に保存される object（重要）

ADR-0005 / 業務知識 v4 U9: **MVP では `original` variant を保持しない**。

しかし upload 時は variant 生成前の **元バイナリ**を一時保管する必要がある。
2 つの選択肢:

| 案 | 保存 prefix | 利点 | 欠点 |
|---|---|---|---|
| **案 A（推奨）** | `photobooks/{pid}/images/{iid}/original/{random}.{ext}` | 既存 StorageKey VO の `GenerateForOriginal` を流用 / 階層が一貫 | image-processor 完了後に明示削除が必要 |
| 案 B | `tmp/uploads/{photobook_id}/{image_id}/{random}.{ext}` | 一時データであることが prefix で明示 / lifecycle で自動削除可能 | 別 prefix の運用が増える / 既存 VO の修正必要 |

→ **案 A 推奨**。MVP では「PR21 で original/ に PUT、PR23 image-processor が
display/thumbnail を生成 + original/ を削除」フローで進める。`images.image_variants`
テーブルの `original` variant は MVP では行を作らない（v4 U9）。

なお、image データモデル §4.2「用途別 variant 構成」では `original` variant を MVP で
保持しないことが確定している。PR23 image-processor がこの削除責務を持つ。PR21 では
**original のキーで R2 に置く + image レコードの `images.normalized_format` は NULL のまま**
で processing に進める。

### 5.3 storage_key の DB 保存

PR21 では `image_variants` 行はまだ作らない（image-processor が available 化と同時に
display / thumbnail variant を作る、PR23）。

PR21 段階では **`images` 行に `pending_storage_key` のような新列**を追加する案もあるが、
これは設計上の追加負債を生む。代替案:

| 案 | 仕組み | 利点 | 欠点 |
|---|---|---|---|
| **案 A（推奨）** | StorageKey は **deterministic 生成**（image_id + random で同じ image_id から再現可能ではない、ただし image_id 単位で 1 つだけ生成 + メモリ保持 → presigned URL に embed） | 列追加不要 / image-processor は images.id から `photobooks/{pid}/images/{iid}/original/*` を listObjectsV2 で見つけて処理 | listObjectsV2 のコスト |
| 案 B | `images` テーブルに `original_storage_key` 列を追加 | 直接参照可 | schema 変更が PR21 範囲を超える |
| 案 C | `image_variants` に kind=`pending` を許容 | 既存テーブル流用 | UNIQUE (image_id, kind) と相性悪、kind 列の意味論が崩れる |

→ **案 A 推奨**（schema 変更を避ける、image-processor は ListObjectsV2 で original prefix
を探索）。PR23 で必要なら案 B に昇格を検討。

---

## 6. upload-intent flow

### 6.1 同 TX 内の処理順序

```
1. middleware: vrcpb_draft_<photobook_id> Cookie で draft session 認可
2. handler: photobook_id (URL) と context の draft session photobook_id 一致を確認
3. UseCase IssueUploadIntent:
   a. Upload Verification consume (atomic UPDATE、PR20 既存)
      → 失敗なら 403 ErrUploadVerificationFailed
   b. 申告 content_type / size / source_format の軽量検証
      - content_type whitelist: image/jpeg / image/png / image/webp / image/heic
      - size <= 10MB
      - source_format whitelist: jpg / png / webp / heic
   c. 新規 image_id 生成
   d. images 行 INSERT (status='uploading', owner_photobook_id=$pid,
      usage_kind='photo', source_format=$src_fmt)
   e. storage_key 生成 (photobooks/{pid}/images/{iid}/original/{random}.{ext})
   f. R2 PresignPutObject(ContentLength=申告値, ContentType=申告値, ExpiresIn=15min)
4. response: { image_id, upload_url, headers, storage_key (optional), expires_at }
```

a と d は **同一 DB TX** で実行。Upload Verification consume が成功したのに
images INSERT が失敗したら全体 rollback（consume も巻き戻る）。

### 6.2 失敗時の挙動

| 失敗 | レスポンス | 処理 |
|---|---|---|
| draft session 不正 | 401 | middleware で拒否 |
| photobook_id 不一致 | 403 | handler で拒否 |
| Upload Verification consume 失敗 | 403 `upload_verification_failed` | TX rollback、何もしない |
| content_type / size / source_format 不正 | 400 `invalid_upload_parameters` | TX rollback |
| images INSERT 失敗 | 500 | TX rollback |
| R2 PresignPutObject 失敗 | 500 `r2_presign_failed` | TX rollback（ただし consume も巻き戻る） |

注意: R2 presign は network 不要（純粋に AWS Signature V4 を計算するだけ）なので、
**通常は失敗しない**。ただし client 初期化失敗 / config 不正は起こりうる。

### 6.3 idempotency / orphan / cleanup

- idempotency key は MVP では持たない（同じ photobook_id + intent を複数回呼ぶと
  別 image_id で 20 回まで作られる、Upload Verification 上限で制限済）
- upload intent expiration: 15 分（presigned URL TTL）
- orphan uploading cleanup: 一定時間（推奨 24 時間）`status='uploading'` のままの
  Image を Reconcile で `failed` 化、対応する R2 object を削除。**PR21 では実装しない**、
  PR23 / PR24 で別途。

---

## 7. complete-upload flow

### 7.1 同 TX 内の処理順序

```
1. middleware: vrcpb_draft_<photobook_id> Cookie で draft session 認可
2. handler: photobook_id (URL) と context 一致を確認
3. UseCase CompleteUpload:
   a. images.FindByID(image_id)
      → 不存在なら 404 ErrImageNotFound
   b. owner_photobook_id == photobook_id を確認
      → 不一致なら 404（情報を漏らさない）
   c. status == uploading を確認
      → processing 以降なら 409 idempotent return（既に complete 済の冪等扱い）
   d. R2 HeadObject(storage_key を listObjects で見つける、または別 image VO で保持)
      → 不存在なら image を MarkFailed(reason=object_not_found) → 422 / 4xx
   e. ContentLength <= 10MB を確認
      → 超過なら MarkFailed(reason=file_too_large)
   f. ContentType が申告と一致（or whitelist 内）
      → 不一致なら MarkFailed(reason=size_mismatch / unsupported_format)
   g. images.MarkProcessing()
      → 0 行なら ErrConflict（並行で別 reqest が complete した）
4. response: { image_id, status: "processing" }
```

### 7.2 storage_key 解決の実装

§5.3 案 A の trade-off:
- option 1: complete-upload 時に Frontend から `storage_key` を一緒に送る
  - **推奨**: response の `storage_key` を Frontend に渡し、complete 時に return する
  - サーバ側で deterministic 性を担保するため、storage_key の prefix（photobooks/{pid}/images/{iid}/）を再生成して contains チェック
- option 2: ListObjectsV2 で `photobooks/{pid}/images/{iid}/original/` prefix を listing
  - 1 image につき 1 object のみという前提が崩れたとき detection が困難
  - R2 ListObjectsV2 課金（Class A）

→ **option 1 推奨**。サーバ側で `storage_key` の prefix を検証することで spoofing 防止。

### 7.3 Image MarkProcessing の SQL

PR18 既実装の `UpdateImageStatusProcessing` を流用:

```sql
UPDATE images
   SET status = 'processing',
       updated_at = $2
 WHERE id = $1
   AND status = 'uploading';
```

0 行 → `ErrConflict`（複数 complete 同時呼び出しの片方）。

### 7.4 失敗時の挙動

| 失敗 | レスポンス | DB 影響 |
|---|---|---|
| draft session 不正 | 401 | なし |
| photobook 不一致 | 404 | なし |
| image 不存在 | 404 | なし |
| status != uploading（既に processing 等） | 200（冪等） | MarkProcessing 0 行扱い → 既存 Image を返す |
| HeadObject 不存在 | 422 `object_not_found` | MarkFailed(object_not_found) |
| size 超過 / mismatch | 422 `size_mismatch` | MarkFailed(file_too_large or size_mismatch) |
| content-type 不正 | 422 `unsupported_format` | MarkFailed(unsupported_format) |

---

## 8. Image status 方針

### 8.1 PR21 範囲

| 状態 | 何で遷移するか | PR |
|---|---|---|
| uploading | upload-intent で INSERT | PR21 |
| processing | complete-upload で UPDATE | PR21 |
| available | image-processor で UPDATE | **PR23**（PR21 では実装しない） |
| failed | complete-upload エラー / image-processor エラー | PR21 / PR23 |
| deleted / purged | ops 操作 | PR24 |

### 8.2 PR21 で processing どまりにする理由

- image-processor 本体（HEIC 変換 / EXIF 除去 / variant 生成 / libheif）は範囲外
- PR21 までで「実画像が R2 に届く」「Backend が DB に reflect する」が確認できれば
  Pre-condition は満たす
- PR22 Frontend UI で表示する画像は image-processor 完了後の `display` variant が
  必要だが、PR22 着手時点では fake processor or admin command で processing →
  available に手動遷移できる仕組みを別途検討（§16 Q7）

### 8.3 fake processor（PR21 では作らない、PR22 で検討）

PR22 着手時に「画像が表示できる」状態にするため、暫定的に PR22 で `admin run-processor`
コマンドを作って processing → available に進める案がある（Outbox 抜きの bypass）。
PR21 ではこれを **作らない**。

---

## 9. Backend API 案

### 9.1 PR21 で追加する endpoint

```
POST /api/photobooks/{id}/images/upload-intent
  Cookie: vrcpb_draft_<id>
  Header: Authorization: Bearer <upload_verification_token>
  Body (JSON):
    {
      "content_type": "image/jpeg",
      "declared_byte_size": 1234567,
      "source_format": "jpg"
    }
  Response 201:
    {
      "image_id": "uuid",
      "upload_url": "https://<account>.r2.cloudflarestorage.com/...",
      "required_headers": {
        "Content-Type": "image/jpeg",
        "Content-Length": "1234567"
      },
      "storage_key": "photobooks/{pid}/images/{iid}/original/abc.jpg",
      "expires_at": "2026-04-27T12:15:00Z"
    }

POST /api/photobooks/{id}/images/{imageId}/complete
  Cookie: vrcpb_draft_<id>
  Body (JSON):
    {
      "storage_key": "photobooks/{pid}/images/{iid}/original/abc.jpg"
    }
  Response 200:
    {
      "image_id": "uuid",
      "status": "processing"
    }
```

### 9.2 認可

- 両 endpoint とも **draft session Cookie** が必須
- upload-intent は **追加で `Authorization: Bearer <upload_verification_token>`**
  が必須（PR20 UseCase で consume）
- complete は Authorization 不要（image_id ＋ photobook_id ＋ status の DB 整合性で守る）

### 9.3 ログ / セキュリティ

- presigned URL は **logs に出さない**（response にのみ含める）
- storage_key は response に出すが logs には出さない
- R2 credentials は Backend env 経由でのみ参照、絶対に response / logs に出さない

---

## 10. CORS 方針

### 10.1 Backend CORS（既存 + 拡張）

現在 Backend は `app.vrc-photobook.com` からのリクエストを受ける middleware を
持っている（PR12 の `ALLOWED_ORIGINS`）。

PR21 で追加が必要かどうか:
- `Authorization` header を許可
- Cookie credentials は `SameSite=Strict` のため cross-origin で送れない（同一 origin
  扱い、`api.vrc-photobook.com` ↔ `app.vrc-photobook.com` は別 origin だが
  `Domain=.vrc-photobook.com` Cookie で対応）
- Content-Type / preflight OPTIONS 対応

→ **PR21 で Backend CORS の見直し**（既存 middleware の許可 header / method を拡張）。

### 10.2 R2 CORS（§3.4）

R2 bucket 側で CORS 設定を行う（Cloudflare Dashboard、ユーザー手動）。Frontend が
直接 PUT する前提のため必須。

### 10.3 Safari / iPhone Safari の挙動

- preflight OPTIONS が走る複数 header 送信時の挙動差は PR22 で実機確認
- `Access-Control-Allow-Origin` は `*` ではなく `app.vrc-photobook.com` 厳格一致
- **PR21 では Safari 実機確認を行わない**（Frontend UI なし）

---

## 11. Security / validation

| 項目 | 値 / 方針 |
|---|---|
| max size | 10MB（申告 + R2 HeadObject の二重確認） |
| content-type whitelist | `image/jpeg` / `image/png` / `image/webp` / `image/heic` |
| 拡張子 trust | しない（PR23 image-processor で magic number 検証） |
| SVG | **拒否**（content-type / source_format に含めない） |
| HTML upload | **拒否**（同上） |
| filename | DB 保存しない（プライバシー / 表示用には caption を使う） |
| checksum | **使わない**（MVP、PR23 で再検討） |
| presigned URL expiration | **15 分** |
| Upload Verification | 20 intent / 30 分（PR20 既存） |
| draft session | 必須 |
| logs に出さない | presigned URL / storage_key / raw token / Cookie / R2 credentials |
| public bucket | **禁止** |
| Content-Length signature | **必須**（M1 spike で 403 SignatureDoesNotMatch を実証済、ADR-0005 §M1 PoC） |

---

## 12. Test 方針

### 12.1 PR21 で書くテスト

**fake R2 client tests**:
- PresignPutObject 戻り値 URL / Headers / ExpiresAt
- HeadObject 存在 / 不存在 / size / content-type 各パターン

**UseCase tests（実 DB + fake R2）**:
- IssueUploadIntent 成功（Upload Verification consume + Image INSERT + presigned URL）
- IssueUploadIntent 失敗:
  - Upload Verification token 不正
  - content_type 拒否（svg / html）
  - size 超過
  - source_format 不正
  - photobook 不一致
- CompleteUpload 成功（HeadObject OK → MarkProcessing）
- CompleteUpload 失敗:
  - object not found → MarkFailed(object_not_found)
  - size mismatch → MarkFailed(file_too_large)
  - content-type mismatch → MarkFailed(unsupported_format)
  - 別 photobook の image_id
  - status != uploading（idempotent return）
  - storage_key prefix 不一致

**HTTP handler integration tests**:
- POST upload-intent → 201
- POST complete → 200
- 失敗パスを HTTP code で確認

**migration**:
- 既存 PR18 の images / image_variants schema は変更なし（PR21 では migration 追加なし）

### 12.2 PR21 で書かないテスト

- 実 Cloudflare R2 への presign + PUT + HeadObject E2E（手動 curl で 1 回確認、§13）
- Frontend E2E（PR22）
- Safari widget 表示 / preflight 動作（PR22）
- image-processor variant 生成（PR23）

---

## 13. Cloudflare / GCP 実操作計画（停止ポイント明確化）

### 13.1 全体フロー

```
[Step A] PR21 実装着手 (Backend code, fake R2 のみで TDD)
   ↓
[Step B] STOP: ユーザー手動 Cloudflare Dashboard 操作
   - vrcpb-images bucket 作成
   - R2 API token 作成（Object Read & Write、bucket=vrcpb-images）
   - R2 CORS 設定
   - 取得値: account_id / access_key_id / secret_access_key
   - **値はチャットに貼らない**
   ↓
[Step C] STOP: ユーザー手動 GCP Secret Manager 登録
   - R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY を登録
   - 対話シェルで `gcloud secrets versions add ... --data-file=-`
   - Cloud Run service account の secretAccessor 権限を確認
   ↓
[Step D] STOP: ユーザー手動 Cloud Run revision 更新
   - `gcloud run services update vrcpb-api --update-secrets=...`
   - --update-env-vars=R2_BUCKET_NAME=vrcpb-images,R2_ENDPOINT=https://...
   - new revision にトラフィック 100%
   ↓
[Step E] Claude Code: 実 R2 への curl 1 回確認
   - upload-intent POST → presigned URL 取得
   - その URL に curl PUT で 1024 byte の dummy 画像
   - HeadObject 確認
   - complete POST → processing 遷移確認
   - 実画像は upload 後すぐ削除（手動）
   ↓
[Step F] PR21 実装完了として PR commit / push
```

### 13.2 各停止ポイントの確認内容

| Step | ユーザー実施 | Claude Code 客観確認 |
|---|---|---|
| B | Dashboard で bucket / token / CORS 作成 | bucket 名のみ報告、値はチャット禁止 |
| C | Secret 登録 | `gcloud secrets versions list` で `enabled` 確認、値は確認しない |
| D | revision 更新 | `gcloud run services describe vrcpb-api --format=...` で env 注入確認 |
| E | （ユーザー対話シェル経由）curl 確認 | Claude Code が指示出し、出力解釈 |

### 13.3 Turnstile widget の同タイミング作成

PR20 計画書で「Turnstile Dashboard 操作は PR21 まで先送り」と決めた。**PR21 で R2 と
同時に実施する**のが効率的（Cloudflare Dashboard 訪問が 1 回で済む）。

```
[Step B 拡張]:
   - vrcpb-images R2 bucket 作成
   - R2 API token 作成
   - R2 CORS 設定
   - Turnstile widget 作成（hostname=app.vrc-photobook.com、action=upload）
   - 取得値: R2 ×3 + Turnstile sitekey + Turnstile secret
```

### 13.4 サブシェルとパス制約

`wsl-shell-rules.md` に従い、Cloudflare Dashboard / Secret Manager 操作はユーザー
対話シェルで実施。Claude Code Bash からは `gcloud` の客観確認のみ。

---

## 14. PR21 の実装範囲（明確化）

### PR21 で実装する

- Backend:
  - `internal/imageupload/`（新パッケージ） or `internal/image/internal/usecase/`
    に upload-intent / complete UseCase を配置（§16 Q5）
  - R2 client interface + AWS SDK v2 実装 + fake
  - HTTP handler 2 本 + router 統合
  - Backend CORS middleware（既存拡張）
- tests: fake R2 client での UseCase / handler integration
- Step E の手動 curl 確認（実 R2 で 1 回）

### PR21 で実装しない

- Frontend upload UI / Turnstile widget 表示
- image-processor 本体（HEIC / EXIF / variant 生成 / available 化）
- Outbox events / SendGrid
- moderation UI
- Safari / iPhone Safari 実機確認
- R2 lifecycle / cleanup batch（PR23 / PR24）
- 既存 spike R2 / Cloud Run / Cloud SQL の削除

---

## 15. Cloud SQL 残置/一時削除判断

### 15.1 PR21 計画書完了時点での判断材料

- PR21 実装にすぐ進むなら残置（Repository / UseCase test / 実 R2 連動 curl 確認まで連続）
- 数日空くなら一時削除
- 累計経過: ~7 時間 / ~¥17（PR17 完了から）。30 日放置で予算 ¥1,000 超過リスク
- 再作成 ~10 分

### 15.2 推奨

**残置継続**（PR21 実装に連続着手予定 + 実 R2 curl 確認も DB 必要）。
次回判断タイミング: 「PR21 実装 PR の完了時 or 2 日後」の早い方。

---

## 16. ユーザー判断事項（PR21 着手前に確認）

| # | 判断対象 | 推奨案 | 代替案 | 影響 |
|---|---|---|---|---|
| Q1 | R2 bucket 名 | **`vrcpb-images`** | 別名（avoid `vrcpb-spike`） | spike と区別 |
| Q2 | Secret 名前空間 | **本実装用に `R2_*` を新規 Secret として登録**（spike と service レベルで分離） | 既存 spike Secret を上書き | 推奨案安全 |
| Q3 | R2 bucket / token / CORS 作成タイミング | **PR21 着手後 Step B で実施** | 計画書段階で先行実施 | 推奨案で実装と密結合 |
| Q4 | Turnstile widget 作成タイミング | **PR21 Step B で R2 と同時** | PR22 Frontend 直前 | Dashboard 訪問 1 回で済む |
| Q5 | upload-intent / complete UseCase の配置 | **`internal/imageupload/` 新パッケージ**（image / photobook / uploadverification を横断） | `internal/image/internal/usecase/` に統合 | 横断 UseCase の関心分離 |
| Q6 | PR21 で API endpoint 追加 | **追加する**（upload-intent / complete の 2 本） | しない（UseCase まで） | PR22 で curl / Frontend から叩ける |
| Q7 | complete 完了時の Image 状態 | **processing 止まり**（image-processor は PR23） | available まで（fake processor） | 推奨案で範囲固定 |
| Q8 | storage_key を response に出すか | **出す**（complete 時の prefix 検証用、Frontend が echo back） | 出さない | Frontend 実装で必要 |
| Q9 | checksum を使うか | **MVP では使わない** | 使う（sha256） | PR23 で再検討 |
| Q10 | R2 CORS 設定タイミング | **Step B（R2 bucket 作成と同時）** | PR22 直前 | bucket 作成済の手順に組み込み |
| Q11 | Backend CORS middleware 拡張 | **PR21 で実施**（Authorization header 許可） | 既存のまま | upload-intent で必要 |
| Q12 | R2 への手動 curl 確認 (Step E) | **PR21 完了直前に 1 回実施**（dummy 画像 1024 byte） | 不要 | 実環境動作確認 |
| Q13 | original variant の保存 prefix | **`photobooks/{pid}/images/{iid}/original/{random}.{ext}`**（StorageKey VO 流用） | `tmp/uploads/...` 別 prefix | 推奨案で既存 VO 流用 |
| Q14 | upload-intent idempotency key | **持たない**（Upload Verification 上限で代替） | 持つ | MVP シンプル |
| Q15 | orphan uploading cleanup | **PR21 では実装しない**（PR23 / PR24 で Reconcile 整備） | PR21 内 cron | 範囲固定 |
| Q16 | Cloud SQL 残置 | **残置継続**（PR21 連続着手） | 一時削除 | PR20 判断と整合 |

Q1〜Q16 への回答後、PR21 着手フローに進む。

---

## 17. 関連

- [ADR-0005 画像アップロード方式](../adr/0005-image-upload-flow.md)
- [Image 集約 計画](./m2-image-upload-plan.md)
- [Photobook ↔ Image 連携 計画](./m2-photobook-image-connection-plan.md)
- [Upload Verification / Turnstile 計画](./m2-upload-verification-plan.md)
- [Image ドメイン設計](../design/aggregates/image/ドメイン設計.md)
- [Image データモデル設計](../design/aggregates/image/データモデル設計.md)
- [M1 R2 PoC 実証結果](../../harness/spike/backend/README.md)
- [Post-deploy Final Roadmap](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
- [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
- [`.agents/rules/wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
