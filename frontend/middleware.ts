import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

// 業務知識 v4 §7.6 / ADR-0003 §決定 §SSR時のCookie検証 に従い、
// ヘッダ制御は middleware に一本化する（next.config.mjs の headers() には書かない）。
//
// M1 学習: next.config.mjs の headers() と middleware の両方で X-Robots-Tag を出すと
// Workers 実環境で値が `noindex, nofollow, noindex, nofollow` と二重出力された。
// 本実装でも middleware 一本化を維持する
// （harness/work-logs/2026-04-26_m1-live-deploy-verification.md）。
//
// 出し分けポリシー:
//   - 全ページ:                            X-Robots-Tag: noindex, nofollow（v4 §7.6: MVP は全 noindex）
//   - /draft, /manage, /edit, /prepare:    Referrer-Policy: no-referrer（token URL 漏洩対策、ADR-0003 §API 設計ルール）
//   - それ以外:                             Referrer-Policy: strict-origin-when-cross-origin
//
// /draft, /manage, /edit, /prepare のルートは app/(draft|manage) 配下に実装済。本 middleware の
// prefix match で no-referrer が自動付与される。
//
// /prepare は Upload Staging 画面（docs/plan/m2-upload-staging-plan.md §5.3）。raw token は
// URL に乗らない（photobook_id のみ）が、draft session cookie 配下の編集系経路として
// 既存 /draft, /edit と同等の Referrer-Policy で扱う。

const SENSITIVE_PATH_PREFIXES = ["/draft", "/manage", "/edit", "/prepare"];

function isSensitivePath(pathname: string): boolean {
  return SENSITIVE_PATH_PREFIXES.some((prefix) => pathname.startsWith(prefix));
}

export function middleware(req: NextRequest) {
  const res = NextResponse.next();

  res.headers.set("X-Robots-Tag", "noindex, nofollow");

  if (isSensitivePath(req.nextUrl.pathname)) {
    res.headers.set("Referrer-Policy", "no-referrer");
  } else {
    res.headers.set("Referrer-Policy", "strict-origin-when-cross-origin");
  }

  return res;
}

export const config = {
  // _next 配下 / 静的アセット / favicon は middleware を通さない
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
