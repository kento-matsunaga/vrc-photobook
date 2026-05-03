# m2-design-refresh STOP β-3: 動線画面 1 (Create / Prepare) 詳細分割計画

> 状態: STOP β-3 **承認済 設計判断資料**（user 確認済、Q-3-1〜Q-3-11 はすべて推奨 default で確定）。β-3-1 から実装着手可。実装 / commit / push は sub-step 単位で進める。deploy はしない。
>
> 前提:
> - HEAD == origin/main == `3487710`
> - STOP β-1 (`a61163c`) / β-2 (`0d8f156`〜`3487710`) 完了
> - LP / About / Terms / Privacy / Help / landing image asset pipeline 完成済
> - deploy 未実施
> - 親計画: `docs/plan/m2-design-refresh-plan.md` §6 STOP β-3
> - design 正典:
>   - `design/source/project/wf-screens-a.jsx:206-308` (Create M / PC)
>   - `design/source/project/wf-screens-a.jsx:334-443` (Prepare M / PC)
>   - `design/source/project/wf-shared.jsx:29-48` (PC header → PublicTopBar)
>   - `design/source/project/wireframe-styles.css` (各 widget class)
> - 方針:
>   - design はそのまま（visual / layout のみ）
>   - **business logic は一切変更しない** (Turnstile L0-L4 / upload concurrency / polling / reload 復元 / credentials:include)
>   - 法務 / production truth (visibility note / 失敗 reason 文言 / file size 制限) は削らない
>   - Backend / deploy / Workers / Scheduler / Job / DB / Secret / env / binding 変更は禁止
>   - `design/usephot/` raw PNG / generated assets は触らない
>   - β-3 は **frontend SSR / Client component の visual restyle のみ**

---

## 0. 分割理由 + 順序

### 0.1 推奨順: β-3-1 → β-3-2

| 順 | sub-step | 理由 |
|---|---|---|
| 1 | **β-3-1 Create entry** | 公開経路 (Server + Client)。LP / About / Terms / Privacy / Help と同じ PublicTopBar 統合パターンを動線にも展開。Turnstile / radio / input / counter の design wf-* class 流用パターンを最初に確定する |
| 2 | **β-3-2 Prepare upload staging** | draft session 経路で機能ロジックが大きい (concurrency=2 / polling / reload 復元 / 10 min slow notice / failure reasons)。β-3-1 で確定した PublicTopBar / wf-input / wf-radio / wf-note を流用、追加で wf-upload-tile / wf-m-stick-cta / wf-grid-2-1 を導入 |

### 0.2 代替検討（採用しない理由）

| 案 | 採否 | 理由 |
|---|---|---|
| Create / Prepare を 1 commit に集約 | 不採用 | shell (public vs draft session) と機能依存 (Turnstile + POST のみ vs upload concurrency + polling) が異なり、レビュー / rollback 単位を分けた方が安全 |
| Prepare を先 | 不採用 | PublicTopBar 統合パターン確立前に機能複雑な Prepare を弄ると後修正リスク。Create で pattern 確定が安全 |
| 共通 wf-radio / wf-input / wf-counter コンポーネントを先に切り出す | 不採用 | β-3 内で 2 用途しかなく、共通化は YAGNI。`wireframe-styles.css` の class を Tailwind で再現する形で十分。共通化は後続 STOP (β-4 Edit) でも需要が出たら β-6 まとめで検討 |

---

## 1. β-3-1: Create entry

### 1.1 scope

- `/create` ページに **PublicTopBar 統合** (LP / About / Terms / Privacy / Help と一貫した shell)
- eyebrow + h1 を design 正典に整合 (eyebrow「Step 1 / 3」、h1「どんなフォトブックを作りますか?」)
- type radio 7 個を design `.wf-radio` 視覚 (active border-teal-500 + bg teal-50 + dot radial) に整合
- title / creator input を design `.wf-input` (border-line / radius-8 / focus teal-200) + `.wf-counter` 文字数表示に整合
- visibility note を design `.wf-note` (border-l teal-300 + bg teal-50 + i icon) に整合 (β-2b-1 PolicyNotice と同 spirit)
- Turnstile widget 周辺余白を design 通り (Turnstile widget 自体は既存 component 流用、視覚は周辺レイアウトで調整)
- submit button を design `.wf-btn primary lg full` (Mobile) / `.wf-btn primary lg` 右寄せ (PC) に整合
- error 表示は既存維持 (`role="alert"` / `data-testid="create-error"` / status-error tone)
- **business logic / Turnstile L0-L4 / POST /api/photobooks / window.location.replace は一切変更しない**

