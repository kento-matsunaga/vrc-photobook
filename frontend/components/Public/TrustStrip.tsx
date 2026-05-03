// LP / About の closing element として使う 4 chip horizontal trust strip。
//
// 採用元 (m2-design-refresh STOP β-2a):
//   - design/source/project/wf-shared.jsx:67-73 `WFFooter` の trust block
//   - design/source/project/wireframe-styles.css:596-608 `.wf-trust`（chip 並び + ::before "✓"）
//
// design 正典の 4 chip (`wf-shared.jsx:69-72`):
//   - 完全無料 / スマホで完成 / 安全・安心 / VRCユーザー向け
//   - 旧 chip「スマホで完結」「ログイン不要」「VRC ユーザー向け」(space あり) を design 正典に置換
//
// 全 chip 共通で teal-500 の checkmark prefix を持つ (`wireframe-styles.css:604-608`)。
// 旧版で chip 別に Camera / Lock / Sparkle SVG を使っていたのを廃し、design に揃える。
//
// design 制約:
//   - 4 cell grid（狭幅では 2 列に折り返し可、wrap 許容）
//   - cell: ✓ checkmark (teal-500) + label (text-xs ink-medium)
//   - 区切りは border-divider-soft top
//
// 設計参照:
//   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §STOP β-2a Q-2a-3
//   - docs/plan/m2-design-refresh-plan.md §6 STOP β-2

const items: ReadonlyArray<string> = [
  "完全無料",
  "スマホで完成",
  "安全・安心",
  "VRCユーザー向け",
];

export function TrustStrip() {
  return (
    <div
      data-testid="trust-strip"
      className="mt-8 grid grid-cols-2 gap-3 border-t border-divider-soft pt-4 sm:grid-cols-4"
    >
      {items.map((label) => (
        <div
          key={label}
          className="flex items-center justify-center gap-1.5 text-center"
        >
          <span aria-hidden="true" className="font-bold text-teal-500">
            ✓
          </span>
          <span className="text-xs font-medium text-ink-medium">{label}</span>
        </div>
      ))}
    </div>
  );
}
