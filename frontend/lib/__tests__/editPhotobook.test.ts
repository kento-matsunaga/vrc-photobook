// editPhotobook API client の unit test。
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  bulkReorderPhotos,
  clearCoverImage,
  fetchEditView,
  fetchEditViewClient,
  isEditApiError,
  mergePages,
  movePhoto,
  prepareAttachImages,
  removePhoto,
  reorderPages,
  setCoverImage,
  splitPage,
  updatePageCaption,
  updatePhotobookSettings,
  updatePhotoCaption,
} from "@/lib/editPhotobook";

const ORIGINAL_API = process.env.NEXT_PUBLIC_API_BASE_URL;

beforeEach(() => {
  process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test";
});

afterEach(() => {
  vi.unstubAllGlobals();
  process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
});

describe("fetchEditView", () => {
  it("正常_200_payloadをcamelCase化", async () => {
    const body = {
      photobook_id: "00000000-0000-0000-0000-000000000001",
      status: "draft",
      version: 3,
      settings: {
        type: "memory", title: "T", layout: "simple",
        opening_style: "light", visibility: "unlisted",
      },
      pages: [
        {
          page_id: "p1", display_order: 0,
          photos: [
            {
              photo_id: "ph1", image_id: "im1", display_order: 0,
              variants: {
                display: { url: "https://r.test/d", width: 1600, height: 1200, expires_at: "2026-01-01T00:15:00Z" },
                thumbnail: { url: "https://r.test/t", width: 480, height: 360, expires_at: "2026-01-01T00:15:00Z" },
              },
            },
          ],
        },
      ],
      processing_count: 1, failed_count: 0,
    };
    vi.stubGlobal("fetch", vi.fn(async () => ({ status: 200, json: async () => body })));
    const got = await fetchEditView("00000000-0000-0000-0000-000000000001", "vrcpb_draft_x=v");
    expect(got.version).toBe(3);
    expect(got.settings.openingStyle).toBe("light");
    expect(got.pages[0].photos[0].variants.display.width).toBe(1600);
    expect(got.processingCount).toBe(1);
  });

  for (const tt of [
    { status: 401, kind: "unauthorized" },
    { status: 404, kind: "not_found" },
    { status: 409, kind: "version_conflict" },
    { status: 400, kind: "bad_request" },
    { status: 500, kind: "server_error" },
  ] as const) {
    it(`異常_${tt.status}_kind_${tt.kind}`, async () => {
      vi.stubGlobal("fetch", vi.fn(async () => ({ status: tt.status, json: async () => ({}) })));
      try {
        await fetchEditView("00000000-0000-0000-0000-000000000001", "");
      } catch (e) {
        expect(isEditApiError(e)).toBe(true);
        if (isEditApiError(e)) expect(e.kind).toBe(tt.kind);
      }
    });
  }

  it("異常_network失敗", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new Error("dns"); }));
    try {
      await fetchEditView("00000000-0000-0000-0000-000000000001", "");
    } catch (e) {
      expect(isEditApiError(e)).toBe(true);
      if (isEditApiError(e)) expect(e.kind).toBe("network");
    }
  });

  it("正常_images 配列を camelCase 化（β-3）", async () => {
    const body = {
      photobook_id: "00000000-0000-0000-0000-000000000001",
      status: "draft", version: 5,
      settings: { type: "memory", title: "T", layout: "simple", opening_style: "light", visibility: "unlisted" },
      pages: [],
      processing_count: 1, failed_count: 1,
      images: [
        {
          image_id: "img-1",
          status: "processing",
          source_format: "jpeg",
          original_byte_size: 2_500_000,
          created_at: "2026-05-02T00:00:00Z",
        },
        {
          image_id: "img-2",
          status: "failed",
          source_format: "png",
          original_byte_size: 3_000_000,
          failure_reason: "decode_failed",
          created_at: "2026-05-02T00:00:01Z",
        },
      ],
    };
    vi.stubGlobal("fetch", vi.fn(async () => ({ status: 200, json: async () => body })));
    const got = await fetchEditView("00000000-0000-0000-0000-000000000001", "");
    expect(got.images).toHaveLength(2);
    expect(got.images[0].imageId).toBe("img-1");
    expect(got.images[0].status).toBe("processing");
    expect(got.images[0].originalByteSize).toBe(2_500_000);
    expect(got.images[1].failureReason).toBe("decode_failed");
  });

  it("正常_未知 status は failed に倒す（defensive）", async () => {
    const body = {
      photobook_id: "p", status: "draft", version: 0,
      settings: { type: "memory", title: "T", layout: "simple", opening_style: "light", visibility: "unlisted" },
      pages: [], processing_count: 0, failed_count: 0,
      images: [
        { image_id: "img-x", status: "purged", source_format: "jpeg", original_byte_size: 1, created_at: "2026-05-02T00:00:00Z" },
      ],
    };
    vi.stubGlobal("fetch", vi.fn(async () => ({ status: 200, json: async () => body })));
    const got = await fetchEditView("p", "");
    expect(got.images[0].status).toBe("failed");
  });

  it("正常_images 省略時は空配列に正規化", async () => {
    const body = {
      photobook_id: "p", status: "draft", version: 0,
      settings: { type: "memory", title: "T", layout: "simple", opening_style: "light", visibility: "unlisted" },
      pages: [], processing_count: 0, failed_count: 0,
    };
    vi.stubGlobal("fetch", vi.fn(async () => ({ status: 200, json: async () => body })));
    const got = await fetchEditView("p", "");
    expect(got.images).toEqual([]);
  });
});

