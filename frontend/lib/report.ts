// Report submission API client（Browser-side fetch wrapper）。
//
// 設計参照:
//   - docs/plan/m2-report-plan.md §6
//
// セキュリティ:
//   - reporter_contact / detail / Turnstile token / source_ip_hash を log に出さない
//   - report_id は thanks view に表示しない（PR35a §16 #7）
//   - 失敗詳細は kind のみ返す（敵対者対策で内部理由を漏らさない）

/** Backend のベース URL を取得する。 */
function getApiBaseUrl(): string {
  const url = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  if (url === "") {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return url.replace(/\/$/, "");
}

/** Report 送信 API のエラー種別。 */
export type SubmitReportError =
  | { kind: "invalid_payload" }
  | { kind: "turnstile_failed" }
  | { kind: "not_found" }
  | { kind: "server_error" }
  | { kind: "network" };

export type SubmitReportInput = {
  slug: string;
  reason: string;
  detail?: string;
  reporterContact?: string;
  turnstileToken: string;
};

/**
 * POST /api/public/photobooks/{slug}/reports を呼び出す。
 *
 * 成功時は何も返さない（thanks view では report_id を表示しないため、
 * Frontend では成否のみ扱う）。失敗時は SubmitReportError を throw。
 *
 * Cookie 不要 endpoint（公開機能）。Backend で Turnstile siteverify + Origin 検証。
 */
export async function submitReport(in_: SubmitReportInput): Promise<void> {
  const url = `${getApiBaseUrl()}/api/public/photobooks/${encodeURIComponent(in_.slug)}/reports`;
  const body = {
    reason: in_.reason,
    detail: in_.detail ?? "",
    reporter_contact: in_.reporterContact ?? "",
    turnstile_token: in_.turnstileToken,
  };
  let res: Response;
  try {
    res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      // 公開 endpoint のため credentials は送らない
    });
  } catch {
    throw { kind: "network" } satisfies SubmitReportError;
  }
  if (res.status === 201) {
    return;
  }
  switch (res.status) {
    case 400:
      throw { kind: "invalid_payload" } satisfies SubmitReportError;
    case 403:
      throw { kind: "turnstile_failed" } satisfies SubmitReportError;
    case 404:
      throw { kind: "not_found" } satisfies SubmitReportError;
    default:
      throw { kind: "server_error" } satisfies SubmitReportError;
  }
}

/** SubmitReportError かどうかを判定する。 */
export function isSubmitReportError(e: unknown): e is SubmitReportError {
  return (
    typeof e === "object" &&
    e !== null &&
    "kind" in e &&
    typeof (e as { kind: unknown }).kind === "string"
  );
}

/** PR35a §3.6 / 計画書 §7.2 に従う 6 種の reason 定義（Frontend select 用）。 */
export const REPORT_REASONS: ReadonlyArray<{ value: string; label: string }> = [
  { value: "harassment_or_doxxing", label: "嫌がらせ・晒し" },
  { value: "unauthorized_repost", label: "無断転載の可能性" },
  { value: "subject_removal_request", label: "被写体として削除希望" },
  { value: "sensitive_flag_missing", label: "センシティブ設定の不足" },
  { value: "minor_safety_concern", label: "年齢・センシティブに関する問題" },
  { value: "other", label: "その他" },
];
