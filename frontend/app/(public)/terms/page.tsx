// 利用規約。
//
// 業務知識 v4 §7.1 を Single Source of Truth として、
// 個人運営・非公式ファンメイド前提で MVP 段階の最低限の規約を提供する。
//
// 法的レビュー前である旨を冒頭で明記し、ローンチ後に専門家レビューで改訂する想定。
//
// 設計参照:
//   - docs/spec/vrc_photobook_business_knowledge_v4.md §7.1（規約）/ §7.3（権利侵害申立て）/ §7.4（未成年保護）
//   - design/design-system/(typography|colors|spacing).md
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow / Referrer-Policy: strict-origin-when-cross-origin が付与される（frontend/middleware.ts）
//   - 動的データ（token / Cookie / Secret / 任意 ID）は出さない（静的説明のみ）

import type { Metadata } from "next";
import Link from "next/link";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";

export const metadata: Metadata = {
  title: "利用規約｜VRC PhotoBook",
  description:
    "VRC PhotoBook の利用規約。投稿画像の権利、禁止事項、運営による一時非表示・削除、管理 URL の取り扱い、免責事項を記載しています。",
};

export default function TermsPage() {
  return (
    <main className="mx-auto min-h-screen w-full max-w-screen-md bg-surface px-4 py-8 sm:px-6 sm:py-10">
      <header className="space-y-2">
        <p className="text-xs font-medium uppercase tracking-wide text-brand-teal">
          Terms
        </p>
        <h1 className="text-h1 text-ink">利用規約</h1>
        <p className="text-sm text-ink-medium">
          最終更新: 2026-05-01（MVP 初版）
        </p>
      </header>

      <section className="mt-6 rounded-lg border border-divider bg-surface-soft p-4">
        <p className="text-sm text-ink-strong">
          本利用規約は、個人運営の非公式ファンメイドサービス「VRC PhotoBook」の MVP 段階版です。
          法律文書としての専門家レビューを経ていないため、ローンチ後に改訂される場合があります。
          利用にあたっては最新の本文をご確認ください。
        </p>
      </section>

      <section aria-labelledby="terms-1" className="mt-6 space-y-2">
        <h2 id="terms-1" className="text-h2 text-ink">
          第 1 条 サービスの目的と性質
        </h2>
        <p className="text-sm text-ink-strong">
          VRC PhotoBook は、VRChat で撮影された写真を Web フォトブックとしてまとめて共有するためのサービスです。
          スマートフォンファースト・ログイン不要を方針とし、本サービスは VRChat 公式とは関係しない非公式ファンメイドサービスです。
        </p>
      </section>

      <section aria-labelledby="terms-2" className="mt-6 space-y-2">
        <h2 id="terms-2" className="text-h2 text-ink">
          第 2 条 投稿される画像に関する権利と責任
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            作成者は、投稿する画像について必要な権利（著作権、被写体の許諾、ワールド・アバター等の取り扱い等）を有していることを表明するものとします。
          </li>
          <li>
            公開操作時の権利・配慮確認は、作成者の自己責任による宣言として記録されます。
          </li>
        </ul>
      </section>

      <section aria-labelledby="terms-3" className="mt-6 space-y-2">
        <h2 id="terms-3" className="text-h2 text-ink">
          第 3 条 禁止事項
        </h2>
        <p className="text-sm text-ink-strong">
          以下の行為を禁止します。
        </p>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>他者のプライバシー侵害、無断転載、誹謗中傷</li>
          <li>性的表現、未成年を連想させる性的表現、暴力表現</li>
          <li>
            関係者の同意なく特定可能な情報を晒す行為（個人攻撃・晒し・doxxing 含む）
          </li>
          <li>
            運営によるサービス運用を不正に妨害する行為（自動化された大量投稿、過剰なリクエスト等）
          </li>
        </ul>
      </section>

      <section aria-labelledby="terms-4" className="mt-6 space-y-2">
        <h2 id="terms-4" className="text-h2 text-ink">
          第 4 条 運営の権限と運用
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            通報等を受けた場合、運営は内容を確認し、必要に応じて<strong>一時非表示・削除等の措置</strong>を講じることができます。
          </li>
          <li>
            明らかな権利侵害や未成年保護関連の懸念がある場合、作成者への事前通知なく措置を講じる場合があります。
          </li>
          <li>
            運営は、判断・対応のすべてを手動で行います（自動的な処分は行いません）。
          </li>
        </ul>
      </section>

      <section aria-labelledby="terms-5" className="mt-6 space-y-2">
        <h2 id="terms-5" className="text-h2 text-ink">
          第 5 条 管理 URL の取り扱い
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            管理 URL は、フォトブックの編集・公開停止・削除を行う唯一のリンクです。管理責任は作成者に帰属します。
          </li>
          <li>
            管理 URL の紛失・漏洩は作成者の責任となります。第三者に共有しないでください。
          </li>
          <li>
            管理 URL の保存方法は{" "}
            <Link href="/help/manage-url" className="underline text-brand-teal">
              管理 URL について
            </Link>{" "}
            をご参照ください。
          </li>
        </ul>
      </section>

      <section aria-labelledby="terms-6" className="mt-6 space-y-2">
        <h2 id="terms-6" className="text-h2 text-ink">
          第 6 条 サービスの変更・停止
        </h2>
        <p className="text-sm text-ink-strong">
          本サービスは MVP 段階であり、機能・仕様・公開範囲は予告なく変更・停止される場合があります。
          長期にわたるデータ保持や継続提供は保証されません。
        </p>
      </section>

      <section aria-labelledby="terms-7" className="mt-6 space-y-2">
        <h2 id="terms-7" className="text-h2 text-ink">
          第 7 条 免責
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            本サービスは現状有姿で提供され、運営は表明・保証を行いません。
          </li>
          <li>
            本サービスの利用または利用不能から生じるいかなる損害についても、運営は責任を負いません（適用法令で禁じられない範囲）。
          </li>
          <li>
            通信障害・第三者サービスの停止・データ消失その他の事象による不利益についても同様です。
          </li>
        </ul>
      </section>

      <section aria-labelledby="terms-8" className="mt-6 space-y-2">
        <h2 id="terms-8" className="text-h2 text-ink">
          第 8 条 お問い合わせ・準拠法
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            権利侵害・削除希望・不適切表現の報告は、対象フォトブックページの「このフォトブックを通報」から行ってください。
            運営は通報を正式な窓口として扱います。
          </li>
          <li>
            その他のお問い合わせは、運営の X アカウント（
            <a
              href="https://x.com/Noa_Fortevita"
              className="underline text-brand-teal"
              rel="noopener noreferrer"
            >
              @Noa_Fortevita
            </a>
            、運営者表示名: ERENOA）までご連絡ください。
            メールでの問い合わせは MVP では提供していません。
          </li>
          <li>
            本規約に関する準拠法は日本法とし、紛争解決は東京地方裁判所を第一審の専属的合意管轄とします。
          </li>
        </ul>
      </section>

      <section aria-labelledby="terms-9" className="mt-6 space-y-2">
        <h2 id="terms-9" className="text-h2 text-ink">
          第 9 条 改訂
        </h2>
        <p className="text-sm text-ink-strong">
          本規約は予告なく改訂される場合があります。重要な変更があった場合は、本ページ上で告知します。
          法的レビューはローンチ後に別途実施し、その際にも改訂の対象となります。
        </p>
      </section>

      <PublicPageFooter />
    </main>
  );
}
