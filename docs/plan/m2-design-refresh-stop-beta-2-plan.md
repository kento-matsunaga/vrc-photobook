# STOP β-2 実装計画（m2-design-refresh / 静的画面 5 系統）

> 作成: 2026-05-03
> 状態: **STOP β-2 設計判断資料**。承認後 β-2a 着手。実装 / commit / push / deploy はしない
> 起点: STOP β-1 完了 (`a61163c`、token + base layout + PublicTopBar 新規) を踏まえ、静的画面 5 系統 (4.1 Landing / 4.11 About / 4.13 Terms / 4.14 Privacy / 4.12 Help) を design 正典に整合
>
> 関連:
> - 親計画: `docs/plan/m2-design-refresh-plan.md` v2
> - design archive 正典: `design/source/project/`
> - β-1 commit: `a61163c feat(design): adopt design tokens + base layout`

---

## 0. 分割理由 + 順序

### 0.1 推奨順: β-2a → β-2b → β-2c

| 順 | sub-step | 理由 |
|---|---|---|
| 1 | **β-2a Landing + public shell** | LP がエントリ、PublicTopBar の実利用を開始して「production-ready な共通 shell」を確立する。MockBook / TrustStrip / PublicPageFooter は LP / About / Footer 全画面で共有される foundation のため、ここで設計確定が後続を効率化 |
| 2 | **β-2b Policy / static content pages** | β-2a で確立した PublicTopBar / PublicPageFooter を About / Terms / Privacy / Help で流用。各 page はほぼ同パターン（header → eyebrow → h1 → notice → TOC → content cards → footer）でテンプレ化が利く。content 整合（Q-C / Q-D / Q-E / Q-F）はここで全部確定 |
| 3 | **β-2c landing image asset pipeline** | β-2a / β-2b の visual 完成を見てから image variant の **実需要**（hero 1 枚 / mock 2 枚 / sample 5 枚 等）を確定する方が手戻り少。design archive は MockBook / sample で gradient placeholder を使っており、real photo は production enhancement 扱いで後段挿入が自然 |

### 0.2 代替検討（採用しない理由）

| 案 | 採否 | 理由 |
|---|---|---|
| β-2c を先 | 不採用 | image variant 仕様が β-2a 設計に依存。手戻りリスク。design archive は placeholder で完成しており、real photo は後段で問題なし |
| β-2a / β-2b 並列 | 不採用 | review / commit が複雑化、共通 component (Footer / TrustStrip) の競合 commit を生む |
| β-2 全部を 1 commit | 不採用 | 5 page × Mobile + PC + image pipeline = 最低 12 file 規模、review 単位として大きすぎる。rollback も困難 |

---

## 1. β-2a: Landing + public shell

### 1.1 scope

LP (`/`) の design 正典適用 + 共通 shell (PublicTopBar 実利用 / MockBook / TrustStrip / PublicPageFooter) の確立。

### 1.2 変更予定ファイル

| 種別 | ファイル | 変更内容 |
|---|---|---|
| 修正 | `frontend/app/page.tsx` | LP 全面 restyle、`PublicTopBar` 実利用開始、design 正典 hero / sample strip / 特徴 / 用途 / CTA band 構造 |
| 修正 | `frontend/components/Public/MockBook.tsx` | design `wf-screens-a.jsx:4-43` の `MockBook` 構造に refactor (left cover + right 2x2 grid, top spans 2) |
| 修正 | `frontend/components/Public/TrustStrip.tsx` | design 正典 4 chip 「**完全無料 / スマホで完成 / 安全・安心 / VRCユーザー向け**」に update |
| 修正 | `frontend/components/Public/__tests__/TrustStrip.test.tsx` | label 同期 (「ログイン不要」「スマホで完結」削除、「スマホで完成」「安全・安心」追加) |
| 修正 | `frontend/components/Public/PublicPageFooter.tsx` | design `wf-shared.jsx:64-84` `WFFooter` 構造 (「© VRC PhotoBook」 + links 4 横並び + Trust strip option) |
| 修正 | `frontend/components/Public/__tests__/PublicPageFooter.test.tsx` | structure 同期 |
| 修正 | `frontend/components/Public/__tests__/MockBook.test.tsx` | structure 同期（spread 構造に変わるため）|
| 修正 | `frontend/app/__tests__/public-pages.test.tsx` | LP rendering assertion 同期 |
| (potentially) | `frontend/components/Public/SectionEyebrow.tsx` | design `wf-section-title` / `.wf-eyebrow` に整合 (必要時のみ) |

