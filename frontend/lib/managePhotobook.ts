// Manage page API client（Server-side fetch wrapper）。
//
// 設計参照:
//   - docs/plan/m2-public-viewer-and-manage-plan.md §6 / §12
//
// セキュリティ:
//   - manage_url_token / Cookie 値はログに出さない
//   - 失敗詳細は kind だけを返す
//   - Server Component から呼ぶ前提（Cookie を Backend に転送するため Cookie ヘッダを手で組み立てる）

/** Backend のベース URL を取得する。 */
function getApiBaseUrl(): string {
  const url = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  if (url === "") {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return url.replace(/\/$/, "");
}

/** API のエラー種別。 */
export type ManageLookupError =
  | { kind: "unauthorized" }
  | { kind: "not_found" }
  | { kind: "server_error" }
  | { kind: "network" };

export type ManagePhotobook = {
  photobookId: string;
  type: string;
  title: string;
  status: string;
  visibility: string;
  sensitive: boolean;
  hiddenByOperator: boolean;
  publicUrlSlug?: string;
  publicUrlPath?: string;
  publishedAt?: string;
  deletedAt?: string;
  draftExpiresAt?: string;
  manageUrlTokenVersion: number;
  availableImageCount: number;
  /** photobook 本体の楽観ロック用 version。M-1a で PATCH の expected_version に使う。 */
  version: number;
};

/**
 * GET /api/manage/photobooks/{id} を呼び出す。
 *
 * Cookie ヘッダを手動で渡す（Server Component / Edge Runtime からの fetch では
 * `credentials: include` が直接効かないため、headers に Cookie を組み立てて転送する）。
 * 引数 cookieHeader は呼び出し元（Server Component）で `next/headers` から取得した値。
 *
 * 失敗時は ManageLookupError を throw。
 */
export async function fetchManagePhotobook(
  photobookId: string,
  cookieHeader: string,
  signal?: AbortSignal,
): Promise<ManagePhotobook> {
  const url = `${getApiBaseUrl()}/api/manage/photobooks/${encodeURIComponent(photobookId)}`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "GET",
      cache: "no-store",
      headers: cookieHeader === "" ? {} : { Cookie: cookieHeader },
      signal,
    });
  } catch {
    throw { kind: "network" } satisfies ManageLookupError;
  }

  if (res.status === 200) {
    const body = (await res.json()) as ApiManagePayload;
    return mapManagePayload(body);
  }
  if (res.status === 401) {
    throw { kind: "unauthorized" } satisfies ManageLookupError;
  }
  if (res.status === 404) {
    throw { kind: "not_found" } satisfies ManageLookupError;
  }
  throw { kind: "server_error" } satisfies ManageLookupError;
}

// === API レスポンス（snake_case）→ TS（camelCase）===

type ApiManagePayload = {
  photobook_id: string;
  type: string;
  title: string;
  status: string;
  visibility: string;
  sensitive: boolean;
  hidden_by_operator: boolean;
  public_url_slug?: string;
  public_url_path?: string;
  published_at?: string;
  deleted_at?: string;
  draft_expires_at?: string;
  manage_url_token_version: number;
  available_image_count: number;
  version: number;
};

function mapManagePayload(p: ApiManagePayload): ManagePhotobook {
  return {
    photobookId: p.photobook_id,
    type: p.type,
    title: p.title,
    status: p.status,
    visibility: p.visibility,
    sensitive: p.sensitive ?? false,
    hiddenByOperator: p.hidden_by_operator,
    publicUrlSlug: p.public_url_slug,
    publicUrlPath: p.public_url_path,
    publishedAt: p.published_at,
    deletedAt: p.deleted_at,
    draftExpiresAt: p.draft_expires_at,
    manageUrlTokenVersion: p.manage_url_token_version,
    availableImageCount: p.available_image_count,
    version: p.version ?? 0,
  };
}

/** Manage 経路エラー判定 type guard。 */
export function isManageLookupError(e: unknown): e is ManageLookupError {
  return typeof e === "object" && e !== null && "kind" in e;
}

// =============================================================================
// M-1a: Manage safety baseline mutation API
// -----------------------------------------------------------------------------
// 設計参照: docs/plan/m-1-manage-mvp-safety-plan.md §3
//
// 種別:
//   - 直接 Backend を叩く (credentials: "include" cross-origin):
//       updateVisibilityFromManage / updateSensitiveFromManage
//   - Workers Route Handler 経由（app domain Cookie 操作が必要）:
//       issueDraftSessionFromManage  → /manage/<id>/issue-draft (新 Route Handler)
//       revokeManageSession          → /manage/<id>/revoke-session (新 Route Handler)
//
// セキュリティ:
//   - raw session_token / Cookie は Route Handler 内で消費し、Frontend lib では受け取らない
//   - 失敗詳細は kind だけを返す（既存 ManageLookupError と同パターン）
// =============================================================================

/** Manage mutation API のエラー種別。 */
export type ManageMutationError =
  | { kind: "unauthorized" }
  | { kind: "not_found" }
  | { kind: "version_conflict" }
  | { kind: "public_change_not_allowed" }
  | { kind: "not_draft" }
  | { kind: "invalid_payload" }
  | { kind: "server_error" }
  | { kind: "network" };

