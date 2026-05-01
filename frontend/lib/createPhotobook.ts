// POST /api/photobooks クライアント wrapper（Browser から呼ぶ）。
//
// 設計参照:
//   - docs/plan/m2-create-entry-plan.md §7（API 設計）
//   - .agents/rules/turnstile-defensive-guard.md L3（trim 後 empty なら fetch せず reject）
//
// セキュリティ:
//   - turnstile_token / draft_edit_token / photobook_id を console.* に出さない
//   - response の draft_edit_token は呼び出し側が即座に window.location.replace で消費する設計。
//     localStorage / sessionStorage に保存しない、再利用しない、URL bar に表示しない
//   - 失敗時の詳細は kind だけ返す。Cloudflare 側のエラー詳細を画面・log に出さない

const ENDPOINT = "/api/photobooks";

/** photobook_type の列挙値（backend `photobook_type.Parse` と一致）。 */
export const PHOTOBOOK_TYPES = [
  "event",
  "daily",
  "portfolio",
  "avatar",
  "world",
  "memory",
  "free",
] as const;

export type PhotobookType = (typeof PHOTOBOOK_TYPES)[number];

export type CreatePhotobookInput = {
  type: PhotobookType;
  /** 任意。空文字 / 100 文字以下。 */
  title?: string;
  /** 任意。空文字 / 50 文字以下。 */
  creatorDisplayName?: string;
  /** Turnstile widget で取得した token。trim 後 non-empty 必須。 */
  turnstileToken: string;
};

export type CreatePhotobookSuccess = {
  /** 即座に window.location.replace に渡す用。raw token を URL に含む。 */
  draftEditUrlPath: string;
  /** 参考用、UI 表示はしない。Frontend 側で値を保持・再利用しない。 */
  draftExpiresAt: string;
};

export type CreatePhotobookError =
  | { kind: "invalid_payload" }
  | { kind: "turnstile_failed" }
  | { kind: "turnstile_unavailable" }
  | { kind: "server_error" }
  | { kind: "network" };

/**
 * POST /api/photobooks を呼ぶ。
 *
 * L3 ガード: turnstileToken が trim 後 empty なら fetch せず即 reject（Backend へ余計な
 * 通信をしない、Turnstile 多層防御 L3 layer）。
 *
 * 成功時の response.draft_edit_token は本関数では返さない。**呼び出し側で即座に
 * `window.location.replace(out.draftEditUrlPath)` を呼ぶことで raw token を localStorage /
 * sessionStorage / URL bar / browser history に残さない設計**。
 */
export async function createPhotobook(
  input: CreatePhotobookInput,
): Promise<CreatePhotobookSuccess> {
  if (
    typeof input.turnstileToken !== "string" ||
    input.turnstileToken.trim() === ""
  ) {
    throw { kind: "turnstile_failed" } satisfies CreatePhotobookError;
  }

  const body = JSON.stringify({
    type: input.type,
    title: input.title ?? "",
    creator_display_name: input.creatorDisplayName ?? "",
    turnstile_token: input.turnstileToken,
  });

  let res: Response;
  try {
    res = await fetch(ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
      cache: "no-store",
    });
  } catch {
    throw { kind: "network" } satisfies CreatePhotobookError;
  }

  if (res.ok) {
    const json = (await res.json()) as {
      draft_edit_url_path?: string;
      draft_expires_at?: string;
    };
    if (
      typeof json.draft_edit_url_path !== "string" ||
      json.draft_edit_url_path === ""
    ) {
      throw { kind: "server_error" } satisfies CreatePhotobookError;
    }
    return {
      draftEditUrlPath: json.draft_edit_url_path,
      draftExpiresAt: json.draft_expires_at ?? "",
    };
  }

  switch (res.status) {
    case 400:
      throw { kind: "invalid_payload" } satisfies CreatePhotobookError;
    case 403:
      throw { kind: "turnstile_failed" } satisfies CreatePhotobookError;
    case 503:
      throw { kind: "turnstile_unavailable" } satisfies CreatePhotobookError;
    default:
      throw { kind: "server_error" } satisfies CreatePhotobookError;
  }
}

/** kind 判定 type guard。 */
export function isCreatePhotobookError(
  e: unknown,
): e is CreatePhotobookError {
  return (
    typeof e === "object" &&
    e !== null &&
    "kind" in e &&
    typeof (e as { kind: unknown }).kind === "string"
  );
}
