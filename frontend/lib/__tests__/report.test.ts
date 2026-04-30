// report API client の unit test。
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  REPORT_REASONS,
  isSubmitReportError,
  submitReport,
  type SubmitReportError,
} from "@/lib/report";

const ORIGINAL_API = process.env.NEXT_PUBLIC_API_BASE_URL;

beforeEach(() => {
  process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test";
});

afterEach(() => {
  vi.unstubAllGlobals();
  process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
});

describe("submitReport", () => {
  const tests: Array<{
    name: string;
    description: string;
    status: number;
    wantKind?: SubmitReportError["kind"];
    wantOK?: boolean;
  }> = [
    {
      name: "正常_201で例外なし",
      description: "Given: 201, When: submitReport, Then: resolve",
      status: 201,
      wantOK: true,
    },
    {
      name: "異常_400でinvalid_payload",
      description: "Given: 400, When: submitReport, Then: invalid_payload",
      status: 400,
      wantKind: "invalid_payload",
    },
    {
      name: "異常_403でturnstile_failed",
      description: "Given: 403, When: submitReport, Then: turnstile_failed",
      status: 403,
      wantKind: "turnstile_failed",
    },
    {
      name: "異常_404でnot_found",
      description: "Given: 404, When: submitReport, Then: not_found",
      status: 404,
      wantKind: "not_found",
    },
    {
      name: "異常_500でserver_error",
      description: "Given: 500, When: submitReport, Then: server_error",
      status: 500,
      wantKind: "server_error",
    },
  ];

  for (const tt of tests) {
    it(tt.name, async () => {
      const mockFetch = vi.fn(async () => ({
        status: tt.status,
        json: async () => ({}),
      }));
      vi.stubGlobal("fetch", mockFetch);

      if (tt.wantOK) {
        await expect(
          submitReport({
            slug: "uqfwfti7glarva5saj",
            reason: "harassment_or_doxxing",
            turnstileToken: "ts-token-test",
          }),
        ).resolves.toBeUndefined();
        // POST body / URL 検証
        const calls = mockFetch.mock.calls as unknown as Array<[string, RequestInit]>;
        const url = calls[0][0];
        expect(url).toBe(
          "https://api.test/api/public/photobooks/uqfwfti7glarva5saj/reports",
        );
        const init = calls[0][1];
        expect(init.method).toBe("POST");
        const body = JSON.parse(init.body as string) as {
          reason: string;
          detail: string;
          reporter_contact: string;
          turnstile_token: string;
        };
        expect(body.reason).toBe("harassment_or_doxxing");
        expect(body.turnstile_token).toBe("ts-token-test");
        // contact / detail は未指定なら空文字（Backend は空文字 → None VO）
        expect(body.detail).toBe("");
        expect(body.reporter_contact).toBe("");
      } else {
        await expect(
          submitReport({
            slug: "uqfwfti7glarva5saj",
            reason: "other",
            turnstileToken: "ts-token-test",
          }),
        ).rejects.toMatchObject({ kind: tt.wantKind });
      }
    });
  }

  // L3 多層防御 Turnstile ガード。`.agents/rules/turnstile-defensive-guard.md`。
  describe("L3_Turnstile_defensive_guard", () => {
    const guardCases: Array<{ name: string; description: string; token: string }> = [
      {
        name: "異常_空文字tokenでturnstile_failed_fetch呼ばれない",
        description: "Given: turnstileToken='', When: submitReport, Then: turnstile_failed",
        token: "",
      },
      {
        name: "異常_空白のみtokenでturnstile_failed_fetch呼ばれない",
        description: "Given: turnstileToken=' ', When: submitReport, Then: turnstile_failed",
        token: "   ",
      },
      {
        name: "異常_タブ改行のみtokenでturnstile_failed_fetch呼ばれない",
        description: "Given: turnstileToken='\\t\\n', When: submitReport, Then: turnstile_failed",
        token: "\t\n",
      },
    ];
    for (const tt of guardCases) {
      it(tt.name, async () => {
        const mockFetch = vi.fn(async () => ({ status: 201, json: async () => ({}) }));
        vi.stubGlobal("fetch", mockFetch);
        await expect(
          submitReport({
            slug: "uqfwfti7glarva5saj",
            reason: "harassment_or_doxxing",
            turnstileToken: tt.token,
          }),
        ).rejects.toMatchObject({ kind: "turnstile_failed" });
        // fetch は 1 度も呼ばれていない（Backend に whitespace token を送っていない）
        expect(mockFetch).not.toHaveBeenCalled();
      });
    }
  });

  it("異常_network失敗_kindはnetwork", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new Error("dns fail");
      }),
    );
    try {
      await submitReport({
        slug: "uqfwfti7glarva5saj",
        reason: "other",
        turnstileToken: "ts-token-test",
      });
      throw new Error("should have thrown");
    } catch (e) {
      expect(isSubmitReportError(e)).toBe(true);
      if (isSubmitReportError(e)) {
        expect(e.kind).toBe("network");
      }
    }
  });

  it("正常_detail_contact_あり_でPOST_bodyに含まれる", async () => {
    const mockFetch = vi.fn(async () => ({
      status: 201,
      json: async () => ({}),
    }));
    vi.stubGlobal("fetch", mockFetch);
    await submitReport({
      slug: "uqfwfti7glarva5saj",
      reason: "minor_safety_concern",
      detail: "テスト本文",
      reporterContact: "user@example.test",
      turnstileToken: "ts-token-test",
    });
    const calls = mockFetch.mock.calls as unknown as Array<[string, RequestInit]>;
    const init = calls[0][1];
    const body = JSON.parse(init.body as string) as {
      detail: string;
      reporter_contact: string;
    };
    expect(body.detail).toBe("テスト本文");
    expect(body.reporter_contact).toBe("user@example.test");
  });
});

