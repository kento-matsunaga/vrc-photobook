// VRC PhotoBook について（About）。
//
// サービスの背景・できること・できないことを、業務知識 v4 §3 機能群に沿って整理する。
// MVP 段階のため、進行中の機能も透明性を持って明示する。
//
// 設計参照:
//   - docs/spec/vrc_photobook_business_knowledge_v4.md §1 / §2.6 / §3
//   - design/design-system/(typography|colors|spacing).md
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow が付与される
//   - 動的データ（token / Cookie / Secret / 任意 ID）は出さない（静的説明のみ）

import type { Metadata } from "next";
import Link from "next/link";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";

export const metadata: Metadata = {
  title: "VRC PhotoBook について｜About",
  description:
    "VRC PhotoBook は VRChat 写真をログイン不要でフォトブックにまとめる非公式ファンメイドサービスです。MVP 段階のできること・できないことを記載しています。",
};

const canDo: ReadonlyArray<{ title: string; body: string }> = [
  {
    title: "ログイン不要での作成・公開",
    body: "アカウント登録なしで、スマホからフォトブックを作成・公開できます（業務知識 v4 §3.1 / §3.2）。",
  },
  {
    title: "管理 URL での編集・削除",
    body: "公開直後に表示される管理用 URL を保存しておけば、いつでも編集・公開停止・削除ができます（業務知識 v4 §3.4 / §3.5）。",
  },
  {
    title: "公開範囲の選択",
    body: "公開（誰でも閲覧可）/ 限定公開（URL を知っている人のみ、MVP の既定）/ 非公開（管理 URL を持つ人のみ）から選べます（業務知識 v4 §2.6）。",
  },
  {
    title: "X 共有用 OGP の自動生成",
    body: "公開時に OGP 画像を自動生成し、X タイムラインで見やすい形で共有できます（業務知識 v4 §3.8）。",
  },
  {
    title: "通報窓口",
    body: "公開フォトブックの閲覧画面から、権利侵害・センシティブ・未成年関連の懸念を通報できます。運営は通報を手動で確認・対応します（業務知識 v4 §3.6）。",
  },
  {
    title: "荒らし抑止のレート制限",
    body: "通報・アップロード・公開操作には、IP ハッシュベースの利用制限を設けています。検証は Cloudflare Turnstile を併用します（業務知識 v4 §3.7）。",
  },
];

const cannotDo: ReadonlyArray<{ title: string; body: string }> = [
  {
    title: "管理 URL のメール送信（再選定中）",
    body: "メールプロバイダの選定中のため、MVP では公開完了画面での 1 回表示と、ユーザーご自身による保存（.txt / 自分宛メール / コピー）を標準としています。提供開始時には改めてお知らせします。",
  },
  {
    title: "X 連携ログイン（Phase 2）",
    body: "MVP ではログイン機能を提供していません。複数フォトブックの横断管理や有料機能は、ログインを伴う将来フェーズの想定です。",
  },
  {
    title: "検索エンジンへの掲載",
    body: "MVP では全ページに noindex を付与しており、検索エンジンには掲載されません（業務知識 v4 §7.6）。",
  },
  {
    title: "運営側の Web 管理画面",
    body: "運営対応は cmd/ops CLI で行い、MVP では Web 管理画面は提供しません（業務知識 v4 §6 / ADR-0002）。",
  },
];

export default function AboutPage() {
  return (
    <main className="mx-auto min-h-screen w-full max-w-screen-md bg-surface px-4 py-8 sm:px-6 sm:py-10">
      <header className="space-y-2">
        <p className="text-xs font-medium uppercase tracking-wide text-brand-teal">
          About
        </p>
        <h1 className="text-h1 text-ink">VRC PhotoBook について</h1>
        <p className="text-sm text-ink-medium">
          VRChat コミュニティ向け、ログイン不要で動くフォトブックサービスです。
        </p>
      </header>

      <section aria-labelledby="about-overview" className="mt-6 space-y-2">
        <h2 id="about-overview" className="text-h2 text-ink">
          サービスの位置づけ
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>個人運営の非公式ファンメイドサービス（VRChat 公式とは関係ありません）</li>
          <li>運営者表示名: ERENOA / 連絡用 X アカウント:{" "}
            <a
              href="https://x.com/Noa_Fortevita"
              className="underline text-brand-teal"
              rel="noopener noreferrer"
            >
              @Noa_Fortevita
            </a>
          </li>
          <li>
            VRChat で撮影した写真を、フォトブックとしてまとめ、X で共有することを主目的としたサービスです。
          </li>
          <li>
            スマホファースト・ログイン不要を方針とし、管理 URL 方式で編集・削除を実現しています。
          </li>
          <li>
            現在は MVP 段階のため、機能・仕様は順次追加・変更されます。
          </li>
        </ul>
      </section>

      <section aria-labelledby="about-can" className="mt-8 space-y-3">
        <h2 id="about-can" className="text-h2 text-ink">
          できること
        </h2>
        <ul className="grid gap-3 sm:grid-cols-2">
          {canDo.map((c) => (
            <li
              key={c.title}
              className="rounded-lg border border-divider bg-surface p-4 shadow-sm"
            >
              <p className="text-sm font-bold text-ink">{c.title}</p>
              <p className="mt-1 text-sm text-ink-strong">{c.body}</p>
            </li>
          ))}
        </ul>
      </section>

      <section aria-labelledby="about-cannot" className="mt-8 space-y-3">
        <h2 id="about-cannot" className="text-h2 text-ink">
          MVP ではできないこと
        </h2>
        <ul className="grid gap-3 sm:grid-cols-2">
          {cannotDo.map((c) => (
            <li
              key={c.title}
              className="rounded-lg border border-divider bg-surface-soft p-4"
            >
              <p className="text-sm font-bold text-ink">{c.title}</p>
              <p className="mt-1 text-sm text-ink-strong">{c.body}</p>
            </li>
          ))}
        </ul>
      </section>

      <section aria-labelledby="about-policy" className="mt-8 space-y-2">
        <h2 id="about-policy" className="text-h2 text-ink">
          ポリシーと窓口
        </h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-ink-strong">
          <li>
            <Link href="/terms" className="underline text-brand-teal">
              利用規約
            </Link>{" "}
            （投稿される画像の権利、運営による一時非表示・削除、免責など）
          </li>
          <li>
            <Link href="/privacy" className="underline text-brand-teal">
              プライバシーポリシー
            </Link>{" "}
            （取得する情報、利用目的、保持期間、外部サービス利用、未成年保護）
          </li>
          <li>
            <Link href="/help/manage-url" className="underline text-brand-teal">
              管理 URL について
            </Link>{" "}
            （管理用 URL の保存方法、紛失時の対応、メール送信機能の状況）
          </li>
          <li>
            権利侵害・削除希望・不適切表現の報告は、対象フォトブックの「このフォトブックを通報」リンクから運営にお送りください。
          </li>
        </ul>
        <p className="text-xs text-ink-soft">
          法的レビューはローンチ後に別途実施し、規約・ポリシーは順次改訂されます。
        </p>
      </section>

      <PublicPageFooter />
    </main>
  );
}
