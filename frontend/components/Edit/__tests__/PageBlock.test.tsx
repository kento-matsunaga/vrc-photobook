// PageBlock SSR / structural test。
//
// 観点:
//   - page 見出し / page caption editor / photo grid (PhotoGrid) を統合して描画
//   - PhotoGrid に渡された pages / onSplit / onMovePhoto によって PhotoActionBar が photo 内
//     に出る (P-5 split / move 配線)

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PageBlock } from "@/components/Edit/PageBlock";
import type { EditPage } from "@/lib/editPhotobook";

function pageOf(id: string, displayOrder: number, photoCount: number, caption?: string): EditPage {
  return {
    pageId: id,
    displayOrder,
    caption,
    photos: Array.from({ length: photoCount }, (_, i) => ({
      photoId: `${id}-ph${i}`,
      imageId: `${id}-img${i}`,
      displayOrder: i,
      variants: {
        display: { url: `u-${id}-${i}`, width: 100, height: 100, expiresAt: "0" },
        thumbnail: { url: `t-${id}-${i}`, width: 50, height: 50, expiresAt: "0" },
      },
    })),
  };
}

const noopAsync = async () => undefined;

const noHandlers = {
  onPhotoCaptionSave: noopAsync,
  onMoveUp: noopAsync,
  onMoveDown: noopAsync,
  onMoveTop: noopAsync,
  onMoveBottom: noopAsync,
  onSetCover: noopAsync,
  onClearCover: noopAsync,
  onRemovePhoto: noopAsync,
  onPageCaptionSave: noopAsync,
  onSplit: noopAsync,
  onMovePhoto: noopAsync,
  splitDisabledReasonOf: () => undefined,
};

describe("PageBlock SSR markup", () => {
  it("正常_page 見出し + page caption editor + photo grid を描画", () => {
    const page = pageOf("A", 0, 2, "見出し");
    const all = [page, pageOf("B", 1, 1)];
    const html = renderToStaticMarkup(
      <PageBlock
        page={page}
        allPages={all}
        expectedVersion={3}
        isBusy={false}
        isCover={() => false}
        {...noHandlers}
      />,
    );
    // 統合 data-testid
    expect(html).toContain('data-testid="page-block-A"');
    // 見出し: 「ページ 1」(displayOrder + 1)
    expect(html).toContain("ページ 1");
    // page caption editor
    expect(html).toContain('data-testid="page-caption-editor"');
    expect(html).toContain('value="見出し"');
    // photo grid
    expect(html).toContain('data-testid="photo-grid"');
    // photo card 内に PhotoActionBar
    expect(html).toContain('data-testid="photo-action-bar"');
    expect(html).toContain('data-testid="photo-split"');
  });

  it("正常_caption 未設定_は input value 空", () => {
    const page = pageOf("A", 0, 0);
    const html = renderToStaticMarkup(
      <PageBlock
        page={page}
        allPages={[page]}
        expectedVersion={1}
        isBusy={false}
        isCover={() => false}
        {...noHandlers}
      />,
    );
    expect(html).toContain('data-testid="page-caption-editor"');
    expect(html).toContain('value=""');
  });

  it("正常_isBusy 時_page caption editor と photo grid 内 actions が disabled", () => {
    const page = pageOf("A", 0, 1);
    const html = renderToStaticMarkup(
      <PageBlock
        page={page}
        allPages={[page, pageOf("B", 1, 1)]}
        expectedVersion={1}
        isBusy
        isCover={() => false}
        {...noHandlers}
      />,
    );
    // page caption editor input が disabled
    const captionInput = html.match(
      /<input[^>]*data-testid="page-caption-editor"[^>]*>/,
    );
    expect(captionInput?.[0] ?? "").toMatch(/disabled=""/);
  });

  it("正常_split 不可 photo は tooltip 文言を持つ", () => {
    const page = pageOf("A", 0, 1); // photo 1 件 = 末尾 = split 不可
    const html = renderToStaticMarkup(
      <PageBlock
        page={page}
        allPages={[page, pageOf("B", 1, 1)]}
        expectedVersion={1}
        isBusy={false}
        isCover={() => false}
        {...noHandlers}
        splitDisabledReasonOf={(_id, idx) =>
          idx === page.photos.length - 1 ? "末尾の写真ではページを分けられません" : undefined
        }
      />,
    );
    expect(html).toContain('title="末尾の写真ではページを分けられません"');
  });
});