### 1.3 design source 対応

| 既存 frontend | design source (file:line) | 内容 |
|---|---|---|
| `app/page.tsx` Mobile | `wf-screens-a.jsx:45-118` `WFLanding_M` | hero 縦 stack + MockBook small + sample 4 列 + 特徴 4 (mobile card) + 用途 3 (with thumb) + CTA band + Trust + Footer |
| `app/page.tsx` PC | `wf-screens-a.jsx:119-203` `WFLanding_PC` | hero `wf-grid-2` 2 列 + sample 5 列 + 特徴 4 列 (`wf-grid-4`) + 用途 3 列 (`wf-grid-3`) + CTA band + Trust + Footer |
| `MockBook.tsx` | `wf-screens-a.jsx:4-43` `MockBook` (small / default 切替) | left cover (`58%` width, gradient bg, title placeholder) + right page (`48%` width, 2x2 grid, top spans 2 / bottom 2 image) |
| `PublicPageFooter.tsx` | `wf-shared.jsx:64-84` `WFFooter` + `wireframe-styles.css:493-501` `.wf-footer` + `:596-608` `.wf-trust` | `<div>© VRC PhotoBook</div>` + `<div class="links">` (About / Help / Terms / Privacy) + 上部に optional Trust strip |
| `TrustStrip.tsx` | `wf-shared.jsx:67-72` (chip array) + `:596-608` `.wf-trust` | 「完全無料」「スマホで完成」「安全・安心」「VRCユーザー向け」horizontal flex with `✓` prefix |

### 1.4 production 補完 UI（plan §0.1「足りないものは足す」）

| 追加 | 理由 | design 影響 |
|---|---|---|
| `data-testid="hero" / "sample-strip" / "feature-grid" / "use-case-grid" / "cta-band"` | test hook | design 影響なし（attribute 追加） |
| `aria-label="ヒーロー" / "作例ストリップ"` 等 | accessibility | design 影響なし |
| LP 内 anchor `#examples` (sample strip section) | PublicTopBar の「作例」link 先 | design 影響なし（既存 sample strip にid 付与） |
| Mobile sticky 下部 CTA は未採用 | design では LP に sticky CTA 無し（CTA band で代用）。design そのまま採用 | — |
| Hero CTA `「✦ 今すぐ作る」` の遷移先 | 既存 production: `/create` に遷移 | design archive は遷移先 anno で `/create` 明記 |
| Hero CTA `「🖼 作例を見る」` の遷移先 | LP 内 anchor `#examples` (β-2a で同 LP 内に sample strip 設置) | design 通り |

### 1.5 visual QA 観点

| 観点 | 期待動作 |
|---|---|
| Mobile 360×740 | hero h1 折返、MockBook small 200px 高、sample 4 列 1 行、特徴 4 縦 card、用途 3 縦 card with 64×48 thumb、CTA band 縦、Trust strip `grid-cols-2` (狭幅) → `sm:grid-cols-4` |
| PC 1280×820 | hero `lg:grid-cols-2` text + MockBook、sample 5 列 (`lg:grid-cols-5`)、特徴 4 列 (`lg:grid-cols-4`)、用途 3 列 (`lg:grid-cols-3`)、CTA band 横長、Trust 4 列 |
| text overflow | h1「VRC写真を、Webフォトブックに。」改行維持、長 hero subtext は `leading-relaxed` |
| horizontal scroll | なし（max-w-screen-xl + px padding） |
| sticky / header 重なり | PublicTopBar `sticky top-0 z-10` + page content `mt-0` でセクション重ならない |
| MockBook left/right 重なり | design 通り left 58% / right 48% で 6% overlap、`box-shadow-lg` で立体感 |

