// createPhotobook API client のテスト。
//
// 観点:
//   - L3 trim guard（空 / 空白のみで fetch せず即 reject）
//   - API base URL（NEXT_PUBLIC_API_BASE_URL 経由で Backend に POST）
//   - 成功 path（response の draft_edit_url_path を返す）
//   - error mapping（400 / 403 / 503 / その他 / network）
//   - response の draft_edit_token は **本関数では返さない**（呼び出し側で window.location.replace 想定）

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  createPhotobook,
  isCreatePhotobookError,
  type CreatePhotobookError,
} from "@/lib/createPhotobook";

const ORIGINAL_API = process.env.NEXT_PUBLIC_API_BASE_URL;

describe("createPhotobook L3 trim guard", () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test";
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
  });

  it("正常_空文字tokenでfetch呼ばずturnstile_failedをreject", async () => {
    await expect(
      createPhotobook({ type: "memory", turnstileToken: "" }),
    ).rejects.toMatchObject({ kind: "turnstile_failed" });
    expect(fetch as unknown as ReturnType<typeof vi.fn>).not.toHaveBeenCalled();
  });

  it("正常_空白のみtokenでfetch呼ばずturnstile_failed", async () => {
    await expect(
      createPhotobook({ type: "memory", turnstileToken: "   " }),
    ).rejects.toMatchObject({ kind: "turnstile_failed" });
    expect(fetch as unknown as ReturnType<typeof vi.fn>).not.toHaveBeenCalled();
  });

  it("正常_タブ改行のみでfetch呼ばずturnstile_failed", async () => {
    await expect(
      createPhotobook({ type: "memory", turnstileToken: "\t\n" }),
    ).rejects.toMatchObject({ kind: "turnstile_failed" });
    expect(fetch as unknown as ReturnType<typeof vi.fn>).not.toHaveBeenCalled();
  });
});

describe("createPhotobook success path", () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test";
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
  });

  it("正常_201で_draftEditUrlPath_と_draftExpiresAt_を返す", async () => {
    (fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: async () => ({
        photobook_id: "test-id",
        draft_edit_token: "test-token",
        draft_edit_url_path: "/draft/test-token",
        draft_expires_at: "2026-05-08T12:34:56Z",
      }),
    });
    const out = await createPhotobook({
      type: "memory",
      title: "テスト",
      creatorDisplayName: "テスター",
      turnstileToken: "valid-token",
    });
    expect(out.draftEditUrlPath).toBe("/draft/test-token");
    expect(out.draftExpiresAt).toBe("2026-05-08T12:34:56Z");
  });

  it("正常_NEXT_PUBLIC_API_BASE_URL先のapi_photobooksにPOSTする", async () => {
    const mockFetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: async () => ({
        photobook_id: "id-1",
        draft_edit_token: "tok-1",
        draft_edit_url_path: "/draft/tok-1",
        draft_expires_at: "2026-05-08T00:00:00Z",
      }),
    });
    vi.stubGlobal("fetch", mockFetch);

    await createPhotobook({ type: "memory", turnstileToken: "valid-token" });

    expect(mockFetch).toHaveBeenCalledTimes(1);
    const [calledUrl, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(calledUrl).toBe("https://api.test/api/photobooks");
    expect(init.method).toBe("POST");
    expect(init.cache).toBe("no-store");
  });

  it("正常_NEXT_PUBLIC_API_BASE_URL末尾スラッシュは除去されてfetchされる", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test/";
    const mockFetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: async () => ({
        draft_edit_url_path: "/draft/tok-2",
        draft_expires_at: "2026-05-08T00:00:00Z",
      }),
    });
    vi.stubGlobal("fetch", mockFetch);

    await createPhotobook({ type: "memory", turnstileToken: "valid-token" });

    const [calledUrl] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(calledUrl).toBe("https://api.test/api/photobooks");
  });

  it("異常_NEXT_PUBLIC_API_BASE_URL未設定でErrorをthrow", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "";
    const mockFetch = vi.fn();
    vi.stubGlobal("fetch", mockFetch);

    await expect(
      createPhotobook({ type: "memory", turnstileToken: "valid-token" }),
    ).rejects.toThrow("NEXT_PUBLIC_API_BASE_URL is not set");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("正常_response_に_draft_edit_url_path_が無い場合_server_error", async () => {
    (fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: async () => ({}),
    });
    await expect(
      createPhotobook({ type: "memory", turnstileToken: "valid-token" }),
    ).rejects.toMatchObject({ kind: "server_error" });
  });
});

describe("createPhotobook error mapping", () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test";
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
  });

  const cases: Array<{ status: number; want: CreatePhotobookError["kind"] }> = [
    { status: 400, want: "invalid_payload" },
    { status: 403, want: "turnstile_failed" },
    { status: 503, want: "turnstile_unavailable" },
    { status: 500, want: "server_error" },
    { status: 502, want: "server_error" },
  ];

  for (const c of cases) {
    it(`正常_HTTP_${c.status}で${c.want}にマップ`, async () => {
      (fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
        ok: false,
        status: c.status,
        json: async () => ({ status: "error" }),
      });
      await expect(
        createPhotobook({ type: "memory", turnstileToken: "valid-token" }),
      ).rejects.toMatchObject({ kind: c.want });
    });
  }

  it("異常_fetch_throwでnetwork", async () => {
    (fetch as unknown as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new Error("network down"),
    );
    await expect(
      createPhotobook({ type: "memory", turnstileToken: "valid-token" }),
    ).rejects.toMatchObject({ kind: "network" });
  });
});

describe("isCreatePhotobookError type guard", () => {
  it("正常_kind_string_を持つobjectをtrue判定", () => {
    expect(isCreatePhotobookError({ kind: "turnstile_failed" })).toBe(true);
  });

  it("異常_kind無し_kind非string_null_undefinedはfalse", () => {
    expect(isCreatePhotobookError(null)).toBe(false);
    expect(isCreatePhotobookError(undefined)).toBe(false);
    expect(isCreatePhotobookError({})).toBe(false);
    expect(isCreatePhotobookError({ kind: 123 })).toBe(false);
    expect(isCreatePhotobookError("string")).toBe(false);
  });
});
