// LP hero に置く mock-book コンポーネント。
//
// 採用元:
//   - design/mockups/prototype/screens-a.jsx の `.mock-book`（mobile）
//   - design/mockups/prototype/pc-screens-a.jsx の `.pc-book-mock` / `.pc-book-pages` /
//     `.pc-book-card.c1` / `.pc-book-card.c2`（PC、rotate 小カード x2）
//   - design/mockups/prototype/styles.css の `.photo.v-a` 相当 gradient placeholder
//
// design-system:
//   - white card + radius-lg + shadow（mobile shadow-sm 相当 / PC shadow 相当）
//   - grid 1.1fr / 1fr（左: タイトル / 日付 / ワールド名、右: gradient cover photo 3:4）
//   - PC のみ後ろに gradient 小カードを 2 つ rotate(6deg)/(-3deg) で重ねる
//   - 文字: タイトル text-base font-extrabold leading-tight、日付・ワールド名 font-num text-xs
//
// 制約:
//   - 実画像は使わず gradient placeholder のみ（MVP）
//   - rotate 小カードは PC のみ（sm:rotate）
//   - ページ背景には gradient を出さない（カード内 photo に閉じ込める）
//
// 設計参照: harness/work-logs/2026-05-01_pr37-design-rebuild-plan.md §3.1 / §5

type Props = {
  title: string;
  /** 日付表記（例: 2026.04.24）。font-num で表示する数値文字列。 */
  date?: string;
  /** ワールド名（例: Midnight Social Club）。font-num で表示。 */
  worldLabel?: string;
};

const COVER_GRADIENT =
  "linear-gradient(135deg, #C4B5FD 0%, #F9A8D4 60%, #FBCFE8 100%)";
const DECO_C1_GRADIENT =
  "linear-gradient(135deg, #C4B5FD, #F9A8D4)";
const DECO_C2_GRADIENT =
  "linear-gradient(135deg, #A7F3D0, #BAE6FD)";

export function MockBook({ title, date, worldLabel }: Props) {
  return (
    <div data-testid="mock-book" className="relative">
      {/* PC のみ表示する rotate 小カード x2（mobile では非表示） */}
      <span
        aria-hidden="true"
        className="pointer-events-none absolute -right-3 -top-2 hidden h-16 w-12 rotate-6 rounded border-[3px] border-surface shadow sm:block"
        style={{ backgroundImage: DECO_C1_GRADIENT }}
      />
      <span
        aria-hidden="true"
        className="pointer-events-none absolute -bottom-2 right-2 hidden h-14 w-16 -rotate-3 rounded border-[3px] border-surface shadow sm:block"
        style={{ backgroundImage: DECO_C2_GRADIENT }}
      />

      {/* 本体カード */}
      <div className="relative grid grid-cols-[1.1fr_1fr] gap-2 rounded-lg border border-divider bg-surface p-3 shadow-sm sm:p-4">
        <div className="flex flex-col justify-center px-1 py-2 sm:px-2">
          <p className="text-base font-extrabold leading-tight text-ink sm:text-lg">
            {title}
          </p>
          {date ? (
            <p className="mt-2 font-num text-[10px] text-ink-medium sm:text-xs">
              {date}
            </p>
          ) : null}
          {worldLabel ? (
            <p className="font-num text-[9px] text-ink-soft sm:text-[10px]">
              {worldLabel}
            </p>
          ) : null}
        </div>
        <div
          aria-hidden="true"
          className="aspect-[3/4] min-h-[130px] rounded-md border border-divider-soft"
          style={{ backgroundImage: COVER_GRADIENT }}
        />
      </div>
    </div>
  );
}

/**
 * LP の thumbnail strip（mobile 4 col / PC 5 col）用 1 セル。
 *
 * gradient placeholder photo を 1:1 アスペクトで描画する。
 * 写真がまだ無い MVP の視覚的フックとして prototype の `.photo.v-*` を再現。
 */
type ThumbProps = {
  variant: "a" | "b" | "c" | "d" | "e";
};

const THUMB_GRADIENTS: Record<ThumbProps["variant"], string> = {
  a: "linear-gradient(135deg, #C4B5FD, #F9A8D4 60%, #FBCFE8)",
  b: "linear-gradient(135deg, #A7F3D0, #BAE6FD 50%, #C4B5FD)",
  c: "linear-gradient(135deg, #FBCFE8, #DDD6FE 50%, #A5B4FC)",
  d: "linear-gradient(135deg, #FDE68A, #FBCFE8 60%, #C4B5FD)",
  e: "linear-gradient(135deg, #BFDBFE, #DDD6FE)",
};

export function MockThumb({ variant }: ThumbProps) {
  return (
    <span
      aria-hidden="true"
      data-testid={`mock-thumb-${variant}`}
      className="block aspect-square rounded-sm border border-divider-soft"
      style={{ backgroundImage: THUMB_GRADIENTS[variant] }}
    />
  );
}
