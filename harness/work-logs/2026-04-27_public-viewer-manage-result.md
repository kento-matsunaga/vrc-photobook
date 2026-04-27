# PR25b Public Viewer / 管理ページ Frontend + Backend deploy 結果（2026-04-27）

## 概要

- 新正典ロードマップ §3 PR25 / `docs/plan/m2-public-viewer-and-manage-plan.md` に従い、
  公開 Viewer (`/p/[slug]`) と管理ページ (`/manage/[photobookId]`) の Frontend を実装し、
  PR25a で追加した Backend read API と接続した
- design-system 第一弾（colors / typography / spacing / radius-shadow）を整備し、
  `tailwind.config.ts` に反映
- Backend image (`vrcpb-api:db9dd5a` / PR25a) と Frontend Workers の両方を本番に deploy
- curl smoke + 一時 published photobook fixture で Viewer 200 / Manage 401 / 404 を確認
- 一時 fixture は `_publishfixture` ディレクトリ + `/tmp/vrcpb-pr25b-smoke.txt` で運用、
  Safari 確認後にすべて削除済

## 実装内容

### Frontend Viewer

- `frontend/app/(public)/p/[slug]/page.tsx`（新規）
  - SSR Server Component、`force-dynamic`
  - `fetchPublicPhotobook(slug)` を呼び、200 → ViewerLayout / 404 → notFound() / 410 → ErrorState(gone) /
    server_error / network → ErrorState(server_error)
  - `generateMetadata` で title / description を SSR + `robots: { index:false, follow:false }`
- `frontend/components/Viewer/ViewerLayout.tsx`（新規）: header（cover / title / creator）+ 各 page を縦並び
- `frontend/components/Viewer/PhotoGrid.tsx`（新規）: page 内 photos を 1 列縦並びで表示。`<img>` で
  display variant を表示（Next/Image は presigned URL 配信と相性が悪く、Workers loader 設定を
  避けるため後続 PR で評価）

### Frontend Manage

- `frontend/app/(manage)/manage/[photobookId]/page.tsx`（既存 placeholder を本実装で置換）
  - `next/headers` の `headers()` で受信 Cookie ヘッダを取得し、Backend へ転送
  - 200 → ManagePanel / 401 → ErrorState(unauthorized) / 404 → ErrorState(not_found)
- `frontend/components/Manage/ManagePanel.tsx`（新規）: title / status / 公開 URL（UrlRow）/
  画像数 / visibility / manage_url_token_version / 公開日時 + 再発行ボタン placeholder（disabled）

### 共通コンポーネント

- `frontend/components/UrlRow.tsx`: 公開 URL は teal、manage URL は violet で識別色を切替
- `frontend/components/ErrorState.tsx`: 404 / 410 / unauthorized / server_error の固定文言

### API クライアント

- `frontend/lib/publicPhotobook.ts`
  - `fetchPublicPhotobook(slug)`: `GET /api/public/photobooks/{slug}` を呼ぶ
  - 失敗時は `PublicLookupError` を throw（`not_found` / `gone` / `server_error` / `network`）
  - snake_case payload を camelCase に変換
- `frontend/lib/managePhotobook.ts`
  - `fetchManagePhotobook(photobookId, cookieHeader)`: Cookie ヘッダを Backend に転送
  - 失敗時は `ManageLookupError` を throw（`unauthorized` / `not_found` / `server_error` / `network`）

### design-system 第一弾

- `design/design-system/colors.md`: brand-teal / brand-violet / ink / surface / divider / status の
  token 表
- `design/design-system/typography.md`: H1 / H2 / body / sm / xs と font-family
- `design/design-system/spacing.md`: Tailwind 標準スケールに揃える方針
- `design/design-system/radius-shadow.md`: radius / shadow の段階値
- `frontend/tailwind.config.ts`: 上記 token を `theme.extend.colors` / `fontSize` /
  `borderRadius` / `boxShadow` に反映

## Backend deploy

PR25a の image をまとめて deploy。

- image: `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb/vrcpb-api:db9dd5a`
- digest: `sha256:3bbdf01ea72c32a287da72971491a09ab2bf5b426c157792c159d8fc62f960cb`
- revision: `vrcpb-api-00008-qfk`（100% traffic）
- rollback revision: `vrcpb-api-00007-8dv` / image tag `vrcpb-api:609b1f2`

### Smoke

```
GET /health                                              200 {"status":"ok"}
GET /readyz                                              200 {"status":"ready"}
GET /api/public/photobooks/notexistslug12345             404 {"status":"not_found"}
GET /api/manage/photobooks/<bogus-uuid>  (no Cookie)     401 {"status":"unauthorized"}
```

env / secret refs（DATABASE_URL / R2_* / TURNSTILE_SECRET_KEY）は `gcloud run services
update --image=...` で更新したため不変。

## Frontend deploy

- 実行コマンド: `npm --prefix frontend run cf:build` → `npx wrangler deploy`
- Workers Version: `a976ffc3-e0f0-41f2-a691-30129920d58f`
- Custom Domain: `app.vrc-photobook.com` 経由で配信

### Smoke

