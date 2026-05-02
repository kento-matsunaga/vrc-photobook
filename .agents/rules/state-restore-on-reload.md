# Reload 後 state 復元ルール

## 適用範囲

ユーザの作業状態（upload 中の画像、編集中の caption、選択中の cover、進行中の processing 等）を保持する **すべての Client Component 画面**。

## 原則

> **Reload で消えてはいけない state は、server ground truth または local persistence で復元できる設計にする。**
> **client-only `useState` だけで保持しない。**

理由:
- ブラウザ reload / タブ復帰 / OS 強制終了等で `useState` は揮発する。
- ユーザの作業状況が消えると「失敗した」と誤認して再操作 → 重複 upload / 重複作成 / API rate limit / DB / R2 ストレージ汚染 → 障害連鎖。
- 2026-05-03 STOP ε 前に観測された `/prepare` reload-loss は、`useState<QueueState>` だけで queue を保持し、server 側の image record（owner_photobook_id + status）を読み出して復元する経路が無かった。

## 必須パターン

### 1. Server ground truth を優先

ユーザの作業対象 entity（image / photobook / page / photo 等）は、server 側に永続化されている state を SSR fetch で取得 → Client Component の初期 state にする。

```tsx
// app/(draft)/prepare/[photobookId]/page.tsx (Server Component)
export default async function PreparePage({ params }) {
  const { photobookId } = await params;
  const cookieHeader = await getRequestCookieHeader();
  const view = await fetchEditView(photobookId, cookieHeader);  // ← server ground truth
  return <PrepareClient initialView={view} ... />;
}

// app/(draft)/prepare/[photobookId]/PrepareClient.tsx (Client Component)
"use client";
export function PrepareClient({ initialView, ... }) {
  // ✅ server から復元した queue を初期 state にする
  const [queue, setQueue] = useState<QueueState>(() =>
    mergeServerImages(emptyQueue(), initialView.images, ..., labelLookup, idGen),
  );
  // ...
}
```

### 2. Polling 中も server ground truth に reconcile

`useState` の優先度より server の最新値を優先。client が「進行中」と思っていても server で「失敗」になっていれば server 側を反映。

```tsx
const pollOnce = async () => {
  const v = await fetchEditViewClient(photobookId);
  setView(viewToState(v));
  setQueue((q) => {
    const reconciled = reconcileWithServer(q, v.placedImageIds, v.processingCount);
    return mergeServerImages(reconciled, v.images, v.placedImageIds, ..., idGen);
  });
};
```

### 3. localStorage は補助のみ

server が持たない情報（filename 表示ラベル等）の **補助** として localStorage を使う場合:
- key に内部識別子（imageId 等）を含めて良いが、**UI / DOM / data-testid / aria-label / console には raw 値を出さない**
- TTL / 上限 entry / cross-photobook 名前空間で持つ
- localStorage 不在環境（SSR / Safari Private mode）でも no-op フォールバック

### 4. Server に無い state は明示的に「reload で消える」と仕様化

例: Turnstile token、upload-verification token、選択中の File はメモリ + 進行中のみ存在する設計。これらは reload で再取得が必要であることを UI で表示する。

## 禁止事項

1. **ユーザ作業対象の state を `useState` だけで保持し、server ground truth を持たない**
2. **server response shape を変更せず、Frontend だけで状態保持を頑張る**（server 側に列追加 → API 拡張する判断を避けない）
3. **localStorage に Cookie 値 / token / Secret を保存**
4. **Reload 後に raw 内部識別子（imageId / photo_id / page_id 等）が DOM に露出**

## チェックリスト

新しい Client Component を追加する PR / ユーザ作業 state を扱う変更で:

- [ ] reload 後に消えてはいけない state を列挙した（PR description）
- [ ] 各 state について server ground truth / local persistence のどちらで復元するか明記した
- [ ] server ground truth が必要な場合、Backend API response に必要な field を追加した
- [ ] SSR + Polling 両方で server を ground truth に reconcile する関数（`mergeServerImages` 等）を持つ
- [ ] reload 復元の SSR test（initialView から正しい初期 queue が組まれること）を追加
- [ ] raw 内部識別子が DOM / data-testid / aria-label / console に出ないことを assert
- [ ] `.agents/rules/security-guard.md` の Cookie / token 取扱を遵守

## Why

2026-05-03 STOP ε 前の観測で `/prepare` の reload-loss が「ユーザ動線で最も致命的な事故」だった。修正は β-2 (Backend `images:[]` 追加) + β-3 (Frontend SSR + merge) の 2 段だったが、事故クラスは「Client-only state でユーザ作業を保持する設計」全般。今後 `/edit`, `/manage` 等で同種の reload-loss を作らないため、本ルールで設計判断を強制。

## 関連

- `frontend/components/Prepare/UploadQueue.ts` `mergeServerImages` / `reconcileWithServer`
- `frontend/lib/prepareLocalLabels.ts` filename 補助 cache
- `frontend/app/(draft)/prepare/[photobookId]/PrepareClient.tsx` SSR + polling 復元
- `harness/failure-log/2026-05-03_prepare-reload-queue-loss.md`
- `harness/failure-log/2026-05-03_image-processing-visibility.md`
- `.agents/rules/security-guard.md`
- 9ac7699 / f455fe4 commit

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-03 | 初版作成。β-2 / β-3 の reload-loss 修正をクラス level にルール化 |
