# M2 Image / Upload 実装計画（PR18 候補）

> 作成日: 2026-04-27
> 位置付け: M2 ドメイン疎通フェーズ（PR12〜PR17）完了後、Image / Upload フェーズ
> （PR18〜PR24 想定）の入口。実装コードはまだ書かない。
>
> 上流参照（必読）:
> - [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §3.7 / §3.10 / §6.12 / §6.14
> - [`docs/adr/0005-image-upload-flow.md`](../adr/0005-image-upload-flow.md)
> - [`docs/design/aggregates/image/ドメイン設計.md`](../design/aggregates/image/ドメイン設計.md)
> - [`docs/design/aggregates/image/データモデル設計.md`](../design/aggregates/image/データモデル設計.md)
> - [`docs/design/aggregates/photobook/データモデル設計.md`](../design/aggregates/photobook/データモデル設計.md) §3.2 owner_photobook_id
> - [`docs/design/cross-cutting/outbox.md`](../design/cross-cutting/outbox.md)
> - [`harness/work-logs/2026-04-27_post-deploy-final-roadmap.md`](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)
> - [`docs/plan/m2-photobook-session-integration-plan.md`](./m2-photobook-session-integration-plan.md)（PR 分割テンプレ）
> - [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md)
> - [`.agents/rules/testing.md`](../../.agents/rules/testing.md)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)

---

## 0. 本計画書の使い方

- 設計の正典は `docs/design/aggregates/image/` と `docs/adr/0005-image-upload-flow.md`。
  本書はそれを **どう PR に切り出すか / PR18 の境界をどこに置くか** を整理する。
- Image / Upload は範囲が広いので、**PR18〜PR24 のフェーズに分割**する（§2）。
- **PR18 の実装範囲は domain model + migration + sqlc + repository + test に限定**する。
  R2 / Turnstile / presigned URL / Frontend は PR19〜PR22 で別 PR に切り出す。
- §18 のユーザー判断事項に答えてもらってから PR18 実装に着手する。

---

## 1. 目的

- M2 ドメイン疎通完了状態（PR17）に画像アップロードの土台を載せる。
- Image aggregate のドメインモデル + DB スキーマ + Repository を確定させる。
- 画像メタは DB（`images` / `image_variants`）、画像バイナリは R2（後続 PR）の
  分離方針を **データレイヤから** 確立する。
- Photobook ↔ Image の所有関係（`owner_photobook_id`、ADR-0005 / image データモデル §3.2）
  を MVP 段階から DB 制約として担保する。
- presigned URL / Turnstile / EXIF 除去 / variant 生成 / Outbox は本 PR では実装しない。
  PR19 以降の入り口として API / 実装方針だけ明記する。

---

## 2. フェーズ分割（PR18〜PR24 想定）

| PR | 名称 | 主な実装範囲 | 主要参照 |
|---|---|---|---|
| **PR18** | **Image aggregate domain + DB** | Image / ImageVariant / VO、`images` / `image_variants` migration、sqlc、Repository、unit / repository test | image 設計、ADR-0005 |
| PR19 | Photobook ↔ Image 連携 | `pages` / `photos` / `page_metas` migration、Photobook 側の Image 参照 UseCase（addPhoto / removePhoto / setCover）、Photobook の `cover_image_id` FK 追加 | photobook 設計 |
| PR20 | Upload-Verification (Turnstile) | `upload_verification_sessions` migration、Turnstile 検証（test sitekey）、UseCase、middleware | ADR-0005 §Turnstile, ADR-0001 |
| PR21 | R2 + presigned URL | R2 bucket、Secret Manager、`POST /api/photobooks/{id}/images/upload-intent` / `POST /api/images/{id}/complete`、HeadObject 検証 | ADR-0005 §presigned, R2 spike |
| PR22 | 編集 UI 最小骨格 | `/edit/<id>` の photo upload UI（design/mockups Edit 画面参照）、CORS、Safari 実機検証 | design/mockups, safari-verification |
| PR23 | Outbox + image-processor 雛形 | `outbox_events` migration、`ImageIngestionRequested` 発火、image-processor の最小 stub（実検証は別） | outbox 設計 |
| PR24 | OGP / Moderation / 公開ページ整備 | `photobook_ogp_images` migration、moderation 状態の Image 反映、Report 連携 | OGP / moderation 設計 |

