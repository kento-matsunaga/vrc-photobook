// VRC PhotoBook ランディングページ (m2-design-refresh STOP β-2a)。
//
// 採用元 (design 正典):
//   - design/source/project/wf-screens-a.jsx:45-118 `WFLanding_M` (mobile)
//   - design/source/project/wf-screens-a.jsx:119-203 `WFLanding_PC` (PC)
//   - design/source/project/wf-shared.jsx:29-48 `WFBrowser` の header (PC)
//     → production では PublicTopBar に置換 (`frontend/components/Public/PublicTopBar.tsx`)
//   - design/source/project/wireframe-styles.css:351-369 `.wf-h1` / `.wf-eyebrow` / `.wf-sub`
//   - design/source/project/wireframe-styles.css:228-253 `.wf-btn` (primary lg / lg)
//   - design/source/project/wireframe-styles.css:337-349 `.wf-section-title`
//   - design/source/project/wireframe-styles.css:611-618 `.wf-cta-band`
//   - design/source/project/wireframe-styles.css:565-567 `.wf-grid-2` / `.wf-grid-4` / `.wf-grid-3`
//
// design 正典構造:
//   1. PublicTopBar (showPrimaryCta=true)
//   2. Hero: eyebrow + h1 + sub + 2 CTA + MockBook (Mobile=stack / PC=wf-grid-2)
//   3. Sample strip: 4 thumb (Mobile aspect-1/1) / 5 thumb (PC aspect-4/3) → id="examples"
//   4. Feature 4 (ログイン不要 / URLで共有 / 管理URLで編集 / イベント・おはツイ・作品集)
//   5. Use-case 3 (イベント / おはツイ / 作品集) + 右端 image placeholder
//   6. CTA band (✦ さあ、あなたの思い出をカタチにしよう ✦ / ✦ 無料でフォトブックを作る)
//   7. PublicPageFooter (ε-fix: trust strip は非表示。実機 smoke で「無関係な情報量が
//      多く LP の集中導線を弱める」とのフィードバックを反映)
//
// 「design はそのまま、足りないものは足す」(plan §0.1):
//   - design 文言・配色・寸法は崩さない
//   - production 補助として data-testid / aria-label / SVG icon を追加
//   - h1 改行は design `<br/>` 通り
//   - 旧版で出していた `lp-policy` block は design に存在しないため削除
//     (利用規約 / プライバシーは PublicPageFooter の About / Terms / Privacy リンクで案内)
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow / Referrer-Policy が付与される
//   - 実画像は使わず gradient placeholder のみ (β-2c で landing image 差し替え予定)
//   - 作成導線「今すぐ作る」/「無料でフォトブックを作る」は /create に直結
//
// 設計参照:
//   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §STOP β-2a
//   - docs/plan/m2-design-refresh-plan.md §0.1 / §6 STOP β-2
//   - docs/spec/vrc_photobook_business_knowledge_v4.md §3.9 ランディング機能

import type { Metadata } from "next";
import Link from "next/link";

import type { LandingImage } from "@/components/Public/LandingPicture";
import { MockBook, MockThumb } from "@/components/Public/MockBook";
import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";

export const metadata: Metadata = {
  title: "VRC PhotoBook｜VRC写真を、Webフォトブックに。",
  description:
    "VRChat で撮った写真を、ログイン不要・スマホで 1 冊のフォトブックにまとめて X で共有できるサービスです（非公式ファンメイド）。",
};

// design `wf-screens-a.jsx:67-82` (M) / `:154-165` (PC) を統合した 4 feature。
// PC は body が長め (`:155-158`)。production は PC 文言を採用 (情報量優先)。
const features: ReadonlyArray<{ title: string; body: string; iconPath: string }> = [
  {
    title: "ログイン不要",
    body: "アカウント登録なしですぐフォトブックを作成できます。",
    // 「user」 (`wf-shared.jsx:103` 👤): 単体 person silhouette
    iconPath:
      "M16 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2 M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8",
  },
  {
    title: "URLで共有",
    body: "生成されたURLを共有するだけで、みんなが写真を楽しめます。",
    // 「link」 (`wf-shared.jsx:104` 🔗): chain link
    iconPath:
      "M10 13a5 5 0 0 0 7 0l3-3a5 5 0 1 0-7-7l-1 1 M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1",
  },
  {
    title: "管理URLで編集",
    body: "管理用URLがあればいつでも編集・追加・並べ替えが可能です。",
    // 「edit」 (`wf-shared.jsx:105` ✎): pencil
    iconPath:
      "M12 20h9 M16.5 3.5a2.121 2.121 0 1 1 3 3L7 19l-4 1 1-4 12.5-12.5z",
  },
  {
    title: "イベント・おはツイ・作品集",
    body: "イベントの記録やおはツイ・作品集など様々な用途に活用できます。",
    // 「calendar」 (`wf-shared.jsx:106` 📅): calendar
    iconPath:
      "M3 6a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2zM3 10h18 M8 2v4 M16 2v4",
  },
];

