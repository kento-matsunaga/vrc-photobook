// 公開ページ共通の footer。
//
// 用途: LP / Terms / Privacy / About / 既存 Help / Viewer の脚部に置く
// リンク群と「非公式ファンメイドサービス」表記を一元化する。
//
// design 制約:
//   - design-system（colors / typography / spacing）に揃え、
//     border-divider-soft + text-ink-soft で目立ちすぎない区切りにする
//   - 装飾を持たず、リンクは下線のみ（フッタとして機能優先）
//
// 参照:
//   - design/design-system/colors.md（divider-soft / ink-soft）
//   - design/design-system/typography.md（text-xs / text-sm）
//   - 業務知識 v4 §3.6（通報窓口は対象フォトブックの通報リンク）
//   - docs/plan/post-pr36-submit-report-visibility-decision.md（unlisted も通報対象、本 footer 自体には通報リンクは置かない）

import Link from "next/link";

type PublicPageFooterProps = {
  /** 含めるリンク。省略時は標準セット（top / about / terms / privacy / help）を出す。 */
  links?: ReadonlyArray<{ href: string; label: string }>;
};

const defaultLinks: ReadonlyArray<{ href: string; label: string }> = [
  { href: "/", label: "トップ" },
  { href: "/about", label: "VRC PhotoBook について" },
  { href: "/terms", label: "利用規約" },
  { href: "/privacy", label: "プライバシーポリシー" },
  { href: "/help/manage-url", label: "管理 URL について" },
];

export function PublicPageFooter({ links }: PublicPageFooterProps) {
  const items = links ?? defaultLinks;
  return (
    <footer
      data-testid="public-page-footer"
      className="mt-12 border-t border-divider-soft pt-6 text-center"
    >
      <nav aria-label="サイト内リンク">
        <ul className="flex flex-wrap items-center justify-center gap-x-4 gap-y-2 text-sm text-ink-medium">
          {items.map((item) => (
            <li key={item.href}>
              <Link href={item.href} className="underline hover:text-ink-strong">
                {item.label}
              </Link>
            </li>
          ))}
        </ul>
      </nav>
      <p className="mt-3 text-xs text-ink-soft">
        VRC PhotoBook（非公式ファンメイドサービス）
      </p>
    </footer>
  );
}
