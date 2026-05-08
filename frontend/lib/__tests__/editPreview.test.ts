// editPreview (EditView -> PublicPhotobook 変換 helper) の unit test。
//
// 参照: docs/plan/m2-edit-page-split-and-preview-plan.md §6.9 / §7.4

import { describe, expect, it } from "vitest";

import type { EditView } from "@/lib/editPhotobook";
import {
  PREVIEW_CREATOR_DISPLAY_NAME,
  PREVIEW_FALLBACK_TITLE,
  PREVIEW_SLUG,
  editViewToPreview,
} from "@/lib/editPreview";
import type { PublicPhotobook } from "@/lib/publicPhotobook";

// 最小 EditView fixture を返す helper (test 各ケースで上書き)。
function makeEditView(overrides: Partial<EditView> = {}): EditView {
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

const FIXED_NOW = new Date("2026-05-08T12:00:00Z");

describe("editViewToPreview", () => {
  it("正常_settings_と_photobook_id_を_PublicPhotobook_に_camelCase_保持", () => {
    const v = makeEditView({
      settings: {
        type: "live",
        title: "イベント振り返り",
        description: "本書は試作です",
        layout: "simple",
        openingStyle: "light",
        visibility: "unlisted",
        coverTitle: "Cover Title",
      },
    });
    const p = editViewToPreview(v, FIXED_NOW);
    expect(p.photobookId).toBe(v.photobookId);
    expect(p.slug).toBe(PREVIEW_SLUG);
    expect(p.type).toBe("live");
    expect(p.title).toBe("イベント振り返り");
    expect(p.description).toBe("本書は試作です");
    expect(p.layout).toBe("simple");
    expect(p.openingStyle).toBe("light");
    expect(p.coverTitle).toBe("Cover Title");
    expect(p.publishedAt).toBe(FIXED_NOW.toISOString());
  });

  it("正常_creator_display_name_は_固定文言_creator_x_id_は_undefined", () => {
    const p = editViewToPreview(makeEditView(), FIXED_NOW);
    expect(p.creatorDisplayName).toBe(PREVIEW_CREATOR_DISPLAY_NAME);
    expect(p.creatorXId).toBeUndefined();
  });

  it("正常_title_空文字_の場合_fallback_title", () => {
    const p = editViewToPreview(makeEditView({
      settings: {
        type: "memory",
        title: "",
        layout: "simple",
        openingStyle: "light",
        visibility: "unlisted",
      },
    }), FIXED_NOW);
    expect(p.title).toBe(PREVIEW_FALLBACK_TITLE);
  });

  it("正常_cover_あり_は_PublicVariantSet_と互換でそのまま伝搬", () => {
    const cover = {
      display: { url: "https://r.test/d", width: 1600, height: 1200, expiresAt: "2026-05-08T01:00:00Z" },
      thumbnail: { url: "https://r.test/t", width: 480, height: 360, expiresAt: "2026-05-08T01:00:00Z" },
    };
    const p = editViewToPreview(makeEditView({ cover }), FIXED_NOW);
    expect(p.cover?.display.url).toBe("https://r.test/d");
    expect(p.cover?.display.width).toBe(1600);
    expect(p.cover?.thumbnail.url).toBe("https://r.test/t");
  });

  it("正常_cover_なし_は_undefined", () => {
    const p = editViewToPreview(makeEditView({ cover: undefined }), FIXED_NOW);
    expect(p.cover).toBeUndefined();
  });

  it("正常_pages_順序_と_caption_を保持_photo_caption_も保持", () => {
    const v = makeEditView({
      pages: [
        {
          pageId: "page-A",
          displayOrder: 0,
          caption: "page A cap",
          photos: [
            {
              photoId: "ph-A1",
              imageId: "img-A1",
              displayOrder: 0,
              caption: "photo A1 cap",
              variants: {
                display: { url: "https://r.test/A1d", width: 800, height: 600, expiresAt: "2026-05-08T01:00:00Z" },
                thumbnail: { url: "https://r.test/A1t", width: 240, height: 180, expiresAt: "2026-05-08T01:00:00Z" },
              },
            },
            {
              photoId: "ph-A2",
              imageId: "img-A2",
              displayOrder: 1,
              variants: {
                display: { url: "https://r.test/A2d", width: 800, height: 600, expiresAt: "2026-05-08T01:00:00Z" },
                thumbnail: { url: "https://r.test/A2t", width: 240, height: 180, expiresAt: "2026-05-08T01:00:00Z" },
              },
            },
          ],
        },
        {
          pageId: "page-B",
          displayOrder: 1,
          photos: [],
        },
      ],
    });
    const p = editViewToPreview(v, FIXED_NOW);
    expect(p.pages).toHaveLength(2);
    expect(p.pages[0].caption).toBe("page A cap");
    expect(p.pages[0].photos).toHaveLength(2);
    expect(p.pages[0].photos[0].caption).toBe("photo A1 cap");
    expect(p.pages[0].photos[0].variants.display.url).toBe("https://r.test/A1d");
    expect(p.pages[0].photos[1].caption).toBeUndefined();
    expect(p.pages[1].caption).toBeUndefined();
    expect(p.pages[1].photos).toEqual([]);
  });

  it("正常_pages_0件_でも_変換が成立", () => {
    const p = editViewToPreview(makeEditView({ pages: [] }), FIXED_NOW);
    expect(p.pages).toEqual([]);
  });

  it("正常_meta_は_常に_undefined_Phase_A_では_page_meta_未対応", () => {
    const v = makeEditView({
      pages: [
        {
          pageId: "p",
          displayOrder: 0,
          photos: [],
        },
      ],
    });
    const p = editViewToPreview(v, FIXED_NOW);
    expect(p.pages[0].meta).toBeUndefined();
  });

  it("正常_now_引数省略時_は_現時刻が_publishedAt_に入る", () => {
    const before = Date.now();
    const p = editViewToPreview(makeEditView());
    const after = Date.now();
    const ts = Date.parse(p.publishedAt);
    expect(ts).toBeGreaterThanOrEqual(before);
    expect(ts).toBeLessThanOrEqual(after);
  });

  it("正常_出力は_PublicPhotobook_型と互換_type_assertion", () => {
    // 型として PublicPhotobook に代入できる (compile-time check) を runtime でも確認。
    // ここでは struct shape だけ minimal に確認 (型 assertion は TS が compile 時に検証)。
    const out: PublicPhotobook = editViewToPreview(makeEditView(), FIXED_NOW);
    expect(out.photobookId).toBeDefined();
    expect(out.slug).toBeDefined();
    expect(out.publishedAt).toBeDefined();
    expect(Array.isArray(out.pages)).toBe(true);
  });
});