PR23 / PR24 は範囲がまだ広いので、PR21 完了時点でさらに分割を判断する。

---

## 3. Image aggregate 設計（PR18 で実装するもの）

正典は [Image ドメイン設計](../design/aggregates/image/ドメイン設計.md)。本節は **PR18 で
実装する範囲** に絞った要約。設計と差異が出た場合は設計が優先。

### 3.1 Entity / VO

PR18 で実装する Entity / VO:

- Entity: `Image`（集約ルート）、`ImageVariant`
- VO（既存 `photobook_id` を再利用）:
  - `image_id`（UUIDv7）
  - `image_usage_kind`（`photo` / `cover` / `ogp`）
  - `image_format`（`jpg` / `png` / `webp` / `heic`）
  - `normalized_format`（`jpg` / `webp`）
  - `image_dimensions`（width × height、1〜8192 / 40MP 上限）
  - `byte_size`（1〜10485760 byte）
  - `mime_type`（`image/jpeg` / `image/png` / `image/webp`）
  - `storage_key`（命名規則は §5）
  - `variant_kind`（`original` / `display` / `thumbnail` / `ogp`）
  - `image_status`（`uploading` / `processing` / `available` / `failed` / `deleted` / `purged`）
  - `failure_reason`（12 種固定、image データモデル §3.0）

`PhotobookId` は既存 `backend/internal/photobook/domain/vo/photobook_id` を再利用する
（`vrcpb/backend/internal/photobook/domain/vo/photobook_id`）。

### 3.2 不変条件（PR18 で domain / DB 双方に効かせる）

- `owner_photobook_id` は生成時に決まり、以降変更不可
- `storage_key` は `image_variants` の `(image_id, kind)` UNIQUE で 1 画像 1 種 1 行
- `content_type` / `mime_type` は許可リストのみ（CHECK）
- `original_byte_size` ≤ 10MB（CHECK）
- `original_width` / `original_height` ≤ 8192、合計 ≤ 40MP（domain 側で検証、DB は単軸 CHECK）
- `status=failed` のときは `failure_reason` が必須（CHECK）
- `status=available` のときは `normalized_format` / `metadata_stripped_at` /
  `original_*` / `variants(display, thumbnail|ogp)` が必須
- `deleted_at` がある画像は通常表示対象外（Repository で WHERE 句に組み込む）
- `uploading` / `processing` の Image は Photobook attach 不可（domain で検証）

### 3.3 状態遷移（PR18 では status のみ持ち、遷移ロジックは最小）

```
uploading ─┬─► processing ─┬─► available
           │               └─► failed
           └─► failed
available ─► deleted ─► purged
failed    ─► deleted ─► purged
```

PR18 の UseCase は domain メソッドの遷移検証のみ。Outbox 発火 / 非同期処理は PR21 / PR23。

---

## 4. DB 設計案（PR18 で追加する migration）

正典は [Image データモデル設計](../design/aggregates/image/データモデル設計.md)。本節は
PR18 で **どの migration ファイルをどの順で追加するか** を整理する。

### 4.1 追加 migration

既存 migration（PR9a / PR9b で適用済み）:
```
00001_create_health_check.sql
00002_create_sessions.sql
00003_create_photobooks.sql
00004_add_photobooks_fk_to_sessions.sql
```

PR18 で追加:

| 番号 | ファイル | 内容 |
|---|---|---|
| 00005 | `00005_create_images.sql` | `images` テーブル作成（CHECK / index 含む） |
| 00006 | `00006_create_image_variants.sql` | `image_variants` テーブル作成（FK / UNIQUE 含む） |

### 4.2 `images` の `owner_photobook_id` FK タイミング

選択肢:

| 案 | FK 追加タイミング | 利点 | 欠点 |
|---|---|---|---|
| **案 A（推奨）** | PR18 で追加（`ON DELETE RESTRICT`） | 不正な所有関係が即座に拒否される / image データモデル §3.2 と完全一致 | photobook 削除時は明示削除フローが必須（既に設計通り） |
| 案 B | PR19 で追加 | 同 PR で `pages` / `photos` を入れるため一括で扱いやすい | PR18 期間中に整合性が取れない一時状態が発生（スパイク的検証で混乱の温床） |

→ **案 A 推奨**。Photobook の purge ハンドラは PR23 / PR24 で実装するが、それまでは
RESTRICT が効くだけで実害なし。

