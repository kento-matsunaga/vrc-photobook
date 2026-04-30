// 公開ページ共通の footer。
//
// 用途: LP / Terms / Privacy / About / 既存 Help / Viewer の脚部に置く
// リンク群と「非公式ファンメイドサービス」表記を一元化する。
//
// design 制約:
//   - design-system（colors / typography / spacing）に揃え、
//     border-divider-soft + text-ink-soft で目立ちすぎない区切りにする
//   - 装飾を持たず、リンクは下線のみ（フッタとして機能優先）
//   - LP / About では showTrustStrip で 4 cell trust strip を closing element として出す
//
// 参照:
//   - design/design-system/colors.md（divider-soft / ink-soft）
//   - design/design-system/typography.md（text-xs / text-sm）
//   - design/mockups/prototype/screens-a.jsx の `.trust-strip` / pc-shared.jsx PCTrust
//   - 業務知識 v4 §3.6（通報窓口は対象フォトブックの通報リンク）
//   - harness/work-logs/2026-05-01_pr37-design-rebuild-plan.md §3.5 / §8

import type { ReactNode } from "react";
import Link from "next/link";

import { TrustStrip } from "./TrustStrip";

type PublicPageFooterProps = {
  /** 含めるリンク。省略時は標準セット（top / about / terms / privacy / help）を出す。 */
  links?: ReadonlyArray<{ href: string; label: string }>;
  /** true なら footer 上部に TrustStrip を出す。LP / About のみ true。 */
  showTrustStrip?: boolean;
  /** リンクと著作権の間に挿入する任意のスロット（例: Viewer の通報リンク）。 */
  extraSlot?: ReactNode;
};

const defaultLinks: ReadonlyArray<{ href: string; label: string }> = [
  { href: "/", label: "トップ" },
  { href: "/about", label: "VRC PhotoBook について" },
  { href: "/terms", label: "利用規約" },
  { href: "/privacy", label: "プライバシーポリシー" },
  { href: "/help/manage-url", label: "管理 URL について" },
];

export function PublicPageFooter({
  links,
  showTrustStrip = false,
  extraSlot,
}: PublicPageFooterProps) {
  const items = links ?? defaultLinks;
  return (
    <footer
      data-testid="public-page-footer"
      className="mt-12 border-t border-divider-soft pt-6 text-center"
    >
      {showTrustStrip ? <TrustStrip /> : null}
      <nav aria-label="サイト内リンク" className={showTrustStrip ? "mt-6" : ""}>
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
      {extraSlot ? <div className="mt-3">{extraSlot}</div> : null}
      <p className="mt-3 text-xs text-ink-soft">
        VRC PhotoBook（非公式ファンメイドサービス）
      </p>
    </footer>
  );
}
