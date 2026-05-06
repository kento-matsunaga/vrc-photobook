"use client";

// Lightbox: 全画面ビューア (MVP)。
//
// 採用元: TESTImage 完成イメージ「Lightbox (全画面ビューア) iPhone 11」列
//
// MVP 5 機能 (プロンプト §1):
//   1. 写真タップで全画面表示 (LightboxTrigger 経由)
//   2. ピンチで拡大・縮小 (2 ポインタの距離比でスケール)
//   3. ダブルタップで等倍に戻る (300ms 以内の連続タップで scale=1, tx=0, ty=0)
//   4. ドラッグで移動 (拡大時のみ、1 ポインタの移動量を tx/ty に積算)
//   5. サムネイルで素早く移動 (下部 thumb strip クリックで activeIndex 切替)
//   + 閉じる: × / 背景タップ / Esc キー (3 経路、Safari 戻るは history 操作しないので
//     体感的に history.back で戻ると Lightbox は消えず page だけ遷移、これは MVP 範囲外)
//
// Phase 2 (本実装で作らない、プロンプト §1 やらないこと):
//   - 自動再生、100% 表示トグル、フルスクリーン API
//
// 設計判断 (v2):
//   - ライブラリ追加なし、pointer events + transform translate / scale で実装
//     (Q3 確認、cbabbe6 と同方針)
//   - touch-action: none を image 要素に付け、ブラウザのデフォルト pinch / pan を抑制
//   - body scroll は overflow-hidden で抑制 (open 中のみ)
//   - presigned URL は src 渡しのみ。15 分で expire するが MVP は割り切り、エラー時は
//     onError で隣の photo へ自動 skip しないが、黒画面で固まらないよう alt と
//     エラー時の薄いプレースホルダ枠を出す
//
// セキュリティ:
//   - photo index / image_id は data-attr に出さない
//   - presigned URL を console / data-* に出さない

import { useEffect, useRef, useState } from "react";
import type { PointerEvent as ReactPointerEvent } from "react";

import type { PublicPhoto } from "@/lib/publicPhotobook";

type Props = {
  photos: PublicPhoto[];
  isOpen: boolean;
  activeIndex: number;
  onClose: () => void;
  onSelect: (index: number) => void;
};

type GestureState = {
  pointers: Map<number, { x: number; y: number }>;
  initialDistance: number;
  initialScale: number;
  initialTx: number;
  initialTy: number;
  lastTapTime: number;
  lastTapX: number;
  lastTapY: number;
  panStartX: number;
  panStartY: number;
};

const MIN_SCALE = 1;
const MAX_SCALE = 4;
const DOUBLE_TAP_MS = 300;
const DOUBLE_TAP_DISTANCE_PX = 30;

function distance(
  a: { x: number; y: number },
  b: { x: number; y: number },
): number {
  const dx = a.x - b.x;
  const dy = a.y - b.y;
  return Math.hypot(dx, dy);
}

