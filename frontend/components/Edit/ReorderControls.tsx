// ReorderControls: 上下ボタン式 reorder（PR26 §7 案 C 採用）。
//
// drag & drop は PR41+ で評価。MVP は a11y / Safari で安定する上下ボタン。
"use client";

type Props = {
  disabled?: boolean;
  isFirst: boolean;
  isLast: boolean;
  onMoveUp: () => Promise<void>;
  onMoveDown: () => Promise<void>;
  onMoveTop: () => Promise<void>;
  onMoveBottom: () => Promise<void>;
};

export function ReorderControls({
  disabled,
  isFirst,
  isLast,
  onMoveUp,
  onMoveDown,
  onMoveTop,
  onMoveBottom,
}: Props) {
  const cls =
    "rounded-sm border border-divider px-2 py-1 text-xs text-ink-medium hover:bg-surface-soft disabled:cursor-not-allowed disabled:opacity-50";
  return (
    <div className="flex items-center gap-1" role="group" aria-label="並び替え">
      <button
        type="button"
        disabled={disabled || isFirst}
        onClick={onMoveTop}
        className={cls}
        aria-label="先頭へ"
        data-testid="reorder-top"
      >
        ⤒
      </button>
      <button
        type="button"
        disabled={disabled || isFirst}
        onClick={onMoveUp}
        className={cls}
        aria-label="一つ上へ"
        data-testid="reorder-up"
      >
        ↑
      </button>
      <button
        type="button"
        disabled={disabled || isLast}
        onClick={onMoveDown}
        className={cls}
        aria-label="一つ下へ"
        data-testid="reorder-down"
      >
        ↓
      </button>
      <button
        type="button"
        disabled={disabled || isLast}
        onClick={onMoveBottom}
        className={cls}
        aria-label="末尾へ"
        data-testid="reorder-bottom"
      >
        ⤓
      </button>
    </div>
  );
}
