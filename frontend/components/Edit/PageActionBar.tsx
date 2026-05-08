// PageActionBar: page header の操作 (上と結合 / page reorder ↑↓)。
//
// 設計参照:
//   - docs/plan/m2-edit-page-split-and-preview-plan.md §6.4 / §6.6
//
// 責務:
//   - 「↑ 上と結合」: page index >= 1 のときのみ button を出す
//     click → confirm() で確認、approved なら onMerge を呼ぶ
//     成功時は親側で setView (B 方式)
//     1 page only photobook では呼出されない (caller が page index 0 の場合は merge button を出さない)
//   - 「↑ ページを上へ」: 先頭 page (idx === 0) は disabled、それ以外は隣接 swap
//   - 「↓ ページを下へ」: 末尾 page (idx === pageCount - 1) は disabled、それ以外は隣接 swap
//
// disable 条件:
//   - busy: 親が mutation 進行中
//   - 単独 page (pageCount <= 1) では merge / reorder ボタンを非表示
//
// raw 値非露出: page_id を画面テキストに出さない (button label / tooltip / data-testid suffix のみ)
"use client";

type Props = {
  pageIndex: number;
  pageCount: number;
  disabled?: boolean;
  /** 「上と結合」押下。confirm UI は本 component 内で実施。 */
  onMerge: () => Promise<void>;
  /** ↑ 押下 (隣接 swap)。 */
  onMoveUp: () => Promise<void>;
  /** ↓ 押下 (隣接 swap)。 */
  onMoveDown: () => Promise<void>;
};

const MERGE_CONFIRM_MESSAGE =
  "上のページと結合します。このページのタイトルは破棄されます。よろしいですか?";

export function PageActionBar({
  pageIndex,
  pageCount,
  disabled,
  onMerge,
  onMoveUp,
  onMoveDown,
}: Props) {
  // 1 page only は何も出さない (cannot_remove_last_page reason の到達抑止 + UI clutter 回避)
  if (pageCount <= 1) {
    return null;
  }

  const isFirst = pageIndex === 0;
  const isLast = pageIndex === pageCount - 1;
  const baseDisabled = Boolean(disabled);

  const handleMerge = () => {
    if (baseDisabled) return;
    // window.confirm は SSR では呼べないが本 component は "use client"。
    // jsdom 環境 (vitest browser-like) では confirm が undefined のため、その場合は即実行。
    if (typeof window !== "undefined" && typeof window.confirm === "function") {
      if (!window.confirm(MERGE_CONFIRM_MESSAGE)) return;
    }
    void onMerge();
  };

  return (
    <div className="flex flex-wrap items-center gap-2" data-testid="page-action-bar">
      {!isFirst && (
        <button
          type="button"
          disabled={baseDisabled}
          onClick={handleMerge}
          aria-label="上のページと結合"
          className="inline-flex h-8 items-center rounded-md border border-divider bg-surface px-2.5 text-[11px] font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-45"
          data-testid="page-merge"
        >
          ↑ 上と結合
        </button>
      )}
      <div role="group" aria-label="ページの並び替え" className="inline-flex h-8 overflow-hidden rounded-md border border-divider">
        <button
          type="button"
          disabled={baseDisabled || isFirst}
          onClick={() => void onMoveUp()}
          aria-label="ページを上へ"
          title={isFirst ? "先頭ページのため上へ移動できません" : undefined}
          className="px-2 text-[11px] font-semibold text-ink-strong transition-colors hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-45"
          data-testid="page-reorder-up"
        >
          ↑
        </button>
        <button
          type="button"
          disabled={baseDisabled || isLast}
          onClick={() => void onMoveDown()}
          aria-label="ページを下へ"
          title={isLast ? "末尾ページのため下へ移動できません" : undefined}
          className="border-l border-divider px-2 text-[11px] font-semibold text-ink-strong transition-colors hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-45"
          data-testid="page-reorder-down"
        >
          ↓
        </button>
      </div>
    </div>
  );
}
