# m2-design-refresh STOP β-2b: Static content pages 詳細分割計画

> 状態: STOP β-2b **承認済 設計判断資料**（user 確認済、Q-2b1-1〜Q-2b3-3 はすべて確定）。β-2b-1 から実装着手可。実装 / commit / push は sub-step 単位で進める。deploy はしない。
>
> 前提:
> - HEAD == origin/main == `0d8f156`
> - STOP β-1 (`a61163c`) / β-2a (`0d8f156`) 完了
> - deploy 未実施
> - 親計画: `docs/plan/m2-design-refresh-stop-beta-2-plan.md` §2 (β-2b 章)
> - design 正典:
>   - `design/source/project/wf-screens-c.jsx`
>   - `design/source/project/wf-shared.jsx`
>   - `design/source/project/wireframe-styles.css`
>   - `design/source/project/VRC PhotoBook Wireframe.html`
> - 方針:
>   - design はそのまま
>   - 足りない production 要件は補助 UI として足す
>   - 法務 / policy 文言は削らない
>   - production truth は design の placeholder より優先
>   - Backend / deploy / Workers / Scheduler / Job / DB / Secret / env / binding 変更は禁止
>   - `design/usephot/` raw PNG / generated assets は β-2c scope。今回は触らない

---

## 0. 分割理由 + 順序

### 0.1 推奨順: β-2b-1 → β-2b-2 → β-2b-3

| 順 | sub-step | 理由 |
|---|---|---|
| 1 | **β-2b-1 Static page shell + Terms / Privacy foundation** | PolicyArticle / scroll-mt 補正 / PublicTopBar 静的ページ導入の **共通基盤**を最初に確定。Terms / Privacy は構造（TOC + N article + notice + chip）が共通でテンプレ化が利く。後続 About / Help の PublicTopBar 統合 pattern もここで決まる |
| 2 | **β-2b-2 About** | 6 + 4 + ポリシー 3 button という独自レイアウト。PolicyArticle 不使用。β-2b-1 の PublicTopBar pattern を流用 |
| 3 | **β-2b-3 Help / manage-url** | Q1〜Q6 の独自レイアウト。PolicyArticle 不使用。最後にすることで β-2b-1 / β-2b-2 で確定した design token / PublicTopBar / shell 整合をそのまま適用 |

### 0.2 代替検討（採用しない理由）

| 案 | 採否 | 理由 |
|---|---|---|
| Terms と Privacy を別 commit | 不採用 | PolicyArticle の `scroll-mt-6 → scroll-mt-20` 変更が両 page 共通、TOC pattern も共通。1 commit に集約した方が原子性高い |
| About を最初 | 不採用 | PublicTopBar pattern 確立前に About を弄ると後で再修正リスク。Terms / Privacy で pattern を確定してから About に流用が安全 |
| 4 page を 1 commit に集約 | 不採用 | 親計画 (m2-design-refresh-stop-beta-2-plan.md §2.7) の page 別 commit 方針と整合。3 sub-step に揃えてレビュー / rollback 単位を保つ |

---

## 1. β-2b-1: Static page shell + Terms / Privacy foundation

### 1.1 scope

- `PolicyArticle` / `PolicyToc` / `PolicyNotice` 共通 component の comment refresh + `scroll-mt-6 → scroll-mt-20` (PublicTopBar sticky 補正)
- Terms / Privacy 両 page に **PublicTopBar 統合**
- Terms / Privacy 両 page の visual を design 正典に整合 (eyebrow inline 「最終更新」/ wf-note / wf-toc / wf-box card)
- Privacy chips を **production truth 5 service** (Cloudflare / Turnstile / R2 / Cloud Run / Cloud SQL) に整理
  - design 正典に存在する PostHog / Sentry は **採用しない**（未採用サービス、production truth 優先）
  - 既存 production にある Google Secret Manager は **chip から外す**（infra-only、エンドユーザの個人情報処理に直結しない。Q-2b1-1 で確認）
- 法務文言（権利・免責・準拠法・未成年・第三者提供 等）は **削らない**

### 1.2 変更予定ファイル

