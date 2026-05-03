// 公開ページ共通の footer。
//
// 採用元 (m2-design-refresh STOP β-2a):
//   - design/source/project/wf-shared.jsx:64-84 `WFFooter`（trust + links 構造）
//   - design/source/project/wireframe-styles.css:493-501 `.wf-footer`（border-top / spacing）
//
// design 正典の links (`wf-shared.jsx:78`): 「About / Help / Terms / Privacy」 4 link.
// 旧「トップ」link は β-2a で削除（PublicTopBar の logo が `/` 遷移を担うため重複）。
//
// 用途: LP / Terms / Privacy / About / Help / Viewer の脚部に置くリンク群と
// 「非公式ファンメイド」表記を一元化する。
//
// 参照:
//   - 業務知識 v4 §3.6（通報窓口は対象フォトブックの通報リンク）
//   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §STOP β-2a Q-2a-4
//   - docs/plan/m2-design-refresh-plan.md §6 STOP β-2

import type { ReactNode } from "react";
import Link from "next/link";

import { TrustStrip } from "./TrustStrip";

type PublicPageFooterProps = {
  /** 含めるリンク。省略時は design 正典セット（About / Help / Terms / Privacy）を出す。 */
  links?: ReadonlyArray<{ href: string; label: string }>;
  /** true なら footer 上部に TrustStrip を出す。LP / About のみ true。 */
  showTrustStrip?: boolean;
  /** リンクと著作権の間に挿入する任意のスロット（例: Viewer の通報リンク）。 */
  extraSlot?: ReactNode;
};

// design 正典 (`wf-shared.jsx:78`) の順序を維持: About / Help / Terms / Privacy
const defaultLinks: ReadonlyArray<{ href: string; label: string }> = [
  { href: "/about", label: "About" },
  { href: "/help/manage-url", label: "Help" },
  { href: "/terms", label: "Terms" },
  { href: "/privacy", label: "Privacy" },
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
