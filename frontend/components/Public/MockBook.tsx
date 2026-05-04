// LP hero / sample で使うモック写真集の SSR コンポーネント。
//
// 採用元 (m2-design-refresh STOP β-2a / β-2c):
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
// β-2c (本 commit): design は placeholder のみ提示していた cover / spread cell に
// 実画像 (mock-cover / hero / sample) を差し込む。
//   - 左 cover: 背景に <picture> 写真 + dark gradient overlay で title 可読性確保
//   - 右 page top (col-span-2): 横長写真 (16:9 推奨) を object-cover で配置
//   - 右 page bottom L/R: 縦長写真を object-cover で配置 (sample strip と同写真は alt="" 装飾扱い)
//   - prop 未指定時は β-2a 同等の gradient placeholder fallback を維持 (後方互換)
//
// 制約:
//   - design 寸法 / 枠 / 影 / radius は β-2a と同じ
//   - 写真は frontend/scripts/build-landing-images.sh 生成物のみ参照 (raw filename を書かない)
//
// 設計参照:
//   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §3
//   - docs/plan/m2-design-refresh-plan.md §0.1 / §6 STOP β-2

import { LandingPicture, type LandingImage } from "./LandingPicture";

type Props = {
  /** Cover 左下に表示する photobook タイトル。改行は \n で表現可。 */
  title: string;
  /** 日付表記（例: 2026.04.24）。font-num で表示する数値文字列。 */
  date?: string;
  /** ワールド名（例: Midnight Social Club）。font-num で表示。 */
  worldLabel?: string;
  /** 左 cover 背景写真。未指定 → teal-50→surface gradient (design 互換 placeholder) */
  cover?: LandingImage;
  /** 右 page top (col-span-2) の写真。横長 16:9 推奨。未指定 → gradient placeholder */
  spreadTop?: LandingImage;
  /** 右 page bottom-left の写真。未指定 → gradient placeholder */
  spreadBottomLeft?: LandingImage;
  /** 右 page bottom-right の写真。未指定 → gradient placeholder */
  spreadBottomRight?: LandingImage;
};

export function MockBook({
  title,
  date,
  worldLabel,
  cover,
  spreadTop,
  spreadBottomLeft,
  spreadBottomRight,
}: Props) {
  const hasCover = Boolean(cover);
  return (
    // outer 枠: mobile 200px / PC 280px (`wf-screens-a.jsx:6` `h = small ? 200 : 280`)
    <div data-testid="mock-book" className="relative h-[200px] sm:h-[280px]">
      {/* 左 cover (`wf-screens-a.jsx:9-23` width 58% / asymmetric radius 4-14-14-4 / shadow-lg) */}
      <div className="absolute left-0 top-0 flex h-full w-[58%] flex-col justify-end overflow-hidden rounded-l-[4px] rounded-r-[14px] border border-divider bg-gradient-to-br from-teal-50 to-surface p-4 shadow-lg sm:p-5">
        {hasCover && cover ? (
          <>
            {/* 背景写真 (object-cover で枠を埋める / object-position は image 単位で調整、ε-fix) */}
            <span aria-hidden="true" className="absolute inset-0">
              <LandingPicture
                slug={cover.slug}
                alt={cover.alt}
                width={cover.width}
                height={cover.height}
                objectPosition={cover.objectPosition}
                className="h-full w-full object-cover"
                eager
              />
            </span>
            {/* title 可読性確保のための bottom 暗 gradient overlay */}
            <span
              aria-hidden="true"
              className="absolute inset-x-0 bottom-0 h-2/3 bg-gradient-to-t from-black/70 via-black/30 to-transparent"
            />
          </>
        ) : null}
        <div className="relative">
          <p
            className={`whitespace-pre-line text-sm font-extrabold leading-tight sm:text-base ${
              hasCover ? "text-white drop-shadow-md" : "text-ink"
            }`}
          >
            {title}
          </p>
          {date ? (
            <p
              className={`mt-2 font-num text-[10px] sm:text-xs ${
                hasCover ? "text-white/85" : "text-ink-medium"
              }`}
            >
              {date}
            </p>
          ) : null}
          {worldLabel ? (
            <p
              className={`font-num text-[9px] sm:text-[10px] ${
                hasCover ? "text-white/70" : "text-ink-soft"
              }`}
            >
              {worldLabel}
            </p>
          ) : null}
        </div>
      </div>

      {/* 右 page (`wf-screens-a.jsx:24-40` width 48% / height 85% / 2x2 grid top span 2 / shadow-lg) */}
      <div className="absolute right-0 top-[20px] grid h-[85%] w-[48%] grid-cols-2 grid-rows-2 gap-1.5 rounded-l-[14px] rounded-r-[4px] border border-divider bg-surface p-1.5 shadow-lg sm:top-[30px]">
        <SpreadCell image={spreadTop} colSpan2 eager />
        <SpreadCell image={spreadBottomLeft} />
        <SpreadCell image={spreadBottomRight} />
      </div>
    </div>
  );
}

