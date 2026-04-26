// /manage/token/[token] Route Handler。
//
// 役割（ADR-0003 / 業務知識 v4 §6.13 / 計画 m2-photobook-session-integration-plan.md §12）:
//   1. URL path から raw manage_url_token を取り出す
//   2. Backend `/api/auth/manage-session-exchange` を呼ぶ
//   3. 返ってきた raw session_token を **本 Route Handler で Set-Cookie**
//   4. `/manage/<photobook_id>` へ 302 redirect
//
// セキュリティ:
//   - raw manage_url_token / session_token はログに出さない
//   - response body / redirect URL に raw token を出さない
//   - Cookie 値全体・Set-Cookie ヘッダ全体をログに出さない
//   - 失敗時は token 値を含まない固定 redirect でエラー表示

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

import {
  exchangeManageToken,
  type ApiExchangeError,
} from "@/lib/api";
import {
  buildSessionCookieName,
  buildSessionCookieOptions,
} from "@/lib/cookies";

export async function GET(
  _req: NextRequest,
  ctx: { params: Promise<{ token: string }> },
): Promise<Response> {
  const { token } = await ctx.params;

  if (typeof token !== "string" || token.length === 0) {
    return redirectInvalid();
  }

  try {
    const out = await exchangeManageToken(token);

    const res = NextResponse.redirect(
      buildManagePageUrl(out.photobookId),
      302,
    );
    res.cookies.set({
      name: buildSessionCookieName("manage", out.photobookId),
      value: out.sessionToken,
      ...buildSessionCookieOptions(out.expiresAt),
    });
    res.headers.set("Cache-Control", "no-store");
    return res;
  } catch (err) {
    const _reason = (err as ApiExchangeError)?.kind ?? "network";
    return redirectInvalid();
  }
}

function redirectInvalid(): Response {
  const res = NextResponse.redirect(buildErrorRedirectUrl(), 302);
  res.headers.set("Cache-Control", "no-store");
  return res;
}

function buildManagePageUrl(photobookId: string): string {
  const base = (process.env.NEXT_PUBLIC_BASE_URL ?? "http://localhost:3000").replace(/\/$/, "");
  return `${base}/manage/${photobookId}`;
}

function buildErrorRedirectUrl(): string {
  const base = (process.env.NEXT_PUBLIC_BASE_URL ?? "http://localhost:3000").replace(/\/$/, "");
  return `${base}/?reason=invalid_manage_token`;
}
