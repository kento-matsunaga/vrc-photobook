# m2-edit /p 整合 + ページ分割 + プレビュー (STOP P-α 仕様確定)

## 0. メタ情報

| 項目 | 値 |
|---|---|
| 状態 | **STOP P-α 設計判断資料** (実装未着手) |
| 前提 commit | `8684a1d` (main、viewer v2 redesign deploy 済) |
| 前提 Workers version | `673a8e03-fcaf-4ffc-8c97-12149f52d4dd` |
| 関連既存 plan | `m2-frontend-edit-ui-fullspec-plan.md` (PR27) / `m2-photobook-image-connection-plan.md` (PR19) |
| Phase A scope | page caption 編集 / page split / merge / photo move-between-pages / page reorder / 同画面 preview |
| Phase B scope | page meta (event_date / world / cast_list / photographer / note) UI 追加、Backend は migration / domain 既存実装 |
| 着手予定 | STOP P-1 (Backend split / move / caption の SQL + Repository + UseCase + handler) |
| Q1〜Q8 確定 (本書承認時) | 後述 §1.2 |

## 1. 確定事項 (P-α 承認で固定)

### 1.1 scope

Phase A に含む:

- 各 page に caption 編集 UI (Backend mutation 追加、API response は既出)
- page を任意の photo の後で 2 つに分ける (split)
- 隣接する 2 page を結合する (merge)
- photo を別 page へ移動する (move-between-pages)
- page の並べ替え (reorder)
- /edit と同画面で「編集 ⇄ プレビュー」トグル、プレビューは v2 ViewerLayout を再利用

Phase A に含まない (Phase B 以降):

- page meta (event_date / world / cast_list / photographer / note) の編集 UI と表示連携
- /p に meta が表示されるための public viewer API 拡張
- photo 単位の **page 跨ぎ drag & drop** (page picker dropdown のみ)
- preview を別 tab / 別 route で開く方式 (本書では同画面トグル一本)

### 1.2 確定 Q&A (前回ターンで user 確定)

| # | 確定内容 |
|---|---|
| Q1 | 各 photo に「ここで分ける」+ 各 page 先頭に「上と結合」 |
| Q2 | photo 移動は page picker dropdown |
| Q3 | preview は同画面「編集 ⇄ プレビュー」トグル |
| Q4 | Page meta は Phase B 分離 |
| Q5 | 30 page 上限 / photo 上限は domain 既存ルール維持 |
| Q6 | Backend deploy は既存 Cloud Build manual trigger (`docs/runbook/backend-deploy.md`) |
| Q7 | DB migration は forward-only / Backend は前 image tag rollback / Workers は前 version rollback |
| Q8 | 実装順は Backend → frontend lib → UI → Preview → 検証 → deploy |

### 1.3 user 追加調整 (前ターン)

> Phase A の endpoint は最初から 5 個全部でもよいが、実装順は **split / move / caption を先に**するのが安全です。merge と pages/reorder は UI 体験を良くするが、最初の価値は「ページを自由に分けられる」ことなので、核は split と move です。

→ STOP 内訳に反映 (§9)。

## 2. 既存コードの状態調査結果 (重要)

### 2.1 DB schema は **既に Phase A + Phase B 用カラム揃い済**

| migration | テーブル | 関連列 |
|---|---|---|
| `00007_create_photobook_pages.sql` | `photobook_pages` | `caption text NULL` (CHECK len 0..200) ✓ |
| `00008_create_photobook_photos.sql` | `photobook_photos` | `caption text NULL` / `display_order int NOT NULL` / UNIQUE(page_id, display_order) ✓ |
| `00009_create_photobook_page_metas.sql` | `photobook_page_metas` | `world / cast_list text[] / photographer / note / event_date date` 全 NULL 許容 ✓ |

> **DB migration: 不要**。Phase A も Phase B もカラム追加無し。

### 2.2 Domain (集約子エンティティ) も実装済

- `domain/page.go` — Page (`pageCaption *caption.Caption`)、`Reorder(newOrder, now)` メソッドあり
- `domain/page_meta.go` — PageMeta (5 field、length validation、`maxWorldLen=200` / `maxPhotographerLen=100` / `maxNoteLen=1000` / `maxCastEntries=50` / `maxCastEntryLen=100`)
- `domain/photo.go` — Photo (`pageID`, `displayOrder`, `imageID`, `caption`)
- `domain/photobook.go` — 集約ルート、`Version() int`、`Status()` (draft / published / deleted)
- 集約子テーブル更新ルール (`.agents/rules/domain-standard.md`) は厳守 — 子テーブル単独 Repository 公開禁止 / 親 version+1 同一 TX

> **Domain 拡張: 不要**。新 Page caption setter / PageMeta setter ですらすでに `RestorePage` / `RestorePageMeta` で戻せる。新規メソッドは Page に `WithCaption(*caption, now)` 程度で足りる (immutable inst の差替).

### 2.3 SQL queries (sqlc 入力) — 一部欠如、追加が必要

**既出 (再利用):**

- `CreatePhotobookPage` / `ListPhotobookPagesByPhotobookID` / `FindPhotobookPageByID` / `CountPhotobookPagesByPhotobookID` / `DeletePhotobookPage` / `UpdatePhotobookPageOrder`
- `CreatePhotobookPhoto` / `ListPhotobookPhotosByPageID` / `FindPhotobookPhotoByID` / `CountPhotobookPhotosByPageID` / `DeletePhotobookPhoto`
- `UpdatePhotobookPhotoOrder` (single, UNIQUE 衝突あり前提)
- `BulkOffsetPhotoOrdersOnPage` (+1000 escape) ← **page reorder にも同パターンで使う**
- `UpdatePhotobookPhotoCaption`
- `UpsertPhotobookPageMeta` / `FindPhotobookPageMetaByPageID` (Phase B で利用)
- `BumpPhotobookVersionForDraft` (親 version+1、同 TX 必須)

**追加が必要 (Phase A):**

| 新 query 名 | 用途 | 備考 |
|---|---|---|
| `UpdatePhotobookPageCaption` | page caption 単独編集 | photo 版と同じパターン、`WHERE id = $1 AND photobook_id = $2`、`:execrows` |
| `BulkOffsetPagesInPhotobook` | page reorder の +1000 escape | photo 版と同パターン、対象は同 photobook の全 page |
| `UpdatePhotobookPhotoPageAndOrder` | photo を別 page に移動 | `UPDATE photobook_photos SET page_id = $2, display_order = $3 WHERE id = $1` |

`UpdatePhotobookPhotoOrder` の単一 UPDATE は新 page で UNIQUE 衝突する可能性があるため、move 内部では target page の display_order を **同 TX で先に offset (+1000)** → INSERT/UPDATE → 順次戻す方式を採る。