| File | 変更内容 |
|---|---|
| `frontend/components/Public/PolicyArticle.tsx` | comment を design source line ref に refresh / `scroll-mt-6 → scroll-mt-20` |
| `frontend/components/Public/__tests__/PolicyArticle.test.tsx` | `scroll-mt-20` assert に更新 |
| `frontend/app/(public)/terms/page.tsx` | PublicTopBar 統合 / eyebrow に「Terms · 最終更新 2026-05-01」inline / 各 article を design 視覚 (wf-box 風) に整合 |
| `frontend/app/(public)/privacy/page.tsx` | PublicTopBar 統合 / eyebrow inline / chips 5 件 (Secret Manager 削除) / 各 article を design 視覚に整合 |
| `frontend/app/__tests__/public-pages.test.tsx` | Terms / Privacy section の SSR assertion を update (PublicTopBar / chip 5 / 不存在確認 [PostHog / Sentry / Secret Manager が SSR HTML に出ないこと]) |

### 1.3 design source 対応

| 要素 | design source | 採用方針 |
|---|---|---|
| Terms PC | `wf-screens-c.jsx:358-381` | 視覚 align: TOC `wf-toc` + 9 article の `wf-box` 風 card |
| Terms M | `wf-screens-c.jsx:331-357` | TOC + 9 `wf-m-card` 風 card |
| Privacy PC | `wf-screens-c.jsx:413-442` | TOC + 10 `wf-box` 風 card + chip section（`wf-section-title` + `wf-row` flex-wrap） |
| Privacy M | `wf-screens-c.jsx:384-412` | TOC + 10 `wf-m-card` + chip section |
| `.wf-toc` | `wireframe-styles.css:538-545` | left teal-200 border + 4px padding-left + display:grid gap-2.5 + text-[12.5px] ink-2 |
| `.wf-note` | `wireframe-styles.css:398-425` | warn-soft tone + i icon (teal-500 fill) — 既存 PolicyNotice はこの spirit に近い、border-left 3px teal-300 + bg teal-50 に整える |
| `.wf-box` / `.wf-box.lg` | `wireframe-styles.css:165-175` | radius 12 / lg=16 / padding 14-20 / shadow-sm + border line / paper bg |
| `.wf-section-title` | `wireframe-styles.css:337-349` | text-[12px] font-bold tracking-[0.04em] + ::before 4×14 teal-500 bar |
| `.wf-h1` / `.wf-h1.lg` | `wireframe-styles.css:351-358` | text-h1 / sm:text-h1-lg (β-1 token) |
| `.wf-eyebrow` | `wireframe-styles.css:360-364` | text-xs font-bold uppercase tracking-[0.14em] text-teal-600 (β-2a で SectionEyebrow を整合済) |
| `.wf-badge` (chip) | `wireframe-styles.css:372-395` | 22px 高 + radius-pill + bg-soft + ink-2 + line-2 border。`.wf-badge.teal` 派生は teal-50/100/700 |

### 1.4 content migration 方針

- **Terms 9 article 内容**: 既存 `terms/page.tsx:70-173` 全維持。design は placeholder「Article 1〜9」のみで内容未確定 → production truth (業務知識 v4 §7.1 / §7.3 / §7.4) を維持
- **Privacy 10 article 内容**: 既存 `privacy/page.tsx:83-192` 全維持
- **Privacy chips: 6 → 5 (Secret Manager 削除)**
  - 残す chip text (確定、design の短さ × Cloudflare 内サービスの粒度識別を両立):
    - `Cloudflare Workers`
    - `Turnstile`
    - `R2`
    - `Cloud Run`
    - `Cloud SQL`
  - 削除: Google Secret Manager（infra-only、ユーザーに見せる privacy chip としては過剰）
  - chip text は design の短さは維持しつつ、何のサービスか分かる粒度にする（Cloudflare 1 件では広すぎるため Cloudflare Workers と明示）
