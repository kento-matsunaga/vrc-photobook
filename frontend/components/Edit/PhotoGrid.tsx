// PhotoGrid: 編集ページ用 photo grid。
//
// 機能:
//   - display variant を表示
//   - 各 photo に caption editor / reorder controls / 削除 / cover 設定 を配置
//
// セキュリティ:
//   - storage_key 完全値は応答 view に含まれないため、表示は presigned URL のみ
//   - presigned URL は HTML 内に出るが console.log しない
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
      <div className="rounded-lg border border-dashed border-divider bg-surface-soft px-4 py-8 text-center text-sm text-ink-medium">
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
    <ul className="space-y-4" data-testid="photo-grid">
      {page.photos.map((photo, idx) => {
        const isFirst = idx === 0;
        const isLast = idx === page.photos.length - 1;
        const cover = isCover(photo.imageId);
        const busy = isBusy || pendingId === photo.photoId;
        return (
          <li
            key={photo.photoId}
            className="overflow-hidden rounded-lg border border-divider bg-surface shadow-sm"
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
                <span className="absolute left-2 top-2 rounded bg-brand-teal px-2 py-1 text-xs font-medium text-white">
                  cover
                </span>
              )}
            </div>
            <div className="space-y-3 p-4">
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
                    className="rounded-sm border border-divider px-3 py-1 text-xs text-ink-medium hover:bg-surface-soft disabled:opacity-50"
                  >
                    coverを外す
                  </button>
                ) : (
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => wrap(photo.photoId, () => onSetCover(photo.imageId))}
                    className="rounded-sm border border-divider px-3 py-1 text-xs text-ink-medium hover:bg-surface-soft disabled:opacity-50"
                  >
                    coverに設定
                  </button>
                )}
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => wrap(photo.photoId, () => onRemovePhoto(photo.photoId))}
                  className="ml-auto rounded-sm border border-divider px-3 py-1 text-xs text-status-error hover:bg-status-error-soft disabled:opacity-50"
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
