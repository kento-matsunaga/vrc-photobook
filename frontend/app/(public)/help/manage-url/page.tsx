// 管理 URL についての FAQ ページ。
//
// 設計参照:
//   - docs/plan/m2-email-provider-reselection-plan.md §7（採用候補 A の FAQ / 紛失時案内）
//   - docs/adr/0006-email-provider-and-manage-url-delivery.md
//   - harness/work-logs/2026-05-01_pr37-design-rebuild-plan.md §4（PublicPageFooter に揃える）
//
// 公開ルートのため、middleware で X-Robots-Tag: noindex, nofollow / Referrer-Policy:
// strict-origin-when-cross-origin が付与される（frontend/middleware.ts）。
//
// このページに raw token / 実管理 URL は出さない。
import type { Metadata } from "next";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";

export const metadata: Metadata = {
  title: "管理 URL について｜VRC PhotoBook",
  description:
    "VRC PhotoBook の管理用 URL の保存方法、紛失時の対応、メール送信機能の状況についてのよくある質問。",
};

export default function ManageUrlHelpPage() {
  return (
    <main className="mx-auto max-w-screen-md space-y-6 p-4 sm:p-6">
      <header className="space-y-2">
        <p className="text-xs font-medium uppercase text-brand-teal">よくある質問</p>
        <h1 className="text-h1 text-ink">管理用 URL について</h1>
      </header>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold text-ink-strong">
          公開 URL と管理用 URL は別物ですか？
        </h2>
        <p className="text-sm text-ink-strong">
          別物です。<strong>公開 URL</strong> は誰でも閲覧できる読み取り専用のページ、
          <strong>管理用 URL</strong> は作成者だけが編集 / 公開停止 / 削除に使えるリンクです。
          公開 URL を共有しても、管理権限は渡りません。
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold text-ink-strong">
          管理用 URL は再表示できますか？
        </h2>
        <p className="text-sm text-ink-strong">
          できません。公開直後の Complete 画面で 1 度だけ表示されます。安全のため、運営側で
          再表示や検索はできない仕組みになっています。
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold text-ink-strong">
          管理用 URL を紛失したらどうなりますか？
        </h2>
        <p className="text-sm text-ink-strong">
          現在のところ、紛失すると <strong>編集や公開停止ができなくなります</strong>。
          公開ページ自体は引き続き表示されます。メール送信機能は現在ご利用いただけません
          （後述）。紛失したフォトブックの取り扱いについてご相談がある場合は、
          公式 X アカウント等のお問い合わせ窓口までご連絡ください。
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold text-ink-strong">
          管理用 URL のおすすめの保存方法は？
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>パスワードマネージャ（1Password / Bitwarden / iCloud キーチェーン等）への保存</li>
          <li>Complete 画面の <strong>.txt ファイルとして保存</strong> ボタン</li>
          <li>Complete 画面の <strong>自分宛にメールを書く</strong> ボタンで自分のメール宛に送信</li>
          <li>Complete 画面の <strong>コピー</strong> ボタンでクリップボードに保持し、信頼できるメモ帳に貼り付け</li>
        </ul>
        <p className="text-xs text-ink-soft">
          ブラウザのスクリーンショットでも保存できますが、URL 文字列は token を含むため、
          画像が他者の目に触れる場所（SNS / 共有アルバム）に置かないようご注意ください。
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold text-ink-strong">
          管理用 URL のメール送信機能はありますか？
        </h2>
        <p className="text-sm text-ink-strong">
          現在は <strong>提供していません</strong>。VRC PhotoBook はメール送信プロバイダの再選定中で、
          MVP では Complete 画面の 1 度表示 + ユーザーご自身の保存（.txt / 自分宛メール / コピー）
          を標準としています。提供開始の見通しが立ち次第、本ページでお知らせします。
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold text-ink-strong">
          管理用 URL は外部に共有してよいですか？
        </h2>
        <p className="text-sm text-ink-strong">
          <strong>共有しないでください。</strong> 共有相手はあなたと同じ権限で編集 / 公開停止
          ができてしまいます。共同で編集したい場合でも、URL を直接渡すのは避け、運営の
          機能拡張をお待ちください。
        </p>
      </section>

      <PublicPageFooter />
    </main>
  );
}
