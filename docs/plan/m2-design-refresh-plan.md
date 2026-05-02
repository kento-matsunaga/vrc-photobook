# M2 design refresh PR 計画書（m2-design-refresh）— **v2**

> 作成: 2026-05-03 / **v2 更新: 2026-05-03**
> 状態: **STOP α（設計判断資料）** ユーザ承認待ち。STOP β 以降の実装は本書承認後に着手
> 起点: M2 ローンチ前運用整備フェーズ完了 (`489c532`、Backend / Workers / Job + 全 hotfix live、Chrome/Edge/Safari smoke 全 PASS) のうえで、design/source/ から取得した最新 wireframe + visual design に既存 Frontend を全面 align する作業
>
> v2 変更点（2026-05-03 audit 反映）:
> - **§0 / §1 に「design はそのまま / 足りないものは足す」正典方針を追加**
> - **§4 を 16 artboard 全件 + numbering 4.1〜4.14 + design source file:line range 付きで全面差替え**（PC / Mobile を別行で分離、design archive を予測ではなく archive 通りに固定）
> - **§5 共通 components / shell / token / TopBar nav / Footer / Turnstile / MockBook を file:line link 付きで整理**
> - §6 STOP β-1〜β-6 に numbering を反映、route handler artboard と reference-only artboard の扱い明記
> - **§8 Mobile 寸法を 390×844 → 360×740 に修正**（design archive HTML 正典）
> - §10 open questions に Q-A〜Q-G の user 判断（採用済み解決方針）を反映
>
> 関連 docs:
> - `design/source/README.md` — design 配信元の取扱い指示（claude.ai/design 由来の handoff bundle）
> - `design/source/chats/chat1.md` — design iteration 履歴（最終形は「ティール基調 + ワイヤーフレーム構造維持」+ Mac 風 chrome 削除）
> - **`design/source/project/VRC PhotoBook Wireframe.html`** — **正典 entry**（artboard 構成 / 寸法 / 各 artboard 名 numbering の確定根拠）
> - `design/source/project/wireframe-styles.css` — **design token の正典**（color / radius / shadow / typography / shell 全部）
> - `design/source/project/wf-shared.jsx` — Mobile / PC shell + Footer + Turnstile + Img + Section + Icon 正典
> - `design/source/project/wf-screens-a.jsx` — 4.1 Landing / 4.2 Create / 4.3 Draft route handler / 4.4 Prepare の 4 numbering 7 artboard
> - `design/source/project/wf-screens-b.jsx` — 4.5 Edit / 4.6 公開完了 / 4.7 Manage route handler / 4.8 Manage の 4 numbering 7 artboard
> - `design/source/project/wf-screens-c.jsx` — 4.9 Viewer / 4.10 Report / 4.11 About / 4.12 Help / 4.13 Terms / 4.14 Privacy + 共通 ErrorState の 6 numbering + 1 共通 = 13 artboard
> - `design/source/project/wf-flows.jsx` — overview の sitemap & primary flow（**production 実装対象外、reference 専用**）
> - `design/usephot/` — 採用候補の **VRChat 実写 PNG 7 枚**（縦 4、横 3、各 9〜14 MB、合計約 78 MB）
> - `design/wireframes/system-wireframe-brief.md` — wireframe brief（既存）
> - `design/design-system/{colors,typography,spacing,radius-shadow}.md` — PR25b 既存 token、本書で更新
> - `docs/plan/vrc-photobook-final-roadmap.md` §1.1 / §1.3 — LP final design pass の正典
> - `harness/failure-log/2026-05-01_pr37-public-pages-design-mismatch.md` §5 — design 適用ルール、本 PR で継続適用
>
> 関連 ADR:
> - `docs/adr/0001-tech-stack.md` — Frontend = Next.js + Cloudflare Workers / OpenNext
> - `docs/adr/0003-frontend-token-session-flow.md` — Cookie / token UI 取扱い
>
> 関連 rules:
> - `.agents/rules/safari-verification.md` — Safari / iPhone Safari 必須確認
> - `.agents/rules/security-guard.md` — Cookie / token / Secret 不記録
> - `.agents/rules/state-restore-on-reload.md` — server ground truth 維持（design 変更で UI が ground truth から外れない）
> - `.agents/rules/predeploy-verification-checklist.md` — deploy 完了基準
> - `.agents/rules/pr-closeout.md` — PR 完了処理

---

## 0. 本計画書の使い方 + 正典方針

ユーザの直接指示: **「機能やワイヤーフレームはそのまま。変えるのは design のみ」**。design/source/chats/chat1.md の最終 user 発言と整合。

本書は **STOP α 設計判断資料**として扱い、全実装は STOP β 以降。STOP α 段階で以下を user に明示確認したうえで実装着手。

### 0.1 正典方針（v2 で追加 / 必須）

> **design はそのまま / 足りない production 要件は補助 UI として足す**。

具体的には:

- ✅ **`design/source/project/VRC PhotoBook Wireframe.html` + imports を visual oracle とする**（design archive そのものを正典、予測 / 解釈は禁止）
- ✅ **design の layout / visual hierarchy / spacing / color / typography / artboard 構造は原則そのまま再現する**
- ✅ ただし **production として不足している以下は追加してよい**:
  - 法務・規約上必要な説明
  - 既存実装で必要な状態表示
  - エラー復旧導線
  - accessibility
  - data-testid / test hook
  - 既存機能を壊さないための補助 UI
- ✅ **追加 UI は design を置き換えず**、補助文言・secondary text・notice・status chip など既存 design slot の意味づけで足す
- ❌ **design にある要素を production 都合で消す場合は勝手に削らず**、必ず「なぜ消すか」「代替 UI は何か」を user 判断事項として §10.2 に明記する

### 0.2 design source の参照規則

- design source の file:line 引用は **`rg` / `nl -ba` で実 file から確定したものだけを使う**（推測 / memory による line number は禁止）
- 関数名 / class 名は **archive の表記をそのまま使う**（type-correction / case-correction を含めて archive 一致）
- design archive にある要素 (button / radio / checkbox / icon / 文言) を実装する際は、archive と production 文言が衝突する場合 §0.1 に従い「追加」で解決する

---

## 1. スコープ宣言

### 1.1 本 PR で **やること**

- 既存 frontend の **見た目だけ** を `design/source/project/` の wireframe / visual design に align
- **16 artboard**（13 機能画面 × PC + Mobile = 26 view + 4.3 draft route handler 1 artboard + 4.7 manage route handler 1 artboard + 共通 ErrorState 1 artboard、加えて reference 専用の overview 1 artboard）を adoption 範囲に含めて scope 整理
- `tailwind.config.ts` の design token を新仕様に更新（既存 PR25b token は前提値、design token と統合）
- 共通 components (TopBar / Footer / Card / Button / Notice / Badge / Section / Turnstile shell / MockBook / etc) を新 design 準拠に refactor
- `design/usephot/` 7 枚を LP / sample / 公開 Viewer の placeholder 位置で実写採用（圧縮 + Workers static asset 配信）
- iPhone / Android Safari / Chrome / Edge 全部で崩れないよう responsive 確認
- §0.1 に従い、**法務 / 既存機能 / 状態表示 / エラー復旧導線 / accessibility / test hook を補助 UI として追加**

