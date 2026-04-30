// プライバシーポリシー。
//
// 業務知識 v4 §7.2〜§7.5 を Single Source of Truth として、
// 取得する情報・利用目的・保持期間・第三者提供・外部サービス利用を整理する。
//
// 法的レビュー前である旨を冒頭で明記し、ローンチ後に専門家レビューで改訂する想定。
//
// 設計参照:
//   - docs/spec/vrc_photobook_business_knowledge_v4.md §7.2 / §7.3 / §7.4 / §7.5 / §7.6
//   - docs/adr/0006-email-provider-and-manage-url-delivery.md（メール送信機能は再選定中）
//   - docs/runbook/usage-limit.md（IP ハッシュ運用）
//   - design/design-system/(typography|colors|spacing).md
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow / Referrer-Policy: strict-origin-when-cross-origin が付与される
//   - 動的データ（token / Cookie / Secret / 任意 ID）は出さない（静的説明のみ）
//   - 実装と齟齬する記述は書かない（メール提供中・第三者提供あり等は不可）

import type { Metadata } from "next";
import Link from "next/link";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";

export const metadata: Metadata = {
  title: "プライバシーポリシー｜VRC PhotoBook",
  description:
    "VRC PhotoBook のプライバシーポリシー。取得する情報、利用目的、保持期間、外部サービス利用、未成年保護の方針を記載しています。",
};

