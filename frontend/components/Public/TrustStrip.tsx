// LP / About の closing element として使う 4 cell horizontal trust strip。
//
// 採用元: design/mockups/prototype/screens-a.jsx の `.trust-strip`（mobile）
//          design/mockups/prototype/pc-screens-a.jsx の `<PCTrust/>`（PC）
//
// design-system:
//   - 4 セル grid（狭幅では 2 列に折り返し可、wrap 許容）
//   - cell: ico (text-brand-teal) + label (text-xs ink-medium)
//   - 区切りは border-divider-soft top
//
// 設計参照: harness/work-logs/2026-05-01_pr37-design-rebuild-plan.md §3.1 / §6

type Item = { label: string; iconPath: string };

// SVG path は design/mockups/prototype/shared.jsx Icon の最小ベクトル（stroke-current で teal）
const items: ReadonlyArray<Item> = [
  {
    label: "完全無料",
    iconPath: "M5 12l4.5 4.5L19 7", // Check
  },
  {
    label: "スマホで完結",
    iconPath:
      "M4 8a2 2 0 0 1 2-2h2l1.5-2h5L16 6h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8Z", // Camera
  },
  {
    label: "ログイン不要",
    iconPath:
      "M4 10h16v11H4zM8 10V7a4 4 0 0 1 8 0v3", // Lock
  },
  {
    label: "VRC ユーザー向け",
    iconPath:
      "M12 2l1.8 5.4L19 9.2l-5.2 1.8L12 16l-1.8-5L5 9.2l5.2-1.8z", // Sparkle (filled)
  },
];

export function TrustStrip() {
  return (
    <div
      data-testid="trust-strip"
      className="mt-8 grid grid-cols-2 gap-3 border-t border-divider-soft pt-4 sm:grid-cols-4"
    >
      {items.map((item) => (
        <div
          key={item.label}
          className="flex flex-col items-center gap-1 text-center"
        >
          <span aria-hidden="true" className="text-brand-teal">
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.8"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d={item.iconPath} />
            </svg>
          </span>
          <span className="text-xs font-medium text-ink-medium">{item.label}</span>
        </div>
      ))}
    </div>
  );
}
