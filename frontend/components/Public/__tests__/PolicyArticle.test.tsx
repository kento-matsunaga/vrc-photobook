// PolicyArticle / PolicyToc / PolicyNotice の SSR レンダリング検証。
//
// 観点 (m2-design-refresh STOP β-2b-1):
//   - PolicyArticle: design `wf-box` 化 (rounded-lg / border-divider-soft / bg-surface /
//                    shadow-sm / padding 5-6) + scroll-mt-20 (PublicTopBar sticky 53px 補正)
//   - PolicyToc: design `wf-toc` left teal-200 bar + grid (旧 outer rounded-lg card は廃止)
//   - PolicyNotice: design `wf-note` border-l-[3px] teal-300 + bg teal-50 + i icon teal-500
//   - 旧 scroll-mt-6 / 旧 text-status-warn は出ない

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import {
  PolicyArticle,
  PolicyNotice,
  PolicyToc,
} from "@/components/Public/PolicyArticle";

describe("PolicyArticle", () => {
  it("正常_id_number_title_children_を含む", () => {
    const html = renderToStaticMarkup(
      <PolicyArticle id="terms-x" number="第 X 条" title="サンプルタイトル">
        <p>本文サンプル</p>
      </PolicyArticle>,
    );
    expect(html).toContain('data-testid="policy-article-terms-x"');
    expect(html).toContain('id="terms-x"');
    expect(html).toContain('id="terms-x-heading"');
    expect(html).toContain("第 X 条");
    expect(html).toContain("サンプルタイトル");
    expect(html).toContain("本文サンプル");
  });

  it("正常_PublicTopBar_sticky補正の_scroll-mt-20_を持ち_旧scroll-mt-6_は出ない", () => {
    const html = renderToStaticMarkup(
      <PolicyArticle id="terms-x" number="第 X 条" title="t">
        <p>b</p>
      </PolicyArticle>,
    );
    expect(html).toContain("scroll-mt-20");
    expect(html).not.toContain("scroll-mt-6");
  });

  it("正常_design_wf-box_card_視覚_rounded_border_shadow_を持つ", () => {
    const html = renderToStaticMarkup(
      <PolicyArticle id="terms-x" number="第 X 条" title="t">
        <p>b</p>
      </PolicyArticle>,
    );
    expect(html).toContain("rounded-lg");
    expect(html).toContain("border-divider-soft");
    expect(html).toContain("bg-surface");
    expect(html).toContain("shadow-sm");
  });
});

describe("PolicyToc", () => {
  it("正常_items_が_anchor_リンクとして出る", () => {
    const html = renderToStaticMarkup(
      <PolicyToc
        ariaLabel="テスト目次"
        items={[
          { id: "x-1", label: "第 1 条 サンプル" },
          { id: "x-2", label: "第 2 条 サンプル" },
        ]}
      />,
    );
    expect(html).toContain('data-testid="policy-toc"');
    expect(html).toContain('aria-label="テスト目次"');
    expect(html).toContain('href="#x-1"');
    expect(html).toContain('href="#x-2"');
    expect(html).toContain("目次");
  });

  it("正常_design_wf-toc_left-teal-bar_視覚を持つ", () => {
    const html = renderToStaticMarkup(
      <PolicyToc
        ariaLabel="x"
        items={[{ id: "x-1", label: "x" }]}
      />,
    );
    // design `wireframe-styles.css:538-545` の border-left 2px teal-200
    expect(html).toContain("border-l-2");
    expect(html).toContain("border-teal-200");
  });
});

describe("PolicyNotice", () => {
  it("正常_children_と_design_wf-note_視覚_teal-300_teal-50_teal-500_を持つ", () => {
    const html = renderToStaticMarkup(
      <PolicyNotice>テストメッセージ</PolicyNotice>,
    );
    expect(html).toContain('data-testid="policy-notice"');
    expect(html).toContain("テストメッセージ");
    // design `wireframe-styles.css:398-425` の border-left teal-300 + bg teal-50 + i icon teal-500
    expect(html).toContain("border-teal-300");
    expect(html).toContain("bg-teal-50");
    expect(html).toContain("bg-teal-500");
    // 旧 status-warn 系の警告アイコンは廃止 (design は teal 系で統一)
    expect(html).not.toContain("text-status-warn");
  });
});
