// PR10.5: /manage/token/[token] Route Handler のテスト。
//
// 方針は draft 側 (route.test.ts) と同じ:
//   - global.fetch を mock
//   - GET を直接呼び出す
//   - raw token を console / log に出さない

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fakeToken43 } from "@/lib/__tests__/test_helpers";
import { GET } from "./route";

const ORIGINAL_API = process.env.NEXT_PUBLIC_API_BASE_URL;
const ORIGINAL_BASE = process.env.NEXT_PUBLIC_BASE_URL;
const ORIGINAL_COOKIE_DOMAIN = process.env.COOKIE_DOMAIN;

beforeEach(() => {
  process.env.NEXT_PUBLIC_API_BASE_URL = "http://backend.example.test";
  process.env.NEXT_PUBLIC_BASE_URL = "http://localhost:3000";
  delete process.env.COOKIE_DOMAIN;
});

afterEach(() => {
  vi.unstubAllGlobals();
  process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL_API;
  process.env.NEXT_PUBLIC_BASE_URL = ORIGINAL_BASE;
  if (ORIGINAL_COOKIE_DOMAIN === undefined) {
    delete process.env.COOKIE_DOMAIN;
  } else {
    process.env.COOKIE_DOMAIN = ORIGINAL_COOKIE_DOMAIN;
  }
});

function makeReq(url: string): Request {
  return new Request(url, { method: "GET" });
}

describe("GET /manage/token/[token] success path", () => {
  it("正常_Backend200で302+Set-Cookie+Locationにtoken無し", async () => {
    // Given: Backend が 200 を返す, When: GET, Then: 302 + Location が /manage/<id> + Set-Cookie 必須属性
    const manageRaw = fakeToken43("manage-success");
    const sessionRaw = fakeToken43("session-manage");
    const photobookId = "22222222-2222-2222-2222-222222222222";
    const expiresAt = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString();

    const fetchMock = vi.fn(async (input: string | URL | Request) => {
      const u = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      expect(u).toBe("http://backend.example.test/api/auth/manage-session-exchange");
      return new Response(
        JSON.stringify({
          session_token: sessionRaw,
          photobook_id: photobookId,
          expires_at: expiresAt,
          token_version_at_issue: 0,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    const req = makeReq(`http://localhost:3000/manage/token/${manageRaw}`);
    const res = await GET(req as never, { params: Promise.resolve({ token: manageRaw }) });

    expect(res.status).toBe(302);

    const loc = res.headers.get("location") ?? "";
    expect(loc).toBe(`http://localhost:3000/manage/${photobookId}`);
    expect(loc.includes(manageRaw)).toBe(false);
    expect(loc.includes(sessionRaw)).toBe(false);

    expect(res.headers.get("cache-control")).toBe("no-store");

    const setCookie = res.headers.get("set-cookie") ?? "";
    expect(setCookie).toContain(`vrcpb_manage_${photobookId}=`);
    expect(setCookie).toMatch(/HttpOnly/i);
    expect(setCookie).toMatch(/Secure/i);
    expect(setCookie).toMatch(/SameSite=Strict/i);
    expect(setCookie).toContain("Path=/");
    expect(setCookie).toMatch(/Max-Age=\d+/i);
    expect(setCookie.toLowerCase().includes("domain=")).toBe(false);

    const body = await res.text();
    expect(body.includes(manageRaw)).toBe(false);
    expect(body.includes(sessionRaw)).toBe(false);

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("正常_COOKIE_DOMAIN設定時はDomain=.vrc-photobook.com", async () => {
    // Given: COOKIE_DOMAIN='.vrc-photobook.com', When: GET 200, Then: Set-Cookie に Domain=...
    process.env.COOKIE_DOMAIN = ".vrc-photobook.com";
    const manageRaw = fakeToken43("manage-domain");
    const sessionRaw = fakeToken43("session-domain-m");
    const photobookId = "33333333-3333-3333-3333-333333333333";
    const expiresAt = new Date(Date.now() + 60 * 60 * 1000).toISOString();

    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(
          JSON.stringify({
            session_token: sessionRaw,
            photobook_id: photobookId,
            expires_at: expiresAt,
            token_version_at_issue: 1,
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );

    const req = makeReq(`http://localhost:3000/manage/token/${manageRaw}`);
    const res = await GET(req as never, { params: Promise.resolve({ token: manageRaw }) });

    const setCookie = res.headers.get("set-cookie") ?? "";
    expect(setCookie).toContain("Domain=.vrc-photobook.com");
  });
});

describe("GET /manage/token/[token] failure path", () => {
  it("異常_Backend401で/?reason=invalid_manage_tokenへredirect+Set-Cookie無し", async () => {
    // Given: Backend 401, When: GET, Then: 302 + Location が reason redirect / Set-Cookie 無し
    const manageRaw = fakeToken43("manage-401");
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(`{"status":"unauthorized"}`, { status: 401 })),
    );

    const req = makeReq(`http://localhost:3000/manage/token/${manageRaw}`);
    const res = await GET(req as never, { params: Promise.resolve({ token: manageRaw }) });

    expect(res.status).toBe(302);
    const loc = res.headers.get("location") ?? "";
    expect(loc).toBe("http://localhost:3000/?reason=invalid_manage_token");
    expect(loc.includes(manageRaw)).toBe(false);
    expect(res.headers.get("set-cookie")).toBeNull();
  });

  it("異常_token空文字_Backend呼び出しせずredirect", async () => {
    // Given: token=''（空）, When: GET, Then: 302 reason redirect / fetch 呼ばれない
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const req = makeReq(`http://localhost:3000/manage/token/`);
    const res = await GET(req as never, { params: Promise.resolve({ token: "" }) });
    expect(res.status).toBe(302);
    expect(res.headers.get("location")).toBe("http://localhost:3000/?reason=invalid_manage_token");
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
