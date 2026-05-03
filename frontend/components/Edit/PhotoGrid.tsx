// PhotoGrid: 編集ページ用 photo grid。
//
// 機能:
//   - display variant を表示
//   - 各 photo に caption editor / reorder controls / 削除 / cover 設定 を配置
//
// セキュリティ:
//   - storage_key 完全値は応答 view に含まれないため、表示は presigned URL のみ
//   - presigned URL は HTML 内に出るが console.log しない
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - design `wf-screens-b.jsx:25-39` (M wf-grid-2) / `:135-153` (PC wf-grid-3) 視覚整合
//   - 各 photo card は `wf-box` 風 (rounded-lg + border-divider-soft + shadow-sm + p-2)
//   - cover badge を wf-badge.teal 風 (`wireframe-styles.css:386-389`)
//   - reorder / cover / 削除 ボタン視覚整合
//   - 全 data-testid (photo-grid / photo-row-{id} / photo-remove-{id}) **完全維持**
//   - onCaptionSave / onMoveUp/Down/Top/Bottom / onSetCover / onClearCover / onRemovePhoto
//     handler / pendingId state / wrap helper は **触らない**
"use client";

import { useState } from "react";

import type { EditPhoto, EditPage } from "@/lib/editPhotobook";
import { CaptionEditor } from "./CaptionEditor";
import { ReorderControls } from "./ReorderControls";

type Props = {
  page: EditPage;
  expectedVersion: number;
  isCover: (imageId: string) => boolean;
  isBusy: boolean;
  onCaptionSave: (photoId: string, caption: string | null) => Promise<void>;
  onMoveUp: (photoId: string) => Promise<void>;
  onMoveDown: (photoId: string) => Promise<void>;
  onMoveTop: (photoId: string) => Promise<void>;
  onMoveBottom: (photoId: string) => Promise<void>;
  onSetCover: (imageId: string) => Promise<void>;
  onClearCover: () => Promise<void>;
  onRemovePhoto: (photoId: string) => Promise<void>;
};

export function PhotoGrid({
  page,
  isCover,
  isBusy,
  onCaptionSave,
  onMoveUp,
  onMoveDown,
  onMoveTop,
  onMoveBottom,
  onSetCover,
  onClearCover,
  onRemovePhoto,
}: Props) {
  const [pendingId, setPendingId] = useState<string | null>(null);

  if (page.photos.length === 0) {
    return (
      <div className="rounded-lg border-2 border-dashed border-divider-soft bg-surface-soft px-4 py-8 text-center text-xs text-ink-medium">
        まだ写真がありません。下のアップロード欄から写真を追加してください。
      </div>
    );
  }

  const wrap = async (id: string, fn: () => Promise<void>) => {
    setPendingId(id);
    try {
      await fn();
    } finally {
      setPendingId(null);
    }
  };

  return (
    <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-1" data-testid="photo-grid">
      {page.photos.map((photo, idx) => {
        const isFirst = idx === 0;
        const isLast = idx === page.photos.length - 1;
        const cover = isCover(photo.imageId);
        const busy = isBusy || pendingId === photo.photoId;
        return (
          <li
            key={photo.photoId}
            className="overflow-hidden rounded-lg border border-divider-soft bg-surface shadow-sm"
            data-testid={`photo-row-${photo.photoId}`}
          >
            <div className="relative">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={photo.variants.display.url}
                alt=""
                width={photo.variants.display.width}
                height={photo.variants.display.height}
                loading="lazy"
                decoding="async"
                className="block h-auto w-full"
              />
              {cover && (
                <span className="absolute left-2 top-2 inline-flex items-center rounded-full border border-teal-100 bg-teal-50 px-2.5 py-0.5 text-[10.5px] font-bold text-teal-700">
                  cover
                </span>
              )}
            </div>
            <div className="space-y-3 p-3 sm:p-4">
              <CaptionEditor
                initialValue={photo.caption ?? ""}
                disabled={busy}
                onSave={(v) => wrap(photo.photoId, () => onCaptionSave(photo.photoId, v))}
              />
              <div className="flex flex-wrap items-center gap-2">
                <ReorderControls
                  disabled={busy}
                  isFirst={isFirst}
                  isLast={isLast}
                  onMoveUp={() => wrap(photo.photoId, () => onMoveUp(photo.photoId))}
                  onMoveDown={() => wrap(photo.photoId, () => onMoveDown(photo.photoId))}
                  onMoveTop={() => wrap(photo.photoId, () => onMoveTop(photo.photoId))}
                  onMoveBottom={() => wrap(photo.photoId, () => onMoveBottom(photo.photoId))}
                />
                {cover ? (
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => wrap(photo.photoId, () => onClearCover())}
                    className="inline-flex h-8 items-center rounded-md border border-divider bg-surface px-2.5 text-[11px] font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-45"
                  >
                    coverを外す
                  </button>
                ) : (
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => wrap(photo.photoId, () => onSetCover(photo.imageId))}
                    className="inline-flex h-8 items-center rounded-md border border-divider bg-surface px-2.5 text-[11px] font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-45"
                  >
                    coverに設定
                  </button>
                )}
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => wrap(photo.photoId, () => onRemovePhoto(photo.photoId))}
                  className="ml-auto inline-flex h-8 items-center rounded-md border border-divider bg-surface px-2.5 text-[11px] font-semibold text-status-error transition-colors hover:bg-status-error-soft disabled:cursor-not-allowed disabled:opacity-45"
                  data-testid={`photo-remove-${photo.photoId}`}
                >
                  削除
                </button>
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

// 直接 photo を export しないが、テストで型を参照したい場合のための再 export。
export type { EditPhoto };
