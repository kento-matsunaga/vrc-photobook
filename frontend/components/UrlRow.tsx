// UrlRow: 公開 URL / 管理 URL を表示する 1 行コンポーネント。
//
// design 参照: `design/source/project/wf-shared.jsx` の URL row pattern (β-1 design archive 正典移行後)
//
// セキュリティ:
//   - URL 値そのものはコンソール / 画面上のテキスト表示のみ。logging しない
//   - manage URL は識別色 violet を使う (現状 ManagePanel から kind="public" のみ呼出、manage path は
//     dead code 扱い、F-02/F-04 FOLLOW-UP で整理予定)

import type { ReactNode } from "react";

export type UrlRowKind = "public" | "manage";

type Props = {
  kind: UrlRowKind;
  label: string;
  url: string;
  /** 任意の右端アクション（例: コピー）。Server Component から渡せるよう ReactNode で受ける */
  action?: ReactNode;
};

/**
 * 公開 URL / 管理 URL の表示行。
 *
 * mockup の UrlRow 構造を Tailwind で再現:
 *   - kind=public: brand-teal を使う
 *   - kind=manage: brand-violet を使う
 */
export function UrlRow({ kind, label, url, action }: Props) {
  const tone =
    kind === "public"
      ? "text-brand-teal"
      : "text-brand-violet";
  const bgTone =
    kind === "public"
      ? "bg-brand-teal-soft"
      : "bg-purple-50";
  return (
    <div
      className={`flex items-center justify-between gap-3 rounded-lg border border-divider px-4 py-3 ${bgTone}`}
      data-testid={`url-row-${kind}`}
    >
      <div className="min-w-0 flex-1">
        <div className={`text-xs font-medium ${tone}`}>{label}</div>
        <div className="mt-1 break-all font-num text-sm text-ink-strong">{url}</div>
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}
