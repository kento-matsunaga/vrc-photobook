// Terms / Privacy 共通の policy primitives (article / TOC / notice)。
//
// 採用元 (m2-design-refresh STOP β-2b-1):
//   - design/source/project/wireframe-styles.css:165-175 `.wf-box` (article card 寸法 / 影)
//   - design/source/project/wireframe-styles.css:337-349 `.wf-section-title`
//   - design/source/project/wireframe-styles.css:398-425 `.wf-note` (notice 視覚)
//   - design/source/project/wireframe-styles.css:538-545 `.wf-toc` (TOC left teal-bar)
//   - design/source/project/wf-screens-c.jsx:331-381 (Terms M / PC)
//   - design/source/project/wf-screens-c.jsx:384-442 (Privacy M / PC)
//
// design 正典の視覚:
//   - PolicyArticle: `wf-box` 風 card (rounded-lg / border-divider-soft / bg-surface /
//                    padding 5-6 / shadow-sm)。第 N 条 prefix は font-num teal-600
//   - PolicyToc:     `wf-toc` 風 left teal-200 border + 4px padding-left + grid。
//                    rounded-lg outer card は廃止し、design 通り inline left-bar
//   - PolicyNotice:  `wf-note` 風 border-left teal-300 + bg teal-50 + i icon (teal-500)
//
// 「足りないものは足す」(plan §0.1):
//   - production 補助として data-testid / aria-labelledby を維持
//   - PolicyArticle の scroll-mt は PublicTopBar sticky (~53px) を考慮し scroll-mt-20 (80px)
//     に変更。anchor scroll で article title が sticky topbar に隠れないようにする
//
// 既存 API は維持 (id / number / title / children / items / ariaLabel)。
//
// 設計参照:
//   - docs/plan/m2-design-refresh-stop-beta-2b-plan.md §1
//   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §2

import type { ReactNode } from "react";

type PolicyArticleProps = {
  /** anchor id（例: "terms-1"）。TOC の href="#terms-1" に対応。 */
  id: string;
  /** 第 N 条のラベル。例: "第 1 条" */
  number: string;
  /** 条のタイトル。例: "サービスの目的と性質" */
  title: string;
  children: ReactNode;
};

export function PolicyArticle({
  id,
  number,
  title,
  children,
}: PolicyArticleProps) {
  const headingId = `${id}-heading`;
  return (
    <section
      id={id}
      aria-labelledby={headingId}
      data-testid={`policy-article-${id}`}
      className="scroll-mt-20 rounded-lg border border-divider-soft bg-surface p-5 shadow-sm sm:p-6"
    >
      <h2 id={headingId} className="text-h2 text-ink">
        <span className="mr-2 font-num text-sm font-bold text-teal-600">
          {number}
        </span>
        {title}
      </h2>
      <div className="mt-3 space-y-2 text-sm leading-relaxed text-ink-strong">
        {children}
      </div>
    </section>
  );
}

/**
 * Terms / Privacy 共通の TOC（目次）。
 *
 * 採用元: design `wireframe-styles.css:538-545` `.wf-toc`
 *   - border-left 2px teal-200 + padding-left
 *   - display:grid + gap-2.5 + text-[12.5px] + ink-2
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
      className="border-l-2 border-teal-200 py-1 pl-4"
    >
      <p className="mb-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
        目次
      </p>
      <ul className="grid gap-2 text-[12.5px] text-ink-medium sm:grid-cols-2">
        {items.map((it) => (
          <li key={it.id}>
            <a
              href={`#${it.id}`}
              className="text-teal-600 underline hover:text-teal-700"
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
 * 採用元: design `wireframe-styles.css:398-425` `.wf-note`
 *   - border-left 3px teal-300 + bg teal-50 + radius 8 + padding 10x14 + flex gap 10
 *   - ::before "i" icon (white on teal-500 bg, round, font-serif italic)
 */
type NoticeProps = {
  children: ReactNode;
};

export function PolicyNotice({ children }: NoticeProps) {
  return (
    <div
      data-testid="policy-notice"
      className="flex items-start gap-2.5 rounded-lg border-l-[3px] border-teal-300 bg-teal-50 p-3.5"
    >
      <span
        aria-hidden="true"
        className="grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full bg-teal-500 font-serif text-xs font-bold italic leading-none text-white"
      >
        i
      </span>
      <p className="text-xs leading-[1.6] text-ink-strong">{children}</p>
    </div>
  );
}
