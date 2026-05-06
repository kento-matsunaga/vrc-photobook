// PageNavSidebar: PC 左サイドバー（ページジャンプ用ナビ）。
//
// デザイン参照:
//   - design 最終調整版 §2 PC 左カラム「ページジャンプ用ナビゲーション (縦スクロールと連動)」
//
// 実装方針 (MVP):
//   - 縦サムネイルの list を出し、各 li は `<a href="#page-N">` で anchor jump
//   - 「現在見ているページのハイライト」（scroll spy）は後続 PR で追加
//   - 軽量に Server Component で実装し、Client Component / state は持たない
//
// セキュリティ:
//   - サムネ url は presigned。data-attr / console には出さない

import type { PublicPage } from "@/lib/publicPhotobook";

type Props = {
  pages: PublicPage[];
};

export function PageNavSidebar({ pages }: Props) {
  if (pages.length === 0) return null;
  return (
    <nav
      aria-label="ページナビゲーション"
      className="sticky top-20 max-h-[calc(100vh-6rem)] overflow-y-auto"
      data-testid="page-nav-sidebar"
    >
      <ol className="space-y-2">
        {pages.map((page, idx) => {
          const num = idx + 1;
          const firstPhoto = page.photos[0];
          return (
            <li key={idx}>
              <a
                href={`#page-${num}`}
                className="group flex items-center gap-2 rounded-md p-1 transition-colors hover:bg-surface-soft"
              >
                <span className="font-num w-6 shrink-0 text-center text-xs font-bold text-ink-medium group-hover:text-teal-700">
                  {String(num).padStart(2, "0")}
                </span>
                <div className="aspect-square w-12 shrink-0 overflow-hidden rounded border border-divider-soft bg-surface-soft">
                  {firstPhoto ? (
                    /* eslint-disable-next-line @next/next/no-img-element */
                    <img
                      src={firstPhoto.variants.thumbnail.url}
                      alt=""
                      width={firstPhoto.variants.thumbnail.width}
                      height={firstPhoto.variants.thumbnail.height}
                      loading="lazy"
                      decoding="async"
                      className="block h-full w-full object-cover"
                    />
                  ) : null}
                </div>
              </a>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
