# M2 Photobook ↔ Image 連携 実装計画（PR19 候補）

> 作成日: 2026-04-27
> 位置付け: PR18（Image aggregate domain + DB）完了後、Photobook と Image を
> page / photo 構造で接続するフェーズの入口。実装コードはまだ書かない。
>
> 上流参照（必読）:
> - [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §2.1 / §2.6 / §3.1 / §3.2
> - [`docs/design/aggregates/photobook/ドメイン設計.md`](../design/aggregates/photobook/ドメイン設計.md) §3〜§5
> - [`docs/design/aggregates/photobook/データモデル設計.md`](../design/aggregates/photobook/データモデル設計.md) §3〜§7
> - [`docs/design/aggregates/image/ドメイン設計.md`](../design/aggregates/image/ドメイン設計.md)
> - [`docs/design/aggregates/image/データモデル設計.md`](../design/aggregates/image/データモデル設計.md)
> - [`docs/adr/0005-image-upload-flow.md`](../adr/0005-image-upload-flow.md)
> - [`docs/plan/m2-image-upload-plan.md`](./m2-image-upload-plan.md)
> - [`docs/plan/m2-photobook-session-integration-plan.md`](./m2-photobook-session-integration-plan.md)
> - [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md)
> - [`.agents/rules/testing.md`](../../.agents/rules/testing.md)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)

---

## 0. 本計画書の使い方

- 設計の正典は `docs/design/aggregates/photobook/` と `docs/design/aggregates/image/`。
  本書はそれを **どう PR19 に切り出すか** を整理する。設計と差異が出た場合は設計が優先。
- **PR19 の実装範囲は migration + domain + sqlc + Repository + UseCase + test**。
  Public API / Frontend upload UI / CORS / R2 / Turnstile / image-processor / Outbox
  は実装しない（§13）。
- §15 のユーザー判断事項に答えてもらってから PR19 実装に着手する。

---

## 1. 目的

- Photobook にページ構造（page / photo）を接続する。
- Photo は available な Image を参照する（domain + DB FK の二重で担保）。
- Photobook の `cover_image_id` を images(id) FK へ昇格する。
- Photobook 集約として PR9 で枠だけ作っていた `Page[]` / `Photo[]` を
  業務ロジック付きで動かせる状態にする。
- 楽観ロック（version、I-V1〜I-V3）を page / photo 操作にも適用する。

---

## 2. PR19 の対象範囲

### 対象（PR19 で実装する）

- migration 4 本（§3.1）
- Photobook aggregate に Page / Photo / Cover の振る舞いを追加（§6）
- Page entity / Photo entity を子エンティティとして実装
- 値オブジェクト追加: `display_order`（int 型 VO）、`caption`（length 0..200）
- sqlc set 追加（photobook 用 set に photobook_pages / photobook_photos /
  photobook_page_metas 系 query を追加、もしくは独立 set として追加）
- Repository 拡張: `PhotobookRepository` に page/photo 操作メソッドを追加するか、
  PageRepository / PhotoRepository に分離するかは §8 で判断
- UseCase: `AddPage` / `RemovePage` / `AddPhoto` / `RemovePhoto` / `ReorderPhoto` /
  `SetCoverImage` / `ClearCoverImage`
- 同一 TX 内で「Image available 検証 → photobook_photos INSERT → photobook version +1」
  を実行する
- unit test + repository test + UseCase test（実 DB 必須）

### 対象外（PR19 では実装しない）

- R2 接続 / bucket 操作 / presigned URL / complete-upload
- Turnstile 検証 / `upload_verification_sessions`
- Frontend upload UI / drag-and-drop / progress / CORS middleware
- image-processor（EXIF 除去 / variant 生成 / HEIC 変換）
- Outbox events（`PhotoAdded` / `PhotobookEdited` 等）
- public upload / edit API endpoint の追加
- Safari / iPhone Safari 再検証
- Cloud SQL / spike / Cloudflare Dashboard 操作
- Cloud Run revision 更新

