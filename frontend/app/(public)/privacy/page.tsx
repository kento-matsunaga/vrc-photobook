// プライバシーポリシー（m2-design-refresh STOP β-2b-1）。
//
// 採用元 (design 正典):
//   - design/source/project/wf-screens-c.jsx:384-412 `WFPrivacy_M`
//   - design/source/project/wf-screens-c.jsx:413-442 `WFPrivacy_PC`
//   - design/source/project/wf-shared.jsx:29-48 `WFBrowser` (PC header → PublicTopBar)
//   - design/source/project/wireframe-styles.css:165-175 `.wf-box`
//   - design/source/project/wireframe-styles.css:351-369 `.wf-h1` / `.wf-eyebrow`
//   - design/source/project/wireframe-styles.css:372-395 `.wf-badge` (chip)
//   - design/source/project/wireframe-styles.css:398-425 `.wf-note`
//   - design/source/project/wireframe-styles.css:538-545 `.wf-toc`
//
// 業務知識 v4 §7.2〜§7.5 を Single Source of Truth として、取得情報・利用目的・
// 保持期間・第三者提供・外部サービス利用・未成年保護を整理する。
//
// design 正典構造:
//   1. PublicTopBar
//   2. eyebrow inline 「Privacy · 最終更新 2026-05-01」
//   3. h1 「プライバシーポリシー」
//   4. PolicyNotice
//   5. PolicyToc (10 anchor)
//   6. PolicyArticle x 10
//   7. External services chips (production truth 5 件)
//   8. PublicPageFooter (showTrustStrip=false)
//
// External services chips の確定方針 (plan §1.4 / §1.6):
//   採用 5 件: Cloudflare Workers / Turnstile / R2 / Cloud Run / Cloud SQL
//   削除: Google Secret Manager (infra-only、ユーザに見せる privacy chip としては過剰)
//   追記しない: Sentry / PostHog の「未採用」補足文 (誤読リスク)。SSR test の不存在
//   assert で十分 (`public-pages.test.tsx`)
//
// 「足りないものは足す」(plan §0.1):
//   - design は placeholder のみ提示。10 article の本文・法務文言は production truth として
//     既存を維持 (削減なし)
//   - 法務文言 (個人情報 / IP ハッシュ / ソルト / 第三者提供 / 保持期間 / 削除請求 /
//     未成年 / SEO 等) は削らない
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow / Referrer-Policy が付与される
//   - 動的データ（token / Cookie / Secret / 任意 ID）は出さない（静的説明のみ）
//   - 実装と齟齬する記述は書かない（メール提供中・第三者提供あり等は不可）
//
// 設計参照:
//   - docs/plan/m2-design-refresh-stop-beta-2b-plan.md §1
//   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §2.3.3
//   - docs/adr/0006-email-provider-and-manage-url-delivery.md

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
  title: "プライバシーポリシー｜VRC PhotoBook",
  description:
    "VRC PhotoBook のプライバシーポリシー。取得する情報、利用目的、保持期間、外部サービス利用、未成年保護の方針を記載しています。",
};

const TOC = [
  { id: "privacy-1", label: "第 1 条 取得する情報" },
  { id: "privacy-2", label: "第 2 条 利用目的" },
  { id: "privacy-3", label: "第 3 条 IP / scope ハッシュの取り扱い" },
  { id: "privacy-4", label: "第 4 条 第三者提供" },
  { id: "privacy-5", label: "第 5 条 利用する外部サービス" },
  { id: "privacy-6", label: "第 6 条 保持期間" },
  { id: "privacy-7", label: "第 7 条 削除請求・権利侵害申立て" },
  { id: "privacy-8", label: "第 8 条 未成年保護" },
  { id: "privacy-9", label: "第 9 条 SEO・検索エンジン" },
  { id: "privacy-10", label: "第 10 条 改訂" },
];

