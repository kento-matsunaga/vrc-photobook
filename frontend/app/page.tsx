// VRC PhotoBook ランディングページ。
//
// 業務知識 v4 §3.9「ランディング機能（サービス入口）」準拠。
// 「ログイン不要・スマホで作る、VRChat 写真のフォトブック」を最初に伝える。
//
// 設計参照:
//   - docs/spec/vrc_photobook_business_knowledge_v4.md §1.1〜§1.2 / §3.9 / §7.6
//   - design/design-system/(colors|typography|spacing|radius-shadow).md
//   - design/mockups/prototype/screens-a.jsx の LP モック（hero / 特徴 / 信頼バッジ）
//   - docs/plan/vrc-photobook-final-roadmap.md PR37
//
// 制約:
//   - MVP は全ページ noindex（root layout / middleware で付与）
//   - 作成導線（draft 新規作成）は MVP 範囲外のため CTA は /about と /help/manage-url の 2 系統に絞る
//   - 装飾的 gradient / hero scale typography は使わない（design-system 準拠）

import type { Metadata } from "next";
import Link from "next/link";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";

export const metadata: Metadata = {
  title: "VRC PhotoBook｜VRChat 写真をログイン不要で 1 冊に",
  description:
    "VRChat で撮った写真を、ログイン不要・スマホで 1 冊のフォトブックにまとめて X で共有できるサービスです（非公式ファンメイド）。",
};

const features: ReadonlyArray<{ title: string; body: string }> = [
  {
    title: "ログイン不要",
    body: "アカウント登録なしで作成・公開・編集まで完結します。共有 PC でも安心して使える設計です。",
  },
  {
    title: "管理 URL で編集・削除",
    body: "公開直後に表示される管理用 URL を保存しておけば、いつでも編集・公開停止・削除ができます。",
  },
  {
    title: "公開 / 限定公開 / 非公開",
    body: "目的に応じて公開範囲を選べます。MVP の既定は限定公開（URL を知っている人のみ閲覧可）です。",
  },
  {
    title: "通報窓口を用意",
    body: "公開フォトブックには通報リンクがあり、権利侵害・センシティブ・未成年関連の懸念を運営に届けられます。",
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

export default function HomePage() {
  return (
    <main className="mx-auto min-h-screen w-full max-w-screen-md bg-surface px-4 py-8 sm:px-6 sm:py-10">
      <header className="space-y-3" data-testid="lp-header">
        <p className="text-xs font-medium uppercase tracking-wide text-brand-teal">
          VRC PhotoBook
        </p>
        <h1 className="text-h1 text-ink">
          VRChat 写真を、ログイン不要で 1 冊に。
        </h1>
        <p className="text-body text-ink-strong">
          スマホで撮った VRChat の思い出を、フォトブックとしてまとめて X で共有できるサービスです。
          アカウント登録は要りません。管理用 URL で、いつでも編集・公開停止ができます。
        </p>
        <p className="text-xs text-ink-soft">
          本サービスは個人運営の非公式ファンメイドであり、VRChat 公式とは関係ありません。
        </p>
      </header>

      <section
        aria-labelledby="lp-features-heading"
        className="mt-8"
        data-testid="lp-features"
      >
        <h2 id="lp-features-heading" className="text-h2 text-ink">
          できること
        </h2>
        <ul className="mt-4 grid gap-3 sm:grid-cols-2">
          {features.map((f) => (
            <li
              key={f.title}
              className="rounded-lg border border-divider bg-surface p-4 shadow-sm"
            >
              <p className="text-sm font-bold text-ink">{f.title}</p>
              <p className="mt-1 text-sm text-ink-strong">{f.body}</p>
            </li>
          ))}
        </ul>
      </section>

      <section
        aria-labelledby="lp-cta-heading"
        className="mt-8"
        data-testid="lp-cta"
      >
        <h2 id="lp-cta-heading" className="text-h2 text-ink">
          まず読むページ
        </h2>
        <p className="mt-2 text-sm text-ink-medium">
          MVP では作成導線をまだ公開していません。サービス全体像と管理 URL の取り扱いを先にご確認ください。
        </p>
        <ul className="mt-3 grid gap-3">
          {ctaLinks.map((c) => (
            <li key={c.href}>
              <Link
                href={c.href}
                className="block rounded-lg border border-brand-teal bg-brand-teal-soft p-4 text-left hover:bg-surface-soft"
                data-testid={`lp-cta-${c.href.replace(/\//g, "-")}`}
              >
                <p className="text-sm font-bold text-brand-teal">{c.label}</p>
                <p className="mt-1 text-sm text-ink-strong">{c.description}</p>
              </Link>
            </li>
          ))}
        </ul>
      </section>

      <section
        aria-labelledby="lp-policy-heading"
        className="mt-8"
        data-testid="lp-policy"
      >
        <h2 id="lp-policy-heading" className="text-h2 text-ink">
          ポリシーと窓口
        </h2>
        <ul className="mt-3 list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            <Link href="/terms" className="underline hover:text-brand-teal">
              利用規約
            </Link>
            （投稿される画像の権利、運営による一時非表示・削除、免責など）
          </li>
          <li>
            <Link href="/privacy" className="underline hover:text-brand-teal">
              プライバシーポリシー
            </Link>
            （取得する情報、利用目的、保持期間、外部サービス利用）
          </li>
          <li>
            権利侵害・削除希望・不適切表現の報告は、対象フォトブックページの「このフォトブックを通報」から運営にお送りください。
          </li>
        </ul>
        <p className="mt-3 text-xs text-ink-soft">
          MVP 段階のため、ページの内容や仕様は予告なく変更されることがあります。
        </p>
      </section>

      <PublicPageFooter />
    </main>
  );
}