### 1.2 本 PR で **やらないこと**

- **routing 変更**（`app/**` の path 構造は不変）
- **API 変更**（Backend は一切触らない、Cloud Run / Workers 設定変更なし）
- **business logic 変更**（rights agreement / OCC / publish flow / attach-images / reload restore は不変）
- **`data-testid` 削除**（既存 test が依存、追加は OK）
- **SSR markup assertion を破る変更**（既存 test の `expect(html).toContain(...)` 文字列 / class はそのまま動くこと）
- **Cookie / Secret / token 取扱変更**
- **Backend deploy / Workers Secrets 変更 / Scheduler 変更 / Job execute / DB migration**
- **design/source/project/uploads/** の流用（プレースホルダ画像は既に除外、`design/usephot/` を採用）
- **flow-primary artboard の production 反映**（design archive 内 reference 専用、§4 に明記）
- **/draft / /manage Route Handler artboard の spinner UI 実装**（既に HTTP 302 redirect の Route Handler 実装済、§4 / §6 に明記）

### 1.3 design medium について

`design/source/README.md` 明記:
> The design medium is HTML/CSS/JS — these are prototypes, not production code. Your job is to recreate them pixel-perfectly in whatever technology makes sense for the target codebase. Match the visual output; don't copy the prototype's internal structure unless it happens to fit.

→ design の `.jsx` 構造は **参考のみ**。production code は既存 React/Next.js component 構造を維持しつつ、**Tailwind class と CSS token を design に揃える**形で実装する。

---

## 2. design token（正典化方針）

### 2.1 design 側の正典（`design/source/project/wireframe-styles.css:7-51` `:root` 抽出）

**Color**:
| 用途 | token | hex | source line |
|---|---|---|---|
| primary | `--teal-500` | `#15B2A8` | 13 |
| primary hover | `--teal-600` | `#0E988F` | 14 |
| primary deep | `--teal-700` | `#0A7A73` | 15 |
| accent soft | `--teal-50` | `#EDFAF8` | 8 |
| accent soft 2 | `--teal-100` | `#D4F2EE` | 9 |
| accent border | `--teal-200` | `#A8E5DD` | 10 |
| accent line | `--teal-300` | `#6FD2C5` | 11 |
| accent middle | `--teal-400` | `#3CC1B1` | 12 |
| accent darkest | `--teal-800` | `#095F59` | 16 |
| ink primary | `--ink` | `#0F2A2E` | 18 |
| ink 2 (subtitle) | `--ink-2` | `#2C4A4F` | 19 |
| ink 3 (caption) | `--ink-3` | `#5C7378` | 20 |
| ink 4 (mute) | `--ink-4` | `#8FA2A6` | 21 |
| line | `--line` | `#E1E8EA` | 22 |
| line 2 | `--line-2` | `#ECF1F2` | 23 |
| line 3 | `--line-3` | `#F4F7F8` | 24 |
| bg | `--bg` | `#F6F9FA` | 26 |
| paper | `--paper` | `#FFFFFF` | 27 |
| soft | `--soft` | `#F0F6F7` | 28 |

**Radius / Shadow / 凡例 alias**: `wireframe-styles.css:30-50`
- radius: `12 / 16 / 20px`（`--radius` / `--radius-lg` / `--radius-xl`）
- shadow 3 段階（`--shadow-sm` / `--shadow` / `--shadow-lg`）
- legacy alias `--wf-bg` / `--wf-paper` / `--wf-ink` / etc は production では使わない（Tailwind class に置換）

**Typography**: `wireframe-styles.css:54` `.wf-root, .wf-root *`: `Hiragino Sans, Noto Sans JP, -apple-system, system-ui, sans-serif`
**Heading sizes**: `wireframe-styles.css:351-369` h1 30px (`.lg` 42px) / h2 18px / eyebrow 11px upper / sub 13.5px

### 2.2 既存 PR25b token との差分

| token | 既存 | design | 整合方針 |
|---|---|---|---|
| primary teal | `#14B8A6` (`brand.teal`) | `#15B2A8` (`--teal-500`) | **design に揃える** (`#15B2A8`)。既存 4 段階 ramp は teal-50/100/200/300/400/500/600/700/800 の 9 段階に拡張 |
| accent soft | `#E6F7F5` (`brand.teal-soft`) | `#EDFAF8` (`--teal-50`) | design に揃える |
| ink primary | `#0F172A` (`ink.DEFAULT`) | `#0F2A2E` (`--ink`) | design に揃える（緑寄り、teal アクセントと整合） |
| ink subtitle | `#334155` (`ink.strong`) | `#2C4A4F` (`--ink-2`) | design に揃える |
| ink caption | `#64748B` (`ink.medium`) | `#5C7378` (`--ink-3`) | design に揃える |
| ink mute | `#94A3B8` (`ink.soft`) | `#8FA2A6` (`--ink-4`) | design に揃える |
| line | `#E5EAED` (`divider.DEFAULT`) | `#E1E8EA` (`--line`) | design に揃える |
| bg | `#F7F9FA` (`surface.soft`) | `#F6F9FA` (`--bg`) | design に揃える（差は 1）|
| violet (manage URL) | `#8B5CF6` (`brand.violet`) | （manage は別経路、design 内では同色維持） | 維持 |
| status error | `#EF4444` | （inline） | 維持（業界標準 red） |
| status warn | `#D97706` | inline `#B25A00` (text on `#FFF4E5`) | design の **warn** 配色を採用、status warn は維持 |
| h1 | 24px / 700 | 30px (lg 42px) / 800 | design に揃える |
| h2 | 18px / 700 | 18px / 700 | 一致 |
| body | 14px | 13.5px | 14px 維持（既存 component の高さ整合） |
| sm / xs | 12 / 11px | 11.5 / 10.5px | 12 / 11px 維持（最小読みやすさ） |
| radius default | 12 / 16 / 20 / 8 | 12 / 16 / 20 | 一致（8px は sm として維持） |
| shadow | sm / DEFAULT 2 段 | 3 段（sm / DEFAULT / lg） | **lg 追加** |

→ **方針**: 既存 PR25b token を **design 値で上書き更新**。token 名は既存（`brand.teal`, `ink`, `surface`, `divider`）を維持し、値だけ swap。9 段階 ramp `teal-50..800` を新規追加。`shadow.lg` 追加。

### 2.3 採用しない既存 token

なし（業界標準 status error / brand.violet は維持）。

### 2.4 token 更新の影響範囲

- `frontend/tailwind.config.ts` — 値 swap + ramp 追加
- `frontend/app/globals.css` — base color / font-family を確認
- 既存 component の class（`bg-brand-teal` / `text-ink-medium` / `border-divider` 等）はそのまま動く（**class 名不変、値だけ変わる**）
- ただし `bg-brand-teal-soft`（`#E6F7F5` → `#EDFAF8`）等は微妙に視覚変化

---

## 3. design/usephot/ 配信戦略

### 3.1 採用画像

`design/usephot/` 配下の VRChat 実写 PNG 7 枚:

