// PublicTopBar の SSR レンダリング検証。
//
// 観点 (m2-design-refresh STOP β-1):
//   - data-testid="public-topbar" を持つ
//   - design 正典 nav の 3 link「作例 / 使い方 / よくある質問」が描画される
//     (`design/source/project/wf-shared.jsx:37-39`)
//   - primary CTA「無料で作る」が showPrimaryCta=true (default) で描画される
//   - showPrimaryCta=false で CTA が描画されない
//   - logo「VRC PhotoBook」が描画される
//   - href が production route に紐付く（/ / /about / /help/manage-url / /create / /#examples）

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PublicTopBar } from "@/components/Public/PublicTopBar";

describe("PublicTopBar", () => {
  it("正常_data-testid + logo + 3 nav link + primary CTA が描画される", () => {
    const html = renderToStaticMarkup(<PublicTopBar />);
    expect(html).toContain('data-testid="public-topbar"');
    expect(html).toContain("VRC PhotoBook");
    expect(html).toContain("作例");
    expect(html).toContain("使い方");
    expect(html).toContain("よくある質問");
    expect(html).toContain("無料で作る");
  });

  it("正常_nav の href が production route に紐付く", () => {
    const html = renderToStaticMarkup(<PublicTopBar />);
    expect(html).toContain('href="/"');
    expect(html).toContain('href="/#examples"');
    expect(html).toContain('href="/about"');
    expect(html).toContain('href="/help/manage-url"');
    expect(html).toContain('href="/create"');
  });

  it("正常_showPrimaryCta=false で CTA は描画されない", () => {
    const html = renderToStaticMarkup(<PublicTopBar showPrimaryCta={false} />);
    expect(html).toContain('data-testid="public-topbar"');
    expect(html).not.toContain('data-testid="public-topbar-cta"');
    expect(html).not.toContain("無料で作る");
  });

  it("正常_主要ナビゲーション aria-label が存在する (accessibility)", () => {
    const html = renderToStaticMarkup(<PublicTopBar />);
    expect(html).toMatch(/aria-label="主要ナビゲーション"/);
    expect(html).toMatch(/aria-label="VRC PhotoBook トップへ"/);
  });
});
