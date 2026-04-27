// Edit page API client（Server / Client 両方から呼ぶ）。
//
// 設計参照:
//   - docs/plan/m2-frontend-edit-ui-fullspec-plan.md §4 / §6
//
// セキュリティ:
//   - storage_key 完全値 / presigned URL は console.log しない
//   - 失敗詳細は kind だけを返し、内容を画面に出さない
//   - Server Component から呼ぶときは Cookie ヘッダを手動で渡す
//   - Client Component から呼ぶときは credentials: include + 同 origin proxy
//     (本実装は Cross-origin / Edge runtime のため Cookie ヘッダを直接転送するパターン)

function getApiBaseUrl(): string {
  const url = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  if (url === "") {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return url.replace(/\/$/, "");
}

/** API のエラー種別。 */
export type EditApiError =
  | { kind: "unauthorized" }
  | { kind: "not_found" }
  | { kind: "bad_request" }
  | { kind: "version_conflict" }
  | { kind: "server_error" }
  | { kind: "network" };

export function isEditApiError(e: unknown): e is EditApiError {
  return typeof e === "object" && e !== null && "kind" in e;
}

// === edit-view types ===

export type EditPresignedURL = {
  url: string;
  width: number;
  height: number;
  expiresAt: string;
};

export type EditVariantSet = {
  display: EditPresignedURL;
  thumbnail: EditPresignedURL;
};

export type EditPhoto = {
  photoId: string;
  imageId: string;
  displayOrder: number;
  caption?: string;
  variants: EditVariantSet;
};

export type EditPage = {
  pageId: string;
  displayOrder: number;
  caption?: string;
  photos: EditPhoto[];
};

export type EditSettings = {
  type: string;
  title: string;
  description?: string;
  layout: string;
  openingStyle: string;
  visibility: string;
  coverTitle?: string;
};

export type EditView = {
  photobookId: string;
  status: string;
  version: number;
  settings: EditSettings;
  coverImageId?: string;
  cover?: EditVariantSet;
  pages: EditPage[];
  processingCount: number;
  failedCount: number;
  draftExpiresAt?: string;
};

/** Server Component から edit-view を取得する（Cookie ヘッダを手動転送）。 */
export async function fetchEditView(
  photobookId: string,
  cookieHeader: string,
  signal?: AbortSignal,
): Promise<EditView> {
  const url = `${getApiBaseUrl()}/api/photobooks/${encodeURIComponent(photobookId)}/edit-view`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "GET",
      cache: "no-store",
      headers: cookieHeader === "" ? {} : { Cookie: cookieHeader },
      signal,
    });
  } catch {
    throw { kind: "network" } satisfies EditApiError;
  }
  if (res.status === 200) {
    const body = (await res.json()) as ApiEditViewPayload;
    return mapEditViewPayload(body);
  }
  throw mapStatusToError(res.status);
}

function mapStatusToError(status: number): EditApiError {
  if (status === 401) return { kind: "unauthorized" };
  if (status === 404) return { kind: "not_found" };
  if (status === 400) return { kind: "bad_request" };
  if (status === 409) return { kind: "version_conflict" };
  return { kind: "server_error" };
}

// === mutation API（Client Component から呼ぶ、credentials: include） ===

async function mutate(
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
    throw { kind: "network" } satisfies EditApiError;
  }
  if (res.status >= 200 && res.status < 300) {
    return res;
  }
  throw mapStatusToError(res.status);
}

/** photo caption を更新（null/空文字でクリア）。 */
export async function updatePhotoCaption(
  photobookId: string,
  photoId: string,
  caption: string | null,
  expectedVersion: number,
): Promise<void> {
  const url = `${getApiBaseUrl()}/api/photobooks/${encodeURIComponent(photobookId)}/photos/${encodeURIComponent(photoId)}/caption`;
  await mutate(url, "PATCH", { caption, expected_version: expectedVersion });
}

export type ReorderItem = { photoId: string; displayOrder: number };

/** 同 page 内の photo を一括 reorder。 */
export async function bulkReorderPhotos(
  photobookId: string,
  pageId: string,
  assignments: ReorderItem[],
  expectedVersion: number,
): Promise<void> {
  const url = `${getApiBaseUrl()}/api/photobooks/${encodeURIComponent(photobookId)}/photos/reorder`;
  await mutate(url, "PATCH", {
    page_id: pageId,
    assignments: assignments.map((a) => ({
      photo_id: a.photoId,
      display_order: a.displayOrder,
    })),
    expected_version: expectedVersion,
  });
}