export default function PrivacyPage() {
  return (
    <main className="mx-auto min-h-screen w-full max-w-screen-md bg-surface px-4 py-8 sm:px-6 sm:py-10">
      <header className="space-y-2">
        <p className="text-xs font-medium uppercase tracking-wide text-brand-teal">
          Privacy
        </p>
        <h1 className="text-h1 text-ink">プライバシーポリシー</h1>
        <p className="text-sm text-ink-medium">
          最終更新: 2026-05-01（MVP 初版）
        </p>
      </header>

      <section className="mt-6 rounded-lg border border-divider bg-surface-soft p-4">
        <p className="text-sm text-ink-strong">
          本ポリシーは、個人運営の非公式ファンメイドサービス「VRC PhotoBook」の MVP 段階版です。
          法律文書としての専門家レビューを経ていないため、ローンチ後に改訂される場合があります。
          利用にあたっては最新の本文をご確認ください。
        </p>
      </section>

      <section aria-labelledby="privacy-1" className="mt-6 space-y-2">
        <h2 id="privacy-1" className="text-h2 text-ink">
          第 1 条 取得する情報
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            作成者がフォトブック作成時に入力する情報（表示名、任意で X ID、タイトル、本文、画像）
          </li>
          <li>
            通報機能に任意で入力された詳細・連絡先（短期保持、一定期間後に NULL 化）
          </li>
          <li>
            アクセスログおよび IP アドレスのハッシュ値（生 IP は保存しません）
          </li>
          <li>
            セッションを維持するための HttpOnly Cookie（管理 URL 入場・編集 token 入場・画像アップロード検証 session で使用）
          </li>
          <li>
            Cloudflare Turnstile の検証トークン（bot 検出のため、検証完了後にサーバ側で破棄）
          </li>
          <li>
            画像ファイルに付随するメタデータ（EXIF / XMP / IPTC 等）。位置情報を含む可能性のあるメタデータは公開時に除去します
          </li>
          <li>
            管理 URL 控えのメール送信先アドレス（<strong>現在この機能は提供していません</strong>。提供開始時のみ短期保持、用途完了後に NULL 化します）
          </li>
        </ul>
      </section>

      <section aria-labelledby="privacy-2" className="mt-6 space-y-2">
        <h2 id="privacy-2" className="text-h2 text-ink">
          第 2 条 利用目的
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>サービスの提供・公開・運用（フォトブック作成・公開 / 編集 / 削除）</li>
          <li>通報対応および通報者への必要な連絡</li>
          <li>
            荒らし・スパム抑止のためのレート制限（IP ハッシュおよび関連 scope ハッシュ）
          </li>
          <li>サービス品質の改善・障害分析</li>
          <li>
            管理 URL 控え送信機能の提供（提供開始時のみ、それ以外の目的では使用しません）
          </li>
        </ul>
      </section>

      <section aria-labelledby="privacy-3" className="mt-6 space-y-2">
        <h2 id="privacy-3" className="text-h2 text-ink">
          第 3 条 IP ハッシュ・scope ハッシュの取り扱い
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            生 IP アドレスは保存しません。受信時にバージョン管理されたソルトと SHA-256 でハッシュ化した値（IP ハッシュ）のみを記録します。
          </li>
          <li>
            通報・利用制限（レート制限）に用いる scope ハッシュも、IP ハッシュおよび対象 photobook ID の組み合わせを SHA-256 でハッシュ化した値です。
          </li>
          <li>
            ソルトはローテーション可能であり、ローテーション時には長期にわたる追跡性を意図的に失います。
          </li>
        </ul>
      </section>

      <section aria-labelledby="privacy-4" className="mt-6 space-y-2">
        <h2 id="privacy-4" className="text-h2 text-ink">
          第 4 条 第三者提供
        </h2>
        <p className="text-sm text-ink-strong">
          法令に基づく場合（裁判所の命令、警察の捜査関係事項照会等）または人の生命・身体・財産の保護のために必要な場合を除き、第三者へ提供することはありません。
        </p>
      </section>

      <section aria-labelledby="privacy-5" className="mt-6 space-y-2">
        <h2 id="privacy-5" className="text-h2 text-ink">
          第 5 条 利用する外部サービス
        </h2>
        <p className="text-sm text-ink-strong">
          サービス提供のため、以下の外部サービスを利用しています。
          各社のプライバシーポリシーも該当するため、必要に応じて各社の最新ポリシーをご確認ください。
        </p>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            Cloudflare Workers / Cloudflare R2 / Cloudflare Turnstile
            （フロントエンド配信、画像オブジェクトストレージ、bot 検証）
          </li>
          <li>
            Google Cloud Run / Google Cloud SQL / Google Secret Manager
            （バックエンド API、データベース、シークレット管理）
          </li>
          <li>
            メール送信プロバイダ（管理 URL 控え機能の提供時のみ）— 現在は再選定中であり提供していません
          </li>
        </ul>
      </section>

      <section aria-labelledby="privacy-6" className="mt-6 space-y-2">
        <h2 id="privacy-6" className="text-h2 text-ink">
          第 6 条 保持期間
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            論理削除されたフォトブックおよび画像は、一定の保持期間を経て物理削除されます。
          </li>
          <li>
            通報の詳細・連絡先・IP ハッシュは、用途完了後の一定期間内に NULL 化されます。
          </li>
          <li>
            管理 URL 控え送信先メールアドレスは、提供時のみ送信処理に必要な短期間（24 時間目安）でのみ保持されます。
          </li>
          <li>
            利用制限（レート制限）に用いる scope ハッシュは、固定窓の期限経過後に削除対象となります。
          </li>
        </ul>
      </section>

      <section aria-labelledby="privacy-7" className="mt-6 space-y-2">
        <h2 id="privacy-7" className="text-h2 text-ink">
          第 7 条 削除請求・権利侵害申立て
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            作成者は、自身のフォトブックの管理 URL を用いて、いつでも自分のフォトブックを削除できます。
          </li>
          <li>
            被写体・権利者など第三者からの削除申立ては、対象フォトブックページの「このフォトブックを通報」から運営にお送りください。
            運営は通報を正式な窓口として扱います（業務上のフローは{" "}
            <Link href="/terms" className="underline text-brand-teal">
              利用規約
            </Link>
            第 4 条参照）。
          </li>
        </ul>
      </section>

      <section aria-labelledby="privacy-8" className="mt-6 space-y-2">
        <h2 id="privacy-8" className="text-h2 text-ink">
          第 8 条 未成年保護
        </h2>
        <p className="text-sm text-ink-strong">
          本サービスは未成年の利用を制限しませんが、未成年を被写体とするセンシティブな表現、
          あるいはアバターを通じて未成年を連想させる性的表現は禁止します。
          通報カテゴリ「年齢・センシティブに関する問題（minor_safety_concern）」は優先的に対応し、必要に応じて即時一時非表示にします。
        </p>
      </section>

      <section aria-labelledby="privacy-9" className="mt-6 space-y-2">
        <h2 id="privacy-9" className="text-h2 text-ink">
          第 9 条 SEO・検索エンジン
        </h2>
        <p className="text-sm text-ink-strong">
          MVP では、本サービスのすべてのページに <code className="font-num text-xs text-ink">noindex, nofollow</code> を付与しています。
          検索エンジンへの掲載や横断的な検索の対象とはなりません。
        </p>
      </section>

      <section aria-labelledby="privacy-10" className="mt-6 space-y-2">
        <h2 id="privacy-10" className="text-h2 text-ink">
          第 10 条 改訂
        </h2>
        <p className="text-sm text-ink-strong">
          本ポリシーは予告なく改訂される場合があります。重要な変更があった場合は本ページ上で告知します。
          法的レビューはローンチ後に別途実施し、その際にも改訂の対象となります。
        </p>
      </section>

      <PublicPageFooter />
    </main>
  );
}
