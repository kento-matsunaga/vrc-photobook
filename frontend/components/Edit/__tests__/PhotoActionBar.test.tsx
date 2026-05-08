// PhotoActionBar SSR / structural test。

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PhotoActionBar } from "@/components/Edit/PhotoActionBar";
import type { EditPage } from "@/lib/editPhotobook";

function pageOf(id: string, displayOrder: number, photoCount = 1): EditPage {
  return {
    pageId: id,
    displayOrder,
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

const noopSplit = async () => undefined;
const noopMove = async () => undefined;

describe("PhotoActionBar SSR markup", () => {
  it("正常_split ボタンと PageMovePicker を描画", () => {
    const pages = [pageOf("A", 0), pageOf("B", 1)];
    const html = renderToStaticMarkup(
      <PhotoActionBar
        pages={pages}
        currentPageId="A"
        onSplit={noopSplit}
        onMove={noopMove}
      />,
    );
    expect(html).toContain('data-testid="photo-action-bar"');
    expect(html).toContain('data-testid="photo-split"');
    expect(html).toContain("ここで分ける");
    // PageMovePicker の data-testid 群
    expect(html).toContain('data-testid="page-move-picker"');
    expect(html).toContain('data-testid="page-move-picker-target"');
    expect(html).toContain('data-testid="page-move-picker-execute"');
  });

  it("正常_splitDisabledReason 指定で split ボタン disabled + tooltip", () => {
    const pages = [pageOf("A", 0), pageOf("B", 1)];
    const html = renderToStaticMarkup(
      <PhotoActionBar
        pages={pages}
        currentPageId="A"
        splitDisabledReason="ページ数が上限 (30) に達しています"
        onSplit={noopSplit}
        onMove={noopMove}
      />,
    );
    const splitBtn = html.match(
      /<button[^>]*data-testid="photo-split"[^>]*>/,
    );
    expect(splitBtn?.[0] ?? "").toMatch(/disabled=""/);
    expect(splitBtn?.[0] ?? "").toMatch(/title="ページ数が上限/);
  });

  it("正常_disabled prop で split ボタンも disabled", () => {
    const pages = [pageOf("A", 0), pageOf("B", 1)];
    const html = renderToStaticMarkup(
      <PhotoActionBar
        pages={pages}
        currentPageId="A"
        disabled
        onSplit={noopSplit}
        onMove={noopMove}
      />,
    );
    const splitBtn = html.match(
      /<button[^>]*data-testid="photo-split"[^>]*>/,
    );
    expect(splitBtn?.[0] ?? "").toMatch(/disabled=""/);
  });

  it("正常_末尾 photo の splitDisabledReason 文言が tooltip に出る", () => {
    const pages = [pageOf("A", 0)];
    const html = renderToStaticMarkup(
      <PhotoActionBar
        pages={pages}
        currentPageId="A"
        splitDisabledReason="末尾の写真ではページを分けられません"
        onSplit={noopSplit}
        onMove={noopMove}
      />,
    );
    expect(html).toContain('title="末尾の写真ではページを分けられません"');
  });
});
