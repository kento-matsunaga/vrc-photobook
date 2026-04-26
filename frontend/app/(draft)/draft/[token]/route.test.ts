// PR10.5: /draft/[token] Route Handler のテスト。
//
// 方針:
//   - global.fetch を mock して Backend response を差し替える
//   - GET を直接呼び出す（実 dev server を起動しない）
//   - raw token を console / log に出さない（assert で部分一致のみ）

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

// makeReq は NextRequest 互換の Request を作る。Route Handler は r.cookies / r.nextUrl を
// ほぼ使わないため、標準の Request で代用できる。
function makeReq(url: string): Request {
  return new Request(url, { method: "GET" });
}

describe("GET /draft/[token] success path", () => {
  it("正常_Backend200で302+Set-Cookie+Locationにtoken無し", async () => {
    // Given: Backend が 200 を返し、{session_token, photobook_id, expires_at} を返す
    // When: Route Handler の GET が呼ばれる
    // Then: 302 redirect / Location が /edit/<photobook_id> / Set-Cookie に必須属性 / Location にも body にも raw token が出ない
    const draftRaw = fakeToken43("draft-success");
    const sessionRaw = fakeToken43("session-draft");
    const photobookId = "01234567-89ab-cdef-0123-456789abcdef";
    const expiresAt = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString();

    const fetchMock = vi.fn(async (input: string | URL | Request) => {
      const u = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      expect(u).toBe("http://backend.example.test/api/auth/draft-session-exchange");
      return new Response(
        JSON.stringify({
          session_token: sessionRaw,
          photobook_id: photobookId,
          expires_at: expiresAt,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    const req = makeReq(`http://localhost:3000/draft/${draftRaw}`);
    const res = await GET(req as never, { params: Promise.resolve({ token: draftRaw }) });

    expect(res.status).toBe(302);

    const loc = res.headers.get("location") ?? "";
    expect(loc).toBe(`http://localhost:3000/edit/${photobookId}`);
    expect(loc.includes(draftRaw)).toBe(false);
    expect(loc.includes(sessionRaw)).toBe(false);

    expect(res.headers.get("cache-control")).toBe("no-store");

    const setCookie = res.headers.get("set-cookie") ?? "";
    expect(setCookie).toContain(`vrcpb_draft_${photobookId}=`);
    expect(setCookie).toMatch(/HttpOnly/i);
    expect(setCookie).toMatch(/Secure/i);
    expect(setCookie).toMatch(/SameSite=Strict/i);
    expect(setCookie).toContain("Path=/");
    expect(setCookie).toMatch(/Max-Age=\d+/i);
    // Domain は COOKIE_DOMAIN 未設定なので付かない
    expect(setCookie.toLowerCase().includes("domain=")).toBe(false);

    // body は redirect なので空 or 短い HTML（raw token を含まない）
    const body = await res.text();
    expect(body.includes(draftRaw)).toBe(false);
    expect(body.includes(sessionRaw)).toBe(false);

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("正常_COOKIE_DOMAIN設定時はDomain=.vrcphotobook.com", async () => {
    // Given: COOKIE_DOMAIN='.vrcphotobook.com' 設定 + Backend 200
    // When: GET, Then: Set-Cookie に Domain=.vrcphotobook.com が含まれる
    process.env.COOKIE_DOMAIN = ".vrcphotobook.com";
    const draftRaw = fakeToken43("draft-domain");
    const sessionRaw = fakeToken43("session-domain");
    const photobookId = "11111111-1111-1111-1111-111111111111";
    const expiresAt = new Date(Date.now() + 60 * 60 * 1000).toISOString();

    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(
          JSON.stringify({
            session_token: sessionRaw,
            photobook_id: photobookId,
            expires_at: expiresAt,
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );

    const req = makeReq(`http://localhost:3000/draft/${draftRaw}`);
    const res = await GET(req as never, { params: Promise.resolve({ token: draftRaw }) });

    const setCookie = res.headers.get("set-cookie") ?? "";
    expect(setCookie).toContain("Domain=.vrcphotobook.com");
  });
});

describe("GET /draft/[token] failure path", () => {
  it("異常_Backend401で/?reason=invalid_draft_tokenへredirect+Set-Cookie無し", async () => {
    // Given: Backend が 401, When: GET, Then: 302 + Location が /?reason=invalid_draft_token /
    // Set-Cookie が出ない / raw token が漏れない
    const draftRaw = fakeToken43("draft-401");
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(`{"status":"unauthorized"}`, { status: 401 })),
    );

    const req = makeReq(`http://localhost:3000/draft/${draftRaw}`);
    const res = await GET(req as never, { params: Promise.resolve({ token: draftRaw }) });

    expect(res.status).toBe(302);
    const loc = res.headers.get("location") ?? "";
    expect(loc).toBe("http://localhost:3000/?reason=invalid_draft_token");
    expect(loc.includes(draftRaw)).toBe(false);

    expect(res.headers.get("set-cookie")).toBeNull();
    expect(res.headers.get("cache-control")).toBe("no-store");
  });

  it("異常_Backend500でも詳細を出さずredirect", async () => {
    // Given: Backend 500, When: GET, Then: 302 + reason=invalid_draft_token（区別しない）
    const draftRaw = fakeToken43("draft-500");
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(`{"status":"internal_error"}`, { status: 500 })),
    );

    const req = makeReq(`http://localhost:3000/draft/${draftRaw}`);
    const res = await GET(req as never, { params: Promise.resolve({ token: draftRaw }) });

    expect(res.status).toBe(302);
    expect(res.headers.get("location")).toBe("http://localhost:3000/?reason=invalid_draft_token");
    expect(res.headers.get("set-cookie")).toBeNull();
  });

  it("異常_fetch_networkエラーでも詳細を出さずredirect", async () => {
    // Given: fetch が throw, When: GET, Then: 302 + reason=invalid_draft_token / Set-Cookie 無し
    const draftRaw = fakeToken43("draft-network");
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new Error("network down");
      }),
    );

    const req = makeReq(`http://localhost:3000/draft/${draftRaw}`);
    const res = await GET(req as never, { params: Promise.resolve({ token: draftRaw }) });

    expect(res.status).toBe(302);
    expect(res.headers.get("location")).toBe("http://localhost:3000/?reason=invalid_draft_token");
    expect(res.headers.get("set-cookie")).toBeNull();
  });

  it("異常_token空文字_Backend呼び出しせずredirect", async () => {
    // Given: token=''（空）, When: GET, Then: 302 + Location が reason redirect / fetch が呼ばれない
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const req = makeReq(`http://localhost:3000/draft/`);
    const res = await GET(req as never, { params: Promise.resolve({ token: "" }) });
    expect(res.status).toBe(302);
    expect(res.headers.get("location")).toBe("http://localhost:3000/?reason=invalid_draft_token");
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
