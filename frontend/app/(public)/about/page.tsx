// VRC PhotoBook について（About、design rebuild）。
//
// 採用元:
//   - LP の `feature-cell` パターン（ico + t + s）
//   - design/mockups/prototype/screens-b.jsx Viewer "Memories card" の dashed border
//     区切り（**1 箇所のみ**：サービスの位置づけ card 内のメタ）
//   - design/design-system/(typography|colors|spacing|radius-shadow).md
//
// 設計参照:
//   - harness/work-logs/2026-05-01_pr37-design-rebuild-plan.md §3.2 / §6
//   - docs/spec/vrc_photobook_business_knowledge_v4.md §1 / §2.6 / §3
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow が付与される
//   - 動的データ（token / Cookie / Secret / 任意 ID）は出さない（静的説明のみ）

import type { Metadata } from "next";
import Link from "next/link";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";

export const metadata: Metadata = {
  title: "VRC PhotoBook について｜About",
  description:
    "VRC PhotoBook は VRChat 写真をログイン不要でフォトブックにまとめる非公式ファンメイドサービスです。MVP 段階のできること・できないことを記載しています。",
};

const canDo: ReadonlyArray<{ title: string; body: string; iconPath: string }> = [
  {
    title: "ログイン不要での作成・公開",
    body: "アカウント登録なしで、スマホからフォトブックを作成・公開できます（業務知識 v4 §3.1 / §3.2）。",
    iconPath:
      "M17 20v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2 M9 7a4 4 0 1 0 0 0",
  },
  {
    title: "管理 URL での編集・削除",
    body: "公開直後に表示される管理用 URL を保存しておけば、いつでも編集・公開停止・削除ができます（v4 §3.4 / §3.5）。",
    iconPath:
      "M12 20h9 M16.5 3.5a2.1 2.1 0 1 1 3 3L7 19l-4 1 1-4 12.5-12.5z",
  },
  {
    title: "公開範囲の選択",
    body: "公開（誰でも閲覧可）/ 限定公開（URL を知っている人のみ、MVP の既定）/ 非公開（管理 URL 保持者のみ）から選べます（v4 §2.6）。",
    iconPath:
      "M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6",
  },
  {
    title: "X 共有用 OGP の自動生成",
    body: "公開時に OGP 画像を自動生成し、X タイムラインで見やすい形で共有できます（v4 §3.8）。",
    iconPath:
      "M3 4h18v16H3z M3 10h18 M9 4v16",
  },
  {
    title: "通報窓口",
    body: "公開フォトブックの閲覧画面から、権利侵害・センシティブ・未成年関連の懸念を通報できます。運営は通報を手動で確認・対応します（v4 §3.6）。",
    iconPath:
      "M2 12a10 10 0 1 0 20 0 10 10 0 0 0-20 0 M12 8v4 M12 16h.01",
  },
  {
    title: "荒らし抑止のレート制限",
    body: "通報・アップロード・公開操作には、IP ハッシュベースの利用制限を設けています。検証は Cloudflare Turnstile を併用します（v4 §3.7）。",
    iconPath:
      "M4 10h16v11H4z M8 10V7a4 4 0 0 1 8 0v3",
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
    body: "MVP では全ページに noindex を付与しており、検索エンジンには掲載されません（v4 §7.6）。",
  },
  {
    title: "運営側の Web 管理画面",
    body: "運営対応は cmd/ops CLI で行い、MVP では Web 管理画面は提供しません（v4 §6 / ADR-0002）。",
  },
];

