import { NextResponse } from "next/server";

/**
 * M1 PoC: manage 入場 Route Handler
 *
 * 検証目的:
 *  - /manage/token/{token} で受け取った token を session Cookie に交換
 *  - /manage/{photobook_id} へ redirect で URL から token を消す
 *  - draft 入場と同じ Cookie 属性 / Edge runtime / redirect 挙動が成立するか
 *
 * 重要:
 *  - raw token の値はログ・画面に絶対出さない
 *  - Cookie 値はダミー固定値
 */

export const runtime = "edge";

const FIXED_PHOTOBOOK_ID = "sample-photobook-id";
// PoC 用ダミー session 値。本実装では 32 バイトの暗号論的乱数を base64url 化する。
const POC_DUMMY_SESSION_VALUE = "poc-manage-session-value";
// manage session は 24h〜7d 想定。PoC は 24 時間で確認。
const ONE_DAY_IN_SECONDS = 60 * 60 * 24;

export async function GET(req: Request) {
  const url = new URL(req.url);
  const redirectUrl = new URL(`/manage/${FIXED_PHOTOBOOK_ID}`, url.origin);

  const res = NextResponse.redirect(redirectUrl, 302);

  res.cookies.set(`vrcpb_manage_${FIXED_PHOTOBOOK_ID}`, POC_DUMMY_SESSION_VALUE, {
    httpOnly: true,
    secure: true,
    sameSite: "strict",
    path: "/",
    maxAge: ONE_DAY_IN_SECONDS,
  });

  res.headers.set("Referrer-Policy", "no-referrer");
  res.headers.set("X-Robots-Tag", "noindex, nofollow");

  return res;
}