### 1.2 変更予定ファイル

| File | 変更内容 |
|---|---|
| `frontend/app/(public)/create/page.tsx` | PublicTopBar 統合 (`<><PublicTopBar /><main>...`) / eyebrow を「Step 1 / 3」(design 通り) / main wrapper を他公開ページと統一 (`max-w-screen-md px-4 py-6 sm:px-9 sm:py-9`) / showTrustStrip=false 維持 |
| `frontend/app/(public)/create/CreateClient.tsx` | type radio container を design `.wf-radio` 視覚 (active 状態 + dot radial) / input を `.wf-input` (h-[42px] / rounded-md / border-divider / focus:border-teal-400 + ring-teal-200) / counter を `.wf-counter` (text-[10.5px] text-ink-soft text-right) / visibility note を `.wf-note` 化 / submit button を design `.wf-btn primary lg` 視覚 / 既存 data-testid (create-page / create-form / create-type-{key} / create-error / create-submit-button) 維持 |
| `frontend/app/(public)/create/__tests__/CreateClient.test.tsx` | visual update sync (active class / counter / visibility note 表記)。機能 test (Turnstile / disable / submit) は既存維持 |

### 1.3 design source 対応

| 要素 | design source | 採用方針 |
|---|---|---|
| Create PC | `wf-screens-a.jsx:255-308` | wf-pc-container narrow / eyebrow Step 1/3 / wf-grid-3 で 7 radio (3 列 × 3 行 + 余り) / wf-grid-2 で title + creator / wf-note visibility / 右寄せ submit |
| Create M | `wf-screens-a.jsx:206-254` | WFMobile (production: PublicTopBar 統合) / 縦 stack: eyebrow + h1 + sub + radio 7 縦 / title + creator 縦 / wf-note + Turnstile + 全幅 submit + error note |
| `.wf-radio` | `wireframe-styles.css:289-313` | flex gap-2.5 / px-3.5 py-3 / rounded-[10px] / border-divider / hover:border-teal-200 / active: border-teal-500 + bg-teal-50 + border-[1.5px] / dot 16×16 round border-ink-soft / active dot radial teal-500 |
| `.wf-input` | `:256-266` | h-[42px] / rounded-md / border-divider / px-3 / text-[13px] / focus:outline-2 outline-teal-200 + border-teal-400 |
| `.wf-label` | `:279-284` | text-xs font-semibold text-ink-strong mb-1.5 block |
| `.wf-counter` | `:285` | text-[10.5px] text-ink-soft text-right mt-1 (font-num) |
| `.wf-note` | `:398-425` | β-2b-1 PolicyNotice 流用 (border-l teal-300 + bg teal-50 + i icon teal-500 round) |
| `.wf-btn primary lg` | `:228-251` | h-12 px-6 rounded-[10px] bg-brand-teal text-white font-bold text-sm + disabled:opacity-45 (full は w-full) |

### 1.4 content / 機能維持方針

- **type 7 種**: 既存 `TYPE_OPTIONS` (`memory / event / daily / portfolio / avatar / world / free`) と label / description は変更しない
- **任意入力**: title (max 100) / creator_display_name (max 50) の制約と placeholder は維持。counter 表示 (`0 / 100`) を design 正典に整合
- **visibility note**: 「公開範囲は限定公開（URL を知っている人のみ閲覧可能）が既定」維持。design `wf-note` 視覚で再ラップ
- **rights agreement 等は publish 時** (本ページでは触れない、既存通り)
- **Turnstile L0-L4 多層ガード**: `.agents/rules/turnstile-defensive-guard.md` 完全遵守。callback ref / disabled 判定 / submit early return / lib defensive guard / Backend 不在 — **変更なし**
- **error mapping**: 既存 `ERROR_MESSAGES` (invalid_payload / turnstile_failed / turnstile_unavailable / server_error / network) 維持
- **遷移**: POST /api/photobooks → `window.location.replace(out.draftEditUrlPath)` (raw token を history に残さない既存方針) 維持
- **法務文言 (個人運営 / 非公式 / Turnstile / Bot 検証)**: 該当 strings の出現行数を old vs new で比較し削減ゼロを確認

### 1.5 visual QA 観点

