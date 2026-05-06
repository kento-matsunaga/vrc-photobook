"use client";

// ViewerInteractionProvider: Lightbox の open / close / index 状態を保持する Client Provider。
//
// 設計判断 (v2):
//   - 三角構造: Server (PageHero) を children として受け取り、内部の LightboxTrigger
//     (Client) が同じ Context を消費する。Provider の "use client" 必須 (落とし穴 #5)
//   - flat photos 配列を Provider props で受け取り、Lightbox 表示時はこれを使う
//     (落とし穴 #4 対応: 全ページ横断で「次の写真」ナビゲーション)
//   - history.pushState は使わない。MVP は Esc / × / 背景タップで閉じる、
//     Safari 戻るボタンの 4 経路目はインスタンス内の onClose で完結
//   - Lightbox 自体は本ファイルから default 動的 import せず、子から Lightbox を
//     直接 import する。Provider の責務は state holder + flat photos の expose のみ
//
// セキュリティ:
//   - presigned URL を含む photos array は Provider 内 state に保持するが、
//     console.log / data-attr 露出はしない

import { createContext, useCallback, useContext, useMemo, useState } from "react";
import type { ReactNode } from "react";

import type { PublicPhoto } from "@/lib/publicPhotobook";
import { Lightbox } from "@/components/Viewer/Lightbox";

type ViewerInteractionState = {
  isLightboxOpen: boolean;
  activeIndex: number;
};

type ViewerInteractionApi = ViewerInteractionState & {
  openAt: (index: number) => void;
  close: () => void;
  setIndex: (index: number) => void;
};

const ViewerInteractionContext = createContext<ViewerInteractionApi | null>(null);

export function useViewerInteraction(): ViewerInteractionApi {
  const ctx = useContext(ViewerInteractionContext);
  if (ctx === null) {
    throw new Error(
      "useViewerInteraction must be used within ViewerInteractionProvider",
    );
  }
  return ctx;
}

type Props = {
  /** 全ページ横断の flat photo 配列。Lightbox はこの index で navigate する */
  flatPhotos: PublicPhoto[];
  children: ReactNode;
};

export function ViewerInteractionProvider({ flatPhotos, children }: Props) {
  const [isOpen, setIsOpen] = useState<boolean>(false);
  const [index, setIdx] = useState<number>(0);

  const openAt = useCallback(
    (i: number) => {
      const clamped = Math.max(0, Math.min(i, flatPhotos.length - 1));
      setIdx(clamped);
      setIsOpen(true);
    },
    [flatPhotos.length],
  );

  const close = useCallback(() => {
    setIsOpen(false);
  }, []);

  const setIndex = useCallback(
    (i: number) => {
      const clamped = Math.max(0, Math.min(i, flatPhotos.length - 1));
      setIdx(clamped);
    },
    [flatPhotos.length],
  );

  const api = useMemo<ViewerInteractionApi>(
    () => ({
      isLightboxOpen: isOpen,
      activeIndex: index,
      openAt,
      close,
      setIndex,
    }),
    [isOpen, index, openAt, close, setIndex],
  );

  return (
    <ViewerInteractionContext.Provider value={api}>
      {children}
      <Lightbox
        photos={flatPhotos}
        isOpen={isOpen}
        activeIndex={index}
        onClose={close}
        onSelect={setIndex}
      />
    </ViewerInteractionContext.Provider>
  );
}
