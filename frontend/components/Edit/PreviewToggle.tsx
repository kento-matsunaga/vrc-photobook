// PreviewToggle: edit / preview mode 切替 button。
//
// 設計参照:
//   - docs/plan/m2-edit-page-split-and-preview-plan.md §6.8
//
// 振る舞い:
//   - mode === "edit"   → 「📖 プレビュー」 (押下で preview に切替)
//   - mode === "preview"→ 「✏️ 編集に戻る」 (押下で edit に戻る)
//   - aria-pressed で現在 mode を伝える
//   - mutation 進行中などで disabled のときは押せない
"use client";

export type ViewMode = "edit" | "preview";

type Props = {
  mode: ViewMode;
  disabled?: boolean;
  onToggle: () => void;
};

export function PreviewToggle({ mode, disabled, onToggle }: Props) {
  const isPreview = mode === "preview";
  const label = isPreview ? "✏️ 編集に戻る" : "📖 プレビュー";
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onToggle}
      aria-pressed={isPreview}
      aria-label={isPreview ? "編集に戻る" : "プレビューを開く"}
      className="inline-flex h-9 items-center rounded-md border border-divider bg-surface px-3 text-xs font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-45"
      data-testid="preview-toggle"
    >
      {label}
    </button>
  );
}