| 観点 | 期待 |
|---|---|
| Mobile 360×740 | TopBar / eyebrow / h1 / sub / radio 7 縦 / title + creator + counter / wf-note / Turnstile / submit (full width) / error 表示 / footer 縦 stack。横はみ出し無し |
| PC 1280×820 | narrow 760px / radio wf-grid-3 (3 col × 3 row) / title+creator wf-grid-2 / wf-note / Turnstile / 右寄せ submit |
| TopBar sticky と h1 | scroll 0 で h1 隠れず (pt-6 程度) |
| radio active 状態 | border teal-500 + bg teal-50 + dot 内側 teal-500 radial、選択切替で transition 自然 |
| input focus | outline teal-200 ring + border teal-400 で keyboard focus visible |
| counter 桁 | 数字 font-num (SF Pro Display) / 100 桁あふれ無し |
| submit disabled | Turnstile 未完で opacity-45 / cursor-not-allowed |
| Turnstile widget スピナー中の押下 | `.agents/rules/turnstile-defensive-guard.md` L1-L2 で submit されない |

### 1.6 test 方針

- 既存 test (`CreateClient.test.tsx` 4 tests) は機能テスト中心 — visual class assertion のみ minor update
  - active radio class assertion (`bg-teal-50` / `border-teal-500`) を追加 or update
  - counter 表記 (`0 / 100`) 維持
  - visibility note 表記 (「限定公開」「URL を知っている人のみ閲覧可能」) 維持
- 既存 page-level test (`create-page` data-testid 確認) は既存維持。SSR test は無いため新規追加は不要
- 機能 test (Turnstile validation / disable / submit) は変更なし

### 1.7 commit 方針

**1 commit**: `feat(design): restyle create entry with topbar and design system`

理由: PublicTopBar 統合 / radio / input / note / submit を 1 PR で揃えると review が直感的。test sync は同 commit で。

---

## 2. β-3-2: Prepare upload staging

### 2.1 scope

- `/prepare/[photobookId]` ページに **PublicTopBar 統合** (draft session 経路だが LP に戻る nav は妥当)
- eyebrow + h1 を design 正典に整合 (eyebrow「Step 2 / 3」、h1「写真をまとめて追加」)
- ファイル制約説明 (JPEG/PNG/WebP / 10MB / 20 枚 / HEIC 未対応) を design wf-sub に整合
- Turnstile widget の周辺レイアウト (M: header 直下 / PC: 右 sidebar) を design 通り
- ファイル選択 picker を design `.wf-box.dashed` (dashed border + center + 余白大) に整合
- 進捗パネル (合計/完了/処理中/失敗 + bar) を design `.wf-box` (PC は右 sidebar) に整合
- 画像 tile grid を Mobile 2 col / PC 4 col に (design 通り)
- ImageTile を design `.wf-upload-tile` (rounded-[10px] / border / shadow-sm / bar 4px / stat 10px) に整合
- Mobile bottom sticky CTA `.wf-m-stick-cta` を導入 (既存は通常 flex 配置、design 通り sticky bottom + border-top + shadow)
- PC layout を `.wf-grid-2-1` (left 2fr: picker + tiles / right 1fr: 進捗 + Turnstile + CTA + note) に整合
- slow notice (10 min 超過) は design 通り `.wf-note` で表示
- **business logic は一切変更しない** (concurrency=2 / 5 sec polling + backoff / Page Visibility API / reconcileWithServer / mergeServerImages / SSR initialView 復元 / credentials:include / Turnstile L0-L4 / 失敗 reason mapping)

### 2.2 変更予定ファイル

