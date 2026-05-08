// PageMovePicker: photo を別 page (or 同 page) に移動するための picker UI。
//
// 設計参照:
//   - docs/plan/m2-edit-page-split-and-preview-plan.md §6.5
//
// 振る舞い:
//   - 移動先 page を <select> で選び、position (start / end) を radio 風 button で選んで実行
//   - 同 page 移動 (= 内部 reorder) は MVP では既存 ReorderControls 経由なので、本 picker
//     では disable + tooltip
//   - 中間挿入は Phase B+ (drag が必要)、MVP は start / end のみ
//
// raw 値 (page_id 等) を画面 / DOM テキストに出さない:
//   - <option> の value には page_id を入れるが、表示文字列は「ページ N」+ caption (任意) +
//     枚数のみで raw を出さない
//   - photo_id は本 component では扱わない (caller の責務)
"use client";

import { useId, useState } from "react";

import type { EditPage, MovePosition } from "@/lib/editPhotobook";

type Props = {
  /** 移動先候補となる photobook 配下の全 page (display_order ASC)。 */
  pages: EditPage[];
  /** 現在 photo が属している page (= 同 page 移動を抑止するため identifier として渡す)。 */
  currentPageId: string;
  /** 親 component が busy 状態なら全 control を disable。 */
  disabled?: boolean;
  /** 「実行」押下で呼ばれる callback。 */
  onMove: (targetPageId: string, position: MovePosition) => Promise<void>;
};

export function PageMovePicker({ pages, currentPageId, disabled, onMove }: Props) {
  const selectId = useId();

  // 初期選択は「現在の page 以外で最も index が小さい page」、なければ undefined。
  const firstOther = pages.find((p) => p.pageId !== currentPageId);
  const [target, setTarget] = useState<string>(firstOther?.pageId ?? "");
  const [position, setPosition] = useState<MovePosition>("end");
  const [busy, setBusy] = useState(false);

  const isSamePage = target === currentPageId;
  const canExecute = !disabled && !busy && target !== "" && !isSamePage;

  const handleExecute = async () => {
    if (!canExecute) return;
    setBusy(true);
    try {
      await onMove(target, position);
    } finally {
      setBusy(false);
    }
  };

  // 候補 page が現在 page だけの場合、移動先がないため picker 全体を disable。
  const hasTarget = pages.some((p) => p.pageId !== currentPageId);

  return (
    <div className="flex flex-wrap items-center gap-2" data-testid="page-move-picker">
      <label htmlFor={selectId} className="sr-only">
        移動先ページ
      </label>
      <select
        id={selectId}
        value={target}
        disabled={disabled || busy || !hasTarget}
        onChange={(e) => setTarget(e.target.value)}
        className="h-8 rounded-md border border-divider bg-surface px-2 text-[11px] text-ink-strong disabled:cursor-not-allowed disabled:opacity-45"
        data-testid="page-move-picker-target"
      >
        {!hasTarget && <option value="">(他のページがありません)</option>}
        {pages.map((p) => {
          const isCurrent = p.pageId === currentPageId;
          const photoCount = p.photos.length;
          // raw page_id は表示しない。表示は「ページ N: caption (M 枚)」形式。
          const cap = p.caption && p.caption.trim() !== "" ? `: ${p.caption}` : "";
          return (
            <option key={p.pageId} value={p.pageId} disabled={isCurrent}>
              {`ページ ${p.displayOrder + 1}${cap} (${photoCount} 枚)${isCurrent ? " - 現在" : ""}`}
            </option>
          );
        })}
      </select>
      <div role="group" aria-label="挿入位置" className="inline-flex h-8 overflow-hidden rounded-md border border-divider">
        <button
          type="button"
          disabled={disabled || busy || !hasTarget}
          aria-pressed={position === "start"}
          onClick={() => setPosition("start")}
          className={`px-2 text-[11px] font-semibold transition-colors disabled:cursor-not-allowed disabled:opacity-45 ${
            position === "start"
              ? "bg-teal-50 text-teal-700"
              : "bg-surface text-ink-medium hover:text-ink-strong"
          }`}
          data-testid="page-move-picker-position-start"
        >
          先頭
        </button>
        <button
          type="button"
          disabled={disabled || busy || !hasTarget}
          aria-pressed={position === "end"}
          onClick={() => setPosition("end")}
          className={`border-l border-divider px-2 text-[11px] font-semibold transition-colors disabled:cursor-not-allowed disabled:opacity-45 ${
            position === "end"
              ? "bg-teal-50 text-teal-700"
              : "bg-surface text-ink-medium hover:text-ink-strong"
          }`}
          data-testid="page-move-picker-position-end"
        >
          末尾
        </button>
      </div>
      <button
        type="button"
        disabled={!canExecute}
        onClick={() => void handleExecute()}
        className="inline-flex h-8 items-center rounded-md border border-divider bg-surface px-2.5 text-[11px] font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-45"
        data-testid="page-move-picker-execute"
      >
        移動
      </button>
    </div>
  );
}
