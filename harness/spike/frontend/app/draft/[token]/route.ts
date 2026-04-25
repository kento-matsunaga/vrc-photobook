import { NextResponse } from "next/server";

/**
 * M1 PoC: draft 入場 Route Handler
 *
 * 検証目的:
 *  - URL の token を受け取り、ダミー検証扱いにする
 *  - HttpOnly Cookie に短命 session 値を入れる
 *  - /edit/{photobook_id} へ redirect することで URL から token を消せるか確認する
 *  - Set-Cookie + 302 redirect の組み合わせが Safari / iPhone Safari でも成立するか
 *
 * 重要:
 *  - PoC のため raw token の値はログ・画面に絶対出さない
 *  - Cookie 値はダミー固定値（本実装では 256bit 乱数 + DB 保存の hash）
 *  - 固定 photobook_id を使う（PoC のため）
 */

// Cloudflare Pages 上で動かすため Edge runtime を指定
export const runtime = "edge";

const FIXED_PHOTOBOOK_ID = "sample-photobook-id";
// PoC 用ダミー session 値。本実装では 32 バイトの暗号論的乱数を base64url 化する。
const POC_DUMMY_SESSION_VALUE = "poc-draft-session-value";
const SEVEN_DAYS_IN_SECONDS = 60 * 60 * 24 * 7;

export async function GET(req: Request) {
  // params は読み取らない（token 値をログに出さないため、URL から取得しても変数化しない）
  // ※ raw token 検証ロジックは本実装で Backend API へ委譲する。PoC は素通り扱い。

  // redirect 先を組み立て
  const url = new URL(req.url);
  const redirectUrl = new URL(`/edit/${FIXED_PHOTOBOOK_ID}`, url.origin);

  const res = NextResponse.redirect(redirectUrl, 302);

  // session Cookie 発行
  res.cookies.set(`vrcpb_draft_${FIXED_PHOTOBOOK_ID}`, POC_DUMMY_SESSION_VALUE, {
    httpOnly: true,
    secure: true,
    sameSite: "strict",
    path: "/",
    maxAge: SEVEN_DAYS_IN_SECONDS,
  });

  // 念のため Referrer-Policy を route 側でも明示
  res.headers.set("Referrer-Policy", "no-referrer");
  res.headers.set("X-Robots-Tag", "noindex, nofollow");

  return res;
}