### 1.6 検証コマンド

```bash
npm --prefix frontend run build
( cd frontend && npx tsc --noEmit )
( cd frontend && npx vitest run )
npm --prefix frontend run cf:build
npm --prefix frontend run cf:check
```

### 1.7 test 維持 / 同期 update

- 維持必須: `harness-class-guards.test.tsx` 既存 5 case、`PublicTopBar.test.tsx` 4 case (β-1)
- 同期 update: `TrustStrip.test.tsx` (4 label 変更) / `PublicPageFooter.test.tsx` (links 順序 / structure) / `MockBook.test.tsx` (spread 構造) / `public-pages.test.tsx` (LP rendering)

### 1.8 commit 方針

**単一 commit**: `feat(design): restyle landing page (m2-design-refresh STOP β-2a)`

理由:
- LP 関連 component (page.tsx + MockBook + TrustStrip + PublicPageFooter) は密結合
- test 同期 update も同 commit にまとめると review 単位として整合
- 5〜8 file 程度の変更で抑え、rollback 容易

---

## 2. β-2b: Policy / static content pages

### 2.1 scope

About / Terms / Privacy / Help (4 page) を design 正典構造に整合。各 page の content は **production truth + design 構造**で再構成（Q-C / Q-D / Q-E / Q-F に従う）。

### 2.2 変更予定ファイル + design source

| page | 既存 | design source | 主な変更 |
|---|---|---|---|
| `app/(public)/about/page.tsx` (244 lines、PolicyArticle 不使用) | 独自構造 | `wf-screens-c.jsx:179-227` (M) / `:228-273` (PC) | 「サービスの位置づけ」+ できること **6 件** + できないこと **4 件** + ポリシーと窓口 (3 button: /terms / /privacy / /help/manage-url) 構造に再整理。Mobile 縦 stack、PC `wf-grid-2` 並列 |
| `app/(public)/terms/page.tsx` (179 lines、PolicyArticle 9) | PolicyArticle 9 articles 既存 | `wf-screens-c.jsx:331-357` (M) / `:358-381` (PC) | 既存 9 articles 数を維持（design と一致）、TOC `wf-toc` を header 直下に追加、各 article を `wf-box` (PC) / `wf-m-card` (Mobile) に visual update |
| `app/(public)/privacy/page.tsx` (198 lines、PolicyArticle 10) | PolicyArticle 10 articles 既存 | `wf-screens-c.jsx:384-412` (M) / `:413-442` (PC) | 既存 10 articles 数を維持（design と一致）、TOC + External services chips 追加。**chips 内容は production truth (Q-F)** |
| `app/(public)/help/manage-url/page.tsx` (104 lines、PolicyArticle 不使用) | 独自構造 | `wf-screens-c.jsx:276-301` (M) / `:302-328` (PC) | Q1〜Q6 構造に整理。各 Q を `wf-m-card` (Mobile) / `wf-box` (PC) で表示 |
| `components/Public/PolicyArticle.tsx` | 既存 | `wireframe-styles.css:165-175` `.wf-box` + `:337-349` `.wf-section-title` | section heading + body を design token に整合 |
| 各 `__tests__/*.test.tsx` | 既存 | — | structure / content 同期 update |

### 2.3 各 page の構造詳細

#### 2.3.1 About (4.11)

design 正典 (`wf-screens-c.jsx:179-273`):

```
header
├── eyebrow: 「About」
└── h1: 「VRC PhotoBook について」/ Mobile は <br/> 改行 (画面幅対応)

WFSection 「サービスの位置づけ」
└── card with 3 line placeholder (long / long / medium)

WFSection 「できること (6件)」
└── 6 行 (check icon + label, border-bottom 区切り)

WFSection 「MVPでできないこと (4件)」
└── 4 行 (× icon + label, border-bottom)

WFSection 「ポリシーと窓口」
└── 3 button vertical (Mobile) / 3 button horizontal (PC `wf-grid-3`):
    /terms / /privacy / /help/manage-url
```

