// MockBook / MockThumb の SSR レンダリング検証。
//
// 観点:
//   - mock-book は title / date / worldLabel を表示
//   - 装飾的 rotate 小カード x2 は sm:hidden で mobile 非表示（hidden class を含む）
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

  it("正常_PC専用の_rotate小カードは_mobile非表示_になる", () => {
    const html = renderToStaticMarkup(<MockBook title="Title" />);
    // 2 つの装飾 span に hidden sm:block が付く
    expect(html.match(/aria-hidden="true"/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
    expect(html).toContain("hidden");
    expect(html).toContain("sm:block");
  });

  it("正常_date_worldLabel_省略時は表示しない", () => {
    const html = renderToStaticMarkup(<MockBook title="Title" />);
    expect(html).toContain("Title");
    // 数値文字列（仮に 2026 で始まるもの）が出ていない
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

  it("正常_aspect-square_と_rounded-sm_を持つ", () => {
    const html = renderToStaticMarkup(<MockThumb variant="a" />);
    expect(html).toContain("aspect-square");
    expect(html).toContain("rounded-sm");
  });
});