| ファイル名（先頭省略） | 解像度 | 元サイズ |
|---|---|---|
| `82E37915-...VRChat_2026-04-06_23-37-30.108_2160x3840.png` | 2160×3840 (縦) | 14.2 MB |
| `VRChat_2026-03-03_22-55-45.806_2160x3840.png` | 2160×3840 (縦) | 11.1 MB |
| `VRChat_2026-03-13_14-03-24.992_3840x2160.png` | 3840×2160 (横) | 10.4 MB |
| `VRChat_2026-03-22_23-48-33.324_2160x3840.png` | 2160×3840 (縦) | 9.9 MB |
| `VRChat_2026-03-27_00-02-57.153_2160x3840.png` | 2160×3840 (縦) | 10.4 MB |
| `VRChat_2026-03-27_00-07-06.943_2160x3840.png` | 2160×3840 (縦) | 12.5 MB |
| `VRChat_2026-04-14_15-59-36.459_3840x2160.png` | 3840×2160 (横) | 10.0 MB |

合計約 78 MB。**そのまま frontend bundle に含めるのは絶対 NG**（Workers asset は max 25 MB 等の制約 + bundle size 観点 + LCP 観点）。

### 3.2 配信戦略の選択肢

| 案 | 内容 | 長所 | 短所 |
|---|---|---|---|
| **A. 事前圧縮 → public/ に静的配信（推奨）** | `design/usephot/` を `frontend/public/img/landing/*.{webp,jpg}` に **事前圧縮** + 解像度 multi-variant 生成（hero / mock / card / thumb の 4 サイズ × WebP/JPEG fallback）。`<img srcset>` で responsive | Cloudflare Workers の static asset として直接配信、別 service 不要、CDN cache OK | bundle size +1〜3 MB（圧縮後）、画像追加時に手動再生成 |
| B. R2 経由（OGP_BUCKET 流用 or 新 bucket） | R2 に upload、Workers の R2 binding 経由で変換配信 | bundle size 0、R2 storage は格安 | 新規配信 endpoint / cache 設計、R2 binding の cors / cache header 設計、deploy 複雑化 |
| C. Next.js Image (`next/image`) + CDN loader | Cloudflare Image Resizing or Vercel image optimizer | on-demand 最適化、複数 variant 自動 | OpenNext + Cloudflare Workers での `next/image` は loader 設定必要、課金が乗る可能性 |

**推奨**: **A 案（事前圧縮 + public 配信）**。
- LP / 公開 Viewer 等の image は **数枚固定 + 解像度有限**（hero 1 枚、sample strip 4 枚程度）
- 動的 user-uploaded image は既に R2 / image-processor / variant 経由（別系統）
- A 案で生成する image は LP placeholder のみで、変更頻度 ≪ 1 回 / month 想定

### 3.3 圧縮仕様（A 案）

各 image を以下 variant に事前生成して `frontend/public/img/landing/` に配置:

| variant | 用途 | 解像度 (横長) | 解像度 (縦長) | format | 期待サイズ |
|---|---|---|---|---|---|
| `hero` | LP main hero (PC 1280px / Mobile fullwidth) | 1920×1080 | 1080×1920 | WebP q80 + JPEG q85 fallback | 各 ~150 KB |
| `mock` | MockBook 表紙 / spread | 800×1200 | 800×1200 | 同上 | 各 ~80 KB |
| `card` | sample strip 4 枚 / feature card | 600×800 | 600×800 | 同上 | 各 ~60 KB |
| `thumb` | thumbnail / sample tile | 240×320 | 240×320 | 同上 | 各 ~20 KB |

合計 7 枚 × 4 variant × 2 format = 56 file、合計約 4 MB（圧縮後）。bundle 増加分は許容範囲（現 Workers Total Upload 4875 KiB に +4 MB → 約 9 MB、Workers limit 25 MB / 1 file 5 MB を考慮し各 file 1 MB 以下を維持）。

圧縮ツール:
- `cwebp` (WebP encoder) + `cjpeg` (mozjpeg) を使う簡易 shell script で生成
- `frontend/scripts/build-landing-images.sh` を新設（generated artifact、git で配信先 `public/img/landing/` のみ commit）
- 元 PNG (`design/usephot/`) は git 管理する/しないを §3.4 で判断

### 3.4 元 PNG の git 取扱い（**v2: design/usephot は git 除外確定**）

- **`design/usephot/` を `.gitignore` 追加**: 元写真は user の手元のみ、変換結果のみ commit。理由: (1) 78 MB は git repo に重い、(2) ChaeckImage / TESTImage と同方針、(3) 個人写真の merge / branch 管理リスク回避
- **`design/source/` は現状維持で git 管理**: 356 KB、archive そのまま、design 正典として参照する

### 3.5 photo の権利

`design/usephot/` の VRChat 実写は user 本人撮影 + 自身のアバターのみ含むものを user が選定済（`docs/plan/vrc-photobook-final-roadmap.md` §1.3 で「採用画像素材として user-local VRChat photo folder の利用許可を user から受領済」記載）。**実 Windows パスは redact 表記、commit / docs に raw パス記載なし**。本 PR でも同方針継続。

---

## 4. 16 artboard 完全 linking table（PC ↔ Mobile 別行 / design source file:line 明示）

### 4.1 design archive 正典寸法（`design/source/project/VRC PhotoBook Wireframe.html:31-34` 抽出）

| 種別 | 寸法 |
|---|---|
| **Mobile artboard** | **360 × 740** (`M_W = 360, M_H = 740`) |
| **PC artboard** | **1280 × 820** (`PC_W = 1280, PC_H = 820`) |
| **Route Handler artboard** | **540 × 360** (`ROUTE_W = 540, ROUTE_H = 360`) |
| **Flow / overview** | **1240 × 1100** (`FLOW_W = 1240, FLOW_H = 1100`) |
| **共通 ErrorState** | **920 × 620** (HTML 上の `width={920} height={620}` 直接指定) |

### 4.2 16 artboard linking table

design archive `VRC PhotoBook Wireframe.html` (101 行) で wire up された **16 artboard** を numbering / Mobile / PC / 既存 frontend / responsive 戦略の順で固定。