### 2.4 Repository methods — 一部欠如

**既出 (再利用):** `AddPage` / `RemovePage` / `ListPagesByPhotobookID` / `CountPagesByPhotobookID` / `AddPhoto` / `RemovePhoto` / `ReorderPhoto` / `ListPhotosByPageID` / `CountPhotosByPageID` / `SetCoverImage` / `ClearCoverImage` / `UpsertPageMeta` / `FindPageMetaByPageID` / `BumpVersion` / `UpdatePhotoCaption` / `BulkReorderPhotosOnPage` / `UpdateSettings`

**追加 (Phase A):**

- `UpdatePageCaption(ctx, photobookID, pageID, *caption, expectedVersion, now) error`
- `BulkReorderPagesInPhotobook(ctx, photobookID, assignments []PageOrderAssignment, expectedVersion, now) error`
- `MovePhotoBetweenPages(ctx, photobookID, photoID, sourcePageID, targetPageID, targetDisplayOrder, expectedVersion, now) error`

> Split / Merge は **既存 primitives を組み合わせた UseCase 内部処理**として実装する (Repository に SplitPage / MergePages 直接 method を作らない、Repository は thin)。UseCase が `BulkOffsetPhotoOrdersOnPage` + 既存 add_page / remove_page / reorder primitive を 1 TX で組み立てる。

### 2.5 UseCase 構造体

**既出:** `AddPage` / `AddPhoto` / `AttachAvailableImages` / `CreateDraftPhotobook` / `UpdatePhotoCaption` / `BulkReorderPhotosOnPage` / `UpdatePhotobookSettings` / `GetEditView` / `GetPublicPhotobook` / `GetManagePhotobook` / `PhotobookEdit` (cover) / `PublishFromDraft` / 等

**追加 (Phase A):**

- `UpdatePageCaption` (photo caption と同 pattern、約 70 行)
- `MovePhotoBetweenPages` (核ロジック、約 200 行、§4.3)
- `SplitPage` (核ロジック、約 250 行、§4.4)
- `MergePages` (約 150 行、§4.5)
- `ReorderPages` (約 120 行、§4.6)

### 2.6 Frontend lib / UI / Viewer

- `lib/editPhotobook.ts` の `EditPage` 型は `caption?: string` を **既に保持** (line 56-61)
- `editViewPayload` (Backend) は `caption` を返している (`get_edit_view.go:228 Caption: captionToPtr(page.Caption())` / `edit_handler.go:216 editPagePayload.Caption`)
- → 現在の /edit UI が表示していないだけ。**API 拡張なしで読み取りは即座に使える**
- `components/Viewer/*` (v2 redesign) の Cover / PageHero / PageMeta / PageNote / TypeAccent / Lightbox 等は **Server Component を中心に組まれており、入力 prop が `PublicPhotobook` 型** → `EditView → PublicPhotobook` 変換関数 1 個で再利用可能
- `EditClient.tsx` は 689 行、`view.version` を OCC token として一貫使用、`bumpVersion(applyXxx(v, ...))` で local 楽観更新、`reload()` で再 fetch

## 3. Backend endpoint spec (final)

### 3.1 endpoint 一覧 (5 件、Phase A)

| 順序 | method | path | 主用途 | priority |
|---|---|---|---|---|
| 1 | `PATCH` | `/api/photobooks/{id}/pages/{pageId}/caption` | page caption 設定 / 解除 | core |
| 2 | `POST` | `/api/photobooks/{id}/pages/{pageId}/split` | 指定 photo の **次から** 新 page に分離 | **核 (最優先)** |
| 3 | `PATCH` | `/api/photobooks/{id}/photos/{photoId}/move` | photo を別 page に移動 | **核 (最優先)** |
| 4 | `POST` | `/api/photobooks/{id}/pages/{pageId}/merge-into/{targetPageId}` | 2 page 結合 (UX 補強) | secondary |
| 5 | `PATCH` | `/api/photobooks/{id}/pages/reorder` | 全 page を新順序で一括再配置 (UX 補強) | secondary |

### 3.2 共通の規約

- すべて `Cookie` 認可 (draft session middleware 通過必須)
- すべて `expected_version` 必須 / 不一致時 409 + `{"status":"version_conflict"}`
- すべて draft 状態以外で 409 + `{"status":"version_conflict"}` (敵対者観測抑止のため理由分離せず、旧 cover-image PATCH と同方針)
- 失敗時 raw photobook_id / image_id / token / Cookie / Secret / storage_key / presigned URL を body に出さない
- CORS: `cors.go` の `AllowedMethods` に `PATCH` / `POST` 追加済 (cors-mutation-methods rule、`a8fe0db` で対応済)
- 成功時 response body shape: **「version のみ」または「更新後 EditView 全体」のどちらにするか**

### 3.3 推奨 response shape: **更新後 EditView を返す (option B)**

| option | pros | cons |
|---|---|---|
| A. version のみ返す `{"version": N+1}` | 軽量、既存 mutation と整合 (caption / cover / settings は既に version のみ) | UI 側で `applyXxx` 楽観更新ロジックが mutation ごとに必要、split / merge / move のロジックが Frontend 側に複製、整合性ドリフト リスク高 |
| **B. 更新後 EditView 全体を返す** ✓ | UI は受信値で `setView(next)` 1 行、Backend が ground truth を保証、整合性ドリフト無し、複雑な split / merge ロジックを Frontend で複製不要 | response body が大きい (5〜30 KB)、presigned URL 期限再発行が同 endpoint で起こる |
| C. 差分 (diff) を返す | 軽量 + 楽観更新も簡単 | spec が複雑、test 困難、UI Bug 出やすい |

**recommended: B (更新後 EditView 返す)**。理由:

1. split / merge / reorder は **複数 page / photo の display_order を一斉に変える**ため、Frontend で完全再現するのは現実的でない
2. `caption` のような単純更新は既存 A 方式 (version のみ) でよいが、**複合更新は B 方式に統一する方が 1 vs N の差で sane**
3. presigned URL 再発行は副次効果として「URL 期限切れ前に手前で更新」のメリットがある (15 分有効期限なので長い編集 session で得)
4. Phase A の **新 endpoint 5 個すべて B 方式に統一**、既存の caption / cover / settings は **A 方式のまま据え置き** (互換性維持)

