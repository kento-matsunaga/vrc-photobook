# /edit client reload が SSR 用 fetch を使い Cookie なし 401 になる

## 発生日

2026-05-03 STOP α 調査で発見（実装は PR27 で /edit に reload 経路が入った時から）。

## 症状

`/edit/<photobookId>` で「最新を取得」ボタンを押すと「再取得に失敗しました（unauthorized）。ページを再読み込みしてください」。さらに polling（processingCount > 0 時の 5 秒間隔）でも同じ 401 を量産していた。

Cloud Run logs `/edit-view` で `14 GET 401 / 9 GET 200`。SSR 経由（page.tsx）の 200 は通るが、client polling / reload 由来の 401 が圧倒的多数。

事故クラス: **Client Component から authenticated cross-origin API を呼ぶときに、SSR 専用の Cookie ヘッダ手動転送 fetch を使ってしまう**。

## 根本原因

`frontend/app/(draft)/edit/[photobookId]/EditClient.tsx` line 108:
```ts
const next = await fetchEditView(view.photobookId, "");
```

`fetchEditView` は **SSR 専用**。`cookieHeader === ""` の場合 `headers: {}` を渡し、`credentials` 指定なし → ブラウザ既定 `same-origin` で cross-origin 先 (`api.vrc-photobook.com`) には Cookie を送らない → Backend session middleware が 401。

β-3 で `fetchEditViewClient`（`credentials: "include"`）が `lib/editPhotobook.ts` に追加されていたが、`/edit` の `reload()` は古い `fetchEditView` のまま放置。`/prepare` の β-3 polling は新関数に切り替わっていたので片手落ちだった。

## 修正

`frontend/app/(draft)/edit/[photobookId]/EditClient.tsx` (a8fe0db):
- import 行: `fetchEditView` → `fetchEditViewClient`
- `reload()` 内: `fetchEditViewClient(view.photobookId)` 経路に切替
- 経緯コメントを reload 直前に明記

## 追加した test

`frontend/app/(draft)/edit/[photobookId]/__tests__/EditClient.reload.test.ts` (a8fe0db):
- 「正常_fetchEditViewClient を import している」
- 「正常_SSR 用 fetchEditView(空 Cookie) パターンを使っていない」
- 「正常_reload() 内で fetchEditViewClient(...) が呼ばれている」

`frontend/lib/__tests__/editPhotobook.test.ts` (β-3):
- `fetchEditViewClient` が `credentials: "include"` を渡し、Cookie ヘッダを手動で設定しないこと
- 401 / 404 / 500 / network 経路のエラー mapping

## 今後の検知方法（class-level）

`/edit` 1 箇所だけの guard では別画面で再発する可能性がある。横断 guard を追加:

`frontend/__tests__/harness-class-guards.test.ts`:
- すべての `"use client"` ファイル (`app/**/*.tsx` / `components/**/*.tsx`) で `fetchEditView(...)` の SSR variant を**呼び出していない**ことを scan で検証。
- import 自体は許容しないルール（client は `fetchEditViewClient` のみ使う）。

`.agents/rules/client-vs-ssr-fetch.md`:
- Client Component から authenticated API を呼ぶときは `credentials:"include"` 経路必須
- SSR 用 helper（`fetchEditView` 等）を `"use client"` ファイルから呼ばない
- 新 Client API を追加する場合は `*Client` suffix で明示

## 残る follow-up

- `manage/...`, `public/...` 系の Client Component でも同種の class-level guard が当たるか確認
- DOM testing library を入れて「最新を取得」ボタン押下の behavior test を追加

## 関連

- `frontend/lib/editPhotobook.ts` `fetchEditView` (SSR) / `fetchEditViewClient` (Client)
- `frontend/app/(draft)/edit/[photobookId]/EditClient.tsx`
- `.agents/rules/client-vs-ssr-fetch.md`
- a8fe0db commit