---

## 3. DB 設計案

### 3.1 追加 migration（命名は設計と整合）

設計の正典は `docs/design/aggregates/photobook/データモデル設計.md` §4〜§7。
**テーブル名は `photobook_pages` / `photobook_photos` / `photobook_page_metas`**
（指示書の `pages` / `photos` / `page_metas` ではなく、設計通りの命名を採用）。

| 番号 | ファイル | 内容 |
|---|---|---|
| 00007 | `00007_create_photobook_pages.sql` | `photobook_pages` テーブル |
| 00008 | `00008_create_photobook_photos.sql` | `photobook_photos` テーブル |
| 00009 | `00009_create_photobook_page_metas.sql` | `photobook_page_metas` テーブル |
| 00010 | `00010_add_photobooks_cover_image_fk.sql` | `photobooks.cover_image_id` → `images(id)` FK |

00007〜00009 は依存があるため、**この順で適用**:

1. `photobook_pages`（photobooks への FK）
2. `photobook_photos`（photobook_pages + images への FK）
3. `photobook_page_metas`（photobook_pages への FK）
4. `photobooks.cover_image_id` FK（images への FK 追加）

### 3.2 `photobook_pages`（設計 §4 と完全一致）

```sql
id              uuid        NOT NULL PRIMARY KEY  -- gen_random_uuid()
photobook_id    uuid        NOT NULL              -- FK photobooks(id) ON DELETE CASCADE
display_order   int         NOT NULL              -- 0始まり連番（DB は uniqueness のみ保証）
caption         text        NULL                  -- 0..200
created_at      timestamptz NOT NULL DEFAULT now()
updated_at      timestamptz NOT NULL DEFAULT now()

UNIQUE (photobook_id, display_order)
INDEX  (photobook_id)
CHECK  (caption IS NULL OR char_length(caption) BETWEEN 0 AND 200)
CHECK  (display_order >= 0)
```

設計上の不変条件:
- I1: photobook には最低 1 ページ（DB 制約化はしない、アプリ層で保証）
- I3: display_order は連番 0, 1, 2, ...（DB は uniqueness のみ、連続性はアプリ）

PR19 では `deleted_at` を持たない（**設計に存在しないため追加しない**）。
削除は CASCADE で物理削除 or アプリ層の soft delete（`photobooks.status='deleted'`
等）で表現する。指示書の `deleted_at` カラムは設計に無いため不採用（§15 Q1 で確認）。

### 3.3 `photobook_photos`（設計 §6 と完全一致）

```sql
id              uuid        NOT NULL PRIMARY KEY
page_id         uuid        NOT NULL              -- FK photobook_pages(id) ON DELETE CASCADE
image_id        uuid        NOT NULL              -- FK images(id) ON DELETE RESTRICT
display_order   int         NOT NULL              -- Page 内 0始まり連番
caption         text        NULL                  -- 0..200
created_at      timestamptz NOT NULL DEFAULT now()

UNIQUE (page_id, display_order)
INDEX  (image_id)
CHECK  (caption IS NULL OR char_length(caption) BETWEEN 0 AND 200)
CHECK  (display_order >= 0)
```

`image_id` ON DELETE RESTRICT の理由（設計 §6 §重要な設計判断）:
- Image 集約は `owner_photobook_id` で所有関係を管理
- photobook_photos からの参照が先に消える可能性があるため、誤って参照中の Image だけ
  先に削除されないよう RESTRICT
- Image 集約側の明示削除フローを通すことを強制

### 3.4 `photobook_page_metas`（設計 §5 と完全一致）

```sql
page_id       uuid        NOT NULL PRIMARY KEY -- FK photobook_pages(id) ON DELETE CASCADE
world         text        NULL
cast          text[]      NULL                 -- PostgreSQL 配列
photographer  text        NULL
note          text        NULL                 -- v2 までの comment、v3 で改名
event_date    date        NULL
created_at    timestamptz NOT NULL DEFAULT now()
updated_at    timestamptz NOT NULL DEFAULT now()
```

