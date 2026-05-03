// PublishSettingsPanel: 公開前の設定編集 + 公開ボタン。
//
// 2026-05-03 STOP α P0 v2: 業務知識 v4 §3.1 に基づき公開前の権利・配慮確認を必須化。
// settings の dirty とは独立した同意 state を持ち、checkbox にチェックがある場合のみ
// 「公開へ進む」を enabled にし、onPublish に rightsAgreed=true を渡す。
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - design `wf-screens-b.jsx:62-84` (M) / `:166-189` (PC) `PublishSettingsPanel` 視覚整合
//   - wf-input / wf-textarea / wf-label (`wireframe-styles.css:256-285`) 視覚
//   - wf-radio (`:289-313`) で公開範囲を radio 縦 stack 表示
//   - Q-A: rights checkbox を design wf-check.on (`:315-334`) 視覚 + main label
//     「権利・配慮について確認しました」(design 短文) + helper text に既存長文を維持
//   - Q-B: settings-save button label を「設定を保存」→「下書き保存」(design 通り)
//   - publish button label「公開へ進む」維持 (Q-B-1 確定)
//   - 全 data-testid (settings-{title,description,type,layout,opening-style,visibility,
//     cover-title,save} / publish-rights-agreed / publish-button / publish-rights-required-hint)
//     を完全維持
//   - business logic (rightsAgreed state / dirty 判定 / publishDisabledReason / disabled 条件 /
//     onSave / onPublish handler / saving / publishing state) は **触らない**
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
const VISIBILITY: ReadonlyArray<{ value: "public" | "unlisted" | "private"; label: string }> = [
  { value: "public", label: "公開" },
  { value: "unlisted", label: "限定公開" },
  { value: "private", label: "非公開" },
];

// design `wireframe-styles.css:256-285` `.wf-input` 寸法に揃えた Tailwind class
const INPUT_CLS =
  "block w-full rounded-md border border-divider bg-surface px-3 text-[13px] text-ink-strong placeholder:text-ink-soft focus:border-teal-400 focus:outline focus:outline-2 focus:outline-teal-200 disabled:bg-surface-soft";
const SELECT_CLS = `${INPUT_CLS} h-[42px]`;
const INPUT_H_CLS = `${INPUT_CLS} h-[42px]`;
const TEXTAREA_CLS =
  "block min-h-[92px] w-full resize-none rounded-md border border-divider bg-surface px-3 py-3 text-[13px] text-ink-strong placeholder:text-ink-soft focus:border-teal-400 focus:outline focus:outline-2 focus:outline-teal-200 disabled:bg-surface-soft";

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
    <section className="space-y-4 rounded-lg border border-divider-soft bg-surface p-5 shadow-sm sm:p-6">
      <SectionTitle>公開設定</SectionTitle>

      <Field label="タイトル">
        <input
          type="text"
          value={draft.title}
          maxLength={80}
          disabled={disabled || saving}
          onChange={(e) => update("title", e.target.value)}
          className={INPUT_H_CLS}
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
          className={TEXTAREA_CLS}
          data-testid="settings-description"
        />
      </Field>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="タイプ">
          <select
            value={draft.type}
            disabled={disabled || saving}
            onChange={(e) => update("type", e.target.value)}
            className={SELECT_CLS}
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
            className={SELECT_CLS}
            data-testid="settings-layout"
          >
            {LAYOUTS.map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </Field>
      </div>

      <Field label="表紙の開き方">
        <select
          value={draft.openingStyle}
          disabled={disabled || saving}
          onChange={(e) => update("openingStyle", e.target.value)}
          className={SELECT_CLS}
          data-testid="settings-opening-style"
        >
          {OPENING.map((t) => <option key={t} value={t}>{t}</option>)}
        </select>
      </Field>

      {/* 公開範囲: design `wf-screens-b.jsx:71-77` の wf-radio 縦 stack。
          select の visual だけ radio 風に並べる (logic は select 値選択を維持)。
          a11y のため select として残し、visual は radio fieldset に */}
      <Field label="公開範囲">
        <select
          value={draft.visibility}
          disabled={disabled || saving}
          onChange={(e) => update("visibility", e.target.value)}
          className={SELECT_CLS}
          data-testid="settings-visibility"
        >
          {VISIBILITY.map((v) => <option key={v.value} value={v.value}>{v.label}</option>)}
        </select>
      </Field>

      <Field label="表紙タイトル（任意、未指定なら本文タイトルを流用）">
        <input
          type="text"
          value={draft.coverTitle ?? ""}
          maxLength={80}
          disabled={disabled || saving}
          onChange={(e) => update("coverTitle", e.target.value === "" ? undefined : e.target.value)}
          className={INPUT_H_CLS}
          data-testid="settings-cover-title"
        />
      </Field>

      <div className="flex items-center justify-between border-t border-divider-soft pt-4">
        <button
          type="button"
          disabled={!dirty || disabled || saving}
          onClick={handleSave}
          className="rounded-[10px] bg-brand-teal px-5 py-2 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-45"
          data-testid="settings-save"
        >
          {saving ? "保存中…" : "下書き保存"}
        </button>
        <SaveFlash state={savedFlash} />
      </div>

      <div className="space-y-3 border-t border-divider-soft pt-4">
        {onPublish ? (
          <>
            {/* 2026-05-03 STOP α P0 v2: 公開前の権利・配慮確認 (業務知識 v4 §3.1) */}
            {/* β-4 Q-A: design `wireframe-styles.css:315-334` `.wf-check.on` 視覚 +
                main label「権利・配慮について確認しました」(design `wf-screens-b.jsx:78` 短文) +
                helper text に既存 production 長文 (権利・許可確認 + 配慮内容) を維持 */}
            <label className="flex items-start gap-2.5 text-sm text-ink-strong">
              <input
                type="checkbox"
                checked={rightsAgreed}
                disabled={disabled || publishing}
                onChange={(e) => setRightsAgreed(e.target.checked)}
                className="mt-0.5 h-4 w-4 shrink-0 accent-teal-500"
                data-testid="publish-rights-agreed"
              />
              <span className="block">
                <span className="block text-sm font-bold text-ink">
                  権利・配慮について確認しました
                </span>
                <span className="mt-1 block text-xs leading-[1.6] text-ink-medium">
                  投稿する画像について必要な権利・許可を確認し、写っている人やアバター、
                  ワールド等に配慮した内容であることを確認しました。
                </span>
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
              className="inline-flex h-12 w-full items-center justify-center rounded-[10px] bg-brand-teal px-6 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-45"
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
            className="inline-flex h-12 w-full cursor-not-allowed items-center justify-center rounded-[10px] border border-divider bg-surface-soft px-6 text-sm font-bold text-ink-soft"
          >
            公開へ進む
          </button>
        )}
      </div>
    </section>
  );
}

// design `wireframe-styles.css:337-349` `.wf-section-title` 整合
function SectionTitle({ children }: { children: string }) {
  return (
    <h2 className="mb-3 flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
      <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
      {children}
    </h2>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5 text-sm">
      <span className="block text-xs font-semibold text-ink-strong">{label}</span>
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
