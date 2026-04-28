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
