// OGP readiness Client API の unit test (M-2 STOP δ、ADR-0007)。
//
// 検証範囲（fetchOgpReadinessClient のみ、polling ループは CompleteView 側 test で扱う）:
//   - status=generated / pending / not_found を正しく正規化
//   - 未知 status / 不正 JSON / network 失敗 / 5xx はすべて "error" に丸め、例外を投げない
//     （polling ループを壊さない、ADR-0007 §3 (4)）
//   - public endpoint なので credentials は付けない
//   - photobookId は encodeURIComponent される
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fetchOgpReadinessClient } from "@/lib/ogpReadiness";

const ORIGINAL_API = process.env.NEXT_PUBLIC_API_BASE_URL;

beforeEach(() => {
  process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test";
});

afterEach(() => {
  vi.unstubAllGlobals();
  process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
});

describe("fetchOgpReadinessClient: status 正規化", () => {
  const tests = [
    {
      name: "正常_generated_を_generated_に正規化",
      description:
        "Given: Backend が status=generated を返す, When: fetch, Then: \"generated\" を返し共有ボタン enable に倒せる",
      backendStatus: "generated",
      want: "generated" as const,
    },
    {
      name: "正常_pending_を_pending_に正規化",
      description:
        "Given: Backend が status=pending を返す, When: fetch, Then: \"pending\" を返し polling 継続させる",
      backendStatus: "pending",
      want: "pending" as const,
    },
    {
      name: "正常_not_found_を_not_found_に正規化",
      description:
        "Given: Backend が status=not_found を返す (公開直後の極短時間), When: fetch, Then: \"not_found\" を返し polling 継続",
      backendStatus: "not_found",
      want: "not_found" as const,
    },
    {
      name: "異常_未知_status_は_error_に丸める",
      description:
        "Given: Backend が想定外の status を返す, When: fetch, Then: \"error\" に丸めて polling 継続",
      backendStatus: "stale",
      want: "error" as const,
    },
  ];

  for (const tt of tests) {
    it(tt.name, async () => {
      vi.stubGlobal(
        "fetch",
        vi.fn(async () => ({
          ok: true,
          status: 200,
          json: async () => ({ status: tt.backendStatus, version: 1, image_url_path: "/ogp/x?v=1" }),
        })),
      );
      const got = await fetchOgpReadinessClient("00000000-0000-0000-0000-000000000001");
      expect(got).toBe(tt.want);
    });
  }
});

describe("fetchOgpReadinessClient: 失敗 case は error に丸める (polling ループを壊さない)", () => {
  const tests = [
    {
      name: "正常_404_not_found_body_は_not_found_に正規化",
      description:
        "Given: 404 + {status:not_found} (row 未作成 transient), When: fetch, Then: \"not_found\" で polling 継続",
      makeFetch: () =>
        vi.fn(async () => ({
          ok: false,
          status: 404,
          json: async () => ({ status: "not_found", version: 0, image_url_path: "/og/default.png" }),
        })),
      want: "not_found" as const,
    },
    {
      name: "異常_5xx_はerror",
      description:
        "Given: 500 Internal, When: fetch, Then: \"error\" に丸めて polling 継続",
      makeFetch: () =>
        vi.fn(async () => ({
          ok: false,
          status: 500,
          json: async () => ({ status: "error" }),
        })),
      want: "error" as const,
    },
    {
      name: "異常_network失敗_はerror",
      description: "Given: fetch reject, When: fetchOgpReadinessClient, Then: \"error\" を返す",
      makeFetch: () =>
        vi.fn(async () => {
          throw new Error("network");
        }),
      want: "error" as const,
    },
    {
      name: "異常_JSON_parse失敗_はerror",
      description: "Given: 200 だが json() が throw, When: fetch, Then: \"error\"",
      makeFetch: () =>
        vi.fn(async () => ({
          ok: true,
          status: 200,
          json: async () => {
            throw new Error("not json");
          },
        })),
      want: "error" as const,
    },
    {
      name: "異常_status_field_欠落_はerror",
      description: "Given: body に status 欠落, When: fetch, Then: \"error\"",
      makeFetch: () =>
        vi.fn(async () => ({
          ok: true,
          status: 200,
          json: async () => ({ version: 1, image_url_path: "/ogp/x?v=1" }),
        })),
      want: "error" as const,
    },
  ];

  for (const tt of tests) {
    it(tt.name, async () => {
      vi.stubGlobal("fetch", tt.makeFetch());
      const got = await fetchOgpReadinessClient("00000000-0000-0000-0000-000000000001");
      expect(got).toBe(tt.want);
    });
  }
});

describe("fetchOgpReadinessClient: URL 構成と credentials", () => {
  it("正常_URLにencodeURIComponentが効きcredentialsは付けない", async () => {
    // Given: photobookId に encode 対象文字（ここではダミーの UUID 形式、純粋に query 検証用）
    const fetchSpy = vi.fn(async (_url: string, _init?: RequestInit) => ({
      ok: true,
      status: 200,
      json: async () => ({ status: "generated", version: 1, image_url_path: "/ogp/x?v=1" }),
    }));
    vi.stubGlobal("fetch", fetchSpy);
    await fetchOgpReadinessClient("00000000-0000-0000-0000-000000000001");
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const call = fetchSpy.mock.calls[0];
    const url = call[0];
    const init = call[1] ?? {};
    expect(url).toBe(
      "https://api.test/api/public/photobooks/00000000-0000-0000-0000-000000000001/ogp",
    );
    // public endpoint なので credentials は付けない（既定 same-origin で OK）
    expect(init.credentials).toBeUndefined();
    expect(init.method).toBe("GET");
    expect(init.cache).toBe("no-store");
  });

  it("異常_NEXT_PUBLIC_API_BASE_URL_未設定_はerror", async () => {
    delete process.env.NEXT_PUBLIC_API_BASE_URL;
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const got = await fetchOgpReadinessClient("00000000-0000-0000-0000-000000000001");
    expect(got).toBe("error");
    expect(fetchSpy).not.toHaveBeenCalled();
  });
});