// design `wf-screens-a.jsx:86-101` (M) / `:173-186` (PC) の 3 use case。
const useCases: ReadonlyArray<{ title: string; body: string; iconPath: string }> = [
  {
    title: "イベント",
    body: "イベントの記録や、思い出のシーンをまとめます。",
    iconPath:
      "M3 6a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2zM3 10h18 M8 2v4 M16 2v4",
  },
  {
    title: "おはツイ",
    body: "おはツイの記録や、日々の交流をまとめます。",
    // 「chat」 (`wf-shared.jsx:107` 💬): speech bubble
    iconPath:
      "M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z",
  },
  {
    title: "作品集",
    body: "ワールドや写真作品を、美しくまとめます。",
    // 「book」 (`wf-shared.jsx:108` 📖): open book
    iconPath:
      "M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2zM22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z",
  },
];

const heroDate = "2026.04.24";
// ε-fix: モック写真集の表紙文言を実利用イメージに寄せた汎用語へ変更
// (旧 "ミッドナイト ソーシャルクラブ" は VRChat ワールド名前提が強すぎて初見の
//  ユーザーには伝わりにくい、という実機 smoke フィードバックを反映)。
// world label は誤解 (実在ワールド名と勘違いされる) を避けるため非表示にする。
const heroTitle = "おはツイ\nまとめ！";
const thumbVariants: ReadonlyArray<"a" | "c" | "b" | "d" | "e"> = [
  "a",
  "c",
  "b",
  "d",
  "e",
];

// β-2c: design placeholder を `frontend/scripts/build-landing-images.sh` 生成の実画像に差し替える。
// alt 文言は user prompt 確定:
//   - hero: VRChat の写真をフォトブックにまとめたイメージ
//   - mock-cover / sample: VRChat フォトブックの作例写真
// raw filename は build script 内の MAPPING に閉じ込め、ここでは stable name (slug) のみ参照。
const SAMPLE_ALT = "VRChat フォトブックの作例写真";
const HERO_ALT = "VRChat の写真をフォトブックにまとめたイメージ";

// LP MockBook 内 spread 配置 (β-2c Q-2c-1 確定):
//   - 左 cover         : mock-cover (縦長 9:16)
//   - 右 page top span : hero       (横長 16:9 → 2-cell span にフィット)
//   - 右 page bottom L : sample-04  (装飾、alt="" は sample strip と重複表示のため)
//   - 右 page bottom R : sample-01  (装飾、同上)
//
// ε-fix: object-cover 中央クロップで顔位置がずれて見える問題を回避するため、画像
// ごとに objectPosition を持たせる。縦長は顔が上寄りに写っていることが多いので
// "center 30%" を既定とし、横長 (hero / sample-02) は中央寄り or やや上で構図を保つ。
const heroBookCover: LandingImage = {
  slug: "mock-cover",
  alt: SAMPLE_ALT,
  width: 720,
  height: 1280,
  objectPosition: "center 30%",
};
const heroBookSpreadTop: LandingImage = {
  slug: "hero",
  alt: HERO_ALT,
  width: 1600,
  height: 900,
  objectPosition: "center 40%",
};
const heroBookSpreadBottomLeft: LandingImage = {
  slug: "sample-04",
  alt: "",
  width: 640,
  height: 1138,
  objectPosition: "center 32%",
};
const heroBookSpreadBottomRight: LandingImage = {
  slug: "sample-01",
  alt: "",
  width: 640,
  height: 1138,
  objectPosition: "center 32%",
};

