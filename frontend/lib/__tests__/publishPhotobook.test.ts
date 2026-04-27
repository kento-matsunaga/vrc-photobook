// publishPhotobook API client の unit test。
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  isPublishApiError,
  publishPhotobook,
  type PublishApiError,
} from "@/lib/publishPhotobook";

const ORIGINAL_API = process.env.NEXT_PUBLIC_API_BASE_URL;

beforeEach(() => {
  process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test";
});

afterEach(() => {
  vi.unstubAllGlobals();
  process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
});

describe("publishPhotobook", () => {
  it("正常_200_payloadをcamelCase化", async () => {
    const body = {
      photobook_id: "00000000-0000-0000-0000-000000000001",
      slug: "ok12pp34zz56gh78",
      public_url_path: "/p/ok12pp34zz56gh78",
      manage_url_path: "/manage/token/REDACTED_FOR_TEST",
      published_at: "2026-01-01T00:00:00Z",
    };
    const calls: any[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return { status: 200, json: async () => body };
    }));
    const got = await publishPhotobook("00000000-0000-0000-0000-000000000001", 3);
    expect(got.publicUrlPath).toBe("/p/ok12pp34zz56gh78");
    expect(got.manageUrlPath).toBe("/manage/token/REDACTED_FOR_TEST");
    expect(got.slug).toBe("ok12pp34zz56gh78");
    // request body
    const reqBody = JSON.parse(calls[0].init.body);
    expect(reqBody.expected_version).toBe(3);
  });

  for (const tt of [
    { status: 401, kind: "unauthorized" },
    { status: 404, kind: "not_found" },
    { status: 400, kind: "bad_request" },
    { status: 409, kind: "version_conflict" },
    { status: 500, kind: "server_error" },
  ] as const) {
    it(`異常_${tt.status}_kind_${tt.kind}`, async () => {
      vi.stubGlobal("fetch", vi.fn(async () => ({ status: tt.status, json: async () => ({}) })));
      try {
        await publishPhotobook("pb1", 1);
        throw new Error("should have thrown");
      } catch (e) {
        expect(isPublishApiError(e)).toBe(true);
        if (isPublishApiError(e)) {
          const err: PublishApiError = e;
          expect(err.kind).toBe(tt.kind);
        }
      }
    });
  }

  it("異常_network失敗", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new Error("net"); }));
    try {
      await publishPhotobook("pb1", 1);
      throw new Error("should have thrown");
    } catch (e) {
      expect(isPublishApiError(e)).toBe(true);
      if (isPublishApiError(e)) expect(e.kind).toBe("network");
    }
  });
});
