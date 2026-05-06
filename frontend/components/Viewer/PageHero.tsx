// PageHero: 1 ページ分の表示単位 (ヘッダ + ヒーロー写真 + サブサムネ + caption + meta + note)。
//
// 採用元: TESTImage 完成イメージ「ページ 01〜」のメイン構成
//
// 設計判断 (v2):
//   - レイアウト 4 種差別化軸 = aspect ratio + caption 位置 (cbabbe6 は grid 密度軸、別軸)
//     * simple   : aspect-[4/3] hero、caption 下、sub max 4 横並び
//     * card     : aspect-[4/3] hero、rounded shadow、caption padded box
//     * magazine : aspect-[16/9] hero (cinematic)、caption 横 (PC) / 下 (Mobile)、sub max 5
//     * large    : aspect-[3/2] hero、テキスト大きめ、sub max 3
//   - 落とし穴 #4 対応: photoIndexBase prop を受け取り、サムネ→Lightbox flat index に
//     pageOffset + i を加算
//   - Server Component。LightboxTrigger (Client) を import するが、Server 側からは
//     ただの JSX として埋めるだけで OK (Next.js 15 App Router の許容範囲)
//   - hero は最初の photo (index 0)、subs は index 1〜
//
// セキュリティ:
//   - presigned URL は <img src> 渡しのみ
//   - photoIndex は flat 配列上の index (0 始まり)、内部 image_id は出さない

import type { PublicPage } from "@/lib/publicPhotobook";
import { LightboxTrigger } from "@/components/Viewer/LightboxTrigger";
import { PageMeta } from "@/components/Viewer/PageMeta";
import { PageNote } from "@/components/Viewer/PageNote";

type Layout = "simple" | "card" | "magazine" | "large" | string;

type Props = {
  page: PublicPage;
  pageNumber: number;
  layout: Layout;
  /** ViewerLayout が pages.flatMap で計算した、本 page 先頭 photo の flat index */
  photoIndexBase: number;
};

type LayoutSpec = {
  heroAspect: string;
  heroPadding: string;
  captionPosition: "below" | "side";
  subMax: number;
  subAspect: string;
  rounded: string;
  shadow: string;
};

function specFor(layout: Layout): LayoutSpec {
  switch (layout) {
    case "card":
      return {
        heroAspect: "aspect-[4/3]",
        heroPadding: "p-2 sm:p-3",
        captionPosition: "below",
        subMax: 4,
        subAspect: "aspect-square",
        rounded: "rounded-xl",
        shadow: "shadow-md",
      };
    case "magazine":
      return {
        heroAspect: "aspect-[16/9]",
        heroPadding: "p-0",
        captionPosition: "side",
        subMax: 5,
        subAspect: "aspect-[3/4]",
        rounded: "rounded-md",
        shadow: "shadow-sm",
      };
    case "large":
      return {
        heroAspect: "aspect-[3/2]",
        heroPadding: "p-0",
        captionPosition: "below",
        subMax: 3,
        subAspect: "aspect-[3/4]",
        rounded: "rounded-md",
        shadow: "shadow-md",
      };
    case "simple":
    default:
      return {
        heroAspect: "aspect-[4/3]",
        heroPadding: "p-0",
        captionPosition: "below",
        subMax: 4,
        subAspect: "aspect-square",
        rounded: "rounded-md",
        shadow: "shadow-sm",
      };
  }
}

