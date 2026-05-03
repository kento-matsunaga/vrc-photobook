// /create — フォトブック作成入口（Client Component）。
//
// 機能 (β-3 で **不変**):
//   - type 選択（7 種、既定 memory）
//   - title / creator_display_name 任意入力
//   - Turnstile widget（action="photobook-create"）+ L0-L4 多層ガード
//   - submit → POST /api/photobooks → response.draft_edit_url_path に window.location.replace
//     （raw token を browser history / localStorage に残さない）
//
// m2-design-refresh STOP β-3 (本 commit、visual のみ):
//   - type radio を design `.wf-radio` 風 (active border-teal-500 + bg-teal-50 + dot radial-gradient inline style)
//   - title / creator input を `.wf-input` 風 (h-[42px] + focus outline-2 outline-teal-200 + border-teal-400)
//   - 文字数 counter を `.wf-counter` (font-num text-[10.5px] text-ink-soft text-right)
//   - visibility note を `.wf-note` 風 (border-l teal-300 + bg teal-50 + i icon)
//   - submit button を design `.wf-btn primary lg` 風 (rounded-[10px] + shadow-sm)
//   - 既存 type description (production truth) は維持
//   - 既存 data-testid (create-form / create-type-{key} / create-error / create-submit-button) は維持
//   - business logic / Turnstile L0-L4 / POST / window.location.replace は触らない
//
// 設計参照:
//   - design/source/project/wf-screens-a.jsx:206-308 (Create M / PC)
//   - design/source/project/wireframe-styles.css:256-285 (.wf-input / .wf-label / .wf-counter)
//   - design/source/project/wireframe-styles.css:289-313 (.wf-radio / .dot)
//   - design/source/project/wireframe-styles.css:398-425 (.wf-note)
//   - design/source/project/wireframe-styles.css:228-251 (.wf-btn primary)
//   - docs/plan/m2-design-refresh-stop-beta-3-plan.md §1
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

// design `wireframe-styles.css:310-313` `.wf-radio.active .dot` の radial-gradient を inline style で表現
// (Tailwind arbitrary では radial-gradient の冗長さが大きいため、β-2a `THUMB_GRADIENTS` と同方針)
const ACTIVE_DOT_BG =
  "radial-gradient(circle, #15B2A8 42%, transparent 44%)";

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
      {/* type 選択 (`wf-screens-a.jsx:215-224` M 縦 / `:264-277` PC wf-grid-3) */}
      <fieldset>
        <legend className="text-xs font-bold tracking-[0.04em] text-ink-strong sm:text-[12px]">
          タイプ
        </legend>
        <p className="mt-1 text-xs text-ink-soft">あとから変更できます。</p>
        <div className="mt-3 grid gap-2 sm:grid-cols-3 sm:gap-3">
          {TYPE_OPTIONS.map((opt) => {
            const active = type === opt.key;
            return (
              <label
                key={opt.key}
                className={`group relative flex cursor-pointer items-start gap-2.5 rounded-[10px] bg-surface p-3.5 text-left transition-colors hover:border-teal-200 ${
                  active
                    ? "border-[1.5px] border-teal-500 bg-teal-50"
                    : "border border-divider"
                }`}
                data-testid={`create-type-${opt.key}`}
              >
                <input
                  type="radio"
                  name="type"
                  value={opt.key}
                  checked={active}
                  onChange={() => setType(opt.key)}
                  className="sr-only"
                />
                {/* design `wireframe-styles.css:305-313` `.wf-radio .dot` (16x16 round) +
                    active dot radial-gradient (inline style 採用、Q-3-8 確定) */}
                <span
                  aria-hidden="true"
                  className={`mt-0.5 inline-block h-4 w-4 shrink-0 rounded-full border-[1.5px] ${
                    active ? "border-teal-500" : "border-ink-soft"
                  }`}
                  style={active ? { backgroundImage: ACTIVE_DOT_BG } : undefined}
                />
                <span className="block flex-1">
                  <span className="block text-sm font-bold text-ink">{opt.label}</span>
                  <span className="mt-1 block text-xs leading-[1.5] text-ink-medium">
                    {opt.description}
                  </span>
                </span>
              </label>
            );
          })}
        </div>
      </fieldset>

      {/* title / creator (PC は wf-grid-2 並列、Mobile 縦) */}
      <div className="grid gap-4 sm:grid-cols-2 sm:gap-5">
        {/* title (`:282-285` wf-label + wf-input + wf-counter) */}
        <div>
          <label
            htmlFor="create-title"
            className="block text-xs font-semibold text-ink-strong"
          >
            タイトル（任意・最大 100 文字）
          </label>
          <input
            id="create-title"
            name="title"
            type="text"
            maxLength={100}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="mt-1.5 block h-[42px] w-full rounded-md border border-divider bg-surface px-3 text-[13px] text-ink-strong placeholder:text-ink-soft focus:border-teal-400 focus:outline focus:outline-2 focus:outline-teal-200"
            placeholder="後で入力できます"
          />
          <p className="mt-1 text-right font-num text-[10.5px] text-ink-soft">
            {title.length} / 100
          </p>
        </div>

        {/* creator_display_name */}
        <div>
          <label
            htmlFor="create-creator"
            className="block text-xs font-semibold text-ink-strong"
          >
            作成者の表示名（任意・最大 50 文字）
          </label>
          <input
            id="create-creator"
            name="creator_display_name"
            type="text"
            maxLength={50}
            value={creator}
            onChange={(e) => setCreator(e.target.value)}
            className="mt-1.5 block h-[42px] w-full rounded-md border border-divider bg-surface px-3 text-[13px] text-ink-strong placeholder:text-ink-soft focus:border-teal-400 focus:outline focus:outline-2 focus:outline-teal-200"
            placeholder="後で入力できます"
          />
          <p className="mt-1 text-right font-num text-[10.5px] text-ink-soft">
            {creator.length} / 50
          </p>
        </div>
      </div>

      {/* visibility note (design `wireframe-styles.css:398-425` `.wf-note` 風) */}
      <div className="flex items-start gap-2.5 rounded-lg border-l-[3px] border-teal-300 bg-teal-50 p-3.5">
        <span
          aria-hidden="true"
          className="grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full bg-teal-500 font-serif text-xs font-bold italic leading-none text-white"
        >
          i
        </span>
        <div className="text-xs leading-[1.6] text-ink-strong">
          <p>
            公開範囲は <strong>限定公開</strong>（URL を知っている人のみ閲覧可能）が既定です。
            公開操作は次のステップ以降で行います。
          </p>
          <p className="mt-1 text-ink-soft">
            公開前の権利・配慮確認は publish 時に行います。本ページでは触れません。
          </p>
        </div>
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

      {/* submit (PC right-aligned per design `wf-screens-a.jsx:299-301`、Mobile full width) */}
      <div className="flex sm:justify-end">
        <button
          type="submit"
          disabled={!canSubmit}
          data-testid="create-submit-button"
          className="inline-flex h-12 w-full items-center justify-center rounded-[10px] bg-brand-teal px-6 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-45 sm:w-auto sm:min-w-[200px]"
        >
          {formState === "submitting" ? "作成中…" : "編集を始める"}
        </button>
      </div>
    </form>
  );
}