### 4.3 `cover_image_id` FK の扱い

既存 `photobooks.cover_image_id` カラムには FK が張られていない（00003 コメント参照）。

- PR18 では追加しない（`images` は新規追加直後のため整合性の心配はない）
- PR19 で `ALTER TABLE photobooks ADD CONSTRAINT photobooks_cover_image_id_fkey ...`
  を入れる（`ON DELETE SET NULL`）

### 4.4 `images` 主要列（image データモデル設計 §3 と完全一致）

```sql
id                    uuid        NOT NULL PRIMARY KEY
owner_photobook_id    uuid        NOT NULL  -- FK photobooks(id) ON DELETE RESTRICT
usage_kind            text        NOT NULL  -- CHECK photo|cover|ogp
source_format         text        NOT NULL  -- CHECK jpg|png|webp|heic
normalized_format     text        NULL      -- CHECK jpg|webp、available 以降必須
original_width        int         NULL      -- <= 8192
original_height       int         NULL      -- <= 8192
original_byte_size    bigint      NULL      -- <= 10485760
metadata_stripped_at  timestamptz NULL
status                text        NOT NULL DEFAULT 'uploading'
                                            -- CHECK uploading|processing|available|failed|deleted|purged
uploaded_at           timestamptz NOT NULL DEFAULT now()
available_at          timestamptz NULL
failed_at             timestamptz NULL
failure_reason        text        NULL      -- CHECK 12 種固定
deleted_at            timestamptz NULL
created_at            timestamptz NOT NULL DEFAULT now()
updated_at            timestamptz NOT NULL DEFAULT now()
```

CHECK 制約（抜粋、詳細は image データモデル §3.0 / §3.1）:

```sql
-- failed のときは failure_reason 必須
CHECK (status != 'failed' OR failure_reason IS NOT NULL)

-- available のときは normalized_format / 寸法 / size 必須
CHECK (
  status NOT IN ('available', 'deleted', 'purged')
  OR (normalized_format IS NOT NULL
      AND original_width IS NOT NULL
      AND original_height IS NOT NULL
      AND original_byte_size IS NOT NULL)
)

-- 寸法上限
CHECK (original_width IS NULL OR original_width BETWEEN 1 AND 8192)
CHECK (original_height IS NULL OR original_height BETWEEN 1 AND 8192)
CHECK (original_byte_size IS NULL OR original_byte_size BETWEEN 1 AND 10485760)
```

INDEX:

- `(owner_photobook_id)`
- `(owner_photobook_id, usage_kind)`
- `(status, deleted_at)`
- `(status, failed_at) WHERE status = 'failed'`
- `(status, available_at) WHERE status = 'available'`

### 4.5 `image_variants` 主要列

```sql
id           uuid        NOT NULL PRIMARY KEY
image_id     uuid        NOT NULL  -- FK images(id) ON DELETE CASCADE
kind         text        NOT NULL  -- CHECK original|display|thumbnail|ogp
storage_key  text        NOT NULL
width        int         NOT NULL  -- >= 1
height       int         NOT NULL  -- >= 1
byte_size    bigint      NOT NULL  -- >= 1
mime_type    text        NOT NULL  -- CHECK image/jpeg|image/png|image/webp
created_at   timestamptz NOT NULL DEFAULT now()

UNIQUE (image_id, kind)
INDEX  (storage_key)
```

`storage_key` は **bucket 名を含めず** に保存する（バケット切替に耐える）。

---

## 5. StorageKey 設計（命名規則）

ADR-0005 §storage_key に確定済の規則を踏襲。**PR18 では generation 関数のみ実装し、
R2 への put は行わない**。

```
photobooks/{photobook_id}/images/{image_id}/original/{random}.{ext}
photobooks/{photobook_id}/images/{image_id}/display/{random}.webp
photobooks/{photobook_id}/images/{image_id}/thumbnail/{random}.webp
photobooks/{photobook_id}/ogp/{ogp_id}/{random}.png
```

- `{random}` は 12 byte の暗号論的乱数を base64url（padding なし）で 16 文字
- `{ext}` は original のみ元拡張子（`jpg` / `png` / `webp` / `heic`）、他は固定
- `{photobook_id}` / `{image_id}` は UUID 形式の文字列

