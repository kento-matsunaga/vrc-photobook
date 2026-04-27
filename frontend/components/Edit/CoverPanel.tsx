// CoverPanel: cover 画像のプレビュー / クリア。
//
// cover 設定は PhotoGrid 側の各 photo の「coverに設定」ボタンから。
// 本パネルは「現状の cover」を表示するためのもの。
"use client";

import type { EditVariantSet } from "@/lib/editPhotobook";

type Props = {
  cover?: EditVariantSet;
  coverTitle?: string;
  disabled?: boolean;
  onClear: () => Promise<void>;
};

export function CoverPanel({ cover, coverTitle, disabled, onClear }: Props) {
  if (!cover) {
    return (
      <section className="rounded-lg border border-dashed border-divider bg-surface-soft p-4 text-sm text-ink-medium">
        <h2 className="mb-2 text-h2 text-ink">表紙</h2>
        <p>表紙はまだ未設定です。photo 一覧から「coverに設定」を選んでください。</p>
      </section>
    );
  }
  return (
    <section className="space-y-3 rounded-lg border border-divider bg-surface p-4 shadow-sm">
      <h2 className="text-h2 text-ink">表紙</h2>
      <div className="overflow-hidden rounded-lg border border-divider-soft">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={cover.thumbnail.url}
          alt=""
          width={cover.thumbnail.width}
          height={cover.thumbnail.height}
          loading="lazy"
          decoding="async"
          className="block h-auto w-full"
        />
      </div>
      {coverTitle && (
        <p className="text-sm text-ink-medium">表紙タイトル: {coverTitle}</p>
      )}
      <button
        type="button"
        disabled={disabled}
        onClick={onClear}
        className="rounded-sm border border-divider px-3 py-1 text-xs text-ink-medium hover:bg-surface-soft disabled:opacity-50"
        data-testid="cover-clear"
      >
        表紙をクリア
      </button>
    </section>
  );
}
