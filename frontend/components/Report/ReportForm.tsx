// ReportForm: 通報フォーム client component。
//
// 設計参照:
//   - docs/plan/m2-report-plan.md §7 / §11
//
// 仕様:
//   - reason select / detail textarea / reporter_contact optional / Turnstile widget
//   - Turnstile 必須（token 取得後のみ submit ボタン有効）
//   - submit 中は disabled（重複送信防止）
//   - 成功後は thanks view に切替（report_id は表示しない、PR35a §16 #7）
//   - エラー時はエラー状態に応じた文言を表示
//
// セキュリティ:
//   - Turnstile token / detail / reporter_contact 値を console.log しない
//   - URL に値を出さない（POST body 経由のみ）
//   - report_id を表示・URL に出さない
"use client";

import { useCallback, useState } from "react";

import { TurnstileWidget } from "@/components/TurnstileWidget";
import {
  REPORT_REASONS,
  isSubmitReportError,
  submitReport,
  type SubmitReportError,
} from "@/lib/report";
import { formatRetryAfterDisplay } from "@/lib/retryAfter";

const MAX_DETAIL_LEN = 2000;
const MAX_CONTACT_LEN = 200;

type FormState = "idle" | "submitting" | "success" | "error";

type Props = {
  slug: string;
  turnstileSiteKey: string;
};

