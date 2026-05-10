// VRC PhotoBook について（m2-design-refresh STOP β-2b-2）。
//
// 採用元 (design 正典):
//   - design/source/project/wf-screens-c.jsx:179-227 `WFAbout_M`
//   - design/source/project/wf-screens-c.jsx:228-273 `WFAbout_PC`
//   - design/source/project/wf-shared.jsx:29-48 `WFBrowser` (PC header → PublicTopBar)
//   - design/source/project/wireframe-styles.css:165-175 `.wf-box`
//   - design/source/project/wireframe-styles.css:228-253 `.wf-btn` (policy buttons)
//   - design/source/project/wireframe-styles.css:337-349 `.wf-section-title`
//   - design/source/project/wireframe-styles.css:565-573 grid / row utilities
//
// design 正典構造:
//   1. PublicTopBar
//   2. eyebrow + h1「VRC PhotoBook について」 (Mobile <br/> 改行)
//   3. サービスの位置づけ wf-box (本文 + dl meta + MVP 注記)
//   4. できること (6 件): ✓ 円 icon row × 6 (M 縦 / PC wf-grid-2 で並列)
//   5. MVP ではできないこと (4 件): × 円 icon row × 4
//   6. ポリシーと窓口 wf-box: 3 button block (M 縦 / PC wf-grid-3) + 通報窓口別段落
//   7. PublicPageFooter (ε-fix: trust strip は非表示。LP / About いずれも実機 smoke で
//      「LP / About では情報過多」というフィードバックがあったため非表示化。
//      TrustStrip コンポーネント自体は単体テストのため残す)
//
// 「足りないものは足す」(plan §0.1):
//   - design は placeholder line のみ。production truth (canDo 6 / cannotDo 4 本文) を維持
//   - dl meta (運営 / 運営者表示名 ERENOA / @Noa_Fortevita) は production truth として残す
//   - 通報窓口の説明 + 法的レビュー注記は別段落で補足 (削減なし)
//   - canDo / cannotDo の旧 SVG iconPath は廃止し、design の ✓ / × 円 outline に統一
//
// 業務知識:
//   - v4 §1 / §2.6 / §3 / §6 / §7.6
//   - ADR-0002 (cmd/ops CLI 運用)
//   - ADR-0006 (Email Provider 再選定中)
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow が付与される
//   - 動的データ (token / Cookie / Secret / 任意 ID) は出さない
//
// 設計参照:
//   - docs/plan/m2-design-refresh-stop-beta-2b-plan.md §2
//   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §2.3.1