| File | 変更内容 |
|---|---|
| `frontend/app/(draft)/prepare/[photobookId]/page.tsx` | PublicTopBar 統合 (Server Component で wrap)、main wrapper 統一 |
| `frontend/app/(draft)/prepare/[photobookId]/PrepareClient.tsx` | eyebrow + h1 + sub design 整合 / picker section を `.wf-box dashed` 化 / 進捗パネルを `.wf-box` (PC は wf-grid-2-1 右側) / tiles grid Mobile 2col PC 4col / Mobile bottom sticky CTA `.wf-m-stick-cta` / PC は wf-grid-2-1 layout / slow notice を `.wf-note` 化 / 既存 data-testid (prepare-page / prepare-picker / prepare-summary / prepare-progress / prepare-tiles / prepare-error / prepare-proceed / prepare-proceed-error / prepare-normal-notice / prepare-slow-notice / prepare-file-input) 維持 |
| `frontend/components/Prepare/ImageTile.tsx` | `.wf-upload-tile` 視覚 (rounded-[10px] / border-divider / p-2 / shadow-sm / gap-1.5) / bar を design (h-1 bg-divider-soft → inner bg-teal-500 width %) / stat を design (text-[10px] flex-justify-between) / failed 状態 (border-status-error + bg-status-error-soft) 維持 / data-testid `prepare-tile-{id}` 維持 |
| `frontend/app/(draft)/prepare/[photobookId]/__tests__/PrepareClient.test.tsx` | SSR markup test 同期 (data-testid 維持、wf-* class 名追加 assertion / 進捗 / picker / sticky CTA presence)。機能 test (logic) は既存維持 |
| `frontend/components/Prepare/__tests__/UploadQueue.test.ts` | 純粋 logic test、変更なし |

### 2.3 design source 対応

| 要素 | design source | 採用方針 |
|---|---|---|
| Prepare PC | `wf-screens-a.jsx:390-443` | wf-pc-container / eyebrow Step 2/3 / wf-grid-2-1 (left 2fr / right 1fr) / dashed picker 大 / wf-grid-4 tiles / 右 sidebar wf-box 進捗 + Turnstile + CTA + note |
| Prepare M | `wf-screens-a.jsx:334-389` | WFMobile (production: PublicTopBar 統合) / 縦 stack: eyebrow + h1 + sub / Turnstile / dashed picker 中央 / 進捗 wf-box / wf-grid-2 tiles / wf-m-stick-cta bottom |
| `.wf-box.dashed` | `wireframe-styles.css:165-175` + dashed | rounded-lg + border-2 dashed + bg-surface + p-6 (Mobile) / p-9 (PC) + text-center |
| `.wf-upload-tile` | `:442-464` | rounded-[10px] + border-divider + bg-surface + p-2 + flex-col gap-1.5 + shadow-sm。bar h-1 rounded-sm bg-divider-soft + inner h-full bg-teal-500 width var(--p)。stat text-[10px] flex-justify-between text-ink-soft。failed: border-status-error + bg-status-error-soft + bar inner bg-status-error |
| `.wf-m-stick-cta` | `:513-520` | sticky bottom-0 + bg-surface + border-t border-divider-soft + p-3 + flex gap-2.5 + shadow-up |
| `.wf-grid-2-1` | `:568` | grid grid-cols-[2fr_1fr] gap-[22px] (PC) / Mobile は単 col に reset |
| `.wf-note` (slow notice) | `:398-425` | β-2b-1 PolicyNotice 流用、warn 系は warn-soft tone (orange) |

### 2.4 content / 機能維持方針

- **Turnstile L0-L4 多層ガード**: `.agents/rules/turnstile-defensive-guard.md` 完全遵守 — 変更なし
- **upload concurrency=2 / 5 sec polling + exponential backoff / max 10 min duration / Page Visibility API**: `PrepareClient.tsx` の実装 logic は変更なし
- **SSR initialView 復元 + reconcileWithServer + mergeServerImages**: `.agents/rules/state-restore-on-reload.md` 完全遵守 — 変更なし
- **credentials:include polling**: `.agents/rules/client-vs-ssr-fetch.md` 完全遵守、`fetchEditViewClient` 経由 — 変更なし
- **raw imageId / storage_key / upload URL の DOM / data-testid / aria-label / console 露出禁止**: 既存方針維持、`prepare-tile-{id}` の id は internal tile id (UUID 風 random)
- **failure reason mapping**: 既存 `FAILED_REASON_LABEL` (verification_failed / rate_limited / validation_failed / upload_failed / complete_failed / network / processing_failed / unknown) 維持
- **slow notice (10 min)**: 既存 `prepare-slow-notice` 表示 + role="status" 維持
- **20 枚上限 / 10 MB / HEIC 未対応**: 既存制約維持。表示テキスト変更なし
- **法務 / 制約文言**: 「JPEG / PNG / WebP」「10MB / 1 枚」「20 枚」「HEIC / HEIF 未対応」「Bot 検証」「対象の画像がありません」「全ての画像処理が終わるまでお待ちください」「画像の処理は通常 1〜2 分」「画像の処理に時間がかかっています」等は削減なし

### 2.5 visual QA 観点