- **Notice 文言**: 既存「専門家レビュー前」「最新の本文をご確認ください」維持
- **eyebrow + 最終更新**: design 形式「Terms · 最終更新 2026-05-01」(eyebrow inline) に統合（Q-2b1-3）
- **Mobile bottom links / Footer extra slot**: design は外部 X / `/help/manage-url` link を出すが、PublicTopBar nav に同 link が既にあるため重複削減のため付加しない（Help link は PublicTopBar の「よくある質問」が担当）
- **第三者 Link**:
  - X 連絡用 `@Noa_Fortevita`（Terms 第 8 条）→ 既存 anchor 維持
  - Help link「管理 URL について」（Terms 第 5 条）→ 既存 inline link 維持

### 1.5 法務文言を削らないための確認方法

PR 内 git diff で以下を 2 段階で確認:

1. **diff grep 検査** (削除側):
   ```bash
   git diff -U0 frontend/app/\(public\)/terms/page.tsx \
       frontend/app/\(public\)/privacy/page.tsx \
     | grep -E '^-' \
     | grep -E '権利|免責|責任|個人情報|準拠法|未成年|肖像|著作|削除請求|第三者提供|通報|noindex|管理 URL|セッション|EXIF|位置情報|ハッシュ|ソルト|管轄|改訂|誹謗中傷|プライバシー'
   ```
   ヒット 0 件であること（章番号や見出しの単純 reorder で文言が消えていないこと）

2. **文字数比較**:
   - 既存 terms/page.tsx 文字数 vs 変更後（visual 変更で ±10% 以内）
   - 既存 privacy/page.tsx 文字数 vs 変更後（同上）
   - 大幅減少していたら法務文言が削れている疑い

3. **構造数固定 test** (test 内 assertion):
   - Terms: PolicyArticle 9 個 (id="terms-1" ... "terms-9")
   - Privacy: PolicyArticle 10 個 (id="privacy-1" ... "privacy-10")
   - TOC anchor 9 / 10 個

### 1.6 Privacy chip production truth 確認方法

- production 採用済の確認源:
  - `docs/adr/0001-tech-stack.md` (Cloudflare Workers / Cloud Run / Cloud SQL / R2)
  - `docs/adr/0005-turnstile-action-binding.md` (Turnstile)
  - `frontend/.env.production.example` / `backend/.env.production` (使用 service)
  - `harness/work-logs/2026-04-26_m1-live-deploy-verification.md` (live 確認)
- chip 5 件最終確定:
  | chip text | service 実体 | purpose 表記 |
  |---|---|---|
  | Cloudflare Workers | Cloudflare Workers | フロントエンド配信 |
  | Turnstile | Cloudflare Turnstile | bot 検証 |
  | R2 | Cloudflare R2 | 画像オブジェクトストレージ |
  | Cloud Run | Google Cloud Run | バックエンド API |
  | Cloud SQL | Google Cloud SQL | データベース |
- guard test (`public-pages.test.tsx`、不存在確認):
  ```ts
  expect(html).not.toContain("PostHog");
  expect(html).not.toContain("Sentry");
  expect(html).not.toContain("Google Secret Manager");
  expect(html).not.toContain("Secret Manager");
  ```
- 未採用サービス (Sentry / PostHog) の補足文は **本文に追記しない**:
  - 未採用サービス名を privacy 本文に出すと「使っているのか」と誤読されるリスクがある
  - 不存在は SSR test で assert するのみで十分

### 1.7 visual QA 観点

| 観点 | 期待 |
|---|---|
| Mobile 360×740 | TopBar sticky / eyebrow + h1 + notice / TOC card / 9-10 article card 縦 stack / chip 2 行 wrap / footer。横はみ出し無し |
| PC 1280×820 | `max-w-screen-md` (768px) narrow / TOC `wf-box` 内 / 9-10 article 縦 / chip 1 行 |
| TOC anchor scroll | クリックで article 直前まで scroll、`scroll-mt-20` で sticky topbar (~53px) 分の補正 |
| TopBar sticky と h1 | scroll 位置 0 で h1 が topbar に隠れない（`pt-6` 程度の上余白） |
| 長 JP text | 「権利侵害・削除希望・不適切表現の報告」/ 「ハッシュおよび関連 scope ハッシュ」等 word-break 不要 (line-break 自然) |
| chip overflow | Mobile で 2 行 wrap (5 chip / 2 col grid)、PC で 1 行 (5 chip flex-wrap、はみ出さない) |

