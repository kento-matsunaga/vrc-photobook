// PublishSettingsPanel: 公開前の設定編集 + 公開ボタン。
//
// 2026-05-03 STOP α P0 v2: 業務知識 v4 §3.1 に基づき公開前の権利・配慮確認を必須化。
// settings の dirty とは独立した同意 state を持ち、checkbox にチェックがある場合のみ
// 「公開へ進む」を enabled にし、onPublish に rightsAgreed=true を渡す。
"use client";

import { useState } from "react";

import type { EditSettings } from "@/lib/editPhotobook";

type Props = {
  initial: EditSettings;
  disabled?: boolean;
  publishDisabledReason?: string;
  onSave: (next: EditSettings) => Promise<void>;
  /** 2026-05-03 STOP α P0 v2: rightsAgreed は checkbox 値、Frontend が publish 経路で
   *  Backend に送る。false で publish 不可は呼び出し側でも防ぐが、UI でも disable する。 */
  onPublish?: (rightsAgreed: boolean) => Promise<void>;
};

const TYPES = ["event", "daily", "portfolio", "avatar", "world", "memory", "free"] as const;
const LAYOUTS = ["simple", "magazine", "card", "large"] as const;
const OPENING = ["light", "cover_first_view"] as const;
const VISIBILITY = ["public", "unlisted", "private"] as const;

