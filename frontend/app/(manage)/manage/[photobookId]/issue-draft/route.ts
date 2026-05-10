// /manage/[photobookId]/issue-draft Route Handler (M-1a)。
//
// 役割:
//   1. manage Cookie 経由で Backend `/api/manage/photobooks/{id}/draft-session` を proxy
//   2. 返ってきた raw draft session_token を **本 Route Handler で Set-Cookie** (app domain)
//   3. JSON `{ "edit_url": "/edit/<photobookId>" }` を返す（Frontend 側が遷移）
//
// 設計参照:
//   - docs/plan/m-1-manage-mvp-safety-plan.md §3.2.5 (案 A)
//   - 業務知識 v4 §6.13 (manage / draft 漏洩リスクは同等)
//   - .agents/rules/client-vs-ssr-fetch.md (cross-origin Cookie の扱い)
//
// セキュリティ:
//   - raw session_token はログ / response body / URL に出さない
//   - Cookie 値全体 / Set-Cookie ヘッダ全体をログに出さない
//   - 失敗時は固定 reason の JSON で返し、token 値を含めない

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

import {
  buildSessionCookieName,
  buildSessionCookieOptions,
} from "@/lib/cookies";

type ManageDraftSessionPayload = {
  session_token: string;
  photobook_id: string;
  expires_at: string;
};

type Body409 = { status?: string; reason?: string };

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
  const upstreamURL = `${apiBase}/api/manage/photobooks/${encodeURIComponent(photobookId)}/draft-session`;

  let upstream: Response;
  try {
    upstream = await fetch(upstreamURL, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        // manage Cookie を Backend に転送（cross-origin の cookie 引継ぎ）
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
  if (upstream.status === 409) {
    let parsed: Body409 = {};
    try {
      parsed = (await upstream.json()) as Body409;
    } catch {
      // ignore
    }
    return jsonNoStore(409, parsed);
  }
  if (upstream.status < 200 || upstream.status >= 300) {
    return jsonNoStore(upstream.status, { status: "internal_error" });
  }

  let payload: ManageDraftSessionPayload;
  try {
    payload = (await upstream.json()) as ManageDraftSessionPayload;
  } catch {
    return jsonNoStore(500, { status: "internal_error" });
  }
  if (
    typeof payload.session_token !== "string" || payload.session_token.length === 0 ||
    typeof payload.photobook_id !== "string" || payload.photobook_id.length === 0 ||
    typeof payload.expires_at !== "string" || payload.expires_at.length === 0
  ) {
    return jsonNoStore(500, { status: "internal_error" });
  }

  const expiresAt = new Date(payload.expires_at);
  if (Number.isNaN(expiresAt.getTime())) {
    return jsonNoStore(500, { status: "internal_error" });
  }

  const editPath = `/edit/${payload.photobook_id}`;
  const res = NextResponse.json(
    { edit_url: editPath },
    { status: 200, headers: { "Cache-Control": "no-store" } },
  );
  // app-domain HttpOnly draft session Cookie を発行
  res.cookies.set({
    name: buildSessionCookieName("draft", payload.photobook_id),
    value: payload.session_token,
    ...buildSessionCookieOptions(expiresAt),
  });
  return res;
}

function jsonNoStore(status: number, body: Record<string, unknown>): Response {
  return NextResponse.json(body, {
    status,
    headers: { "Cache-Control": "no-store" },
  });
}