### 1.8 commit 方針

**1 commit**:
```
feat(design): restyle terms and privacy with topbar and policy chip cleanup
```

理由: PolicyArticle scroll-mt 変更 / PublicTopBar 統合 / chip cleanup は両 page で同時に整合させた方が原子性高い（片方だけ反映で TOC anchor 高さズレ等の中間状態を避ける）。

---

## 2. β-2b-2: About

### 2.1 scope

- About を design 正典 6 + 4 構造に整理（既存も既に 6+4 数一致だが visual を整合）
- PublicTopBar 統合
- 既存 6 canDo / 4 cannotDo content を維持（production truth、削減なし）
- 「サービスの位置づけ」 dashed dl meta block (運営 / 運営者表示名 / @Noa_Fortevita) は production truth として維持
- 「ポリシーと窓口」 を design 3 button block (`/terms / /privacy / /help/manage-url`) に再整形 (Q-2b2-1)

### 2.2 変更予定ファイル

| File | 変更内容 |
|---|---|
| `frontend/app/(public)/about/page.tsx` | PublicTopBar 統合 / eyebrow + h1 design 整合 / 「サービスの位置づけ」 wf-box 化 (dl meta 維持) / canDo 6 を ✓ 円 icon + border-bottom row に / cannotDo 4 を × 円 icon + border-bottom row に / 「ポリシーと窓口」 を 3 button block (`wf-grid-3` PC / 縦 stack M) |
| `frontend/app/__tests__/public-pages.test.tsx` | About section を update (PublicTopBar presence / 6+4 数固定 / 3 button block 視覚 assert) |

### 2.3 design source 対応

- **About PC**: `wf-screens-c.jsx:228-273`
  - `wf-pc-container narrow` (760px)
  - eyebrow + h1.lg
  - サービスの位置づけ: `wf-box` + section-title + 3 line placeholder（→ production: dl meta 維持）
  - 6+4: `wf-grid-2` で canDo `wf-box` (6 行) と cannotDo `wf-box` (4 行) を並列、各行は ✓ / × + line
  - ポリシーと窓口: `wf-box` + `wf-grid-3` で 3 button
- **About M**: `wf-screens-c.jsx:179-227`
  - `WFMobile` (production: PublicTopBar に置換)
  - eyebrow + h1
  - WFSection 4 つ（位置づけ / できること 6 / MVPでできないこと 4 / ポリシーと窓口 3 button 縦）
- **共通 token**:
  - `.wf-row` border-bottom 区切り (`wireframe-styles.css:573` `.wf-row`)
  - ✓ icon: `width:18, height:18, border:1.5px ink, border-radius:50%, font-weight:700`（design `wf-screens-c.jsx:196`）
  - × icon: 同寸法、文字 ×

### 2.4 content migration 方針

- **canDo 6 件**: 既存 `about/page.tsx:29-66` 全維持。design 6 と数一致。文言は production truth (業務知識 v4 §3.1 / §3.2 / §3.4 / §3.5 / §3.6 / §3.7 / §3.8)
- **cannotDo 4 件**: 既存 `about/page.tsx:68-85` 全維持。design 4 と数一致。文言は production truth (ADR-0006 / Phase 2 / noindex / cmd/ops CLI)
- **「サービスの位置づけ」**: design は placeholder 3 line のみ。既存の dl meta (運営 / 運営者表示名 ERENOA / @Noa_Fortevita) は production truth として **維持**（design に存在しない補助情報だが、運営連絡 / 個人運営の表明として必須）
- **「ポリシーと窓口」**:
  - 既存 (line 209-239): bulleted list with 説明文
  - design: 3 button block (`/terms / /privacy / /help/manage-url`) のみ + 通報は anno で別記
  - **Q-2b2-1**: design 3 button + 説明文を別 p で短縮統合 / 既存 bulleted 維持 のどちら?
  - **推奨**: design 3 button block を採用 + 通報窓口の補足 (1 段落) は別 p tag で button block の下に置く