**production 補完 (Q-C)**: 既存 about page は PolicyArticle を使わず独自構造（244 lines）。design 6 + 4 構造に再整理しつつ、既存の **法的・規約上必要な情報があれば card 構造を壊さず保持**する。具体的な 6 + 4 項目の content 確定は §4 open question。

#### 2.3.2 Terms (4.13)

design 正典 (`wf-screens-c.jsx:331-381`):

```
header
├── eyebrow: 「Terms · 最終更新 2026.05.01」
└── h1: 「利用規約」

Notice (wf-note)

WFSection 「目次 (TOC)」
└── wf-toc (left teal border + 9 anchor links: Article 1〜9)

各 Article (9 件)
└── wf-m-card / wf-box: 
    ├── h3: 「Article {i}」
    └── body lines

Mobile bottom links: /help/manage-url, External: X
PC Footer extra slot: /help/manage-url + X (external)
```

**production 補完 (Q-E)**: 既存 9 articles 構造維持、章数は design と一致。各 article の内容は既存 production 文言を保持。**TOC + design visual** を追加するのみ。

#### 2.3.3 Privacy (4.14)

design 正典 (`wf-screens-c.jsx:384-442`):

```
header
├── eyebrow: 「Privacy · 最終更新 2026.05.01」
└── h1: 「プライバシーポリシー」

Notice (wf-note)

WFSection 「目次 (TOC)」
└── wf-toc (10 anchor links: Article 1〜10)

各 Article (10 件)
└── wf-m-card / wf-box (各 2 line)

WFSection 「External services chips」
└── Mobile: wf-row gap-1.5 wrap
    PC: wf-box with wf-section-title + chips
    chips: production truth に合わせる

Mobile/PC: /terms link
```

**production 補完 (Q-F)**:

design archive chips: `['Cloudflare','Turnstile','R2','Sentry','PostHog']`

**production 採用済 service** (確認要、現状把握):
| service | 用途 | 採用済? |
|---|---|---|
| Cloudflare Workers | Frontend hosting | ✅ |
| Cloudflare Turnstile | bot 検証 | ✅ |
| Cloudflare R2 | 画像 storage / OGP | ✅ |
| Google Cloud Run | Backend service / Jobs | ✅ |
| Google Cloud SQL (PostgreSQL) | DB | ✅（現状 vrcpb-api-verify、本番化は PR39 後続） |
| Google Cloud Build | CI / image build | ✅ |
| Google Artifact Registry | image storage | ✅ |
| Cloud Scheduler | Job tick | ✅ |
| **PostHog** | analytics | ❌ **未採用、削除** |
| **Sentry** | error tracking | ❌ **未採用、削除** |

**chip 採用候補**: Cloudflare / Turnstile / R2 / Cloud Run / Cloud SQL / Cloud Build / Artifact Registry / Cloud Scheduler

→ user 判断: 全部出すか、ユーザに見せるべき主要 service のみ (Cloudflare / Turnstile / R2 / Cloud Run / Cloud SQL の 5 個程度) に絞るか。**§4 open question**。

#### 2.3.4 Help / manage-url (4.12)

design 正典 (`wf-screens-c.jsx:276-328`):

```
header
├── (Mobile) topbar 「管理URL FAQ」
├── eyebrow: 「Help」 (PC のみ)
└── h1: 「管理 URL の使い方」(Mobile <br/> 改行)

Q1〜Q6 (各 wf-m-card / wf-box, narrow PC):
├── Q1. 公開URLと管理用URLの違い
├── Q2. 管理用URLは再表示不可
├── Q3. 紛失時は編集/公開停止不可
├── Q4. 保存方法
├── Q5. メール送信機能は現在なし
└── Q6. 外部共有禁止

各 Q card:
├── h3: 「Q{i}. {タイトル}」
└── body 3 lines (long / long / medium placeholder)

Footer
```

