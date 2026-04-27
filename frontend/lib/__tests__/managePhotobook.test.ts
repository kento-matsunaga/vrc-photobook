// managePhotobook API client の unit test。
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  fetchManagePhotobook,
  isManageLookupError,
  type ManageLookupError,
} from "@/lib/managePhotobook";

const ORIGINAL_API = process.env.NEXT_PUBLIC_API_BASE_URL;

beforeEach(() => {
  process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.test";
});

afterEach(() => {
  vi.unstubAllGlobals();
  process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
});

describe("fetchManagePhotobook", () => {
  const tests: Array<{
    name: string;
    description: string;
    status: number;
    body?: unknown;
    wantKind?: ManageLookupError["kind"];
    wantOK?: boolean;
  }> = [
    {
      name: "正常_200で_payloadをcamelCase化",
      description: "Given: 200 + snake_case payload, When: fetch, Then: camelCase",
      status: 200,
      body: {
        photobook_id: "00000000-0000-0000-0000-000000000001",
        type: "event",
        title: "Sample",
        status: "published",
        visibility: "unlisted",
        hidden_by_operator: false,
        public_url_slug: "ok12pp34zz56gh78",
        public_url_path: "/p/ok12pp34zz56gh78",
        manage_url_token_version: 1,
        available_image_count: 5,
      },
      wantOK: true,
    },
    {
      name: "異常_401でunauthorized",
      description: "Cookie 不在で 401",
      status: 401,
      wantKind: "unauthorized",
    },
    {
      name: "異常_404でnot_found",
      description: "photobook 不存在で 404",
      status: 404,
      wantKind: "not_found",
    },
    {
      name: "異常_500でserver_error",
      description: "DB 障害で 500",
      status: 500,
      wantKind: "server_error",
    },
  ];

  for (const tt of tests) {
    it(tt.name, async () => {
      const mockFetch = vi.fn(async (_url: unknown, _init: unknown) => ({
        status: tt.status,
        json: async () => tt.body,
      }));
      vi.stubGlobal("fetch", mockFetch);

      if (tt.wantOK) {
        const got = await fetchManagePhotobook(
          "00000000-0000-0000-0000-000000000001",
          "vrcpb_manage_xxx=value",
        );
        expect(got.publicUrlPath).toBe("/p/ok12pp34zz56gh78");
        expect(got.availableImageCount).toBe(5);
        // Cookie が手動で組み立てられて Backend に渡されている
        const calls = mockFetch.mock.calls;
        const init = calls[0][1] as RequestInit | undefined;
        const headers = init?.headers as Record<string, string> | undefined;
        expect(headers?.Cookie).toBe("vrcpb_manage_xxx=value");
      } else {
        await expect(
          fetchManagePhotobook("00000000-0000-0000-0000-000000000001", ""),
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
      await fetchManagePhotobook("00000000-0000-0000-0000-000000000001", "");
    } catch (e) {
      expect(isManageLookupError(e)).toBe(true);
      if (isManageLookupError(e)) {
        expect(e.kind).toBe("network");
      }
    }
  });
});
