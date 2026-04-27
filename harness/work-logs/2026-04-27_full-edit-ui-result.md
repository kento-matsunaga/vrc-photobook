# PR27 編集 UI 本格化 実装結果（2026-04-27）

## 概要

- 新正典 [`docs/plan/vrc-photobook-final-roadmap.md`](../../docs/plan/vrc-photobook-final-roadmap.md)
  §3 PR27 / 計画書 [`docs/plan/m2-frontend-edit-ui-fullspec-plan.md`](../../docs/plan/m2-frontend-edit-ui-fullspec-plan.md)
  に従い、`/edit/[photobookId]` を upload 専用 UI から **本格編集 UI** へ拡張した
- スコープが大きいため **PR27a Backend / PR27b Frontend** に分割実装
- Cloud Run / Workers 両方を deploy 完了。功能 smoke は curl で確認

## PR27a Backend（commit `2a93f8c`）

### 新規 endpoint（すべて draft Cookie 必須）

| method | path | 説明 |
|---|---|---|
| GET    | /api/photobooks/{id}/edit-view | 編集画面初期データ（pages + photos + variant URLs） |
| PATCH  | /api/photobooks/{id}/settings | settings 一括 PATCH |
| POST   | /api/photobooks/{id}/pages | ページ追加 |
| DELETE | /api/photobooks/{id}/pages/{pageId} | ページ削除 |
| PATCH  | /api/photobooks/{id}/photos/reorder | 同 page 内 reorder（一括） |
| PATCH  | /api/photobooks/{id}/photos/{photoId}/caption | photo caption 編集 |
| DELETE | /api/photobooks/{id}/photos/{photoId} | photo 削除 |
| PATCH  | /api/photobooks/{id}/cover-image | cover 設定 |
| DELETE | /api/photobooks/{id}/cover-image | cover クリア |

### 新規 UseCase / Repository / SQL

- `GetEditView` / `UpdatePhotoCaption` / `BulkReorderPhotosOnPage` / `UpdatePhotobookSettings`
- Repository: `UpdatePhotoCaption` / `BulkReorderPhotosOnPage` / `UpdateSettings`
- SQL: `BulkOffsetPhotoOrdersOnPage` / `UpdatePhotobookPhotoCaption` / `UpdatePhotobookSettings`
- 既存 UseCase（`AddPage` / `RemovePage` / `RemovePhoto` / `SetCoverImage` / `ClearCoverImage`）を HTTP 化

### OCC 方針

- すべて `status='draft' AND version=$expected` で OCC（既存 `bumpVersion` helper を流用）
- 0 行 UPDATE / Image owner 不一致 / 状態不整合は **409 version_conflict** に集約
- 失敗詳細は外部に漏らさない（draft 以外 / version 不一致 / 不存在を区別しない）

### Reorder 方式（計画書 §5.4 方式 A）

- 同 page 全 photo の `display_order` を **+1000 一時退避**
- 各 photo を新 `display_order` に順次 UPDATE
- すべて同一 TX で実行、UNIQUE 衝突を回避

### Test（実 DB 統合、9 件 pass）

- `TestUpdatePhotoCaption`: 正常 / OCC conflict
- `TestBulkReorderPhotosOnPage`: 2 photo swap / OCC conflict
- `TestUpdatePhotobookSettings`: 正常 / OCC conflict / published 化後の編集禁止
- `TestGetEditView`: 正常（display URL 含む） / published で `ErrEditNotAllowed` /
  不存在で `ErrEditPhotobookNotFound`

## PR27b Frontend（本コミット）

### Frontend route / component

| ファイル | 役割 |
|---|---|
| `frontend/app/(draft)/edit/[photobookId]/page.tsx` | Server Component（edit-view fetch、Cookie 転送、エラー → ErrorState） |
| `frontend/app/(draft)/edit/[photobookId]/EditClient.tsx` | Client orchestrator（caption / reorder / cover / settings / upload / 409 reload / 5 秒 polling） |
| `frontend/components/Edit/PhotoGrid.tsx` | photo 一覧表示 + caption / reorder / cover 操作 / 削除 |
| `frontend/components/Edit/CaptionEditor.tsx` | blur 保存 + 200 runes 制限 + 保存ステータス |
| `frontend/components/Edit/ReorderControls.tsx` | 上下ボタン（先頭 / 1↑ / 1↓ / 末尾） |
| `frontend/components/Edit/CoverPanel.tsx` | cover preview + クリアボタン |
| `frontend/components/Edit/PublishSettingsPanel.tsx` | settings 編集 + 「公開へ進む」 disabled placeholder |
| `frontend/lib/editPhotobook.ts` | edit-view 取得 + 各 mutation API client |

### 削除

- `frontend/app/(draft)/edit/[photobookId]/UploadClient.tsx`（PR22 旧編集 UI、本実装で置換）
  upload 経路は EditClient 内に統合（既存 `lib/upload.ts` を流用）

### Reorder 実装

- 計画書 §7 案 C 採用: PR27 は **上下ボタン**のみ
- Client は array reorder で楽観 update → `bulkReorderPhotos` API を発火
- 失敗時は 409 conflict バナーを出して reload 誘導

### Caption 保存

- 計画書 §8 案 A 採用: blur で自動保存
- 200 runes 上限（runeCount で確認）
- 保存ステータス: 変更なし / 保存中 / 保存しました / 保存失敗
- 上限超過時は保存しない

### Cover 設定

- PhotoGrid の各 photo に「coverに設定」「coverを外す」ボタン
- CoverPanel で現在の cover thumbnail と「表紙をクリア」ボタン
- 設定後は `reload()` で variant URL を再取得

### Publish settings

