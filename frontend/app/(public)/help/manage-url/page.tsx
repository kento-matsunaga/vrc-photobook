// Help / 管理 URL FAQ ページ (m2-design-refresh STOP β-2b-3)。
//
// 採用元 (design 正典):
//   - design/source/project/wf-screens-c.jsx:276-301 `WFHelp_M`
//   - design/source/project/wf-screens-c.jsx:302-328 `WFHelp_PC`
//   - design/source/project/wf-shared.jsx:29-48 `WFBrowser` (PC header → PublicTopBar)
//   - design/source/project/wireframe-styles.css:165-175 `.wf-box`
//   - design/source/project/wireframe-styles.css:351-369 `.wf-h1` / `.wf-eyebrow`
//
// design 正典構造:
//   1. PublicTopBar
//   2. eyebrow「Help」+ h1「管理 URL の使い方」(Mobile `<br/>` 改行: 「管理 URL の」/「使い方」)
//   3. Q1〜Q6 wf-box card stack:
//      Q1. 公開 URL と管理用 URL は別物ですか？
//      Q2. 管理用 URL は再表示できますか？
//      Q3. 管理用 URL を紛失したらどうなりますか？
//      Q4. 管理用 URL のおすすめの保存方法は？
//      Q5. 管理用 URL のメール送信機能はありますか？
//      Q6. 管理用 URL は外部に共有してよいですか？
//   4. PublicPageFooter (showTrustStrip=false / 既定)
//
// 「足りないものは足す」(plan §0.1):
//   - design は placeholder 3 line のみ。production の Q1-Q6 本文 (再表示不可 / 紛失時挙動 /
//     保存方法 / メール送信再選定中 / 外部共有禁止 等) は **削らない**
//   - design Mobile の「管理URL FAQ」 topbar title + back arrow は PublicTopBar に統合 (back 不要)
//   - design は Q 見出しを短ラベル「再表示不可」等で置くが、production は疑問形を維持
//     (読み手の自然さ優先、design 構造 Q1〜Q6 card は採用)
//   - 各 Q に `id="help-q{N}"` + `data-testid="help-q{N}"` を付与 (β-2b-3 では TOC は出さないが、
//     将来 TOC 追加時の anchor 用と test 固定用)
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow / Referrer-Policy が付与される
//   - 動的データ (token / Cookie / Secret / 任意 ID) は出さない
//   - 実 token / 実 URL は出さない (Q4 内 URL は token を含む旨を文章で説明)
//
// 設計参照:
//   - docs/plan/m2-design-refresh-stop-beta-2b-plan.md §3
//   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §2.3.4
//   - docs/adr/0006-email-provider-and-manage-url-delivery.md

import type { Metadata } from "next";
import type { ReactNode } from "react";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";

export const metadata: Metadata = {
  title: "管理 URL の使い方｜VRC PhotoBook",
  description:
    "VRC PhotoBook の管理用 URL の保存方法、紛失時の対応、メール送信機能の状況についてのよくある質問。",
};

// design `wf-screens-c.jsx:291-294` Q card 構造 (`wf-m-card` / `wf-box` + bold question + body)
// と `wireframe-styles.css:165-175` `.wf-box` 寸法 (rounded-lg / border / shadow-sm / padding) を統合。
// PolicyArticle と同じ shape だが、data-testid pattern を `help-q{N}` に揃えるため local 定義。
function HelpQuestion({
  id,
  number,
  title,
  children,
}: {
  /** anchor id (例: "help-q1")、将来 TOC 追加時の `href="#help-q1"` 用 */
  id: string;
  /** Q プレフィックス (例: "Q1.") */
  number: string;
  /** 疑問形タイトル (例: "公開 URL と管理用 URL は別物ですか？") */
  title: string;
  children: ReactNode;
}) {
  const headingId = `${id}-heading`;
  return (
    <section
      id={id}
      data-testid={id}
      aria-labelledby={headingId}
      className="scroll-mt-20 rounded-lg border border-divider-soft bg-surface p-5 shadow-sm sm:p-6"
    >
      <h2 id={headingId} className="text-h2 text-ink">
        <span className="mr-2 font-num text-sm font-bold text-teal-600">
          {number}
        </span>
        {title}
      </h2>
      {/* Unit 2 polish: PolicyArticle と同等の readability rhythm に揃える (leading 1.75 / gap 3) */}
      <div className="mt-3.5 space-y-3 text-sm leading-[1.75] text-ink-strong">
        {children}
      </div>
    </section>
  );
}