選定理由（ADR-0005 §storage_key）:

- 推測困難性: `{random}` で同 photobook 内の他 image を辿れない
- photobook 削除時の GC: prefix `photobooks/{photobook_id}/` を一括削除（R2 ListObjects）
- variant 管理: `{image_id}/{variant}/` で各派生を分離
- public/private 切替: bucket public access は OFF、配信は CDN + 署名 URL（後続）
- R2 list 性能: 第 1 階層を photobook_id にすることで 1 photobook 内 list が高速

PR18 では `StorageKey` VO のコンストラクタと、`Of(photobookID, imageID, kind, ext)`
ヘルパを実装する。

---

## 6. R2 方針（PR18 では実装しない、PR21 入口として整理）

| 項目 | 方針 | 確定タイミング |
|---|---|---|
| bucket 名候補 | `vrcpb-images`（命名は M1 spike `vrcpb-spike-images` と区別） | PR21 着手時 |
| public access | 原則 OFF。配信は署名 URL or Workers 経由 | 確定済（ADR-0005） |
| presigned URL | PUT 用 15 分有効（ADR-0005 §presigned URL） | 確定済 |
| HEAD 検証 | complete 時に必須 | 確定済 |
| Secret 管理 | Secret Manager に R2 endpoint / access key / secret key を分離保管 | PR21 |
| spike 流用 | 流用しない（spike R2 は M1 検証専用、本実装は新規 bucket） | 確定済 |
| lifecycle | bucket 側 lifecycle は MVP では設定しない（孤児削除は app 側 Reconcile） | 確定済 |

PR21 で確認する項目:

- M1 spike R2 (`vrcpb-spike-images`) の保持/削除判定
- Wrangler OAuth は R2 を含まない（memory 参照）→ API token もしくは S3 API credentials が必要
- Cloudflare R2 console で bucket / token を作成（**ユーザー手動**）

---

## 7. Turnstile 方針（PR20 入口として整理）

ADR-0005 §Turnstile で確定済。PR18 では実装しない。

- `upload_verification_sessions` テーブルを **PR20 で追加**（PR18 では作らない）
- 1 検証あたり 20 intent / 30 分有効 / 対象 photobook_id に紐付け / atomic consume
- session token は Cookie/header に載せ、DB には SHA-256 hash のみ保存
- Turnstile secret は Secret Manager
- test sitekey と本番 sitekey を分離（`harness/spike/turnstile/` 検証で使用したテスト
  sitekey 経験を引き継ぐ）
- Safari / iPhone Safari の widget 表示は PR20 で実機確認（safari-verification ルール）

---

## 8. Presigned URL flow（PR21 入口として整理）

ADR-0005 §基本フローで確定済。PR21 で実装する流れの要点:

1. Client: draft session Cookie 付きで `POST /api/photobooks/{id}/images/upload-intent`
2. Backend: draft session 認可
3. Backend: upload-verification session 検証（Turnstile session、PR20 で実装）
4. Backend: rate limit / 枚数上限 / 拡張子 / 申告 content-type の軽量検証
5. Backend: `images` row 作成（`status=uploading`、`owner_photobook_id` 固定）
6. Backend: presigned PUT URL 発行（15 分有効）+ 期待 storage_key を返す
7. Client: R2 へ直接 PUT
8. Client: `POST /api/images/{id}/complete`
9. Backend: HeadObject で R2 存在確認、storage_key 一致、size <= 10MB
10. Backend: `images.status` を `processing` に遷移、`outbox_events` に
    `ImageIngestionRequested` を INSERT（同 TX）
11. image-processor（PR23）が非同期で本検証 → `available` / `failed`
12. Client: Photobook の page / photo に attach（PR19 で実装）

考慮点（PR21 / PR23 で詰める）:

- duplicate complete: `images.status` が `uploading` でなければ冪等に return
- failed upload cleanup: 一定時間 `uploading` のままの行は Reconcile で `failed` 化
- orphan image GC: Photobook が削除されたが Image が残っている場合は Reconcile で削除
- retry: client 側 retry は別 image_id で（同じ image_id への再 PUT は不可）

---

## 9. EXIF / privacy 方針（PR23 image-processor で本実装）

業務知識 v4 §3.10 / ADR-0005 §image-processor で確定済。PR18 では DB 列のみ用意:

