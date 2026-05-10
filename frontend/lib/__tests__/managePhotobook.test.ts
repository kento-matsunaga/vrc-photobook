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

// =============================================================================
// M-1a: Manage safety baseline mutations
// =============================================================================

import {
  isManageMutationError,
  issueDraftSessionFromManage,
  revokeManageSession,
  updateSensitiveFromManage,
  updateVisibilityFromManage,
} from "@/lib/managePhotobook";

describe("updateVisibilityFromManage", () => {
  it("正常_200_で_versionが返る", async () => {
    const mockFetch = vi.fn(async (_url: unknown, _init: unknown) => ({
      status: 200,
      json: async () => ({ version: 7 }),
    }));
    vi.stubGlobal("fetch", mockFetch);
    const got = await updateVisibilityFromManage(
      "00000000-0000-0000-0000-000000000001",
      "private",
      6,
    );
    expect(got.version).toBe(7);
    const init = mockFetch.mock.calls[0][1] as RequestInit;
    expect(init.method).toBe("PATCH");
    expect(init.credentials).toBe("include");
    expect(JSON.parse(String(init.body))).toEqual({
      visibility: "private",
      expected_version: 6,
    });
  });

  it("異常_409_public_change_not_allowed", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 409,
        json: async () => ({
          status: "manage_precondition_failed",
          reason: "public_change_not_allowed",
        }),
      })),
    );
    try {
      await updateVisibilityFromManage(
        "00000000-0000-0000-0000-000000000001",
        "unlisted",
        1,
      );
      throw new Error("should reject");
    } catch (e) {
      expect(isManageMutationError(e)).toBe(true);
      if (isManageMutationError(e)) {
        expect(e.kind).toBe("public_change_not_allowed");
      }
    }
  });

  it("異常_409_version_conflict", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 409,
        json: async () => ({ status: "version_conflict" }),
      })),
    );
    await expect(
      updateVisibilityFromManage(
        "00000000-0000-0000-0000-000000000001",
        "private",
        1,
      ),
    ).rejects.toMatchObject({ kind: "version_conflict" });
  });
});

describe("updateSensitiveFromManage", () => {
  it("正常_200_で_versionが返る", async () => {
    const mockFetch = vi.fn(async (_url: unknown, _init: unknown) => ({
      status: 200,
      json: async () => ({ version: 9 }),
    }));
    vi.stubGlobal("fetch", mockFetch);
    const got = await updateSensitiveFromManage(
      "00000000-0000-0000-0000-000000000001",
      true,
      8,
    );
    expect(got.version).toBe(9);
    const init = mockFetch.mock.calls[0][1] as RequestInit;
    expect(JSON.parse(String(init.body))).toEqual({
      sensitive: true,
      expected_version: 8,
    });
  });

  it("異常_409_version_conflict", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 409,
        json: async () => ({ status: "version_conflict" }),
      })),
    );
    await expect(
      updateSensitiveFromManage(
        "00000000-0000-0000-0000-000000000001",
        false,
        1,
      ),
    ).rejects.toMatchObject({ kind: "version_conflict" });
  });
});

describe("issueDraftSessionFromManage", () => {
  it("正常_200_で_edit_urlが返る", async () => {
    const mockFetch = vi.fn(async (_url: unknown, _init: unknown) => ({
      status: 200,
      json: async () => ({ edit_url: "/edit/00000000-0000-0000-0000-000000000001" }),
    }));
    vi.stubGlobal("fetch", mockFetch);
    const got = await issueDraftSessionFromManage(
      "00000000-0000-0000-0000-000000000001",
    );
    expect(got.editUrl).toBe("/edit/00000000-0000-0000-0000-000000000001");
    // Workers Route Handler を叩く（同 origin）
    const url = String(mockFetch.mock.calls[0][0]);
    expect(url).toBe("/manage/00000000-0000-0000-0000-000000000001/issue-draft");
    const init = mockFetch.mock.calls[0][1] as RequestInit;
    expect(init.method).toBe("POST");
    expect(init.credentials).toBe("include");
  });

  it("異常_409_not_draft_reasonがマップされる", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 409,
        json: async () => ({
          status: "manage_precondition_failed",
          reason: "not_draft",
        }),
      })),
    );
    try {
      await issueDraftSessionFromManage("00000000-0000-0000-0000-000000000001");
      throw new Error("should reject");
    } catch (e) {
      expect(isManageMutationError(e)).toBe(true);
      if (isManageMutationError(e)) {
        expect(e.kind).toBe("not_draft");
      }
    }
  });
});

describe("revokeManageSession", () => {
  it("正常_200_で_voidを返す", async () => {
    const mockFetch = vi.fn(async (_url: unknown, _init: unknown) => ({
      status: 200,
      json: async () => ({ ok: true }),
    }));
    vi.stubGlobal("fetch", mockFetch);
    await expect(
      revokeManageSession("00000000-0000-0000-0000-000000000001"),
    ).resolves.toBeUndefined();
    const url = String(mockFetch.mock.calls[0][0]);
    expect(url).toBe("/manage/00000000-0000-0000-0000-000000000001/revoke-session");
  });

  it("異常_401_unauthorized", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({ status: 401, json: async () => ({}) })),
    );
    await expect(
      revokeManageSession("00000000-0000-0000-0000-000000000001"),
    ).rejects.toMatchObject({ kind: "unauthorized" });
  });
});
