"use client";

// PageNavSidebar: PC 左の縦サムネイル + ページ番号ナビ。
//
// 採用元: TESTImage 完成イメージ「PC (デスクトップ)」左コラム
//
// 設計判断 (v2):
//   - Client Component (scroll spy で active ハイライト)
//   - IntersectionObserver でページ要素の表示を監視。一番上に来た要素を active 扱い
//   - クリックで anchor jump (`#page-N`)、scroll smooth は CSS で global 設定済とする
//   - Mobile では非表示 (sm:block で制御)
//
// セキュリティ:
//   - thumbnail URL は src 渡しのみ、photobook_id / image_id は data-* に出さない

import { useEffect, useRef, useState } from "react";

import type { PublicPage } from "@/lib/publicPhotobook";

type Props = {
  pages: PublicPage[];
};

export function PageNavSidebar({ pages }: Props) {
  const [activePageNumber, setActivePageNumber] = useState<number>(1);
  const containerRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (typeof window === "undefined" || !("IntersectionObserver" in window)) {
      return;
    }
    const observed: HTMLElement[] = [];
    pages.forEach((_, i) => {
      const el = document.getElementById(`page-${i + 1}`);
      if (el instanceof HTMLElement) observed.push(el);
    });
    if (observed.length === 0) return;

    const obs = new IntersectionObserver(
      (entries) => {
        // 上端寄りで表示されている要素を選ぶ (boundingClientRect.top が小さい正値)
        const visible = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible.length > 0) {
          const id = visible[0].target.id;
          const m = id.match(/^page-(\d+)$/);
          if (m) setActivePageNumber(parseInt(m[1], 10));
        }
      },
      { rootMargin: "-100px 0px -50% 0px", threshold: [0, 0.1, 0.5, 1] },
    );
    observed.forEach((el) => obs.observe(el));
    return () => obs.disconnect();
  }, [pages]);

  return (
    <nav
      ref={containerRef}
      data-testid="viewer-page-nav"
      aria-label="ページナビゲーション"
      className="sticky top-[88px] flex max-h-[calc(100vh-120px)] flex-col gap-2 self-start overflow-y-auto pr-1"
    >
      {pages.map((page, i) => {
        const pageNumber = i + 1;
        const padded = String(pageNumber).padStart(2, "0");
        const hero = page.photos[0];
        const isActive = pageNumber === activePageNumber;
        return (
          <a
            key={pageNumber}
            href={`#page-${pageNumber}`}
            data-testid={`viewer-page-nav-${pageNumber}`}
            aria-current={isActive ? "page" : undefined}
            className={`group flex items-center gap-3 rounded-md p-2 transition-colors ${
              isActive ? "bg-teal-50" : "hover:bg-surface-soft"
            }`}
          >
            <span
              className={`block h-12 w-12 shrink-0 overflow-hidden rounded ${
                isActive ? "ring-2 ring-teal-500" : "ring-1 ring-divider-soft"
              }`}
            >
              {hero ? (
                <img
                  src={hero.variants.thumbnail.url}
                  alt=""
                  width={hero.variants.thumbnail.width}
                  height={hero.variants.thumbnail.height}
                  loading="lazy"
                  decoding="async"
                  className="h-full w-full object-cover"
                />
              ) : (
                <span className="block h-full w-full bg-gradient-to-br from-teal-50 to-surface-soft" />
              )}
            </span>
            <span className="flex-1 text-left">
              <span
                className={`block font-num text-base font-bold leading-tight ${
                  isActive ? "text-teal-700" : "text-ink-strong"
                }`}
              >
                {padded}
              </span>
              {page.caption ? (
                <span className="block truncate text-[11px] text-ink-medium">
                  {page.caption}
                </span>
              ) : null}
            </span>
          </a>
        );
      })}
    </nav>
  );
}
