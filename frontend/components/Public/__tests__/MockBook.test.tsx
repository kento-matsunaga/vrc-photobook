// MockBook / MockThumb の SSR レンダリング検証。
//
// 観点 (m2-design-refresh STOP β-2a):
//   - design 正典 (`design/source/project/wf-screens-a.jsx:4-43`) の構造を満たす
//     - 左 cover (width 58%) + 右 page (width 48%, 2x2 grid)
//     - 右 page の 3 cell は aria-hidden="true"
//   - mock-book は title / date / worldLabel を実テキストで表示
//   - 旧 PC 専用 rotate 小カード装飾は削除されている（design に無い）
//   - MockThumb 5 variants の data-testid を含む

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { MockBook, MockThumb } from "@/components/Public/MockBook";

describe("MockBook", () => {
  it("正常_title_date_worldLabel_を含む", () => {
    const html = renderToStaticMarkup(
      <MockBook
        title="テストタイトル"
        date="2026.04.24"
        worldLabel="Test World"
      />,
    );
    expect(html).toContain('data-testid="mock-book"');
    expect(html).toContain("テストタイトル");
    expect(html).toContain("2026.04.24");
    expect(html).toContain("Test World");
  });

  it("正常_左cover58_と_右page48_2x2grid_の design 正典構造を持つ", () => {
    const html = renderToStaticMarkup(<MockBook title="Title" />);
    // 左 cover: width 58% / 非対称 radius 4px-14px / teal-50 → surface gradient
    expect(html).toContain("w-[58%]");
    expect(html).toContain("rounded-l-[4px]");
    expect(html).toContain("rounded-r-[14px]");
    expect(html).toContain("from-teal-50");
    // 右 page: width 48% / 2x2 grid / 非対称 radius 14px-4px
    expect(html).toContain("w-[48%]");
    expect(html).toContain("grid-cols-2");
    expect(html).toContain("grid-rows-2");
    expect(html).toContain("rounded-l-[14px]");
    expect(html).toContain("rounded-r-[4px]");
    // 右 page の 3 cell すべて aria-hidden（装飾）+ top span 2
    expect(html).toContain("col-span-2");
    expect(html.match(/aria-hidden="true"/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
  });

  it("正常_旧rotate装飾_は削除されている", () => {
    const html = renderToStaticMarkup(<MockBook title="Title" />);
    // design 正典には存在しない rotate 装飾は出さない
    expect(html).not.toContain("rotate-6");
    expect(html).not.toContain("-rotate-3");
  });

  it("正常_date_worldLabel_省略時は表示しない", () => {
    const html = renderToStaticMarkup(<MockBook title="Title" />);
    expect(html).toContain("Title");
    expect(html).not.toMatch(/2026\.\d{2}\.\d{2}/);
  });
});

describe("MockThumb", () => {
  it("正常_variants_a_b_c_d_e_全て_data-testid_付与", () => {
    const variants = ["a", "b", "c", "d", "e"] as const;
    for (const v of variants) {
      const html = renderToStaticMarkup(<MockThumb variant={v} />);
      expect(html).toContain(`data-testid="mock-thumb-${v}"`);
      expect(html).toContain('aria-hidden="true"');
    }
  });

  it("正常_aspect-square_default_と_sm:aspect-[4/3]_PC_を持つ", () => {
    const html = renderToStaticMarkup(<MockThumb variant="a" />);
    // mobile = 1:1 (`wf-screens-a.jsx:62-64`), PC = 4:3 (`:141-143`)
    expect(html).toContain("aspect-square");
    expect(html).toContain("sm:aspect-[4/3]");
    expect(html).toContain("rounded-sm");
  });
});
