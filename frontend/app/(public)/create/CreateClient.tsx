// /create — フォトブック作成入口（Client Component）。
//
// 機能:
//   - type 選択（7 種、既定 memory）
//   - title / creator_display_name 任意入力
//   - Turnstile widget（action="photobook-create"）+ L0-L4 多層ガード
//   - submit → POST /api/photobooks → response.draft_edit_url_path に window.location.replace
//     （raw token を browser history / localStorage に残さない）
//
// 設計参照:
//   - docs/plan/m2-create-entry-plan.md §8
//   - .agents/rules/turnstile-defensive-guard.md L0-L4

"use client";

import { useCallback, useState } from "react";
import type { FormEvent } from "react";

import { TurnstileWidget } from "@/components/TurnstileWidget";
import {
  PHOTOBOOK_TYPES,
  createPhotobook,
  isCreatePhotobookError,
  type PhotobookType,
} from "@/lib/createPhotobook";

type Props = {
  turnstileSiteKey: string;
};

type TypeOption = {
  key: PhotobookType;
  label: string;
  description: string;
};

const TYPE_OPTIONS: ReadonlyArray<TypeOption> = [
  { key: "memory", label: "思い出", description: "VRC で過ごした特別な時間を 1 冊に。" },
  { key: "event", label: "イベント", description: "オフ会・ライブ・撮影会などの記録。" },
  { key: "daily", label: "日々（おはツイ等）", description: "日常の交流や日々の積み重ねを残す。" },
  { key: "portfolio", label: "作品集", description: "ワールドや写真作品を美しくまとめる。" },
  { key: "avatar", label: "アバター紹介", description: "アバターの魅力をプロフィールブックに。" },
  { key: "world", label: "ワールド紹介", description: "気に入ったワールドのガイドとして。" },
  { key: "free", label: "自由作成", description: "用途を決めずに、自由にページを作る。" },
];

type FormState = "idle" | "submitting" | "success" | "error";

const ERROR_MESSAGES: Record<string, string> = {
  invalid_payload: "入力内容を確認してください。",
  turnstile_failed: "認証に失敗しました。しばらくしてから再度お試しください。",
  turnstile_unavailable: "認証サービスが一時的に利用できません。少し時間をおいて再度お試しください。",
  server_error: "一時的なエラーが発生しました。少し時間をおいて再度お試しください。",
  network: "通信エラーが発生しました。接続を確認して再度お試しください。",
};

function mapErrorMessage(kind: string): string {
  return ERROR_MESSAGES[kind] ?? ERROR_MESSAGES.server_error;
}