// production truth に揃えた 5 chip (plan §1.4)。
// Cloudflare 内サービス (Workers / R2 / Turnstile) は粒度識別のため個別 chip。
// 「Cloudflare」単独だと広すぎるため Cloudflare Workers と明示する。
const externalServices: ReadonlyArray<{
  /** chip 表示名 (design 短ラベル方式)。 */
  name: string;
  /** chip 副表示の用途説明 (補足情報量保持)。 */
  purpose: string;
  /** data-testid suffix (chip 数 5 固定 + 名前識別)。 */
  slug: string;
}> = [
  { name: "Cloudflare Workers", purpose: "フロントエンド配信", slug: "cloudflare-workers" },
  { name: "Turnstile", purpose: "bot 検証", slug: "turnstile" },
  { name: "R2", purpose: "画像オブジェクトストレージ", slug: "r2" },
  { name: "Cloud Run", purpose: "バックエンド API", slug: "cloud-run" },
  { name: "Cloud SQL", purpose: "データベース", slug: "cloud-sql" },
];

export default function PrivacyPage() {
  return (
    <>
      <PublicTopBar />
      <main className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9">
        <header className="space-y-2">
          {/* design `wf-screens-c.jsx:388` `Privacy · 最終更新 2026.05.01` を inline 化、日付ハイフン形式 */}
          <SectionEyebrow>Privacy · 最終更新 2026-05-01</SectionEyebrow>
          <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">
            プライバシーポリシー
          </h1>
        </header>

        <div className="mt-5">
          <PolicyNotice>
            本ポリシーは、個人運営の非公式ファンメイドサービス「VRC PhotoBook」の MVP 段階版です。
            法律文書としての専門家レビューを経ていないため、ローンチ後に改訂される場合があります。
            利用にあたっては最新の本文をご確認ください。
          </PolicyNotice>
        </div>

        <div className="mt-5">
          <PolicyToc items={TOC} ariaLabel="プライバシーポリシー 目次" />
        </div>

        <div className="mt-5 space-y-3 sm:space-y-4">
          <PolicyArticle id="privacy-1" number="第 1 条" title="取得する情報">
            <ul className="list-disc space-y-1 pl-5">
              <li>
                作成者がフォトブック作成時に入力する情報（表示名、任意で X ID、タイトル、本文、画像）
              </li>
              <li>通報機能に任意で入力された詳細・連絡先（短期保持、一定期間後に NULL 化）</li>
              <li>アクセスログおよび IP アドレスのハッシュ値（生 IP は保存しません）</li>
              <li>セッションを維持するための HttpOnly Cookie（管理 URL 入場・編集 token 入場・画像アップロード検証 session で使用）</li>
              <li>Cloudflare Turnstile の検証トークン（bot 検出のため、検証完了後にサーバ側で破棄）</li>
              <li>画像ファイルに付随するメタデータ（EXIF / XMP / IPTC 等）。位置情報を含む可能性のあるメタデータは公開時に除去します</li>
              <li>
                管理 URL 控えのメール送信先アドレス（<strong>現在この機能は提供していません</strong>。提供開始時のみ短期保持、用途完了後に NULL 化します）
              </li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="privacy-2" number="第 2 条" title="利用目的">
            <ul className="list-disc space-y-1 pl-5">
              <li>サービスの提供・公開・運用（フォトブック作成・公開 / 編集 / 削除）</li>
              <li>通報対応および通報者への必要な連絡</li>
              <li>荒らし・スパム抑止のためのレート制限（IP ハッシュおよび関連 scope ハッシュ）</li>
              <li>サービス品質の改善・障害分析</li>
              <li>管理 URL 控え送信機能の提供（提供開始時のみ、それ以外の目的では使用しません）</li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="privacy-3" number="第 3 条" title="IP ハッシュ・scope ハッシュの取り扱い">
            <ul className="list-disc space-y-1 pl-5">
              <li>生 IP アドレスは保存しません。受信時にバージョン管理されたソルトと SHA-256 でハッシュ化した値（IP ハッシュ）のみを記録します。</li>
              <li>通報・利用制限（レート制限）に用いる scope ハッシュも、IP ハッシュおよび対象 photobook ID の組み合わせを SHA-256 でハッシュ化した値です。</li>
              <li>ソルトはローテーション可能であり、ローテーション時には長期にわたる追跡性を意図的に失います。</li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="privacy-4" number="第 4 条" title="第三者提供">
            <p>
              法令に基づく場合（裁判所の命令、警察の捜査関係事項照会等）または人の生命・身体・財産の保護のために必要な場合を除き、第三者へ提供することはありません。
            </p>
          </PolicyArticle>

          <PolicyArticle id="privacy-5" number="第 5 条" title="利用する外部サービス">
            <p>
              サービス提供のため、以下の外部サービスを利用しています。各社のプライバシーポリシーも該当するため、必要に応じて各社の最新ポリシーをご確認ください。
            </p>
            <ul
              className="mt-3 grid gap-2 sm:grid-cols-2"
              data-testid="privacy-external-services"
            >
              {externalServices.map((s) => (
                <li
                  key={s.slug}
                  data-testid={`privacy-chip-${s.slug}`}
                  className="flex items-center justify-between gap-3 rounded-md border border-teal-100 bg-teal-50 px-3 py-2"
                >
                  <span className="font-num text-sm font-bold text-teal-700">
                    {s.name}
                  </span>
                  <span className="text-xs text-ink-medium">{s.purpose}</span>
                </li>
              ))}
            </ul>
            <p className="mt-3 text-sm">
              メール送信プロバイダ（管理 URL 控え機能の提供時のみ）— 現在は再選定中であり提供していません。
            </p>
          </PolicyArticle>

          <PolicyArticle id="privacy-6" number="第 6 条" title="保持期間">
            <ul className="list-disc space-y-1 pl-5">
              <li>論理削除されたフォトブックおよび画像は、一定の保持期間を経て物理削除されます。</li>
              <li>通報の詳細・連絡先・IP ハッシュは、用途完了後の一定期間内に NULL 化されます。</li>
              <li>管理 URL 控え送信先メールアドレスは、提供時のみ送信処理に必要な短期間（24 時間目安）でのみ保持されます。</li>
              <li>利用制限（レート制限）に用いる scope ハッシュは、固定窓の期限経過後に削除対象となります。</li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="privacy-7" number="第 7 条" title="削除請求・権利侵害申立て">
            <ul className="list-disc space-y-1 pl-5">
              <li>作成者は、自身のフォトブックの管理 URL を用いて、いつでも自分のフォトブックを削除できます。</li>
              <li>
                被写体・権利者など第三者からの削除申立ては、対象フォトブックページの「このフォトブックを通報」から運営にお送りください。
                運営は通報を正式な窓口として扱います（業務上のフローは{" "}
                <Link
                  href="/terms"
                  className="text-teal-600 underline hover:text-teal-700"
                >
                  利用規約
                </Link>
                第 4 条参照）。
              </li>
            </ul>
          </PolicyArticle>

          <PolicyArticle id="privacy-8" number="第 8 条" title="未成年保護">
            <p>
              本サービスは未成年の利用を制限しませんが、未成年を被写体とするセンシティブな表現、
              あるいはアバターを通じて未成年を連想させる性的表現は禁止します。
              通報カテゴリ「年齢・センシティブに関する問題（minor_safety_concern）」は優先的に対応し、必要に応じて即時一時非表示にします。
            </p>
          </PolicyArticle>

          <PolicyArticle id="privacy-9" number="第 9 条" title="SEO・検索エンジン">
            <p>
              MVP では、本サービスのすべてのページに{" "}
              <code className="font-num text-xs text-ink">noindex, nofollow</code> を付与しています。
              検索エンジンへの掲載や横断的な検索の対象とはなりません。
            </p>
          </PolicyArticle>

          <PolicyArticle id="privacy-10" number="第 10 条" title="改訂">
            <p>
              本ポリシーは予告なく改訂される場合があります。重要な変更があった場合は本ページ上で告知します。
              法的レビューはローンチ後に別途実施し、その際にも改訂の対象となります。
            </p>
          </PolicyArticle>
        </div>

        <PublicPageFooter />
      </main>
    </>
  );
}