ただし、**page caption 単独更新 (endpoint #1)** は既存 photo caption と同じ A 方式 (version のみ) の方が整合的。**caption は A、split / merge / move / reorder は B、で混在を許容**する。

#### 推奨 response shapes (確定)

```
PATCH /pages/{pageId}/caption        → 200 {"version": N+1}                         (A 方式)
POST  /pages/{pageId}/split          → 200 EditView (更新後)                          (B 方式)
PATCH /photos/{photoId}/move         → 200 EditView (更新後)                          (B 方式)
POST  /pages/{pageId}/merge-into/... → 200 EditView (更新後)                          (B 方式)
PATCH /pages/reorder                 → 200 EditView (更新後)                          (B 方式)
```

### 3.4 endpoint 個別 spec

#### 3.4.1 PATCH `/pages/{pageId}/caption`

```http
PATCH /api/photobooks/{photobookId}/pages/{pageId}/caption
Content-Type: application/json
Cookie: <draft session>

{
  "caption": "屋上に集まったメンバー、夕焼けを背に。",   // null or "" でクリア
  "expected_version": 17
}
```

**Response 200**:
```json
{ "version": 18 }
```

**Response 409** (`{"status":"version_conflict"}`):
- expected_version 不一致
- photobook が draft 以外
- pageId が当該 photobook 配下にない

**Response 400** (`{"status":"bad_request"}`):
- caption length 200 超過
- caption が文字以外を含む (validation は Domain `caption.New` に委譲)

**Response 404**: photobookId / pageId 不存在 (敵対者観測抑止のため bad_request と区別する場面なし)

#### 3.4.2 POST `/pages/{pageId}/split`

```http
POST /api/photobooks/{photobookId}/pages/{pageId}/split
Content-Type: application/json
Cookie: <draft session>

{
  "split_at_photo_id": "<切断点 photo>",   // この photo の "次" から新 page へ。"" or null は不可
  "new_page_caption": null,                // 新 page の caption (任意)、未指定なら null
  "expected_version": 17
}
```

**Behavior**: 指定 `split_at_photo_id` の **次の photo** から末尾までを、新 page に移す。

- 切断点が page 末尾 photo → 新 page は空 (= 元 page と同内容、新 page 0 photo)。**これは禁止 (400)**: split は「写真を 2 page に分ける」操作、空 page を生むだけの操作はできない。空 page が欲しいなら既存 `POST /pages` を使う。
- 切断点が page 先頭 photo → 新 page には photo が 1 枚以上残り、元 page には先頭 1 枚のみ残る。これは OK。
- 30 page 上限到達時 → 409 + `{"status":"version_conflict","reason":"page_limit_exceeded"}` (新 reason、`docs/security/publish-precondition-ux.md` の publish_precondition_failed パターンを編集 mutation にも展開)

**Response 200** (B 方式、更新後 EditView):
```json
{
  "photobook_id": "<id>",
  "version": 18,
  "pages": [...],
  "settings": {...},
  "cover": {...},
  "images": [...],
  ...  // 既存 editViewPayload 全体
}
```

**Response 409**:
- expected_version 不一致 / draft 以外 → `{"status":"version_conflict"}`
- 30 page 上限到達 → `{"status":"version_conflict","reason":"page_limit_exceeded"}`
- split_at_photo_id が page 末尾 → `{"status":"version_conflict","reason":"split_would_create_empty_page"}` (操作上不可能なケースを reason 化)

**Response 400**:
- split_at_photo_id が当該 page にない / 不正 UUID
- expected_version 欠落

#### 3.4.3 PATCH `/photos/{photoId}/move`

```http
PATCH /api/photobooks/{photobookId}/photos/{photoId}/move
Content-Type: application/json
Cookie: <draft session>

{
  "target_page_id": "<page>",            // 同 photobook 配下の page、source と異なってよい / 同じでもよい
  "target_display_order": 2,             // 0..targetPage.photos.length (先頭 / 末尾 含む) の整数
  "expected_version": 17
}
```

**Behavior**:

- source page = photo 現在の page、target page = body 指定。同一でも別でもよい
- target page の `target_display_order` 位置に photo を挿入、既存 photo を後ろに 1 ずつシフト
- source page から photo を抜いた後、後続 photo を 1 ずつ前詰め
- 同 page の場合は `BulkReorderPhotosOnPage` と等価動作

**Response 200**: B 方式 (EditView)

**Response 409**:
- expected_version / draft 不一致 → `{"status":"version_conflict"}`

**Response 400**:
- target_page_id が当該 photobook 配下にない
- target_display_order 範囲外 (0..targetPage.photos.length 外)
- photo_id 不存在 / 不正 UUID

#### 3.4.4 POST `/pages/{pageId}/merge-into/{targetPageId}`

```http
POST /api/photobooks/{photobookId}/pages/{pageId}/merge-into/{targetPageId}
Content-Type: application/json
Cookie: <draft session>

{
  "expected_version": 17
}
```

**Behavior**:

- `pageId` (source) の全 photo を `targetPageId` (target) の **末尾に追加**
- source page 自身を削除 (CASCADE で source の photobook_page_metas も自動削除)
- target page の caption / meta はそのまま (source 側の caption / meta は捨てられる、明示警告 UI を Frontend で出す)
- 削除された source page 以降の page の display_order を 1 つずつ繰り上げ (gap 防止)
- photobook.version+1

**Response 200**: B 方式 (EditView)

**Response 409**:
- expected_version / draft 不一致 → `{"status":"version_conflict"}`
- pageId == targetPageId (自己 merge) → `{"status":"version_conflict","reason":"merge_into_self"}` ← UI 側で出さない想定だが defensive
- 1 page しか存在せず source = sole page → `{"status":"version_conflict","reason":"cannot_remove_last_page"}` (集約不変条件: photobook には 1 page 以上必要、I1 ルール `00007` 仕様参照)

**Response 400**:
- pageId / targetPageId が当該 photobook 配下にない
- photo 数の合計が制限超過 (現状 photo 数上限は domain で確認、§5.6 で定義)

**Response 404**: photobookId / pageId / targetPageId 不存在

#### 3.4.5 PATCH `/pages/reorder`

```http
PATCH /api/photobooks/{photobookId}/pages/reorder
Content-Type: application/json
Cookie: <draft session>

{
  "assignments": [
    { "page_id": "<p1>", "display_order": 0 },
    { "page_id": "<p2>", "display_order": 1 },
    { "page_id": "<p3>", "display_order": 2 }
  ],
  "expected_version": 17
}
```

**Behavior**:

- assignments は当該 photobook の **全 page** を含む必要あり (部分 reorder 不可)
- display_order は 0..N-1 の重複 / 欠番なし permutation でなければ 400
- 内部実装: `BulkOffsetPagesInPhotobook` (+1000 escape) → `UpdatePhotobookPageOrder` を順次

**Response 200**: B 方式 (EditView)

**Response 409**:
- expected_version / draft 不一致 → `{"status":"version_conflict"}`

**Response 400**:
- assignments が page 全件と一致しない (page_id 欠落 / 余剰 / 重複)
- display_order の permutation 不正

## 4. Atomicity / TX rules

### 4.1 共通

- 全 mutation は `database.WithTx(ctx, pool, func(tx pgx.Tx) error { ... })` で 1 TX
- TX 内冒頭で `bumpVersion` (= UPDATE photobooks SET version+1 WHERE id=$1 AND version=$2 AND status='draft', execrows == 0 → ErrOptimisticLockConflict)
- 親 version bump 後に子テーブル更新を続け、失敗時は全体 rollback
- `now` パラメータは Application 層から渡す (`time.Now().UTC()` を handler / usecase で 1 度 captureして使い回す)
- 全 SQL の `now()` 直書きは避ける (`.agents/rules/domain-standard.md` 時刻パラメータ化ルール)

### 4.2 page display_order の連続性 (0..N-1)

DB 側は UNIQUE(photobook_id, display_order) のみ。連続性は Application 層で担保:

- **AddPage**: 末尾追加 (count = N → 新 page display_order = N)
- **DeletePhotobookPage** (内部使用、merge / 既存 RemovePage): 削除後に後続 page を `display_order--`
- **page reorder**: `BulkOffsetPagesInPhotobook(photobookID)` で全 page を +1000 → 新 order を順次書く
- **split**: 新 page を `sourcePage.displayOrder + 1` に挿入、後続 page を 1 つずつ後ろにシフト

### 4.3 photo display_order の連続性 (page 内 0..N-1)

- **AddPhoto**: page 末尾追加 (count = M → display_order = M)
- **RemovePhoto** + page 内 photos に対し後続 `display_order--`
- **bulk reorder (同 page)**: `BulkOffsetPhotoOrdersOnPage(pageID)` で +1000 → 新 order
- **move-between-pages** (異なる source / target):
  1. source page から photo を「抜く」: source の `display_order > photo.displayOrder` の photo を `display_order--` で前詰め
  2. target page の `display_order >= target_display_order` の photo を **+1000** offset (escape)
  3. photo の `page_id, display_order` を target に UPDATE
  4. target の +1000 offset した photo を target_display_order+1 から順次戻す

### 4.4 photobooks.version は同一 TX で +1

`BumpPhotobookVersionForDraft` を **TX の冒頭** で実行。子テーブル UPDATE は version bump 後に続ける。`SplitPage` / `MovePhotoBetweenPages` のような複数子操作でも version+1 は **1 回のみ** (累積 +1)。

### 4.5 draft 以外は mutation 不可

`BumpPhotobookVersionForDraft` の `WHERE status='draft'` 条件で 0 行更新 → ErrOptimisticLockConflict として handler 層で 409 `{"status":"version_conflict"}` を返す (status / version の差を理由として分離しない、敵対者観測抑止)。

## 5. Edge case 一覧 (実装時に必ずテストする)

### 5.1 30 page 到達時 split

- 既存 page 数が 30 → split で 31 page になる
- expected: 409 + `{"status":"version_conflict","reason":"page_limit_exceeded"}`
- TX rollback で page count / display_order に変化なし

### 5.2 empty page split (defensive)

- source page に photo 0 件 (通常ありえないが、move で空にした直後など)
- expected: 400 (split_at_photo_id が source に存在しないため不正)

### 5.3 page 先頭 photo で split (`split_at_photo_id == photos[0]`)

- expected: 元 page には先頭 1 photo、新 page には 2..N が移る (OK)

### 5.4 page 末尾 photo で split (`split_at_photo_id == photos[N-1]`)

- expected: 新 page は空になる
- これは **400 + `{"status":"split_would_create_empty_page"}`** で拒否 (空 page を作る操作は別 endpoint = `POST /pages` がある)

### 5.5 merge target が同一 page (`pageId == targetPageId`)

- expected: 409 + `{"status":"version_conflict","reason":"merge_into_self"}`
- UI 側でも出させない想定だが defensive

### 5.6 photo 上限 (page 内 / photobook 内)

- 現状 `domain.MaxPhotosPerPhotobook` 等の定義は未確認。merge で結果 photo 数が page 上限超 → 拒否すべきか
- **Phase A は維持: page 内 photo 数の hard 上限は MVP では設けない**。実用上 100 photo / page でも UI が破綻しないことを Frontend smoke で確認する
- 1 photobook 全体の photo 数上限は別 issue (Backend で MAX 200 等を実装するなら別 STOP)

### 5.7 move target_display_order 範囲外

- target page の photo 数 K に対し、target_display_order が `< 0` or `> K`
- expected: 400 (invalid_parameters)
- 注意: `== K` は「末尾追加」で valid

### 5.8 source page が空になる move

- source page に photo 1 枚のみ → 別 page に move すると source は 0 photo になる
- expected: 200 (空 page は許容、既存仕様)

### 5.9 move で photo の image が cover_image_id

- cover_image_id は photo ではなく image を指す。photo の page 移動で cover 指定は不変
- expected: cover はそのまま、photo の page_id / display_order だけ変わる
- 注: photo を **削除** すると cover 解除が必要かは別問題 (既存 RemovePhoto の挙動を踏襲、本書では触らない)

### 5.10 concurrent edit / expected_version mismatch

- expected: 409 + `{"status":"version_conflict"}`
- TX rollback、副作用なし
- Frontend は `EditClient.handleApiError` の既存 path で「最新を取得」CTA を出す (UX 既存)

### 5.11 reorder assignments 部分 / 重複 / 欠番

- 全 page を含まない / display_order 重複 / 0..N-1 permutation でない → 400
- TX に入る前に handler / usecase 入口で validate

### 5.12 split / merge / move を立て続けに行う (race ではなく順次)

- 各 mutation で version+1、Frontend は B 方式 response の `version` を view.version に反映 → 連続操作 OK
- 中間で他 client が編集 → 409、reload で解消

### 5.13 sole page を merge source にする

- 1 page しか存在しない photobook で merge 実行 → source = sole = target にできない (5.5) し、別 target もないので操作自体不可
- UI 側で merge button を非表示にすれば足りるが defensive で 409 + `cannot_remove_last_page`

### 5.14 split / merge / move 後の cover image 表示整合

- B 方式の response に `cover` (variant URL) を再発行して含めるため、Frontend は `setView(response)` で自動整合
- presigned URL 期限切れリスクは EditView 全体再 fetch なので解消

## 6. Frontend UI 設計 (Phase A)

### 6.1 components 構成 (新規 + 改修)

```
components/Edit/
├── PageBlock.tsx         (新規) — 1 page 分の見出し + photo grid + page-level actions
├── PageCaptionEditor.tsx (新規) — page caption 編集 (photo CaptionEditor とほぼ同形)
├── PageActionBar.tsx     (新規) — page header の操作 (上と結合 / page 削除 / page 上下移動 / page reorder)
├── PhotoActionBar.tsx    (新規 or PhotoGrid 内部) — 各 photo の「ここで分ける」/「他のページへ移動」
├── PageMovePicker.tsx    (新規) — photo move-between-pages の page picker dropdown
├── PreviewToggle.tsx     (新規) — 編集 ⇄ プレビュー切替 button
├── PreviewPane.tsx       (新規) — Viewer (v2) を re-render する内部コンテナ
├── PhotoGrid.tsx         (改修) — 各 photo に PhotoActionBar 追加
├── CaptionEditor.tsx     (既存維持) — photo caption 編集
├── ReorderControls.tsx   (既存維持) — photo の同 page 内 reorder
├── CoverPanel.tsx        (既存維持)
└── PublishSettingsPanel.tsx (既存維持)
```

### 6.2 PageBlock コンポ案

```
[PageHeader: PAGE 01 + caption editor + PageActionBar]
  PageActionBar buttons:
    ↑ 上のページと結合 (page index >= 1 のとき)
    ↕ ページ順 (drag handle、reorder controls)
    🗑 page 削除 (既存 RemovePage)

[PhotoGrid (改修)]
  各 photo に既存 ReorderControls + 追加で:
    ✂ ここで分ける  (clicked → POST /split)
    📥 他のページへ移動  (clicked → PageMovePicker dropdown を開く)
    cover に設定 / coverを外す (既存)
    🗑 削除 (既存)
```

Mobile では PageActionBar と PhotoActionBar が「⋮ メニュー」popover に格納、PC では inline 表示。

### 6.3 「ここで分ける」UX (Q1 確定)

- 各 photo card の右下に icon-only button「✂ ここで分ける」
- click → confirm modal (or aria-live で alert): 「この写真の次から新しいページに分けます。よろしいですか?」 (split は破壊的でないが、page 30 上限前なら気軽でよい、modal なし即実行 + Toast 「ページを分けました」 が UX 推奨)
- POST /split → response の EditView で `setView` 更新
- 30 page 上限時は button disable + tooltip「ページ数が上限 (30) に達しています」

### 6.4 「上と結合」UX (Q1 確定)

- page index >= 1 の page の header 左端に button「↑ 上と結合」
- click → confirm modal: 「上のページと結合します。このページの caption は破棄されます。」
- POST /merge-into → response で setView
- 1 page しか無いなら button 非表示

### 6.5 photo 移動 dropdown UX (Q2 確定)

- photo card の「📥 他のページへ移動」click → `<select>` like dropdown
- 選択肢: 全 page (現在の page は disabled or 表示しない)、各 entry に「PAGE 01: caption (3 photos)」表記
- 選択 → 「末尾に追加」 / 「先頭に挿入」のみ MVP (中間挿入は drag が必要、Phase B)
- PATCH /move → setView
- 同 page 内 reorder は既存 ReorderControls (UP/DOWN/TOP/BOTTOM) を維持

### 6.6 page reorder UX (Q1 / Q5 確定)

- PageActionBar に「↑ ページを上へ」「↓ ページを下へ」inline buttons
- Phase A は **隣接 swap のみ** (drag は Phase B)
- 内部的には PATCH /pages/reorder に全 page の新 order を渡す (1 swap でも全件送る、Backend は permutation validate)

### 6.7 page caption 編集

- PageBlock の header に inline `<input>` (PageCaptionEditor)
- focus 外し / Enter key で PATCH /caption (既存 photo CaptionEditor と同 pattern)
- A 方式 response (`{"version": N+1}`) を受けて `setView((v) => bumpVersion({...v, pages: v.pages.map(...) }))` で楽観更新

### 6.8 preview toggle UX (Q3 確定)

- /edit page top-right 固定: button 「📖 プレビュー」(編集中) / 「✏️ 編集に戻る」(プレビュー中)
- Mobile: bottom sticky で「プレビュー / 編集」タブ
- toggle = 同画面 state、reload なし
- preview state では **同 page** の中身全体を `<PreviewPane photobook={editViewToPreview(view)} />` で置換
- PreviewPane は v2 ViewerLayout を draft data 受け取りで render
- preview 中も `view.version` は固定、編集 mutation を投げない (preview は read-only)
- presigned URL は EditView と同じものを使う (15 分有効、preview 中に切れたら edit に戻って `reload()` で再取得を user に促す)

### 6.9 EditView → PublicPhotobook 変換 (PreviewPane 用)

```ts
// frontend/lib/editPreview.ts (新規)
import type { EditView } from "@/lib/editPhotobook";
import type { PublicPhotobook } from "@/lib/publicPhotobook";

export function editViewToPreview(v: EditView): PublicPhotobook {
  return {
    photobookId: v.photobookId,
    slug: "draft-preview",                      // 識別用、本物 slug にしない
    type: v.settings.type,
    title: v.settings.title || "(タイトル未設定)",
    description: v.settings.description,
    layout: v.settings.layout,
    openingStyle: v.settings.openingStyle,
    creatorDisplayName: "プレビュー (公開時に creator 名)",  // draft なので暫定
    creatorXId: undefined,                      // draft では未確定 (publish 時の creator から)
    coverTitle: v.settings.coverTitle,
    cover: v.cover,                             // EditVariantSet と PublicVariantSet は形式互換
    publishedAt: new Date().toISOString(),      // 暫定 (now)
    pages: v.pages.map((p) => ({
      caption: p.caption,
      photos: p.photos.map((ph) => ({
        caption: ph.caption,
        variants: ph.variants,
      })),
      meta: undefined,                           // Phase A は meta 未対応
    })),
  };
}
```

**型互換性チェック**:

- `EditPresignedURL` ↔ `PublicPresignedURL`: 両方とも `{url, width, height, expiresAt}` で形式互換
- `EditVariantSet` ↔ `PublicVariantSet`: 両方とも `{display, thumbnail}` で形式互換
- `EditPhoto.caption?: string` ↔ `PublicPhoto.caption?: string`: 互換
- `EditPage.caption?: string` ↔ `PublicPage.caption?: string`: 互換 (photos は再構成)

### 6.10 EditClient のステート拡張

```ts
type ViewMode = "edit" | "preview";

const [mode, setMode] = useState<ViewMode>("edit");
// preview state は editView から派生、別 useState 不要
const previewPhotobook = useMemo(
  () => mode === "preview" ? editViewToPreview(view) : null,
  [mode, view],
);
```

### 6.11 既存 EditClient mutation handler の拡張方針

| 既存 handler | Phase A 改修 |
|---|---|
| `onCaptionSave` (photo) | 不変 (既存維持) |
| `reorderTo` (photo within page) | 不変、ただし PageMovePicker から「同 page 内」reorder と「page 跨ぎ」move を振り分け |
| `onSetCover` / `onClearCover` | 不変 |
| `onRemovePhoto` | 不変 |
| `onSaveSettings` | 不変 |
| `onAddPage` | 不変 (空 page を追加する旧導線、Phase A でも維持) |
| 新 `onSavePageCaption(pageId, caption)` | A 方式、`setView((v) => bumpVersion({ ...v, pages: v.pages.map(...) }))` |
| 新 `onSplitPage(pageId, splitAtPhotoId)` | B 方式、`setView(response.editView)` |
| 新 `onMergePage(sourcePageId, targetPageId)` | B 方式 |
| 新 `onMovePhoto(photoId, targetPageId, targetDisplayOrder)` | B 方式 |
| 新 `onReorderPages(assignments)` | B 方式 |

## 7. Test matrix (実装時に最低限カバー)

### 7.1 Backend repository test (`*_test.go`、postgres test container)

| Repository method | テストケース最低限 |
|---|---|
| `UpdatePageCaption` | 正常 / OCC 不一致 / draft 以外 / pageId 不一致 / caption length 200 / null clear / photobook_id 不一致 |
| `BulkReorderPagesInPhotobook` | 正常 (3 page 並べ替え) / OCC / draft 以外 / 一部 missing / display_order 重複 / 0..N-1 permutation 違反 / 1 page only |
| `MovePhotoBetweenPages` | 同 page 移動 (= bulk reorder と等価) / 別 page 末尾 / 別 page 先頭 / 別 page 中間 / target_display_order 範囲外 / source = sole photo (空 page 残る) / target page = source page (no-op) / OCC |

### 7.2 Backend usecase test (table-driven + Builder)

| UseCase | テストケース |
|---|---|
| `UpdatePageCaption` | 7.1 と同等 + caption.New validation エラー path |
| `MovePhotoBetweenPages` | 7.1 + 集約境界 (異なる photobook の page を target にする → ErrEditNotAllowed 相当) |
| `SplitPage` | 切断点先頭 / 中間 / 末尾 (拒否) / 30 page 到達 / source page に photo 0 件 / OCC / draft 以外 / split_at_photo_id 不存在 |
| `MergePages` | 通常 / source = target (拒否) / 1 page 残るのみ (拒否) / target に photo 既存 / source / target が異なる photobook (拒否) / OCC |
| `ReorderPages` | 通常 / 部分 (拒否) / permutation 違反 (拒否) / OCC |

### 7.3 Backend handler test (`edit_handler_test.go` 拡張)

各 endpoint で:

- 200 path (正常)
- 401 (Cookie なし) — middleware で弾かれる前提、本 handler では 401 直接返さない
- 404 (photobookId / pageId / photoId 不存在)
- 409 (OCC / draft 以外 / page_limit_exceeded / split_would_create_empty_page / merge_into_self / cannot_remove_last_page)
- 400 (request body 不正 / target_display_order 範囲外 / assignments 不正)
- response shape (A 方式 = `{"version": N}`, B 方式 = full EditView)
- raw photobook_id / image_id / token を error body に出さない

### 7.4 Frontend lib test (`lib/__tests__/editPhotobook.test.ts` 拡張)

新 mutation 5 関数:

- `updatePageCaption` / `splitPage` / `mergePages` / `movePhoto` / `reorderPages`
- 200 path (mock fetch)
- 401 / 404 / 409 / 400 各 error mapping
- B 方式: response の EditView shape parse 確認
- A 方式: response の version parse 確認
- credentials: "include" / Content-Type 確認 (`.agents/rules/client-vs-ssr-fetch.md` 遵守)

新 lib `editPreview.ts`:

- `editViewToPreview(v)` の table-driven test:
  - cover あり / なし
  - meta は常に undefined (Phase A)
  - description / coverTitle 有無
  - 5 page / 0 page (defensive)
- 出力 PublicPhotobook が型互換であることを type assertion で確認

### 7.5 Frontend component / EditClient behavior test

| test | 確認内容 |
|---|---|
| `EditClient.preview-toggle.test.tsx` | mode toggle で `<PreviewPane>` 表示 / 編集に戻ると元の grid 表示 / preview 中は mutation 不可 (button disable) |
| `EditClient.split.test.tsx` | 「ここで分ける」click → splitPage 呼出 + setView(response) / 30 page 到達時 button disabled / page 末尾 photo で disable |
| `EditClient.merge.test.tsx` | 「上と結合」click → mergePages 呼出 / 1 page only で button 非表示 |
| `EditClient.move.test.tsx` | PageMovePicker → movePhoto 呼出 / 同 page 選択は disable |
| `EditClient.page-caption.test.tsx` | PageCaptionEditor onBlur → updatePageCaption 呼出 + 楽観更新 |
| `harness-class-guards.test.ts` (既存拡張) | 新 mutation 5 関数が `credentials: "include"` 経路を使っていることを source guard |
| `ViewerLayout.test.tsx` (既存) | 既存テスト保持、preview pane 経由でも render 可能 |

### 7.6 Regression (既存テスト全 PASS)

- 既存 396 test 全 PASS (現在 main の状態)
- 特に `EditClient.publish.test.ts` / `EditClient.reload.test.ts` / `harness-class-guards.test.ts`
- publish flow / OCC / credentials / sensitive guard / business violation guard 不変

## 8. Rollback path (Q7 確定)

### 8.1 forward-only DB migration

本 Phase A は **migration 0 件**のため rollback 自体不要。Phase B で追加するなら Phase B 開始時に再評価。

### 8.2 Backend (Cloud Run) rollback

- 各 STOP の deploy で前 image tag を rollback target として記録
- 失敗時 `gcloud run services update-traffic vrcpb-api --region=asia-northeast1 --to-revisions <prev>=100`
- 詳細手順は `docs/runbook/backend-deploy.md` §1.5 既存

### 8.3 Workers (vrcpb-frontend) rollback

- 各 deploy で前 version ID を rollback target として記録 (現状 `673a8e03-...`)
- 失敗時 `npx wrangler deployments rollback <prev-version-id>` または最新 version を `--to 0` で無効化
- forward-only DB との組み合わせ: Backend を rollback すれば Workers の新 UI が新 endpoint を呼んでも 404、Frontend は既存 error path で 「再読み込みしてください」表示 → user に体感される

### 8.4 部分 deploy 順序 (Q8 確定)

1. **Backend deploy 先**: 新 5 endpoint が live になっても旧 Workers (現行 UI) は呼ばないので影響なし
2. **Workers deploy 後**: 新 UI が新 endpoint を呼ぶ
3. **trouble 発生時**: Workers を前 version rollback → 旧 UI に戻る → Backend は新 endpoint live のまま (使われない、害なし) → 後で Backend rollback or hotfix

## 9. Implementation STOP split (確定、user 調整反映)

> user 調整: 実装順は **split / move / caption を先**にする。merge / reorder は UX 補強で後。

| STOP | 範囲 | priority | 期間目安 |
|---|---|---|---|
| **STOP P-α (本書承認)** | 仕様確定、commit せず提示 | (本書) | (完了後 stop) |
| **STOP P-1**: SQL + Repository (核 3 endpoint) | `UpdatePhotobookPageCaption` SQL + `UpdatePhotobookPhotoPageAndOrder` SQL + Repository methods (UpdatePageCaption / MovePhotoBetweenPages) + sqlc 再生成 | core | 1.0 day |
| **STOP P-2**: UseCase (核 3 endpoint) | UpdatePageCaption + MovePhotoBetweenPages + SplitPage UseCase + handler + edit_handler.go 配線 + cors_test.go assertion + repository / usecase / handler test | core | 2.0 day |
| **STOP P-3**: SQL + Repository + UseCase (補強 2 endpoint) | `BulkOffsetPagesInPhotobook` SQL + Repository (BulkReorderPagesInPhotobook) + UseCase (MergePages / ReorderPages) + handler 配線 + 全 test | secondary | 1.5 day |
| **STOP P-4**: Frontend lib | `lib/editPhotobook.ts` に updatePageCaption / splitPage / movePhoto / mergePages / reorderPages 追加 + `lib/editPreview.ts` 新規 + 型 + lib test | core | 0.5 day |
| **STOP P-5**: Frontend UI part 1 (split / move / caption) | PageBlock / PageCaptionEditor / PhotoActionBar / PageMovePicker + EditClient 配線 + UI test (publish-precondition-ux ルール遵守 / 「最新を取得」CTA は version_conflict のみ) | core | 1.5 day |
| **STOP P-6**: Frontend UI part 2 (merge / page reorder + Preview toggle) | PageActionBar 「上と結合」/「↑↓ page reorder」+ PreviewToggle + PreviewPane + EditClient mode state + UI test | core+secondary | 1.5 day |
| **STOP P-γ**: 検証 | typecheck / vitest / next build / cf:build / cf:check / Backend `go test ./...` / `golangci-lint run` 全 PASS、git diff --check / Secret grep | gate | 0.5 day |
| **STOP P-δ**: deploy | Backend Cloud Build deploy → Workers wrangler deploy (順序遵守、§8.4) | gate | 0.5 day |
| **STOP P-ε**: 実機 smoke | PC + iPhone Safari + Android Chrome、split / merge / move / caption / preview / 30 page 上限到達時の UX、24 production view + ErrorState 4 variant 既存 regression | gate | 1.0 day |

**Phase A 合計: 約 10 day** (前 plan の 7 day から増加、user 順序調整 + STOP 細分化のため)。

### 9.1 中間 deploy 戦略

- STOP P-2 完了 = 核 3 endpoint live になるが Frontend UI なし → ユーザ体感変化なし
- STOP P-3 完了 = 5 endpoint live、UI なし
- STOP P-6 完了 = UI 完備
- **STOP P-δ で 1 度に Backend + Workers を deploy**、Backend 先行 deploy で UI 不整合 window を最小化
- 万一 STOP P-2 で Backend だけ deploy → STOP P-3 までの間、Frontend は新 endpoint を呼ばないので影響なし

### 9.2 中間 commit 戦略

- 各 STOP で 1 commit + push (review 単位を STOP に揃える)
- merge ではなく fast-forward (またはまとめて squash merge は最後の判断)

## 10. Open questions (本書承認時に user 判断を求める)

| # | 質問 | recommended |
|---|---|---|
| **OQ1** | response shape: caption は A 方式 (version のみ)、split / move / merge / reorder は B 方式 (EditView) で OK か | ✅ §3.3 通り、混在許容 |
| **OQ2** | 30 page 上限到達時の split は 409 + reason `page_limit_exceeded` で OK か (敵対者観測抑止より UX 復旧優先、authenticated owner 経路) | ✅ `.agents/rules/publish-precondition-ux.md` のルールを編集 mutation にも展開 |
| **OQ3** | merge は source page の caption / page_meta を破棄、target のものを保持で OK か (UI で confirm modal で警告) | ✅ シンプルで Backend 1 TX で完結 |
| **OQ4** | move-between-pages の target_display_order は MVP で「先頭 / 末尾」のみ UI 提供、中間挿入は Phase B か | ✅ Q2 確定の page picker dropdown と整合 |
| **OQ5** | preview pane で creator name は「プレビュー」固定で OK か (publish 時に Backend が photo book.creator から取得する) | ✅ draft session に creator 概念がないため、暫定 / 公開時上書き |
| **OQ6** | page reorder は隣接 swap のみ MVP、drag は Phase B で OK か | ✅ MVP で十分、UX 確認後判断 |
| **OQ7** | photo の page 移動を **drag & drop** で UX 強化するのは Phase B / C か | ✅ Phase B (page meta) と同じ STOP に含めるか別途検討 |
| **OQ8** | preview 中に edit 操作 (split etc.) を試みた場合の挙動は **disable + tooltip「プレビュー中は編集できません」** で OK か | ✅ 単純、誤操作防止 |
| **OQ9** | publish 後 (status=published) は /edit 自体が立ち上がらない (既存 ErrEditNotAllowed)、本 Phase でも維持で OK か | ✅ 既存通り |

## 11. Backend 実装の核 ロジック (sketch)

### 11.1 SplitPage (核 endpoint、§3.4.2)

```go
// usecase/split_page.go
package usecase

type SplitPageInput struct {
    PhotobookID       photobook_id.PhotobookID
    SourcePageID      page_id.PageID
    SplitAtPhotoID    photo_id.PhotoID
    NewPageCaption    *caption.Caption  // optional
    ExpectedVersion   int
    Now               time.Time
}

type SplitPageOutput struct {
    View EditPhotobookView  // B 方式
}

var (
    ErrSplitWouldCreateEmptyPage = errors.New("split would create empty page")
    ErrPageLimitExceeded         = errors.New("page limit exceeded")  // 既存 domain.ErrPageLimitExceeded を流用可
)

func (u *SplitPage) Execute(ctx context.Context, in SplitPageInput) (SplitPageOutput, error) {
    var view EditPhotobookView
    err := database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
        repo := rdb.NewPhotobookRepository(tx)
        // 1. version+1 + draft 確認 (1 度きり、累積 +1 にしない)
        if err := repo.BumpVersion(ctx, in.PhotobookID, in.ExpectedVersion, in.Now); err != nil {
            return err  // ErrOptimisticLockConflict
        }
        // 2. 現在 page 数取得 + 30 上限確認
        pageCount, err := repo.CountPagesByPhotobookID(ctx, in.PhotobookID)
        if err != nil { return err }
        if pageCount >= domain.MaxPagesPerPhotobook { return ErrPageLimitExceeded }
        // 3. source page の photo を全取得
        photos, err := repo.ListPhotosByPageID(ctx, in.SourcePageID)
        if err != nil { return err }
        // 4. 切断点 photo の index を特定
        splitIdx := -1
        for i, ph := range photos {
            if ph.ID() == in.SplitAtPhotoID { splitIdx = i; break }
        }
        if splitIdx == -1 { return ErrPhotoNotFound }
        // 5. 切断点 photo が末尾なら拒否
        if splitIdx == len(photos)-1 { return ErrSplitWouldCreateEmptyPage }
        // 6. 新 page を sourcePage.displayOrder + 1 に挿入
        sourcePage, err := repo.FindPageByID(ctx, in.SourcePageID)
        if err != nil { return err }
        newPageOrder := sourcePage.DisplayOrder().Int() + 1
        // 6a. 後続 page を 1 つ後ろに shift (BulkOffset で +1000 → 順次 -999)
        // ... pages[displayOrder >= newPageOrder] を +1
        // 6b. 新 page INSERT
        // 7. 切断点以降の photo の page_id / display_order を新 page に
        // 8. 元 page の display_order を再採番 (もし gap が出たら)
        // 9. 更新後 EditView を取得
        // ... GetEditView を内部呼出 (TX 共有はせず、TX commit 後に呼出 or repo.ListXxx で組み立て)
        return nil
    })
    if err != nil { return SplitPageOutput{}, err }
    return SplitPageOutput{View: view}, nil
}
```

> 細部 (display_order shift の SQL 順序) は STOP P-2 で SQL test と一緒に詰める。

### 11.2 MovePhotoBetweenPages (§3.4.3)

```go
// repository method (新規)
func (r *PhotobookRepository) MovePhotoBetweenPages(
    ctx context.Context,
    photobookID photobook_id.PhotobookID,
    photoID photo_id.PhotoID,
    sourcePageID, targetPageID page_id.PageID,
    targetDisplayOrder int,
    expectedVersion int,
    now time.Time,
) error {
    // 1. version+1 + draft 確認 (BumpVersion)
    // 2. 同 page 移動なら BulkReorderPhotosOnPage と等価
    // 3. 別 page 移動:
    //    a. source page から photo を抜く: BulkOffsetPhotoOrdersOnPage(sourcePageID) 不要、直接
    //       UPDATE photobook_photos SET display_order = display_order - 1
    //         WHERE page_id = sourcePageID AND display_order > <photo の現 order>
    //    b. target page で +1000 escape: BulkOffsetPhotoOrdersOnPage(targetPageID) → ただし
    //       これは全 photo を +1000 するので 6 回の UPDATE で済む。代わりに
    //       UPDATE photobook_photos SET display_order = display_order + 1
    //         WHERE page_id = targetPageID AND display_order >= targetDisplayOrder
    //       で UNIQUE 衝突回避するため +1000 escape を使う
    //    c. UpdatePhotobookPhotoPageAndOrder(photoID, targetPageID, targetDisplayOrder)
    //    d. target の +1000 photo を順次戻す
}
```

### 11.3 編集 mutation の error 共通変換 (handler 層)

`writeEditMutationError` (既存) を拡張、新 reason code:

```go
case errors.Is(err, usecase.ErrPageLimitExceeded):
    writeJSONStatus(w, http.StatusConflict, map[string]any{
        "status": "version_conflict",
        "reason": "page_limit_exceeded",
    })
case errors.Is(err, usecase.ErrSplitWouldCreateEmptyPage):
    writeJSONStatus(w, http.StatusConflict, map[string]any{
        "status": "version_conflict",
        "reason": "split_would_create_empty_page",
    })
case errors.Is(err, usecase.ErrMergeIntoSelf):
    writeJSONStatus(w, http.StatusConflict, map[string]any{
        "status": "version_conflict",
        "reason": "merge_into_self",
    })
case errors.Is(err, usecase.ErrCannotRemoveLastPage):
    writeJSONStatus(w, http.StatusConflict, map[string]any{
        "status": "version_conflict",
        "reason": "cannot_remove_last_page",
    })
```

> reason 文言は publish 系の reason enum と区別する。混同 / 取り違え防止のため authenticated owner 向け reason として独立。

## 12. Phase B (今回スコープ外、参考)

| 項目 | 概要 | depends |
|---|---|---|
| Page meta 編集 UI | event_date / world / cast_list / photographer / note の入力 | Backend 既存 (UpsertPageMeta repo + sqlc query 既出) → handler 追加のみ |
| Public viewer の page_meta 反映 | `/api/public/photobooks/{slug}` の response に page_meta を埋める | get_public_photobook usecase 拡張 |
| /p の PageMeta コンポ表示 | 既に実装済 (silent skip)、Backend 拡張で自動表示 | (なし) |
| photo drag & drop で page 跨ぎ移動 | Phase A の dropdown に追加 | (なし) |
| page drag & drop reorder | Phase A の隣接 swap に追加 | (なし) |
| Sensitive flag 編集 | Backend / Frontend 双方で field 追加 | (大改修、別 STOP) |

## 13. 参照

- `m2-frontend-edit-ui-fullspec-plan.md` — PR27 の /edit fullspec
- `m2-photobook-image-connection-plan.md` — PR19 の Page / Photo / PageMeta 設計
- `docs/design/aggregates/photobook/データモデル設計.md` §4 / §5 / §6
- `docs/design/aggregates/photobook/ドメイン設計.md` §3.2
- `.agents/rules/domain-standard.md` — 集約子テーブル / 親 version OCC / display_order 連続性
- `.agents/rules/publish-precondition-ux.md` — authenticated owner 経路の reason 分離原則
- `.agents/rules/cors-mutation-methods.md` — 新 PATCH / POST endpoint 追加時の CORS test
- `.agents/rules/client-vs-ssr-fetch.md` — Frontend mutation の `credentials: "include"` 必須

## 14. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-06 | **v1**: STOP P-α 仕様確定資料初版。DB migration 不要 (既存 schema 完備) を発見。実装順は split/move/caption 先行 (user 調整) |
