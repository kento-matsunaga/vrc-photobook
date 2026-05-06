// PageNote: 各ページの自由記述メモ（photobook_page_metas.note）。
//
// デザイン参照:
//   - design 最終調整版 §2 PC main col の「Note (メモ)」クリーム色背景の引用ブロック
//
// 配置: photo grid と次ページの間。caption とは別階層の「読み物」枠。

type Props = {
  note?: string;
};

export function PageNote({ note }: Props) {
  const trimmed = note?.trim();
  if (!trimmed) return null;
  return (
    <aside
      className="rounded-md border border-amber-100 bg-amber-50/80 px-4 py-3 text-sm leading-relaxed text-ink-strong"
      data-testid="page-note"
    >
      <p className="mb-1 text-[11px] font-bold tracking-[0.04em] text-amber-900/80">
        Note (メモ)
      </p>
      <p className="whitespace-pre-line">{trimmed}</p>
    </aside>
  );
}