export function CreateClient({ turnstileSiteKey }: Props) {
  const [type, setType] = useState<PhotobookType>("memory");
  const [title, setTitle] = useState("");
  const [creator, setCreator] = useState("");
  const [turnstileToken, setTurnstileToken] = useState("");
  const [formState, setFormState] = useState<FormState>("idle");
  const [errorMessage, setErrorMessage] = useState("");

  // L0-L4 Turnstile 多層ガード: callback は useCallback で安定化
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

  // L1: 送信ボタン disable 判定（trim 後 non-empty 必須）
  const canSubmit =
    formState !== "submitting" &&
    typeof turnstileToken === "string" &&
    turnstileToken.trim() !== "" &&
    PHOTOBOOK_TYPES.includes(type);

  const onSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    // L2: onSubmit 冒頭の再評価 early return
    if (typeof turnstileToken !== "string" || turnstileToken.trim() === "") {
      setFormState("error");
      setErrorMessage(mapErrorMessage("turnstile_failed"));
      return;
    }

    setFormState("submitting");
    setErrorMessage("");
    try {
      const out = await createPhotobook({
        type,
        title,
        creatorDisplayName: creator,
        turnstileToken,
      });
      setFormState("success");
      // raw token を browser history に残さないため replace を使う
      window.location.replace(out.draftEditUrlPath);
    } catch (e) {
      const kind = isCreatePhotobookError(e) ? e.kind : "server_error";
      setFormState("error");
      setErrorMessage(mapErrorMessage(kind));
    }
  };

  return (
    <form
      onSubmit={onSubmit}
      data-testid="create-form"
      className="mt-6 space-y-6"
    >
      {/* type 選択 */}
      <fieldset>
        <legend className="text-h2 text-ink">タイプ</legend>
        <p className="mt-1 text-xs text-ink-soft">あとから変更できます。</p>
        <div className="mt-3 grid gap-2 sm:grid-cols-2">
          {TYPE_OPTIONS.map((opt) => {
            const active = type === opt.key;
            return (
              <label
                key={opt.key}
                className={`flex cursor-pointer items-start gap-3 rounded-lg border p-4 transition-colors ${
                  active
                    ? "border-brand-teal bg-brand-teal-soft"
                    : "border-divider bg-surface hover:bg-surface-soft"
                }`}
                data-testid={`create-type-${opt.key}`}
              >
                <input
                  type="radio"
                  name="type"
                  value={opt.key}
                  checked={active}
                  onChange={() => setType(opt.key)}
                  className="mt-1 h-4 w-4 accent-brand-teal"
                />
                <span className="block">
                  <span className="text-sm font-bold text-ink">{opt.label}</span>
                  <span className="mt-1 block text-xs text-ink-medium">
                    {opt.description}
                  </span>
                </span>
              </label>
            );
          })}
        </div>
      </fieldset>

      {/* title 任意 */}
      <div>
        <label
          htmlFor="create-title"
          className="block text-sm font-medium text-ink-strong"
        >
          タイトル（任意）
        </label>
        <input
          id="create-title"
          name="title"
          type="text"
          maxLength={100}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          className="mt-1 block w-full rounded-md border border-divider bg-surface px-3 py-2 text-base text-ink-strong"
          placeholder="後で入力できます"
        />
      </div>

      {/* creator_display_name 任意 */}
      <div>
        <label
          htmlFor="create-creator"
          className="block text-sm font-medium text-ink-strong"
        >
          作成者の表示名（任意）
        </label>
        <input
          id="create-creator"
          name="creator_display_name"
          type="text"
          maxLength={50}
          value={creator}
          onChange={(e) => setCreator(e.target.value)}
          className="mt-1 block w-full rounded-md border border-divider bg-surface px-3 py-2 text-base text-ink-strong"
          placeholder="後で入力できます"
        />
      </div>

      {/* visibility 説明 */}
      <div className="rounded-lg border border-divider bg-surface-soft p-4">
        <p className="text-sm text-ink-strong">
          公開範囲は <strong>限定公開</strong>（URL を知っている人のみ閲覧可能）が既定です。
          公開操作は次のステップ以降で行います。
        </p>
        <p className="mt-1 text-xs text-ink-soft">
          公開前の権利・配慮確認は publish 時に行います。本ページでは触れません。
        </p>
      </div>

      {/* Turnstile widget */}
      <div className="rounded-md border border-divider bg-surface-soft p-3">
        <p className="mb-2 text-xs text-ink-medium">送信前の bot 検証が必要です</p>
        <TurnstileWidget
          sitekey={turnstileSiteKey}
          action="photobook-create"
          onVerify={handleTurnstileVerify}
          onError={handleTurnstileError}
          onExpired={handleTurnstileExpired}
          onTimeout={handleTurnstileTimeout}
        />
      </div>

      {/* error */}
      {formState === "error" && errorMessage !== "" && (
        <p
          role="alert"
          data-testid="create-error"
          className="rounded-md border border-status-error bg-status-error-soft px-3 py-2 text-sm text-status-error"
        >
          {errorMessage}
        </p>
      )}

      {/* submit */}
      <button
        type="submit"
        disabled={!canSubmit}
        data-testid="create-submit-button"
        className="inline-flex h-12 w-full items-center justify-center rounded bg-brand-teal px-6 text-sm font-bold text-white hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-60 sm:w-auto"
      >
        {formState === "submitting" ? "作成中…" : "編集を始める"}
      </button>
    </form>
  );
}