- v4 §2.7 非採用用語: `comment` カラムは禁止
- domain VO は string 上限のみ（業務 lint で長さは別途）

### 3.5 `photobooks.cover_image_id` FK 追加

```sql
ALTER TABLE photobooks
    ADD CONSTRAINT photobooks_cover_image_id_fkey
    FOREIGN KEY (cover_image_id)
    REFERENCES images (id)
    ON DELETE SET NULL;
```

ON DELETE SET NULL の理由（設計 §3.2 / 業務知識 v4）:
- Image 集約側で孤児 GC を担当
- Image が削除された時に Photobook 全体を巻き込まない
- 通常運用では `owner_photobook_id` 縛りで Photobook 物理削除時に連鎖削除されるため、
  SET NULL に到達するのは異常系のみ

なお、既存 `backend/internal/photobook/domain/photobook.go` の
`coverImageID *photobook_id.PhotobookID // 仮: 本来は ImageID。PR11 で置換` を
**PR19 で `*image_id.ImageID` に置換する**。

---

## 4. Owner 整合（最重要）

### 4.1 ルール

`photobook_photos.image_id` が指す Image は、
**`images.owner_photobook_id == photobook_photos.page_id → photobook_pages.photobook_id`**
を満たす必要がある。

つまり「あるページに写真として置ける Image は、そのページが属する Photobook が所有する
Image だけ」。

### 4.2 担保方法

DB CHECK / FK では複数テーブルにまたがる整合は表現できない。選択肢:

| 案 | 仕組み | 利点 | 欠点 |
|---|---|---|---|
| **案 A（推奨）** | UseCase / Repository で検証（INSERT 前に Image を SELECT して owner 一致を確認） | シンプル、既存の photobook UseCase と同パターン | 検査と INSERT の間に競合（極稀） |
| 案 B | DB trigger | 強制的に整合 | 複雑化、テスト容易性が落ちる、debug 難しい |
| 案 C | photobook_photos に photobook_id 列を冗長化 + UNIQUE/CHECK | 1 レコード内で完結 | スキーマ複雑化、設計と乖離 |

→ **案 A 推奨**。`AddPhoto` UseCase 内で同一 TX 内に
`SELECT images WHERE id = $1 FOR UPDATE` → owner / status 検証 → INSERT。

検査と INSERT の競合は、Image 状態が `available` から外れる遷移（MarkDeleted 等）が
別 TX で起きた場合のみ発生。`FOR UPDATE` の row lock で十分防げる。

### 4.3 Repository test での確認項目

- 自 photobook 所有の available Image を attach できる
- **別 photobook 所有の available Image を attach すると拒否される**
- uploading / processing / failed / deleted Image を attach すると拒否される

---

## 5. Image 状態整合

### 5.1 ルール

attach できる Image:
- `image.status == 'available'`
- `image.deleted_at IS NULL`
- `image.owner_photobook_id == 対象 Photobook の ID`

uploading / processing / failed / deleted / purged は不可。

### 5.2 担保方法

3 段階:

1. **domain 層**: `Image.CanAttachToPhotobook()` が `IsAvailable() && deleted_at == nil`
   を判定（PR18 で実装済）。同一 photobook 縛りはここでは判定しない（呼び出し側が
   画像 ID と photobook ID をペアで渡す）。
2. **Repository 層**: `AddPhoto` の SQL に明示的な状態条件を入れる:
   ```sql
   INSERT INTO photobook_photos ...
   FROM images
   WHERE images.id = $image_id
     AND images.owner_photobook_id = $photobook_id
     AND images.status = 'available'
     AND images.deleted_at IS NULL
   ```
   行が無ければ INSERT が起きない → 0 行影響を ErrConflict として返す。
