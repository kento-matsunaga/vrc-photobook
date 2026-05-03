// 利用規約（m2-design-refresh STOP β-2b-1）。
//
// 採用元 (design 正典):
//   - design/source/project/wf-screens-c.jsx:331-357 `WFTerms_M`
//   - design/source/project/wf-screens-c.jsx:358-381 `WFTerms_PC`
//   - design/source/project/wf-shared.jsx:29-48 `WFBrowser` (PC header → PublicTopBar)
//   - design/source/project/wireframe-styles.css:165-175 `.wf-box`
//   - design/source/project/wireframe-styles.css:351-369 `.wf-h1` / `.wf-eyebrow` / `.wf-sub`
//   - design/source/project/wireframe-styles.css:398-425 `.wf-note`
//   - design/source/project/wireframe-styles.css:538-545 `.wf-toc`
//
// 業務知識 v4 §7.1 / §7.3 / §7.4 を Single Source of Truth として、
// 個人運営・非公式ファンメイド前提で MVP 段階の最低限の規約を提供する。
//
// design 正典構造:
//   1. PublicTopBar (showPrimaryCta=true)
//   2. eyebrow inline 「Terms · 最終更新 2026-05-01」(date を eyebrow に統合、別 p 廃止)
//   3. h1 「利用規約」 (text-h1 / sm:text-h1-lg)
//   4. PolicyNotice (wf-note 化)
//   5. PolicyToc (9 anchor、wf-toc left teal bar)
//   6. PolicyArticle x 9 (wf-box card)
//   7. PublicPageFooter (showTrustStrip=false / 既定)
//
// 「足りないものは足す」(plan §0.1):
//   - design は placeholder「Article 1〜9」のみ提示。9 article の本文・法務文言は
//     production truth として既存を維持 (削減なし)
//   - 法務文言 (権利 / 免責 / 準拠法 / 未成年 / 通報 / 改訂 等) は削らない
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow / Referrer-Policy が付与される
//   - 動的データ（token / Cookie / Secret / 任意 ID）は出さない（静的説明のみ）
//
// 設計参照:
//   - docs/plan/m2-design-refresh-stop-beta-2b-plan.md §1
//   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §2.3.2

import type { Metadata } from "next";
import Link from "next/link";

import {
  PolicyArticle,
  PolicyNotice,
  PolicyToc,
} from "@/components/Public/PolicyArticle";
import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";

export const metadata: Metadata = {
  title: "利用規約｜VRC PhotoBook",
  description:
    "VRC PhotoBook の利用規約。投稿画像の権利、禁止事項、運営による一時非表示・削除、管理 URL の取り扱い、免責事項を記載しています。",
};

const TOC = [
  { id: "terms-1", label: "第 1 条 サービスの目的と性質" },
  { id: "terms-2", label: "第 2 条 投稿される画像に関する権利と責任" },
  { id: "terms-3", label: "第 3 条 禁止事項" },
  { id: "terms-4", label: "第 4 条 運営の権限と運用" },
  { id: "terms-5", label: "第 5 条 管理 URL の取り扱い" },
  { id: "terms-6", label: "第 6 条 サービスの変更・停止" },
  { id: "terms-7", label: "第 7 条 免責" },
  { id: "terms-8", label: "第 8 条 お問い合わせ・準拠法" },
  { id: "terms-9", label: "第 9 条 改訂" },
];

