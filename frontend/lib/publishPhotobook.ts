// Publish API client（PR28）。
//
// 設計参照:
//   - docs/plan/m2-frontend-edit-ui-fullspec-plan.md §10
//
// セキュリティ:
//   - response の manage_url_path は raw token を含む。**console.log しない**
//   - manage URL は本コミット / work-log には書かない（業務知識 v4: 再表示禁止）
//   - 401 / 404 / 409 を kind で返す

function getApiBaseUrl(): string {
  const url = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  if (url === "") {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return url.replace(/\/$/, "");
}

export type PublishApiError =
  | { kind: "unauthorized" }
  | { kind: "not_found" }
  | { kind: "bad_request" }
  | { kind: "version_conflict" }
  | { kind: "server_error" }
  | { kind: "network" };

export function isPublishApiError(e: unknown): e is PublishApiError {
  return typeof e === "object" && e !== null && "kind" in e;
}

export type PublishResult = {
  photobookId: string;
  slug: string;
  publicUrlPath: string; // "/p/{slug}"
  manageUrlPath: string; // "/manage/token/{raw}"
  publishedAt: string;
};

/**
 * draft photobook を publish する。
 *
 * 戻り値の manageUrlPath は raw token を含む。**1 回限り**ユーザーに見せ、再取得不可。
 */
export async function publishPhotobook(
  photobookId: string,
  expectedVersion: number,
): Promise<PublishResult> {
  const url = `${getApiBaseUrl()}/api/photobooks/${encodeURIComponent(photobookId)}/publish`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ expected_version: expectedVersion }),
    });
  } catch {
    throw { kind: "network" } satisfies PublishApiError;
  }
  if (res.status === 200) {
    const body = (await res.json()) as ApiPublishResponse;
    return {
      photobookId: body.photobook_id,
      slug: body.slug,
      publicUrlPath: body.public_url_path,
      manageUrlPath: body.manage_url_path,
      publishedAt: body.published_at,
    };
  }
  if (res.status === 401) throw { kind: "unauthorized" } satisfies PublishApiError;
  if (res.status === 404) throw { kind: "not_found" } satisfies PublishApiError;
  if (res.status === 400) throw { kind: "bad_request" } satisfies PublishApiError;
  if (res.status === 409) throw { kind: "version_conflict" } satisfies PublishApiError;
  throw { kind: "server_error" } satisfies PublishApiError;
}

type ApiPublishResponse = {
  photobook_id: string;
  slug: string;
  public_url_path: string;
  manage_url_path: string;
  published_at: string;
};
