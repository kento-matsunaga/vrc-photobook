// PageMeta の SSR レンダリング検証。
//
// 観点:
//   - meta undefined / 全 field 空 → null（DOM 消費しない）
//   - 各 field の組み合わせで正しく出る
//   - cast_list が "/" 区切りで結合される
//   - eventDate が "YYYY.MM.DD" 形式に整形される
//   - invalid date は出ない

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PageMeta } from "@/components/Viewer/PageMeta";

type Case = {
  name: string;
  description: string;
  meta: import("@/lib/publicPhotobook").PublicPageMeta | undefined;
  expectEmpty: boolean;
  expectInHTML?: readonly string[];
  notInHTML?: readonly string[];
};

describe("PageMeta", () => {
  const cases: readonly Case[] = [
    {
      name: "正常_undefined_meta_returns_null",
      description: "Given: meta undefined, Then: null（DOM 出さない）",
      meta: undefined,
      expectEmpty: true,
    },
    {
      name: "正常_全field空_returns_null",
      description: "Given: 全 field 空, Then: null",
      meta: { eventDate: "", world: "", castList: [] as string[], photographer: "" },
      expectEmpty: true,
    },
    {
      name: "正常_全field入り",
      description: "Given: date / world / cast / photographer 全部, Then: 4 バッジが描画",
      meta: {
        eventDate: "2025-05-03",
        world: "夕暮れの港町",
        castList: ["サクラ", "ミナ", "ルカ"],
        photographer: "すずきさん",
      },
      expectEmpty: false,
      expectInHTML: [
        'data-testid="page-meta"',
        "2025.05.03",
        "夕暮れの港町",
        "サクラ / ミナ / ルカ",
        "すずきさん",
        "World",
        "Cast",
        "Photographer",
      ],
    },
    {
      name: "正常_dateのみ",
      description: "Given: eventDate のみ, Then: date バッジのみ",
      meta: { eventDate: "2025-12-01" },
      expectEmpty: false,
      expectInHTML: ["2025.12.01"],
      notInHTML: ["World", "Cast", "Photographer"],
    },
    {
      name: "異常_invalid_date_format",
      description: "Given: invalid date, Then: 日付バッジは出ない",
      meta: { eventDate: "not-a-date", world: "TestWorld" },
      expectEmpty: false,
      expectInHTML: ["TestWorld"],
      notInHTML: ["📅"],
    },
    {
      name: "正常_castListに空文字混在",
      description: "Given: cast に空文字が混ざる, Then: 空文字は除外",
      meta: { castList: ["A", "", "B"] },
      expectEmpty: false,
      expectInHTML: ["A / B"],
    },
  ];

  for (const tt of cases) {
    it(tt.name, () => {
      const html = renderToStaticMarkup(<PageMeta meta={tt.meta} />);
      if (tt.expectEmpty) {
        expect(html, tt.description).toBe("");
        return;
      }
      for (const s of tt.expectInHTML ?? []) {
        expect(html, tt.description).toContain(s);
      }
      for (const s of tt.notInHTML ?? []) {
        expect(html, tt.description).not.toContain(s);
      }
    });
  }
});