3. **UseCase 層**: 上の 0 行影響を業務エラーに変換。失敗時は明示的な
   `ErrImageNotAttachable` を返す（domain で `ErrAttachOnNotAvailable` を再利用）。

### 5.3 Cover の場合

`SetCoverImage` も同じ条件で検証する。`photobooks.cover_image_id` 更新時:

```sql
UPDATE photobooks
   SET cover_image_id = $image_id, updated_at = $now, version = version + 1
 WHERE id = $photobook_id
   AND version = $expected_version
   AND status = 'draft'  -- published 状態の cover 変更は別 PR で
   AND EXISTS (SELECT 1 FROM images
               WHERE id = $image_id
                 AND owner_photobook_id = $photobook_id
                 AND status = 'available'
                 AND deleted_at IS NULL);
```

---

## 6. Photobook aggregate 変更案

### 6.1 追加する domain メソッド

| メソッド | 引数 | 戻り値 | 状態前提 |
|---|---|---|---|
| `AddPage` | (caption, displayOrder, now, expectedVersion) | (Photobook, error) | status=draft |
| `RemovePage` | (pageID, now, expectedVersion) | (Photobook, error) | status=draft |
| `AddPhoto` | (pageID, image (Image VO), caption, displayOrder, now, expectedVersion) | (Photobook, error) | status=draft, image.CanAttachToPhotobook() |
| `RemovePhoto` | (photoID, now, expectedVersion) | (Photobook, error) | status=draft |
| `ReorderPhoto` | (photoID, newDisplayOrder, now, expectedVersion) | (Photobook, error) | status=draft |
| `SetCoverImage` | (image (Image VO), now, expectedVersion) | (Photobook, error) | status=draft, image.CanAttachToPhotobook(), image.OwnerPhotobookID == p.ID |
| `ClearCoverImage` | (now, expectedVersion) | (Photobook, error) | status=draft |

### 6.2 不変条件（追加）

- I1: page は最低 1 件（最後の page を `RemovePage` しようとすると拒否）
- I2: 各 page は photo を最低 1 枚（最後の photo を `RemovePhoto` で **page も同時削除**
  しないと拒否、または page 残存を許容して I2 を「published 時のみ」に弱めるかは §15 Q4）
- I3: display_order は集約内で連続（0,1,2,...）。RemovePage/RemovePhoto 時に詰める
- I4: cover の有無は `OpeningStyle` に整合（`cover_first_view` なら cover 必須）
- 楽観ロック I-V1〜I-V3: 操作毎に `expectedVersion` を取り、成功時に version+1

### 6.3 published / deleted 状態の扱い

PR19 では **draft 状態のみ編集可** とする。published 後の編集は MVP 範囲外で、
PR23 / PR24 以降の Moderation / Reissue サイクルで扱う（§15 Q2）。

published 直後の hot fix 用 API が必要かは MVP の現場運用判断（業務知識 v4 §3.2 では
publish 後の編集は不可、再 publish のみ）。

---

## 7. Version / 楽観ロック

### 7.1 設計の正典

業務知識 v4 §楽観ロック / 設計 §I-V1〜I-V3:
- すべての状態変更操作は `expectedVersion` を受け取る
- 不一致のとき `OptimisticLockConflict`
- 成功時に `version` を +1

### 7.2 page / photo 操作での適用

選択肢:

| 案 | photobooks.version 更新 | 利点 | 欠点 |
|---|---|---|---|
| **案 A（推奨）** | page / photo / cover 変更ごとに +1 | 全集約変更で一貫 / 並行編集を確実に検知 | UPDATE 文が複合的になる（photobooks UPDATE + photobook_photos INSERT を同 TX） |
| 案 B | page / photo は独自 version、photobooks.version は publish/reissue 時のみ | Photobook UPDATE が減る | 集約境界が崩れる、楽観ロックの設計と矛盾 |
| 案 C | photobooks.version は触らず、photobook_photos に独自 version | 同上の欠点 | 同上 |

