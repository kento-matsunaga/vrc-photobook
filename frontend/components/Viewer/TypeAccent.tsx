// TypeAccent: photobook.type に応じた小さなバッジ表示。
//
// 採用元: TESTImage 完成イメージ「タイプ別アクセント」7 種
//
// 設計判断 (v2):
//   - 7 type それぞれにラベル + ベースカラー (teal palette からの派生で全体トーン崩さない)
//   - 未知の type は null (silent skip、Backend が後から増やしても破綻しない)
//   - バッジ最小実装 (icon は使わない、文字 + dot 装飾のみで軽量)

type AccentSpec = {
  label: string;
  toneCls: string;
};

const ACCENT_BY_TYPE: Record<string, AccentSpec> = {
  event: { label: "Event", toneCls: "border-teal-200 bg-teal-50 text-teal-700" },
  portfolio: {
    label: "Portfolio",
    toneCls: "border-purple-200 bg-purple-50 text-purple-700",
  },
  world: {
    label: "World",
    toneCls: "border-amber-200 bg-amber-50 text-amber-700",
  },
  casual: {
    label: "Casual",
    toneCls: "border-sky-200 bg-sky-50 text-sky-700",
  },
  oha_tweet: {
    label: "おはツイ",
    toneCls: "border-rose-200 bg-rose-50 text-rose-700",
  },
  collection: {
    label: "Collection",
    toneCls: "border-emerald-200 bg-emerald-50 text-emerald-700",
  },
  archive: {
    label: "Archive",
    toneCls: "border-slate-300 bg-slate-100 text-slate-700",
  },
  // alias: API は memory / outing 等を将来追加する可能性。未知は null
  memory: {
    label: "Memory",
    toneCls: "border-teal-200 bg-teal-50 text-teal-700",
  },
};

type Props = {
  type: string;
};

export function TypeAccent({ type }: Props) {
  const spec = ACCENT_BY_TYPE[type];
  if (!spec) return null;
  return (
    <span
      data-testid={`type-accent-${type}`}
      className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-0.5 text-[11px] font-bold tracking-wide ${spec.toneCls}`}
    >
      <span aria-hidden="true" className="block h-1.5 w-1.5 rounded-full bg-current opacity-70" />
      {spec.label}
    </span>
  );
}
