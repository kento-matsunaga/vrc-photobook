// Backend API クライアント（Server-side fetch wrapper）。
//
// 設計参照:
//   - docs/plan/m2-photobook-session-integration-plan.md §10 / §12
//
// セキュリティ:
//   - raw token をログ出力しない（fetch の引数も含めて、エラー時に値そのものを露出させない）
//   - Backend が返す raw session_token は呼び出し元（Route Handler）が Cookie に書くだけで、
//     ログ・画面・URL に出さない
//   - Backend は Set-Cookie を出さないため、本クライアントでも credentials を扱わない

/** Backend のベース URL を取得する（Server-side 専用）。 */
export function getApiBaseUrl(): string {
  // NEXT_PUBLIC_API_BASE_URL は build 時に bundle に inline される公開値。
  // Server Component / Route Handler からも参照可能。
  const url = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  if (url === "") {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return url.replace(/\/$/, "");
}

/** draft session 交換のレスポンス。 */
export type DraftSessionExchangeResponse = {
  sessionToken: string;
  photobookId: string;
  expiresAt: string;
};

/** manage session 交換のレスポンス。 */
export type ManageSessionExchangeResponse = {
  sessionToken: string;
  photobookId: string;
  expiresAt: string;
  tokenVersionAtIssue: number;
};

/** API 呼び出しのエラー種別（Route Handler 側で handling）。 */
export type ApiExchangeError =
  | { kind: "unauthorized" }
  | { kind: "bad_request" }
  | { kind: "server_error" }
  | { kind: "network" };

/**
 * draft_edit_token を session_token に交換する。
 *
 * Backend `/api/auth/draft-session-exchange` を呼ぶ。Backend は Set-Cookie を出さない。
 * 戻ってくる raw session_token は呼び出し元が Cookie に書く。
 *
 * 失敗時は ApiExchangeError 形のオブジェクトを throw する（Route Handler で catch）。
 */
export async function exchangeDraftToken(
  rawDraftToken: string,
  signal?: AbortSignal,
): Promise<DraftSessionExchangeResponse> {
  const res = await callExchange(
    `${getApiBaseUrl()}/api/auth/draft-session-exchange`,
    { draft_edit_token: rawDraftToken },
    signal,
  );
  return {
    sessionToken: res["session_token"] as string,
    photobookId: res["photobook_id"] as string,
    expiresAt: res["expires_at"] as string,
  };
}

/**
 * manage_url_token を session_token に交換する。
 */
export async function exchangeManageToken(
  rawManageToken: string,
  signal?: AbortSignal,
): Promise<ManageSessionExchangeResponse> {
  const res = await callExchange(
    `${getApiBaseUrl()}/api/auth/manage-session-exchange`,
    { manage_url_token: rawManageToken },
    signal,
  );
  return {
    sessionToken: res["session_token"] as string,
    photobookId: res["photobook_id"] as string,
    expiresAt: res["expires_at"] as string,
    tokenVersionAtIssue: res["token_version_at_issue"] as number,
  };
}

/** 共通呼び出し。エラーは ApiExchangeError 形でビルドして throw する。 */
async function callExchange(
  url: string,
  body: Record<string, string>,
  signal?: AbortSignal,
): Promise<Record<string, unknown>> {
  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      // Backend は Set-Cookie を出さないので credentials は不要。
      // 念のため明示的に "omit" にして、ブラウザ側 Cookie 送信もしない（Server-side 経路）。
      credentials: "omit",
      cache: "no-store",
      signal,
    });
  } catch {
    // network / fetch 例外は raw token を含む可能性のあるエラーメッセージを露出させない
    const err: ApiExchangeError = { kind: "network" };
    throw err;
  }
  if (res.status === 401) {
    const err: ApiExchangeError = { kind: "unauthorized" };
    throw err;
  }
  if (res.status === 400) {
    const err: ApiExchangeError = { kind: "bad_request" };
    throw err;
  }
  if (!res.ok) {
    const err: ApiExchangeError = { kind: "server_error" };
    throw err;
  }
  // 成功 body は JSON。型は呼び出し元が確定させる。
  return (await res.json()) as Record<string, unknown>;
}