| 観点 | 期待 |
|---|---|
| Mobile 360×740 | TopBar / eyebrow / h1 / sub / Turnstile / dashed picker / 進捗 wf-box / 2 col tiles / sticky CTA bottom (overlap せず scrollable area 確保) / footer (sticky CTA で隠れない) |
| PC 1280×820 | wf-pc-container / eyebrow + h1 / wf-grid-2-1: left (dashed picker 大 + wf-grid-4 tiles) / right (進捗 + Turnstile + CTA + note)。横はみ出し無し |
| Sticky CTA Mobile | scroll しても bottom 固定、footer / footer link が disabled CTA に隠れないよう min-height 確保 |
| dashed picker | border-2 dashed + 中央 placeholder + Turnstile 未完時は file-input disabled + opacity 半 |
| 進捗 wf-box | 完了/合計の n/m 表記、bar アニメーション、処理中/失敗 detail |
| upload-tile bar | uploading 時は teal-500 width 50%-pulse / processing は idle / failed は border 赤 + reason 文言 |
| Turnstile widget remount | `.agents/rules/turnstile-defensive-guard.md` L0 (callback ref) 維持で無限ループなし |
| 10 min slow notice | warn-soft tone (orange) で出現、normal-notice は teal-soft tone |
| TopBar sticky と h1 | scroll 0 で h1 隠れず、scroll 中も TopBar 固定 |

### 2.6 test 方針

- 既存 `PrepareClient.test.tsx` (11 tests): 機能 test 中心 (prepare-progress / prepare-normal-notice / prepare-slow-notice / 復元された画像 / 主要 testid)
  - 維持必須: 上記すべて
  - sync update: wf-* class assertion (sticky CTA / dashed picker / wf-grid-2-1 PC layout) を追加
  - h1 文言「写真をまとめて追加」は維持 (design 一致)
  - eyebrow「Step 2 / 3」は新規 assertion 追加
  - PublicTopBar presence (`data-testid="public-topbar"`) を新規追加
- 既存 `UploadQueue.test.ts` (36 tests): 純粋 logic test、変更なし
- ImageTile の test は無いが、新規 SSR test を追加するか判断 (既存 PrepareClient.test.tsx で間接的に assert される) — **追加しない** (β-3 scope は visual restyle、既存 test で不足なら β-6 で追加)

### 2.7 commit 方針

**1 commit**: `feat(design): restyle prepare upload staging with design system`

理由: page wrapper / PrepareClient / ImageTile / test を 1 PR でまとめると review が直感的。

---

## 3. visual QA 統合 matrix

| 観点 | β-3-1 (Create) | β-3-2 (Prepare) |
|---|---|---|
| Mobile 360×740 横はみ出し | radio 7 縦 + input + note + Turnstile + submit | TopBar + picker + 進捗 + tiles 2col + sticky CTA |
| PC 1280×820 narrow / wf-grid-2-1 | narrow 760 + wf-grid-3 radio + wf-grid-2 input | wf-pc-container + wf-grid-2-1 (2fr 1fr) |
| TopBar sticky + h1 | pt-6 で隠れず | pt-6 / sticky CTA との合成で h1 余白十分 |
| Turnstile widget 視覚 | 既存 widget 流用、周辺余白 design 整合 | 同左 + Mobile sticky CTA との同居 |
| radio active 状態 | border-teal-500 + bg-teal-50 + dot radial | n/a |
| input focus | outline-teal-200 + border-teal-400 | n/a |
| upload-tile bar / failed | n/a | h-1 teal-500 / failed border-error + bg-error-soft |
| sticky CTA Mobile | n/a | bottom-0 + border-t + shadow-up + flex |

---

## 4. test 方針 統合

### 4.1 SSR markup tests (各 page)

| page | 主要 assertion |
|---|---|
| `/create` | `data-testid="public-topbar"` / eyebrow「Step 1 / 3」/ h1「どんなフォトブックを作りますか?」/ create-page / create-form / create-type-{key} 7 件 / create-error / create-submit-button / visibility「限定公開」/ Turnstile widget 配置 |
| `/prepare/[id]` | `data-testid="public-topbar"` / eyebrow「Step 2 / 3」/ h1「写真をまとめて追加」/ prepare-page / prepare-picker / prepare-summary / prepare-progress / prepare-tiles / prepare-tile-{id} / prepare-proceed / prepare-error / prepare-normal-notice / prepare-slow-notice (条件付き) / dashed picker 視覚 / sticky CTA |

