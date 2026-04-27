# PR28 publish flow 完成 実装結果（2026-04-27）

## 概要

- 新正典 [`docs/plan/vrc-photobook-final-roadmap.md`](../../docs/plan/vrc-photobook-final-roadmap.md)
  §3 PR28 / 計画書 [`docs/plan/m2-frontend-edit-ui-fullspec-plan.md`](../../docs/plan/m2-frontend-edit-ui-fullspec-plan.md)
  §10 に従い、PR27 で disabled placeholder だった「公開へ進む」ボタンを実機能化
- 既存 `PublishFromDraft` UseCase を HTTP 化 / Frontend に publish API client +
  Complete 画面を追加
- Cloud Run / Workers 両方 deploy 完了
- これで **作成 → 編集 → 公開 → 閲覧 → 管理** の MVP 導線が一通り通る

## Backend publish endpoint 実装内容

- **`POST /api/photobooks/{id}/publish`**（draft Cookie 必須）
- 既存 `PublishFromDraft` UseCase の HTTP 化
- request body: `{ "expected_version": <int> }`
- response: `{ photobook_id, slug, public_url_path, manage_url_path, published_at }`
  - `manage_url_path` は **raw token を含む**（`/manage/token/<raw>` 形式）
  - 業務知識 v4 に従い **再表示禁止**。body 経由 1 回のみ伝送し、log / Set-Cookie /
    DB 永続化はしない
- OCC 違反 / 状態不整合 / 不変条件違反は **409 version_conflict** に集約
  - `ErrPublishConflict` / `ErrOptimisticLockConflict` / `ErrNotDraft` /
    `ErrRightsNotAgreed` / `ErrEmptyTitle` / `ErrEmptyCreatorName` 全て 409
- Cache-Control: no-store / X-Robots-Tag: noindex,nofollow

### Backend test 結果（実 DB 統合、5 件 pass）

- 正常 publish（response field 検証 + token hash / draft token などが含まれない確認）
- 既に published になった photobook の re-publish → 409
- version 不一致 → 409
- 不正 UUID → 404
- 不正 body → 400

## Frontend publish flow 実装内容

### lib/publishPhotobook.ts

- `publishPhotobook(photobookId, expectedVersion)` API client
- エラー種別: `unauthorized` / `not_found` / `bad_request` / `version_conflict` /
  `server_error` / `network`
- snake_case → camelCase 変換
- 11 件 unit test pass（200 / 各エラー / network）

### EditClient 統合

- `publishResult` を Client state で保持（**URL 遷移しない**）
- `publishResult !== null` のとき `<CompleteView />` に切替
- 公開ボタンの disabled 条件:
  - 写真 0 枚 → `"公開には最低 1 枚の写真が必要です。"`
  - processing 件数 > 0 → `"処理中の写真があります。完了してから公開してください。"`
  - 設定 dirty → `"変更を保存してから公開してください。"`
- 409 conflict 時は EditClient の既存 conflict バナー経由で reload 誘導
- 401 / network は ErrorMsg 表示

### components/Complete/

- `UrlCopyPanel.tsx`: 公開 URL（teal）/ 管理 URL（violet）のコピーパネル。
  `navigator.clipboard.writeText` 経由
- `ManageUrlWarning.tsx`: 「再表示できません」警告（業務知識 v4）。
  スクリーンショット保存を促す
- `CompleteView.tsx`: 完了画面本体。
  - 公開 URL コピー / 管理 URL コピー（warning 付き）
  - 「公開ページを開く」（外部 link、target=\_blank）
  - 「編集ページに戻る」（draft session は publish 時に revoke される設計のため、
    ボタンはトップ `/` への遷移にとどめる）

## Complete 画面実装内容

design 参照: `screens-a.jsx` Complete / `pc-screens-a.jsx` PCComplete / `shared.jsx` UrlRow。
prototype は値の抽出元として使用、直接 import せず Tailwind クラスで再現。

| 要素 | 実装 |
|---|---|
| ヘッダ | 「公開完了」バッジ + 「フォトブックを公開しました」 |
| 公開 URL | `UrlCopyPanel kind="public"` （teal）+ helper "VRChat やフレンドに共有可能" |
| 管理 URL | `ManageUrlWarning` + `UrlCopyPanel kind="manage"`（violet）|
| アクション | 「公開ページを開く」（teal CTA、新タブ）/ 「編集ページに戻る」 |

## URL コピー実装内容

- `navigator.clipboard.writeText` で 1 クリックコピー
- 結果は state で 3 秒間「コピーしました」/ 「失敗」を表示後 `idle` に戻る
- 失敗時は値を含むエラー文を出さない（汎用「失敗」のみ）
- URL 値そのものは画面に表示するが console.log しない

## Viewer visual Safari 確認結果（残課題の状況）

PR25b / PR27 で残っていた「**実画像を含む `/p/[slug]` の完全 visual Safari 確認**」は、
本 PR の機能 smoke レベルでは **未完了**。理由と対応:

### 本 PR で確認済（機能 smoke、curl）

- `/p/<bogus>` → 404 + `noindex` ヘッダ + meta + Cache-Control: no-store
- `/edit/<bogus>` no Cookie → 200 SSR + `ErrorState(unauthorized)` レンダリング
- `/api/photobooks/<bogus>/publish` no Cookie → 401
- Cloud Run logs / Workers logs に Secret / token 漏洩なし

