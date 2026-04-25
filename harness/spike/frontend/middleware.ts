import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

/**
 * M1 PoC ヘッダ制御
 *
 * 検証目的:
 *  - Cloudflare Pages 上でページ種別ごとに Referrer-Policy を出し分けられるか確認する
 *  - X-Robots-Tag: noindex, nofollow が全ページに付与されるか確認する
 *
 * 注意:
 *  - 本実装ではないため、最小ロジックに留める
 *  - パス分岐は startsWith でラフに判定する
 */
const SENSITIVE_PATH_PREFIXES = ["/draft", "/manage", "/edit"];

function isSensitivePath(pathname: string): boolean {
  return SENSITIVE_PATH_PREFIXES.some((prefix) => pathname.startsWith(prefix));
}

export function middleware(req: NextRequest) {
  const res = NextResponse.next();
  const { pathname } = req.nextUrl;

  // 全ページ noindex, nofollow（v4 §7.6 / 業務知識 全ページ noindex 方針）
  res.headers.set("X-Robots-Tag", "noindex, nofollow");

  // Referrer-Policy 出し分け
  if (isSensitivePath(pathname)) {
    // draft / manage / edit 系: token 漏洩対策で no-referrer
    res.headers.set("Referrer-Policy", "no-referrer");
  } else {
    // 通常ページ: strict-origin-when-cross-origin
    res.headers.set("Referrer-Policy", "strict-origin-when-cross-origin");
  }

  return res;
}

export const config = {
  // _next, static asset, favicon, public 配下を除外
  matcher: ["/((?!_next/static|_next/image|favicon.ico|og-sample.png).*)"],
};
