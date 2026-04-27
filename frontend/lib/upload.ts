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

const ALLOWED_CONTENT_TYPES = new Set([
  "image/jpeg",
  "image/png",
  "image/webp",
  "image/heic",
]);

export const MAX_UPLOAD_BYTE_SIZE = 10 * 1024 * 1024; // 10MB

/** ファイル軽量検証の結果。失敗時は kind を返す。 */
export type FileValidationError =
  | { kind: "invalid_type" }
  | { kind: "too_large" };

/** File を Frontend 側で軽量検証する。 */
export function validateFile(file: File): FileValidationError | null {
  // HEIC は browser によって content_type が空になる場合がある。拡張子で fallback。
  let ct = file.type;
  if (!ct) {
    const lower = file.name.toLowerCase();
    if (lower.endsWith(".heic") || lower.endsWith(".heif")) {
      ct = "image/heic";
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
export function sourceFormatOf(contentType: string): "jpg" | "png" | "webp" | "heic" | null {
  switch (contentType) {
    case "image/jpeg":
      return "jpg";
    case "image/png":
      return "png";
    case "image/webp":
      return "webp";
    case "image/heic":
      return "heic";
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
  | { kind: "network" };

/** POST /api/photobooks/{id}/upload-verifications */
export async function issueUploadVerification(
  photobookId: string,
  turnstileToken: string,
  signal?: AbortSignal,
): Promise<UploadVerificationResponse> {
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