| numbering | screen | Mobile design (file:lines) | PC design (file:lines) | 既存 frontend file | responsive strategy |
|---|---|---|---|---|---|
| (overview) | Sitemap & Primary Flows (**reference 専用、production 実装対象外**) | — (PC/Mobile 区別なし) | `design/source/project/wf-flows.jsx:57-153` `Flow_PrimaryFlow` (1240×1100) | — (production 反映しない) | reference のみ。site map ナビゲーション等を design に合わせるための解説資料 |
| **4.1** | Landing | `design/source/project/wf-screens-a.jsx:45-118` `WFLanding_M` | `design/source/project/wf-screens-a.jsx:119-203` `WFLanding_PC` | `app/page.tsx` + `components/Public/{MockBook,SectionEyebrow,TrustStrip,PublicPageFooter}.tsx` | base + `sm:` 以下 → Mobile / `lg:` 以上 → PC（PC: hero `wf-grid-2` 2 列 / sample strip 5 列 / 特徴 4 列 / 用途 3 列、Mobile: 縦 stack + 4 列 sample / 特徴 / 用途 縦 card） |
| **4.2** | Create | `wf-screens-a.jsx:206-254` `WFCreate_M` | `wf-screens-a.jsx:255-308` `WFCreate_PC` | `app/(public)/create/page.tsx` + `CreateClient.tsx` | Mobile: type radio 縦 stack / PC: `wf-grid-3` 3 列 + `narrow` container |
| **4.3** | **/draft/{token} Route Handler** | (PC/Mobile 共通の単独 artboard) `wf-screens-a.jsx:311-331` `WFDraftExchange` (540×360) | 同左 | `app/(draft)/draft/[token]/route.ts` | **production 実装は HTTP 302 redirect、UI なし**。design artboard は spinner UI 付きだが「Route Handler — UI なし」と annotation（line 318）で明記。**production には spinner artboard を反映しない**（既に server-side redirect で処理）。design は仕様 reference として参照する |
| **4.4** | Prepare | `wf-screens-a.jsx:334-389` `WFPrepare_M` | `wf-screens-a.jsx:390-443` `WFPrepare_PC` | `app/(draft)/prepare/[photobookId]/PrepareClient.tsx` + `components/Prepare/{ImageTile,UploadQueue}.tsx` | Mobile: 縦 stack + bottom sticky CTA / PC: `wf-grid-2-1` (left content 2fr + right side rail 1fr) |
| **4.5** | Edit | `wf-screens-b.jsx:4-92` `WFEdit_M` | `wf-screens-b.jsx:93-194` `WFEdit_PC` | `app/(draft)/edit/[photobookId]/EditClient.tsx` + `components/Edit/{PhotoGrid,CoverPanel,PublishSettingsPanel,CaptionEditor,ReorderControls}.tsx` | Mobile: 全 panel 縦 stack / PC: `wf-grid-1-2-1` (`250px 1fr 310px`) 3 列 |
| **4.6** | 公開完了 view | `wf-screens-b.jsx:197-246` `WFComplete_M` | `wf-screens-b.jsx:247-293` `WFComplete_PC` | `components/Complete/CompleteView.tsx` | Mobile: stack + 各セクション独立 / PC: 公開URL + 管理URL を `wf-grid-2` 2 列で並列 |
| **4.7** | **/manage/token/{token} Route Handler** | (PC/Mobile 共通の単独 artboard) `wf-screens-b.jsx:296-316` `WFManageExchange` (540×360) | 同左 | `app/(manage)/manage/token/[token]/route.ts` | **production 実装は HTTP 302 redirect、UI なし**（同 4.3 同方針）。design は spec reference として参照 |
| **4.8** | Manage | `wf-screens-b.jsx:319-364` `WFManage_M` | `wf-screens-b.jsx:365-411` `WFManage_PC` | `app/(manage)/manage/[photobookId]/page.tsx` + `components/Manage/HiddenByOperatorBanner.tsx` 等 | Mobile: 情報 panel 縦 stack（公開写真数 / 公開設定 / 管理 ver / 公開日時 を行ごと）/ PC: `wf-grid-2-1` + 情報 panel は `wf-grid-4` 4 列の数字 grid |
| **4.9** | Public Viewer | `wf-screens-c.jsx:4-33` `WFViewer_M` | `wf-screens-c.jsx:34-76` `WFViewer_PC` | `app/(public)/p/[slug]/page.tsx` + `components/Public/*.tsx` | Mobile: `aspectRatio:'4/5'` cover + 縦に Page 01-03 / PC: `wf-grid-2-1` で **左 = cover + Pages、右 = sticky Creator card + Report link**（**新規 sticky panel component 追加要、Q-G 採用方針**）|
| **4.10** | Report | `wf-screens-c.jsx:79-129` `WFReport_M` | `wf-screens-c.jsx:130-176` `WFReport_PC` | `app/(public)/p/[slug]/report/page.tsx` + `components/Report/ReportForm.tsx` | Mobile: reason radio 縦 stack 6 件 / PC: reason `wf-grid-2` 2 列、Detail / Contact 同 box 内 |
| **4.11** | About | `wf-screens-c.jsx:179-227` `WFAbout_M` | `wf-screens-c.jsx:228-273` `WFAbout_PC` | `app/(public)/about/page.tsx` + `components/Public/PolicyArticle.tsx` | Mobile: できること 6 縦リスト + できないこと 4 縦リスト（design line 192-211）/ PC: `wf-grid-2` で 左右に並列（design line 240-259）|
| **4.12** | Help · 管理URL | `wf-screens-c.jsx:276-301` `WFHelp_M` | `wf-screens-c.jsx:302-328` `WFHelp_PC` | `app/help/manage-url/page.tsx` | Mobile: Q1〜Q6 縦 stack `wf-m-card` / PC: 同じく縦 stack `wf-box`、`narrow` container。**design の Q1〜Q6 構造を採用**（Q-D） |
| **4.13** | Terms | `wf-screens-c.jsx:331-357` `WFTerms_M` | `wf-screens-c.jsx:358-381` `WFTerms_PC` | `app/(public)/terms/page.tsx` + `PolicyArticle` | Mobile: 9 article + TOC `wf-toc` / PC: 同構造 + `narrow` |
| **4.14** | Privacy | `wf-screens-c.jsx:384-412` `WFPrivacy_M` | `wf-screens-c.jsx:413-442` `WFPrivacy_PC` | `app/(public)/privacy/page.tsx` + `PolicyArticle` | Mobile: 10 article + TOC + External services chips / PC: 同構造。**chips は design slot 採用、内容を production truth (PostHog/Sentry 削除) に合わせる**（Q-F） |
| (errors) | 共通 ErrorState (4種) | (PC/Mobile 共通の単独 artboard) `wf-screens-c.jsx:445-475` `WFErrorStates` (920×620、4 status を 2x2 grid で 1 artboard) | 同左 | `components/ErrorState.tsx` (variant prop で 401/404/410/500) | 既存 ErrorState は responsive 対応済、design archive は 4 status を 1 view に並べる説明 artboard、production は variant 別 render |

### 4.3 16 artboard / 26 view 数え方の整理

design archive `VRC PhotoBook Wireframe.html:42-91` で wire up される DCArtboard 総数:

| 種別 | 件数 | 内訳 |
|---|---|---|
| 13 機能画面 × PC + Mobile | **26 view** | 4.1 / 4.2 / 4.4 / 4.5 / 4.6 / 4.8 / 4.9 / 4.10 / 4.11 / 4.12 / 4.13 / 4.14 各 PC + Mobile = 12 × 2 = 24、+ Landing PC + Mobile = 2 → 計 26 ※ 13 画面ではなく **12 画面 × 2 view** が PC+Mobile 別 artboard、加えて 4.3 と 4.7 の Route Handler は単独 |
| Route Handler (4.3 / 4.7) | **2 artboard** | PC/Mobile 区別なし、共通 540×360 |
| ErrorState 4 in 1 | **1 artboard** | 920×620、4 status を 1 artboard 内 |
| Sitemap & Flow (overview) | **1 artboard** | 1240×1100、production 実装対象外 |
| **合計** | **30 unit / 16 distinct artboard** | (上記をまとめた dimention 数: 16) |