export function ReportForm({ slug, turnstileSiteKey }: Props) {
  const [reason, setReason] = useState<string>(REPORT_REASONS[0]?.value ?? "harassment_or_doxxing");
  const [detail, setDetail] = useState<string>("");
  const [contact, setContact] = useState<string>("");
  const [turnstileToken, setTurnstileToken] = useState<string>("");
  const [formState, setFormState] = useState<FormState>("idle");
  const [errorMessage, setErrorMessage] = useState<string>("");

  // Turnstile callback は useCallback で安定参照を維持する。TurnstileWidget 内部は
  // useRef で最新版を呼ぶため再 mount は起きないが、防御的に親側でも安定化する
  // （`harness/failure-log/2026-04-29_turnstile-widget-remount-loop.md`）。
  const handleTurnstileVerify = useCallback((token: string) => {
    setTurnstileToken(token);
  }, []);
  const handleTurnstileError = useCallback(() => {
    setTurnstileToken("");
  }, []);
  const handleTurnstileExpired = useCallback(() => {
    setTurnstileToken("");
  }, []);
  const handleTurnstileTimeout = useCallback(() => {
    setTurnstileToken("");
  }, []);

  // L1: 多層防御 Turnstile ガード（`.agents/rules/turnstile-defensive-guard.md`）。
  // 空白のみのトークンは未完了扱い（widget 中断 / 古い state 残存対策）。
  const isTurnstileReady =
    typeof turnstileToken === "string" && turnstileToken.trim() !== "";
  const canSubmit = isTurnstileReady && formState !== "submitting";

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    // L2: 多層防御 Turnstile ガード（button disable を JS から強制発火された場合の保険）。
    if (!isTurnstileReady) {
      setFormState("error");
      setErrorMessage(
        "bot 検証が完了していません。Turnstile のチェックが完了してから送信してください。",
      );
      return;
    }
    if (formState === "submitting") return;
    if (detail.length > MAX_DETAIL_LEN) {
      setFormState("error");
      setErrorMessage("詳細は 2000 文字以内で入力してください。");
      return;
    }
    if (contact.length > MAX_CONTACT_LEN) {
      setFormState("error");
      setErrorMessage("連絡先は 200 文字以内で入力してください。");
      return;
    }
    setFormState("submitting");
    setErrorMessage("");
    try {
      await submitReport({
        slug,
        reason,
        detail: detail !== "" ? detail : undefined,
        reporterContact: contact !== "" ? contact : undefined,
        turnstileToken,
      });
      setFormState("success");
    } catch (e) {
      setFormState("error");
      setErrorMessage(mapErrorMessage(e));
    }
  }

  if (formState === "success") {
    return (
      <section
        role="status"
        aria-live="polite"
        data-testid="report-thanks-view"
        className="rounded-lg border border-divider-soft bg-surface-soft p-6 text-center"
      >
        <h2 className="text-h2 text-ink">通報を受け付けました</h2>
        <p className="mt-3 text-sm text-ink-medium">
          ご報告ありがとうございました。運営が確認のうえ、必要に応じて対応します。
        </p>
        <p className="mt-2 text-xs text-ink-soft">
          通報の進捗・結果のお知らせは、連絡先を入力された場合に運営判断で行うことがあります。
        </p>
      </section>
    );
  }

  return (
    <form onSubmit={onSubmit} className="space-y-6" data-testid="report-form">
      <div>
        <label htmlFor="report-reason" className="block text-sm font-medium text-ink-strong">
          通報理由
        </label>
        <select
          id="report-reason"
          name="reason"
          required
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          className="mt-1 block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm text-ink-strong"
        >
          {REPORT_REASONS.map((r) => (
            <option key={r.value} value={r.value}>
              {r.label}
            </option>
          ))}
        </select>
      </div>

      <div>
        <label htmlFor="report-detail" className="block text-sm font-medium text-ink-strong">
          詳細（任意）
        </label>
        <textarea
          id="report-detail"
          name="detail"
          rows={5}
          maxLength={MAX_DETAIL_LEN}
          value={detail}
          onChange={(e) => setDetail(e.target.value)}
          className="mt-1 block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm text-ink-strong"
          placeholder="状況を簡潔にご記入ください"
        />
        <p className="mt-1 text-xs text-ink-soft">
          個人情報・URL・他者の連絡先など、通報内容と関係のない情報は書かないでください。
        </p>
      </div>

      <div>
        <label htmlFor="report-contact" className="block text-sm font-medium text-ink-strong">
          連絡先（任意）
        </label>
        <input
          id="report-contact"
          name="reporter_contact"
          type="text"
          maxLength={MAX_CONTACT_LEN}
          value={contact}
          onChange={(e) => setContact(e.target.value)}
          className="mt-1 block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm text-ink-strong"
          placeholder="メールアドレス / X ID 等（任意）"
        />
        <p className="mt-1 text-xs text-ink-soft">
          連絡先は通報対応のためにのみ使用します。記入がない場合、結果の通知は行いません。
        </p>
      </div>

      <div className="rounded-md border border-divider bg-surface-soft p-3">
        <p className="mb-2 text-xs text-ink-medium">送信前の bot 検証が必要です</p>
        <TurnstileWidget
          sitekey={turnstileSiteKey}
          action="report-submit"
          onVerify={handleTurnstileVerify}
          onError={handleTurnstileError}
          onExpired={handleTurnstileExpired}
          onTimeout={handleTurnstileTimeout}
        />
      </div>

      {formState === "error" && errorMessage !== "" && (
        <p
          role="alert"
          data-testid="report-error"
          className="rounded-md border border-status-error bg-status-error-soft px-3 py-2 text-sm text-status-error"
        >
          {errorMessage}
        </p>
      )}

      <button
        type="submit"
        disabled={!canSubmit}
        data-testid="report-submit-button"
        aria-disabled={!canSubmit}
        className="inline-flex w-full items-center justify-center rounded-md bg-accent-violet px-4 py-2 text-sm font-medium text-on-accent disabled:cursor-not-allowed disabled:opacity-60 sm:w-auto"
      >
        {formState === "submitting" ? "送信中…" : "通報を送信"}
      </button>
    </form>
  );
}

function mapErrorMessage(e: unknown): string {
  if (!isSubmitReportError(e)) {
    return "通報の送信に失敗しました。時間をおいて再度お試しください。";
  }
  const err = e as SubmitReportError;
  switch (err.kind) {
    case "invalid_payload":
      return "入力内容に不備があります。理由・詳細・連絡先をご確認ください。";
    case "turnstile_failed":
      return "bot 検証に失敗しました。再度ページを読み込み直して、もう一度お試しください。";
    case "not_found":
      return "通報対象のフォトブックが見つかりませんでした。";
    case "rate_limited":
      return `短時間に通報を送信しすぎました。${formatRetryAfterDisplay(err.retryAfterSeconds)}時間をおいて再度お試しください。`;
    case "server_error":
    case "network":
      return "通信エラーが発生しました。時間をおいて再度お試しください。";
  }
}