**production 補完 (Q-D)**: 既存 help/manage-url page (104 lines) は独自構造。design Q1〜Q6 構造で再構成、既存 page 内の content (answer 本文) は **削らず** Q1〜Q6 のいずれかに割り当てる。

### 2.4 visual QA 観点

| 観点 | 期待 |
|---|---|
| 各 page Mobile 360×740 | header / eyebrow / h1 / notice / TOC / cards 縦 stack / footer |
| 各 page PC 1280×820 | `wf-pc-container.narrow` (max-w-screen-md 程度 / 760px) で読みやすい line-length |
| TOC anchor 動作 | クリックで該当 article まで scroll、`scroll-mt-20` 等で sticky header 分の補正 |
| chip overflow (Privacy) | Mobile で chips が wrap して横はみ出さない |
| PC sticky header 重なり | PublicTopBar sticky 下に notice/TOC が隠れない |

### 2.5 検証コマンド

β-2a と同じ 5 段階検証。

### 2.6 test 維持 / 同期 update

- 維持必須: 既存 `harness-class-guards.test.tsx` / `PublicTopBar.test.tsx` (β-1) / `TrustStrip.test.tsx` (β-2a で同期済) / `PublicPageFooter.test.tsx` (β-2a で同期済)
- 同期 update: `PolicyArticle.test.tsx` / `public-pages.test.tsx`
- 必要に応じ追加: `about.test.tsx` (新規、6+4 structure assertion)、`help.test.tsx` (新規、Q1〜Q6 assertion)

### 2.7 commit 方針

**page 別に 4 commit に分割**:

```
feat(design): restyle about page with 6+4 structure (m2-design-refresh STOP β-2b)
feat(design): restyle terms page with TOC and 9 articles (m2-design-refresh STOP β-2b)
feat(design): restyle privacy page with TOC, 10 articles, production-truth chips (m2-design-refresh STOP β-2b)
feat(design): restyle help/manage-url with Q1-Q6 structure (m2-design-refresh STOP β-2b)
```

理由: 各 page は独立して review / rollback 可能、commit graph で工程が明確。`PolicyArticle.tsx` 共通変更があれば最初の (terms commit) に同梱して以降 page から流用。

---

## 3. β-2c: landing image asset pipeline

### 3.1 scope

`design/usephot/` の VRChat 実写 PNG 7 枚を公式採用するための **圧縮 pipeline + generated assets**。`design/usephot/` raw PNG は git 除外維持、生成済 webp/jpeg のみ git 管理。

### 3.2 変更予定ファイル

| 種別 | ファイル | 内容 |
|---|---|---|
| 新規 | `frontend/scripts/build-landing-images.sh` | bash script、`design/usephot/*.png` を読み、4 variant × 2 format で `frontend/public/img/landing/` に出力 |
| 新規 | `frontend/public/img/landing/manifest.json` | 生成された variant の listing (filename / dimension / format)、`MockBook` / LP component が動的参照する index |
| 新規 | `frontend/public/img/landing/*.webp` (約 28 file) | 7 photos × 4 variant (hero/mock/card/thumb) WebP q80 |
| 新規 | `frontend/public/img/landing/*.jpg` (約 28 file) | 同 JPEG q85 (WebP fallback) |
| 修正 | `frontend/components/Public/MockBook.tsx` | gradient placeholder の代わりに real image を `<picture><source type="image/webp"><img srcset>` で配信 |
| 修正 | `frontend/app/page.tsx` | sample strip 5 photos を gradient placeholder → real image |
| 新規 | `frontend/__tests__/landing-images-exclusion.test.ts` | raw PNG が git に commit されていないこと、`design/usephot/` に対応する webp/jpeg が `public/img/landing/` に揃っていること、bundle size が制限内であることを scan |

### 3.3 圧縮仕様（plan §3.3 から再掲、変更なし）

