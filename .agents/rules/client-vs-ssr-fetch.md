# Client / SSR fetch 分離ルール

## 適用範囲

Frontend (Next.js + Cloudflare Workers / OpenNext) で **authenticated cross-origin API**（`api.vrc-photobook.com` の `/api/*`）を呼ぶすべての fetch helper / 呼び出し箇所。

## 原則

> **Client Component から authenticated cross-origin API を呼ぶときは、必ず `credentials: "include"` 経路を使う。**
> **SSR 用に Cookie ヘッダ手動転送する fetch helper を `"use client"` ファイルから呼ばない。**

理由:
- SSR (Server Component / Route Handler) は Next.js から `headers()` で受信 Cookie を取得し、Backend へ手動で `Cookie: ...` ヘッダ転送する設計（Edge runtime のため）。
- Client Component は browser fetch を使うため、cross-origin で Cookie を送るには `credentials: "include"` が必要。
- 同じ helper 関数を SSR 用 / Client 用兼用にすると、Client から呼んだときに Cookie ヘッダを手動で渡せず、`credentials` も `same-origin` 既定なので Cookie が一切送られない → Backend session middleware が 401。
- 2026-05-03 STOP α で `/edit` の `reload()` がこのパターンに陥り、polling と「最新を取得」ボタンが常時 401 を量産していた事故が発生。

## 必須パターン

### 1. lib に SSR / Client 両方の関数を分離

```ts
// lib/editPhotobook.ts

/** SSR Server Component から呼ぶ。Cookie ヘッダ手動転送。 */
export async function fetchEditView(
  photobookId: string,
  cookieHeader: string,   // ← Server Component が headers().get("cookie") から渡す
  signal?: AbortSignal,
): Promise<EditView> {
  // ... fetch with headers: { Cookie: cookieHeader } ...
}

/** Browser Client Component から呼ぶ。credentials:"include"。 */
export async function fetchEditViewClient(
  photobookId: string,
  signal?: AbortSignal,
): Promise<EditView> {
  // ... fetch with credentials: "include" ...
}
```

### 2. Client Component は `*Client` suffix の関数だけ import する

```tsx
// ✅ 正しい (Client Component)
"use client";
import { fetchEditViewClient } from "@/lib/editPhotobook";

const reload = async () => {
  const next = await fetchEditViewClient(photobookId);
  // ...
};

// ❌ 禁止 (Client Component から SSR 用関数を呼ぶ)
"use client";
import { fetchEditView } from "@/lib/editPhotobook";

const reload = async () => {
  const next = await fetchEditView(photobookId, "");  // ← Cookie 送られない、401 になる
  // ...
};
```

### 3. mutation 系も `credentials: "include"` 経路

`updatePhotoCaption` / `bulkReorderPhotos` / `setCoverImage` / `clearCoverImage` / `removePhoto` / `addPage` / `updatePhotobookSettings` / `prepareAttachImages` / `publishPhotobook` 等、Client Component から呼ぶ mutation はすべて `mutate()` helper 経由で `credentials: "include"` を使う。

## 禁止事項

1. **`"use client"` ファイル内で SSR 用 fetch helper を import / 呼出**
2. **同じ関数名で SSR / Client 兼用にする**（呼び出し側が用途を判別できない）
3. **`fetchXxx(..., "")` のように空 Cookie ヘッダを渡して "Client でも動くから" と SSR 用関数を流用**
4. **Cross-origin fetch で `credentials` 指定なし**（既定の `same-origin` で Cookie が送られない）

## チェックリスト（PR レビュー / closeout で使う）

新しい authenticated API client / Client Component を追加した PR では:

- [ ] lib に SSR 用 (`fetchXxx`) と Client 用 (`fetchXxxClient`) が分離されている
- [ ] Client Component は `*Client` 関数だけ import している
- [ ] Client 用関数のテストが `credentials: "include"` を渡し、Cookie ヘッダを手動設定しないことを assert
- [ ] Client Component の reload / polling 経路の guard test（source 構造 or behavior test）が存在
- [ ] `frontend/__tests__/harness-class-guards.test.ts` (横断 antipattern guard) が PASS

## Why（なぜこのルールが必要か）

2026-05-03 STOP α で `/edit/EditClient.tsx` の `reload()` が `fetchEditView(view.photobookId, "")` を呼び続けており、polling / 「最新を取得」ボタンが常時 401 を量産していた。修正は 1 行（`fetchEditViewClient` に置換）だが、事故クラスは「Client Component で認証 API を SSR fetch と取り違える」という設計レベルの問題。

`/edit` を直しても別画面で再発する可能性がある。本ルール + 横断 guard test で事故クラス全体を抑え込む。

## 関連

- `.agents/rules/security-guard.md` — Cookie / 認可全般
- `.agents/rules/safari-verification.md` — Safari ITP / Cross-origin Cookie
- `harness/failure-log/2026-05-03_client-reload-ssr-fetch-mistake.md`
- `frontend/__tests__/harness-class-guards.test.ts` — 横断 antipattern scan
- a8fe0db commit (1 箇所修正), 9c4fb7d commit (rights agreement 実装)

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-03 | 初版作成。STOP α `/edit` reload 401 事故をクラス level に抽象化 |
