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

/**
 * publish API のエラー種別。
 *
 * 2026-05-03 STOP α P0 v2: 旧来は 409 を一律 `version_conflict` に丸めていたが、
 * authenticated owner 向けの復旧導線のため `publish_precondition_failed` (reason 付き)
 * を分離する。
 *
 *   - version_conflict: 楽観ロック / 状態競合 → 「最新を取得」CTA を案内
 *   - publish_precondition_failed: ユーザが直すべき公開前提 → reason 別文言を出す
 */
export type PublishPreconditionReason =
  | "rights_not_agreed"
  | "not_draft"
  | "empty_creator"
  | "empty_title"
  | "unknown_precondition";

export type PublishApiError =
  | { kind: "unauthorized" }
  | { kind: "not_found" }
  | { kind: "bad_request" }
  | { kind: "version_conflict" }
  | { kind: "publish_precondition_failed"; reason: PublishPreconditionReason }
  | { kind: "rate_limited"; retryAfterSeconds: number }
  | { kind: "server_error" }
  | { kind: "network" };

/** 409 response body から status / reason を解釈する。形式不正なら version_conflict 既定。 */
async function parse409Body(res: Response): Promise<PublishApiError> {
  try {
    const body = (await res
      .clone()
      .json()
      .catch(() => null)) as { status?: unknown; reason?: unknown } | null;
    if (body !== null) {
      const status = typeof body.status === "string" ? body.status : "";
      if (status === "publish_precondition_failed") {
        const raw = typeof body.reason === "string" ? body.reason : "";
        const reason = normalizePublishPreconditionReason(raw);
        return { kind: "publish_precondition_failed", reason };
      }
      if (status === "version_conflict") {
        return { kind: "version_conflict" };
      }
    }
  } catch {
    // body 解釈失敗は version_conflict 既定（旧互換）
  }
  return { kind: "version_conflict" };
}

function normalizePublishPreconditionReason(s: string): PublishPreconditionReason {
  if (
    s === "rights_not_agreed" ||
    s === "not_draft" ||
    s === "empty_creator" ||
    s === "empty_title"
  ) {
    return s;
  }
  return "unknown_precondition";
}

/** 429 response から再試行秒数を取り出す（Retry-After header → body → 既定 60 秒）。 */
async function extractRetryAfterSeconds(res: Response): Promise<number> {
  const hdr = res.headers.get("Retry-After");
  const fromHdr = hdr !== null ? Number(hdr) : NaN;
  if (Number.isFinite(fromHdr) && fromHdr >= 1) {
    return Math.floor(fromHdr);
  }
  try {
    const body = (await res
      .clone()
      .json()
      .catch(() => null)) as { retry_after_seconds?: unknown } | null;
    const fromBody = body?.retry_after_seconds;
    if (typeof fromBody === "number" && Number.isFinite(fromBody) && fromBody >= 1) {
      return Math.floor(fromBody);
    }
  } catch {
    // body 解釈失敗は既定値で進む
  }
  return 60;
}

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
 *
 * 2026-05-03 STOP α P0 v2: rights_agreed フラグを必須化（業務知識 v4 §3.1）。
 * false / 不在の場合 Backend が 409 publish_precondition_failed reason=rights_not_agreed
 * を返す。Frontend 側の checkbox 同意を request に乗せる経路で使う。
 */
export async function publishPhotobook(
  photobookId: string,
  expectedVersion: number,
  rightsAgreed: boolean,
): Promise<PublishResult> {
  const url = `${getApiBaseUrl()}/api/photobooks/${encodeURIComponent(photobookId)}/publish`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        expected_version: expectedVersion,
        rights_agreed: rightsAgreed,
      }),
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
  if (res.status === 409) {
    throw await parse409Body(res);
  }
  if (res.status === 429) {
    const retryAfterSeconds = await extractRetryAfterSeconds(res);
    throw { kind: "rate_limited", retryAfterSeconds } satisfies PublishApiError;
  }
  throw { kind: "server_error" } satisfies PublishApiError;
}

type ApiPublishResponse = {
  photobook_id: string;
  slug: string;
  public_url_path: string;
  manage_url_path: string;
  published_at: string;
};
