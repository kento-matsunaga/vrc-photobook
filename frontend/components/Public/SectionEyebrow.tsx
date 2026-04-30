// 公開ページ共通の eyebrow（small accent label）コンポーネント。
//
// 採用元: design/mockups/prototype/screens-a.jsx LP の eyebrow + design-system
// design-system: text-xs (11px / 1.4 / 500) + brand-teal + uppercase tracking
//
// 設計参照: harness/work-logs/2026-05-01_pr37-design-rebuild-plan.md §6 / §8
//
// 制約: 装飾は文字色とトラッキングのみ。background や icon は持たせない。

type Props = {
  children: string;
};

export function SectionEyebrow({ children }: Props) {
  return (
    <p className="text-xs font-medium uppercase tracking-wide text-brand-teal">
      {children}
    </p>
  );
}