export default function AboutPage() {
  return (
    <main className="mx-auto min-h-screen w-full max-w-screen-md bg-surface px-4 py-8 sm:px-6 sm:py-10">
      <header className="space-y-2">
        <SectionEyebrow>About</SectionEyebrow>
        <h1 className="text-h1 text-ink">VRC PhotoBook について</h1>
        <p className="text-sm text-ink-medium">
          VRChat コミュニティ向け、ログイン不要で動くフォトブックサービスです。
        </p>
      </header>

      {/* サービスの位置づけ（dashed メタ 1 箇所のみ） */}
      <section
        aria-labelledby="about-overview"
        className="mt-6 rounded-lg border border-divider bg-surface p-5 shadow-sm sm:p-6"
      >
        <h2 id="about-overview" className="text-h2 text-ink">
          サービスの位置づけ
        </h2>
        <p className="mt-3 text-sm text-ink-strong">
          VRChat で撮影された写真を、ログイン不要・スマホファーストでフォトブックとしてまとめ、X で共有することを主目的としたサービスです。
          管理用 URL 方式で、編集・公開停止・削除をログインなしで実現しています。
        </p>
        <dl className="mt-4 grid gap-3 border-t border-dashed border-divider pt-4 text-sm sm:grid-cols-3">
          <div>
            <dt className="text-xs text-ink-medium">運営</dt>
            <dd className="mt-1 font-bold text-ink">個人運営の非公式ファンメイド</dd>
          </div>
          <div>
            <dt className="text-xs text-ink-medium">運営者表示名</dt>
            <dd className="mt-1 font-bold text-ink">ERENOA</dd>
          </div>
          <div>
            <dt className="text-xs text-ink-medium">連絡用 X アカウント</dt>
            <dd className="mt-1">
              <a
                href="https://x.com/Noa_Fortevita"
                rel="noopener noreferrer"
                className="font-num font-bold text-brand-teal underline hover:text-brand-teal-hover"
              >
                @Noa_Fortevita
              </a>
            </dd>
          </div>
        </dl>
        <p className="mt-3 text-xs text-ink-soft">
          MVP 段階のため、機能・仕様は順次追加・変更されます。
        </p>
      </section>

      {/* できること */}
      <section aria-labelledby="about-can" className="mt-10">
        <h2 id="about-can" className="text-h2 text-ink">
          できること
        </h2>
        <ul className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {canDo.map((c) => (
            <li
              key={c.title}
              className="rounded-lg border border-divider bg-surface p-4 shadow-sm"
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
                  <path d={c.iconPath} />
                </svg>
              </span>
              <p className="mt-3 text-sm font-bold text-ink">{c.title}</p>
              <p className="mt-1 text-sm text-ink-medium">{c.body}</p>
            </li>
          ))}
        </ul>
      </section>

      {/* MVP ではできないこと */}
      <section aria-labelledby="about-cannot" className="mt-10">
        <h2 id="about-cannot" className="text-h2 text-ink">
          MVP ではできないこと
        </h2>
        <ul className="mt-4 grid gap-3 sm:grid-cols-2">
          {cannotDo.map((c) => (
            <li
              key={c.title}
              className="rounded-lg border border-divider bg-surface-soft p-4"
            >
              <span
                aria-hidden="true"
                className="grid h-8 w-8 place-items-center rounded-full bg-surface-raised text-ink-soft"
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <circle cx="12" cy="12" r="9" />
                  <path d="M5 12h14" />
                </svg>
              </span>
              <p className="mt-3 text-sm font-bold text-ink">{c.title}</p>
              <p className="mt-1 text-sm text-ink-medium">{c.body}</p>
            </li>
          ))}
        </ul>
      </section>

      {/* ポリシーと窓口 */}
      <section aria-labelledby="about-policy" className="mt-10">
        <h2 id="about-policy" className="text-h2 text-ink">
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
            （取得する情報、利用目的、保持期間、外部サービス利用、未成年保護）
          </li>
          <li>
            <Link href="/help/manage-url" className="text-brand-teal underline hover:text-brand-teal-hover">
              管理 URL について
            </Link>
            （管理用 URL の保存方法、紛失時の対応、メール送信機能の状況）
          </li>
          <li>
            権利侵害・削除希望・不適切表現の報告は、対象フォトブックの「このフォトブックを通報」リンクから運営にお送りください。
          </li>
        </ul>
        <p className="mt-3 text-xs text-ink-soft">
          法的レビューはローンチ後に別途実施し、規約・ポリシーは順次改訂されます。
        </p>
      </section>

      <PublicPageFooter showTrustStrip />
    </main>
  );
}