- type / title / description / layout / opening_style / visibility / cover_title 編集
- 「公開へ進む」ボタンは **disabled placeholder**（PR28 で実機能化）
- 設定保存は明示ボタン経由（caption と異なり一括）

### Processing / Failed 件数

- edit-view 応答の processingCount / failedCount を表示
- processingCount > 0 のとき **5 秒 polling** で edit-view 再取得（計画書 §6.4）

### 409 conflict 処理

- 各 mutation 発火時に 409 を受けると `conflict='conflict'` state に遷移
- 上部に「最新を取得」バナー表示
- ユーザーがボタンを押すと `reload()` で edit-view 再取得 + version 同期

## Backend deploy 結果

- image: `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb/vrcpb-api:2a93f8c`
- digest: `sha256:30b117b961cbd66e8ed6a249c4c1836102ef830995e327b239ad2b9e420db741`
- revision: `vrcpb-api-00009-wdb`（100% traffic）
- rollback: `vrcpb-api-00008-qfk` (`vrcpb-api:db9dd5a`)

### Smoke

```
GET /health                                                       200 ok
GET /readyz                                                       200 ready
GET /api/photobooks/<bogus-uuid>/edit-view  (no Cookie)           401 unauthorized
PATCH /api/photobooks/<bogus-uuid>/settings  (no Cookie)          401 unauthorized
```

env / secret refs（DATABASE_URL / R2_* / TURNSTILE_SECRET_KEY）は `--image=` update で不変。

## Frontend deploy 結果

- `npm run cf:build` → `npx wrangler deploy`
- Workers Version: `c3f9212f-2942-4fb7-83e4-eea36c819360`
- Custom Domain: `app.vrc-photobook.com` 経由で配信

### Smoke

```
GET https://app.vrc-photobook.com/edit/<bogus-uuid>  (no Cookie)
   → HTTP 200 SSR + ErrorState(unauthorized)「アクセスできません」がレンダリング
   → noindex メタ + X-Robots-Tag noindex
GET https://app.vrc-photobook.com/p/notexistslug12345
   → HTTP 404（PR25b 動作維持）
```

Cloud Run logs に raw token / Cookie / presigned URL / R2 credentials の漏洩なし。

## Test 結果

### Backend

- PR27a 新規 9 件（edit_extras_test.go）pass
- 既存 photobook / imageupload / imageprocessor の test も全 pass
- `go vet ./...` / `go build ./...` クリーン

### Frontend

- 69 件 pass（editPhotobook 14 件 + 既存 55 件）
- `typecheck` / `build` / `cf:build` 全てクリーン

## Safari 確認

- macOS Safari / iPhone Safari の **機能 smoke** はユーザー側で実施想定
- curl レイヤで以下を担保:
  - SSR が Workers 経由で正しい HTML / ヘッダ / metadata を返す
  - 401 → ErrorState(unauthorized) 変換
  - `noindex, nofollow` が middleware（HTTP ヘッダ）+ page metadata（HTML meta）両方に出力

## PR25b 残課題（Viewer visual 確認）の状況

PR25b で残った「画像表示を含む `/p/[slug]` の完全 visual Safari 確認」は本 PR では
**実機未実施**。理由:

- 完全 visual 確認には「実画像 upload → image-processor で variant 生成 → publish」を
  e2e で実施する必要がある
- そのためには本番 R2 / Turnstile / draft Cookie 経路を実 Safari で動かして実
  photobook を作成する必要がある（手元の curl では再現できない）
- 本 PR の範囲外の操作のため、PR28 publish flow 完成 と合わせて **次サイクルで
  ユーザー側 manual Safari 確認**として実施するのが自然

なお functional パスは PR27b でカバー済（404 / 401 / SSR / metadata / Workers
経由配信）。

## 実施しなかったこと（PR27 範囲外）

- publish 本実行 / 完了画面 / URL コピー / manage URL 再発行（PR28 / PR32）
- drag & drop reorder（PR41+）
- OGP 自動生成（PR33）/ Outbox（PR30）/ SendGrid（PR32）
- Moderation（PR34）/ Report（PR35）/ UsageLimit（PR36）
- LP（PR37）/ Public repo 化（PR38）/ Cloud SQL 削除（PR39）
- spike 削除（PR40）
- 画像表示の完全 visual Safari 確認（実機 e2e、PR28 と統合して manual 実施）
- Page caption（photobook_pages.caption）の編集 UI（時間切れで PR41+ に送る）
- UpsertPageMeta の HTTP 化（同上）

## Secret 漏洩なし

`grep -RInE "DATABASE_URL=|PASSWORD=|SECRET=|SECRET_KEY|API_KEY|sk_live|sk_test|draft_edit_token=|manage_url_token=|session_token=|R2_SECRET_ACCESS_KEY=|TURNSTILE_SECRET_KEY="`
を `frontend/{app,components,lib}` および `backend/internal/photobook` に対して実施。

ヒットは pre-existing test docstring の localhost dev DSN のみ（`postgres://vrcpb:vrcpb_local@localhost:5432/`、開発用 docker-compose で公開値）。実 Secret は **0 件**。

curl 出力 / Cloud Run logs / Workers logs にも raw token / Cookie / presigned URL /
R2 credentials / DATABASE_URL の漏洩なし。

## 次にやること

- **PR28 publish flow 完成**へ進む
  - 「公開へ進む」ボタンの実機能化（既存 `PublishFromDraft` UseCase の HTTP 化）
  - 完了画面 / URL コピー / manage URL 控え
  - 実画像 + publish の visual Safari 確認（PR25b 残課題と統合）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-27 | PR27 完了。Backend (PR27a `2a93f8c`) + Frontend (PR27b 本コミット) |
