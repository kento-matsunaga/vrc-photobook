// /manage/[photobookId]/revoke-session Route Handler (M-1a)。
//
// 役割:
//   1. manage Cookie 経由で Backend `/api/manage/photobooks/{id}/session-revoke` を proxy
//   2. 成功時に **app-domain manage Cookie を Max-Age=-1 でクリア**
//   3. JSON `{ "ok": true }` を返す
//
// 設計参照:
//   - docs/plan/m-1-manage-mvp-safety-plan.md §3.2.4
//   - 業務知識 v4 §3.4 (この端末から管理権限を削除)
//
// セキュリティ:
//   - raw token / Cookie 値をログに出さない
//   - 元の manage_url_token は失効させない（別端末からの再入場を妨げない）

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

import {
  buildClearCookieOptions,
  buildSessionCookieName,
} from "@/lib/cookies";

export async function POST(
  req: NextRequest,
  ctx: { params: Promise<{ photobookId: string }> },
): Promise<Response> {
  const { photobookId } = await ctx.params;
  if (typeof photobookId !== "string" || photobookId.length === 0) {
    return jsonNoStore(404, { status: "not_found" });
  }

  const cookieHeader = req.headers.get("cookie") ?? "";
  if (cookieHeader === "") {
    return jsonNoStore(401, { status: "unauthorized" });
  }

  const apiBase = (process.env.NEXT_PUBLIC_API_BASE_URL ?? "").replace(/\/$/, "");
  if (apiBase === "") {
    return jsonNoStore(500, { status: "internal_error" });
  }
  const upstreamURL = `${apiBase}/api/manage/photobooks/${encodeURIComponent(photobookId)}/session-revoke`;

  let upstream: Response;
  try {
    upstream = await fetch(upstreamURL, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Cookie: cookieHeader,
      },
      body: JSON.stringify({}),
    });
  } catch {
    return jsonNoStore(500, { status: "network" });
  }

  if (upstream.status === 401) {
    return jsonNoStore(401, { status: "unauthorized" });
  }
  if (upstream.status === 404) {
    return jsonNoStore(404, { status: "not_found" });
  }
  if (upstream.status < 200 || upstream.status >= 300) {
    return jsonNoStore(upstream.status, { status: "internal_error" });
  }

  // 成功時: app-domain manage Cookie をクリア
  const res = NextResponse.json(
    { ok: true },
    { status: 200, headers: { "Cache-Control": "no-store" } },
  );
  res.cookies.set({
    name: buildSessionCookieName("manage", photobookId),
    value: "",
    ...buildClearCookieOptions(),
  });
  return res;
}

function jsonNoStore(status: number, body: Record<string, unknown>): Response {
  return NextResponse.json(body, {
    status,
    headers: { "Cache-Control": "no-store" },
  });
}
