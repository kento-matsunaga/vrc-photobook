// LP hero / sample で使うモック写真集の SSR コンポーネント。
//
// 採用元 (m2-design-refresh STOP β-2a):
//   - design/source/project/wf-screens-a.jsx:4-43 `MockBook`（M = small / PC = default）
//   - design/source/project/wireframe-styles.css:165-201 `.wf-box` / `.wf-img`
//   - design/source/project/wireframe-styles.css:209-227 `.wf-line` placeholder
//
// design 正典構造 (`wf-screens-a.jsx:7-41`):
//   - position relative の枠 (height: small=200 / default=280)
//   - 左 cover (絶対配置 width 58%, full height):
//     - bg: `linear-gradient(135deg, var(--teal-50) 0%, var(--paper) 60%)`
//     - border-radius: `4px 14px 14px 4px` (cover 背) / shadow-lg / padding 20
//     - flex-col justify-end で下端に title + 2 placeholder line
//   - 右 page (絶対配置 width 48%, height 85%, top: small=20 / default=30):
//     - 2x2 grid (top span 2 + 2 cell) / gap 6 / padding 6
//     - border-radius: `14px 4px 4px 14px` (page 背) / shadow-lg
//
// 「design はそのまま、足りないものは足す」(plan §0.1):
//   - design は title/date/world を `wf-line` 細棒 placeholder で表現するが、production は
//     real text 表示が必要 (LP hero と利用者文脈を結ぶ)。design 寸法/位置はそのままに、
//     line を real text に差し替える。
//   - 旧 violet/pink/teal vivid gradient と PC rotate 装飾は design に存在しないため削除。
//
// 制約:
//   - 実画像は使わず、design の placeholder（subtle teal-50 → surface 系 gradient + dashed border）を SSR
//   - 右 page の 3 cell は aria-hidden="true"（装飾）
//
// 設計参照:
//   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §STOP β-2a
//   - docs/plan/m2-design-refresh-plan.md §0.1 / §6 STOP β-2

type Props = {
  /** Cover 左下に表示する photobook タイトル。改行は \n で表現可。 */
  title: string;
  /** 日付表記（例: 2026.04.24）。font-num で表示する数値文字列。 */
  date?: string;
  /** ワールド名（例: Midnight Social Club）。font-num で表示。 */
  worldLabel?: string;
};

export function MockBook({ title, date, worldLabel }: Props) {
  return (
    // outer 枠: mobile 200px / PC 280px (`wf-screens-a.jsx:6` `h = small ? 200 : 280`)
    <div data-testid="mock-book" className="relative h-[200px] sm:h-[280px]">
      {/* 左 cover (`wf-screens-a.jsx:9-23` width 58% / asymmetric radius 4-14-14-4 / teal-50→paper gradient / shadow-lg) */}
      <div className="absolute left-0 top-0 flex h-full w-[58%] flex-col justify-end overflow-hidden rounded-l-[4px] rounded-r-[14px] border border-divider bg-gradient-to-br from-teal-50 to-surface p-4 shadow-lg sm:p-5">
        <p className="whitespace-pre-line text-sm font-extrabold leading-tight text-ink sm:text-base">
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

      {/* 右 page (`wf-screens-a.jsx:24-40` width 48% / height 85% / top 20→30 / 2x2 grid top span 2 / shadow-lg) */}
      <div className="absolute right-0 top-[20px] grid h-[85%] w-[48%] grid-cols-2 grid-rows-2 gap-1.5 rounded-l-[14px] rounded-r-[4px] border border-divider bg-surface p-1.5 shadow-lg sm:top-[30px]">
        <span
          aria-hidden="true"
          className="col-span-2 rounded-sm border border-dashed border-divider-soft bg-gradient-to-br from-teal-50 to-surface-soft"
        />
        <span
          aria-hidden="true"
          className="rounded-sm border border-dashed border-divider-soft bg-gradient-to-br from-teal-50 to-surface-soft"
        />
        <span
          aria-hidden="true"
          className="rounded-sm border border-dashed border-divider-soft bg-gradient-to-br from-teal-50 to-surface-soft"
        />
      </div>
    </div>
  );
}

/**
 * LP の thumbnail strip（mobile 4 col 1:1 / PC 5 col 4:3）用 1 セル。
 *
 * 採用元 (m2-design-refresh STOP β-2a):
 *   - design/source/project/wf-screens-a.jsx:62-64 (Mobile 4 thumb, aspect 1/1, radius 6)
 *   - design/source/project/wf-screens-a.jsx:141-143 (PC 5 thumb, aspect 4/3, radius 10)
 *
 * 写真がまだ無い MVP の視覚的フックとして design の `WFImg`/`.wf-img` 相当を再現する。
 * 旧版で variant 毎の vivid gradient を使っていたのを廃し、design の subtle paper-style
 * placeholder（teal-50 → surface-soft の diagonal gradient + dashed border）に揃える。
 */
type ThumbProps = {
  variant: "a" | "b" | "c" | "d" | "e";
};

const THUMB_GRADIENTS: Record<ThumbProps["variant"], string> = {
  a: "linear-gradient(135deg, #EDFAF8, #F6F9FA)",
  b: "linear-gradient(135deg, #F0F6F7, #EDFAF8)",
  c: "linear-gradient(135deg, #EDFAF8, #FFFFFF)",
  d: "linear-gradient(135deg, #D4F2EE, #F0F6F7)",
  e: "linear-gradient(135deg, #F6F9FA, #EDFAF8)",
};

export function MockThumb({ variant }: ThumbProps) {
  return (
    <span
      aria-hidden="true"
      data-testid={`mock-thumb-${variant}`}
      className="block aspect-square rounded-sm border border-dashed border-divider-soft sm:aspect-[4/3]"
      style={{ backgroundImage: THUMB_GRADIENTS[variant] }}
    />
  );
}
