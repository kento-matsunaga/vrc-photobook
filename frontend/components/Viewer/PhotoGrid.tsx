// PhotoGrid: 公開 Viewer のページ内 photo 一覧 / 単一表示。
//
// design 参照: design/mockups/prototype/screens-b.jsx Viewer
//
// セキュリティ:
//   - storage_key 完全値を表示しない（本コンポーネントでは presigned URL のみを扱う）
//   - 短命 presigned URL は HTML 内に出るが、外部 console / log への出力は行わない

import type { PublicPage } from "@/lib/publicPhotobook";

type Props = {
  page: PublicPage;
};

/**
 * 1 ページ分の photo を縦に並べる最小骨格。
 *
 * MVP では magazine / card / large / simple の layout 差をすべてここで表現せず、
 * デフォルトの縦並びで提示する。layout 別 UI は PR27（編集 UI 本格化）で扱う。
 */
export function PhotoGrid({ page }: Props) {
  if (page.photos.length === 0) {
    return null;
  }
  return (
    <section className="space-y-4" data-testid="photo-grid">
      {page.caption && (
        <p className="text-body text-ink-medium">{page.caption}</p>
      )}
      <div className="space-y-4">
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
              <figcaption className="px-4 py-2 text-sm text-ink-medium">
                {photo.caption}
              </figcaption>
            )}
          </figure>
        ))}
      </div>
    </section>
  );
}
