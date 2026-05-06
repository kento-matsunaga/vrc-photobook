// PageMeta: 各ページのメタ情報行（日付 / World / Cast / Photographer）。
//
// デザイン参照:
//   - design 最終調整版 §2 PC main col 「01 Page 01 [📅 日付] [🌍 World] [👥 Cast] [📷 Photographer]」
//   - design 最終調整版 §1 Mobile cover_first / 軽め(light) のメタバッジ行
//
// セキュリティ:
//   - meta フィールドはすべて作成者の自由テキスト。React 自動エスケープに任せる
//   - innerHTML / dangerouslySetInnerHTML は使わない

import type { PublicPageMeta } from "@/lib/publicPhotobook";

type Props = {
  meta?: PublicPageMeta;
};

/**
 * メタ行。meta が undefined または全フィールド空のときは null を返して
 * DOM を消費しない（Backend 未拡張のときの安全側挙動）。
 */
export function PageMeta({ meta }: Props) {
  if (!meta) return null;
  const eventDate = meta.eventDate ? formatDateLabel(meta.eventDate) : undefined;
  const world = meta.world?.trim() || undefined;
  const castList = meta.castList?.filter((s) => s.trim().length > 0) ?? [];
  const photographer = meta.photographer?.trim() || undefined;

  if (!eventDate && !world && castList.length === 0 && !photographer) {
    return null;
  }

  return (
    <div
      className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-ink-medium"
      data-testid="page-meta"
    >
      {eventDate && (
        <span className="inline-flex items-center gap-1">
          <span aria-hidden="true">📅</span>
          <span className="font-num">{eventDate}</span>
        </span>
      )}
      {world && (
        <span className="inline-flex items-center gap-1">
          <span aria-hidden="true">🌍</span>
          <span className="font-medium">World</span>
          <span>{world}</span>
        </span>
      )}
      {castList.length > 0 && (
        <span className="inline-flex items-center gap-1">
          <span aria-hidden="true">👥</span>
          <span className="font-medium">Cast</span>
          <span>{castList.join(" / ")}</span>
        </span>
      )}
      {photographer && (
        <span className="inline-flex items-center gap-1">
          <span aria-hidden="true">📷</span>
          <span className="font-medium">Photographer</span>
          <span>{photographer}</span>
        </span>
      )}
    </div>
  );
}

/** "YYYY-MM-DD" → "YYYY.MM.DD"。invalid は空文字 */
function formatDateLabel(iso: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m) return "";
  return `${m[1]}.${m[2]}.${m[3]}`;
}