// LP sample strip (5 thumb) variant → sample slug マッピング。
// design Mobile pos 0..3 (a/c/b/d) + PC pos 4 (e、Mobile では sm:hidden で非表示)。
// sample-02 のみ landscape 16:9、他は portrait 9:16 (CSS aspect で表示比固定 + object-cover)。
// objectPosition: 縦長は "center 30%" 寄り、横長 sample-02 は "center center" 既定。
const SAMPLE_BY_VARIANT: Record<"a" | "b" | "c" | "d" | "e", LandingImage> = {
  a: { slug: "sample-01", alt: SAMPLE_ALT, width: 640, height: 1138, objectPosition: "center 30%" },
  b: { slug: "sample-03", alt: SAMPLE_ALT, width: 640, height: 1138, objectPosition: "center 30%" },
  c: { slug: "sample-02", alt: SAMPLE_ALT, width: 640, height: 360, objectPosition: "center center" },
  d: { slug: "sample-04", alt: SAMPLE_ALT, width: 640, height: 1138, objectPosition: "center 32%" },
  e: { slug: "sample-05", alt: SAMPLE_ALT, width: 640, height: 1138, objectPosition: "center 30%" },
};

// design `wireframe-styles.css:337-349` `.wf-section-title` (12px/700/0.04em + ::before teal bar)
function SectionTitle({ children, id }: { children: string; id?: string }) {
  return (
    <h2
      id={id}
      className="mb-3 flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong sm:mb-4"
    >
      <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
      {children}
    </h2>
  );
}

// design `wf-shared.jsx:101-121` `WFIcon` (emoji-based 飾り) を SVG 化。design の
// 寸法 `wireframe-styles.css:584-594` `.wf-feat-icon` (44 round, teal-50 bg, teal-100 border, teal-600 ink)。
function FeatIcon({ d }: { d: string }) {
  return (
    <span
      aria-hidden="true"
      className="grid h-11 w-11 shrink-0 place-items-center rounded-full border border-teal-100 bg-teal-50 text-teal-600"
    >
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d={d} />
      </svg>
    </span>
  );
}

