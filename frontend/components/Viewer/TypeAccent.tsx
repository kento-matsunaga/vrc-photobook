// TypeAccent: フォトブックタイプ別の小さな世界観アクセント。
//
// デザイン参照:
//   - design 最終調整版「タイプ別アクセント (例)」パネル
//
// MVP 実装方針:
//   - バッジ程度の最小実装。タイプ別の専用機能（衣装情報 / ワールド情報 / 作品番号）は後続 PR
//   - 表紙の付近 or ページヘッダ近辺に小さく出す
//   - 業務知識 v4 §4.1 のラベルを使用

type PhotobookType =
  | "event"
  | "daily"
  | "portfolio"
  | "avatar"
  | "world"
  | "memory"
  | "free";

type Props = {
  type: string;
};

const TYPE_LABEL: Record<PhotobookType, { label: string; tone: string }> = {
  event: { label: "イベント", tone: "bg-rose-50 text-rose-800 border-rose-200" },
  daily: { label: "おはツイ", tone: "bg-amber-50 text-amber-800 border-amber-200" },
  portfolio: {
    label: "作品集",
    tone: "bg-violet-50 text-violet-800 border-violet-200",
  },
  avatar: { label: "アバター紹介", tone: "bg-pink-50 text-pink-800 border-pink-200" },
  world: { label: "ワールド", tone: "bg-sky-50 text-sky-800 border-sky-200" },
  memory: {
    label: "思い出",
    tone: "bg-orange-50 text-orange-800 border-orange-200",
  },
  free: { label: "自由", tone: "bg-slate-50 text-slate-800 border-slate-200" },
};

export function TypeAccent({ type }: Props) {
  const key = isKnownType(type) ? type : "free";
  const { label, tone } = TYPE_LABEL[key];
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-[11px] font-bold tracking-[0.04em] ${tone}`}
      data-testid="type-accent"
      data-photobook-type={key}
    >
      {label}
    </span>
  );
}

function isKnownType(t: string): t is PhotobookType {
  return (
    t === "event" ||
    t === "daily" ||
    t === "portfolio" ||
    t === "avatar" ||
    t === "world" ||
    t === "memory" ||
    t === "free"
  );
}