→ **案 A 推奨**（設計の I-V1〜I-V3 を厳守）。

### 7.3 SQL パターン

例: `AddPhoto`

```sql
-- 同一 TX 内
-- 1. photobook の楽観ロック取得 + status 確認
UPDATE photobooks
   SET version = version + 1, updated_at = $now
 WHERE id = $photobook_id
   AND version = $expected_version
   AND status = 'draft';
-- 0 行 → ErrOptimisticLockConflict

-- 2. owner / status 検証
SELECT 1 FROM images
 WHERE id = $image_id AND owner_photobook_id = $photobook_id
   AND status = 'available' AND deleted_at IS NULL
 FOR UPDATE;
-- 0 行 → ErrImageNotAttachable

-- 3. INSERT
INSERT INTO photobook_photos (id, page_id, image_id, display_order, caption, created_at)
VALUES ($photo_id, $page_id, $image_id, $display_order, $caption, $now);
```

3 SQL 同 TX 内、いずれか失敗で全ロールバック。

---

## 8. sqlc / Repository 方針

### 8.1 sqlc set

選択肢:

| 案 | 構成 | 利点 | 欠点 |
|---|---|---|---|
| **案 A（推奨）** | 既存 photobook set に query を追加（`internal/photobook/.../queries/photobook.sql` に追記） | 1 set / 1 sqlcgen で集約一致 / Photobook entity と同パッケージ | sqlc.yaml に schema 追加（00007〜00010）が必要 |
| 案 B | 新 page / photo set を独立追加 | 関心分離 | sqlcgen が 2 重で marshaller が膨らむ |

→ **案 A 推奨**。集約ルートが Photobook である以上、sqlc set も 1 つに統合する。

`backend/sqlc.yaml` の photobook set の schema に追加:
```
- migrations/00001_create_health_check.sql
- migrations/00003_create_photobooks.sql
- migrations/00005_create_images.sql           ← 追加（FK 先として）
- migrations/00006_create_image_variants.sql   ← 追加（同上）
- migrations/00007_create_photobook_pages.sql       ← 追加
- migrations/00008_create_photobook_photos.sql      ← 追加
- migrations/00009_create_photobook_page_metas.sql  ← 追加
- migrations/00010_add_photobooks_cover_image_fk.sql ← 追加
```

ただし schema に `00005 / 00006` を入れると sqlcgen に images / image_variants の
Model 型が混入する可能性があるため、sqlc 側で `omit` 機能を使うか、または query は
photobooks / photobook_pages / photobook_photos / photobook_page_metas のみ書いて
images / image_variants は触らない方針で運用する（query を書かない table の Model は
出るが使わなければ問題ない）。

### 8.2 必要な query

photobook set に以下を追加:

- `CreatePage` / `UpdatePageCaption` / `DeletePage` / `ListPagesByPhotobookID` /
  `FindPageByID` / `ReorderPage`
- `CreatePhoto` / `UpdatePhotoCaption` / `DeletePhoto` /
  `ListPhotosByPageID` / `FindPhotoByID` / `ReorderPhoto`
- `UpsertPageMeta` / `FindPageMetaByPageID`
- `SetPhotobookCoverImage` / `ClearPhotobookCoverImage`
- `IncrementPhotobookVersion`（楽観ロック専用、PR9 既存の TouchDraft 等と整合）

INSERT 系は `:exec`、`UPDATE … WHERE version=$expected` は `:execrows`。

### 8.3 Repository 構造

選択肢:

| 案 | 構成 | 利点 | 欠点 |
|---|---|---|---|
| **案 A（推奨）** | `PhotobookRepository` に page/photo メソッドを追加 | 集約ルートに対する Repository が 1 つ / 既存 PR9 と一貫 | `PhotobookRepository` のメソッド数が増える |
| 案 B | `PageRepository` / `PhotoRepository` を分離 | 関心分離 | 集約ルートが分裂、TX 境界の管理が複雑化 |

