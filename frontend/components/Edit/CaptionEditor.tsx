// CaptionEditor: blur 保存 + 保存ステータス。
//
// 設計: PR26 計画書 §8（blur 保存採用）
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - design `wireframe-styles.css:267-278` `.wf-textarea` 視覚整合
//     (rounded-md + border-divider + focus outline-teal-200 + border-teal-400)
//   - blur 保存 logic / runeCount / SaveStatus は **触らない**
"use client";

import { useState } from "react";

type SaveState = "idle" | "saving" | "saved" | "error";

type Props = {
  initialValue: string;
  disabled?: boolean;
  onSave: (value: string | null) => Promise<void>;
};

const MAX_RUNES = 200;

function runeCount(s: string): number {
  // [...s] で code point 単位（Unicode rune に近い）に分解
  return [...s].length;
}

export function CaptionEditor({ initialValue, disabled, onSave }: Props) {
  const [value, setValue] = useState(initialValue);
  const [committed, setCommitted] = useState(initialValue);
  const [state, setState] = useState<SaveState>("idle");
  const overLimit = runeCount(value) > MAX_RUNES;

  const handleBlur = async () => {
    if (disabled) return;
    if (value === committed) return;
    if (overLimit) return; // 不正値は保存しない
    setState("saving");
    try {
      const trimmed = value.trim();
      await onSave(trimmed === "" ? null : trimmed);
      setCommitted(value);
      setState("saved");
    } catch {
      setState("error");
    }
  };

  return (
    <div className="space-y-1">
      <textarea
        value={value}
        disabled={disabled}
        onChange={(e) => setValue(e.target.value)}
        onBlur={handleBlur}
        rows={2}
        placeholder="キャプション（任意、最大 200 文字）"
        className="block w-full resize-none rounded-md border border-divider bg-surface px-3 py-2 text-[13px] text-ink-strong placeholder:text-ink-soft focus:border-teal-400 focus:outline focus:outline-2 focus:outline-teal-200 disabled:bg-surface-soft"
        aria-label="photo caption"
        data-testid="caption-editor"
      />
      <div className="flex items-center justify-between text-[10.5px]">
        <span
          className={`font-num text-ink-soft ${overLimit ? "text-status-error" : ""}`}
          aria-live="polite"
        >
          {runeCount(value)} / {MAX_RUNES}
        </span>
        <SaveStatus state={state} />
      </div>
    </div>
  );
}

function SaveStatus({ state }: { state: SaveState }) {
  if (state === "idle") return <span className="text-ink-soft">変更なし</span>;
  if (state === "saving")
    return <span className="text-ink-medium" aria-live="polite">保存中…</span>;
  if (state === "saved")
    return <span className="text-brand-teal" aria-live="polite">保存しました</span>;
  return <span className="text-status-error" aria-live="polite">保存失敗</span>;
}
