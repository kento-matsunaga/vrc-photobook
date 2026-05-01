// /draft/[token] Route Handler。
//
// 役割（ADR-0003 / 業務知識 v4 §6.13 / 計画 m2-photobook-session-integration-plan.md §12,
//      docs/plan/m2-upload-staging-plan.md §5.2 採用案 A）:
//   1. URL path から raw draft_edit_token を取り出す
//   2. Backend `/api/auth/draft-session-exchange` を呼ぶ
//   3. 返ってきた raw session_token を **本 Route Handler で Set-Cookie**
//   4. `/prepare/<photobook_id>` へ 302 redirect（Upload Staging 画面）
//
// セキュリティ:
//   - raw draft_edit_token / session_token はログに出さない（console.* / error message にも含めない）
//   - response body / redirect URL に raw token を出さない
//   - Cookie 値全体・Set-Cookie ヘッダ全体をログに出さない
//   - 失敗時は `/?reason=invalid_draft_token` 等の **token 値を含まない** redirect でエラー表示

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

import {
  exchangeDraftToken,
  type ApiExchangeError,
} from "@/lib/api";
import {
  buildSessionCookieName,
  buildSessionCookieOptions,
} from "@/lib/cookies";

/**
 * GET /draft/[token]
 *
 * Cloudflare Workers / OpenNext 上で動くため、Edge / Node 互換ランタイムで動く。
 * 本ハンドラは I/O を 1 回だけ行い、即座に redirect する。
 */
export async function GET(
  _req: NextRequest,
  ctx: { params: Promise<{ token: string }> },
): Promise<Response> {
  const { token } = await ctx.params;

  // 早期に空 / 不正フォーマットを弾く（DB 照合へ送る前にコスト削減）。
  // 詳細な形式検証は Backend が行うため、ここでは「明らかに無効」のみを除く。
  if (typeof token !== "string" || token.length === 0) {
    return redirectInvalid();
  }

  try {
    const out = await exchangeDraftToken(token);

    const res = NextResponse.redirect(
      buildPreparePageUrl(out.photobookId),
      302,
    );
    res.cookies.set({
      name: buildSessionCookieName("draft", out.photobookId),
      value: out.sessionToken,
      ...buildSessionCookieOptions(out.expiresAt),
    });
    // Cache 抑止（CDN / ブラウザに raw session_token を含む応答を保持させない）
    res.headers.set("Cache-Control", "no-store");
    return res;
  } catch (err) {
    // err は ApiExchangeError 形を期待（exchangeDraftToken が throw する）。
    // raw token / cause 詳細は出さず、redirect でエラー画面（/）へ。
    const _reason = (err as ApiExchangeError)?.kind ?? "network";
    return redirectInvalid();
  }
}

/** 失敗時の redirect 先。raw token を含まない、固定 reason のみ。 */
function redirectInvalid(): Response {
  // PR10 段階ではエラーページが無いため、トップへ戻す。
  // PR11 以降で `/(error)/auth-failed` 等を作って差し替える。
  const res = NextResponse.redirect(buildErrorRedirectUrl(), 302);
  res.headers.set("Cache-Control", "no-store");
  return res;
}

/** /prepare/<photobook_id> の絶対 URL を組み立てる（Upload Staging 画面）。
 *  next=query で柔軟化する案 B は open redirect リスクのため不採用、ハードコード固定。 */
function buildPreparePageUrl(photobookId: string): string {
  const base = (process.env.NEXT_PUBLIC_BASE_URL ?? "http://localhost:3000").replace(/\/$/, "");
  return `${base}/prepare/${photobookId}`;
}

/** エラー時 redirect 先。 */
function buildErrorRedirectUrl(): string {
  const base = (process.env.NEXT_PUBLIC_BASE_URL ?? "http://localhost:3000").replace(/\/$/, "");
  return `${base}/?reason=invalid_draft_token`;
}
