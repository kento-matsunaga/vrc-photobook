// PageMovePicker SSR / structural test。

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PageMovePicker } from "@/components/Edit/PageMovePicker";
import type { EditPage } from "@/lib/editPhotobook";

function pageOf(id: string, displayOrder: number, photoCount = 0, caption?: string): EditPage {
  return {
    pageId: id,
    displayOrder,
    caption,
    photos: Array.from({ length: photoCount }, (_, i) => ({
      photoId: `${id}-ph${i}`,
      imageId: `${id}-img${i}`,
      displayOrder: i,
      variants: {
        display: { url: "u", width: 1, height: 1, expiresAt: "0" },
        thumbnail: { url: "u", width: 1, height: 1, expiresAt: "0" },
      },
    })),
  };
}

const noop = async () => undefined;

describe("PageMovePicker SSR markup", () => {
  it("正常_全 page を option として描画_現在 page は disabled", () => {
    const pages = [pageOf("A", 0, 2), pageOf("B", 1, 1), pageOf("C", 2, 0)];
    const html = renderToStaticMarkup(
      <PageMovePicker pages={pages} currentPageId="B" onMove={noop} />,
    );
    // 全 3 page が option として出ている
    expect(html).toContain('value="A"');
    expect(html).toContain('value="B"');
    expect(html).toContain('value="C"');
    // 現在 page (B) の option は disabled
    expect(html).toMatch(/<option[^>]*value="B"[^>]*disabled[^>]*>/);
    // raw page_id は表示テキストに出さない (page A は「ページ 1」表記、page_id "A" は value のみ)
    expect(html).toContain("ページ 1");
    expect(html).toContain("ページ 2");
    expect(html).toContain("ページ 3");
    expect(html).toContain("(2 枚)");
    expect(html).toContain("(1 枚)");
  });

  it("正常_caption ありの page は option ラベルに含む", () => {
    const pages = [pageOf("A", 0, 1, "Opening"), pageOf("B", 1, 1)];
    const html = renderToStaticMarkup(
      <PageMovePicker pages={pages} currentPageId="B" onMove={noop} />,
    );
    expect(html).toContain("Opening");
  });

  it("正常_他 page なし_disabled select_と_(他のページがありません)", () => {
    const pages = [pageOf("A", 0, 1)];
    const html = renderToStaticMarkup(
      <PageMovePicker pages={pages} currentPageId="A" onMove={noop} />,
    );
    expect(html).toContain("(他のページがありません)");
    expect(html).toMatch(/<select[^>]*disabled[^>]*>/);
  });

  it("正常_position toggle ボタンと実行ボタンが描画される", () => {
    const pages = [pageOf("A", 0, 1), pageOf("B", 1, 1)];
    const html = renderToStaticMarkup(
      <PageMovePicker pages={pages} currentPageId="A" onMove={noop} />,
    );
    expect(html).toContain('data-testid="page-move-picker-position-start"');
    expect(html).toContain('data-testid="page-move-picker-position-end"');
    expect(html).toContain('data-testid="page-move-picker-execute"');
    expect(html).toContain("先頭");
    expect(html).toContain("末尾");
    expect(html).toContain("移動");
  });

  it("正常_disabled prop で全 control disabled", () => {
    const pages = [pageOf("A", 0, 1), pageOf("B", 1, 1)];
    const html = renderToStaticMarkup(
      <PageMovePicker pages={pages} currentPageId="A" disabled onMove={noop} />,
    );
    // select が disabled
    expect(html).toMatch(/<select[^>]*disabled[^>]*>/);
    // 実行ボタンも disabled
    const execBtn = html.match(
      /<button[^>]*data-testid="page-move-picker-execute"[^>]*>/,
    );
    expect(execBtn?.[0] ?? "").toMatch(/disabled=""/);
  });
});
