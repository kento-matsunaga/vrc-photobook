# PR37 design rebuild plan メモ（2026-05-01、STOP α 承認版）

> **状態**: STOP α 承認済（2026-05-01）。STOP β 実装はこの plan メモを **唯一の正典** として参照する。
> 起点: failure-log [`harness/failure-log/2026-05-01_pr37-public-pages-design-mismatch.md`](../failure-log/2026-05-01_pr37-public-pages-design-mismatch.md) §5 ルール適用
> 前 PR commit: `7f459f5`（PR37 機能編 closeout）/ Workers `6f1e82d7-...` で稼働中
> failure-log §5 のルール適用初事例

## 1. 採用する prototype 画面 ID

| 画面 | 採用元 | 採用度 |
|---|---|---|
| LP `/` mobile | `design/mockups/prototype/screens-a.jsx` の `LP({ go })` | forward port: 高（Hero / mock-book / thumbnail strip / features grid / cta-block / trust-strip）|
| LP `/` PC | `design/mockups/prototype/pc-screens-a.jsx` の `PCLP({ go })` | forward port: 高（pc-hero 2 col + pc-thumb-strip + pc-features-grid 4 col + pc-cta + pc-trust）。**ただし `pc-header` の nav は不採用** |
| About `/about` | LP の `feature-cell` パターン + `screens-b.jsx` Viewer "Memories card" の dashed border 区切り（**1 箇所のみ**）| 部分採用 |
| Terms `/terms` | prototype 直接対応なし | 既存 `/help/manage-url` の温度感を引用 + design-system token 整理 |
| Privacy `/privacy` | 同上 | 同上 |
| 共通 footer | `screens-a.jsx` `trust-strip` + `pc-trust` | trust-strip を LP / About の closing element として、footer 自体は nav リンク列 |

## 2. 採用しない prototype / concept image

- `pc-header` のサービス内 nav（「作例」「使い方」「よくある質問」「無料で作る」）— MVP では死リンクになる
- `screens-a.jsx` の `今すぐ作る` / `作例を見る` 主 CTA — 作成導線・公開済み作例なし、`/about` / `/help/manage-url` への誘導に置き換え
- Hero book mock の **実写真**（gradient placeholder で代替）
- `screens-b.jsx` Viewer の "写真枚数 / 撮影ワールド" メタ — About には不適合
- `concept-images/concept-NN.png` 15 枚 — 業務知識タイプ分類参考、本 PR で直接利用しない
- 装飾的 gradient orb / bokeh / 紫グラデーション全面（**ページ背景には使わない**）— failure-log §5

## 3. ページ別ワイヤーフレーム

### 3.1 `/`（LP）

```
┌─ HERO（mobile 単列 / PC 1.05fr 1fr）
│  左: eyebrow "VRC PhotoBook" (text-xs teal)
│      h1 "VRC写真を、ログイン不要で1冊に。"
│        mobile 26px / PC 40px / extrabold / tracking-tight
│      body 1〜2 文 + ink-soft 非公式注記
│      CTA row（作成導線なし、/about + /help/manage-url 誘導）
│  右: MockBook コンポーネント
│      white card + shadow + radius-lg、grid 1.1fr/1fr
│      左: タイトル + 日付 font-num + ワールド名 font-num
│      右: gradient placeholder cover photo (3/4)
│      PC のみ: 後ろに rotate(6deg)/(-3deg) の小カード x2
├─ THUMB STRIP（mobile 4 cell / PC 5 cell）
│  gradient placeholder photos 1:1
│  上に teal caption（mobile のみ、sparkle icon）
├─ FEATURES（white card 包み、mobile 2 col / PC 4 col）
│  各セル: ico (teal-soft 円) + t + s
│  4 項目: ログイン不要 / 管理 URL で編集 / 公開範囲 / 通報窓口
├─ POLICY（list 3 項目 + 注記）
│  Terms / Privacy / 通報窓口の説明
├─ CTA BLOCK（teal-soft 背景 card）
│  eyebrow "サービスの全体像をまず確認"（作成 CTA でなく /about 誘導）
├─ TRUST STRIP（4 cell horizontal、共通 TrustStrip コンポ）
│  完全無料 / スマホで完結 / 安全・安心 / VRC ユーザー向け
└─ COMMON FOOTER（PublicPageFooter、showTrustStrip=false）
```

### 3.2 `/about`