→ **案 A 推奨**（domain-standard.md「集約ルートに対し 1 Repository」の原則）。

メソッド名（追加分）:

```go
PhotobookRepository
  // 既存
  CreateDraft / FindByID / FindByDraftEditTokenHash / ...

  // PR19 追加
  AddPage(ctx, photobookID, page, expectedVersion) error
  RemovePage(ctx, photobookID, pageID, expectedVersion) error
  AddPhoto(ctx, photobookID, pageID, photo, expectedVersion) error  // image 検証込み
  RemovePhoto(ctx, photobookID, photoID, expectedVersion) error
  ReorderPhoto(ctx, photobookID, photoID, newOrder, expectedVersion) error
  SetCoverImage(ctx, photobookID, imageID, expectedVersion) error
  ClearCoverImage(ctx, photobookID, expectedVersion) error
  UpsertPageMeta(ctx, pageID, meta) error
  FindByIDFull(ctx, photobookID) (Photobook, error)  // pages / photos / metas 込み
```

`FindByIDFull` は今までの `FindByID` の拡張。既存 `FindByID` も残してフォールバック。

---

## 9. Transaction 方針

### 9.1 同一 TX で完結すべき操作

| UseCase | 同 TX で実行する内容 |
|---|---|
| AddPhoto | photobooks.version+1 / images の owner/status 検証 (FOR UPDATE) / photobook_photos INSERT |
| RemovePhoto | photobooks.version+1 / photobook_photos DELETE / display_order 詰め直し |
| AddPage | photobooks.version+1 / photobook_pages INSERT |
| RemovePage | photobooks.version+1 / photobook_pages DELETE（CASCADE で photos も削除） / photobook_pages display_order 詰め直し |
| SetCoverImage | photobooks.version+1 + cover_image_id 更新 / images の owner/status 検証 |
| ReorderPhoto | photobooks.version+1 / photobook_photos display_order 更新（影響行を全て） |

`backend/internal/database.WithTx` を呼び出し側で使う（PR9 の publish_from_draft.go と
同パターン）。

### 9.2 削除方針

PR19 では Page / Photo の **soft delete を持たない**（設計に `deleted_at` 列が無いため）。
削除は CASCADE 物理削除で表現する:

- Photobook 削除（status='deleted' / 'purged'）→ CASCADE で pages / photos / metas
- Page 削除 → CASCADE で photos / page_metas
- Photo 削除 → DELETE (CASCADE 不要、子なし)

Image 集約側の `owner_photobook_id` ON DELETE RESTRICT があるため、photobook_photos に
参照が残る Image は Photobook が `purge` まで進まない限り削除されない（多層防御）。

### 9.3 Hard delete / purge は PR24 以降

- `photobooks.status='purged'` 遷移
- 同 TX で photobook_photos / photobook_pages / photobook_page_metas が CASCADE 削除
- Image 集約側を `deleted` → `purged` に明示遷移（owner_photobook_id ON DELETE RESTRICT
  を解除する手前で）
- R2 object 削除は image-processor が非同期で実施

---

## 10. API 方針

### 10.1 推奨: PR19 では public API 追加なし

PR19 は **domain + DB + Repository + UseCase + test まで**。public API endpoint 追加は
**しない**。

理由:
- PR22 で編集 UI と同時に API を起こす方が API 形状を UI と整合させやすい
- 先行で API を作ると Frontend と JSON 形状の食い違いが出やすい
- PR21 (R2 + presigned URL) と PR19 を完全に分離しておくほうが debug 容易

### 10.2 内部から呼べる UseCase は揃える

PR19 完了時点で以下の UseCase は揃っているが、HTTP handler は無い:

```
photobook/internal/usecase/
  add_page.go
  remove_page.go
  add_photo.go
  remove_photo.go
  reorder_photo.go
  set_cover_image.go
  clear_cover_image.go
  upsert_page_meta.go
```

