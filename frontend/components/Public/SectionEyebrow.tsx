// 公開ページ共通の eyebrow（small accent label）コンポーネント。
//
// 採用元 (m2-design-refresh STOP β-2a):
//   - design/source/project/wireframe-styles.css:360-364 `.wf-eyebrow`
//     (font-size 11px / font-weight 700 / uppercase / letter-spacing 0.14em / color teal-600)
//   - design/source/project/wf-screens-a.jsx:49 `<div className="wf-eyebrow">VRC PhotoBook</div>`
//
// 制約: 装飾は文字色とトラッキングのみ。background や icon は持たせない。

type Props = {
  children: string;
};

export function SectionEyebrow({ children }: Props) {
  return (
    <p className="text-xs font-bold uppercase tracking-[0.14em] text-teal-600">
      {children}
    </p>
  );
}