```
┌─ HEADER: eyebrow "About" + h1 + 1 文要約
├─ サービスの位置づけ card
│  text-h2 + dashed-border 区切り（1 箇所のみ）の中で:
│   非公式注記 / 運営者 ERENOA / 連絡用 X @Noa_Fortevita / 目的 / MVP 注記
├─ できること（feature card grid: mobile 2 / PC 3、6 項目）
│  各セル: ico (teal-soft 円) + t + s
├─ MVP ではできないこと（surface-soft card grid 2 col、4 項目）
│  ico はグレー (ink-soft) で「制約」を視覚化
├─ ポリシーと窓口（list、`/help/manage-url` 風）
├─ TRUST STRIP
└─ COMMON FOOTER
```

### 3.3 `/terms`

```
┌─ HEADER: eyebrow "Terms" + h1 + "最終更新: 2026-05-01"
├─ NOTICE BOX（surface-soft + warn icon）
│  「個人運営の非公式 / 専門家レビュー前 / ローンチ後改訂」
├─ TOC（目次、teal accent text-sm anchor リスト、第 1〜9 条）
├─ ARTICLES（PolicyArticle コンポで第 1〜9 条）
│  各セクション: h2 (text-h2) + body + bullet
│  セクション間: divider-soft 1px
└─ COMMON FOOTER（trust-strip なし）
```

### 3.4 `/privacy`

```
┌─ HEADER + NOTICE BOX（同上、eyebrow "Privacy"）
├─ TOC（第 1〜10 条）
├─ ARTICLES（PolicyArticle、第 1〜10 条）
│  特徴: 第 5 条「外部サービス」は brand 名を font-num + teal-soft chip
└─ COMMON FOOTER（trust-strip なし）
```

### 3.5 footer（`PublicPageFooter`）

```
┌─ TRUST STRIP（任意 prop showTrustStrip で切替、LP / About のみ）
├─ NAV LINKS（5 リンク水平、center 揃え）
│  Top / About / Terms / Privacy / 管理 URL ヘルプ
└─ COPYRIGHT LINE（text-xs ink-soft）
```

## 4. 既存ページとの温度感整合

| 既存ページ | 整合方針 |
|---|---|
| `/help/manage-url` | 整合源として尊重。Terms / Privacy の本文密度・h2 を揃える。help footer は `PublicPageFooter` に切替 |
| Viewer (`ViewerLayout.tsx`) | footer の nav リンクを `PublicPageFooter` 流用、通報リンクは個別保持 |
| Edit / Manage / ReportForm | 触らない（PR 範囲外）|

## 5. 使う / 使わない装飾

### 使う
- `brand-teal` 系 / `ink` 系 / `surface` 系 / `divider` 系
- `rounded-lg` (16px) / `rounded` (12px)
- `shadow-sm` / `shadow`
- gradient placeholder photos（`.photo.v-a〜v-e` 由来、Tailwind arbitrary `bg-[linear-gradient(...)]` で再現）— **mock-book / thumb strip 内の局所用途のみ**
- `font-num`（数値・日付・URL・ブランド名 chip）
- dashed border（About「サービスの位置づけ」card 内の 1 箇所のみ）
- PC mock-book の **小カード rotate のみ**（6deg / -3deg）

### 使わない
- ページ背景の gradient orb / bokeh / 紫グラデーション全面
- **48px 超や過剰な巨大見出し**（mobile h1 26px / PC h1 40px までは許容、prototype forward port として承認済）
- 装飾的回転 transform 多用（PC mock-book 小カードのみ許容）
- backdrop-blur（本 PR では出番なし）
- カード内カード 3 重ネスト

## 6. ページ別 見た目方針

| ページ | 主役 | 密度 | アクセント |
|---|---|---|---|
| `/` | hero + mock-book + features + trust strip | 中（hero 余白多め、features 中、CTA 強め）| teal メイン、cta-block の teal-soft 背景で 1 度強く打ち出す |
| `/about` | feature card grid + 位置づけ card | 中〜高 | teal（できる）/ ink-soft（できない）でセマンティクス分離 |
| `/terms` | 読み物階層と区切り | 低〜中 | eyebrow と h2 anchor のみ teal、本文 ink-strong |
| `/privacy` | 同上 + 第 5 条 chip | 同上 | 同上 + Cloudflare / Google Cloud は font-num chip |
| trust strip | LP / About の closing | 低 | teal icon |
| footer | nav 機能 | 最小 | 下線のみ |

## 7. Safari 確認観点（STOP ε で実機確認）