describe("fetchEditViewClient (β-3 client polling)", () => {
  it("正常_credentials:include を渡し、Cookie ヘッダ手動転送はしない", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return {
        status: 200,
        json: async () => ({
          photobook_id: "p", status: "draft", version: 0,
          settings: { type: "memory", title: "T", layout: "simple", opening_style: "light", visibility: "unlisted" },
          pages: [], processing_count: 0, failed_count: 0,
          images: [],
        }),
      };
    }));
    await fetchEditViewClient("p");
    expect(calls).toHaveLength(1);
    const init = calls[0].init;
    expect(init.credentials).toBe("include");
    expect(init.method).toBe("GET");
    // headers に Cookie を手動で設定しない（browser が credentials:include で自動転送）
    const h = init.headers as Record<string, string> | undefined;
    if (h !== undefined) {
      expect(h.Cookie).toBeUndefined();
      expect(h.cookie).toBeUndefined();
    }
  });

  for (const tt of [
    { status: 401, kind: "unauthorized" },
    { status: 404, kind: "not_found" },
    { status: 500, kind: "server_error" },
  ] as const) {
    it(`異常_${tt.status}_kind_${tt.kind}`, async () => {
      vi.stubGlobal("fetch", vi.fn(async () => ({ status: tt.status, json: async () => ({}) })));
      try {
        await fetchEditViewClient("p");
      } catch (e) {
        expect(isEditApiError(e)).toBe(true);
        if (isEditApiError(e)) expect(e.kind).toBe(tt.kind);
      }
    });
  }

  it("異常_network 失敗で kind=network を返す", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new Error("offline"); }));
    try {
      await fetchEditViewClient("p");
    } catch (e) {
      expect(isEditApiError(e)).toBe(true);
      if (isEditApiError(e)) expect(e.kind).toBe("network");
    }
  });
});

describe("prepareAttachImages (β-3 P0-d)", () => {
  it("正常_POST + expected_version のみを body に出す", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return {
        status: 200,
        json: async () => ({ attached_count: 3, page_count: 1, skipped_count: 0 }),
      };
    }));
    const r = await prepareAttachImages("pb1", 7);
    expect(calls[0].url).toBe("https://api.test/api/photobooks/pb1/prepare/attach-images");
    expect(calls[0].init.method).toBe("POST");
    expect(calls[0].init.credentials).toBe("include");
    const body = JSON.parse(calls[0].init.body as string);
    expect(body).toEqual({ expected_version: 7 });
    expect(r.attachedCount).toBe(3);
    expect(r.pageCount).toBe(1);
    expect(r.skippedCount).toBe(0);
  });

  it("正常_count-only response で raw image_id を含まない", async () => {
    const SECRET_RAW_ID = "img-secret-raw-attach-zzz9999";
    const fetchSpy = vi.fn(async () => ({
      status: 200,
      json: async () => ({ attached_count: 1, page_count: 1, skipped_count: 0 }),
    }));
    vi.stubGlobal("fetch", fetchSpy);
    const r = await prepareAttachImages("pb1", 0);
    // 戻り値型が count-only であること（image_id 等の field を含まない）
    expect(Object.keys(r).sort()).toEqual(["attachedCount", "pageCount", "skippedCount"]);
    expect(JSON.stringify(r)).not.toContain(SECRET_RAW_ID);
  });

  for (const tt of [
    { status: 409, kind: "version_conflict" },
    { status: 401, kind: "unauthorized" },
    { status: 404, kind: "not_found" },
    { status: 400, kind: "bad_request" },
    { status: 500, kind: "server_error" },
    { status: 503, kind: "server_error" },
  ] as const) {
    it(`異常_${tt.status}_kind_${tt.kind}`, async () => {
      vi.stubGlobal("fetch", vi.fn(async () => ({ status: tt.status, json: async () => ({}) })));
      try {
        await prepareAttachImages("pb1", 0);
      } catch (e) {
        expect(isEditApiError(e)).toBe(true);
        if (isEditApiError(e)) expect(e.kind).toBe(tt.kind);
      }
    });
  }
});