export function isManageMutationError(e: unknown): e is ManageMutationError {
  return typeof e === "object" && e !== null && "kind" in e;
}

type Body409 = { status?: string; reason?: string };

function map409Reason(body: Body409): ManageMutationError {
  if (body.status === "manage_precondition_failed") {
    if (body.reason === "public_change_not_allowed") return { kind: "public_change_not_allowed" };
    if (body.reason === "not_draft") return { kind: "not_draft" };
  }
  return { kind: "version_conflict" };
}

async function manageMutate(
  url: string,
  method: "PATCH" | "POST" | "DELETE",
  body?: unknown,
): Promise<Response> {
  let res: Response;
  try {
    res = await fetch(url, {
      method,
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw { kind: "network" } satisfies ManageMutationError;
  }
  if (res.status >= 200 && res.status < 300) {
    return res;
  }
  if (res.status === 401) {
    throw { kind: "unauthorized" } satisfies ManageMutationError;
  }
  if (res.status === 404) {
    throw { kind: "not_found" } satisfies ManageMutationError;
  }
  if (res.status === 400) {
    throw { kind: "invalid_payload" } satisfies ManageMutationError;
  }
  if (res.status === 409) {
    let parsed: Body409 = {};
    try {
      parsed = (await res.json()) as Body409;
    } catch {
      // ignore body parse error; fallback to version_conflict
    }
    throw map409Reason(parsed);
  }
  throw { kind: "server_error" } satisfies ManageMutationError;
}

/** PATCH /api/manage/photobooks/{id}/visibility (unlisted/private のみ受理)。 */
export async function updateVisibilityFromManage(
  photobookId: string,
  visibility: "unlisted" | "private",
  expectedVersion: number,
): Promise<{ version: number }> {
  const url = `${getApiBaseUrl()}/api/manage/photobooks/${encodeURIComponent(photobookId)}/visibility`;
  const res = await manageMutate(url, "PATCH", {
    visibility,
    expected_version: expectedVersion,
  });
  const body = (await res.json()) as { version: number };
  return { version: body.version };
}

/** PATCH /api/manage/photobooks/{id}/sensitive。 */
export async function updateSensitiveFromManage(
  photobookId: string,
  sensitive: boolean,
  expectedVersion: number,
): Promise<{ version: number }> {
  const url = `${getApiBaseUrl()}/api/manage/photobooks/${encodeURIComponent(photobookId)}/sensitive`;
  const res = await manageMutate(url, "PATCH", {
    sensitive,
    expected_version: expectedVersion,
  });
  const body = (await res.json()) as { version: number };
  return { version: body.version };
}

/**
 * 編集を再開: manage session から draft session を発行して /edit/<id> に遷移する。
 *
 * Workers Route Handler `/manage/<id>/issue-draft` (POST) を叩く。Route Handler が
 * Backend `/api/manage/photobooks/<id>/draft-session` を proxy で呼び、raw token を
 * 受け取って app-domain HttpOnly Cookie に書き込み、`{ edit_url }` を JSON で返す。
 *
 * 失敗 (not_draft / unauthorized / network) は ManageMutationError で throw。
 */
export async function issueDraftSessionFromManage(
  photobookId: string,
): Promise<{ editUrl: string }> {
  const url = `/manage/${encodeURIComponent(photobookId)}/issue-draft`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    });
  } catch {
    throw { kind: "network" } satisfies ManageMutationError;
  }
  if (res.status >= 200 && res.status < 300) {
    const body = (await res.json()) as { edit_url: string };
    return { editUrl: body.edit_url };
  }
  if (res.status === 401) throw { kind: "unauthorized" } satisfies ManageMutationError;
  if (res.status === 404) throw { kind: "not_found" } satisfies ManageMutationError;
  if (res.status === 409) {
    let parsed: Body409 = {};
    try {
      parsed = (await res.json()) as Body409;
    } catch {
      // ignore
    }
    throw map409Reason(parsed);
  }
  throw { kind: "server_error" } satisfies ManageMutationError;
}

/**
 * この端末の管理権限を削除: manage session を revoke して app-domain Cookie をクリア。
 *
 * Workers Route Handler `/manage/<id>/revoke-session` (POST) を叩く。Route Handler が
 * Backend `/api/manage/photobooks/<id>/session-revoke` を proxy で呼び、成功時に
 * app-domain manage Cookie を Max-Age=-1 でクリアする。
 *
 * raw token は受け取らない / 表示しない。
 */
export async function revokeManageSession(photobookId: string): Promise<void> {
  const url = `/manage/${encodeURIComponent(photobookId)}/revoke-session`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    });
  } catch {
    throw { kind: "network" } satisfies ManageMutationError;
  }
  if (res.status >= 200 && res.status < 300) {
    return;
  }
  if (res.status === 401) throw { kind: "unauthorized" } satisfies ManageMutationError;
  if (res.status === 404) throw { kind: "not_found" } satisfies ManageMutationError;
  throw { kind: "server_error" } satisfies ManageMutationError;
}