PR22 着手時に `interface/http/handler.go` を増やしてこれらを呼ぶ。

---

## 11. テスト方針

### 11.1 PR19 で書くテスト

**migration**:
- `goose up` / `down` ラウンドトリップ確認

**sqlc**:
- `sqlc generate` で images / image_variants の Model 重複が出ないことを確認
- 出る場合は schema 列挙の調整 or sqlc.yaml の omit

**domain unit test**:
- Page / Photo VO テスト（display_order / caption の境界値）
- `AddPage` / `RemovePage` / `AddPhoto` 等の状態遷移
- I1 / I2 / I3 の不変条件
- 楽観ロック（expectedVersion 不一致でエラー）

**repository test（実 DB）**:
- AddPage で photobook_pages INSERT、ListPages で取得
- AddPhoto で available image を attach できる
- AddPhoto で **uploading / processing / failed の image を attach すると拒否**
- AddPhoto で **別 photobook 所有の image を attach すると拒否**
- AddPhoto で **deleted_at が立った image を attach すると拒否**
- SetCoverImage で同様の検証
- RemovePage で CASCADE が photos / metas を削除する
- ReorderPhoto で UNIQUE 違反が起きないこと

**UseCase test（実 DB、TX 検証）**:
- AddPhoto 成功時に photobook.version が +1
- AddPhoto 失敗時に photobook.version が変わらない（rollback）
- 並行 AddPhoto で expectedVersion 不一致のうち片方が ErrOptimisticLockConflict
- SetCoverImage と AddPhoto を同 photobook に並行で出して、楽観ロックが効くこと

### 11.2 PR19 で書かないテスト

- API integration（HTTP handler test）
- Frontend E2E
- R2 / Turnstile / presigned URL
- Browser upload
- Safari 実機（PR22 の UI 変更時に再開）

---

## 12. セキュリティ / プライバシー

- `caption` / `note` / `world` 等は length CHECK で制限（DB 側）。XSS escape は
  Frontend 側の責務（PR22 で React の標準 escape を使う）
- `caption` / `note` 等の文字種制限はしない（VRChat 名や記号を許可）が、制御文字
  （\x00 等）は domain VO 側で reject する
- raw token / Cookie / Secret / DATABASE_URL の取り扱いは既存ルール通り（出さない）
- `storage_key` は引き続きログに出さない（PR18 と同じ）
- deleted_at 済み Image は通常クエリから除外（PR18 で実装済）
- photobook_photos の image_id 経由で別 photobook の Image が漏れないよう、
  domain / repository / usecase 三段で owner 整合を担保

---

## 13. PR19 実装範囲（明確化）

### PR19 で実装する

- migration: 00007〜00010
- domain: Page / Photo / Cover の振る舞い、楽観ロック適用、不変条件
- VO: display_order / caption / page_meta 関連
- sqlc: photobook set への query 追加 + generate
- Repository: PhotobookRepository への page/photo メソッド追加 + marshaller 拡張
- UseCase: AddPage / RemovePage / AddPhoto / RemovePhoto / ReorderPhoto /
  SetCoverImage / ClearCoverImage / UpsertPageMeta
- tests: domain / repository / UseCase（実 DB 込み）
- 既存 photobook entity の `coverImageID *photobook_id.PhotobookID // 仮` を
  `*image_id.ImageID` に置換

### PR19 で実装しない

- public API endpoint
- Frontend / CORS
- R2 / Turnstile / presigned URL / upload-intent / complete-upload
- image-processor / EXIF / variant 生成
- Outbox events
- SendGrid
- Cloudflare Dashboard 操作
- Cloud SQL 削除
- 既存 spike リソース削除
- Safari 再検証（UI 変更が無いため不要、`.agents/rules/safari-verification.md`
  対象外）

---

## 14. Cloud SQL 残置/一時削除判断

### 14.1 PR19 計画書完了時点での判断材料

