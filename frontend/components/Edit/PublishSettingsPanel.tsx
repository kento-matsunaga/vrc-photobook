// PublishSettingsPanel: 公開前の設定編集 + 公開ボタン placeholder（PR28 で本実装）。
"use client";

import { useState } from "react";

import type { EditSettings } from "@/lib/editPhotobook";

type Props = {
  initial: EditSettings;
  disabled?: boolean;
  onSave: (next: EditSettings) => Promise<void>;
};

const TYPES = ["event", "daily", "portfolio", "avatar", "world", "memory", "free"] as const;
const LAYOUTS = ["simple", "magazine", "card", "large"] as const;
const OPENING = ["light", "cover_first_view"] as const;
const VISIBILITY = ["public", "unlisted", "private"] as const;

export function PublishSettingsPanel({ initial, disabled, onSave }: Props) {
  const [draft, setDraft] = useState<EditSettings>(initial);
  const [saving, setSaving] = useState(false);
  const [savedFlash, setSavedFlash] = useState<"none" | "saved" | "error">("none");
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

      <div className="border-t border-divider pt-4">
        <button
          type="button"
          disabled
          aria-disabled="true"
          className="w-full cursor-not-allowed rounded-md border border-divider bg-surface-soft px-4 py-3 text-sm font-medium text-ink-soft"
          data-testid="publish-button-placeholder"
        >
          公開へ進む（PR28 で実装予定）
        </button>
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