describe("mutation API", () => {
  it("updatePhotoCaption_204成功", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => ({ status: 204 })));
    await expect(
      updatePhotoCaption("pb1", "ph1", "caption", 5),
    ).resolves.toBeUndefined();
  });

  it("updatePhotoCaption_409_throws_version_conflict", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => ({ status: 409, json: async () => ({}) })));
    try {
      await updatePhotoCaption("pb1", "ph1", "x", 5);
    } catch (e) {
      expect(isEditApiError(e)).toBe(true);
      if (isEditApiError(e)) expect(e.kind).toBe("version_conflict");
    }
  });

  it("bulkReorderPhotos_payload整形", async () => {
    const calls: any[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 204 };
    }));
    await bulkReorderPhotos(
      "pb1", "page1",
      [{ photoId: "ph1", displayOrder: 0 }, { photoId: "ph2", displayOrder: 1 }],
      3,
    );
    const call = calls[0];
    expect(call.init.method).toBe("PATCH");
    const body = JSON.parse(call.init.body);
    expect(body.page_id).toBe("page1");
    expect(body.assignments).toEqual([
      { photo_id: "ph1", display_order: 0 },
      { photo_id: "ph2", display_order: 1 },
    ]);
    expect(body.expected_version).toBe(3);
  });

  it("setCoverImage_payload", async () => {
    const calls: any[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 204 };
    }));
    await setCoverImage("pb1", "img1", 4);
    const body = JSON.parse(calls[0].init.body);
    expect(body.image_id).toBe("img1");
    expect(body.expected_version).toBe(4);
  });

  it("clearCoverImage_DELETE", async () => {
    const calls: any[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 204 };
    }));
    await clearCoverImage("pb1", 4);
    expect(calls[0].init.method).toBe("DELETE");
  });

  it("updatePhotobookSettings_optional_fields_to_null", async () => {
    const calls: any[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 204 };
    }));
    await updatePhotobookSettings(
      "pb1",
      { type: "memory", title: "t", layout: "simple", openingStyle: "light", visibility: "unlisted" },
      4,
    );
    const body = JSON.parse(calls[0].init.body);
    expect(body.description).toBe(null);
    expect(body.cover_title).toBe(null);
  });

  it("removePhoto_DELETE_with_page_id", async () => {
    const calls: any[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 204 };
    }));
    await removePhoto("pb1", "page1", "ph1", 3);
    expect(calls[0].init.method).toBe("DELETE");
    const body = JSON.parse(calls[0].init.body);
    expect(body.page_id).toBe("page1");
  });
});

// ============================================================================
// STOP P-4: m2-edit Phase A 5 mutation lib
// ----------------------------------------------------------------------------
// docs/plan/m2-edit-page-split-and-preview-plan.md §3.4 / §7.4
// ============================================================================

// B 方式 endpoint の response として返す EditView 型 fixture (snake_case API payload)。
// 各 test で fetch mock の response body に使う。
const editViewFixture = {
  photobook_id: "00000000-0000-0000-0000-000000000001",
  status: "draft",
  version: 8,
  settings: {
    type: "memory",
    title: "Title",
    layout: "simple",
    opening_style: "light",
    visibility: "unlisted",
  },
  pages: [
    {
      page_id: "page-1",
      display_order: 0,
      caption: "p1-cap",
      photos: [
        {
          photo_id: "photo-1",
          image_id: "image-1",
          display_order: 0,
          variants: {
            display: { url: "https://r.test/d1", width: 1600, height: 1200, expires_at: "2026-05-08T00:15:00Z" },
            thumbnail: { url: "https://r.test/t1", width: 480, height: 360, expires_at: "2026-05-08T00:15:00Z" },
          },
        },
      ],
    },
  ],
  processing_count: 0,
  failed_count: 0,
  images: [],
};

