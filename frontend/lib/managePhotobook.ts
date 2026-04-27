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
  hiddenByOperator: boolean;
  publicUrlSlug?: string;
  publicUrlPath?: string;
  publishedAt?: string;
  deletedAt?: string;
  draftExpiresAt?: string;
  manageUrlTokenVersion: number;
  availableImageCount: number;
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
  hidden_by_operator: boolean;
  public_url_slug?: string;
  public_url_path?: string;
  published_at?: string;
  deleted_at?: string;
  draft_expires_at?: string;
  manage_url_token_version: number;
  available_image_count: number;
};

function mapManagePayload(p: ApiManagePayload): ManagePhotobook {
  return {
    photobookId: p.photobook_id,
    type: p.type,
    title: p.title,
    status: p.status,
    visibility: p.visibility,
    hiddenByOperator: p.hidden_by_operator,
    publicUrlSlug: p.public_url_slug,
    publicUrlPath: p.public_url_path,
    publishedAt: p.published_at,
    deletedAt: p.deleted_at,
    draftExpiresAt: p.draft_expires_at,
    manageUrlTokenVersion: p.manage_url_token_version,
    availableImageCount: p.available_image_count,
  };
}

/** Manage 経路エラー判定 type guard。 */
export function isManageLookupError(e: unknown): e is ManageLookupError {
  return typeof e === "object" && e !== null && "kind" in e;
}
