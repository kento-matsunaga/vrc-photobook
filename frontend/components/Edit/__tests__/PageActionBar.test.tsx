// PageActionBar SSR / structural test (STOP P-6)。

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PageActionBar } from "@/components/Edit/PageActionBar";

const noop = async () => undefined;

describe("PageActionBar SSR markup", () => {
  it("正常_1 page only_は何も描画しない (cannot_remove_last_page を UI で防御)", () => {
    const html = renderToStaticMarkup(
      <PageActionBar
        pageIndex={0}
        pageCount={1}
        onMerge={noop}
        onMoveUp={noop}
        onMoveDown={noop}
      />,
    );
    // null render → 空文字
    expect(html).toBe("");
  });

  it("正常_先頭 page (idx=0)_merge button は出さず_↑ は disabled_↓ は有効", () => {
    const html = renderToStaticMarkup(
      <PageActionBar
        pageIndex={0}
        pageCount={3}
        onMerge={noop}
        onMoveUp={noop}
        onMoveDown={noop}
      />,
    );
    expect(html).toContain('data-testid="page-action-bar"');
    // merge button は idx=0 では出ない (merge_into_self / 上が無い のため)
    expect(html).not.toContain('data-testid="page-merge"');
    // ↑ は disabled
    const upBtn = html.match(/<button[^>]*data-testid="page-reorder-up"[^>]*>/);
    expect(upBtn?.[0] ?? "").toMatch(/disabled=""/);
    // ↓ は enabled
    const downBtn = html.match(/<button[^>]*data-testid="page-reorder-down"[^>]*>/);
    expect(downBtn?.[0] ?? "").not.toMatch(/disabled=""/);
  });

  it("正常_中間 page_merge / ↑ / ↓ すべて有効", () => {
    const html = renderToStaticMarkup(
      <PageActionBar
        pageIndex={1}
        pageCount={3}
        onMerge={noop}
        onMoveUp={noop}
        onMoveDown={noop}
      />,
    );
    expect(html).toContain('data-testid="page-merge"');
    const mergeBtn = html.match(/<button[^>]*data-testid="page-merge"[^>]*>/);
    expect(mergeBtn?.[0] ?? "").not.toMatch(/disabled=""/);
    const upBtn = html.match(/<button[^>]*data-testid="page-reorder-up"[^>]*>/);
    expect(upBtn?.[0] ?? "").not.toMatch(/disabled=""/);
    const downBtn = html.match(/<button[^>]*data-testid="page-reorder-down"[^>]*>/);
    expect(downBtn?.[0] ?? "").not.toMatch(/disabled=""/);
  });

  it("正常_末尾 page_↓ は disabled_↑ / merge は有効", () => {
    const html = renderToStaticMarkup(
      <PageActionBar
        pageIndex={2}
        pageCount={3}
        onMerge={noop}
        onMoveUp={noop}
        onMoveDown={noop}
      />,
    );
    const downBtn = html.match(/<button[^>]*data-testid="page-reorder-down"[^>]*>/);
    expect(downBtn?.[0] ?? "").toMatch(/disabled=""/);
    const upBtn = html.match(/<button[^>]*data-testid="page-reorder-up"[^>]*>/);
    expect(upBtn?.[0] ?? "").not.toMatch(/disabled=""/);
    expect(html).toContain('data-testid="page-merge"');
  });

  it("正常_disabled prop で全 button disabled", () => {
    const html = renderToStaticMarkup(
      <PageActionBar
        pageIndex={1}
        pageCount={3}
        disabled
        onMerge={noop}
        onMoveUp={noop}
        onMoveDown={noop}
      />,
    );
    const mergeBtn = html.match(/<button[^>]*data-testid="page-merge"[^>]*>/);
    expect(mergeBtn?.[0] ?? "").toMatch(/disabled=""/);
    const upBtn = html.match(/<button[^>]*data-testid="page-reorder-up"[^>]*>/);
    expect(upBtn?.[0] ?? "").toMatch(/disabled=""/);
    const downBtn = html.match(/<button[^>]*data-testid="page-reorder-down"[^>]*>/);
    expect(downBtn?.[0] ?? "").toMatch(/disabled=""/);
  });

  it("正常_aria-label / merge ラベルに raw page_id を含めない", () => {
    const html = renderToStaticMarkup(
      <PageActionBar
        pageIndex={1}
        pageCount={3}
        onMerge={noop}
        onMoveUp={noop}
        onMoveDown={noop}
      />,
    );
    expect(html).toContain('aria-label="上のページと結合"');
    expect(html).toContain('aria-label="ページを上へ"');
    expect(html).toContain('aria-label="ページを下へ"');
    // 表示文字列は「上と結合」/ ↑ / ↓ のみ。UUID 風文字列を含まない。
    expect(html).not.toMatch(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}/);
  });
});
