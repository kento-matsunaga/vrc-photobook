// PreviewPane SSR / structural test (STOP P-6)。
//
// 観点:
//   - editViewToPreview 経由で ViewerLayout を render している
//   - draft 用 placeholder (PREVIEW_CREATOR_DISPLAY_NAME / PREVIEW_FALLBACK_TITLE) が出る
//   - data-testid="preview-pane" でラップしている
//   - photo の caption / variants が DOM に出る (ViewerLayout 経由)

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PreviewPane } from "@/components/Edit/PreviewPane";
import {
  PREVIEW_CREATOR_DISPLAY_NAME,
  PREVIEW_FALLBACK_TITLE,
} from "@/lib/editPreview";
import type { EditView } from "@/lib/editPhotobook";

function viewWith(overrides: Partial<EditView> = {}): EditView {
  return {
    photobookId: "00000000-0000-0000-0000-000000000001",
    status: "draft",
    version: 5,
    settings: {
      type: "memory",
      title: "T",
      layout: "simple",
      openingStyle: "light",
      visibility: "unlisted",
    },
    pages: [],
    processingCount: 0,
    failedCount: 0,
    images: [],
    ...overrides,
  };
}

describe("PreviewPane SSR markup", () => {
  it("正常_data-testid=preview-pane でラップ_ViewerLayout を内包", () => {
    const html = renderToStaticMarkup(<PreviewPane view={viewWith()} />);
    expect(html).toContain('data-testid="preview-pane"');
    // ViewerLayout 内の cover (空 page でも ViewerLayout は描画する)
    expect(html).toContain('data-testid="viewer-main"');
  });

  it("正常_title 設定済み_は title をそのまま表示", () => {
    const v = viewWith({
      settings: {
        type: "memory",
        title: "私のフォトブック",
        layout: "simple",
        openingStyle: "light",
        visibility: "unlisted",
      },
    });
    const html = renderToStaticMarkup(<PreviewPane view={v} />);
    expect(html).toContain("私のフォトブック");
  });

  it("正常_title 空_は preview fallback title を出す", () => {
    const v = viewWith({
      settings: {
        type: "memory",
        title: "",
        layout: "simple",
        openingStyle: "light",
        visibility: "unlisted",
      },
    });
    const html = renderToStaticMarkup(<PreviewPane view={v} />);
    expect(html).toContain(PREVIEW_FALLBACK_TITLE);
  });

  it("正常_creator name は preview placeholder", () => {
    const html = renderToStaticMarkup(<PreviewPane view={viewWith()} />);
    expect(html).toContain(PREVIEW_CREATOR_DISPLAY_NAME);
  });

  it("正常_page caption / photo caption が DOM に伝搬", () => {
    const v = viewWith({
      pages: [
        {
          pageId: "page-A",
          displayOrder: 0,
          caption: "Opening Day",
          photos: [
            {
              photoId: "ph-A1",
              imageId: "img-A1",
              displayOrder: 0,
              caption: "First shot",
              variants: {
                display: { url: "https://r.test/A1d", width: 800, height: 600, expiresAt: "2026-05-09T01:00:00Z" },
                thumbnail: { url: "https://r.test/A1t", width: 240, height: 180, expiresAt: "2026-05-09T01:00:00Z" },
              },
            },
          ],
        },
      ],
    });
    const html = renderToStaticMarkup(<PreviewPane view={v} />);
    expect(html).toContain("Opening Day");
    expect(html).toContain("First shot");
    expect(html).toContain("https://r.test/A1d");
  });
});
