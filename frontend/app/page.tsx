// VRC PhotoBook ランディングページ（design rebuild）。
//
// 採用元:
//   - design/mockups/prototype/screens-a.jsx LP 関数（mobile）
//   - design/mockups/prototype/pc-screens-a.jsx PCLP 関数（PC）
//   - design/mockups/prototype/styles.css の `.hero-title` / `.mock-book` / `.feature-cell` /
//     `.cta-block` / `.trust-strip` / `.photo.v-*`
//   - design/mockups/prototype/pc-styles.css の `.pc-hero` / `.pc-features-grid` / `.pc-cta`
//
// 設計参照:
//   - harness/work-logs/2026-05-01_pr37-design-rebuild-plan.md §3.1 / §6（plan メモが正典）
//   - docs/spec/vrc_photobook_business_knowledge_v4.md §3.9 ランディング機能
//   - failure-log: harness/failure-log/2026-05-01_pr37-public-pages-design-mismatch.md
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow / Referrer-Policy が付与される
//   - 作成導線（draft 新規作成）は MVP 範囲外のため CTA は /about + /help/manage-url
//   - mobile h1 26px / PC h1 40px（plan メモ §5 で承認、prototype forward port）
//   - gradient placeholder は MockBook / MockThumb 内の局所用途のみ

import type { Metadata } from "next";
import Link from "next/link";

import { MockBook, MockThumb } from "@/components/Public/MockBook";
import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";

export const metadata: Metadata = {
  title: "VRC PhotoBook｜VRChat 写真をログイン不要で 1 冊に",
  description:
    "VRChat で撮った写真を、ログイン不要・スマホで 1 冊のフォトブックにまとめて X で共有できるサービスです（非公式ファンメイド）。",
};

const features: ReadonlyArray<{ title: string; body: string; iconPath: string }> = [
  {
    title: "ログイン不要",
    body: "アカウント登録なしで作成・公開・編集まで完結。共有 PC でも安心して使える設計です。",
    iconPath:
      "M17 20v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2 M9 7a4 4 0 1 0 0 0", // Users
  },
  {
    title: "管理 URL で編集",
    body: "公開直後に表示される管理用 URL を保存しておけば、いつでも編集・公開停止・削除ができます。",
    iconPath:
      "M12 20h9 M16.5 3.5a2.1 2.1 0 1 1 3 3L7 19l-4 1 1-4 12.5-12.5z", // Pencil
  },
  {
    title: "公開・限定公開・非公開",
    body: "目的に応じて公開範囲を選べます。MVP の既定は限定公開（URL を知っている人のみ）です。",
    iconPath:
      "M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6", // Eye
  },
  {
    title: "通報窓口を用意",
    body: "公開フォトブックには通報リンクがあり、権利侵害・センシティブ・未成年関連の懸念を運営に届けられます。",
    iconPath:
      "M2 12a10 10 0 1 0 20 0 10 10 0 0 0-20 0 M12 8v4 M12 16h.01", // Info
  },
];

const ctaLinks: ReadonlyArray<{ href: string; label: string; description: string }> = [
  {
    href: "/about",
    label: "VRC PhotoBook について",
    description: "サービスの背景・できること・できないこと。",
  },
  {
    href: "/help/manage-url",
    label: "管理 URL の使い方",
    description: "管理用 URL の保存方法と紛失時の取り扱い。",
  },
];

const heroDate = "2026.04.24";
const heroWorld = "Midnight Social Club";
const heroTitle = "ミッドナイト\nソーシャルクラブ";
const thumbVariants: ReadonlyArray<"a" | "c" | "b" | "d" | "e"> = ["a", "c", "b", "d", "e"];