### 2.5 法務文言を削らないための確認方法

§1.5 と同等の grep 検査:
```bash
git diff -U0 frontend/app/\(public\)/about/page.tsx | grep -E '^-' \
  | grep -E '権利|個人運営|非公式|noindex|MVP|管理 URL|通報|プライバシー'
```

### 2.6 visual QA 観点

| 観点 | 期待 |
|---|---|
| Mobile 360×740 | TopBar / eyebrow / h1 / 位置づけ card (dl meta) / canDo 6 行 / cannotDo 4 行 / policy 3 button 縦 / TrustStrip / footer。横はみ出し無し |
| PC 1280×820 | narrow 760px / 位置づけ wf-box / canDo + cannotDo `wf-grid-2` 並列 / policy `wf-grid-3` 3 button |
| TopBar sticky と h1 | scroll-mt 補正不要（anchor scroll なし）。pt-6 程度で h1 が隠れない |
| 6+4 検証 | canDo 6 / cannotDo 4 数固定 (li.length test) |
| @Noa_Fortevita link | rel="noopener noreferrer" / 外部 X anchor 維持 |
| dl meta wrap | Mobile で 1 col / PC で 3 col 並列 (既存維持) |

### 2.7 commit 方針

**1 commit**:
```
feat(design): restyle about with 6+4 structure and topbar
```

---

## 3. β-2b-3: Help / manage-url

### 3.1 scope

- Help を design 正典 Q1〜Q6 構造に再整理
- PublicTopBar 統合
- 既存 6 セクション content を Q1〜Q6 にマッピング（content 削減なし）
- Mobile back arrow は PublicTopBar 統合により不要（Q-2b3-2）
- h1 を「管理 URL の使い方」に変更検討（Q-2b3-1）

### 3.2 変更予定ファイル

| File | 変更内容 |
|---|---|
| `frontend/app/(public)/help/manage-url/page.tsx` | PublicTopBar 統合 / eyebrow + h1 design 整合 / 既存 6 section を Q1〜Q6 wf-m-card / wf-box 化 / data-testid="help-q1" ... "help-q6" 付与 |
| `frontend/app/__tests__/public-pages.test.tsx` | Help section 新規追加 (PublicTopBar presence / Q1〜Q6 / 法務文言維持) |

### 3.3 design source 対応

- **Help PC**: `wf-screens-c.jsx:302-328`
  - `wf-pc-container narrow` (760px)
  - eyebrow「Help」 + h1.lg「管理 URL の使い方」
  - Q1〜Q6 stack: 各 `wf-box` + `<div className="font-bold">Q{i}. {title}</div>` + body 3 line
- **Help M**: `wf-screens-c.jsx:276-301`
  - `WFMobile title="管理URL FAQ" back` (→ production: PublicTopBar に統合、back 不要)
  - h1「管理 URL の<br/>使い方」
  - Q1〜Q6 stack: 各 `wf-m-card`

### 3.4 content migration 方針

| design Q | 既存 page section | 既存 line | 法務 / production truth |
|---|---|---|---|
| Q1. 公開URLと管理用URLの違い | 「公開 URL と管理用 URL は別物ですか？」 | `help/manage-url/page.tsx:31-39` | 「管理権限は渡りません」 |
| Q2. 管理用URLは再表示不可 | 「管理用 URL は再表示できますか？」 | `:41-49` | 「運営側で再表示や検索はできない仕組み」 |
| Q3. 紛失時は編集/公開停止不可 | 「管理用 URL を紛失したらどうなりますか？」 | `:51-61` | 「紛失すると編集や公開停止ができなくなります」「公開ページ自体は引き続き表示」 |
| Q4. 保存方法 | 「管理用 URL のおすすめの保存方法は？」 | `:63-77` | 「.txt ファイル / 自分宛メール / コピー / パスワードマネージャ」 |
| Q5. メール送信機能は現在なし | 「管理用 URL のメール送信機能はありますか？」 | `:79-88` | 「現在は提供していません」「再選定中」 (ADR-0006) |
| Q6. 外部共有禁止 | 「管理用 URL は外部に共有してよいですか？」 | `:90-99` | 「共有しないでください」 |

