// Cookie 生成ユーティリティ。
//
// 設計参照:
//   - docs/adr/0003-frontend-token-session-flow.md
//   - docs/plan/m2-photobook-session-integration-plan.md §6 / §12
//   - .agents/rules/security-guard.md
//
// セキュリティ:
//   - HttpOnly / Secure / SameSite=Strict / Path=/ を必ず付与
//   - Cookie 値・Set-Cookie ヘッダ全体はログに出さない（呼び出し元の責務）
//   - Domain は Server-only env `COOKIE_DOMAIN` から取得し、未設定時は host-only
//   - localhost ではブラウザが Secure context として扱うため、Secure=true のままで OK

import type { ResponseCookie } from "next/dist/compiled/@edge-runtime/cookies";

/** session_type の判別子（cookie 名生成で使う）。 */
export type SessionType = "draft" | "manage";

/** Cookie 名 prefix。 */
const PREFIX_DRAFT = "vrcpb_draft_";
const PREFIX_MANAGE = "vrcpb_manage_";

/** photobook_id を含む Cookie 名を組み立てる。 */
export function buildSessionCookieName(
  type: SessionType,
  photobookId: string,
): string {
  const prefix = type === "draft" ? PREFIX_DRAFT : PREFIX_MANAGE;
  return `${prefix}${photobookId}`;
}

/**
 * COOKIE_DOMAIN を Server env から取得する。
 *
 * 本関数は Route Handler / Server Component からのみ呼ばれる前提（Edge Runtime / Node Runtime）。
 * Client Component で呼ぶとビルド時に env が undefined になる。
 *
 * 戻り値:
 *   - 空文字なら host-only Cookie（Domain 属性を出さない）
 *   - 値があればその値（例: ".vrc-photobook.com"）
 */
export function getCookieDomain(): string {
  return process.env.COOKIE_DOMAIN ?? "";
}

/**
 * Set-Cookie 用のオプションを組み立てる。
 *
 * @param expiresAt 有効期限（ISO8601 文字列 or Date）
 * @param now 現在時刻（テスト用に注入可能、未指定時は new Date()）
 *
 * Max-Age は秒。expires <= now の場合は 0（即時失効）。
 */
export function buildSessionCookieOptions(
  expiresAt: string | Date,
  now: Date = new Date(),
): Pick<ResponseCookie, "httpOnly" | "secure" | "sameSite" | "path" | "maxAge" | "domain"> {
  const expires = typeof expiresAt === "string" ? new Date(expiresAt) : expiresAt;
  const diffSec = Math.floor((expires.getTime() - now.getTime()) / 1000);
  const maxAge = diffSec > 0 ? diffSec : 0;
  const domain = getCookieDomain();

  const opts: Pick<ResponseCookie, "httpOnly" | "secure" | "sameSite" | "path" | "maxAge" | "domain"> = {
    httpOnly: true,
    secure: true,
    sameSite: "strict",
    path: "/",
    maxAge,
  };
  if (domain !== "") {
    opts.domain = domain;
  }
  return opts;
}

/**
 * 明示破棄用の Cookie オプション（Max-Age=0 / Value 空）を返す。
 *
 * 元の draft_edit_token / manage_url_token は失効させない（別端末からの再入場を妨げない、
 * 設計書 §3.3）。Cookie のみ削除。
 */
export function buildClearCookieOptions(): Pick<
  ResponseCookie,
  "httpOnly" | "secure" | "sameSite" | "path" | "maxAge" | "domain"
> {
  const domain = getCookieDomain();
  const opts: Pick<ResponseCookie, "httpOnly" | "secure" | "sameSite" | "path" | "maxAge" | "domain"> = {
    httpOnly: true,
    secure: true,
    sameSite: "strict",
    path: "/",
    maxAge: 0,
  };
  if (domain !== "") {
    opts.domain = domain;
  }
  return opts;
}
