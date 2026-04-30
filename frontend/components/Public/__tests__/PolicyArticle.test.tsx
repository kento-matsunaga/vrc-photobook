// PolicyArticle / PolicyToc / PolicyNotice の SSR レンダリング検証。

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
    // anchor scroll に必要な scroll-mt-* class
    expect(html).toContain("scroll-mt-6");
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
});

describe("PolicyNotice", () => {
  it("正常_warn_icon_と_children_が出る", () => {
    const html = renderToStaticMarkup(
      <PolicyNotice>テストメッセージ</PolicyNotice>,
    );
    expect(html).toContain('data-testid="policy-notice"');
    expect(html).toContain("テストメッセージ");
    expect(html).toContain("text-status-warn");
  });
});
