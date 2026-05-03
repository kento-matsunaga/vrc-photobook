// CoverPanel: cover 画像のプレビュー / クリア。
//
// cover 設定は PhotoGrid 側の各 photo の「coverに設定」ボタンから。
// 本パネルは「現状の cover」を表示するためのもの。
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - design `wf-screens-b.jsx:49-60` (M) / `:127-132` (PC) `CoverPanel` 視覚整合
//   - wf-box / wf-section-title / wf-btn sm 風 (`wireframe-styles.css:165-175` / `:337-349` / `:250`)
//   - onClear / data-testid="cover-clear" / cover prop は **触らない**
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
      <section className="rounded-lg border-2 border-dashed border-divider-soft bg-surface-soft p-5 text-sm text-ink-medium sm:p-6">
        <SectionTitle>表紙</SectionTitle>
        <p className="text-xs leading-[1.6]">
          表紙はまだ未設定です。photo 一覧から「coverに設定」を選んでください。
        </p>
      </section>
    );
  }
  return (
    <section className="space-y-3 rounded-lg border border-divider-soft bg-surface p-5 shadow-sm sm:p-6">
      <SectionTitle>表紙</SectionTitle>
      <div className="overflow-hidden rounded-md border border-divider-soft">
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
        <p className="text-xs text-ink-medium">表紙タイトル: {coverTitle}</p>
      )}
      <button
        type="button"
        disabled={disabled}
        onClick={onClear}
        className="inline-flex h-9 w-full items-center justify-center rounded-md border border-divider bg-surface text-xs font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-45"
        data-testid="cover-clear"
      >
        表紙をクリア
      </button>
    </section>
  );
}

function SectionTitle({ children }: { children: string }) {
  return (
    <h2 className="mb-2 flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
      <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
      {children}
    </h2>
  );
}
