// frontend/lib/upload.ts の unit test（Vitest）。
//
// 実 Backend 接続は行わず、global.fetch を mock で差し替えて 4 つの主要エラー分類を
// 検証する。
//
// セキュリティ:
//   - 実 token / 実 presigned URL / 実 Cookie 値はテストでも使わない
//   - 検証は kind / fetch 呼び出し回数 / URL 形式のみ

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  completeUpload,
  issueUploadIntent,
  issueUploadVerification,
  putToR2,
  sourceFormatOf,
  validateFile,
  type UploadError,
} from "@/lib/upload";

const ORIGINAL_API = process.env.NEXT_PUBLIC_API_BASE_URL;

beforeEach(() => {
  process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test";
});

afterEach(() => {
  vi.unstubAllGlobals();
  process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
});

function makeFile(size: number, type: string, name = "x.jpg"): File {
  const bytes = new Uint8Array(size);
  return new File([bytes], name, { type });
}

describe("validateFile", () => {
  it("正常_jpeg_1KB", () => {
    expect(validateFile(makeFile(1024, "image/jpeg"))).toBeNull();
  });
  it("正常_heic_1KB", () => {
    expect(validateFile(makeFile(1024, "image/heic"))).toBeNull();
  });
  it("正常_heic_拡張子fallback_typeなし", () => {
    expect(validateFile(makeFile(1024, "", "x.heic"))).toBeNull();
  });
  it("異常_svg_拒否", () => {
    expect(validateFile(makeFile(1024, "image/svg+xml"))).toEqual({ kind: "invalid_type" });
  });
  it("異常_html_拒否", () => {
    expect(validateFile(makeFile(1024, "text/html"))).toEqual({ kind: "invalid_type" });
  });
  it("異常_10MB+1_拒否", () => {
    expect(validateFile(makeFile(10 * 1024 * 1024 + 1, "image/jpeg"))).toEqual({ kind: "too_large" });
  });
});

describe("sourceFormatOf", () => {
  it("image/jpeg => jpg", () => expect(sourceFormatOf("image/jpeg")).toBe("jpg"));
  it("image/png => png", () => expect(sourceFormatOf("image/png")).toBe("png"));
  it("image/webp => webp", () => expect(sourceFormatOf("image/webp")).toBe("webp"));
  it("image/heic => heic", () => expect(sourceFormatOf("image/heic")).toBe("heic"));
  it("image/svg+xml => null", () => expect(sourceFormatOf("image/svg+xml")).toBeNull());
});

describe("issueUploadVerification", () => {
  it("正常_201で response を返す", async () => {
    const mockFetch = vi.fn(async () =>
      new Response(JSON.stringify({
        upload_verification_token: "x".repeat(43),
        expires_at: "2026-04-27T00:00:00Z",
        allowed_intent_count: 20,
      }), { status: 201, headers: { "Content-Type": "application/json" } }),
    );
    vi.stubGlobal("fetch", mockFetch);

    const got = await issueUploadVerification("pid", "turnstile-token");
    expect(got.allowedIntentCount).toBe(20);
    expect(got.uploadVerificationToken.length).toBe(43);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    const calls = mockFetch.mock.calls as unknown as Array<[string, RequestInit | undefined]>;
    expect(calls[0][0]).toBe("https://api.test/api/photobooks/pid/upload-verifications/");
  });

  it("異常_403で verification_failed", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("{}", { status: 403 })));
    await expect(issueUploadVerification("pid", "x")).rejects.toMatchObject({ kind: "verification_failed" } satisfies UploadError);
  });

  it("異常_503で turnstile_unavailable", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("{}", { status: 503 })));
    await expect(issueUploadVerification("pid", "x")).rejects.toMatchObject({ kind: "turnstile_unavailable" } satisfies UploadError);
  });

  it("異常_network", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new Error("network down"); }));
    await expect(issueUploadVerification("pid", "x")).rejects.toMatchObject({ kind: "network" } satisfies UploadError);
  });
});

describe("issueUploadIntent", () => {
  it("正常_201で response を返す", async () => {
    const mockFetch = vi.fn(async () =>
      new Response(JSON.stringify({
        image_id: "iid",
        upload_url: "https://r2.test/...",
        required_headers: { "Content-Type": "image/jpeg" },
        storage_key: "photobooks/pid/images/iid/original/xxx.jpg",
        expires_at: "2026-04-27T00:00:00Z",
      }), { status: 201 }),
    );
    vi.stubGlobal("fetch", mockFetch);

    const got = await issueUploadIntent("pid", "uv-token", "image/jpeg", 1024, "jpg");
    expect(got.imageId).toBe("iid");
    expect(got.requiredHeaders["Content-Type"]).toBe("image/jpeg");
    const calls = mockFetch.mock.calls as unknown as Array<[string, RequestInit | undefined]>;
    const init = calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>)["Authorization"]).toBe("Bearer uv-token");
  });

  it("異常_400で invalid_parameters", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("{}", { status: 400 })));
    await expect(issueUploadIntent("pid", "x", "image/jpeg", 1, "jpg")).rejects.toMatchObject({ kind: "invalid_parameters" } satisfies UploadError);
  });
});

describe("putToR2", () => {
  it("正常_PUT_200", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("", { status: 200 })));
    await expect(putToR2("https://r2.test/upload", "image/jpeg", new Blob([new Uint8Array(1024)]))).resolves.toBeUndefined();
  });
  it("異常_403で upload_failed", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("", { status: 403 })));
    await expect(putToR2("https://r2.test/upload", "image/jpeg", new Blob([new Uint8Array(1024)]))).rejects.toMatchObject({ kind: "upload_failed" } satisfies UploadError);
  });
});

describe("completeUpload", () => {
  it("正常_200で processing", async () => {
    vi.stubGlobal("fetch", vi.fn(async () =>
      new Response(JSON.stringify({ image_id: "iid", status: "processing" }), { status: 200 }),
    ));
    const got = await completeUpload("pid", "iid", "photobooks/pid/images/iid/original/x.jpg");
    expect(got.status).toBe("processing");
  });
  it("異常_422で validation_failed", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("{}", { status: 422 })));
    await expect(completeUpload("pid", "iid", "k")).rejects.toMatchObject({ kind: "validation_failed" } satisfies UploadError);
  });
});