- `images.metadata_stripped_at` を持ち、`status=available` で必須
- 原本（`original` variant）は **MVP では保持しない**（v4 U9）
- 公開表示用 variant（`display` / `thumbnail` / `ogp`）は EXIF / XMP / IPTC を全除去
- GPS / シリアル / PC ユーザー名 / 撮影日時を含むメタは原則全除去
- HEIC は内部で JPG / WebP に変換（HEIC のまま variant に保持しない）
- ユーザーへの明示は MVP では「アップロード時に EXIF を除去します」のヘルプテキスト
  程度で十分（v4 §3.10）

PR18 では metadata 除去ロジックは書かない。DB 制約と domain 不変条件のみ。

---

## 10. Moderation 境界（PR24 で本実装）

業務知識 v4 §6.14 / image データモデル §3.2 で確定済。PR18 では:

- `images.status` の `deleted` / `purged` 状態を CHECK に含める
- `hidden_by_operator` 相当の Moderation 状態は **Image には持たない**
  （Photobook 側 `hidden_by_operator` で運用、画像個別の hidden は MVP では持たない）
- Report aggregate との接続は PR24
- ops CLI からの Image 削除は PR24（v4 §6.20 ops execution model）
- R2 object 削除は **遅延**（DB を `deleted` → Reconcile 経由で `purged`、その後 R2 削除）

---

## 11. Outbox 連携（PR23 で本実装）

`docs/design/cross-cutting/outbox.md` 参照。PR18 では実装しない:

- `ImageIngestionRequested` — complete 時に発火
- `ImageBecameAvailable` — image-processor 完了時
- `ImageFailed` — image-processor 失敗時
- `PhotobookPurged` — Photobook 削除時に Image purge を起動（PR24）

PR18 で出てくる Image domain メソッドは **Outbox イベントを返さない**。
PR23 で `events.go` 等を追加する。

---

## 12. Backend API（PR18 では公開 API なし）

PR18 で **公開 API は追加しない**。Repository / sqlc / domain / unit test のみ。

将来 API 案（参考、PR21 以降で追加）:

```
POST   /api/photobooks/{id}/images/upload-intent       PR21
POST   /api/images/{id}/complete                       PR21
DELETE /api/images/{id}                                PR24
POST   /api/photobooks/{id}/pages                      PR19
POST   /api/photobooks/{id}/photos                     PR19
PATCH  /api/photobooks/{id}/photos/{photoId}           PR19
DELETE /api/photobooks/{id}/photos/{photoId}           PR19
```

PR18 完了時点で `cmd/api/main.go` の handler 追加は無し。`/readyz` 200 は維持。

---

## 13. Frontend 連携方針（PR22 で本実装）

PR18 では Frontend に変更を加えない。PR22 着手時に整理:

- `/edit/<id>` ページの photo upload UI（design/mockups/prototype/screens-a.jsx の Edit）
- upload button / drag-and-drop / progress / error display / retry
- iPhone Safari の photo picker（`<input type="file" accept="image/*">` の `capture`/`multiple` 挙動差）
- HEIC 対応: client 圧縮はしない（server 側で HEIC→JPG/WebP 変換、ADR-0005）
- 画像圧縮 client 側で行うかは PR22 で判断（10MB 上限の手前で server reject させる方針が単純）
- Edit 画面の design tokens（`--teal #14B8A6`、`--radius` 12/16px、`.t-h1` 26px/800）
  は design/mockups/prototype/styles.css と pc-styles.css を参照
- CORS middleware は PR21 で R2 へ直接 PUT する Client のため `app.vrc-photobook.com`
  origin → R2 endpoint への CORS 設定を Cloudflare R2 bucket 側で許可

---

## 14. テスト方針

### 14.1 PR18 で書くテスト

- VO 単体テスト（テーブル駆動 + Builder、`.agents/rules/testing.md`）:
  - `image_id` / `image_usage_kind` / `image_format` / `normalized_format`
  - `image_dimensions`（境界値: 1, 8192, 0, 8193, 40MP 上限/超過）
  - `byte_size`（境界値: 1, 10485760, 0, 10485761）
  - `mime_type` / `storage_key` / `variant_kind` / `image_status` / `failure_reason`
