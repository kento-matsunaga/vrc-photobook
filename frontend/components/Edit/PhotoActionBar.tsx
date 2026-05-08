// PhotoActionBar: 各 photo に対する split / move 操作 (m2-edit Phase A 補強)。
//
// 設計参照:
//   - docs/plan/m2-edit-page-split-and-preview-plan.md §6.3 / §6.5
//
// 責務:
//   - 「✂ ここで分ける」(split): photo の "次から" 新 page に分離
//   - 「他のページへ移動」(PageMovePicker): photo を別 page の start / end に移動
//
// disable 条件 (button 自体を disable し、tooltip で理由を出す):
//   - busy: 親が mutation 進行中
//   - splitDisabledReason 非空: 30 page 上限到達 / 末尾 photo 等
//
// 削除 / cover / 同 page reorder は本 component の責務外 (既存 PhotoGrid で扱う)。
"use client";

import type { EditPage, MovePosition } from "@/lib/editPhotobook";

import { PageMovePicker } from "./PageMovePicker";

type Props = {
  pages: EditPage[];
  currentPageId: string;
  disabled?: boolean;
  /** split が UI 上で行えない理由 (空文字 / undefined → 押下可能)。tooltip 表示にも使う。 */
  splitDisabledReason?: string;
  onSplit: () => Promise<void>;
  onMove: (targetPageId: string, position: MovePosition) => Promise<void>;
};

export function PhotoActionBar({
  pages,
  currentPageId,
  disabled,
  splitDisabledReason,
  onSplit,
  onMove,
}: Props) {
  const splitDisabled = Boolean(disabled) || Boolean(splitDisabledReason);

  return (
    <div className="flex flex-wrap items-center gap-2" data-testid="photo-action-bar">
      <button
        type="button"
        disabled={splitDisabled}
        onClick={() => void onSplit()}
        title={splitDisabledReason}
        aria-label="ここで分ける"
        className="inline-flex h-8 items-center rounded-md border border-divider bg-surface px-2.5 text-[11px] font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-45"
        data-testid="photo-split"
      >
        ここで分ける
      </button>
      <PageMovePicker
        pages={pages}
        currentPageId={currentPageId}
        disabled={disabled}
        onMove={onMove}
      />
    </div>
  );
}