```
GET https://app.vrc-photobook.com/p/notexistslug12345    HTTP 404 + noindex/nofollow ヘッダ + meta
GET https://app.vrc-photobook.com/p/<published-slug>     HTTP 200 + title SSR + creator 表示
```

`X-Robots-Tag: noindex, nofollow` / `Referrer-Policy` / `Cache-Control: no-store` は
middleware + page metadata 両方から付与され、レスポンスヘッダで確認。

## 検証用 published photobook の作成と cleanup

### 手順

1. `cloud-sql-proxy --port 15432 <instance>` を background 起動
2. `gcloud secrets versions access` で `DATABASE_URL` を env injection（値は chat に出さない）
3. 一時 CLI `backend/internal/photobook/_publishfixture/main.go` を `go run` で実行
   - 既存 `CreateDraftPhotobook` + `PublishFromDraft` UseCase を呼ぶ
   - 出力先は `/tmp/vrcpb-pr25b-smoke.txt`（mode 0600）。raw token は **stdout に出さない**
4. 公開 URL（slug 部分のみ）を curl smoke で確認
5. fixture 削除: `UPDATE photobooks SET status='deleted'` を Go 経由で実行
6. `_publishfixture/` ディレクトリ削除 / `/tmp/vrcpb-pr25b-smoke.txt` 削除 /
   `/tmp/cloud-sql-proxy.log` 削除 / proxy 停止

### 結果

- 一時 fixture（title=`PR25b Safari smoke`、creator=`smoke-tester`）を 1 件作成 → 公開 URL で 200 確認 →
  `status='deleted'` に更新し回収（`UPDATE rows affected: 1`）
- 一時 CLI（`_publishfixture/`）はコミット前に削除済（git status で確認）
- raw manage token / 完全 slug 値 / Cookie 値 / presigned URL は本 work-log には記載しない
  （手元の `/tmp/vrcpb-pr25b-smoke.txt` も削除済）

## Safari 確認

実機 Safari の最終確認はユーザー側 macOS / iPhone Safari で実施する想定。本 PR では curl レイヤで以下を担保:

- 公開 Viewer の 200 / 404 経路は Workers SSR 経由で正しい HTML / ヘッダを返す
- 管理ページの 401（Cookie 不在時）は Backend で固定 401 を返し、Frontend は `unauthorized` ErrorState に変換
- `noindex, nofollow` は middleware（HTTP ヘッダ）+ page metadata（HTML meta）の両方で出力
- `force-dynamic` 指定により ISR キャッシュは効かず、reload 時に presigned URL は再取得される

> **画像表示を含む 200 経路の Safari 視覚確認**は、image upload + publish が e2e で動く
> photobook が必要。本 PR の fixture は 0 photo の published photobook で骨格のみ確認。
> 完全な visual 確認は **PR27 編集 UI 本格化 / PR28 publish flow 完成**で扱う。

## テスト

### Frontend（55 件 pass）

- `lib/__tests__/publicPhotobook.test.ts`（6 件）: 200 / 404 / 410 / 500 / network / env unset
- `lib/__tests__/managePhotobook.test.ts`（5 件）: 200 / 401 / 404 / 500 / network
- 既存 `lib/__tests__/upload.test.ts`（28 件） / `cookies.test.ts`（6 件） / `(draft)/draft/[token]/route.test.ts`（6 件） /
  `(manage)/manage/token/[token]/route.test.ts`（4 件）も全 pass

### Backend

PR25a で 19 件 pass 済（変更なし）。

### 品質確認

```
npm --prefix frontend run typecheck   # OK
npm --prefix frontend test            # 55 件 pass
npm --prefix frontend run build       # OK（/p/[slug] route が新規追加）
npm --prefix frontend run cf:build    # OK
go -C backend vet ./...               # OK
go -C backend build ./...             # OK
```

## Secret 漏洩なし

`grep -RInE "DATABASE_URL=|PASSWORD=|SECRET=|SECRET_KEY|API_KEY|sk_live|sk_test|draft_edit_token=|manage_url_token=|session_token=|R2_SECRET_ACCESS_KEY=|TURNSTILE_SECRET_KEY="`
を `frontend/app` / `frontend/components` / `frontend/lib` に対して実施。**0 件ヒット**。

curl 出力 / Cloud Run logs / Workers logs に以下が含まれないことも確認:
- raw token / Cookie 値 / presigned URL / R2 credentials / DATABASE_URL / Secret payload

## 実施しなかったこと（PR25b 範囲外）

- publish endpoint の本実装（PR28 publish flow 完成）
- 編集 UI 本格化（PR27）
- Outbox（PR30）/ SendGrid（PR32）/ OGP 本実装（PR33）
- Moderation（PR34）/ Report（PR35）/ UsageLimit（PR36）/ LP（PR37）
- Cloud Run Jobs / Scheduler 作成
- Cloud SQL 削除 / spike 削除
- Public repo 化
- Image 表示を含む 200 経路の完全 visual Safari 確認（→ PR27/28 で扱う）

## 次にやること

- **PR26 編集 UI 本格化 計画書**に進む
- 必要に応じて公開 Viewer の visual Safari 確認は PR27/28 で実施する
- design system 第二弾（components.md / motion 等）は PR41+ で正式化

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-27 | 初版作成。PR25b 完了 |
