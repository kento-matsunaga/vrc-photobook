// Cover.tsx の SSR レンダリング検証 (v2 redesign)。
//
// 観点:
//   - variant cover_first / light の 2 パターン
//   - cover あり (pattern A グラデーション) / cover なし (pattern C フォールバック) /
//     pattern B (portfolio / world / large layout で半透明パネル)
//   - coverTitle 優先 / fallback to title

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { Cover } from "@/components/Viewer/Cover";
import type { PublicPhotobook } from "@/lib/publicPhotobook";

function basePhotobook(overrides: Partial<PublicPhotobook> = {}): PublicPhotobook {
  return {
    photobookId: "redacted",
    slug: "sample-slug",
    type: "event",
    title: "Sunset Memories",
    description: "あの日の集い",
    layout: "magazine",
    openingStyle: "cover_first",
    creatorDisplayName: "ERENOA",
    creatorXId: "Noa_Fortevita",
    coverTitle: undefined,
    cover: {
      display: {
        url: "https://images.example.invalid/cover.jpg",
        width: 1600,
        height: 2400,
        expiresAt: "2099-12-31T23:59:59Z",
      },
      thumbnail: {
        url: "https://images.example.invalid/cover.thumb.jpg",
        width: 400,
        height: 600,
        expiresAt: "2099-12-31T23:59:59Z",
      },
    },
    publishedAt: "2026-04-29T12:00:00Z",
    pages: [],
    ...overrides,
  };
}

describe("Cover", () => {
  type Case = {
    name: string;
    description: string;
    photobook: PublicPhotobook;
    variant: "cover_first" | "light";
    expectInHTML: string[];
    expectNotInHTML?: string[];
    expectPattern: "A" | "B" | "C";
  };

  const cases: Case[] = [
    {
      name: "正常_cover_first_with_image_pattern_A",
      description: "Given cover 画像あり + type event, When variant cover_first, Then パターン A グラデーション + 「読む」CTA が描画",
      photobook: basePhotobook(),
      variant: "cover_first",
      expectInHTML: [
        "Sunset Memories",
        "あの日の集い",
        "ERENOA",
        "2026.04.29",
        '<a href="#page-1"',
        "読む",
        'data-cover-pattern="A"',
        'data-cover-variant="cover_first"',
      ],
      expectNotInHTML: ['data-cover-pattern="B"', 'data-cover-pattern="C"'],
      expectPattern: "A",
    },
    {
      name: "正常_light_with_image_pattern_A_no_cta",
      description: "Given cover 画像あり, When variant light, Then 「読む」CTA は描画されない",
      photobook: basePhotobook(),
      variant: "light",
      expectInHTML: ["Sunset Memories", 'data-cover-variant="light"'],
      expectNotInHTML: ["読む", '<a href="#page-1"'],
      expectPattern: "A",
    },
    {
      name: "正常_no_cover_pattern_C_fallback",
      description: "Given cover 無し, When 任意 variant, Then パターン C フォールバック (画像なし)",
      photobook: basePhotobook({ cover: undefined }),
      variant: "cover_first",
      expectInHTML: ["Sunset Memories", 'data-cover-pattern="C"', "読む"],
      expectNotInHTML: ["images.example.invalid"],
      expectPattern: "C",
    },
    {
      name: "正常_portfolio_type_pattern_B_panel",
      description: "Given type portfolio, When cover あり, Then パターン B 半透明パネル",
      photobook: basePhotobook({ type: "portfolio" }),
      variant: "cover_first",
      expectInHTML: ['data-cover-pattern="B"', "Sunset Memories"],
      expectNotInHTML: ['data-cover-pattern="A"'],
      expectPattern: "B",
    },
    {
      name: "正常_large_layout_pattern_B_panel",
      description: "Given layout large, When cover あり, Then パターン B 半透明パネル",
      photobook: basePhotobook({ layout: "large" }),
      variant: "cover_first",
      expectInHTML: ['data-cover-pattern="B"'],
      expectPattern: "B",
    },
    {
      name: "正常_coverTitle_overrides_title",
      description: "Given coverTitle あり, When cover 描画, Then coverTitle が title より優先",
      photobook: basePhotobook({ coverTitle: "別の表紙文言" }),
      variant: "cover_first",
      expectInHTML: ["別の表紙文言"],
      expectNotInHTML: ["Sunset Memories"],
      expectPattern: "A",
    },
  ];

  for (const tt of cases) {
    it(tt.name, () => {
      const html = renderToStaticMarkup(
        <Cover photobook={tt.photobook} variant={tt.variant} />,
      );
      for (const s of tt.expectInHTML) {
        expect(html).toContain(s);
      }
      for (const s of tt.expectNotInHTML ?? []) {
        expect(html).not.toContain(s);
      }
      expect(html).toContain(`data-cover-pattern="${tt.expectPattern}"`);
    });
  }

  it("正常_raw_photobookId_は_DOM_に出ない", () => {
    const html = renderToStaticMarkup(
      <Cover photobook={basePhotobook({ photobookId: "secret-internal-id" })} variant="light" />,
    );
    expect(html).not.toContain("secret-internal-id");
  });
});
