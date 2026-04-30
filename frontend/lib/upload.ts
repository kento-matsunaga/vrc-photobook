// Upload API client wrapper（Client-side、Browser から呼ぶ）。
//
// 設計参照:
//   - docs/plan/m2-frontend-upload-ui-plan.md §5 / §6
//   - docs/adr/0005-image-upload-flow.md
//
// セキュリティ:
//   - Turnstile token / upload_verification_token / presigned URL / Cookie 値はログ出力しない
//   - 失敗時のエラー詳細は console / 画面に出さない（kind だけを返す）
//   - file name は upload-intent body に乗せない（DB 保存防止）

// PR22.5: HEIC / HEIF は PR23 で image-processor が unsupported_format に倒すため、
// 本リリースでは Frontend 側でも明示的に拒否する。Backend は引き続き
// unsupported_format で防御する。HEIC 本対応（libheif + cgo）は PR25 以降に分離。
const ALLOWED_CONTENT_TYPES = new Set([
  "image/jpeg",
  "image/png",
  "image/webp",
]);

const REJECTED_HEIC_CONTENT_TYPES = new Set([
  "image/heic",
  "image/heif",
  "image/heic-sequence",
  "image/heif-sequence",
]);

const REJECTED_HEIC_EXTENSIONS = [".heic", ".heif", ".hif"];

export const MAX_UPLOAD_BYTE_SIZE = 10 * 1024 * 1024; // 10MB

/** ファイル軽量検証の結果。失敗時は kind を返す。 */
export type FileValidationError =
  | { kind: "invalid_type" }
  | { kind: "heic_unsupported" }
  | { kind: "too_large" };

/** File を Frontend 側で軽量検証する。
 *
 * PR22.5: HEIC / HEIF は明示的に `heic_unsupported` で拒否する（content_type と拡張子の
 * 両軸でガード）。
 */
export function validateFile(file: File): FileValidationError | null {
  const ct = file.type;
  const lower = file.name.toLowerCase();

  // 明示的な HEIC/HEIF 拒否
  if (REJECTED_HEIC_CONTENT_TYPES.has(ct)) {
    return { kind: "heic_unsupported" };
  }
  for (const ext of REJECTED_HEIC_EXTENSIONS) {
    if (lower.endsWith(ext)) {
      return { kind: "heic_unsupported" };
    }
  }

  if (!ALLOWED_CONTENT_TYPES.has(ct)) {
    return { kind: "invalid_type" };
  }
  if (file.size > MAX_UPLOAD_BYTE_SIZE) {
    return { kind: "too_large" };
  }
  if (file.size < 1) {
    return { kind: "too_large" };
  }
  return null;
}

/** content_type → source_format mapping（Backend whitelist と一致）。 */
export function sourceFormatOf(contentType: string): "jpg" | "png" | "webp" | null {
  switch (contentType) {
    case "image/jpeg":
      return "jpg";
    case "image/png":
      return "png";
    case "image/webp":
      return "webp";
    default:
      return null;
  }
}

/** API のベース URL。NEXT_PUBLIC_API_BASE_URL は build 時に inline される公開値。 */
export function getApiBaseUrl(): string {
  const url = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  if (url === "") {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return url.replace(/\/$/, "");
}

export type UploadVerificationResponse = {
  uploadVerificationToken: string;
  expiresAt: string;
  allowedIntentCount: number;
};

export type UploadIntentResponse = {
  imageId: string;
  uploadUrl: string;
  requiredHeaders: Record<string, string>;
  storageKey: string;
  expiresAt: string;
};

export type CompleteResponse = {
  imageId: string;
  status: string;
};

/** 一連の upload 失敗を Frontend 側で扱う種別。 */
export type UploadError =
  | { kind: "verification_failed" }
  | { kind: "turnstile_unavailable" }
  | { kind: "invalid_parameters" }
  | { kind: "upload_failed" }
  | { kind: "complete_failed" }
  | { kind: "validation_failed" }
  | { kind: "rate_limited"; retryAfterSeconds: number }
  | { kind: "network" };

/** extractRetryAfterSeconds は 429 response から再試行秒数を取り出す。
 *
 * 優先順:
 *   1. `Retry-After` header（数値秒、HTTP-date は MVP 非対応）
 *   2. body.retry_after_seconds
 *   3. 既定 60 秒
 *
 * 0 / 負 / NaN / 無効値はすべて 1 秒に丸める（最低 1 秒）。
 */
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

/** POST /api/photobooks/{id}/upload-verifications
 *
 * L3 defensive guard: 空 / 空白文字 token は Backend へ送らず即拒否する。
 * Frontend disabled / startUpload guard と多層防御。
 */
export async function issueUploadVerification(
  photobookId: string,
  turnstileToken: string,
  signal?: AbortSignal,
): Promise<UploadVerificationResponse> {
  if (typeof turnstileToken !== "string" || turnstileToken.trim() === "") {
    throw { kind: "verification_failed" } satisfies UploadError;
  }
  const url = `${getApiBaseUrl()}/api/photobooks/${photobookId}/upload-verifications/`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      credentials: "include", // draft session Cookie を送る
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ turnstile_token: turnstileToken }),
      cache: "no-store",
      signal,
    });
  } catch {
    throw { kind: "network" } satisfies UploadError;
  }
  if (res.status === 403) {
    throw { kind: "verification_failed" } satisfies UploadError;
  }
  if (res.status === 429) {
    const retryAfterSeconds = await extractRetryAfterSeconds(res);
    throw { kind: "rate_limited", retryAfterSeconds } satisfies UploadError;
  }
  if (res.status === 503) {
    throw { kind: "turnstile_unavailable" } satisfies UploadError;
  }
  if (!res.ok) {
    throw { kind: "verification_failed" } satisfies UploadError;
  }
  const body = (await res.json()) as Record<string, unknown>;
  return {
    uploadVerificationToken: body["upload_verification_token"] as string,
    expiresAt: body["expires_at"] as string,
    allowedIntentCount: body["allowed_intent_count"] as number,
  };
}

