// publishPhotobook API client の unit test。
//
// 2026-05-03 STOP α P0 v2: rights_agreed フラグ + 409 response shape 分離
// (version_conflict / publish_precondition_failed) に対応。
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
  it("正常_200_payloadをcamelCase化_rights_agreedを送信", async () => {
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
    const got = await publishPhotobook("00000000-0000-0000-0000-000000000001", 3, true);
    expect(got.publicUrlPath).toBe("/p/ok12pp34zz56gh78");
    expect(got.manageUrlPath).toBe("/manage/token/REDACTED_FOR_TEST");
    expect(got.slug).toBe("ok12pp34zz56gh78");
    // request body: expected_version + rights_agreed=true
    const reqBody = JSON.parse(calls[0].init.body);
    expect(reqBody.expected_version).toBe(3);
    expect(reqBody.rights_agreed).toBe(true);
    expect(calls[0].init.credentials).toBe("include");
  });

  it("正常_rights_agreed=falseもbodyに送る", async () => {
    const calls: any[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return {
        status: 409,
        clone: () => ({
          json: async () => ({ status: "publish_precondition_failed", reason: "rights_not_agreed" }),
        }),
      };
    }));
    try {
      await publishPhotobook("pb1", 1, false);
    } catch {
      // ignore
    }
    const reqBody = JSON.parse(calls[0].init.body);
    expect(reqBody.rights_agreed).toBe(false);
  });

  for (const tt of [
    { status: 401, kind: "unauthorized" },
    { status: 404, kind: "not_found" },
    { status: 400, kind: "bad_request" },
    { status: 500, kind: "server_error" },
  ] as const) {
    it(`異常_${tt.status}_kind_${tt.kind}`, async () => {
      vi.stubGlobal("fetch", vi.fn(async () => ({ status: tt.status, json: async () => ({}) })));
      try {
        await publishPhotobook("pb1", 1, true);
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

  it("異常_409_version_conflictをparse", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 409,
        clone: () => ({
          json: async () => ({ status: "version_conflict" }),
        }),
      })),
    );
    try {
      await publishPhotobook("pb1", 1, true);
      throw new Error("should have thrown");
    } catch (e) {
      expect(isPublishApiError(e)).toBe(true);
      if (isPublishApiError(e)) {
        expect(e.kind).toBe("version_conflict");
      }
    }
  });

  for (const reason of [
    "rights_not_agreed",
    "not_draft",
    "empty_creator",
    "empty_title",
  ] as const) {
    it(`異常_409_publish_precondition_failed_reason_${reason}`, async () => {
      vi.stubGlobal(
        "fetch",
        vi.fn(async () => ({
          status: 409,
          clone: () => ({
            json: async () => ({ status: "publish_precondition_failed", reason }),
          }),
        })),
      );
      try {
        await publishPhotobook("pb1", 1, false);
        throw new Error("should have thrown");
      } catch (e) {
        expect(isPublishApiError(e)).toBe(true);
        if (isPublishApiError(e) && e.kind === "publish_precondition_failed") {
          expect(e.reason).toBe(reason);
        } else {
          throw new Error(`expected publish_precondition_failed got ${JSON.stringify(e)}`);
        }
      }
    });
  }

  it("異常_409_未知reasonはunknown_preconditionにフォールバック", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 409,
        clone: () => ({
          json: async () => ({ status: "publish_precondition_failed", reason: "未定義" }),
        }),
      })),
    );
    try {
      await publishPhotobook("pb1", 1, true);
      throw new Error("should have thrown");
    } catch (e) {
      if (isPublishApiError(e) && e.kind === "publish_precondition_failed") {
        expect(e.reason).toBe("unknown_precondition");
      } else {
        throw new Error("kind mismatch");
      }
    }
  });

  it("異常_409_status不在bodyはversion_conflictに既定", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 409,
        clone: () => ({
          json: async () => ({}),
        }),
      })),
    );
    try {
      await publishPhotobook("pb1", 1, true);
      throw new Error("should have thrown");
    } catch (e) {
      if (isPublishApiError(e)) {
        expect(e.kind).toBe("version_conflict");
      }
    }
  });

  it("異常_network失敗", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new Error("net"); }));
    try {
      await publishPhotobook("pb1", 1, true);
      throw new Error("should have thrown");
    } catch (e) {
      expect(isPublishApiError(e)).toBe(true);
      if (isPublishApiError(e)) expect(e.kind).toBe("network");
    }
  });

  // PR36 commit 4: 429 rate_limited mapping
  it("異常_429でrate_limited_RetryAfter優先", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(JSON.stringify({ status: "rate_limited", retry_after_seconds: 1 }), {
            status: 429,
            headers: { "Retry-After": "3600", "Content-Type": "application/json" },
          }),
      ),
    );
    try {
      await publishPhotobook("pid", 0, true);
      throw new Error("should have thrown");
    } catch (e) {
      if (isPublishApiError(e) && e.kind === "rate_limited") {
        expect(e.retryAfterSeconds).toBe(3600);
      } else {
        throw new Error("not rate_limited");
      }
    }
  });

  it("異常_429でheader無しはbody fallback", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(JSON.stringify({ status: "rate_limited", retry_after_seconds: 90 }), {
            status: 429,
            headers: { "Content-Type": "application/json" },
          }),
      ),
    );
    try {
      await publishPhotobook("pid", 0, true);
      throw new Error("should have thrown");
    } catch (e) {
      if (isPublishApiError(e) && e.kind === "rate_limited") {
        expect(e.retryAfterSeconds).toBe(90);
      } else {
        throw new Error("not rate_limited");
      }
    }
  });
});