### 4.2 機能 test (logic)

| test file | 維持事項 |
|---|---|
| `CreateClient.test.tsx` | Turnstile validation / disable / submit early return / error mapping / window.location.replace 流用 |
| `PrepareClient.test.tsx` | concurrency=2 / polling / reload 復元 / 10 min slow notice / failure reasons / proceed |
| `UploadQueue.test.ts` | 36 tests 全 PASS、変更なし |
| `harness-class-guards.test.ts` | 旧曖昧文言不在 / SSR fetch 不使用 / CORS PATCH/DELETE — 全 PASS 維持 |
| `imageCompression.test.ts` | 22 tests 全 PASS、変更なし |
| `upload.test.ts` | 32 tests 全 PASS、変更なし |
| `editPhotobook.test.ts` | 30 tests 全 PASS、変更なし |
| `uploadVerificationCache.test.ts` | 7 tests 全 PASS、変更なし |
| `prepareLocalLabels.test.ts` | 9 tests 全 PASS、変更なし |
| `retryAfter.test.ts` | 10 tests 全 PASS、変更なし |

### 4.3 PublicTopBar presence tests (両 page)
```ts
expect(html).toContain('data-testid="public-topbar"');
```

### 4.4 raw value 漏洩 guard (β-3-2 重点)
```ts
expect(html).not.toMatch(/imageId|storage_key|upload_url|verification_token|draft_edit_token|manage_url_token/);
expect(html).not.toMatch(/Set-Cookie:/i);
```
※ `prepare-tile-${tile.id}` の `tile.id` は internal random UUID、raw imageId ではない (既存 `newTileId()`)。

---

## 5. 検証コマンド (β-2 各 sub-step と同じ 5 段階)

```bash
npm --prefix /home/erenoa6621/dev/vrc_photobook/frontend run test
npm --prefix /home/erenoa6621/dev/vrc_photobook/frontend run typecheck
npm --prefix /home/erenoa6621/dev/vrc_photobook/frontend run build
npm --prefix /home/erenoa6621/dev/vrc_photobook/frontend run cf:build
npm --prefix /home/erenoa6621/dev/vrc_photobook/frontend run cf:check
git -C /home/erenoa6621/dev/vrc_photobook diff --check
```

5 段階すべて PASS で初めて commit 可。bundle 25 MB / single file 5 MB 上限維持、cf:check Total Upload は 9000 KiB target 維持。

deploy はしない (Backend / Workers / Cloud SQL / Job / Scheduler 一切変更しない)。

---

## 6. commit 方針

**2 commit に分割** (sub-step 単位、別 commit 推奨):

```
1. feat(design): restyle create entry with topbar and design system
2. feat(design): restyle prepare upload staging with design system
```

各 commit:
- `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>` を最後の行に付与
- 該当 sub-step の対象 file のみを `git add <files>` で明示 staging
- `.claude/scheduled_tasks.lock` / `ChaeckImage/` / `TESTImage/` / `design/usephot/` raw PNG は staging しない
- 5 段階検証が PASS してから commit
- `git diff --check` clean 確認

各 commit 後に user 承認 → push (`git push origin main`)。両 sub-step 完了するまで deploy しない。

---

## 7. open questions（**全項目 user 確定済**）

下表は本計画 commit 時点で **すべて user 確認済**（推奨 default 採用）。`確定方針` 列は β-3-1 / β-3-2 実装で採用する。

