// PageHero: 1 ページ分の表示単位。
//
// 構造:
//   [ Page NN ラベル ]
//   [ PageMeta バッジ行 (date / world / cast / photographer) ]
//   [ ヒーロー写真 1 枚 (写真の主従の "主") ]
//   [ サブサムネ横並び (写真の主従の "従"、layout で列数が変わる) ]
//   [ ページ caption ]
//   [ PageNote (任意の自由記述メモ) ]
//
// レイアウト 4 種の最小限の差別化:
//   - simple   : ヒーローのみ大きく、サブサムネは縦に詰めず素朴に
//   - card     : ヒーロー + サブサムネ均等寄り（イベント記録向き）
//   - magazine : ヒーロー + サブサムネで主従を強める（誌面風）
//   - large    : ヒーロー全画面ブリード、サブサムネ少なめ
//
// セキュリティ:
//   - photo / page caption は React 自動エスケープ
//   - presigned URL を console / data-attr に出さない

import type { PublicPage, PublicPhoto } from "@/lib/publicPhotobook";

import { LightboxTrigger } from "./LightboxTrigger";
import { PageMeta } from "./PageMeta";
import { PageNote } from "./PageNote";

type Layout = "simple" | "magazine" | "card" | "large";

type Props = {
  page: PublicPage;
  /** 1-based ページ番号 */
  pageNumber: number;
  layout: string;
  /** lightbox を起動するための flat photo index 起点 */
  photoIndexBase: number;
};

export function PageHero({ page, pageNumber, layout, photoIndexBase }: Props) {
  if (page.photos.length === 0) {
    return null;
  }
  const layoutKey = normalizeLayout(layout);
  const [hero, ...subs] = page.photos;
  const heroIndex = photoIndexBase;
  const subSpec = layoutToSubSpec(layoutKey);
  const visibleSubs = subSpec.maxSubs === null ? subs : subs.slice(0, subSpec.maxSubs);

  return (
    <article
      className="space-y-4 scroll-mt-20"
      data-testid="page-hero"
      data-page-number={pageNumber}
      data-layout={layoutKey}
      id={`page-${pageNumber}`}
    >
      <header className="space-y-2">
        <h2 className="flex items-baseline gap-3 text-h2 text-ink">
          <span className="font-num text-2xl font-extrabold leading-none text-teal-600">
            {String(pageNumber).padStart(2, "0")}
          </span>
          <span className="text-xs font-bold tracking-[0.06em] text-ink-medium">
            Page {String(pageNumber).padStart(2, "0")}
          </span>
        </h2>
        <PageMeta meta={page.meta} />
      </header>

      <HeroPhoto photo={hero} layoutKey={layoutKey} index={heroIndex} />

      {visibleSubs.length > 0 && (
        <div
          className={`grid gap-2 sm:gap-3 ${subSpec.gridClass}`}
          data-testid="page-sub-thumbs"
        >
          {visibleSubs.map((photo, idx) => (
            <SubThumb
              key={idx}
              photo={photo}
              index={photoIndexBase + 1 + idx}
            />
          ))}
        </div>
      )}

      {page.caption && (
        <p className="whitespace-pre-line text-sm leading-relaxed text-ink-strong">
          {page.caption}
        </p>
      )}

      <PageNote note={page.meta?.note} />
    </article>
  );
}

function HeroPhoto({
  photo,
  layoutKey,
  index,
}: {
  photo: PublicPhoto;
  layoutKey: Layout;
  index: number;
}) {
  const isLarge = layoutKey === "large";
  return (
    <figure
      className={`overflow-hidden rounded-lg border border-divider-soft bg-surface-soft ${
        isLarge ? "shadow-md" : "shadow-sm"
      }`}
    >
      <LightboxTrigger photoIndex={index} className="block w-full">
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
      </LightboxTrigger>
      {photo.caption && (
        <figcaption className="px-4 py-2 text-xs leading-relaxed text-ink-medium">
          {photo.caption}
        </figcaption>
      )}
    </figure>
  );
}

function SubThumb({ photo, index }: { photo: PublicPhoto; index: number }) {
  return (
    <figure className="overflow-hidden rounded-md border border-divider-soft bg-surface-soft">
      <LightboxTrigger photoIndex={index} className="block w-full">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={photo.variants.thumbnail.url}
          alt=""
          width={photo.variants.thumbnail.width}
          height={photo.variants.thumbnail.height}
          loading="lazy"
          decoding="async"
          className="block h-auto w-full"
        />
      </LightboxTrigger>
    </figure>
  );
}

function normalizeLayout(layout: string): Layout {
  if (
    layout === "simple" ||
    layout === "card" ||
    layout === "magazine" ||
    layout === "large"
  ) {
    return layout;
  }
  return "simple";
}

function layoutToSubSpec(
  layout: Layout,
): { gridClass: string; maxSubs: number | null } {
  switch (layout) {
    case "large":
      // ヒーロー主役、サブサムネ少なく
      return { gridClass: "grid-cols-3 sm:grid-cols-4", maxSubs: 4 };
    case "magazine":
      // 主従を強める誌面風
      return { gridClass: "grid-cols-3 sm:grid-cols-4 lg:grid-cols-5", maxSubs: null };
    case "card":
      // タイル型均等寄り
      return { gridClass: "grid-cols-3 sm:grid-cols-4 lg:grid-cols-4", maxSubs: null };
    case "simple":
    default:
      // 軽量、控えめ
      return { gridClass: "grid-cols-2 sm:grid-cols-3", maxSubs: null };
  }
}