- **PR19 実装にすぐ進むなら残置**（migration / repository test / UseCase test を
  連続で実行するため、DB が生きている方が手戻りが少ない）
- **数日空くなら一時削除**
- 費用目安: 残置 ¥55/日。30 日放置で ¥1,650
- 再作成手順: `gcloud sql instances create` 〜 migration up 〜 Secret 更新 〜
  Cloud Run revision 切替で約 10 分

### 14.2 推奨

PR18 完了直後と同様、**PR19 実装に連続して進む予定なら残置継続**。
次回判断タイミングは「PR19 実装 PR の完了時 or 2 日後」。

---

## 15. ユーザー判断事項（PR19 着手前に確認）

| # | 判断対象 | 推奨案 | 代替案 | 影響 |
|---|---|---|---|---|
| Q1 | テーブル名 | **設計通り `photobook_pages` / `photobook_photos` / `photobook_page_metas`** | 指示書の `pages` / `photos` / `page_metas` | 推奨に従って良いか（設計の正典との整合） |
| Q2 | `photobook_pages` / `photobook_photos` の `deleted_at` カラム | **持たない**（CASCADE 物理削除、設計通り） | 持つ（指示書通り、ただし設計外） | 推奨に従って良いか |
| Q3 | column 名 | **`display_order`**（設計通り） | `page_no` / `sort_order`（指示書） | 推奨に従って良いか |
| Q4 | I2（page は最低 1 photo）の運用 | **draft 中は弱める**（最後の photo を消すと page だけ残る）。published 時のみ強制 | 常に強制（最後の photo 削除を禁止） | UX とのトレードオフ |
| Q5 | published 後の編集 | **不可**（業務知識 v4 §3.2、再 publish のみ） | 限定的に可（hot fix） | 設計通りなら不可 |
| Q6 | photobook page 上限 | 設計に明示なし。MVP では **30 page** を仮置き | 別の数字 | 不変条件として実装する |
| Q7 | 1 page あたり photo 上限 | MVP では **20 枚** を仮置き | 別の数字 | UI / UsageLimit と整合させる |
| Q8 | layout_type の初期値 | 既存 photobook entity の `layout` を使う（page 個別の layout は持たない） | page meta に layout 列を追加 | 設計通りで良いか |
| Q9 | caption / note 長さ | **0..200**（設計通り） | 別の上限 | 設計通り |
| Q10 | Photobook version+1 のタイミング | **すべての page/photo 操作で +1**（案 A） | publish 時のみ | 設計の I-V1〜I-V3 通り |
| Q11 | DB trigger | **使わない**（UseCase / Repository で担保、案 A） | trigger 採用 | 推奨通り |
| Q12 | cover_image_id FK の ON DELETE | **SET NULL**（設計通り） | RESTRICT / CASCADE | 設計通り |
| Q13 | sqlc set | **既存 photobook set に統合**（案 A） | 新 set を独立 | 集約ルート 1 個に対応 |
| Q14 | API 追加 | **なし**（PR22 で UI と同時） | PR19 で内部 API を出す | 推奨通り |
| Q15 | Cloud SQL 残置 | **残置継続**（PR19 連続着手、~¥55/日） | 一時削除 | PR18 判断と整合 |

Q1〜Q15 への回答後、PR19 実装に進む。

---

## 16. 関連

- [Image 集約 計画](./m2-image-upload-plan.md)
- [Photobook ドメイン設計](../design/aggregates/photobook/ドメイン設計.md)
- [Photobook データモデル設計](../design/aggregates/photobook/データモデル設計.md)
- [Image ドメイン設計](../design/aggregates/image/ドメイン設計.md)
- [Image データモデル設計](../design/aggregates/image/データモデル設計.md)
- [Photobook + Session 接続実装計画](./m2-photobook-session-integration-plan.md)
- [Post-deploy Final Roadmap](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)
- [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md)
- [`.agents/rules/testing.md`](../../.agents/rules/testing.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