| variant | 解像度 (横長) | 解像度 (縦長) | format | 期待サイズ |
|---|---|---|---|---|
| `hero` | 1920×1080 | 1080×1920 | WebP q80 + JPEG q85 | 各 ~150 KB |
| `mock` | 800×1200 | 800×1200 | 同 | 各 ~80 KB |
| `card` | 600×800 | 600×800 | 同 | 各 ~60 KB |
| `thumb` | 240×320 | 240×320 | 同 | 各 ~20 KB |

合計: 7 × 4 × 2 = 56 file、約 4 MB。

### 3.4 build-landing-images.sh（pseudo-code）

```bash
#!/usr/bin/env bash
# 2026-05-XX m2-design-refresh STOP β-2c: landing image asset 圧縮 pipeline
# 入力: design/usephot/*.png (raw、git 除外)
# 出力: frontend/public/img/landing/*.{webp,jpg} + manifest.json
set -euo pipefail
SRC=design/usephot
OUT=frontend/public/img/landing
mkdir -p "$OUT"
declare -A SIZES=( [hero]=1920x1080 [mock]=800x1200 [card]=600x800 [thumb]=240x320 )

for png in "$SRC"/*.png; do
  base=$(basename "$png" .png | tr -d ' ')
  # detect orientation (横長/縦長)
  for variant in hero mock card thumb; do
    long_edge=${SIZES[$variant]%x*}
    short_edge=${SIZES[$variant]#*x}
    # cwebp + cjpeg with appropriate -resize
    cwebp -q 80 -resize "$long_edge" 0 "$png" -o "$OUT/${base}-${variant}.webp"
    magick "$png" -resize "${long_edge}x" -quality 85 "$OUT/${base}-${variant}.jpg"
  done
done

# manifest.json 生成
node frontend/scripts/build-landing-manifest.js > "$OUT/manifest.json"
```

### 3.5 ツール依存

- `cwebp` (libwebp) / `cjpeg` (mozjpeg) または `magick` (ImageMagick)
- インストール: `apt install webp libjpeg-turbo-progs imagemagick` (WSL Ubuntu の場合 sudo が要る → user 対話シェル)
- CI / GitHub Actions では `apt install` を `setup-` step で対応

### 3.6 bundle size impact

| 段階 | Total Upload | gzip |
|---|---|---|
| 現状 (a61163c) | 4876.12 KiB | 1001.74 KiB |
| β-2c 後（目標） | ≤ 9000 KiB | ≤ 1100 KiB |
| Workers asset 上限 | 25 MB（合計）| — |
| Workers asset 単 file 上限 | 5 MB | — |

→ 各 webp/jpg を < 200 KB に抑える（hero でも 200 KB 目標、cwebp -q 80 で達成見込み）。

### 3.7 visual QA 観点

| 観点 | 期待 |
|---|---|
| LP MockBook real photo | left cover に縦長 photo（1 枚）、right page に 縦長 photo 3 枚 (top spans 2 + 2 thumbnail) |
| LP sample strip real photo | 横長 / 縦長 mix で 5 photo |
| 用途 card thumb | 64×48 (Mobile) / 90×60 (PC) で `card` variant 使用 |
| Mobile 360 wide での photo 解像度 | `thumb` 変種も縦 320 px で sufficient、`object-cover` でクロップ |
| webp 非対応 browser | `<picture><source type="image/webp"><img src="...jpg">` で fallback |
| Lazy load | 下部 sample strip / 用途 thumbs は `loading="lazy"`、hero は eager |

### 3.8 raw PNG exclusion check