describe("REPORT_REASONS", () => {
  it("正常_6種_minor_safety_concern含む", () => {
    expect(REPORT_REASONS).toHaveLength(6);
    const values = REPORT_REASONS.map((r) => r.value);
    expect(values).toContain("minor_safety_concern");
    expect(values).toContain("harassment_or_doxxing");
    expect(values).toContain("unauthorized_repost");
    expect(values).toContain("subject_removal_request");
    expect(values).toContain("sensitive_flag_missing");
    expect(values).toContain("other");
  });
});

// PR36 commit 4: 429 rate_limited mapping。
describe("submitReport_429_RateLimited", () => {
  function build429Response(retryAfterHeader: string | null, body: unknown): Response {
    const init: ResponseInit = {
      status: 429,
      headers: retryAfterHeader !== null ? { "Retry-After": retryAfterHeader, "Content-Type": "application/json" } : { "Content-Type": "application/json" },
    };
    return new Response(JSON.stringify(body), init);
  }

  it("正常_Retry-After_header優先で秒数を抽出", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => build429Response("3600", { status: "rate_limited", retry_after_seconds: 999 })),
    );
    try {
      await submitReport({
        slug: "uqfwfti7glarva5saj",
        reason: "harassment_or_doxxing",
        turnstileToken: "ts-token",
      });
      throw new Error("should have thrown");
    } catch (e) {
      expect(isSubmitReportError(e)).toBe(true);
      if (isSubmitReportError(e) && e.kind === "rate_limited") {
        expect(e.retryAfterSeconds).toBe(3600);
      } else {
        throw new Error(`expected rate_limited got ${(e as { kind?: string }).kind}`);
      }
    }
  });

  it("正常_Retry-After無し時はbody.retry_after_secondsを使う", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => build429Response(null, { status: "rate_limited", retry_after_seconds: 120 })),
    );
    try {
      await submitReport({
        slug: "uqfwfti7glarva5saj",
        reason: "other",
        turnstileToken: "ts-token",
      });
      throw new Error("should have thrown");
    } catch (e) {
      if (isSubmitReportError(e) && e.kind === "rate_limited") {
        expect(e.retryAfterSeconds).toBe(120);
      } else {
        throw new Error("not rate_limited");
      }
    }
  });

  it("正常_Retry-Afterもbodyも不正なら既定60秒", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => build429Response(null, { status: "rate_limited" })),
    );
    try {
      await submitReport({
        slug: "uqfwfti7glarva5saj",
        reason: "other",
        turnstileToken: "ts-token",
      });
      throw new Error("should have thrown");
    } catch (e) {
      if (isSubmitReportError(e) && e.kind === "rate_limited") {
        expect(e.retryAfterSeconds).toBe(60);
      } else {
        throw new Error("not rate_limited");
      }
    }
  });

  it("正常_scope_hash_count_limitがbodyに無くても動く", async () => {
    // 漏洩防止: body に scope_hash / count / limit が無いのが正常。
    // mapping は status だけで判断できる。
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => build429Response("60", { status: "rate_limited", retry_after_seconds: 60 })),
    );
    try {
      await submitReport({
        slug: "uqfwfti7glarva5saj",
        reason: "other",
        turnstileToken: "ts-token",
      });
      throw new Error("should have thrown");
    } catch (e) {
      if (isSubmitReportError(e) && e.kind === "rate_limited") {
        expect(e.retryAfterSeconds).toBe(60);
      } else {
        throw new Error("not rate_limited");
      }
    }
  });
});
