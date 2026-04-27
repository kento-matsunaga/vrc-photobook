// publicPhotobook API client の unit test。
//
// セキュリティ:
//   - 実 token / 実 presigned URL / 実 storage_key は使わない
//   - エラー分類は kind のみ確認
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  fetchPublicPhotobook,
  isPublicLookupError,
  type PublicLookupError,
} from "@/lib/publicPhotobook";

const ORIGINAL_API = process.env.NEXT_PUBLIC_API_BASE_URL;

beforeEach(() => {
  process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test";
});

afterEach(() => {
  vi.unstubAllGlobals();
  process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
});

describe("fetchPublicPhotobook", () => {
  const tests: Array<{
    name: string;
    description: string;
    status: number;
    body?: unknown;
    wantKind?: PublicLookupError["kind"];
    wantOK?: boolean;
  }> = [
    {
      name: "正常_200で_payloadをcamelCase化",
      description: "Given: 200 + snake_case payload, When: fetch, Then: camelCase に変換して返る",
      status: 200,
      body: {
        type: "event",
        title: "Sample",
        layout: "simple",
        opening_style: "light",
        creator_display_name: "alice",
        published_at: "2026-01-01T00:00:00Z",
        pages: [
          {
            photos: [
              {
                variants: {
                  display: { url: "https://r.test/d", width: 1600, height: 1200, expires_at: "2026-01-01T00:15:00Z" },
                  thumbnail: { url: "https://r.test/t", width: 480, height: 360, expires_at: "2026-01-01T00:15:00Z" },
                },
              },
            ],
          },
        ],
      },
      wantOK: true,
    },
    {
      name: "異常_404でnot_found",
      description: "Given: 404, Then: throw {kind:'not_found'}",
      status: 404,
      wantKind: "not_found",
    },
    {
      name: "異常_410でgone",
      description: "Given: 410, Then: throw {kind:'gone'}",
      status: 410,
      wantKind: "gone",
    },
    {
      name: "異常_500でserver_error",
      description: "Given: 500, Then: throw {kind:'server_error'}",
      status: 500,
      wantKind: "server_error",
    },
  ];

  for (const tt of tests) {
    it(tt.name, async () => {
      const mockFetch = vi.fn(async () => ({
        status: tt.status,
        json: async () => tt.body,
      }));
      vi.stubGlobal("fetch", mockFetch);

      if (tt.wantOK) {
        const got = await fetchPublicPhotobook("ok12pp34zz56gh78");
        expect(got.title).toBe("Sample");
        expect(got.openingStyle).toBe("light");
        expect(got.pages[0].photos[0].variants.display.width).toBe(1600);
      } else {
        await expect(fetchPublicPhotobook("xx12pp34zz56gh78")).rejects.toMatchObject({
          kind: tt.wantKind,
        });
      }
    });
  }

  it("異常_network失敗_kindはnetwork", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new Error("connection refused");
      }),
    );
    try {
      await fetchPublicPhotobook("nn12pp34zz56gh78");
    } catch (e) {
      expect(isPublicLookupError(e)).toBe(true);
      if (isPublicLookupError(e)) {
        expect(e.kind).toBe("network");
      }
    }
  });

  it("異常_NEXT_PUBLIC_API_BASE_URL未設定でthrow", async () => {
    delete process.env.NEXT_PUBLIC_API_BASE_URL;
    await expect(fetchPublicPhotobook("ee12pp34zz56gh78")).rejects.toThrow(
      "NEXT_PUBLIC_API_BASE_URL",
    );
  });
});