describe("updatePageCaption (A 方式)", () => {
  it("正常_PATCH_caption_と_expected_version_を_body_に出し_version_を返す", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 200, json: async () => ({ version: 9 }) };
    }));
    const r = await updatePageCaption("pb1", "page1", "hello", 8);
    expect(calls[0].url).toBe("https://api.test/api/photobooks/pb1/pages/page1/caption");
    expect(calls[0].init.method).toBe("PATCH");
    expect(calls[0].init.credentials).toBe("include");
    const body = JSON.parse(calls[0].init.body as string);
    expect(body).toEqual({ caption: "hello", expected_version: 8 });
    expect(r.version).toBe(9);
  });

  it("正常_caption_null_でクリア", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 200, json: async () => ({ version: 10 }) };
    }));
    await updatePageCaption("pb1", "page1", null, 9);
    const body = JSON.parse(calls[0].init.body as string);
    expect(body.caption).toBe(null);
  });

  for (const tt of [
    { status: 400, kind: "bad_request" },
    { status: 404, kind: "not_found" },
    { status: 409, kind: "version_conflict" },
    { status: 500, kind: "server_error" },
  ] as const) {
    it(`異常_${tt.status}_kind_${tt.kind}`, async () => {
      vi.stubGlobal("fetch", vi.fn(async () => ({ status: tt.status, json: async () => ({}) })));
      try {
        await updatePageCaption("pb1", "page1", "x", 0);
      } catch (e) {
        expect(isEditApiError(e)).toBe(true);
        if (isEditApiError(e)) expect(e.kind).toBe(tt.kind);
      }
    });
  }

  it("異常_network_失敗で_kind_network", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new Error("offline"); }));
    try {
      await updatePageCaption("pb1", "page1", "x", 0);
    } catch (e) {
      expect(isEditApiError(e)).toBe(true);
      if (isEditApiError(e)) expect(e.kind).toBe("network");
    }
  });
});

describe("splitPage (B 方式)", () => {
  it("正常_POST_photo_id_と_expected_version_を_body_に出し_EditView_を返す", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 200, json: async () => editViewFixture };
    }));
    const view = await splitPage("pb1", "page1", "ph1", 7);
    expect(calls[0].url).toBe("https://api.test/api/photobooks/pb1/pages/page1/split");
    expect(calls[0].init.method).toBe("POST");
    expect(calls[0].init.credentials).toBe("include");
    const body = JSON.parse(calls[0].init.body as string);
    expect(body).toEqual({ photo_id: "ph1", expected_version: 7 });
    // B 方式: EditView shape を返す (camelCase 化済)
    expect(view.version).toBe(8);
    expect(view.photobookId).toBe("00000000-0000-0000-0000-000000000001");
    expect(view.pages[0].photos[0].variants.display.width).toBe(1600);
    expect(view.pages[0].caption).toBe("p1-cap");
  });

  for (const tt of [
    { status: 400, kind: "bad_request" },
    { status: 404, kind: "not_found" },
    { status: 409, kind: "version_conflict" },
    { status: 401, kind: "unauthorized" },
    { status: 500, kind: "server_error" },
  ] as const) {
    it(`異常_${tt.status}_kind_${tt.kind}`, async () => {
      vi.stubGlobal("fetch", vi.fn(async () => ({ status: tt.status, json: async () => ({}) })));
      try {
        await splitPage("pb1", "page1", "ph1", 0);
      } catch (e) {
        expect(isEditApiError(e)).toBe(true);
        if (isEditApiError(e)) expect(e.kind).toBe(tt.kind);
      }
    });
  }
});

describe("movePhoto (B 方式)", () => {
  it("正常_PATCH_target_page_id_position_expected_version_を_body_に出し_EditView_を返す", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 200, json: async () => editViewFixture };
    }));
    const view = await movePhoto("pb1", "ph1", "page2", "end", 6);
    expect(calls[0].url).toBe("https://api.test/api/photobooks/pb1/photos/ph1/move");
    expect(calls[0].init.method).toBe("PATCH");
    expect(calls[0].init.credentials).toBe("include");
    const body = JSON.parse(calls[0].init.body as string);
    expect(body).toEqual({ target_page_id: "page2", position: "end", expected_version: 6 });
    expect(view.version).toBe(8);
  });

  it("正常_position_start_でも同じ_request_shape", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 200, json: async () => editViewFixture };
    }));
    await movePhoto("pb1", "ph1", "page2", "start", 6);
    const body = JSON.parse(calls[0].init.body as string);
    expect(body.position).toBe("start");
  });

  for (const tt of [
    { status: 400, kind: "bad_request" },
    { status: 409, kind: "version_conflict" },
    { status: 404, kind: "not_found" },
  ] as const) {
    it(`異常_${tt.status}_kind_${tt.kind}`, async () => {
      vi.stubGlobal("fetch", vi.fn(async () => ({ status: tt.status, json: async () => ({}) })));
      try {
        await movePhoto("pb1", "ph1", "page2", "end", 0);
      } catch (e) {
        expect(isEditApiError(e)).toBe(true);
        if (isEditApiError(e)) expect(e.kind).toBe(tt.kind);
      }
    });
  }
});