content 削減なし、各 Q の本文は既存 production 文言を維持。

### 3.5 法務文言を削らないための確認方法

§1.5 と同等の grep:
```bash
git diff -U0 frontend/app/\(public\)/help/manage-url/page.tsx | grep -E '^-' \
  | grep -E '紛失|再表示|提供していません|再選定|共有|権限|外部|管理 URL|公開 URL'
```

### 3.6 visual QA 観点

| 観点 | 期待 |
|---|---|
| Mobile 360×740 | TopBar / eyebrow / h1 (改行可) / Q1〜Q6 縦 card / footer |
| PC 1280×820 | narrow 760px / Q1〜Q6 縦 card stack |
| TopBar sticky と h1 | pt-6 で隠れず |
| Q anchor (将来 TOC 拡張用) | 各 Q section に `id="help-q{N}"` を付与（β-2b-3 では TOC 出さず、anchor だけ用意） |
| 長 JP text | Q4 (保存方法 4 bullet) / Q3 (公開ページ説明) wrap 自然 |

### 3.7 commit 方針

**1 commit**:
```
feat(design): restyle help/manage-url with Q1-Q6 structure and topbar
```

---

## 4. visual QA 統合 matrix

| 観点 | β-2b-1 (Terms / Privacy) | β-2b-2 (About) | β-2b-3 (Help) |
|---|---|---|---|
| Mobile 360×740 横はみ出し | TOC + 9-10 card | meta dl + 6+4 + 3 button | Q1-Q6 card |
| PC 1280×820 narrow 760 | TOC + N article + chip (Privacy) | wf-grid-2 (canDo / cannotDo) + wf-grid-3 (policy) | Q1-Q6 card stack |
| TopBar sticky + h1 | scroll-mt-20 / pt-6 | pt-6 (anchor 無し) | pt-6 |
| 長 JP text | 法務文言改行 | 6+4 / dl meta long | Q answer 長文 |
| TOC anchor scroll | smooth → article (`scroll-mt-20`) | n/a | n/a (anchor 用意のみ) |
| chip wrap (Privacy) | M 2 行 / PC 1 行 | n/a | n/a |
| ✓ / × icon | n/a | 6 ✓ + 4 × 円 icon | n/a |

---

## 5. test 方針 統合

### 5.1 SSR markup tests (各 page、`frontend/app/__tests__/public-pages.test.tsx`)

| page | 主要 assertion |
|---|---|
| Terms | PublicTopBar (data-testid="public-topbar") / PolicyToc 9 anchor / PolicyArticle 9 / 第 1〜9 条 / 「専門家レビュー」 / public-page-footer |
| Privacy | PublicTopBar / PolicyToc 10 anchor / PolicyArticle 10 / 第 1〜10 条 / chip 5 (Cloudflare / Turnstile / R2 / Cloud Run / Cloud SQL) / 「現在この機能は提供していません」 / public-page-footer |
| About | PublicTopBar / 「サービスの位置づけ」 / canDo 6 / cannotDo 4 / 3 button (`/terms`, `/privacy`, `/help/manage-url`) / 「ERENOA」 / 「@Noa_Fortevita」 / TrustStrip / public-page-footer |
| Help | PublicTopBar / h1 / Q1〜Q6 各 title (data-testid="help-q1" ... ) / 法務文言 (「再表示や検索はできない」「紛失すると」「現在は提供していません」「共有しないでください」) / public-page-footer |

### 5.2 TOC anchor tests

```ts
// Terms
for (let i = 1; i <= 9; i++) {
  expect(html).toContain(`href="#terms-${i}"`);
  expect(html).toContain(`id="terms-${i}"`);
}
// Privacy
for (let i = 1; i <= 10; i++) {
  expect(html).toContain(`href="#privacy-${i}"`);
  expect(html).toContain(`id="privacy-${i}"`);
}
```

### 5.3 old service chip absence tests (β-2b-1)