- domain メソッドテスト:
  - `Image.NewUploading(...)`（生成時の不変条件、`owner_photobook_id` 必須）
  - `Image.MarkProcessing()` / `MarkAvailable(...)` / `MarkFailed(reason)` / `MarkDeleted(now)`
  - 状態遷移ガード（available → uploading は不可、failed → available 不可など）
  - `Image.AttachVariant(kind, ...)` / `RemoveVariant(kind)`（`(image_id, kind)` UNIQUE 整合）
  - `Image.CanAttachToPage()`（uploading / processing / failed は false）
- Repository テスト（実 DB / Cloud SQL Auth Proxy 経由、photobook テストと同じ pattern）:
  - `CreateUploading` → `FindByID` → `MarkProcessing` → `MarkAvailable`
  - 失敗パス: `MarkFailed` で `failure_reason` 必須が CHECK で効く
  - `owner_photobook_id` の `ON DELETE RESTRICT` が効く（photobook 削除を試みて失敗）
  - `image_variants` の `(image_id, kind)` UNIQUE 違反

### 14.2 PR18 で書かないテスト

- R2 fake / interface mock（PR21）
- Turnstile fake（PR20）
- presigned URL integration（PR21）
- Browser upload E2E（PR22）
- image-processor 全フロー（PR23）

---

## 15. セキュリティ確認（PR18 で domain / DB に効かせる）

- content-type validation: `mime_type` CHECK で `image/jpeg|png|webp` のみ
- file size limit: `original_byte_size <= 10485760` CHECK
- 拡張子 trust 禁止: PR21 で magic number 検証（image-processor）まで信用しない
- SVG 許可しない: `source_format` / `mime_type` の許可リストに含めない
- HTML upload 禁止: 同上
- malware scan: MVP ではやらない（v4 / ADR-0005 範囲外）
- raw object public access 禁止: bucket public access OFF（PR21）
- presigned URL expiration: 15 分（PR21）
- upload intent rate limit: PR20 + Turnstile session 20/30min
- CSRF: SameSite=Strict Cookie + Origin / Referer 検証（既存 photobook session と同方式）
- CORS: `app.vrc-photobook.com` → R2 endpoint のみ許可（PR21）
- logs に出さない: `storage_key` / presigned URL / 申告 filename / Cookie 値
  （`.agents/rules/security-guard.md`）
- Secret 管理: R2 / Turnstile credentials は Secret Manager のみ、ハードコード禁止

PR18 で specifically 効かせるもの:

- migration の CHECK 制約で形式 / サイズ / status / failure_reason を縛る
- Repository のクエリで `deleted_at IS NULL` をデフォルト条件にする
- domain で raw EXIF / filename / storage_key を返さない（VO で型を縛る）

---

## 16. 費用・運用

| 項目 | 想定 | 備考 |
|---|---|---|
| R2 storage | 0 → 数 GB / 月 | MVP は数十 photobook × 数十枚 × 数 MB の規模 |
| R2 Class A operations (PUT/HEAD) | 月数千〜数万 | 無料枠 100 万/月内 |
| R2 Class B operations (GET) | 月数千〜数万 | 無料枠 1000 万/月内 |
| Cloud SQL storage | +10〜100 MB | images / image_variants は数百行規模 |
| Workers requests | +数千 | 編集 UI のアクセス分 |
| Cloud Run invocations | +数千 | upload-intent / complete 分 |
| orphan object cleanup | Reconcile で月次 | image-processor 失敗 / Photobook 削除 |
| 検証 Cloud SQL `vrcpb-api-verify` 残置 | ¥55/日 | PR18 計画書完了時 or 2 日後で再判断 |
| 画像投入後の削除重要性 | 高 | 初の「ユーザー由来コンテンツ」のため、Reconcile / DB 削除 / R2 削除すべて整合させる必要がある |

---

## 17. PR18 の実装範囲（明確化）

### PR18 で実装する

- domain model:
  - `backend/internal/image/domain/image.go`（Image entity）
  - `backend/internal/image/domain/image_variant.go`（ImageVariant entity）
  - VO 群: `backend/internal/image/domain/vo/{image_id, image_usage_kind, image_format, ...}/`
  - tests/Builder（`.agents/rules/testing.md` 準拠）
- migration:
  - `backend/migrations/00005_create_images.sql`
  - `backend/migrations/00006_create_image_variants.sql`
