// PageMeta: ページ単位のメタ情報をアイコン付きバッジ行で表示する。
//
// 採用元: TESTImage 完成イメージ「Date / World / Cast / Photographer」のメタ行
//
// 設計判断 (v2):
//   - meta 全部が undefined → null を返す (PageHero 側で「PageMeta が空なら描画しない」
//     を条件にせず、本コンポ自身が安全側に倒す。Provider 三角構造の落とし穴回避)
//   - castList は最大 4 件 + "他 N 名" 表記 (情報過多回避)
//   - icon は SVG inline (font 依存禁止)、aria-hidden で screen reader 重複防止
//
// 制約:
//   - Server Component。renderToStaticMarkup で全パターン table 駆動 test 可能
//   - 任意項目すべて optional、入力されたものだけ表示

import type { PublicPageMeta } from "@/lib/publicPhotobook";

type Props = {
  meta?: PublicPageMeta;
};

const CAST_VISIBLE_MAX = 4;

function formatEventDate(iso: string): string {
  // "2026-04-29" → "2026.04.29"
  const m = iso.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!m) return iso;
  return `${m[1]}.${m[2]}.${m[3]}`;
}

export function PageMeta({ meta }: Props) {
  if (!meta) return null;
  const items: Array<{ key: string; node: React.ReactNode }> = [];

  if (meta.eventDate && meta.eventDate.trim() !== "") {
    items.push({
      key: "date",
      node: (
        <MetaItem icon={<IconCalendar />} label="Date">
          <span className="font-num">{formatEventDate(meta.eventDate)}</span>
        </MetaItem>
      ),
    });
  }
  if (meta.world && meta.world.trim() !== "") {
    items.push({
      key: "world",
      node: (
        <MetaItem icon={<IconGlobe />} label="World">
          <span>{meta.world}</span>
        </MetaItem>
      ),
    });
  }
  if (meta.castList && meta.castList.length > 0) {
    const visible = meta.castList.slice(0, CAST_VISIBLE_MAX);
    const overflow = meta.castList.length - visible.length;
    items.push({
      key: "cast",
      node: (
        <MetaItem icon={<IconUsers />} label="Cast">
          <span className="font-num">
            {visible.join(" / ")}
            {overflow > 0 ? ` 他 ${overflow} 名` : ""}
          </span>
        </MetaItem>
      ),
    });
  }
  if (meta.photographer && meta.photographer.trim() !== "") {
    items.push({
      key: "photographer",
      node: (
        <MetaItem icon={<IconCamera />} label="Photographer">
          <span>{meta.photographer}</span>
        </MetaItem>
      ),
    });
  }

  if (items.length === 0) return null;

  return (
    <ul
      data-testid="page-meta"
      className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-ink-medium sm:text-[13px]"
    >
      {items.map((it) => (
        <li key={it.key}>{it.node}</li>
      ))}
    </ul>
  );
}

function MetaItem({
  icon,
  label,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span aria-hidden="true" className="text-teal-600">
        {icon}
      </span>
      <span className="sr-only">{label}: </span>
      {children}
    </span>
  );
}

function IconCalendar() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="3" y="5" width="18" height="16" rx="2" />
      <path d="M3 10h18 M8 3v4 M16 3v4" />
    </svg>
  );
}

function IconGlobe() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18 M12 3a14 14 0 0 1 0 18 M12 3a14 14 0 0 0 0 18" />
    </svg>
  );
}

function IconUsers() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M22 21v-2a4 4 0 0 0-3-3.87 M17 3.13a4 4 0 0 1 0 7.75" />
    </svg>
  );
}

function IconCamera() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M5 7h3l2-3h4l2 3h3a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2z" />
      <circle cx="12" cy="13" r="3.5" />
    </svg>
  );
}
