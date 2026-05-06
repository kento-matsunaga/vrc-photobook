"use client";

// ViewerInteractionProvider: 公開ビューアの interactivity を集約する Client Boundary。
//
// 役割:
//   - 全ページ横断の photo flat 配列を保持
//   - Lightbox の open / index 状態を保持
//   - LightboxTrigger / Lightbox から useViewerInteraction() で参照
//
// 設計:
//   - Server Components（PageHero など）を children として受け取る
//   - children 内の Client Component（LightboxTrigger）が context を消費する
//   - state を最上位に置き、Lightbox は同じ Provider 内に常駐させる
//
// セキュリティ:
//   - photo 配列の url（presigned）を console.log しない
//   - state を URL / localStorage に保存しない（reload で消えてよい）

import {
  type ReactNode,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

import type { PublicPhoto } from "@/lib/publicPhotobook";

import { Lightbox } from "./Lightbox";

type ViewerInteractionContextValue = {
  /** flat 配列内の index で lightbox を開く */
  openLightbox: (photoIndex: number) => void;
};

const ViewerInteractionContext =
  createContext<ViewerInteractionContextValue | null>(null);

type Props = {
  /** 全ページを通した flat photo 配列。Server 側で計算済み */
  photos: PublicPhoto[];
  children: ReactNode;
};

export function ViewerInteractionProvider({ photos, children }: Props) {
  const [lightboxIndex, setLightboxIndex] = useState<number | null>(null);

  const openLightbox = useCallback(
    (index: number) => {
      if (index < 0 || index >= photos.length) return;
      setLightboxIndex(index);
    },
    [photos.length],
  );

  const closeLightbox = useCallback(() => {
    setLightboxIndex(null);
  }, []);

  // body scroll lock
  useEffect(() => {
    if (lightboxIndex === null) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, [lightboxIndex]);

  const value = useMemo<ViewerInteractionContextValue>(
    () => ({ openLightbox }),
    [openLightbox],
  );

  return (
    <ViewerInteractionContext.Provider value={value}>
      {children}
      {lightboxIndex !== null && (
        <Lightbox
          photos={photos}
          initialIndex={lightboxIndex}
          onClose={closeLightbox}
        />
      )}
    </ViewerInteractionContext.Provider>
  );
}

/** LightboxTrigger / 内部コンポーネントから利用 */
export function useViewerInteraction(): ViewerInteractionContextValue {
  const ctx = useContext(ViewerInteractionContext);
  if (!ctx) {
    // Provider 外で呼ばれた場合は no-op に倒して落ちないようにする
    return { openLightbox: () => undefined };
  }
  return ctx;
}