/** POST /api/photobooks/{id}/images/upload-intent */
export async function issueUploadIntent(
  photobookId: string,
  uploadVerificationToken: string,
  contentType: string,
  declaredByteSize: number,
  sourceFormat: string,
  signal?: AbortSignal,
): Promise<UploadIntentResponse> {
  const url = `${getApiBaseUrl()}/api/photobooks/${photobookId}/images/upload-intent`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${uploadVerificationToken}`,
      },
      body: JSON.stringify({
        content_type: contentType,
        declared_byte_size: declaredByteSize,
        source_format: sourceFormat,
      }),
      cache: "no-store",
      signal,
    });
  } catch {
    throw { kind: "network" } satisfies UploadError;
  }
  if (res.status === 400) {
    throw { kind: "invalid_parameters" } satisfies UploadError;
  }
  if (res.status === 403) {
    throw { kind: "verification_failed" } satisfies UploadError;
  }
  if (!res.ok) {
    throw { kind: "verification_failed" } satisfies UploadError;
  }
  const body = (await res.json()) as Record<string, unknown>;
  return {
    imageId: body["image_id"] as string,
    uploadUrl: body["upload_url"] as string,
    requiredHeaders: (body["required_headers"] as Record<string, string>) ?? {},
    storageKey: body["storage_key"] as string,
    expiresAt: body["expires_at"] as string,
  };
}

/** R2 への直接 PUT（presigned URL）。 */
export async function putToR2(
  uploadUrl: string,
  contentType: string,
  file: File | Blob,
  signal?: AbortSignal,
): Promise<void> {
  let res: Response;
  try {
    res = await fetch(uploadUrl, {
      method: "PUT",
      headers: {
        "Content-Type": contentType,
        // Content-Length は fetch が自動付与（明示できない、ただし File.size と一致する）
      },
      body: file,
      // R2 endpoint は別 origin。Cookie / credential は不要（presigned URL に署名が含まれる）
      credentials: "omit",
      cache: "no-store",
      signal,
    });
  } catch {
    throw { kind: "network" } satisfies UploadError;
  }
  if (!res.ok) {
    throw { kind: "upload_failed" } satisfies UploadError;
  }
}

/** POST /api/photobooks/{id}/images/{imageId}/complete */
export async function completeUpload(
  photobookId: string,
  imageId: string,
  storageKey: string,
  signal?: AbortSignal,
): Promise<CompleteResponse> {
  const url = `${getApiBaseUrl()}/api/photobooks/${photobookId}/images/${imageId}/complete`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ storage_key: storageKey }),
      cache: "no-store",
      signal,
    });
  } catch {
    throw { kind: "network" } satisfies UploadError;
  }
  if (res.status === 422) {
    throw { kind: "validation_failed" } satisfies UploadError;
  }
  if (!res.ok) {
    throw { kind: "complete_failed" } satisfies UploadError;
  }
  const body = (await res.json()) as Record<string, unknown>;
  return {
    imageId: body["image_id"] as string,
    status: body["status"] as string,
  };
}