export default function TermsPage() {
  return (
    <>
      <PublicTopBar />
      <main className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9">
        <header className="space-y-2">
          {/* design `wf-screens-c.jsx:335` `Terms · 最終更新 2026.05.01` を inline 化。日付はハイフン形式維持 */}
          <SectionEyebrow>Terms · 最終更新 2026-05-01</SectionEyebrow>
          <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">
            利用規約
          </h1>
        </header>

        <div className="mt-5">
          <PolicyNotice>
            本利用規約は、個人運営の非公式ファンメイドサービス「VRC PhotoBook」の MVP 段階版です。
            法律文書としての専門家レビューを経ていないため、ローンチ後に改訂される場合があります。
            利用にあたっては最新の本文をご確認ください。
          </PolicyNotice>
        </div>

        <div className="mt-5">
          <PolicyToc items={TOC} ariaLabel="利用規約 目次" />
        </div>

        <div className="mt-5 space-y-3 sm:space-y-4">
          <PolicyArticle id="terms-1" number="第 1 条" title="サービスの目的と性質">
            <p>
              VRC PhotoBook は、VRChat で撮影された写真を Web フォトブックとしてまとめて共有するためのサービスです。
              スマートフォンファースト・ログイン不要を方針とし、本サービスは VRChat 公式とは関係しない非公式ファンメイドサービスです。
            </p>
          </PolicyArticle>

          <PolicyArticle id="terms-2" number="第 2 条" title="投稿される画像に関する権利と責任">
            <ul className="list-disc space-y-1 pl-5">
              <li>
                作成者は、投稿する画像について必要な権利（著作権、被写体の許諾、ワールド・アバター等の取り扱い等）を有していることを表明するものとします。
              </li>
              <li>
                公開操作時の権利・配慮確認は、作成者の自己責任による宣言として記録されます。
              </li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="terms-3" number="第 3 条" title="禁止事項">
            <p>以下の行為を禁止します。</p>
            <ul className="list-disc space-y-1 pl-5">
              <li>他者のプライバシー侵害、無断転載、誹謗中傷</li>
              <li>性的表現、未成年を連想させる性的表現、暴力表現</li>
              <li>関係者の同意なく特定可能な情報を晒す行為（個人攻撃・晒し・doxxing 含む）</li>
              <li>運営によるサービス運用を不正に妨害する行為（自動化された大量投稿、過剰なリクエスト等）</li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="terms-4" number="第 4 条" title="運営の権限と運用">
            <ul className="list-disc space-y-1 pl-5">
              <li>
                通報等を受けた場合、運営は内容を確認し、必要に応じて<strong>一時非表示・削除等の措置</strong>を講じることができます。
              </li>
              <li>
                明らかな権利侵害や未成年保護関連の懸念がある場合、作成者への事前通知なく措置を講じる場合があります。
              </li>
              <li>運営は、判断・対応のすべてを手動で行います（自動的な処分は行いません）。</li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="terms-5" number="第 5 条" title="管理 URL の取り扱い">
            <ul className="list-disc space-y-1 pl-5">
              <li>
                管理 URL は、フォトブックの編集・公開停止・削除を行う唯一のリンクです。管理責任は作成者に帰属します。
              </li>
              <li>
                管理 URL の紛失・漏洩は作成者の責任となります。第三者に共有しないでください。
              </li>
              <li>
                管理 URL の保存方法は{" "}
                <Link
                  href="/help/manage-url"
                  className="text-teal-600 underline hover:text-teal-700"
                >
                  管理 URL について
                </Link>{" "}
                をご参照ください。
              </li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="terms-6" number="第 6 条" title="サービスの変更・停止">
            <p>
              本サービスは MVP 段階であり、機能・仕様・公開範囲は予告なく変更・停止される場合があります。
              長期にわたるデータ保持や継続提供は保証されません。
            </p>
          </PolicyArticle>

          <PolicyArticle id="terms-7" number="第 7 条" title="免責">
            <ul className="list-disc space-y-1 pl-5">
              <li>本サービスは現状有姿で提供され、運営は表明・保証を行いません。</li>
              <li>
                本サービスの利用または利用不能から生じるいかなる損害についても、運営は責任を負いません（適用法令で禁じられない範囲）。
              </li>
              <li>通信障害・第三者サービスの停止・データ消失その他の事象による不利益についても同様です。</li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="terms-8" number="第 8 条" title="お問い合わせ・準拠法">
            <ul className="list-disc space-y-1 pl-5">
              <li>
                権利侵害・削除希望・不適切表現の報告は、対象フォトブックページの「このフォトブックを通報」から行ってください。
                運営は通報を正式な窓口として扱います。
              </li>
              <li>
                その他のお問い合わせは、運営の X アカウント（
                <a
                  href="https://x.com/Noa_Fortevita"
                  rel="noopener noreferrer"
                  className="font-num text-teal-600 underline hover:text-teal-700"
                >
                  @Noa_Fortevita
                </a>
                、運営者表示名: ERENOA）までご連絡ください。メールでの問い合わせは MVP では提供していません。
              </li>
              <li>
                本規約に関する準拠法は日本法とし、紛争解決は東京地方裁判所を第一審の専属的合意管轄とします。
              </li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="terms-9" number="第 9 条" title="改訂">
            <p>
              本規約は予告なく改訂される場合があります。重要な変更があった場合は、本ページ上で告知します。
              法的レビューはローンチ後に別途実施し、その際にも改訂の対象となります。
            </p>
          </PolicyArticle>
        </div>

        <PublicPageFooter />
      </main>
    </>
  );
}