export function PageHero({ page, pageNumber, layout, photoIndexBase }: Props) {
  if (page.photos.length === 0) {
    // 空ページは header + caption のみ (画像 0 は通常ありえないが防御)
    return (
      <article
        id={`page-${pageNumber}`}
        data-testid={`viewer-page-${pageNumber}`}
        className="scroll-mt-20"
      >
        <PageHeader pageNumber={pageNumber} caption={page.caption} meta={page.meta} />
      </article>
    );
  }

  const spec = specFor(layout);
  const hero = page.photos[0];
  const subs = page.photos.slice(1, 1 + spec.subMax);
  const overflow = Math.max(0, page.photos.length - 1 - spec.subMax);

  const isSideCaption = spec.captionPosition === "side";

  return (
    <article
      id={`page-${pageNumber}`}
      data-testid={`viewer-page-${pageNumber}`}
      data-page-layout={layout}
      className="scroll-mt-20 space-y-4 sm:space-y-5"
    >
      <PageHeader pageNumber={pageNumber} caption={page.caption} meta={page.meta} />

      {/* hero + (magazine のみ) 横 caption */}
      <div
        className={
          isSideCaption
            ? "grid grid-cols-1 gap-4 sm:grid-cols-[1.6fr_1fr] sm:gap-6"
            : ""
        }
      >
        <LightboxTrigger
          photoIndex={photoIndexBase}
          ariaLabel={`Page ${pageNumber} の写真 1 を全画面で開く`}
          className={`group relative block w-full overflow-hidden bg-surface-soft ${spec.heroAspect} ${spec.heroPadding} ${spec.rounded} ${spec.shadow}`}
        >
          <img
            src={hero.variants.display.url}
            alt={hero.caption ?? ""}
            width={hero.variants.display.width}
            height={hero.variants.display.height}
            loading="lazy"
            decoding="async"
            className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.02]"
          />
        </LightboxTrigger>

        {isSideCaption && hero.caption ? (
          <div className="flex flex-col justify-center">
            <p className="text-sm leading-relaxed text-ink-strong sm:text-base">
              {hero.caption}
            </p>
          </div>
        ) : null}
      </div>

      {/* hero caption (below 配置) */}
      {!isSideCaption && hero.caption ? (
        <p
          className={
            layout === "large"
              ? "text-base leading-relaxed text-ink-strong sm:text-lg"
              : "text-sm leading-relaxed text-ink-strong sm:text-[15px]"
          }
        >
          {hero.caption}
        </p>
      ) : null}

      {/* sub thumbnails strip */}
      {subs.length > 0 ? (
        <ul
          data-testid={`viewer-page-${pageNumber}-subs`}
          className={`grid gap-2 sm:gap-3 ${gridColsFor(spec.subMax)}`}
        >
          {subs.map((photo, i) => {
            const flatIndex = photoIndexBase + 1 + i;
            return (
              <li key={i} className={spec.subAspect}>
                <LightboxTrigger
                  photoIndex={flatIndex}
                  ariaLabel={`Page ${pageNumber} の写真 ${i + 2} を全画面で開く`}
                  className="group relative block h-full w-full overflow-hidden rounded-md bg-surface-soft"
                >
                  <img
                    src={photo.variants.thumbnail.url}
                    alt={photo.caption ?? ""}
                    width={photo.variants.thumbnail.width}
                    height={photo.variants.thumbnail.height}
                    loading="lazy"
                    decoding="async"
                    className="h-full w-full object-cover transition-opacity duration-200 group-hover:opacity-90"
                  />
                </LightboxTrigger>
              </li>
            );
          })}
          {overflow > 0 ? (
            <li
              className={`${spec.subAspect} flex items-center justify-center rounded-md border border-dashed border-divider bg-surface-soft text-xs text-ink-medium`}
            >
              +{overflow}
            </li>
          ) : null}
        </ul>
      ) : null}

      <PageNote note={page.meta?.note} />
    </article>
  );
}

function gridColsFor(max: number): string {
  // Mobile は 4col / PC は max まで広げる
  switch (max) {
    case 5:
      return "grid-cols-4 sm:grid-cols-5";
    case 4:
      return "grid-cols-4";
    case 3:
      return "grid-cols-3";
    default:
      return "grid-cols-4";
  }
}

function PageHeader({
  pageNumber,
  caption,
  meta,
}: {
  pageNumber: number;
  caption?: string;
  meta?: PublicPage["meta"];
}) {
  const padded = String(pageNumber).padStart(2, "0");
  return (
    <header className="space-y-2">
      <div className="flex items-baseline gap-3">
        <span className="font-num text-2xl font-bold leading-none text-teal-600 sm:text-3xl">
          {padded}
        </span>
        <span className="text-[11px] font-bold tracking-wider text-ink-medium sm:text-xs">
          PAGE {padded}
        </span>
      </div>
      {caption ? (
        <h2 className="font-serif text-lg font-bold leading-tight text-ink sm:text-xl">
          {caption}
        </h2>
      ) : null}
      <PageMeta meta={meta} />
    </header>
  );
}
