// PageHero.tsx の SSR レンダリング検証 (table 駆動)。
//
// 観点:
//   - layout 4 種で aspect / sub max / caption 配置が変わる
//   - meta あり / なしで PageMeta 描画分岐
//   - photoIndexBase が flat index 計算に使われる
//   - 0 photos / 1 photo / 多数 photos の境界
//
// 注意:
//   - LightboxTrigger ("use client") を含むが、SSR では initial render 時に button タグとして
//     文字列化されるため、aria-label / data-testid (subs) で構造確認可能

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PageHero } from "@/components/Viewer/PageHero";
import { ViewerInteractionProvider } from "@/components/Viewer/ViewerInteractionProvider";
import type { PublicPage, PublicPhoto } from "@/lib/publicPhotobook";

function dummyPhoto(slug: string, caption?: string): PublicPhoto {
  return {
    caption,
    variants: {
      display: {
        url: `https://images.example.invalid/${slug}.jpg`,
        width: 1600,
        height: 1066,
        expiresAt: "2099-12-31T23:59:59Z",
      },
      thumbnail: {
        url: `https://images.example.invalid/${slug}.thumb.jpg`,
        width: 400,
        height: 266,
        expiresAt: "2099-12-31T23:59:59Z",
      },
    },
  };
}

function pageWith(photoCount: number, opts: { caption?: string; meta?: PublicPage["meta"] } = {}): PublicPage {
  const photos: PublicPhoto[] = [];
  for (let i = 0; i < photoCount; i++) {
    photos.push(dummyPhoto(`p-${i}`, `caption-${i}`));
  }
  return {
    caption: opts.caption,
    photos,
    meta: opts.meta,
  };
}

function renderInProvider(node: React.ReactNode, flatPhotos: PublicPhoto[]): string {
  return renderToStaticMarkup(
    <ViewerInteractionProvider flatPhotos={flatPhotos}>{node}</ViewerInteractionProvider>,
  );
}

describe("PageHero", () => {
  type Case = {
    name: string;
    description: string;
    page: PublicPage;
    pageNumber: number;
    layout: string;
    photoIndexBase: number;
    expectInHTML: string[];
    expectNotInHTML?: string[];
  };

  const cases: Case[] = [
    {
      name: "正常_simple_layout_with_meta_and_4_photos",
      description: "Given simple + 4 photos, When render, Then hero + sub 3 thumbs + meta + page header",
      page: pageWith(4, {
        caption: "ページ caption",
        meta: { eventDate: "2026-04-29", world: "Test World" },
      }),
      pageNumber: 1,
      layout: "simple",
      photoIndexBase: 0,
      expectInHTML: [
        'id="page-1"',
        'data-testid="viewer-page-1"',
        'data-page-layout="simple"',
        "PAGE 01",
        "ページ caption",
        '<ul data-testid="page-meta"',
        "2026.04.29",
        "Test World",
        // hero (index 0) + subs (1, 2, 3)
        'aria-label="Page 1 の写真 1 を全画面で開く"',
        'aria-label="Page 1 の写真 2 を全画面で開く"',
        'aria-label="Page 1 の写真 3 を全画面で開く"',
        'aria-label="Page 1 の写真 4 を全画面で開く"',
      ],
    },
    {
      name: "正常_card_layout",
      description: "Given card layout, Then aspect-[4/3] hero + rounded-xl + shadow-md",
      page: pageWith(2, { caption: "card page" }),
      pageNumber: 2,
      layout: "card",
      photoIndexBase: 4,
      expectInHTML: [
        'data-page-layout="card"',
        "aspect-[4/3]",
        "rounded-xl",
        "shadow-md",
      ],
    },
    {
      name: "正常_magazine_layout_with_side_caption",
      description: "Given magazine layout + caption, Then aspect-[16/9] + 横 caption (PC)",
      page: pageWith(3, { caption: "magazine page" }),
      pageNumber: 3,
      layout: "magazine",
      photoIndexBase: 6,
      expectInHTML: [
        'data-page-layout="magazine"',
        "aspect-[16/9]",
        "sm:grid-cols-[1.6fr_1fr]",
      ],
    },
    {
      name: "正常_large_layout",
      description: "Given large layout, Then aspect-[3/2] + sub max 3",
      page: pageWith(8, { caption: "large page" }),
      pageNumber: 4,
      layout: "large",
      photoIndexBase: 9,
      expectInHTML: [
        'data-page-layout="large"',
        "aspect-[3/2]",
        // 8 photos - hero - 3 subs = 4 overflow
        "+4",
      ],
    },
    {
      name: "正常_unknown_layout_は_simple_扱い",
      description: "Given unknown layout, Then default simple spec で描画",
      page: pageWith(2),
      pageNumber: 5,
      layout: "totally_unknown",
      photoIndexBase: 17,
      expectInHTML: [
        'data-page-layout="totally_unknown"',
        "aspect-[4/3]",
      ],
    },
    {
      name: "境界_photos_0件_は_header_のみ_描画",
      description: "Given photos: [], Then header のみ + hero / subs なし",
      page: pageWith(0, { caption: "empty page" }),
      pageNumber: 6,
      layout: "simple",
      photoIndexBase: 19,
      expectInHTML: ["empty page", 'id="page-6"'],
      expectNotInHTML: [
        'aria-label="Page 6 の写真',
        "<img",
      ],
    },
    {
      name: "境界_meta_undefined_は_PageMeta_未描画",
      description: "Given meta undefined, Then page-meta testid が出ない",
      page: pageWith(1, { caption: "no-meta" }),
      pageNumber: 7,
      layout: "simple",
      photoIndexBase: 19,
      expectInHTML: ["no-meta"],
      expectNotInHTML: ['data-testid="page-meta"'],
    },
  ];

  for (const tt of cases) {
    it(tt.name, () => {
      const html = renderInProvider(
        <PageHero
          page={tt.page}
          pageNumber={tt.pageNumber}
          layout={tt.layout}
          photoIndexBase={tt.photoIndexBase}
        />,
        tt.page.photos,
      );
      for (const s of tt.expectInHTML) {
        expect(html).toContain(s);
      }
      for (const s of tt.expectNotInHTML ?? []) {
        expect(html).not.toContain(s);
      }
    });
  }
});