```ts
// Privacy ページ HTML
expect(html).not.toContain("PostHog");
expect(html).not.toContain("Sentry");
expect(html).not.toContain("Google Secret Manager");
expect(html).not.toContain("Secret Manager");
```

### 5.4 PublicTopBar presence tests (β-2b-1 / -2 / -3 すべて)

```ts
// Terms / Privacy / About / Help それぞれの SSR HTML
expect(html).toContain('data-testid="public-topbar"');
```

### 5.5 no raw secret / token wording checks (β-2a 同等の expectNoSecret 維持)

既存の `FORBIDDEN_PATTERNS` を Terms / Privacy / About / Help すべての page test で適用。

### 5.6 PolicyArticle.test.tsx (scroll-mt 変更検証)

```ts
expect(html).toContain("scroll-mt-20");
expect(html).not.toContain("scroll-mt-6");
```

---

## 6. 検証コマンド (β-2a と同じ 5 段階、各 sub-step 完了時に実施)

```bash
npm --prefix /home/erenoa6621/dev/vrc_photobook/frontend run test
npm --prefix /home/erenoa6621/dev/vrc_photobook/frontend run typecheck
npm --prefix /home/erenoa6621/dev/vrc_photobook/frontend run build
npm --prefix /home/erenoa6621/dev/vrc_photobook/frontend run cf:build
npm --prefix /home/erenoa6621/dev/vrc_photobook/frontend run cf:check
```

5 段階すべて PASS で初めて commit 可。bundle 25MB / single file 5MB 上限維持。

deploy はしない (Backend / Workers / Cloud SQL / Job / Scheduler 一切変更しない)。

---

## 7. commit 方針

**3 commit に分割** (sub-step 単位、別 commit 推奨):

```
1. feat(design): restyle terms and privacy with topbar and policy chip cleanup
2. feat(design): restyle about with 6+4 structure and topbar
3. feat(design): restyle help/manage-url with Q1-Q6 structure and topbar
```

各 commit:
- `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>` を最後の行に付与
- 該当 sub-step の対象 file のみを `git add <files>` で明示 staging
- `.claude/scheduled_tasks.lock` / `ChaeckImage/` / `TESTImage/` / `design/usephot/` raw PNG は staging しない
- 5 段階検証が PASS してから commit
- `git diff --check` clean 確認

各 commit 後に user 承認 → push (`git push origin main`)。3 sub-step すべて完了するまで deploy しない。

---

## 8. open questions（**全項目 user 確定済**）

下表は本計画 commit 時点で **すべて user 確認済**。`推奨` 列は確定方針として β-2b-1 / -2 / -3 実装で採用する。