function SpreadCell({
  image,
  colSpan2 = false,
  eager = false,
}: {
  image?: LandingImage;
  colSpan2?: boolean;
  eager?: boolean;
}) {
  const wrapperCls = `${
    colSpan2 ? "col-span-2" : ""
  } relative overflow-hidden rounded-sm border border-divider-soft`;
  if (image) {
    return (
      <span className={wrapperCls}>
        <LandingPicture
          slug={image.slug}
          alt={image.alt}
          width={image.width}
          height={image.height}
          objectPosition={image.objectPosition}
          className="h-full w-full object-cover"
          eager={eager}
        />
      </span>
    );
  }
  return (
    <span
      aria-hidden="true"
      className={`${wrapperCls} border-dashed bg-gradient-to-br from-teal-50 to-surface-soft`}
    />
  );
}

/**
 * LP の thumbnail strip（mobile 4 col 1:1 / PC 5 col 4:3）用 1 セル。
 *
 * 採用元 (m2-design-refresh STOP β-2a):
 *   - design/source/project/wf-screens-a.jsx:62-64 (Mobile 4 thumb, aspect 1/1, radius 6)
 *   - design/source/project/wf-screens-a.jsx:141-143 (PC 5 thumb, aspect 4/3, radius 10)
 *
 * β-2c (本 commit): image を渡すと <picture> で実画像表示、未指定時は β-2a の subtle teal
 * gradient placeholder を維持 (後方互換)。
 */
type ThumbProps = {
  variant: "a" | "b" | "c" | "d" | "e";
  /** sample 実画像 spec。未指定 → gradient placeholder */
  image?: LandingImage;
};

const THUMB_GRADIENTS: Record<ThumbProps["variant"], string> = {
  a: "linear-gradient(135deg, #EDFAF8, #F6F9FA)",
  b: "linear-gradient(135deg, #F0F6F7, #EDFAF8)",
  c: "linear-gradient(135deg, #EDFAF8, #FFFFFF)",
  d: "linear-gradient(135deg, #D4F2EE, #F0F6F7)",
  e: "linear-gradient(135deg, #F6F9FA, #EDFAF8)",
};

export function MockThumb({ variant, image }: ThumbProps) {
  const baseCls =
    "block aspect-square overflow-hidden rounded-sm border border-divider-soft sm:aspect-[4/3]";
  if (image) {
    return (
      <span data-testid={`mock-thumb-${variant}`} className={baseCls}>
        <LandingPicture
          slug={image.slug}
          alt={image.alt}
          width={image.width}
          height={image.height}
          objectPosition={image.objectPosition}
          className="h-full w-full object-cover"
        />
      </span>
    );
  }
  return (
    <span
      aria-hidden="true"
      data-testid={`mock-thumb-${variant}`}
      className={`${baseCls} border-dashed`}
      style={{ backgroundImage: THUMB_GRADIENTS[variant] }}
    />
  );
}
