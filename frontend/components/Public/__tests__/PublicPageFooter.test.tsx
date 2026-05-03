// PublicPageFooter の SSR レンダリング検証。
//
// 観点 (m2-design-refresh STOP β-2a):
//   - design 正典 (`design/source/project/wf-shared.jsx:78`) の 4 link
//     About / Help / Terms / Privacy が出る
//   - 旧「トップ」link は削除 (PublicTopBar の logo が `/` 遷移を担う)
//   - links prop で上書き可
//   - showTrustStrip=true で trust-strip が design 正典 4 chip を出す
//   - extraSlot で任意要素を挿入できる（Viewer の通報リンク用）

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";

describe("PublicPageFooter", () => {
  it("正常_design正典_4link_About_Help_Terms_Privacy_を含む", () => {
    const html = renderToStaticMarkup(<PublicPageFooter />);
    expect(html).toContain('data-testid="public-page-footer"');
    expect(html).toContain('href="/about"');
    expect(html).toContain('href="/help/manage-url"');
    expect(html).toContain('href="/terms"');
    expect(html).toContain('href="/privacy"');
    expect(html).toContain(">About</a>");
    expect(html).toContain(">Help</a>");
    expect(html).toContain(">Terms</a>");
    expect(html).toContain(">Privacy</a>");
    expect(html).toContain("VRC PhotoBook（非公式ファンメイドサービス）");
    expect(html).not.toContain('data-testid="trust-strip"');
  });

  it("正常_旧トップlink_は削除されている", () => {
    const html = renderToStaticMarkup(<PublicPageFooter />);
    // PublicTopBar の logo が `/` を担うため footer に href="/" は出さない
    expect(html).not.toContain('href="/"');
    expect(html).not.toContain(">トップ</a>");
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
    expect(html).toContain("スマホで完成");
    expect(html).toContain("安全・安心");
    expect(html).toContain("VRCユーザー向け");
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
