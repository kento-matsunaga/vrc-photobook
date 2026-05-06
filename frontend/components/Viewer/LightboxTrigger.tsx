"use client";

// LightboxTrigger: 写真を全画面表示するボタンラッパ。
//
// 設計判断 (v2):
//   - Server Component から JSX として呼び出し可能 (children に <img> を渡す)
//   - aria 的には button、Lightbox open は Provider の openAt(photoIndex) を呼ぶ
//   - photoIndex は ViewerLayout が計算した flat 配列上の位置
//   - cursor / focus ring / hover は Tailwind のみ
//
// セキュリティ:
//   - photoIndex は data-* に出さない (raw image_id ではないが、内部 index 露出も最小化)

import type { ReactNode } from "react";

import { useViewerInteraction } from "@/components/Viewer/ViewerInteractionProvider";

type Props = {
  photoIndex: number;
  ariaLabel: string;
  className?: string;
  children: ReactNode;
};

export function LightboxTrigger({
  photoIndex,
  ariaLabel,
  className,
  children,
}: Props) {
  const { openAt } = useViewerInteraction();
  return (
    <button
      type="button"
      aria-label={ariaLabel}
      onClick={() => openAt(photoIndex)}
      className={`cursor-zoom-in focus:outline-none focus-visible:ring-2 focus-visible:ring-teal-400 focus-visible:ring-offset-2 ${className ?? ""}`}
    >
      {children}
    </button>
  );
}
