import { afterEach, describe, expect, it } from "vitest";

import {
  buildClearCookieOptions,
  buildSessionCookieName,
  buildSessionCookieOptions,
  getCookieDomain,
} from "../cookies";

const ORIGINAL_COOKIE_DOMAIN = process.env.COOKIE_DOMAIN;

describe("buildSessionCookieName", () => {
  it("正常_draftはvrcpb_draft_<id>", () => {
    // Given: type=draft, photobook_id, When: buildSessionCookieName, Then: 'vrcpb_draft_<id>'
    expect(buildSessionCookieName("draft", "abc-123")).toBe("vrcpb_draft_abc-123");
  });

  it("正常_manageはvrcpb_manage_<id>", () => {
    // Given: type=manage, photobook_id, When: buildSessionCookieName, Then: 'vrcpb_manage_<id>'
    expect(buildSessionCookieName("manage", "abc-123")).toBe("vrcpb_manage_abc-123");
  });
});

describe("getCookieDomain / buildSessionCookieOptions", () => {
  afterEach(() => {
    if (ORIGINAL_COOKIE_DOMAIN === undefined) {
      delete process.env.COOKIE_DOMAIN;
    } else {
      process.env.COOKIE_DOMAIN = ORIGINAL_COOKIE_DOMAIN;
    }
  });

  it("正常_COOKIE_DOMAIN未設定時はDomain属性が出ない", () => {
    // Given: process.env.COOKIE_DOMAIN 未設定, When: buildSessionCookieOptions, Then: domain プロパティが options に含まれない
    delete process.env.COOKIE_DOMAIN;
    const now = new Date("2026-04-26T00:00:00Z");
    const expires = new Date("2026-04-27T00:00:00Z");
    const opts = buildSessionCookieOptions(expires, now);
    expect(opts.httpOnly).toBe(true);
    expect(opts.secure).toBe(true);
    expect(opts.sameSite).toBe("strict");
    expect(opts.path).toBe("/");
    expect(opts.maxAge).toBe(24 * 60 * 60);
    expect(opts.domain).toBeUndefined();
    expect(getCookieDomain()).toBe("");
  });

  it("正常_COOKIE_DOMAIN設定時はDomain属性が反映", () => {
    // Given: COOKIE_DOMAIN='.vrcphotobook.com', When: buildSessionCookieOptions, Then: domain='.vrcphotobook.com'
    process.env.COOKIE_DOMAIN = ".vrcphotobook.com";
    const now = new Date("2026-04-26T00:00:00Z");
    const expires = new Date("2026-04-26T01:00:00Z");
    const opts = buildSessionCookieOptions(expires, now);
    expect(opts.domain).toBe(".vrcphotobook.com");
    expect(opts.maxAge).toBe(60 * 60);
  });

  it("正常_expires<=nowでmaxAge=0", () => {
    // Given: expires<=now, When: buildSessionCookieOptions, Then: maxAge=0
    const now = new Date("2026-04-26T00:00:00Z");
    const expires = new Date("2026-04-26T00:00:00Z");
    const opts = buildSessionCookieOptions(expires, now);
    expect(opts.maxAge).toBe(0);
  });

  it("正常_buildClearCookieOptions", () => {
    // Given: なし, When: buildClearCookieOptions, Then: maxAge=0 / 必須属性が立つ
    const opts = buildClearCookieOptions();
    expect(opts.httpOnly).toBe(true);
    expect(opts.secure).toBe(true);
    expect(opts.sameSite).toBe("strict");
    expect(opts.path).toBe("/");
    expect(opts.maxAge).toBe(0);
  });
});