import type { Metadata } from "next";
import Link from "next/link";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";

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
    body: "公開直後に表示される管理用 URL を保存しておけば、いつでも編集・公開停止・削除ができます（v4 §3.4 / §3.5）。",
  },
  {
    title: "公開範囲の選択",
    body: "公開（誰でも閲覧可）/ 限定公開（URL を知っている人のみ、MVP の既定）/ 非公開（管理 URL 保持者のみ）から選べます（v4 §2.6）。",
  },
  {
    title: "X 共有用 OGP の自動生成",
    body: "公開時に OGP 画像を自動生成し、X タイムラインで見やすい形で共有できます（v4 §3.8）。",
  },
  {
    title: "通報窓口",
    body: "公開フォトブックの閲覧画面から、権利侵害・センシティブ・未成年関連の懸念を通報できます。運営は通報を手動で確認・対応します（v4 §3.6）。",
  },
  {
    title: "荒らし抑止のレート制限",
    body: "通報・アップロード・公開操作には、IP ハッシュベースの利用制限を設けています。検証は Cloudflare Turnstile を併用します（v4 §3.7）。",
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

const policyLinks: ReadonlyArray<{
  href: string;
  label: string;
  testidSlug: string;
}> = [
  { href: "/terms", label: "利用規約", testidSlug: "terms" },
  { href: "/privacy", label: "プライバシーポリシー", testidSlug: "privacy" },
  { href: "/help/manage-url", label: "管理 URL について", testidSlug: "help-manage-url" },
];

// design `wireframe-styles.css:337-349` `.wf-section-title` (12px / font-bold / tracking-[0.04em] +
// ::before 4×14 teal bar)
function SectionTitle({ children, id }: { children: string; id?: string }) {
  return (
    <h2
      id={id}
      className="mb-3 flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong sm:mb-4"
    >
      <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
      {children}
    </h2>
  );
}

// design `wf-screens-c.jsx:196` ✓ icon (18×18 / border 1.5 ink / round / center / font-bold)
// と `:207` × icon (font-size 11 with 同寸法) を統合した circular outline icon。
function CircleIcon({ kind }: { kind: "check" | "cross" }) {
  const sizeCls = kind === "check" ? "text-[10px]" : "text-[11px]";
  return (
    <span
      aria-hidden="true"
      className={`mt-0.5 grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full border-[1.5px] border-ink font-bold leading-none text-ink ${sizeCls}`}
    >
      {kind === "check" ? "✓" : "×"}
    </span>
  );
}

export default function AboutPage() {
  return (
    <>
      <PublicTopBar />
      <main className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9">
        <header className="space-y-2">
          <SectionEyebrow>About</SectionEyebrow>
          {/* design `wf-screens-c.jsx:184` Mobile h1 「VRC PhotoBook<br/>について」, PC は 1 行 */}
          <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">
            VRC PhotoBook
            <span className="hidden sm:inline"> </span>
            <br className="sm:hidden" />
            について
          </h1>
          <p className="text-sm text-ink-medium">
            VRChat コミュニティ向け、ログイン不要で動くフォトブックサービスです。
          </p>
        </header>

        {/* サービスの位置づけ wf-box (`wf-screens-c.jsx:235-238`) */}
        <section
          data-testid="about-positioning"
          aria-labelledby="about-positioning-heading"
          className="mt-7 rounded-lg border border-divider-soft bg-surface p-5 shadow-sm sm:p-6"
        >
          <SectionTitle id="about-positioning-heading">サービスの位置づけ</SectionTitle>
          <p className="text-sm leading-[1.75] text-ink-strong">
            VRChat で撮影された写真を、ログイン不要・スマホファーストでフォトブックとしてまとめ、X で共有することを主目的としたサービスです。
            管理用 URL 方式で、編集・公開停止・削除をログインなしで実現しています。
          </p>
          {/* dl meta は production truth として維持 (運営 / 運営者表示名 / 連絡用 X) */}
          <dl
            data-testid="about-positioning-meta"
            className="mt-4 grid gap-3 border-t border-dashed border-divider pt-4 text-sm sm:grid-cols-3"
          >
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
                  className="font-num font-bold text-teal-600 underline hover:text-teal-700"
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

        {/* canDo 6 / cannotDo 4 (`wf-screens-c.jsx:240-258` PC wf-grid-2 / Mobile 縦 stack) */}
        <div className="mt-7 grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4">
          <section
            data-testid="about-can"
            aria-labelledby="about-can-heading"
            className="rounded-lg border border-divider-soft bg-surface p-5 shadow-sm sm:p-6"
          >
            <SectionTitle id="about-can-heading">できること (6 件)</SectionTitle>
            <ul data-testid="about-can-list" className="divide-y divide-divider-soft">
              {canDo.map((c, i) => (
                <li
                  key={c.title}
                  data-testid={`about-can-item-${i + 1}`}
                  className="flex items-start gap-3 py-3 first:pt-0 last:pb-0"
                >
                  <CircleIcon kind="check" />
                  <div className="flex-1">
                    <p className="text-sm font-bold text-ink">{c.title}</p>
                    <p className="mt-1 text-xs leading-[1.7] text-ink-medium">
                      {c.body}
                    </p>
                  </div>
                </li>
              ))}
            </ul>
          </section>
          <section
            data-testid="about-cannot"
            aria-labelledby="about-cannot-heading"
            className="rounded-lg border border-divider-soft bg-surface p-5 shadow-sm sm:p-6"
          >
            <SectionTitle id="about-cannot-heading">
              MVP ではできないこと (4 件)
            </SectionTitle>
            <ul data-testid="about-cannot-list" className="divide-y divide-divider-soft">
              {cannotDo.map((c, i) => (
                <li
                  key={c.title}
                  data-testid={`about-cannot-item-${i + 1}`}
                  className="flex items-start gap-3 py-3 first:pt-0 last:pb-0"
                >
                  <CircleIcon kind="cross" />
                  <div className="flex-1">
                    <p className="text-sm font-bold text-ink">{c.title}</p>
                    <p className="mt-1 text-xs leading-[1.7] text-ink-medium">
                      {c.body}
                    </p>
                  </div>
                </li>
              ))}
            </ul>
          </section>
        </div>

        {/* ポリシーと窓口 wf-box + 3 button block (`wf-screens-c.jsx:261-268` PC wf-grid-3) */}
        <section
          data-testid="about-policy"
          aria-labelledby="about-policy-heading"
          className="mt-7 rounded-lg border border-divider-soft bg-surface p-5 shadow-sm sm:p-6"
        >
          <SectionTitle id="about-policy-heading">ポリシーと窓口</SectionTitle>
          <div
            data-testid="about-policy-list"
            className="grid grid-cols-1 gap-2 sm:grid-cols-3 sm:gap-3"
          >
            {policyLinks.map((p) => (
              <Link
                key={p.href}
                href={p.href}
                data-testid={`about-policy-link-${p.testidSlug}`}
                className="inline-flex h-12 w-full items-center justify-center rounded-md border border-divider bg-surface px-4 text-sm font-semibold text-ink-strong shadow-sm transition-colors hover:border-teal-300 hover:text-teal-700"
              >
                {p.label}
              </Link>
            ))}
          </div>
          {/* design 3 button block には説明が無いが、production truth として旧 policy bullet の
              link 概要 (権利・一時非表示・削除・免責 / 取得情報・保持期間・外部サービス /
              保存方法・紛失時) を 1 行 caption で保持 (削減なし) */}
          <p
            data-testid="about-policy-summary"
            className="mt-3 text-xs leading-[1.7] text-ink-soft"
          >
            利用規約は投稿画像の権利・運営による一時非表示・削除・免責、プライバシーポリシーは取得情報・利用目的・保持期間・外部サービス利用、管理 URL についてには保存方法・紛失時の対応・メール送信機能の状況をそれぞれ記載しています。
          </p>
          {/* 通報窓口は別段落で補足 (design `wf-screens-c.jsx:219` `通報は Viewer から` の anno を
              production 文言で具体化、削減なし) */}
          <p
            data-testid="about-report-note"
            className="mt-3 text-xs leading-[1.7] text-ink-medium"
          >
            権利侵害・削除希望・不適切表現の報告は、対象フォトブックページの「このフォトブックを通報」リンクから運営にお送りください。運営は通報を正式な窓口として扱います。
          </p>
          <p className="mt-2 text-xs text-ink-soft">
            法的レビューはローンチ後に別途実施し、規約・ポリシーは順次改訂されます。
          </p>
        </section>

        <PublicPageFooter />
      </main>
    </>
  );
}