```typescript
// frontend/__tests__/landing-images-exclusion.test.ts
import { execSync } from "node:child_process";
import { existsSync, readdirSync } from "node:fs";

describe("landing images asset pipeline", () => {
  it("正常_design/usephot raw PNG が git tracked でない", () => {
    const tracked = execSync("git ls-files design/usephot/", { encoding: "utf-8" }).trim();
    expect(tracked).toBe("");
  });

  it("正常_design/usephot の各 PNG に対して 4 variant × 2 format が存在", () => {
    const SRC = "design/usephot";
    if (!existsSync(SRC)) return; // CI で raw PNG 不在の場合は skip
    const pngs = readdirSync(SRC).filter((f) => f.endsWith(".png"));
    for (const png of pngs) {
      const base = png.replace(/\.png$/, "").replace(/\s/g, "");
      for (const variant of ["hero", "mock", "card", "thumb"] as const) {
        for (const ext of ["webp", "jpg"] as const) {
          expect(existsSync(`frontend/public/img/landing/${base}-${variant}.${ext}`)).toBe(true);
        }
      }
    }
  });

  it("正常_各 generated image が 1 MB 以下 (Workers 5MB 単 file 制限を遠く下回る)", () => {
    const dir = "frontend/public/img/landing";
    if (!existsSync(dir)) return;
    const out = execSync(`find ${dir} -name '*.webp' -o -name '*.jpg' -size +1M`, {
      encoding: "utf-8",
    }).trim();
    expect(out).toBe(""); // empty = 全 file が 1 MB 以下
  });
});
```

### 3.9 検証コマンド

```bash
# 1. 圧縮実行
bash frontend/scripts/build-landing-images.sh

# 2. 生成 file 数確認
find frontend/public/img/landing -name '*.webp' | wc -l   # 期待: 28
find frontend/public/img/landing -name '*.jpg' | wc -l    # 期待: 28

# 3. bundle size 計測
du -sh frontend/public/img/landing/

# 4. 既存 5 段階検証
npm --prefix frontend run build
npx tsc --noEmit
npx vitest run
npm --prefix frontend run cf:build
npm --prefix frontend run cf:check  # Total Upload < 9 MB 確認
```

### 3.10 commit 方針

**1 commit にまとめる**: `chore(design): generate landing image variants from design/usephot (m2-design-refresh STOP β-2c)`

理由:
- script + generated images + LP/MockBook の image 切替は密結合
- 単独で意味を持つ（前後 commit と independence）
- generated artifacts は file 数多いが、内容は機械生成なので review は script + sample 1〜2 枚のみで十分

---

## 4. open questions（実装前の user 判断要）

| Q | 内容 | β-2 sub-step | デフォルト推奨 |
|---|---|---|---|
| Q-2a-1 | LP の Hero CTA「✦ 今すぐ作る」は `/create` 直行で OK か？ | β-2a | OK（design archive と整合） |
| Q-2a-2 | LP「作例を見る」CTA は LP 内 `#examples` anchor で OK か？それとも別 page (`/examples`) を作るか？ | β-2a | LP 内 anchor（design archive 通り、別 page 作成は MVP 範囲外） |
| Q-2a-3 | TrustStrip 4 chip の **正典** は design archive `「完全無料 / スマホで完成 / 安全・安心 / VRCユーザー向け」`。既存 production の `「ログイン不要 / スマホで完結」` を design 正典に置換して OK か？（test 同期 update 含む） | β-2a | design 正典に置換 (Q-A 系の design 優先方針) |
| Q-2a-4 | Footer link 順は design archive `「About / Help / Terms / Privacy」`。既存 `「トップ / About / 利用規約 / プライバシー / 管理 URL」` を置換して OK か？ | β-2a | design 正典に置換、ただし **「トップ」link は production 必要なら追加** (補助 UI) |
| Q-2b-1 | About 「できること 6 件 / MVPでできないこと 4 件」 の **具体 content** は? | β-2b | 既存 about page (244 lines) の内容を 6 + 4 に再分配。不足あれば user 提示 |
| Q-2b-2 | Help (manage-url) 「Q1〜Q6」 の **answer 本文** は? | β-2b | 既存 help page (104 lines) の内容を Q1〜Q6 に再分配。不足あれば user 提示 |
| Q-2b-3 | Terms 「最終更新」表記は design `「2026.05.01」`。既存 page の最終更新日 (現在の事実) で OK か？ | β-2b | 既存日付を維持（design 文字列は placeholder） |
| Q-2b-4 | Privacy External services chips は採用済 5 service (Cloudflare / Turnstile / R2 / Cloud Run / Cloud SQL) で OK か？ それとも採用済 8 service 全部 (+ Cloud Build / Artifact Registry / Cloud Scheduler) を出すか？ | β-2b | **5 service**（user に見せる主要 service のみ、infra detail は出さない方針） |
| Q-2b-5 | 各 page の `data-testid` 命名規則は? | β-2b | `page-{name}` (例: `page-about`, `page-terms`) + section ごとに `section-{name}` |
| Q-2c-1 | 7 photos のうち、**hero にどれを使う**か? sample strip / MockBook の photo 配分は? | β-2c | user 提示。仮 default: 3840×2160 横長 1 枚を hero、2160×3840 縦長 4 枚を MockBook + sample に配分 |
| Q-2c-2 | webp/jpeg 圧縮 tool は WSL ローカルに `cwebp` / `cjpeg` / `magick` が入っているか? 入っていなければ install (sudo) が要 | β-2c | user 確認後 install。代替: WSL に既存の `convert` (ImageMagick) があれば使える |
| Q-2c-3 | `design/usephot/` raw PNG が repo 外（local-only）の場合、CI / GitHub Actions では何をするか? | β-2c | CI では skip + `existsSync(SRC)` ガード（`landing-images-exclusion.test.ts` で対応済）。手動圧縮を local で行い、生成済 webp/jpg を commit する flow |

