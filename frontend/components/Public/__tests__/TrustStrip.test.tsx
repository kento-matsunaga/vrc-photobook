// TrustStrip の SSR レンダリング検証。
//
// 観点 (m2-design-refresh STOP β-2a):
//   - design 正典 (`design/source/project/wf-shared.jsx:69-72`) の 4 chip が出る
//     完全無料 / スマホで完成 / 安全・安心 / VRCユーザー向け
//   - 旧 chip「スマホで完結」「ログイン不要」「VRC ユーザー向け」(space) は出ない
//   - data-testid="trust-strip" を持つ
//   - 全 chip に teal-500 checkmark (`wireframe-styles.css:604-608`)

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { TrustStrip } from "@/components/Public/TrustStrip";

describe("TrustStrip", () => {
  it("正常_design正典_4chip_が全て表示される", () => {
    const html = renderToStaticMarkup(<TrustStrip />);
    expect(html).toContain('data-testid="trust-strip"');
    expect(html).toContain("完全無料");
    expect(html).toContain("スマホで完成");
    expect(html).toContain("安全・安心");
    expect(html).toContain("VRCユーザー向け");
  });

  it("正常_旧chip_スマホで完結_ログイン不要_VRC_ユーザー向け_は表示されない", () => {
    const html = renderToStaticMarkup(<TrustStrip />);
    expect(html).not.toContain("スマホで完結");
    expect(html).not.toContain("ログイン不要");
    expect(html).not.toContain("VRC ユーザー向け");
  });

  it("正常_grid-cols-2_と_sm:grid-cols-4_を持つ", () => {
    const html = renderToStaticMarkup(<TrustStrip />);
    expect(html).toContain("grid-cols-2");
    expect(html).toContain("sm:grid-cols-4");
  });

  it("正常_全chip_に_teal-500_checkmark_が出る", () => {
    const html = renderToStaticMarkup(<TrustStrip />);
    // 4 chip すべてに ✓ marker (aria-hidden, text-teal-500)
    const checkCount = (html.match(/✓/g) ?? []).length;
    expect(checkCount).toBe(4);
    expect(html).toContain("text-teal-500");
  });
});