実装対象の **production view** 数:
- 4.1 Landing: 2 (PC + Mobile)
- 4.2 Create: 2
- 4.4 Prepare: 2
- 4.5 Edit: 2
- 4.6 Complete: 2
- 4.8 Manage: 2
- 4.9 Viewer: 2
- 4.10 Report: 2
- 4.11 About: 2
- 4.12 Help: 2
- 4.13 Terms: 2
- 4.14 Privacy: 2
- 4.3 Draft Route Handler: HTTP 302 のみ（既実装、UI なし）
- 4.7 Manage Route Handler: 同上
- 共通 ErrorState: 4 variant

→ **PC + Mobile 別 view = 24 view、加えて ErrorState 4 variant、Route Handler 2 経路、計 30 production unit**。

### 4.4 production 実装対象外 artboard

| artboard | 理由 |
|---|---|
| `flow-primary` (overview / Sitemap & Primary Flows) | design archive 内の **設計 reference** として site map / 主要遷移を可視化したもの。production には対応する page なし |
| `draft-exch` (4.3 Draft Route Handler の spinner artboard) | production は HTTP 302 redirect で UI を出さない実装が既に live。design 上の spinner artboard は spec reference |
| `manage-exch` (4.7 Manage Route Handler の spinner artboard) | 同上 |

---

## 5. 共通 components / shell / token のリスタイル順序（design source file:line link 付き）

新 design token 適用後、以下を順に restyle。design source link は `wireframe-styles.css` / `wf-shared.jsx` の正典に紐付け。

### 5.1 design source 正典マップ

| design 要素 | 定義場所（file:line） |
|---|---|
| **token** (`:root`) | `wireframe-styles.css:7-51` |
| **Mobile shell** (`.wf-m` / `.wf-m-topbar` / `.wf-m-scroll`) | `wireframe-styles.css:69-92` |
| **Mobile-specific** (`.wf-m-pad` / `.wf-m-card` / `.wf-m-stick-cta`) | `wireframe-styles.css:503-520` |
| **PC shell** (`.wf-pc` / `.wf-pc-app` / `.wf-pc-header` / `.wf-pc-logo` / `.wf-pc-nav` / `.wf-pc-container` + `.wf-pc-chrome` 余白) | `wireframe-styles.css:95-162` |
| **TopBar nav 正典** (logo + 「**作例 / 使い方 / よくある質問**」+ primary CTA「**無料で作る**」) | `wf-shared.jsx:29-48` `WFBrowser` |
| **Mobile TopBar** (logo + back / right slot) | `wf-shared.jsx:3-27` `WFMobile` + `wireframe-styles.css:76-92` |
| **Footer 正典** (Trust strip 4 chip「完全無料 / スマホで完成 / 安全・安心 / VRCユーザー向け」+ links「About / Help / Terms / Privacy」) | `wf-shared.jsx:64-84` `WFFooter` + `wireframe-styles.css:493-501` `.wf-footer` + `wireframe-styles.css:596-608` `.wf-trust` |
| **Turnstile widget shell** (確認 / 未確認 + Cloudflare Turnstile branding) | `wf-shared.jsx:86-98` `WFTurnstile` |
| **Image placeholder** (`wf-img` 対角クロス) | `wf-shared.jsx:50-52` `WFImg` + `wireframe-styles.css:178-206` |
| **Section primitive** (title + anno) | `wf-shared.jsx:54-62` `WFSection` |
| **Icon primitive** (`wf-feat-icon` 円形) | `wf-shared.jsx:101-121` `WFIcon` + `wireframe-styles.css:584-594` |
| **MockBook 構造** (left cover with title + right 2x2 grid 4 image, top spans full width) | `wf-screens-a.jsx:4-43` |
| **Card / Box** (`.wf-box` / `.lg` / `.dashed` / `.fill` / `.tinted`) | `wireframe-styles.css:165-175` |
| **Button** (`.wf-btn` / `.primary` / `.lg` / `.sm` / `.full` / `.disabled`) | `wireframe-styles.css:228-253` |
| **Form input** (`.wf-input` / `.wf-textarea` / `.wf-label` / `.wf-counter` / `.wf-hint`) | `wireframe-styles.css:256-286` |
| **Radio / Check** (`.wf-radio` / `.wf-radio.active` / `.wf-check` / `.wf-check.on`) | `wireframe-styles.css:289-334` |
| **Section title / heading** (`.wf-section-title` / `.wf-h1` / `.wf-h1.lg` / `.wf-h2` / `.wf-eyebrow` / `.wf-sub`) | `wireframe-styles.css:337-369` |
| **Badge / Chip** (`.wf-badge` / `.dark` / `.teal` / `.warn`) | `wireframe-styles.css:372-395` |
| **Note / Callout** (`.wf-note` / `.wf-note.warn`) | `wireframe-styles.css:398-425` |
| **Divider / Loading spinner** | `wireframe-styles.css:427-439` |
| **Upload tile** (`.wf-upload-tile` / `.failed` / `.done`) | `wireframe-styles.css:441-464` |
| **Page tile (editor)** (`.wf-page-tile` / `.active` / `.num`) | `wireframe-styles.css:467-490` |
| **Route handler card** (`.wf-route-card`) | `wireframe-styles.css:523-536` |
| **TOC** (`.wf-toc`) | `wireframe-styles.css:538-545` |
| **Error shell** (`.wf-error-shell`) | `wireframe-styles.css:547-562` |
| **Grid utils** (`.wf-grid-2` / `.wf-grid-3` / `.wf-grid-4` / `.wf-grid-2-1` / `.wf-grid-1-2-1` / `.wf-stack` / `.wf-row`) | `wireframe-styles.css:565-573` |
| **Annotation** (`.wf-anno`) | `wireframe-styles.css:576-581` |
| **CTA band** (`.wf-cta-band`) | `wireframe-styles.css:611-618` |

### 5.2 既存 frontend component との対応 + restyle 順序