| ID | 内容 | 影響範囲 | 確定方針 |
|---|---|---|---|
| Q-3-1 | Create eyebrow を design 正典「Step 1 / 3」にするか、他公開ページと統一の英語短ラベル「Create」にするか? | β-3-1 | **「Step 1 / 3」(design 正典)**: 動線画面の進捗ステップを示すため、design 通り採用 |
| Q-3-2 | `/create` page に PublicTopBar 統合するか? (LP / About / Terms / Privacy / Help と一貫) | β-3-1 | **統合する**: 公開動線、上部 nav は他公開ページと一貫 |
| Q-3-3 | `/prepare/[id]` page (draft session 経路) に PublicTopBar 統合するか? `showPrimaryCta` をどうするか? | β-3-2 | **統合する + showPrimaryCta=false**: nav は LP に戻る link として有用、ただし「無料で作る」CTA は draft 中に違和感のため非表示 |
| Q-3-4 | Mobile bottom sticky CTA を design `.wf-m-stick-cta` (sticky bottom + border-t + shadow) で実装するか? | β-3-2 | **実装する**: design 正典通り、scrollable tiles 領域確保 |
| Q-3-5 | PC Prepare の `.wf-grid-2-1` (2fr 1fr) を採用するか? 既存は中央 1 col | β-3-2 | **採用する**: PC で picker / tiles を左 2fr、進捗 / Turnstile / CTA を右 1fr。design 正典に従う |
| Q-3-6 | ImageTile uploading bar の表現: design は `width: var(--p, 50%)` 固定 vs 既存 `animate-pulse w-1/2`。どちらを採用? | β-3-2 | **animate-pulse 維持**: 50% pulse が「進行中」を視覚的に伝えやすい。design は static placeholder。production 採用は pulse |
| Q-3-7 | Create input focus ring color / width: design `outline 2px teal-200 + border teal-400` vs Tailwind 既定 ring | β-3-1 | **design 通り**: outline-2 outline-teal-200 + focus:border-teal-400 |
| Q-3-8 | radio dot 表現: design `radial-gradient(circle, teal-500 42%, transparent 44%)` を CSS で再現可。Tailwind では bracket / inline style どちらが良いか | β-3-1 | **inline style + Tailwind class 併用**: bracket arbitrary は radial-gradient 表現が冗長になるため、inline style で `backgroundImage: 'radial-gradient(...)'` を許容 (β-2a `THUMB_GRADIENTS` と同 pattern) |
| Q-3-9 | Prepare page Server wrapper (page.tsx) の現状確認 — まだ存在する? | β-3-2 | **要確認**: `app/(draft)/prepare/[photobookId]/page.tsx` が存在し initialView を Server fetch する Server Component と推定。実装着手時に再確認、必要に応じて wrapper も restyle |
| Q-3-10 | 既存 type description ("VRC で過ごした特別な時間を 1 冊に。" 等) は production truth として維持するか? design は placeholder のため未確定 | β-3-1 | **既存維持**: design に proposed なし、production 文言が読みやすさで優位。削減なし |
| Q-3-11 | β-3-1 / β-3-2 を 1 PR にまとめるか / 別々に push するか? | 全体 | **別 commit + 順次 push** (β-2b と一貫): rollback 単位を細かく保つ |

---

## 8. 推奨する最初の着手単位

**β-3-1: Create entry**

理由:
- 公開経路で PublicTopBar 統合パターンを動線画面に展開する最初のケース
- 機能依存が最小 (Turnstile + POST /api/photobooks のみ)
- design wf-radio / wf-input / wf-counter / wf-note / wf-btn pattern を最初に確定 → β-3-2 で流用
- Q-3-1〜Q-3-2、Q-3-7、Q-3-8、Q-3-10 を確定して着手可能

Q-3-1〜Q-3-11 は本計画 commit 時点で **すべて user 確定済**（§7 表参照）。β-3-1 着手即可。

---

## 9. deploy しないことの確認

| 操作 | β-3-1 / β-3-2 |
|---|---|
| Backend deploy (Cloud Run) | ❌ |
| Workers deploy (Cloudflare) | ❌ |
| Cloud SQL / Cloud Job / Scheduler 変更 | ❌ |
| Secret / env / binding 変更 | ❌ |
| `design/usephot/` raw PNG 取扱 | ❌ (β-2c で完了済) |
| business logic 変更 (Turnstile L0-L4 / upload concurrency / polling / reload 復元) | ❌ |
| commit + push (各 sub-step 完了時、user 承認後) | ✅ |

両 sub-step すべて完了後、改めて β-4 (Edit / Complete / Manage) → β-5 (Viewer / Report) → β-6 (visual QA) → γ → δ → ε に進む。

---

## 10. 履歴

| 日付 | 変更 |
|---|---|
| 2026-05-03 | 初版作成。`m2-design-refresh-plan.md` §6 STOP β-3 を 2 sub-step (β-3-1 Create / β-3-2 Prepare) に詳細化 |
| 2026-05-03 | user 確認反映: Q-3-1〜Q-3-11 の 11 項目すべて推奨 default で確定（Step 1/3 eyebrow / PublicTopBar 統合 / showPrimaryCta=false on Prepare / outline-2 teal-200 focus / inline radial-gradient dot / animate-pulse 維持 / wf-grid-2-1 採用 / sticky CTA 実装 / type description 維持 / 別 commit 順次 push） |
