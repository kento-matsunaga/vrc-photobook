"use client";

// LightboxTrigger: 写真の <img> を覆う <button>。
// クリック / タップで Lightbox を open する。
//
// セキュリティ:
//   - photoIndex のみを受け取り、URL や photobook_id は持たない
//   - aria-label に raw ID / URL を出さない

import type { ReactNode } from "react";

import { useViewerInteraction } from "./ViewerInteractionProvider";

type Props = {
  photoIndex: number;
  className?: string;
  children: ReactNode;
};

export function LightboxTrigger({ photoIndex, className, children }: Props) {
  const { openLightbox } = useViewerInteraction();
  return (
    <button
      type="button"
      onClick={() => openLightbox(photoIndex)}
      className={`group relative cursor-zoom-in border-0 bg-transparent p-0 ${className ?? ""}`}
      aria-label="写真を拡大表示"
      data-testid="lightbox-trigger"
    >
      {children}
    </button>
  );
}