export function Lightbox({
  photos,
  isOpen,
  activeIndex,
  onClose,
  onSelect,
}: Props) {
  const [scale, setScale] = useState<number>(1);
  const [tx, setTx] = useState<number>(0);
  const [ty, setTy] = useState<number>(0);

  const gestureRef = useRef<GestureState>({
    pointers: new Map(),
    initialDistance: 0,
    initialScale: 1,
    initialTx: 0,
    initialTy: 0,
    lastTapTime: 0,
    lastTapX: 0,
    lastTapY: 0,
    panStartX: 0,
    panStartY: 0,
  });

  // open / index 変更時に scale / 位置をリセット
  useEffect(() => {
    if (isOpen) {
      setScale(1);
      setTx(0);
      setTy(0);
    }
  }, [isOpen, activeIndex]);

  // body scroll lock + Esc キー
  useEffect(() => {
    if (!isOpen) return;
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      } else if (e.key === "ArrowRight") {
        onSelect(Math.min(activeIndex + 1, photos.length - 1));
      } else if (e.key === "ArrowLeft") {
        onSelect(Math.max(activeIndex - 1, 0));
      }
    };
    window.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = prevOverflow;
      window.removeEventListener("keydown", onKey);
    };
  }, [isOpen, onClose, activeIndex, photos.length, onSelect]);

  if (!isOpen) return null;
  if (photos.length === 0) return null;

  // activeIndex は外部から与えられるが、photos.length 範囲内に clamp して
  // 表示 / counter / prev/next 判定に一貫して使う (out-of-range で UI が壊れない安全側)
  const clampedIndex = Math.max(0, Math.min(activeIndex, photos.length - 1));
  const photo = photos[clampedIndex];

  const onPointerDown = (e: ReactPointerEvent<HTMLDivElement>) => {
    const g = gestureRef.current;
    g.pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    e.currentTarget.setPointerCapture(e.pointerId);

    if (g.pointers.size === 2) {
      // pinch start
      const [a, b] = Array.from(g.pointers.values());
      g.initialDistance = distance(a, b);
      g.initialScale = scale;
      g.initialTx = tx;
      g.initialTy = ty;
    } else if (g.pointers.size === 1) {
      // pan start (only effective when scale > 1)
      g.panStartX = e.clientX - tx;
      g.panStartY = e.clientY - ty;
    }
  };

  const onPointerMove = (e: ReactPointerEvent<HTMLDivElement>) => {
    const g = gestureRef.current;
    if (!g.pointers.has(e.pointerId)) return;
    g.pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });

    if (g.pointers.size === 2) {
      const [a, b] = Array.from(g.pointers.values());
      const newDist = distance(a, b);
      if (g.initialDistance > 0) {
        const ratio = newDist / g.initialDistance;
        const nextScale = Math.max(
          MIN_SCALE,
          Math.min(MAX_SCALE, g.initialScale * ratio),
        );
        setScale(nextScale);
      }
    } else if (g.pointers.size === 1 && scale > 1) {
      // pan: 1 pointer + 拡大時のみ
      const nextTx = e.clientX - g.panStartX;
      const nextTy = e.clientY - g.panStartY;
      setTx(nextTx);
      setTy(nextTy);
    }
  };

  const onPointerUp = (e: ReactPointerEvent<HTMLDivElement>) => {
    const g = gestureRef.current;
    const pos = g.pointers.get(e.pointerId);
    g.pointers.delete(e.pointerId);

    // double tap: 1 pointer up + 直前の up から 300ms 以内 + 同じ位置
    if (pos !== undefined && g.pointers.size === 0) {
      const now = Date.now();
      const dt = now - g.lastTapTime;
      const moved = Math.hypot(pos.x - g.lastTapX, pos.y - g.lastTapY);
      if (dt < DOUBLE_TAP_MS && moved < DOUBLE_TAP_DISTANCE_PX) {
        // double-tap detected
        if (scale > 1) {
          setScale(1);
          setTx(0);
          setTy(0);
        } else {
          setScale(2);
        }
        g.lastTapTime = 0;
      } else {
        g.lastTapTime = now;
        g.lastTapX = pos.x;
        g.lastTapY = pos.y;
      }
    }

    // 拡大解除なら位置もリセット (scale <= 1 で位置オフセットを残さない)
    if (scale <= 1 && (tx !== 0 || ty !== 0)) {
      setTx(0);
      setTy(0);
    }
  };

  const onBackdropClick = (e: React.MouseEvent<HTMLDivElement>) => {
    // image 領域の click は stopPropagation されるので、ここに来るのは backdrop 部
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="全画面ビューア"
      data-testid="lightbox"
      className="fixed inset-0 z-50 flex flex-col bg-black/95"
      onClick={onBackdropClick}
    >
      {/* 上部バー: × close */}
      <div className="flex items-center justify-between px-4 py-3 sm:px-6">
        <span className="font-num text-xs text-white/70 sm:text-sm">
          {clampedIndex + 1} / {photos.length}
        </span>
        <button
          type="button"
          aria-label="閉じる"
          data-testid="lightbox-close"
          onClick={onClose}
          className="grid h-10 w-10 place-items-center rounded-full bg-white/10 text-white transition-colors hover:bg-white/20"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
          >
            <path d="M6 6l12 12 M18 6L6 18" />
          </svg>
        </button>
      </div>

      {/* 画像領域 */}
      <div
        data-testid="lightbox-image-area"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        className="relative flex flex-1 items-center justify-center overflow-hidden touch-none select-none"
        onClick={(e) => e.stopPropagation()}
      >
        <img
          src={photo.variants.display.url}
          alt={photo.caption ?? ""}
          width={photo.variants.display.width}
          height={photo.variants.display.height}
          loading="eager"
          decoding="async"
          draggable={false}
          style={{
            transform: `translate3d(${tx}px, ${ty}px, 0) scale(${scale})`,
            transition:
              gestureRef.current.pointers.size === 0
                ? "transform 0.18s ease-out"
                : "none",
            transformOrigin: "center center",
          }}
          className="max-h-full max-w-full object-contain"
        />
      </div>

      {/* 左右ナビ (PC) */}
      {photos.length > 1 ? (
        <>
          <button
            type="button"
            aria-label="前の写真"
            data-testid="lightbox-prev"
            disabled={clampedIndex === 0}
            onClick={(e) => {
              e.stopPropagation();
              onSelect(Math.max(0, clampedIndex - 1));
            }}
            className="absolute left-2 top-1/2 hidden h-12 w-12 -translate-y-1/2 place-items-center rounded-full bg-white/10 text-white transition-colors hover:bg-white/20 disabled:opacity-30 sm:grid"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <path d="M15 18l-6-6 6-6" />
            </svg>
          </button>
          <button
            type="button"
            aria-label="次の写真"
            data-testid="lightbox-next"
            disabled={clampedIndex === photos.length - 1}
            onClick={(e) => {
              e.stopPropagation();
              onSelect(Math.min(photos.length - 1, clampedIndex + 1));
            }}
            className="absolute right-2 top-1/2 hidden h-12 w-12 -translate-y-1/2 place-items-center rounded-full bg-white/10 text-white transition-colors hover:bg-white/20 disabled:opacity-30 sm:grid"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <path d="M9 18l6-6-6-6" />
            </svg>
          </button>
        </>
      ) : null}

      {/* サムネイル strip */}
      {photos.length > 1 ? (
        <div
          data-testid="lightbox-thumbs"
          className="flex w-full gap-2 overflow-x-auto px-3 py-3 sm:px-6 sm:py-4"
          onClick={(e) => e.stopPropagation()}
        >
          {photos.map((p, i) => (
            <button
              key={i}
              type="button"
              aria-label={`写真 ${i + 1} に移動`}
              data-testid={`lightbox-thumb-${i}`}
              onClick={() => onSelect(i)}
              className={`relative h-14 w-14 shrink-0 overflow-hidden rounded-md border-2 transition-all sm:h-16 sm:w-16 ${
                i === clampedIndex
                  ? "border-white"
                  : "border-transparent opacity-60 hover:opacity-100"
              }`}
            >
              <img
                src={p.variants.thumbnail.url}
                alt=""
                width={p.variants.thumbnail.width}
                height={p.variants.thumbnail.height}
                loading="lazy"
                decoding="async"
                draggable={false}
                className="h-full w-full object-cover"
              />
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
