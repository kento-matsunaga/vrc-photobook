// PreviewToggle SSR / structural test (STOP P-6)。

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PreviewToggle } from "@/components/Edit/PreviewToggle";

const noop = () => undefined;

describe("PreviewToggle SSR markup", () => {
  it("正常_edit mode_「📖 プレビュー」label_aria-pressed=false", () => {
    const html = renderToStaticMarkup(<PreviewToggle mode="edit" onToggle={noop} />);
    expect(html).toContain('data-testid="preview-toggle"');
    expect(html).toContain("プレビュー");
    expect(html).toContain('aria-pressed="false"');
    expect(html).toContain('aria-label="プレビューを開く"');
  });

  it("正常_preview mode_「✏️ 編集に戻る」label_aria-pressed=true", () => {
    const html = renderToStaticMarkup(<PreviewToggle mode="preview" onToggle={noop} />);
    expect(html).toContain("編集に戻る");
    expect(html).toContain('aria-pressed="true"');
    expect(html).toContain('aria-label="編集に戻る"');
  });

  it("正常_disabled prop で button disabled", () => {
    const html = renderToStaticMarkup(
      <PreviewToggle mode="edit" disabled onToggle={noop} />,
    );
    const btn = html.match(/<button[^>]*data-testid="preview-toggle"[^>]*>/);
    expect(btn?.[0] ?? "").toMatch(/disabled=""/);
  });
});
