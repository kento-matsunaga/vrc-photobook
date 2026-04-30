// TrustStrip の SSR レンダリング検証。
//
// 観点:
//   - 4 cell が出る（完全無料 / スマホで完結 / ログイン不要 / VRC ユーザー向け）
//   - data-testid="trust-strip" を持つ

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { TrustStrip } from "@/components/Public/TrustStrip";

describe("TrustStrip", () => {
  it("正常_4cell_が全て表示される", () => {
    const html = renderToStaticMarkup(<TrustStrip />);
    expect(html).toContain('data-testid="trust-strip"');
    expect(html).toContain("完全無料");
    expect(html).toContain("スマホで完結");
    expect(html).toContain("ログイン不要");
    expect(html).toContain("VRC ユーザー向け");
  });

  it("正常_grid-cols-2_と_sm:grid-cols-4_を持つ", () => {
    const html = renderToStaticMarkup(<TrustStrip />);
    expect(html).toContain("grid-cols-2");
    expect(html).toContain("sm:grid-cols-4");
  });
});