### 本 PR で **未実施**（manual / 実機 Safari）

- 実画像 upload → image-processor → publish → `/p/[slug]` で display 画像表示
- macOS Safari / iPhone Safari の visual 確認

### 理由

- 完全 e2e visual 確認には次が必要:
  1. 実 Safari で `/draft/<token>` → `/edit/<id>` への Cookie 経路
  2. 実画像（JPEG/PNG/WebP）の R2 アップロード
  3. image-processor の Cloud Run Jobs（PR31 未実装、ローカル CLI 経由）or 手動実行
  4. publish ボタン押下 → CompleteView 確認
  5. `/p/[slug]` を Safari で開いて画像表示確認
- このうち 1 / 2 / 4 / 5 は実 Safari の操作が必須。Claude 側からは 3 のみ実行可能

### 推奨次手順（ユーザー側 manual、本 PR の deploy 直後でも実施可能）

1. 既存運用の draft URL（または新規発行）で macOS Safari → `/draft/<token>` →
   `/edit/<id>` 着地
2. 実画像をアップロード（PR22 の upload UI 経由）
3. 数分待つか、ローカルで `cloud-sql-proxy + image-processor --all-pending` を実行して
   image を `available` 化
4. edit ページに戻り photo grid 表示確認
5. 「公開へ進む」 → CompleteView 表示 → 公開 URL コピー
6. 公開 URL を新タブで Safari で開いて display 画像表示確認
7. iPhone Safari でも同様に確認
8. 結果を本 work-log の §「Viewer visual Safari 確認結果」に追記

> 上記は **Cloud Run Jobs（PR31）が未稼働の現状でも実施可能**。
> image-processor はローカル CLI（PR23 で実装済）から `--all-pending` で動かせる。

## Manage 確認結果

- `/api/manage/photobooks/<bogus>` no Cookie → 401（PR25 動作維持、PR28 で変更なし）
- 実 Safari での Cookie 経路確認は PR17（過去）で完了済み
- manage URL の受け渡しは CompleteView の `UrlCopyPanel kind="manage"` + `ManageUrlWarning`
  でユーザーに 1 回だけ提示、再表示しない

## Deploy 結果

### Backend

- image: `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb/vrcpb-api:3ec5080`
- digest: `sha256:80ac46eb386dd5b21a675bb94aaf5c5ad72a788548dc99782d3fa16cae2cf53f`
- revision: `vrcpb-api-00010-7vz`（100% traffic）
- rollback: `vrcpb-api-00009-wdb`（image `vrcpb-api:2a93f8c`）
- env / secret refs は `--image=` update で不変
- smoke: /health 200 / /readyz 200 / publish no Cookie → 401

### Frontend

- `npm run cf:build` → `npx wrangler deploy`
- Workers Version: `5222b7eb-f334-417b-aa23-1e1421afa08e`
- Custom Domain: `app.vrc-photobook.com`
- smoke: `/edit/<bogus>` 200 + ErrorState(unauthorized) / `/p/<bogus>` 404 / noindex 出力

## test 結果

### Backend

- PR28 新規 5 件（publish handler）pass
- 既存 photobook / imageupload / imageprocessor すべて pass
- `go vet` / `go build` クリーン

### Frontend

- 76 件 pass（publishPhotobook 11 件 + 既存 65 件）
- `typecheck` / `build` / `cf:build` 全クリーン

## Secret 漏洩なし

`grep -RInE "DATABASE_URL=|PASSWORD=|SECRET=|SECRET_KEY|API_KEY|sk_live|sk_test|draft_edit_token=|manage_url_token=|session_token=|R2_SECRET_ACCESS_KEY=|TURNSTILE_SECRET_KEY="`
を Backend / Frontend の本実装コードに対して実施 → **0 件**。

ヒットは pre-existing test docstring の localhost dev DSN（公開値）のみ。

raw token / Cookie 値 / presigned URL / R2 credentials / DATABASE_URL / 完全な
manage URL の **実値**は、本 work-log / commit メッセージ / curl 出力 /
Cloud Run logs のいずれにも含まれていない。

## 実施しなかったこと（PR28 範囲外）

- Outbox（PR30）/ outbox-worker（PR31）
- SendGrid（PR32）
- OGP 自動生成（PR33）
- manage URL 再発行 / 再送（PR32）
- Moderation（PR34）/ Report（PR35）/ UsageLimit（PR36）
- LP（PR37）/ Public repo 化（PR38）
- Cloud SQL 削除（PR39）/ spike 削除（PR40）
- Cloud Run Jobs / Scheduler 作成
- drag & drop reorder（PR41+）
- **実画像を含む完全 visual Safari 確認**（実機操作、ユーザー側 manual で実施）

## 次にやること

- ユーザー側で **実画像 upload → publish → Safari で /p/[slug] 視覚確認**を実施
  （上記「推奨次手順」に従う）
- その後、新正典 §3 PR29: **Backend deploy 自動化（Cloud Build）**へ進む

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-27 | PR28 完了。Backend + Frontend を 1 commit で実装 + deploy（commit `3ec5080`） |
