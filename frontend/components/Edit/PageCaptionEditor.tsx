// PageCaptionEditor: page caption の blur 保存 editor。
//
// 設計参照:
//   - docs/plan/m2-edit-page-split-and-preview-plan.md §6.7
//
// 振る舞い:
//   - 1 行 input (page caption は photo caption より短い想定だが、上限は同じ 200 文字)
//   - blur で onSave 呼出 (空文字 / whitespace のみは null として送る = caption クリア)
//   - 200 文字超 / 変更なし は保存しない
//   - 保存中 / 保存済 / 失敗 を SaveStatus として表示
//
// CaptionEditor (photo) と仕様はほぼ同形だが、placeholder と aria-label を page 用に変える。
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
  return [...s].length;
}

export function PageCaptionEditor({ initialValue, disabled, onSave }: Props) {
  const [value, setValue] = useState(initialValue);
  const [committed, setCommitted] = useState(initialValue);
  const [state, setState] = useState<SaveState>("idle");
  const overLimit = runeCount(value) > MAX_RUNES;

  const handleBlur = async () => {
    if (disabled) return;
    if (value === committed) return;
    if (overLimit) return;
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
      <input
        type="text"
        value={value}
        disabled={disabled}
        onChange={(e) => setValue(e.target.value)}
        onBlur={handleBlur}
        placeholder="ページのタイトル(任意、最大 200 文字)"
        className="block w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm text-ink-strong placeholder:text-ink-soft focus:border-teal-400 focus:outline focus:outline-2 focus:outline-teal-200 disabled:bg-surface-soft"
        aria-label="page caption"
        data-testid="page-caption-editor"
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
