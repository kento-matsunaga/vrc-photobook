// Cloudflare Turnstile widget の自前 React component。
//
// 設計参照:
//   - docs/plan/m2-frontend-upload-ui-plan.md §4
//   - .agents/rules/turnstile-defensive-guard.md
//   - harness/failure-log/2026-04-29_turnstile-widget-remount-loop.md
//
// 実装方針:
//   - callback prop は useRef で保持し、useEffect 依存配列に関数参照を入れない。
//     親 component が re-render しても widget が remove → re-render される無限ループ
//     を防ぐ（widget の verification challenge がリセットされないようにする）。
//   - useEffect 依存は安定値のみ（scriptLoaded / sitekey / action）。
//   - error / expired / timeout の各 callback を実装し、token を空にする / 上位へ通知する。
//   - error code は token を出さずに console.warn で見える化（debug 用）。
//
// セキュリティ:
//   - Turnstile token は onVerify callback で親に渡すのみ。logs / URL に出さない。
//   - widget 失敗時の error code は console には出すが、画面 / state / token とは混ぜない。
"use client";

import { useEffect, useRef, useState } from "react";

declare global {
  interface Window {
    turnstile?: {
      render(
        container: HTMLElement | string,
        options: {
          sitekey: string;
          action?: string;
          callback?: (token: string) => void;
          "error-callback"?: (code?: string) => void;
          "expired-callback"?: () => void;
          "timeout-callback"?: () => void;
          theme?: "light" | "dark" | "auto";
          appearance?: "always" | "execute" | "interaction-only";
          "refresh-expired"?: "auto" | "manual" | "never";
          "refresh-timeout"?: "auto" | "manual" | "never";
          retry?: "auto" | "never";
          "retry-interval"?: number;
        },
      ): string;
      reset(widgetId?: string): void;
      remove(widgetId?: string): void;
    };
    onloadTurnstileCallback?: () => void;
  }
}

type TurnstileWidgetProps = {
  sitekey: string;
  action?: string;
  onVerify: (token: string) => void;
  onError?: (code?: string) => void;
  onExpired?: () => void;
  onTimeout?: () => void;
};

const TURNSTILE_SCRIPT_URL = "https://challenges.cloudflare.com/turnstile/v0/api.js";

/** Cloudflare Turnstile widget。official script を動的読込み、widget を render する。
 *
 * 親 component が頻繁に re-render しても widget が再 mount されないよう、callback prop
 * は useRef で保持して useEffect 依存配列に入れない。
 */
export function TurnstileWidget({
  sitekey,
  action = "upload",
  onVerify,
  onError,
  onExpired,
  onTimeout,
}: TurnstileWidgetProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const widgetIdRef = useRef<string | null>(null);
  const [scriptLoaded, setScriptLoaded] = useState(false);

  // callback の最新参照を ref に保持。useEffect 依存に入れずに最新版を呼ぶ。
  const onVerifyRef = useRef(onVerify);
  const onErrorRef = useRef(onError);
  const onExpiredRef = useRef(onExpired);
  const onTimeoutRef = useRef(onTimeout);
  useEffect(() => {
    onVerifyRef.current = onVerify;
    onErrorRef.current = onError;
    onExpiredRef.current = onExpired;
    onTimeoutRef.current = onTimeout;
  }, [onVerify, onError, onExpired, onTimeout]);

  // script を 1 回だけ読み込む
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (window.turnstile) {
      setScriptLoaded(true);
      return;
    }
    const existing = document.querySelector<HTMLScriptElement>(
      `script[src="${TURNSTILE_SCRIPT_URL}"]`,
    );
    if (existing) {
      existing.addEventListener("load", () => setScriptLoaded(true));
      return;
    }
    const s = document.createElement("script");
    s.src = TURNSTILE_SCRIPT_URL;
    s.async = true;
    s.defer = true;
    s.onload = () => setScriptLoaded(true);
    document.head.appendChild(s);
  }, []);

  // script 読込み後に widget を render。callback 参照が変わっても再 mount しない。
  useEffect(() => {
    if (!scriptLoaded) return;
    if (!containerRef.current) return;
    if (!window.turnstile) return;
    if (widgetIdRef.current !== null) return; // 二重 render 防止

    const id = window.turnstile.render(containerRef.current, {
      sitekey,
      action,
      callback: (token: string) => {
        onVerifyRef.current(token);
      },
      "error-callback": (code?: string) => {
        // token は出さない。error code のみ debug 出力（運用調査用）。
        if (typeof console !== "undefined") {
          console.warn("turnstile error-callback", { code: code ?? "(none)" });
        }
        onErrorRef.current?.(code);
      },
      "expired-callback": () => {
        onExpiredRef.current?.();
      },
      "timeout-callback": () => {
        if (typeof console !== "undefined") {
          console.warn("turnstile timeout-callback");
        }
        onTimeoutRef.current?.();
      },
      theme: "light",
    });
    widgetIdRef.current = id;

    return () => {
      if (widgetIdRef.current && window.turnstile) {
        try {
          window.turnstile.remove(widgetIdRef.current);
        } catch {
          /* noop */
        }
        widgetIdRef.current = null;
      }
    };
    // 意図的に callback prop を依存に入れない。useRef で最新版を取得する。
    // sitekey / action / scriptLoaded のみ widget の同一性を決める要素として依存。
  }, [scriptLoaded, sitekey, action]);

  return <div ref={containerRef} aria-label="Turnstile widget" />;
}