export default function ManageUrlHelpPage() {
  return (
    <>
      <PublicTopBar />
      <main className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9">
        <header className="space-y-2">
          <SectionEyebrow>Help</SectionEyebrow>
          {/* design `wf-screens-c.jsx:288` Mobile h1「管理 URL の<br/>使い方」, PC は 1 行 */}
          <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">
            管理 URL の<br className="sm:hidden" />使い方
          </h1>
        </header>

        <div className="mt-6 space-y-3 sm:mt-7 sm:space-y-4">
          <HelpQuestion
            id="help-q1"
            number="Q1."
            title="公開 URL と管理用 URL は別物ですか？"
          >
            <p>
              別物です。<strong>公開 URL</strong> は誰でも閲覧できる読み取り専用のページ、
              <strong>管理用 URL</strong> は作成者だけが編集 / 公開停止 / 削除に使えるリンクです。
              公開 URL を共有しても、管理権限は渡りません。
            </p>
          </HelpQuestion>

          <HelpQuestion
            id="help-q2"
            number="Q2."
            title="管理用 URL は再表示できますか？"
          >
            <p>
              できません。公開直後の Complete 画面で 1 度だけ表示されます。安全のため、運営側で
              再表示や検索はできない仕組みになっています。
            </p>
          </HelpQuestion>

          <HelpQuestion
            id="help-q3"
            number="Q3."
            title="管理用 URL を紛失したらどうなりますか？"
          >
            <p>
              現在のところ、紛失すると <strong>編集や公開停止ができなくなります</strong>。
              公開ページ自体は引き続き表示されます。メール送信機能は現在ご利用いただけません
              （後述）。紛失したフォトブックの取り扱いについてご相談がある場合は、
              公式 X アカウント等のお問い合わせ窓口までご連絡ください。
            </p>
          </HelpQuestion>

          <HelpQuestion
            id="help-q4"
            number="Q4."
            title="管理用 URL のおすすめの保存方法は？"
          >
            <ul className="list-disc space-y-1.5 pl-5 marker:text-teal-600">
              <li>パスワードマネージャ（1Password / Bitwarden / iCloud キーチェーン等）への保存</li>
              <li>Complete 画面の <strong>.txt ファイルとして保存</strong> ボタン</li>
              <li>Complete 画面の <strong>自分宛にメールを書く</strong> ボタンで自分のメール宛に送信</li>
              <li>Complete 画面の <strong>コピー</strong> ボタンでクリップボードに保持し、信頼できるメモ帳に貼り付け</li>
            </ul>
            <p className="text-xs text-ink-soft">
              ブラウザのスクリーンショットでも保存できますが、URL 文字列は token を含むため、
              画像が他者の目に触れる場所（SNS / 共有アルバム）に置かないようご注意ください。
            </p>
          </HelpQuestion>

          <HelpQuestion
            id="help-q5"
            number="Q5."
            title="管理用 URL のメール送信機能はありますか？"
          >
            <p>
              現在は <strong>提供していません</strong>。VRC PhotoBook はメール送信プロバイダの再選定中で、
              MVP では Complete 画面の 1 度表示 + ユーザーご自身の保存（.txt / 自分宛メール / コピー）
              を標準としています。提供開始の見通しが立ち次第、本ページでお知らせします。
            </p>
          </HelpQuestion>

          <HelpQuestion
            id="help-q6"
            number="Q6."
            title="管理用 URL は外部に共有してよいですか？"
          >
            <p>
              <strong>共有しないでください。</strong> 共有相手はあなたと同じ権限で編集 / 公開停止
              ができてしまいます。共同で編集したい場合でも、URL を直接渡すのは避け、運営の
              機能拡張をお待ちください。
            </p>
          </HelpQuestion>
        </div>

        <PublicPageFooter />
      </main>
    </>
  );
}