| ID | 内容 | 影響範囲 | 確定方針 |
|---|---|---|---|
| Q-2b1-1 | Privacy chip から **Google Secret Manager を削除** (確定) | β-2b-1 | **削除確定**: infra-only、ユーザーに見せる privacy chip としては過剰 |
| Q-2b1-2 | Privacy chip text 確定: `Cloudflare Workers / Turnstile / R2 / Cloud Run / Cloud SQL` | β-2b-1 | **確定**: design の短さは維持しつつ、Cloudflare 内 service (Workers / R2 / Turnstile) を区別する粒度。「Cloudflare」単独は広すぎるため Workers と明示 |
| Q-2b1-3 | Terms / Privacy 「最終更新」表記形式: design `「Terms · 最終更新 2026.05.01」` (eyebrow inline、ドット日付) vs 既存 `eyebrow + 「最終更新: 2026-05-01」` (別 p、ハイフン日付) | β-2b-1 | **eyebrow inline + ハイフン日付**: design 形式採用 + 日付は既存ハイフン形式維持（page 内 / git log と整合） |
| Q-2b1-4 | PolicyArticle scroll-mt: 既存 `scroll-mt-6` (24px) → `scroll-mt-20` (80px) に変更で OK か? PublicTopBar sticky 高さ ~53px → 80px で安全マージン | β-2b-1 | **scroll-mt-20 に変更**: PublicTopBar sticky に隠れず anchor scroll が見やすくなる |
| Q-2b1-5 | PublicPageFooter `showTrustStrip`: LP / About = true / Terms / Privacy / Help = false で OK か? | β-2b-1 / -2 / -3 | **OK**: 法務 / FAQ ページに trust badge は不要、LP / About のみ |
| Q-2b1-6 | Privacy 第 5 条末尾に「分析・エラー追跡 (Sentry / PostHog 等) は MVP では採用していません」の補足文言を追記するか? | β-2b-1 | **追記しない (確定)**: 未採用サービス名を privacy 本文に出すと「使っているのか」と誤読されるリスクがある。不存在は SSR test の assert (`expect(html).not.toContain("Sentry"/"PostHog")`) のみで担保 |
| Q-2b2-1 | About 「ポリシーと窓口」: design 3 button block (`/terms / /privacy / /help/manage-url`) + 通報補足を 1 段落で別出し / 既存 bulleted list with 説明 のどちら? | β-2b-2 | **design 3 button + 別段落補足**: design 整合 + 通報窓口情報を失わない |
| Q-2b3-1 | Help h1: design 「管理 URL の使い方」 / 既存 「管理用 URL について」 のどちら? | β-2b-3 | **design「管理 URL の使い方」**: LP「使い方」nav と整合、SectionEyebrow も「Help」に揃える |
| Q-2b3-2 | Help Mobile 「back arrow + 管理URL FAQ title」 を PublicTopBar に置換 (back 削除) で OK か? | β-2b-3 | **OK**: LP / Terms / Privacy / About と一貫した PublicTopBar shell |
| Q-2b3-3 | Help Q1〜Q6 に anchor id (`id="help-q1"` 等) と data-testid を付与する? TOC は β-2b-3 では出さない? | β-2b-3 | **anchor id + data-testid 付与、TOC は出さない**: 後続で TOC 追加可、test に固定 anchor が便利 |
| Q-2b-cross-1 | 3 sub-step を 1 PR にまとめる / 各 sub-step を別 PR にする? 現状 main 直 push 運用 | 全体 | **各 sub-step 別 commit + 順次 push**: rollback 単位を細かく保つ。1 PR にまとめる必要なし (現運用は main 直 push) |

---

## 9. 推奨する最初の着手単位

**β-2b-1: Static page shell + Terms / Privacy foundation**

理由:
- PolicyArticle scroll-mt 変更が β-2b-1 で確定 → β-2b-2 / -3 には scroll-mt の影響なし
- PublicTopBar 静的ページ統合 pattern を Terms / Privacy で確立 → About / Help にそのまま流用
- chip cleanup を最初に確定 → 後続で privacy chip の議論を持ち越さない
- 法務 / privacy 文言の整合性確保が最優先 (production launch 必須要件)

Q-2b1-1〜Q-2b1-6 は本計画 commit 時点で **すべて user 確定済**（§8 表参照）。着手即可。

---

## 10. deploy しないことの確認

| 操作 | β-2b-1 / β-2b-2 / β-2b-3 |
|---|---|
| Backend deploy (Cloud Run) | ❌ |
| Workers deploy (Cloudflare) | ❌ |
| Cloud SQL / Cloud Job / Scheduler 変更 | ❌ |
| Secret / env / binding 変更 | ❌ |
| `design/usephot/` raw PNG 取扱 | ❌ (β-2c scope) |
| commit + push (各 sub-step 完了時、user 承認後) | ✅ |

β-2b-1 / -2 / -3 すべて完了後、改めて β-2c → β-3〜β-6 → γ → δ → ε に進む。

---

## 11. 履歴

| 日付 | 変更 |
|---|---|
| 2026-05-03 | 初版作成。`m2-design-refresh-stop-beta-2-plan.md` §2 (β-2b 章) を 3 sub-step (β-2b-1 / -2 / -3) に詳細化 |
| 2026-05-03 | user 確認反映: typo `antiass → 不存在確認` / Privacy chip text 確定 (`Cloudflare Workers / Turnstile / R2 / Cloud Run / Cloud SQL`) / Q-2b1-6 反転（未採用サービス補足は本文に出さず test の不存在確認のみ） |
