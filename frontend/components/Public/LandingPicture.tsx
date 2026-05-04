// LP / MockBook / sample strip 共通の `<picture>` + WebP→JPEG fallback wrapper。
//
// 採用元 (m2-design-refresh STOP β-2c):
//   - frontend/scripts/build-landing-images.sh が生成する
//     `frontend/public/img/landing/{slug}.webp|.jpg` を参照
//   - design は raw photo placeholder のみ提示。実画像差し替えは β-2c で本コンポを通じて行う
//
// ε-fix (本 commit、visual のみ):
//   - LandingImage 型に objectPosition?: string を追加
//   - 顔・主役位置が中央以外にある画像で `object-cover` 中央クロップが切れる問題を回避
//   - img style に `objectPosition` を渡す。className 側の `object-cover` と組合せで使う想定
//
// 制約:
//   - layout shift 回避のため width / height を intrinsic 値で必ず指定する
//   - eager=true は LP hero など above-the-fold の画像のみ。それ以外は lazy 既定
//   - alt に空文字 ("") を許容: 同じ画像が別所で意味ある alt と共に表示されるときの装飾用途
//   - Workers ASSETS binding で配信される static asset のみを参照する (next/image 不使用)
//
// 設計参照: docs/plan/m2-design-refresh-stop-beta-2-plan.md §3

export type LandingImage = {
  /** /img/landing/{slug}.webp / .jpg として参照される stable name (拡張子なし) */
  slug: string;
  /** alt 文 (a11y / SEO 必須)。装飾用途の重複表示時のみ "" を許容 */
  alt: string;
  /** 表示用 intrinsic width (px) — CLS 回避 */
  width: number;
  /** 表示用 intrinsic height (px) — CLS 回避 */
  height: number;
  /**
   * `object-position` 値 (例: "center 30%")。className 側で `object-cover` を指定したときの
   * クロップ中心を制御する。未指定時は CSS 既定 (center center) で表示。顔が上寄り / 下寄り
   * の写真で中央クロップが切れるときに利用。
   */
  objectPosition?: string;
};

type Props = LandingImage & {
  className?: string;
  /** 既定 lazy。LP hero など above-the-fold は true (eager) を指定 */
  eager?: boolean;
};

export function LandingPicture({
  slug,
  alt,
  width,
  height,
  className,
  eager = false,
  objectPosition,
}: Props) {
  return (
    <picture>
      <source srcSet={`/img/landing/${slug}.webp`} type="image/webp" />
      <img
        src={`/img/landing/${slug}.jpg`}
        alt={alt}
        width={width}
        height={height}
        className={className}
        loading={eager ? "eager" : "lazy"}
        decoding="async"
        style={objectPosition ? { objectPosition } : undefined}
      />
    </picture>
  );
}