export default function HomePage() {
  return (
    <>
      <PublicTopBar />
      <main className="mx-auto w-full max-w-screen-md px-4 py-6 sm:max-w-[1120px] sm:px-9 sm:py-9">
        {/* HERO */}
        <section
          data-testid="lp-hero"
          className="grid gap-5 sm:grid-cols-[1.05fr_1fr] sm:items-center sm:gap-12"
        >
          <div className="space-y-3 sm:space-y-4">
            <SectionEyebrow>VRC PhotoBook</SectionEyebrow>
            {/* design `wf-screens-a.jsx:50` (M) / `:126` (PC) — h1 30/42 */}
            <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">
              VRC写真を、
              <br />
              Webフォトブックに。
            </h1>
            {/* design `wf-screens-a.jsx:51` (M) / `:127-130` (PC) */}
            <p className="text-body leading-[1.7] text-ink-strong sm:text-[15px]">
              ログイン不要で、だれでもかんたんに。
              <br />
              思い出をきれいにまとめて、すぐにシェア。
            </p>
            {/* hero CTA: design `wf-screens-a.jsx:53-56` (M stack) / `:131-134` (PC row) */}
            <div className="flex flex-col gap-2.5 pt-1 sm:flex-row sm:gap-3 sm:pt-3">
              <Link
                href="/create"
                data-testid="lp-hero-cta-create"
                className="inline-flex h-12 w-full items-center justify-center rounded-[10px] bg-brand-teal px-6 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover sm:w-auto sm:min-w-[180px]"
              >
                ✦ 今すぐ作る
              </Link>
              <Link
                href="#examples"
                data-testid="lp-hero-cta-examples"
                className="inline-flex h-12 w-full items-center justify-center rounded-[10px] border border-divider bg-surface px-6 text-sm font-bold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700 sm:w-auto sm:min-w-[180px]"
              >
                🖼 作例を見る
              </Link>
            </div>
          </div>

          <div className="sm:pl-2">
            <MockBook
              title={heroTitle}
              date={heroDate}
              cover={heroBookCover}
              spreadTop={heroBookSpreadTop}
              spreadBottomLeft={heroBookSpreadBottomLeft}
              spreadBottomRight={heroBookSpreadBottomRight}
            />
          </div>
        </section>

        {/* SAMPLE STRIP (`wf-screens-a.jsx:62-64` M / `:141-143` PC) — id="examples" anchor */}
        <section
          id="examples"
          data-testid="lp-thumbs"
          aria-label="サンプルイメージ"
          className="mt-5 grid grid-cols-4 gap-1.5 sm:mt-9 sm:grid-cols-5 sm:gap-3"
        >
          {thumbVariants.map((v, i) =>
            i < 4 ? (
              <MockThumb key={v} variant={v} image={SAMPLE_BY_VARIANT[v]} />
            ) : (
              <span key={v} className="hidden sm:block">
                <MockThumb variant={v} image={SAMPLE_BY_VARIANT[v]} />
              </span>
            ),
          )}
        </section>

        {/* FEATURES (`wf-screens-a.jsx:66-83` M wf-m-card stack / `:146-167` PC wf-box.lg + grid-4) */}
        <section
          data-testid="lp-features"
          aria-labelledby="lp-features-heading"
          className="mt-9 sm:mt-11 sm:rounded-lg sm:border sm:border-divider-soft sm:bg-surface sm:p-7 sm:shadow-sm"
        >
          <SectionTitle id="lp-features-heading">VRC PhotoBookの特徴</SectionTitle>
          <ul className="grid grid-cols-1 gap-3 sm:grid-cols-4 sm:gap-4">
            {features.map((f) => (
              <li
                key={f.title}
                className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm sm:rounded-md sm:border-divider-soft sm:bg-surface-soft sm:p-[18px] sm:shadow-none"
              >
                <div className="flex items-start gap-3 sm:block">
                  <span className="sm:mb-2.5 sm:inline-flex">
                    <FeatIcon d={f.iconPath} />
                  </span>
                  <div className="flex-1">
                    <p className="text-[13px] font-bold leading-tight text-ink sm:text-[13.5px]">
                      {f.title}
                    </p>
                    <p className="mt-1.5 text-xs leading-[1.6] text-ink-medium">
                      {f.body}
                    </p>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        </section>

        {/* USE CASES (`wf-screens-a.jsx:85-102` M wf-m-card / `:170-188` PC wf-box.lg + grid-3) */}
        <section
          data-testid="lp-use-cases"
          aria-labelledby="lp-use-cases-heading"
          className="mt-7 sm:mt-5 sm:rounded-lg sm:border sm:border-divider-soft sm:bg-surface sm:p-7 sm:shadow-sm"
        >
          <SectionTitle id="lp-use-cases-heading">こんなシーンで活用できます</SectionTitle>
          <ul className="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-4">
            {useCases.map((u) => (
              <li
                key={u.title}
                className="flex items-center gap-3 rounded-lg border border-divider-soft bg-surface p-3.5 shadow-sm sm:rounded-md sm:border-divider-soft sm:bg-surface-soft sm:p-[14px] sm:shadow-none"
              >
                <FeatIcon d={u.iconPath} />
                <div className="flex-1">
                  <p className="text-[13px] font-bold leading-tight text-ink">{u.title}</p>
                  <p className="mt-1 text-[11px] leading-[1.5] text-ink-medium sm:text-xs">
                    {u.body}
                  </p>
                </div>
                <span
                  aria-hidden="true"
                  className="block h-12 w-16 shrink-0 rounded-md border border-dashed border-divider-soft bg-gradient-to-br from-teal-50 to-surface-soft sm:h-[60px] sm:w-[90px] sm:rounded-lg"
                />
              </li>
            ))}
          </ul>
        </section>

        {/* CTA BAND (`wf-screens-a.jsx:104-108` M / `:191-197` PC) */}
        <section
          data-testid="lp-cta-block"
          className="mt-9 rounded-lg border border-teal-100 bg-teal-50 px-5 py-6 text-center sm:mt-11 sm:px-7 sm:py-8"
        >
          <p className="text-sm font-bold text-ink sm:text-base">
            <span className="hidden sm:inline">✦ </span>
            さあ、あなたの思い出をカタチにしよう
            <span className="hidden sm:inline"> ✦</span>
          </p>
          <div className="mt-3 sm:mt-4">
            <Link
              href="/create"
              data-testid="lp-cta-block-create"
              className="inline-flex h-12 w-full items-center justify-center rounded-[10px] bg-brand-teal px-6 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover sm:w-auto sm:min-w-[300px] sm:px-8"
            >
              ✦ 無料でフォトブックを作る
            </Link>
          </div>
          <p className="mt-2 text-xs text-ink-medium">ログイン不要で作成できます</p>
        </section>

        <PublicPageFooter />
      </main>
    </>
  );
}
