// OGP readiness Client API（M-2 STOP δ、ADR-0007）。
//
// 設計参照:
//   - docs/plan/m2-ogp-sync-publish-plan.md §STOP δ
//   - docs/adr/0007-ogp-sync-publish-fallback.md §3 (4) Frontend polling
//
// 役割:
//   - Complete 画面 (`components/Complete/CompleteView.tsx`) から 2 s 間隔 / 最大 30 s
//     polling し、Backend の OGP 同期生成または outbox-worker fallback が `generated`
//     状態に至ったかを確認する
//   - polling 失敗 (network / 500 / 例外) は **公開完了扱いを壊さない**。呼び出し側は
//     ready / not-ready の 2 値で UI 制御する
//
// エンドポイント:
//   - GET /api/public/photobooks/{photobookId}/ogp
//   - 認証不要の public endpoint（Cookie / credentials は付けない）
//   - response: { status: string, version: number, image_url_path: string }
//
// セキュリティ:
//   - photobookId は public endpoint の path に出るが、Frontend log / DOM / 報告には
//     raw 値を残さない。本 lib も console.log しない
//   - 失敗時は status 文字列にだけ集約し、内部 error 詳細は外に出さない

/** Backend のベース URL を取得する。public endpoint なので Cookie は付けない。 */
function getApiBaseUrl(): string {
  const url = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  if (url === "") {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return url.replace(/\/$/, "");
}

/**
 * OGP readiness の状態。
 *
 * - `generated`: OGP image が R2 に存在し crawler 取得可能（共有 OK）
 * - `pending`: row はあるが画像未完成（同期失敗 → worker fallback 待ち）
 * - `not_found`: photobook_ogp_images row 未作成（公開直後の極短時間など）
 * - `error`: Backend が `status=error` を返した / network・parse 失敗
 *
 * `pending` / `not_found` / `error` のいずれも **呼び出し側は polling 継続** が既定動作。
 * timeout (30 s) に達した時点で「OGP 反映に時間がかかっています」案内 + 共有ボタン enable に倒す。
 */
export type OgpReadinessStatus = "generated" | "pending" | "not_found" | "error";

/** Backend OGP lookup endpoint からの response shape（public_handler.go と一致）。 */
type ogpLookupResponse = {
  status?: unknown;
  version?: unknown;
  image_url_path?: unknown;
};

/**
 * OGP readiness を 1 回確認する Client 用関数。
 *
 * - 失敗時は `"error"` を返し例外を投げない（呼び出し側 polling ループを壊さないため）
 * - 既知の status (`generated` / `pending` / `not_found` / `error`) 以外は `"error"` に丸める
 *
 * @param photobookId polling 対象の photobook ID（raw 値、log / DOM には残さないこと）
 * @param signal      abort 用 signal
 */
export async function fetchOgpReadinessClient(
  photobookId: string,
  signal?: AbortSignal,
): Promise<OgpReadinessStatus> {
  let url: string;
  try {
    url = `${getApiBaseUrl()}/api/public/photobooks/${encodeURIComponent(photobookId)}/ogp`;
  } catch {
    return "error";
  }
  let res: Response;
  try {
    res = await fetch(url, {
      method: "GET",
      cache: "no-store",
      signal,
    });
  } catch {
    return "error";
  }
  if (!res.ok && res.status !== 404) {
    return "error";
  }
  let body: ogpLookupResponse | null;
  try {
    body = (await res.json()) as ogpLookupResponse;
  } catch {
    return "error";
  }
  if (!body || typeof body.status !== "string") {
    return "error";
  }
  return normalizeStatus(body.status);
}

/** 既知の status 文字列に正規化する。未知は `"error"`。 */
function normalizeStatus(raw: string): OgpReadinessStatus {
  switch (raw) {
    case "generated":
      return "generated";
    case "pending":
      return "pending";
    case "not_found":
      return "not_found";
    default:
      return "error";
  }
}