| step | component | 既存 file | design source 参照 | restyle 内容 |
|---|---|---|---|---|
| 1 | `tailwind.config.ts` | `frontend/tailwind.config.ts` | `wireframe-styles.css:7-51` | §2 token 値 swap + 9 段階 teal ramp + shadow.lg 追加 |
| 2 | base layout / TopBar | `app/layout.tsx` + 各 layout file | `wf-shared.jsx:29-48` `WFBrowser` + `wireframe-styles.css:137-162` `.wf-pc-header` | logo + 「作例 / 使い方 / よくある質問」3 link + primary CTA「無料で作る」。Mobile は `wf-shared.jsx:3-27` `WFMobile` 構造（logo + back / right slot）|
| 3 | Footer | `components/Public/PublicPageFooter.tsx` | `wf-shared.jsx:64-84` `WFFooter` | Trust strip + links 構造 |
| 4 | Button | （Tailwind class 直接、専用 component 検討） | `wireframe-styles.css:228-253` | `wf-btn` / `.primary` / `.lg` / `.sm` / `.full` / `.disabled` 相当 class set |
| 5 | Card / Box | （Tailwind class 直接） | `wireframe-styles.css:165-175` | `wf-box` / `.lg` / `.dashed` / `.fill` / `.tinted` 相当 |
| 6 | Notice / Note | （新規 component） | `wireframe-styles.css:398-425` | `wf-note` / `.warn` 相当 component 新設 |
| 7 | Badge / Chip | `components/Manage/HiddenByOperatorBanner.tsx` 等 | `wireframe-styles.css:372-395` | `wf-badge` / `.dark` / `.teal` / `.warn` |
| 8 | Section title | `components/Public/SectionEyebrow.tsx` | `wireframe-styles.css:337-369` | `wf-section-title` / `wf-eyebrow` 相当 |
| 9 | Form input | `components/*/Form*.tsx` 等 | `wireframe-styles.css:256-286` | `wf-input` / `wf-textarea` / `wf-label` / `wf-counter` / `wf-hint` 相当 |
| 10 | Radio / Check | `CreateClient` 等 | `wireframe-styles.css:289-334` | `wf-radio` / `wf-radio.active` / `wf-check` 相当 |
| 11 | Page tile (editor) | `components/Edit/PhotoGrid.tsx` 等 | `wireframe-styles.css:467-490` | `wf-page-tile` / `.active` |
| 12 | Upload tile | `components/Prepare/ImageTile.tsx` | `wireframe-styles.css:441-464` | `wf-upload-tile` / `.failed` / `.done` |
| 13 | MockBook | `components/Public/MockBook.tsx` | `wf-screens-a.jsx:4-43` | LP 用 spread placeholder、`design/usephot/` から実写採用（圧縮 variant 利用）|
| 14 | TrustStrip | `components/Public/TrustStrip.tsx` | `wf-shared.jsx:64-84` `WFFooter` の trust 部 + `wireframe-styles.css:596-608` | 4 chip「完全無料 / スマホで完成 / 安全・安心 / VRCユーザー向け」 |
| 15 | CTA band | （新規 component） | `wireframe-styles.css:611-618` | `wf-cta-band` 相当（LP 下部）|
| 16 | Turnstile widget shell | `components/TurnstileWidget.tsx` | `wf-shared.jsx:86-98` `WFTurnstile` | 確認 / 未確認 表示 + Cloudflare branding（既存 widget 内部は Cloudflare 提供のため shell 部のみ design 化）|

### 5.3 既存 test 互換性

- **既存 SSR test の `expect(html).toContain(...)` で参照する文字列 / data-testid を変更しない**
- 既存 component の **public API（props / displayName / data-testid 命名）は維持**、内部 class のみ変更
- 万一文字列を変更する場合は test と同期 update（同 PR 内）

---

## 6. STOP 設計（β-1〜ε）

依存関係 + リスク順に並べる。各 STOP で commit + push、deploy は **STOP δ で 1 回まとめて**実施（feature flag なし、ビッグバン deploy）。

### STOP β-1: design token + base layout

- `tailwind.config.ts` token 値更新（§2.2）
- `app/layout.tsx` / `globals.css` / TopBar (logo + 作例 / 使い方 / よくある質問 + 無料で作る) / Footer / Button / Card 共通リスタイル
- Mobile shell (`.wf-m` 相当) / PC shell (`.wf-pc` 相当) の Tailwind 化
- 既存全画面が新 token で **動作する** ことを確認（崩れていてもよい、token swap が壊れていない確認）

### STOP β-2: 静的画面（4.1 LP / 4.11 About / 4.13 Terms / 4.14 Privacy / 4.12 Help）

- 5 画面 × PC + Mobile = 10 view + 既存 `Public/*` components リスタイル
- design/usephot 圧縮 + `public/img/landing/` 配置
- 4.1 LP の MockBook / sample strip / 特徴 / 用途 / CTA band / Trust strip 全部適用
- 4.11 About の「できること 6件 / MVPでできないこと 4件」構造（Q-C 採用）
- 4.12 Help の Q1〜Q6 構造（Q-D 採用）
- 4.14 Privacy chips を production truth に合わせる（Q-F 採用、PostHog/Sentry 削除）
- **PC + Mobile 両方の崩れ確認**（Chrome / Edge / iPhone Safari / Android Chrome）

### STOP β-3: 動線画面 1 (4.2 Create / 4.4 Prepare)

- 2 画面 × PC + Mobile = 4 view リスタイル
- Turnstile widget 周りのデザイン整合
- Upload tile / progress UI の visual update
- rights checkbox / reason 別文言は機能維持
- **4.3 Draft Route Handler の spinner artboard は production 実装対象外**（HTTP 302 redirect で済む既実装、§4.4 通り）

### STOP β-4: 動線画面 2 (4.5 Edit / 4.6 Complete / 4.8 Manage)

- 3 画面 × PC + Mobile = 6 view リスタイル
- 4.5 Edit PC は `wf-grid-1-2-1` (`250px 1fr 310px`) の 3 column 適用
- 4.5 `PublishSettingsPanel` リスタイル（**rights checkbox: design 短文を main label、production 長文を helper text として下に追加** = Q-A 採用）
- 4.5 「下書き保存」button（Q-B 採用、§10.2 で詳細）
- 4.6 `CompleteView` 完成画面リスタイル
- 4.8 `Manage/*` リスタイル
- **4.7 Manage Route Handler の spinner artboard は production 実装対象外**（同 4.3）

### STOP β-5: 公開動線 (4.9 Viewer / 4.10 Report) + Errors

- 2 画面 × PC + Mobile = 4 view リスタイル + Errors 1 共通
- 4.9 `Public/*` Viewer リスタイル（実写 cover + page tile）
- 4.9 PC sticky 右 panel 新規 component 追加（**Q-G 採用、Creator card + Report link + OGP annotation を design archive 通り**）
- 4.10 `ReportForm` リスタイル
- 共通 `ErrorState` の 401/404/410/500 リスタイル

### STOP β-6: 全画面 visual QA + 残り class 整理

- design と production の差を screen by screen で QA
- 不要 class / 旧 design 残骸を削除
- responsive breakpoint の最終調整（iPhone SE / iPhone 14 / Android Pixel / iPad / 1280px PC / 1920px PC）

### STOP γ: 検証 + commit + push（実装側 final）

- frontend `npm run build` / `tsc --noEmit` / `vitest run` / `cf:build` / `cf:check` 全 PASS
- 既存 vitest 296 件 PASS（restyle で broken しないこと）
- harness-class-guards.test.ts 等の既存 guard 維持
- 単一 / 複数 commit + push（branch 戦略は §9）

### STOP δ: Workers deploy（一括）

- cf:build + wrangler deploy
- production bundle marker grep（新 token / 新 class が live）
- route smoke（既存 + 新 design 確認）
- bindings / Secrets / env 維持確認
- Secret / raw 値 grep
- 完了報告

### STOP ε: 実機 smoke

- Chrome / Edge / Safari / iPhone Safari / Android Chrome
- **24 production view + ErrorState 4 variant** を全部目視確認（縦スクロール / 横はみ出し / 画像読込 / Turnstile widget / publish flow / reload 復元）
- **Backend は touched しない**ので Backend 側の re-smoke は不要

---

## 7. 既存 test への影響

### 7.1 維持必須

