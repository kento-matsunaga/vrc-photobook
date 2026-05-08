// PageBlock: 1 page 分の見出し + page caption editor + photo grid をまとめる component。
//
// 設計参照:
//   - docs/plan/m2-edit-page-split-and-preview-plan.md §6.2
//
// 責務:
//   - page 見出し (「ページ N」表示)
//   - PageCaptionEditor (page caption の blur 保存)
//   - PhotoGrid (photo の caption / reorder / cover / 削除 / split / move)
//
// 範囲外 (P-6 で追加):
//   - PageActionBar (上と結合 / page 上下移動)
//   - merge / page reorder
"use client";

import type { EditPage, MovePosition } from "@/lib/editPhotobook";

import { PageActionBar } from "./PageActionBar";
import { PageCaptionEditor } from "./PageCaptionEditor";
import { PhotoGrid } from "./PhotoGrid";

type Props = {
  page: EditPage;
  /** photobook 配下の全 page (PhotoActionBar の PageMovePicker に渡す)。 */
  allPages: EditPage[];
  expectedVersion: number;
  isBusy: boolean;
  isCover: (imageId: string) => boolean;
  // photo handlers (PhotoGrid に bridge)
  onPhotoCaptionSave: (photoId: string, caption: string | null) => Promise<void>;
  onMoveUp: (photoId: string) => Promise<void>;
  onMoveDown: (photoId: string) => Promise<void>;
  onMoveTop: (photoId: string) => Promise<void>;
  onMoveBottom: (photoId: string) => Promise<void>;
  onSetCover: (imageId: string) => Promise<void>;
  onClearCover: () => Promise<void>;
  onRemovePhoto: (photoId: string) => Promise<void>;
  // page caption handler (A 方式 / version 反映は親で実施)
  onPageCaptionSave: (caption: string | null) => Promise<void>;
  // split / move handlers (B 方式 / setView は親で実施)
  splitDisabledReasonOf: (photoId: string, idx: number) => string | undefined;
  onSplit: (photoId: string) => Promise<void>;
  onMovePhoto: (photoId: string, targetPageId: string, position: MovePosition) => Promise<void>;
  // STOP P-6: page level mutations (merge / page reorder)。注入時のみ PageActionBar を出す。
  // merge は page index >= 1 のときのみ button が出る (PageActionBar 内部で判定)。
  // 1 page only photobook では PageActionBar 自体が何も描画しない。
  onMergeIntoPrev?: () => Promise<void>;
  onPageMoveUp?: () => Promise<void>;
  onPageMoveDown?: () => Promise<void>;
};

export function PageBlock({
  page,
  allPages,
  expectedVersion,
  isBusy,
  isCover,
  onPhotoCaptionSave,
  onMoveUp,
  onMoveDown,
  onMoveTop,
  onMoveBottom,
  onSetCover,
  onClearCover,
  onRemovePhoto,
  onPageCaptionSave,
  splitDisabledReasonOf,
  onSplit,
  onMovePhoto,
  onMergeIntoPrev,
  onPageMoveUp,
  onPageMoveDown,
}: Props) {
  const showPageActionBar =
    onMergeIntoPrev !== undefined &&
    onPageMoveUp !== undefined &&
    onPageMoveDown !== undefined;
  return (
    <section
      data-testid={`page-block-${page.pageId}`}
      className="space-y-3 rounded-lg border border-divider-soft bg-surface p-4 shadow-sm sm:p-5"
    >
      <header className="space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
            <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
            ページ {page.displayOrder + 1}
          </h2>
          {showPageActionBar && (
            <PageActionBar
              pageIndex={page.displayOrder}
              pageCount={allPages.length}
              disabled={isBusy}
              onMerge={onMergeIntoPrev}
              onMoveUp={onPageMoveUp}
              onMoveDown={onPageMoveDown}
            />
          )}
        </div>
        <PageCaptionEditor
          initialValue={page.caption ?? ""}
          disabled={isBusy}
          onSave={onPageCaptionSave}
        />
      </header>
      <PhotoGrid
        page={page}
        expectedVersion={expectedVersion}
        isCover={isCover}
        isBusy={isBusy}
        onCaptionSave={onPhotoCaptionSave}
        onMoveUp={onMoveUp}
        onMoveDown={onMoveDown}
        onMoveTop={onMoveTop}
        onMoveBottom={onMoveBottom}
        onSetCover={onSetCover}
        onClearCover={onClearCover}
        onRemovePhoto={onRemovePhoto}
        pages={allPages}
        splitDisabledReasonOf={splitDisabledReasonOf}
        onSplit={onSplit}
        onMovePhoto={onMovePhoto}
      />
    </section>
  );
}
