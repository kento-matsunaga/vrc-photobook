// ViewerLayout: 公開 Viewer のページ全体レイアウト。
//
// design 参照: design/mockups/prototype/screens-b.jsx Viewer / pc-screens-b.jsx PCViewer
//
// セキュリティ:
//   - URL に raw token を出さない（route 側で担保済み）
//   - 編集 URL / draft URL / manage URL を表示しない（業務知識 v4）

import Link from "next/link";

import type { PublicPhotobook } from "@/lib/publicPhotobook";
import { PhotoGrid } from "./PhotoGrid";

type Props = {
  photobook: PublicPhotobook;
};

/**
 * Viewer の主レイアウト。
 *
 * MVP は magazine / card / large の layout 差を最小骨格で表現せず、
 * 共通の縦並びで提示する。type / layout に応じた装飾は PR41+ で。
 */
export function ViewerLayout({ photobook }: Props) {
  const coverTitle = photobook.coverTitle ?? photobook.title;
  return (
    <main className="mx-auto max-w-screen-md px-4 py-6 sm:px-6">
      <header className="space-y-3">
        {photobook.cover && (
          <div className="overflow-hidden rounded-lg border border-divider-soft bg-surface-soft">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={photobook.cover.display.url}
              alt=""
              width={photobook.cover.display.width}
              height={photobook.cover.display.height}
              loading="eager"
              decoding="async"
              className="block h-auto w-full"
            />
          </div>
        )}
        <h1 className="text-h1 text-ink">{coverTitle}</h1>
        {photobook.description && (
          <p className="text-body text-ink-strong">{photobook.description}</p>
        )}
        <p className="text-sm text-ink-medium">
          作成者: {photobook.creatorDisplayName}
          {photobook.creatorXId ? (
            <span className="ml-2 font-num">@{photobook.creatorXId}</span>
          ) : null}
        </p>
      </header>

      <div className="mt-8 space-y-10">
        {photobook.pages.map((page, idx) => (
          <PhotoGrid key={idx} page={page} />
        ))}
      </div>

      <footer className="mt-12 border-t border-divider-soft pt-6 text-center">
        <nav aria-label="サイト内リンク">
          <ul className="flex flex-wrap items-center justify-center gap-x-4 gap-y-2 text-sm text-ink-medium">
            <li>
              <Link href="/" className="underline hover:text-ink-strong">
                トップ
              </Link>
            </li>
            <li>
              <Link href="/about" className="underline hover:text-ink-strong">
                VRC PhotoBook について
              </Link>
            </li>
            <li>
              <Link href="/terms" className="underline hover:text-ink-strong">
                利用規約
              </Link>
            </li>
            <li>
              <Link href="/privacy" className="underline hover:text-ink-strong">
                プライバシーポリシー
              </Link>
            </li>
          </ul>
        </nav>
        <p className="mt-3 text-sm">
          <Link
            href={`/p/${photobook.slug}/report`}
            className="underline text-ink-medium hover:text-ink-strong"
            data-testid="viewer-report-link"
          >
            このフォトブックを通報
          </Link>
        </p>
        <p className="mt-3 text-xs text-ink-soft">
          VRC PhotoBook（非公式ファンメイドサービス）
        </p>
      </footer>
    </main>
  );
}