- 既存 vitest 29 files / 296 tests 全 PASS
- `harness-class-guards.test.ts` (旧曖昧文言不在 / SSR 用 fetchEditView 不使用 / CORS PATCH/DELETE)
- `EditClient.publish.test.ts` (rights checkbox / reason 別文言 source guard)
- `EditClient.reload.test.ts` (fetchEditViewClient 使用)
- `PublishSettingsPanel.test.tsx` (checkbox + hint + button disable 条件)
- `PrepareClient.test.tsx` (prepare-progress / prepare-normal-notice / prepare-slow-notice / 復元された画像 / 主要 testid)
- `MockBook.test.tsx` / `TrustStrip.test.tsx` / `PublicPageFooter.test.tsx` / `PolicyArticle.test.tsx`

### 7.2 同期 update 候補

- design に合わせて新規 component（`Notice` / `CtaBand` / `Tile` 共通）を導入する場合、対応する SSR test を追加
- token swap で背景色が変わる SSR test は assertion を update（class 名は維持）
- Q-A の rights checkbox 追加 helper text は SSR test も同期 update

### 7.3 追加 guard test 提案

- `frontend/__tests__/harness-class-guards.test.ts` に「**legacy class が混在しない**」guard を追加（例: 旧 `bg-brand-teal-soft` を `bg-teal-50` に migrate した場合、旧 class が残っていないことを scan）
- ただし migration 期間中は noisy になるので、STOP β-6 の最後で追加

### 7.4 SSR test の見直し

restyle で SSR markup 文字列が変わる箇所が出れば test も update。特に以下は要点検:
- `PrepareClient.test.tsx` 「合計 / 完了 / 処理中 / 失敗」表記の変更
- `PolicyArticle.test.tsx` の section heading 構造変更
- `PublishSettingsPanel.test.tsx` の checkbox 文言（**変えない、helper text を追加するのみ** = Q-A）

---

## 8. responsive 戦略

### 8.1 breakpoint

Tailwind 標準 breakpoint を採用:
- `sm:` 640px (大型 mobile / 小型 tablet)
- `md:` 768px (tablet)
- `lg:` 1024px (small desktop)
- `xl:` 1280px (PC default、design の PC artboard 幅)
- `2xl:` 1536px (large desktop)

### 8.2 design Mobile / PC 寸法のマッピング（**v2 修正済**）

- design Mobile artboard: **360 × 740**（`VRC PhotoBook Wireframe.html:31` `M_W = 360, M_H = 740`）→ **default + sm: 以下**
- design PC artboard: **1280 × 820**（同 line 32 `PC_W = 1280, PC_H = 820`）→ **lg: 以上**
- design は中間 (tablet) を持たないので、tablet は **mobile-first で md: までは Mobile 流用、lg: 以上で PC layout** に切替

### 8.3 iPhone / Android 崩れ対策

| 観点 | 対策 |
|---|---|
| iPhone Safari の `100vh` bug | `min-h-screen` の代わりに `min-h-[100dvh]` （dynamic viewport）使用、Tailwind 3.4+ |
| iPhone safe-area | `env(safe-area-inset-bottom)` を bottom CTA で考慮（既存 `m2-upload-staging` で対応済を継続） |
| Android Chrome の url bar 出入り | viewport 高さ依存の固定 layout を避け、内容で stretch |
| `<input type="file">` の Android picker | 既存 `accept="image/jpeg,image/png,image/webp"` 維持、Photos picker と Camera roll の両方が出ること確認 |
| HEIC reject 表示 | 既存「HEIC / HEIF は現在未対応です」表示維持 |
| 横画面 (landscape) | iPhone landscape で 全画面 layout が成立すること（縦 stack のまま、画像 max-width 維持）|
| 文字 overflow | 長い title / creator name が省略されるか折り返されるかを統一（CSS `text-overflow` / `word-break`）|

---

## 9. branch 戦略 / commit 粒度

### 9.1 branch

`origin/main` から派生 branch `m2-design-refresh`（仮）を作成。STOP β-1〜β-6 を 1 branch で進め、STOP γ で全 commit を整理して PR 化（self-review）→ main へ merge。

### 9.2 commit 粒度

| step | commit |
|---|---|
| design archive 保管 | `chore(design): import wireframe + visual handoff source under design/source/` |
| token swap | `feat(design): adopt new teal palette + ink/line tokens (PR25b values updated)` |
| 静的画面 | `feat(design): restyle landing / about / terms / privacy / help with new tokens` |
| Create / Prepare | `feat(design): restyle create / prepare flows` |
| Edit / Complete / Manage | `feat(design): restyle edit / complete / manage screens` |
| Viewer / Report / Errors | `feat(design): restyle public viewer / report / error states` |
| usephot 圧縮 + 配置 | `chore(design): generate landing image variants under public/img/landing/` |
| 既存 test 同期 | `test(design): sync SSR markup assertions with new structure` |
| harness guard 強化 | `chore(harness): add legacy class regression guard` |

各 commit に `Co-Authored-By` なし、scope は `design`（既存方針: feat/fix/chore/test/docs + scope）。

### 9.3 .gitignore / .gcloudignore 更新

- `.gitignore`: `design/usephot/` を追加（user 個人写真 78 MB を git に入れない）
- `.gcloudignore`: 既存 `frontend/` 全部除外しているため Backend deploy には影響しないが、明示性のため `design/usephot/` を追加してもよい
- `frontend/public/img/landing/` は git 管理対象（生成済 webp/jpeg）

---

## 10. リスク / open questions

### 10.1 リスク

| # | リスク | 緩和策 |
|---|---|---|
| R1 | token swap で既存画面の class 名は同一だが見た目が変わり、user 観測の差が出る | STOP β-1 完了時点で局所目視 + 既存 SSR test PASS を確認 |
| R2 | design/usephot の photo の権利問題（user 本人撮影 + 自身のアバターのみ） | `final-roadmap.md` §1.3 と整合、user 確認済 |
| R3 | 圧縮品質が design の visual 期待を下回る | 圧縮 script の output を user に visual 確認してもらう（STOP β-2 内 sub-step）|
| R4 | iPhone Safari の dvh / safe-area 不整合で bottom CTA が浮く | dvh + safe-area-inset 検証、iPhone 14 / iPhone SE 実機で確認（user）|
| R5 | publish flow の rights checkbox が新 visual に migration したときに動作 break | EditClient.publish.test.ts source guard で検知、STOP β-4 で重点確認 |
| R6 | design の PC chrome 削除指示（chat1.md 末尾）を実装で正しく反映しているか | design/source/wf-shared.jsx の `WFBrowser` を確認、production の PC layout はサイトヘッダ直接スタート |
| R7 | bundle size 増（image variant 4 MB + token CSS） | cf:check で Total Upload を計測、5 MB 以下を目標 |
| R8 | Workers asset 上限 (25 MB / file 5 MB) | webp/jpeg 1 file < 1 MB を維持 |
| R9 | 既存 PR37 design rebuild の component (`MockBook` 等) が新 design と互換でない | 個別 refactor、既存 test を維持 |
| R10 | Edit の 1-2-1 grid が 1024px〜1279px で崩れる | breakpoint を `lg:` に絞る + `xl:` で確認 |

### 10.2 user 判断事項（**v2: Q-A〜Q-G 採用方針確定**）