/** cover_image_id を設定。 */
export async function setCoverImage(
  photobookId: string,
  imageId: string,
  expectedVersion: number,
): Promise<void> {
  const url = `${getApiBaseUrl()}/api/photobooks/${encodeURIComponent(photobookId)}/cover-image`;
  await mutate(url, "PATCH", { image_id: imageId, expected_version: expectedVersion });
}

/** cover_image_id をクリア。 */
export async function clearCoverImage(
  photobookId: string,
  expectedVersion: number,
): Promise<void> {
  const url = `${getApiBaseUrl()}/api/photobooks/${encodeURIComponent(photobookId)}/cover-image`;
  await mutate(url, "DELETE", { expected_version: expectedVersion });
}

/** settings 一括更新。 */
export async function updatePhotobookSettings(
  photobookId: string,
  settings: EditSettings,
  expectedVersion: number,
): Promise<void> {
  const url = `${getApiBaseUrl()}/api/photobooks/${encodeURIComponent(photobookId)}/settings`;
  await mutate(url, "PATCH", {
    type: settings.type,
    title: settings.title,
    description: settings.description ?? null,
    layout: settings.layout,
    opening_style: settings.openingStyle,
    visibility: settings.visibility,
    cover_title: settings.coverTitle ?? null,
    expected_version: expectedVersion,
  });
}

/** photo を削除。 */
export async function removePhoto(
  photobookId: string,
  pageId: string,
  photoId: string,
  expectedVersion: number,
): Promise<void> {
  const url = `${getApiBaseUrl()}/api/photobooks/${encodeURIComponent(photobookId)}/photos/${encodeURIComponent(photoId)}`;
  await mutate(url, "DELETE", { page_id: pageId, expected_version: expectedVersion });
}

/** page を追加（display_order は Backend が決定）。 */
export async function addPage(
  photobookId: string,
  expectedVersion: number,
): Promise<{ pageId: string; displayOrder: number }> {
  const url = `${getApiBaseUrl()}/api/photobooks/${encodeURIComponent(photobookId)}/pages`;
  const res = await mutate(url, "POST", { expected_version: expectedVersion });
  const body = (await res.json()) as { page_id: string; display_order: number };
  return { pageId: body.page_id, displayOrder: body.display_order };
}

// === payload mapping ===

type ApiPresignedURL = { url: string; width: number; height: number; expires_at: string };
type ApiVariantSet = { display: ApiPresignedURL; thumbnail: ApiPresignedURL };
type ApiPhoto = { photo_id: string; image_id: string; display_order: number; caption?: string; variants: ApiVariantSet };
type ApiPage = { page_id: string; display_order: number; caption?: string; photos: ApiPhoto[] };
type ApiSettings = { type: string; title: string; description?: string; layout: string; opening_style: string; visibility: string; cover_title?: string };
type ApiEditViewPayload = {
  photobook_id: string;
  status: string;
  version: number;
  settings: ApiSettings;
  cover_image_id?: string;
  cover?: ApiVariantSet;
  pages: ApiPage[];
  processing_count: number;
  failed_count: number;
  draft_expires_at?: string;
};

function mapPresigned(p: ApiPresignedURL): EditPresignedURL {
  return { url: p.url, width: p.width, height: p.height, expiresAt: p.expires_at };
}

function mapVariantSet(v: ApiVariantSet): EditVariantSet {
  return { display: mapPresigned(v.display), thumbnail: mapPresigned(v.thumbnail) };
}

function mapEditViewPayload(p: ApiEditViewPayload): EditView {
  return {
    photobookId: p.photobook_id,
    status: p.status,
    version: p.version,
    settings: {
      type: p.settings.type,
      title: p.settings.title,
      description: p.settings.description,
      layout: p.settings.layout,
      openingStyle: p.settings.opening_style,
      visibility: p.settings.visibility,
      coverTitle: p.settings.cover_title,
    },
    coverImageId: p.cover_image_id,
    cover: p.cover ? mapVariantSet(p.cover) : undefined,
    pages: p.pages.map((pg) => ({
      pageId: pg.page_id,
      displayOrder: pg.display_order,
      caption: pg.caption,
      photos: pg.photos.map((ph) => ({
        photoId: ph.photo_id,
        imageId: ph.image_id,
        displayOrder: ph.display_order,
        caption: ph.caption,
        variants: mapVariantSet(ph.variants),
      })),
    })),
    processingCount: p.processing_count,
    failedCount: p.failed_count,
    draftExpiresAt: p.draft_expires_at,
  };
}
