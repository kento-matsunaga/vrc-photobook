// 公開ページ共通の TopBar (header)。
//
// 採用元 (m2-design-refresh STOP β-1):
//   - design/source/project/wf-shared.jsx:29-48 `WFBrowser` (header 構造)
//   - design/source/project/wireframe-styles.css:137-162 `.wf-pc-header` / `.wf-pc-logo` /
//     `.wf-pc-nav` (PC layout / logo gradient / nav style)
//
// design 正典の nav 内容 (`wf-shared.jsx:36-41`):
//   - logo (左): teal gradient 矩形 + `VRC PhotoBook`
//   - nav (右): 「作例 / 使い方 / よくある質問」3 link + primary CTA「無料で作る」
//
// β-1 段階では本 component は **新規作成のみ**で、既存 page (LP / About / Help / Terms /
// Privacy / Manage / Viewer / Edit / Prepare / Create / Report) の inline header は
// β-2 以降で順次置換していく。
//
// 「design はそのまま、足りないものは足す」(plan §0.1) に基づき:
//   - design 文言 (作例 / 使い方 / よくある質問 / 無料で作る) を main label として維持
//   - production 必須の補助として:
//     - `data-testid="public-topbar"` を test hook として追加
//     - `nav aria-label="主要ナビゲーション"` を accessibility として追加
//     - 「作例」 link は MVP 範囲外のため anchor `#examples` で LP 内 anchor に運用
//       （/about の「できること」section に LP 内アンカーで近接）
//     - 「使い方」「よくある質問」は既存 /about / /help/manage-url にリンク
//     - 「無料で作る」CTA は /create に遷移
//
// 設計参照:
//   - docs/plan/m2-design-refresh-plan.md §5.1 design source 正典マップ / §6 STOP β-1
//   - docs/plan/m2-design-refresh-plan.md §10.2 Q-A 等の追加 UI 方針
//
// 既存影響:
//   - 本 component は新規追加のため、既存 page / 既存 test を破壊しない

import Link from "next/link";

type PublicTopBarProps = {
  /** primary CTA「無料で作る」を強調表示するか。LP では true、他 page では false（CTA は inline で重複しないため）。 */
  showPrimaryCta?: boolean;
};

const NAV_LINKS: ReadonlyArray<{ href: string; label: string }> = [
  // design `wf-shared.jsx:37-39` の 3 link を維持。「作例」はサンプル LP 内アンカーで運用 (β-2 で実装)。
  { href: "/#examples", label: "作例" },
  { href: "/about", label: "使い方" },
  { href: "/help/manage-url", label: "よくある質問" },
];

export function PublicTopBar({ showPrimaryCta = true }: PublicTopBarProps) {
  return (
    <header
      data-testid="public-topbar"
      className="sticky top-0 z-10 border-b border-divider-soft bg-surface px-6 py-3.5 sm:px-9"
    >
      <div className="mx-auto flex max-w-screen-xl items-center justify-between">
        <Link
          href="/"
          className="flex items-center gap-2.5 text-base font-extrabold text-ink"
          aria-label="VRC PhotoBook トップへ"
        >
          {/* design `wireframe-styles.css:149-157` `.wf-pc-logo::before` の teal gradient 矩形 */}
          <span
            aria-hidden="true"
            className="block h-[22px] w-[26px] rounded-[4px] bg-gradient-to-br from-teal-300 to-teal-500"
            style={{ boxShadow: "inset -8px 0 0 rgba(255,255,255,0.35)" }}
          />
          <span>VRC PhotoBook</span>
        </Link>
        <nav
          aria-label="主要ナビゲーション"
          className="flex items-center gap-4 text-sm text-ink-strong sm:gap-6"
        >
          {NAV_LINKS.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className="hidden hover:text-teal-600 sm:inline"
            >
              {link.label}
            </Link>
          ))}
          {showPrimaryCta ? (
            <Link
              href="/create"
              className="inline-flex h-8 items-center justify-center rounded-md bg-brand-teal px-3 text-xs font-semibold text-white shadow-sm transition-colors hover:bg-brand-teal-hover"
              data-testid="public-topbar-cta"
            >
              無料で作る
            </Link>
          ) : null}
        </nav>
      </div>
    </header>
  );
}