| Q | 内容 | 採用方針（v2） | 追加メモ |
|---|---|---|---|
| **Q-A** | rights checkbox 文言 | **design 短文 checkbox label を main として維持** + その下に **production 必須長文を helper text として追加** | design archive: `wf-screens-b.jsx:78` Mobile / `:185` PC で「権利・配慮について確認しました」/「権利・配慮について確認」。production 必須長文（業務知識 v4 §3.1 法務整合）「投稿する画像について必要な権利・許可を確認し、写っている人やアバター、ワールド等に配慮した内容であることを確認しました。」を **helper text** として下に置く。これで design を崩さず + 法務要件も満たす |
| **Q-B** | /edit 「下書き保存」button | **design 上の button slot は維持**。実装方針:**(1) settings dirty 時のみ「下書き保存」action として動作 / (2) 通常は「保存済み / 自動保存中」status 表示**。既存 auto-save 設計と衝突する場合は user 判断事項に escalate | design archive: `wf-screens-b.jsx:80` Mobile / `:103` PC に「下書き保存」button あり。production は **draft 自動保存 + 明示 save 不要設計**。STOP β-4 で実装方針を最終確定（推奨: design slot に「保存済み」/「変更を保存」を context 切替表示する secondary button） |
| **Q-C** | About 「できること 6件」「MVPでできないこと 4件」 | **design 構造を採用**。既存 content で不足する情報があれば、design card 構造を壊さず追加・差し替え | design archive: `wf-screens-c.jsx:192-211` Mobile / `:240-259` PC。既存 `/about` page の section 数 / content は STOP β-2 で再点検し、design 6 + 4 構造に統合 |
| **Q-D** | Help (/help/manage-url) Q1〜Q6 構造 | **design Q1〜Q6 構造を採用**。既存 FAQ で必要なものは Q1〜Q6 に統合または補助 section として追加 | design archive: `wf-screens-c.jsx:277-284` (Mobile sections array) / `:303-310` (PC) で「公開URLと管理用URLの違い / 管理用URLは再表示不可 / 紛失時は編集/公開停止不可 / 保存方法 / メール送信機能は現在なし / 外部共有禁止」。既存 page と diff 取って統合 |
| **Q-E** | Terms / Privacy 章数 | **design の章数・構造を優先**。法務・実態に必要な条項は削らず追加。design と既存条項が衝突する場合は **既存法務文言を保持しつつ visual を design に合わせる** | design archive: `wf-screens-c.jsx:343` Terms 9 articles / `:396` Privacy 10 articles。既存 article 数を STOP β-2 で確認、超過 / 不足を user 判断事項として escalate（必要に応じ 9/10 を超える章を追加 OK、減らさない）|
| **Q-F** | Privacy External services chips | **design の chip UI を採用**、内容を **production truth に合わせる**。**PostHog / Sentry は本番未採用のため削除**。Cloudflare / Turnstile / R2 + 採用済 Cloud Run / Cloud SQL 等を chip に列挙 | design archive: `wf-screens-c.jsx:404` chips array `['Cloudflare','Turnstile','R2','Sentry','PostHog']`。production 実態: Cloudflare Workers / Cloudflare Turnstile / R2 / Cloud Run / Cloud SQL（必要に応じ追加 service を ADR / `docs/spec` で確認）|
| **Q-G** | Viewer PC sticky right panel | **design 通り sticky right panel を新規 component として追加**。Creator card / Report link / OGP annotation の visual を archive に合わせる。production data で足りない項目は mock せず明記 | design archive: `wf-screens-c.jsx:55-70` PC right panel（Creator card 36×36 avatar + name + @x_id / Report link / OGP annotation）。既存 public viewer の data: creator_display_name / cover variants / pages / photos は API response に存在。X ID / avatar URL は **API response にない可能性** → STOP β-5 で実装前に確認、不足なら「avatar 部分は dashed empty / X ID なし」表示で対応 |

### 10.3 ローンチ blocker としての位置付け

`docs/plan/vrc-photobook-final-roadmap.md` §1.1 で **公開ローンチ blocker = LP final design pass** と整理済。本 PR (m2-design-refresh) はこの blocker の主体実装。完了で §1.1 の P0 を 1 つ解消。

---

## 11. PR 完了処理（pr-closeout 適用）

各 STOP / final 完了報告に以下を含める:

- [ ] **コメント整合チェック**: `bash scripts/check-stale-comments.sh` 実行 + §3 4 区分分類
- [ ] **古い PR 番号コメント整理**: PR25b / PR37 系の古い token 説明 / 構造説明を更新
- [ ] **先送り事項記録**: 本 PR で先送りした項目（例: Phase 2 検索エンジン opt-in）は §11 に記録 + final-roadmap §1.3 反映
- [ ] **generated file 反映**: tailwind.config.ts は generated ではないが、`public/img/landing/` は generated artifact（生成 script を `frontend/scripts/build-landing-images.sh` に保存、commit に同梱）
- [ ] **Secret grep**: 新規 file / commit / log で 0 件
- [ ] **safari-verification.md 適用**: Cookie / redirect / OGP / ヘッダ変更は無いが、iPhone Safari 実機目視は必須（STOP ε）

## 12. 想定タイムライン（参考）

- STOP β-1 (token + base): ~1 day
- STOP β-2 (静的画面 5 × PC+Mobile): ~1〜2 day
- STOP β-3 (Create / Prepare × PC+Mobile): ~1 day
- STOP β-4 (Edit / Complete / Manage × PC+Mobile): ~2 day
- STOP β-5 (Viewer / Report / Errors × PC+Mobile): ~1 day
- STOP β-6 (全画面 QA): ~1 day
- STOP γ 検証 + commit + push: ~0.5 day
- STOP δ Workers deploy + post-deploy smoke: ~0.5 day
- STOP ε 実機 smoke: ~0.5 day

合計 7〜10 day 想定（user 実機 smoke の応答含む）。

---

## 13. user 判断事項（**STOP α 承認で確定**）

1. §3.2 配信戦略 → **A 案（事前圧縮 + public 配信）採用** で OK か?
2. §2.2 token swap → **値だけ swap、class 名・component 名・data-testid は不変** で OK か?
3. §9 branch / commit 戦略 → **単一 branch + STOP ごと commit + STOP δ で一括 deploy** で OK か?
4. §10.2 Q-A〜Q-G の v2 採用方針で OK か?（採用済み解決を上記 §10.2 の表に反映済）
5. user 個人写真 (`design/usephot/`) を `.gitignore` に追加することへの同意

承認頂いたら STOP β-1 着手します。

---

## 14. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-03 | **v1**: 初版作成（STOP α 設計判断資料）|
| 2026-05-03 | **v2**: PC ↔ Mobile を別行に分離、design source file:line link 追加（§4.2）、artboard 数 13 → 16 に修正、numbering 4.1〜4.14 反映、Mobile 寸法 390×844 → **360×740** に修正（§8.2 / §4.1）、§0.1「design はそのまま / 足りないものは足す」正典方針追加、§5.1 design source 正典マップ追加、§10.2 Q-A〜Q-G 採用方針確定、§4.4 production 実装対象外 artboard (flow-primary / draft-exch / manage-exch) 明記 |