| 観点 | macOS Safari | iPhone Safari |
|---|---|---|
| Hero（mobile）h1 26px の改行破綻なし | — | 必須 |
| MockBook grid 1.1fr/1fr が縮まない | 必須 | 必須 |
| PC MockBook rotate 小カードがオーバーフローしない | 必須 | — |
| Thumb strip 4/5 col 切替の breakpoint | 必須（PC）| 必須（mobile）|
| feature-cell の `aspect-ratio` 不変 | 必須 | 必須 |
| Trust strip 4 cell horizontal、狭幅で重ならない（折り返し許容）| — | 必須 |
| Terms / Privacy 第 N 条 anchor scroll | 必須 | 必須 |
| gradient placeholder photos が flat color に degrade しない | 必須 | 必須 |
| `X-Robots-Tag: noindex, nofollow` / `<meta name="robots">` 維持 | 必須 | 必須 |
| `Referrer-Policy` 維持 | 必須 | 必須 |
| 横画面で hero 破綻なし | — | 必須 |
| ダークモード自動切替なし（light テーマ固定）| 必須 | 必須 |
| token / Cookie / 任意 ID / Secret 非露出 | 必須 | 必須 |

## 8. 実装対象ファイル

| ファイル | 種別 | 概要 |
|---|---|---|
| `frontend/app/page.tsx` | M | LP rebuild |
| `frontend/app/(public)/about/page.tsx` | M | About rebuild |
| `frontend/app/(public)/terms/page.tsx` | M | Terms rebuild + TOC |
| `frontend/app/(public)/privacy/page.tsx` | M | Privacy rebuild + TOC + chip |
| `frontend/app/(public)/help/manage-url/page.tsx` | M | footer を PublicPageFooter に切替 |
| `frontend/components/Public/PublicPageFooter.tsx` | M | `showTrustStrip` prop 追加 |
| `frontend/components/Public/TrustStrip.tsx` | A | 4 cell horizontal trust strip |
| `frontend/components/Public/MockBook.tsx` | A | LP 用 mock-book（mobile / PC 両対応 + gradient placeholder）|
| `frontend/components/Public/PolicyArticle.tsx` | A | Terms / Privacy 共通の第 N 条 |
| `frontend/components/Public/SectionEyebrow.tsx` | A | eyebrow 1 行コンポーネント |
| `frontend/components/Viewer/ViewerLayout.tsx` | M | footer を PublicPageFooter 流用 + 通報リンク個別保持 |
| `frontend/app/__tests__/public-pages.test.tsx` | M | tests 再構築 |
| `frontend/components/Public/__tests__/MockBook.test.tsx` | A | SSR レンダリング |
| `frontend/components/Public/__tests__/TrustStrip.test.tsx` | A | SSR レンダリング |
| `frontend/components/Public/__tests__/PolicyArticle.test.tsx` | A | SSR レンダリング |
| `frontend/components/Public/__tests__/PublicPageFooter.test.tsx` | A or M | trustStrip prop の有無テスト |

migration / Backend / API / Workers binding / Secret / env / Job 一切なし。

## 9. テスト / build 方針

- `npm --prefix frontend run typecheck`
- `npm --prefix frontend run test`
- `npm --prefix frontend run build`
- `npm --prefix frontend run cf:build`
- `npm --prefix frontend run cf:check`
- HTML 検査（metadata title / `<meta name="robots">` / `data-testid`）
- redact 対象値 / Secret literal / 完全 UUID grep

## 10. STOP 段取り

| STOP | 内容 |
|---|---|
| α | 本書承認（**承認済 2026-05-01**）|
| β | Frontend 実装 + tests + build 全 PASS + 単一 commit + push |
| γ | 不要 |
| δ | Workers redeploy（`cf:build` + `wrangler deploy`）|
| ε | macOS Safari + iPhone Safari で §7 観点を実機確認 |
| final closeout | work-log + roadmap §1.1 Worker version 更新 + §1.3 design rebuild 完了扱い + failure-log の運用ルールが効いた事例 + commit + push |

rollback: 現 active Workers `6f1e82d7-cf57-41ab-99dd-0ede5266a3a5`。Backend / Cloud Run / Cloud SQL は不変。

## 11. 制約遵守

- raw 値 / token / Cookie / Secret / 完全 hash 等は出さない
- 仮置き値（運営者 `ERENOA` / 連絡用 X `@Noa_Fortevita` / 準拠法）は user 承認済の公開対象値で継続使用
- `.claude/scheduled_tasks.lock` は触らない / commit しない
- author kento-matsunaga 単独、Co-Authored-By なし
- design-system 準拠 + prototype 採用画面 ID 明示 + 既存ページ温度感整合 の **3 軸**を踏襲

## 12. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-01 | 初版（STOP α 承認版）。failure-log §5 のルール適用初事例として、画面別ワイヤーフレーム + 採用 prototype 画面 ID + 既存温度感整合を実装前に固定 |
