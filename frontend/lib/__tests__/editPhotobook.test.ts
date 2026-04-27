// editPhotobook API client の unit test。
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  bulkReorderPhotos,
  clearCoverImage,
  fetchEditView,
  isEditApiError,
  removePhoto,
  setCoverImage,
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