export default function HomePage() {
  return (
    <main className="mx-auto min-h-screen w-full max-w-screen-md bg-surface px-4 py-8 sm:px-6 sm:py-10">
      {/* HERO */}
      <section data-testid="lp-hero" className="grid gap-6 sm:grid-cols-[1.05fr_1fr] sm:items-center sm:gap-10">
        <div className="space-y-3">
          <SectionEyebrow>VRC PhotoBook</SectionEyebrow>
          <h1 className="text-[26px] font-extrabold leading-tight tracking-tight text-ink sm:text-[40px] sm:leading-[1.15]">
            VRChat 写真を、
            <br className="sm:hidden" />
            ログイン不要で
            <br className="hidden sm:block" />1 冊に。
          </h1>
          <p className="text-body text-ink-strong">
            スマホで撮った VRChat の思い出をフォトブックにまとめて、X で共有できるサービスです。
            アカウント登録は要りません。管理用 URL で、いつでも編集・公開停止ができます。
          </p>
          <p className="text-xs text-ink-soft">
            個人運営の非公式ファンメイドサービス。VRChat 公式とは関係ありません。
          </p>
          <div className="flex flex-col gap-2 pt-2 sm:flex-row sm:flex-wrap">
            {/* Primary CTA: 作成導線（作成導線追加 PR で /create に直結） */}
            <Link
              href="/create"
              data-testid="lp-hero-cta-create"
              className="inline-flex h-12 items-center justify-center rounded bg-brand-teal px-5 text-sm font-bold text-white hover:bg-brand-teal-hover"
            >
              今すぐ作る
            </Link>
            {/* Secondary CTAs: サービス説明 / 管理 URL ヘルプ */}
            {ctaLinks.map((c) => (
              <Link
                key={c.href}
                href={c.href}
                data-testid={`lp-hero-cta${c.href.replace(/\//g, "-")}`}
                className="inline-flex h-12 items-center justify-center rounded border border-brand-teal bg-brand-teal-soft px-5 text-sm font-bold text-brand-teal hover:bg-surface-soft"
              >
                {c.label}
              </Link>
            ))}
          </div>
        </div>

        <div className="sm:pl-2">
          <MockBook title={heroTitle} date={heroDate} worldLabel={heroWorld} />
          <p className="mt-4 flex items-center justify-center gap-2 text-center text-xs text-brand-teal sm:hidden">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M12 2l1.8 5.4L19 9.2l-5.2 1.8L12 16l-1.8-5L5 9.2l5.2-1.8z" />
            </svg>
            美しいレイアウトで、思い出を残そう
          </p>
        </div>
      </section>

      {/* THUMB STRIP */}
      <section
        data-testid="lp-thumbs"
        aria-label="サンプルイメージ"
        className="mt-6 grid grid-cols-4 gap-2 sm:mt-8 sm:grid-cols-5 sm:gap-3"
      >
        {thumbVariants.map((v, i) =>
          // mobile は最初の 4 つ、PC は 5 つすべて
          i < 4 ? (
            <MockThumb key={v} variant={v} />
          ) : (
            <span key={v} className="hidden sm:block">
              <MockThumb variant={v} />
            </span>
          ),
        )}
      </section>

      {/* FEATURES */}
      <section
        data-testid="lp-features"
        aria-labelledby="lp-features-heading"
        className="mt-10 rounded-lg border border-divider bg-surface p-5 shadow-sm sm:p-6"
      >
        <h2 id="lp-features-heading" className="text-h2 text-ink">
          VRC PhotoBook の特徴
        </h2>
        <ul className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {features.map((f) => (
            <li
              key={f.title}
              className="rounded border border-divider bg-surface p-4"
            >
              <span
                aria-hidden="true"
                className="grid h-9 w-9 place-items-center rounded-full bg-brand-teal-soft text-brand-teal"
              >
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d={f.iconPath} />
                </svg>
              </span>
              <p className="mt-3 text-sm font-bold text-ink">{f.title}</p>
              <p className="mt-1 text-sm text-ink-medium">{f.body}</p>
            </li>
          ))}
        </ul>
      </section>

      {/* POLICY */}
      <section
        data-testid="lp-policy"
        aria-labelledby="lp-policy-heading"
        className="mt-10"
      >
        <h2 id="lp-policy-heading" className="text-h2 text-ink">
          ポリシーと窓口
        </h2>
        <ul className="mt-3 list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            <Link href="/terms" className="text-brand-teal underline hover:text-brand-teal-hover">
              利用規約
            </Link>
            （投稿される画像の権利、運営による一時非表示・削除、免責など）
          </li>
          <li>
            <Link href="/privacy" className="text-brand-teal underline hover:text-brand-teal-hover">
              プライバシーポリシー
            </Link>
            （取得する情報、利用目的、保持期間、外部サービス利用）
          </li>
          <li>
            権利侵害・削除希望・不適切表現の報告は、対象フォトブックの「このフォトブックを通報」から運営にお送りください。
          </li>
        </ul>
        <p className="mt-3 text-xs text-ink-soft">
          MVP 段階のため、ページの内容や仕様は予告なく変更されることがあります。
        </p>
      </section>

      {/* CTA BLOCK */}
      <section
        data-testid="lp-cta-block"
        className="mt-10 rounded-lg border border-brand-teal bg-brand-teal-soft p-5 text-center sm:p-6"
      >
        <p className="flex items-center justify-center gap-3 text-xs font-medium text-ink-strong before:h-px before:w-4 before:bg-ink-soft after:h-px after:w-4 after:bg-ink-soft">
          さあ、思い出をフォトブックにまとめよう
        </p>
        <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:justify-center">
          <Link
            href="/create"
            data-testid="lp-cta-block-create"
            className="inline-flex h-12 items-center justify-center rounded bg-brand-teal px-6 text-sm font-bold text-white hover:bg-brand-teal-hover"
          >
            今すぐ作る
          </Link>
          <Link
            href="/about"
            data-testid="lp-cta-block-about"
            className="inline-flex h-12 items-center justify-center rounded border border-brand-teal bg-surface px-6 text-sm font-bold text-brand-teal hover:bg-surface-soft"
          >
            VRC PhotoBook について
          </Link>
        </div>
        <p className="mt-3 text-xs text-ink-medium">ログイン不要・スマホで完結</p>
      </section>

      <PublicPageFooter showTrustStrip />
    </main>
  );
}
