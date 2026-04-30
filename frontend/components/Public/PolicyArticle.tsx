// Terms / Privacy 共通の「第 N 条」記事コンポーネント。
//
// 採用元: 既存 frontend/app/(public)/help/manage-url/page.tsx の温度感
//          + design-system text-h2 / text-sm / divider-soft
//
// design-system:
//   - h2 (text-h2) + body (text-sm ink-strong) + bullet list (list-disc pl-5)
//   - 各セクションは aria-labelledby で TOC anchor scroll に対応
//   - セクション間は親レイアウトで space-y-* を使い、本コンポは内側のみ責任を持つ
//
// 設計参照: harness/work-logs/2026-05-01_pr37-design-rebuild-plan.md §3.3 / §3.4 / §6
//
// 用法:
//   <PolicyArticle id="terms-1" number="第 1 条" title="サービスの目的と性質">
//     <p>...</p>
//     <ul className="list-disc space-y-1 pl-5"><li>...</li></ul>
//   </PolicyArticle>

import type { ReactNode } from "react";

type Props = {
  /** anchor id（例: "terms-1"）。TOC の href="#terms-1" に対応。 */
  id: string;
  /** 第 N 条のラベル。例: "第 1 条" */
  number: string;
  /** 条のタイトル。例: "サービスの目的と性質" */
  title: string;
  children: ReactNode;
};

export function PolicyArticle({ id, number, title, children }: Props) {
  const headingId = `${id}-heading`;
  return (
    <section
      id={id}
      aria-labelledby={headingId}
      data-testid={`policy-article-${id}`}
      className="scroll-mt-6 border-t border-divider-soft pt-6"
    >
      <h2 id={headingId} className="text-h2 text-ink">
        <span className="mr-2 font-num text-sm text-brand-teal">{number}</span>
        {title}
      </h2>
      <div className="mt-3 space-y-2 text-sm text-ink-strong">{children}</div>
    </section>
  );
}

/**
 * Terms / Privacy 共通の TOC（目次）。
 *
 * 各 anchor は `<PolicyArticle id="...">` の id と一致させる。
 */
type TocItem = { id: string; label: string };

type TocProps = {
  items: ReadonlyArray<TocItem>;
  ariaLabel: string;
};

export function PolicyToc({ items, ariaLabel }: TocProps) {
  return (
    <nav
      aria-label={ariaLabel}
      data-testid="policy-toc"
      className="rounded-lg border border-divider bg-surface-soft p-4"
    >
      <p className="text-xs font-medium text-ink-medium">目次</p>
      <ul className="mt-2 grid gap-1 text-sm sm:grid-cols-2">
        {items.map((it) => (
          <li key={it.id}>
            <a
              href={`#${it.id}`}
              className="text-brand-teal underline hover:text-brand-teal-hover"
            >
              {it.label}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}

/**
 * Terms / Privacy 冒頭の「専門家レビュー前」notice box。
 *
 * 業務知識 v4 §7 の「法律文書ではないが業務前提として整理」方針 + design-system
 * の surface-soft + warn icon。
 */
type NoticeProps = {
  children: ReactNode;
};

export function PolicyNotice({ children }: NoticeProps) {
  return (
    <div
      data-testid="policy-notice"
      className="flex items-start gap-3 rounded-lg border border-divider bg-surface-soft p-4"
    >
      <span aria-hidden="true" className="mt-0.5 shrink-0 text-status-warn">
        <svg
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.8"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="M10.3 3.5L1.7 18a2 2 0 0 0 1.7 3h17.2a2 2 0 0 0 1.7-3L13.7 3.5a2 2 0 0 0-3.4 0Z" />
          <path d="M12 9v4M12 17h.01" />
        </svg>
      </span>
      <p className="text-sm text-ink-strong">{children}</p>
    </div>
  );
}