---

## 5. 推奨する最初の着手単位

**β-2a 全体を 1 unit として実装**。具体的には以下を 1 PR / 1 commit にまとめる:

| 作業 | ファイル | 順 |
|---|---|---|
| MockBook restyle | `components/Public/MockBook.tsx` + test | 1 |
| TrustStrip update | `components/Public/TrustStrip.tsx` + test | 2 |
| PublicPageFooter restyle | `components/Public/PublicPageFooter.tsx` + test | 3 |
| LP page restyle + PublicTopBar 統合 | `app/page.tsx` + `__tests__/public-pages.test.tsx` | 4 |
| 5 段階検証 | build / typecheck / vitest / cf:build / cf:check | 5 |
| commit + push | (deploy しない) | 6 |

理由:
- 全体で 5〜8 file の変更
- 各 component が密結合（page.tsx で MockBook + TrustStrip + PublicPageFooter を全部 import / 利用）
- test の同期 update も 1 commit でまとめると review 単位として整合
- rollback すれば LP 全体が β-1 状態に戻る、影響範囲明確

着手前に user 判断を仰ぐ open question:
1. **Q-2a-3** TrustStrip label 置換 (「ログイン不要 / スマホで完結」→「スマホで完成 / 安全・安心」) で OK か?
2. **Q-2a-4** Footer link 順 design 正典化 (「トップ」link 削除可否) で OK か?

これらが OK なら β-2a 着手します。

---

## 6. deploy しないことの確認

| 操作 | β-2a / β-2b / β-2c |
|---|---|
| frontend code 変更 | ✅ 実施（commit + push） |
| Backend code 変更 | ❌ しない |
| Cloud Build 起動 | ❌ しない |
| Cloud Run service deploy | ❌ しない |
| Cloud Run Jobs image tag 更新 | ❌ しない |
| Cloud Scheduler 変更 | ❌ しない |
| `wrangler deploy` (本番 Workers) | ❌ しない |
| Workers Secrets / env / binding 変更 | ❌ しない |
| DB migration / Job execute | ❌ しない |
| `design/usephot/` raw PNG commit | ❌ `.gitignore` で除外維持 |

deploy は **STOP δ で β-2 〜 β-6 完了後に一括実施**する方針（plan v2 §6 STOP δ）。

---

## 7. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-03 | 初版作成。STOP β-1 (a61163c) 完了を踏まえ、静的画面 5 系統を β-2a / β-2b / β-2c に分割 |