- sqlc:
  - `backend/sqlc.yaml` に image set を追加
  - `backend/internal/image/infrastructure/repository/rdb/queries/*.sql`
  - sqlc generate で `sqlcgen/` を生成
- Repository:
  - `backend/internal/image/infrastructure/repository/rdb/image_repository.go`
  - `marshaller/`（domain ↔ DB 変換）
  - `tests/`（builder）
- 単体テスト + Repository テスト（実 DB）
- ports.go（PhotobookId 参照、後続 PR で UseCase 経由）
- README/計画更新

### PR18 で実装しない

- R2 接続 / 実 PUT / 実 HEAD
- Turnstile 検証
- presigned URL 発行
- Frontend upload UI / drag-and-drop / progress
- CORS middleware
- 実画像アップロード
- image-processor（EXIF 除去 / variant 生成 / HEIC 変換）
- Moderation UI / Report 連携
- Outbox events
- SendGrid 関連
- Cloudflare Dashboard 操作（R2 bucket / token / CORS 設定）
- Cloud SQL 削除
- 既存 spike リソース削除
- 公開 API endpoint 追加

---

## 18. ユーザー判断事項（PR18 着手前に確認）

| # | 判断対象 | 推奨案 | 代替案 | 影響 |
|---|---|---|---|---|
| Q1 | `images.owner_photobook_id` FK のタイミング | **案 A（PR18 で `ON DELETE RESTRICT` 追加）** | 案 B（PR19 で追加） | 案 A は image データモデル §3.2 と完全一致 |
| Q2 | `cover_image_id` FK 追加 PR | **PR19**（pages / photos 追加と同時） | PR18 で同時追加 | PR19 推奨で OK か |
| Q3 | StorageKey 形式 | **ADR-0005 §storage_key 通り**（`photobooks/{pid}/images/{iid}/{kind}/{random}.{ext}`） | 別案 | 推奨に従って良いか |
| Q4 | allowed content-types | `image/jpeg` / `image/png` / `image/webp`（受付）+ `image/heic`（受付のみ、variant に出ない）| 拡張 | v4 / ADR-0005 通り |
| Q5 | max file size | **10MB（10485760 byte）** | 増減 | v4 §3.10 通り |
| Q6 | 原本（`original` variant）保持 | **保持しない**（v4 U9） | 保持 | 推奨に従って良いか |
| Q7 | EXIF 除去タイミング | **PR23 image-processor で実施**（PR18 ではカラムだけ）| PR21 の complete 時に同期実施 | 推奨に従って良いか |
| Q8 | R2 bucket 名 | `vrcpb-images`（spike `vrcpb-spike-images` と区別）| 別名 | PR21 で確定 |
| Q9 | PR18 を domain + DB のみに絞る | **絞る**（推奨） | sqlc + Repository まで含めて少し広げる | 本書 §17 の通り |
| Q10 | Cloud SQL `vrcpb-api-verify` を本書作成中も残すか | **残す**（PR18 着手も連続予定）| 一時削除 | PR17 完了直後の判断（worklog 追記済）と整合 |
| Q11 | テスト用 photobook の作り方 | **既存 photobook builder の流用**（`backend/internal/photobook/domain/tests/`） | image 専用 fixture を新規 | builder 流用が `.agents/rules/testing.md` に整合 |
| Q12 | v4 の `image_purpose` 等の語彙 | **`usage_kind`（image データモデル設計通り）** | `purpose` | 設計優先 |

Q1〜Q12 への回答後、PR18 実装計画書（実装手順書）を別ドキュメントとして起こすか、
本書をベースに直接実装に入るかを判断する。

---

## 19. 関連

- [Image ドメイン設計](../design/aggregates/image/ドメイン設計.md)
- [Image データモデル設計](../design/aggregates/image/データモデル設計.md)
- [ADR-0005 画像アップロード方式](../adr/0005-image-upload-flow.md)
- [Photobook データモデル設計](../design/aggregates/photobook/データモデル設計.md)
- [Outbox 設計](../design/cross-cutting/outbox.md)
- [Reconcile スクリプト設計](../design/cross-cutting/reconcile-scripts.md)
- [Post-deploy Final Roadmap](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)
- [Photobook + Session 接続実装計画](./m2-photobook-session-integration-plan.md)（PR 分割テンプレ）
- [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md)
- [`.agents/rules/testing.md`](../../.agents/rules/testing.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
- [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
