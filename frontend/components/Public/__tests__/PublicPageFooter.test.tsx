// PublicPageFooter の SSR レンダリング検証。
//
// 観点:
//   - 既定リンク 5 件（Top / About / Terms / Privacy / Help）
//   - links prop で上書き可
//   - showTrustStrip=true で trust-strip が出る
//   - extraSlot で任意要素を挿入できる（Viewer の通報リンク用）

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";

describe("PublicPageFooter", () => {
  it("正常_既定リンク_5件_を含む", () => {
    const html = renderToStaticMarkup(<PublicPageFooter />);
    expect(html).toContain('data-testid="public-page-footer"');
    expect(html).toContain('href="/"');
    expect(html).toContain('href="/about"');
    expect(html).toContain('href="/terms"');
    expect(html).toContain('href="/privacy"');
    expect(html).toContain('href="/help/manage-url"');
    expect(html).toContain("VRC PhotoBook（非公式ファンメイドサービス）");
    expect(html).not.toContain('data-testid="trust-strip"');
  });

  it("正常_links_prop_で上書きできる", () => {
    const html = renderToStaticMarkup(
      <PublicPageFooter
        links={[
          { href: "/about", label: "About" },
          { href: "/terms", label: "Terms" },
        ]}
      />,
    );
    expect(html).toContain('href="/about"');
    expect(html).toContain('href="/terms"');
    expect(html).not.toContain('href="/privacy"');
  });

  it("正常_showTrustStrip=true_で_trust_strip_が出る", () => {
    const html = renderToStaticMarkup(<PublicPageFooter showTrustStrip />);
    expect(html).toContain('data-testid="trust-strip"');
    expect(html).toContain("完全無料");
    expect(html).toContain("スマホで完結");
    expect(html).toContain("ログイン不要");
    expect(html).toContain("VRC ユーザー向け");
  });

  it("正常_extraSlot_で任意要素が挿入される", () => {
    const html = renderToStaticMarkup(
      <PublicPageFooter
        extraSlot={<a data-testid="custom-slot">通報</a>}
      />,
    );
    expect(html).toContain('data-testid="custom-slot"');
    expect(html).toContain("通報");
  });
});
