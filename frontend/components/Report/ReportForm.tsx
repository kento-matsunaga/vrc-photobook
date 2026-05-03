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
//
// m2-design-refresh STOP β-5 (本 commit、visual のみ):
//   - design `wf-screens-c.jsx:79-129` (M) / `:130-176` (PC narrow) `WFReport` 視覚整合
//   - reason を design wf-radio (`wireframe-styles.css:289-313`) に整合
//     (select → radio button list、Mobile 縦 / PC wf-grid-2)
//   - detail を design wf-textarea + wf-counter
//   - contact を design wf-input + wf-counter
//   - Turnstile widget 周辺 wf-box 視覚維持
//   - thanks view を design ✓ 円 icon + 「通報を受け付けました」(`:120-124`) に整合
//   - submit button を wf-btn primary lg full (Mobile) / lg 右寄せ (PC)、teal tone
//   - 全 data-testid (report-form / report-error / report-submit-button / report-thanks-view) **完全維持**
//   - REPORT_REASONS option value は **完全維持** (test 互換: minor_safety_concern /
//     harassment_or_doxxing / unauthorized_repost / subject_removal_request /
//     sensitive_flag_missing / other)
//   - submit logic / Turnstile L0-L4 多層ガード / validation / POST / rate limit /
//     success/error state / mapErrorMessage は **触らない**
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

// design `wireframe-styles.css:310-313` `.wf-radio.active .dot` の radial-gradient inline style
// (β-3-1 Create radio と同 pattern)
const ACTIVE_DOT_BG =
  "radial-gradient(circle, #15B2A8 42%, transparent 44%)";

// wf-input / wf-textarea 共通 class (β-3 / β-4 と同 pattern)
const INPUT_CLS =
  "block w-full rounded-md border border-divider bg-surface px-3 text-[13px] text-ink-strong placeholder:text-ink-soft focus:border-teal-400 focus:outline focus:outline-2 focus:outline-teal-200";
const INPUT_H_CLS = `${INPUT_CLS} h-[42px]`;
const TEXTAREA_CLS =
  "block min-h-[120px] w-full resize-none rounded-md border border-divider bg-surface px-3 py-3 text-[13px] text-ink-strong placeholder:text-ink-soft focus:border-teal-400 focus:outline focus:outline-2 focus:outline-teal-200";

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
    // design `wf-screens-c.jsx:120-124` (M) / `:167-172` (PC) thanks view (✓ 円 icon + center stack)
    return (
      <section
        role="status"
        aria-live="polite"
        data-testid="report-thanks-view"
        className="rounded-lg border border-divider-soft bg-surface p-7 text-center shadow-sm"
      >
        <div className="mx-auto grid h-12 w-12 place-items-center rounded-full border-2 border-ink font-bold text-ink">
          ✓
        </div>
        <h2 className="mt-3 text-h2 font-extrabold text-ink">通報を受け付けました</h2>
        <p className="mt-3 text-sm leading-[1.6] text-ink-medium">
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
      {/* design `wf-screens-c.jsx:88-97` (M wf-stack) / `:140-148` (PC wf-grid-2) Reason select は wf-radio に置換 */}
      <fieldset className="rounded-lg border border-divider-soft bg-surface p-5 shadow-sm sm:p-6">
        <legend className="mb-3 flex items-center gap-2 px-1 text-xs font-bold tracking-[0.04em] text-ink-strong">
          <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
          通報理由
        </legend>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 sm:gap-3">
          {REPORT_REASONS.map((r) => {
            const active = reason === r.value;
            return (
              <label
                key={r.value}
                className={`flex cursor-pointer items-center gap-2.5 rounded-[10px] bg-surface p-3 transition-colors hover:border-teal-200 ${
                  active
                    ? "border-[1.5px] border-teal-500 bg-teal-50"
                    : "border border-divider"
                }`}
              >
                <input
                  type="radio"
                  name="reason"
                  value={r.value}
                  checked={active}
                  onChange={(e) => setReason(e.target.value)}
                  className="sr-only"
                  required
                />
                <span
                  aria-hidden="true"
                  className={`inline-block h-4 w-4 shrink-0 rounded-full border-[1.5px] ${
                    active ? "border-teal-500" : "border-ink-soft"
                  }`}
                  style={active ? { backgroundImage: ACTIVE_DOT_BG } : undefined}
                />
                <span className="flex-1 text-sm text-ink-strong">{r.label}</span>
              </label>
            );
          })}
        </div>
      </fieldset>

      <div className="rounded-lg border border-divider-soft bg-surface p-5 shadow-sm sm:p-6">
        <label
          htmlFor="report-detail"
          className="mb-1.5 block text-xs font-semibold text-ink-strong"
        >
          詳細（任意）
        </label>
        <textarea
          id="report-detail"
          name="detail"
          rows={5}
          maxLength={MAX_DETAIL_LEN}
          value={detail}
          onChange={(e) => setDetail(e.target.value)}
          className={TEXTAREA_CLS}
          placeholder="状況を簡潔にご記入ください"
        />
        <p className="mt-1 text-right font-num text-[10.5px] text-ink-soft">
          {detail.length} / {MAX_DETAIL_LEN}
        </p>
        <p className="mt-2 text-xs leading-[1.5] text-ink-soft">
          個人情報・URL・他者の連絡先など、通報内容と関係のない情報は書かないでください。
        </p>

        <label
          htmlFor="report-contact"
          className="mb-1.5 mt-5 block text-xs font-semibold text-ink-strong"
        >
          連絡先（任意）
        </label>
        <input
          id="report-contact"
          name="reporter_contact"
          type="text"
          maxLength={MAX_CONTACT_LEN}
          value={contact}
          onChange={(e) => setContact(e.target.value)}
          className={INPUT_H_CLS}
          placeholder="メールアドレス / X ID 等（任意）"
        />
        <p className="mt-1 text-right font-num text-[10.5px] text-ink-soft">
          {contact.length} / {MAX_CONTACT_LEN}
        </p>
        <p className="mt-2 text-xs leading-[1.5] text-ink-soft">
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

      <div className="flex sm:justify-end">
        <button
          type="submit"
          disabled={!canSubmit}
          data-testid="report-submit-button"
          aria-disabled={!canSubmit}
          className="inline-flex h-12 w-full items-center justify-center rounded-[10px] bg-brand-teal px-6 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-45 sm:w-auto sm:min-w-[200px]"
        >
          {formState === "submitting" ? "送信中…" : "通報を送信"}
        </button>
      </div>
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