export function PublishSettingsPanel({
  initial,
  disabled,
  publishDisabledReason,
  onSave,
  onPublish,
}: Props) {
  const [draft, setDraft] = useState<EditSettings>(initial);
  const [saving, setSaving] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [savedFlash, setSavedFlash] = useState<"none" | "saved" | "error">("none");
  // 2026-05-03 STOP α P0 v2: 権利・配慮確認同意 (業務知識 v4 §3.1)。
  // settings の dirty とは独立した state。settings 保存とは別に publish request にだけ乗せる。
  const [rightsAgreed, setRightsAgreed] = useState<boolean>(false);
  const dirty = JSON.stringify(draft) !== JSON.stringify(initial);

  const update = <K extends keyof EditSettings>(k: K, v: EditSettings[K]) =>
    setDraft((d) => ({ ...d, [k]: v }));

  const handleSave = async () => {
    if (!dirty || disabled) return;
    setSaving(true);
    setSavedFlash("none");
    try {
      await onSave(draft);
      setSavedFlash("saved");
    } catch {
      setSavedFlash("error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="space-y-4 rounded-lg border border-divider bg-surface p-4 shadow-sm">
      <h2 className="text-h2 text-ink">公開設定</h2>

      <Field label="タイトル">
        <input
          type="text"
          value={draft.title}
          maxLength={80}
          disabled={disabled || saving}
          onChange={(e) => update("title", e.target.value)}
          className="block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm focus:border-brand-teal focus:outline-none disabled:bg-surface-soft"
          data-testid="settings-title"
        />
      </Field>

      <Field label="説明（任意）">
        <textarea
          value={draft.description ?? ""}
          maxLength={500}
          rows={3}
          disabled={disabled || saving}
          onChange={(e) => update("description", e.target.value === "" ? undefined : e.target.value)}
          className="block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm focus:border-brand-teal focus:outline-none disabled:bg-surface-soft"
          data-testid="settings-description"
        />
      </Field>

      <Field label="タイプ">
        <select
          value={draft.type}
          disabled={disabled || saving}
          onChange={(e) => update("type", e.target.value)}
          className="block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm focus:border-brand-teal focus:outline-none disabled:bg-surface-soft"
          data-testid="settings-type"
        >
          {TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
        </select>
      </Field>

      <Field label="レイアウト">
        <select
          value={draft.layout}
          disabled={disabled || saving}
          onChange={(e) => update("layout", e.target.value)}
          className="block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm focus:border-brand-teal focus:outline-none disabled:bg-surface-soft"
          data-testid="settings-layout"
        >
          {LAYOUTS.map((t) => <option key={t} value={t}>{t}</option>)}
        </select>
      </Field>

      <Field label="表紙の開き方">
        <select
          value={draft.openingStyle}
          disabled={disabled || saving}
          onChange={(e) => update("openingStyle", e.target.value)}
          className="block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm focus:border-brand-teal focus:outline-none disabled:bg-surface-soft"
          data-testid="settings-opening-style"
        >
          {OPENING.map((t) => <option key={t} value={t}>{t}</option>)}
        </select>
      </Field>

      <Field label="公開範囲">
        <select
          value={draft.visibility}
          disabled={disabled || saving}
          onChange={(e) => update("visibility", e.target.value)}
          className="block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm focus:border-brand-teal focus:outline-none disabled:bg-surface-soft"
          data-testid="settings-visibility"
        >
          {VISIBILITY.map((t) => <option key={t} value={t}>{t}</option>)}
        </select>
      </Field>

      <Field label="表紙タイトル（任意、未指定なら本文タイトルを流用）">
        <input
          type="text"
          value={draft.coverTitle ?? ""}
          maxLength={80}
          disabled={disabled || saving}
          onChange={(e) => update("coverTitle", e.target.value === "" ? undefined : e.target.value)}
          className="block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm focus:border-brand-teal focus:outline-none disabled:bg-surface-soft"
          data-testid="settings-cover-title"
        />
      </Field>

      <div className="flex items-center justify-between border-t border-divider pt-4">
        <button
          type="button"
          disabled={!dirty || disabled || saving}
          onClick={handleSave}
          className="rounded-md bg-brand-teal px-4 py-2 text-sm font-medium text-white hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-50"
          data-testid="settings-save"
        >
          {saving ? "保存中…" : "設定を保存"}
        </button>
        <SaveFlash state={savedFlash} />
      </div>

      <div className="space-y-3 border-t border-divider pt-4">
        {onPublish ? (
          <>
            {/* 2026-05-03 STOP α P0 v2: 公開前の権利・配慮確認 (業務知識 v4 §3.1) */}
            <label className="flex items-start gap-2 text-sm text-ink-strong">
              <input
                type="checkbox"
                checked={rightsAgreed}
                disabled={disabled || publishing}
                onChange={(e) => setRightsAgreed(e.target.checked)}
                className="mt-0.5 h-4 w-4 shrink-0"
                data-testid="publish-rights-agreed"
              />
              <span className="text-xs leading-relaxed">
                投稿する画像について必要な権利・許可を確認し、写っている人やアバター、
                ワールド等に配慮した内容であることを確認しました。
              </span>
            </label>
            <button
              type="button"
              disabled={
                Boolean(publishDisabledReason) ||
                disabled ||
                publishing ||
                dirty ||
                !rightsAgreed
              }
              onClick={async () => {
                if (!onPublish) return;
                if (!rightsAgreed) return;
                setPublishing(true);
                try {
                  await onPublish(rightsAgreed);
                } finally {
                  setPublishing(false);
                }
              }}
              className="w-full rounded-md bg-brand-teal px-4 py-3 text-sm font-medium text-white hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-50"
              data-testid="publish-button"
            >
              {publishing ? "公開中…" : "公開へ進む"}
            </button>
            {publishDisabledReason && (
              <p className="text-xs text-ink-medium">{publishDisabledReason}</p>
            )}
            {dirty && !publishDisabledReason && (
              <p className="text-xs text-ink-medium">変更を保存してから公開してください。</p>
            )}
            {!rightsAgreed && !publishDisabledReason && !dirty && (
              <p className="text-xs text-ink-medium" data-testid="publish-rights-required-hint">
                公開前に権利・配慮確認への同意が必要です。
              </p>
            )}
          </>
        ) : (
          <button
            type="button"
            disabled
            aria-disabled="true"
            className="w-full cursor-not-allowed rounded-md border border-divider bg-surface-soft px-4 py-3 text-sm font-medium text-ink-soft"
          >
            公開へ進む
          </button>
        )}
      </div>
    </section>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1 text-sm">
      <span className="text-ink-medium">{label}</span>
      {children}
    </label>
  );
}

function SaveFlash({ state }: { state: "none" | "saved" | "error" }) {
  if (state === "saved")
    return <span className="text-xs text-brand-teal" aria-live="polite">保存しました</span>;
  if (state === "error")
    return <span className="text-xs text-status-error" aria-live="polite">保存失敗（再試行してください）</span>;
  return <span className="text-xs text-ink-soft">&nbsp;</span>;
}
