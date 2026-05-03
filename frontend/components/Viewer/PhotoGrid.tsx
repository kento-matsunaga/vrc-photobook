// PhotoGrid: 公開 Viewer のページ内 photo 一覧 / 単一表示。
//
// セキュリティ:
//   - storage_key 完全値を表示しない（本コンポーネントでは presigned URL のみを扱う）
//   - 短命 presigned URL は HTML 内に出るが、外部 console / log への出力は行わない
//
// m2-design-refresh STOP β-5 (本 commit、visual のみ):
//   - design `wf-screens-c.jsx:18-26` (M wf-grid-2) / `:44-53` (PC wf-grid-3) の
//     photo grid 視覚整合
//   - section title「Page NN」(`wireframe-styles.css:337-349` `.wf-section-title` + teal bar)
//   - page.caption / page.photos / photo.caption / data-testid="photo-grid" は **触らない**
//   - photo.variants.display.url の <img> 表示 logic は不変

import type { PublicPage } from "@/lib/publicPhotobook";

type Props = {
  page: PublicPage;
  /** 1-based page number for section title「Page NN」(design `wf-screens-c.jsx:19` / `:46`) */
  pageNumber: number;
};

/**
 * 1 ページ分の photo を grid で並べる。
 *
 * MVP では magazine / card / large / simple の layout 差をすべてここで表現せず、
 * デフォルトの grid (Mobile 2 col / PC 3 col) で提示する。layout 別 UI は PR27（編集 UI 本格化）で扱う。
 */
export function PhotoGrid({ page, pageNumber }: Props) {
  if (page.photos.length === 0) {
    return null;
  }
  return (
    <section className="space-y-3" data-testid="photo-grid">
      {/* design `wf-screens-c.jsx:46` `wf-section-title` で「Page NN」表示 (1-based zero-pad) */}
      <h2 className="flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
        <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
        Page {String(pageNumber).padStart(2, "0")}
      </h2>
      <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-3">
        {page.photos.map((photo, idx) => (
          <figure
            key={idx}
            className="overflow-hidden rounded-lg border border-divider-soft bg-surface-soft"
          >
            {/* MVP は <img> を使う。Next/Image は presigned URL 配信と相性が悪く、
                Workers ランタイムで loader 設定が増えるため後続 PR で評価する。 */}
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={photo.variants.display.url}
              alt=""
              width={photo.variants.display.width}
              height={photo.variants.display.height}
              loading="lazy"
              decoding="async"
              className="block h-auto w-full"
            />
            {photo.caption && (
              <figcaption className="px-3 py-2 text-[11px] leading-[1.5] text-ink-medium">
                {photo.caption}
              </figcaption>
            )}
          </figure>
        ))}
      </div>
      {page.caption && (
        <p className="text-xs leading-[1.6] text-ink-medium">{page.caption}</p>
      )}
    </section>
  );
}
