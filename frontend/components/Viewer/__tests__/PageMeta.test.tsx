// PageMeta.tsx の SSR レンダリング検証 (table 駆動)。
//
// 観点:
//   - 4 field (eventDate / world / castList / photographer) の有無の組み合わせ
//   - undefined / 空 → null (renderToStaticMarkup で "")
//   - cast 4 件超は "他 N 名" 表記

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PageMeta } from "@/components/Viewer/PageMeta";
import type { PublicPageMeta } from "@/lib/publicPhotobook";

describe("PageMeta", () => {
  type Case = {
    name: string;
    description: string;
    meta: PublicPageMeta | undefined;
    expectInHTML: string[];
    expectIsEmpty?: boolean;
  };

  const cases: Case[] = [
    {
      name: "正常_全項目_あり",
      description: "Given date / world / cast / photographer 全部, When render, Then 4 項目とアイコンが出る",
      meta: {
        eventDate: "2026-04-29",
        world: "Sunset Rooftop",
        castList: ["@a", "@b"] as string[],
        photographer: "ERENOA",
      },
      expectInHTML: [
        '<ul data-testid="page-meta"',
        "2026.04.29",
        "Sunset Rooftop",
        "@a / @b",
        "ERENOA",
      ],
    },
    {
      name: "正常_date_のみ",
      description: "Given date のみ, When render, Then date 1 項目のみ + 他 0",
      meta: { eventDate: "2026-04-29" },
      expectInHTML: ["2026.04.29"],
    },
    {
      name: "正常_cast_5件_は_他_1_名_表記",
      description: "Given cast 5 件, When render, Then 4 件 + 他 1 名",
      meta: {
        castList: ["@a", "@b", "@c", "@d", "@e"] as string[],
      },
      expectInHTML: ["@a / @b / @c / @d 他 1 名"],
    },
    {
      name: "境界_meta_undefined_は_null",
      description: "Given meta undefined, When render, Then html 空文字 (DOM を消費しない)",
      meta: undefined,
      expectInHTML: [],
      expectIsEmpty: true,
    },
    {
      name: "境界_全項目_undefined_は_null",
      description: "Given 4 field 全部 undefined, When render, Then html 空文字",
      meta: {},
      expectInHTML: [],
      expectIsEmpty: true,
    },
    {
      name: "境界_castList_空配列_は_skip",
      description: "Given castList: [], When render, Then cast 行は出ない",
      meta: {
        eventDate: "2026-04-29",
        castList: [] as string[],
      },
      expectInHTML: ["2026.04.29"],
    },
  ];

  for (const tt of cases) {
    it(tt.name, () => {
      const html = renderToStaticMarkup(<PageMeta meta={tt.meta} />);
      if (tt.expectIsEmpty) {
        expect(html).toBe("");
      } else {
        for (const s of tt.expectInHTML) {
          expect(html).toContain(s);
        }
      }
    });
  }
});
