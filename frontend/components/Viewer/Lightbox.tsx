"use client";

// Lightbox: 全画面ビューア (MVP)。
//
// デザイン参照:
//   - design 最終調整版 §3 Lightbox MVP スコープ (Phase 1)
//
// MVP スコープ（5 機能）:
//   1. 写真タップで全画面表示
//   2. ピンチで拡大・縮小（pointer events で実装）
//   3. ダブルタップで等倍に戻る
//   4. ドラッグで移動（拡大時のみ）
//   5. サムネイルで素早く移動
//   + 補助: X タップ / 背景タップ / Escape キーで閉じる
//
// Phase 2 以降:
//   - 自動再生（スライドショー）
//   - 100% 表示のトグル
//   - フルスクリーン API
//   - 画像ダウンロード（業務方針で MVP 削除）
//
// セキュリティ:
//   - presigned URL を console.log しない
//   - photoIndex / カウンタ表示は OK（raw ID 露出にはあたらない）

import { useCallback, useEffect, useRef, useState } from "react";

import type { PublicPhoto } from "@/lib/publicPhotobook";

type Props = {
  photos: PublicPhoto[];
  initialIndex: number;
  onClose: () => void;
};

const MIN_SCALE = 1;
const MAX_SCALE = 4;
const DOUBLE_TAP_THRESHOLD_MS = 280;

export function Lightbox({ photos, initialIndex, onClose }: Props) {
  const [index, setIndex] = useState(
    Math.min(Math.max(0, initialIndex), photos.length - 1),
  );
  const [scale, setScale] = useState(1);
  const [translate, setTranslate] = useState({ x: 0, y: 0 });

  const lastTapRef = useRef<number>(0);
  const dragStartRef = useRef<{ x: number; y: number } | null>(null);
  const pinchStartRef = useRef<{ dist: number; scale: number } | null>(null);
  const pointersRef = useRef<Map<number, { x: number; y: number }>>(new Map());

  const photo = photos[index];

  const reset = useCallback(() => {
    setScale(1);
    setTranslate({ x: 0, y: 0 });
  }, []);

  const goTo = useCallback(
    (next: number) => {
      if (next < 0 || next >= photos.length) return;
      setIndex(next);
      reset();
    },
    [photos.length, reset],
  );

  // ESC で閉じる
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
      } else if (e.key === "ArrowRight") {
        goTo(index + 1);
      } else if (e.key === "ArrowLeft") {
        goTo(index - 1);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [index, goTo, onClose]);

  // pointer events での pinch / drag handling
  const onPointerDown = (e: React.PointerEvent) => {
    pointersRef.current.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (pointersRef.current.size === 1) {
      dragStartRef.current = {
        x: e.clientX - translate.x,
        y: e.clientY - translate.y,
      };
    } else if (pointersRef.current.size === 2) {
      const pts = Array.from(pointersRef.current.values());
      const dist = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
      pinchStartRef.current = { dist, scale };
      dragStartRef.current = null;
    }
  };

  const onPointerMove = (e: React.PointerEvent) => {
    if (!pointersRef.current.has(e.pointerId)) return;
    pointersRef.current.set(e.pointerId, { x: e.clientX, y: e.clientY });

    if (pointersRef.current.size === 2 && pinchStartRef.current) {
      const pts = Array.from(pointersRef.current.values());
      const dist = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
      const ratio = dist / pinchStartRef.current.dist;
      const next = clamp(
        pinchStartRef.current.scale * ratio,
        MIN_SCALE,
        MAX_SCALE,
      );
      setScale(next);
      if (next === 1) {
        setTranslate({ x: 0, y: 0 });
      }
    } else if (pointersRef.current.size === 1 && dragStartRef.current && scale > 1) {
      setTranslate({
        x: e.clientX - dragStartRef.current.x,
        y: e.clientY - dragStartRef.current.y,
      });
    }
  };

  const onPointerUp = (e: React.PointerEvent) => {
    pointersRef.current.delete(e.pointerId);
    if (pointersRef.current.size < 2) pinchStartRef.current = null;
    if (pointersRef.current.size === 0) dragStartRef.current = null;
  };

  // ダブルタップで等倍に戻す
  const onTap = () => {
    const now = Date.now();
    if (now - lastTapRef.current < DOUBLE_TAP_THRESHOLD_MS) {
      reset();
      lastTapRef.current = 0;
      return;
    }
    lastTapRef.current = now;
  };

  if (!photo) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex flex-col bg-black/95 text-white"
      role="dialog"
      aria-modal="true"
      aria-label="写真を全画面で表示"
      data-testid="lightbox"
      onClick={(e) => {
        // 背景クリックで閉じる（写真本体クリックは伝播停止）
        if (e.target === e.currentTarget) onClose();
      }}
    >
      {/* header */}
      <div className="flex items-center justify-between px-4 py-3 text-sm">
        <span className="font-num" data-testid="lightbox-counter">
          {index + 1} / {photos.length}
        </span>
        <button
          type="button"
          onClick={onClose}
          aria-label="閉じる"
          data-testid="lightbox-close"
          className="grid h-9 w-9 place-items-center rounded-full bg-white/10 transition-colors hover:bg-white/20"
        >
          <span aria-hidden="true">×</span>
        </button>
      </div>

      {/* photo area */}
      <div
        className="relative flex-1 touch-none select-none overflow-hidden"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        onClick={onTap}
        style={{ touchAction: "none" }}
      >
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={photo.variants.display.url}
          alt=""
          width={photo.variants.display.width}
          height={photo.variants.display.height}
          decoding="async"
          draggable={false}
          className="absolute inset-0 m-auto h-auto max-h-full w-auto max-w-full"
          style={{
            transform: `translate(${translate.x}px, ${translate.y}px) scale(${scale})`,
            transformOrigin: "center center",
            transition: pointersRef.current.size === 0 ? "transform 0.15s ease-out" : "none",
          }}
        />

        {/* 左右ナビゲーション（PC） */}
        {index > 0 && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              goTo(index - 1);
            }}
            aria-label="前の写真"
            className="absolute left-2 top-1/2 hidden -translate-y-1/2 rounded-full bg-white/10 p-3 text-2xl transition-colors hover:bg-white/20 sm:block"
          >
            <span aria-hidden="true">‹</span>
          </button>
        )}
        {index < photos.length - 1 && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              goTo(index + 1);
            }}
            aria-label="次の写真"
            className="absolute right-2 top-1/2 hidden -translate-y-1/2 rounded-full bg-white/10 p-3 text-2xl transition-colors hover:bg-white/20 sm:block"
          >
            <span aria-hidden="true">›</span>
          </button>
        )}
      </div>

      {/* thumbnail strip */}
      <div
        className="overflow-x-auto border-t border-white/10 bg-black/40 px-3 py-2"
        data-testid="lightbox-thumbs"
      >
        <div className="flex gap-2">
          {photos.map((p, i) => (
            <button
              key={i}
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                goTo(i);
              }}
              aria-label={`${i + 1} 枚目に移動`}
              aria-current={i === index ? "true" : undefined}
              className={`shrink-0 overflow-hidden rounded-md border-2 transition-colors ${
                i === index
                  ? "border-white"
                  : "border-white/20 hover:border-white/60"
              }`}
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={p.variants.thumbnail.url}
                alt=""
                width={p.variants.thumbnail.width}
                height={p.variants.thumbnail.height}
                loading="lazy"
                decoding="async"
                className="block h-14 w-auto"
              />
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

function clamp(v: number, lo: number, hi: number): number {
  return Math.min(Math.max(v, lo), hi);
}