describe("mergePages (B 方式)", () => {
  it("正常_POST_expected_version_のみを_body_に出し_EditView_を返す", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 200, json: async () => editViewFixture };
    }));
    const view = await mergePages("pb1", "pageSrc", "pageTgt", 5);
    expect(calls[0].url).toBe("https://api.test/api/photobooks/pb1/pages/pageSrc/merge-into/pageTgt");
    expect(calls[0].init.method).toBe("POST");
    expect(calls[0].init.credentials).toBe("include");
    const body = JSON.parse(calls[0].init.body as string);
    expect(body).toEqual({ expected_version: 5 });
    expect(view.pages).toHaveLength(1);
  });

  for (const tt of [
    { status: 400, kind: "bad_request" },
    { status: 409, kind: "version_conflict" },
    { status: 404, kind: "not_found" },
  ] as const) {
    it(`異常_${tt.status}_kind_${tt.kind}`, async () => {
      vi.stubGlobal("fetch", vi.fn(async () => ({ status: tt.status, json: async () => ({}) })));
      try {
        await mergePages("pb1", "pageSrc", "pageTgt", 0);
      } catch (e) {
        expect(isEditApiError(e)).toBe(true);
        if (isEditApiError(e)) expect(e.kind).toBe(tt.kind);
      }
    });
  }
});

describe("reorderPages (B 方式)", () => {
  it("正常_PATCH_assignments_を_snake_case_に整形し_EditView_を返す", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 200, json: async () => editViewFixture };
    }));
    const view = await reorderPages(
      "pb1",
      [
        { pageId: "pageC", displayOrder: 0 },
        { pageId: "pageA", displayOrder: 1 },
        { pageId: "pageB", displayOrder: 2 },
      ],
      4,
    );
    expect(calls[0].url).toBe("https://api.test/api/photobooks/pb1/pages/reorder");
    expect(calls[0].init.method).toBe("PATCH");
    expect(calls[0].init.credentials).toBe("include");
    const body = JSON.parse(calls[0].init.body as string);
    expect(body).toEqual({
      assignments: [
        { page_id: "pageC", display_order: 0 },
        { page_id: "pageA", display_order: 1 },
        { page_id: "pageB", display_order: 2 },
      ],
      expected_version: 4,
    });
    expect(view.version).toBe(8);
  });

  for (const tt of [
    { status: 400, kind: "bad_request" },
    { status: 409, kind: "version_conflict" },
    { status: 404, kind: "not_found" },
  ] as const) {
    it(`異常_${tt.status}_kind_${tt.kind}`, async () => {
      vi.stubGlobal("fetch", vi.fn(async () => ({ status: tt.status, json: async () => ({}) })));
      try {
        await reorderPages("pb1", [{ pageId: "p", displayOrder: 0 }], 0);
      } catch (e) {
        expect(isEditApiError(e)).toBe(true);
        if (isEditApiError(e)) expect(e.kind).toBe(tt.kind);
      }
    });
  }
});

describe("error body raw 値非露出 (defensive)", () => {
  // EditApiError は kind だけを露出する (raw error body の reason / 内部詳細を含まない)。
  // P-2 / P-3 で Backend は reason 付き body を返すが、Frontend lib では kind に丸める。
  // reason 別 UI 文言は EditClient 側 (P-5/P-6) で handler 化する想定。
  it("正常_409_reason_付き_body_でも_kind_だけが_throw_される", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => ({
      status: 409,
      json: async () => ({ status: "version_conflict", reason: "merge_into_self" }),
    })));
    try {
      await mergePages("pb1", "pgA", "pgA", 0);
    } catch (e) {
      expect(isEditApiError(e)).toBe(true);
      if (isEditApiError(e)) {
        expect(e.kind).toBe("version_conflict");
        // reason / status などの raw key は含まない (kind のみ)
        expect(Object.keys(e)).toEqual(["kind"]);
      }
    }
  });
});
