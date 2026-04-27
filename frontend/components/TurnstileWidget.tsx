// Cloudflare Turnstile widget の自前 React component。
//
// 設計参照:
//   - docs/plan/m2-frontend-upload-ui-plan.md §4
//
// セキュリティ:
//   - Turnstile token は onVerify callback で親に渡すのみ。logs / URL に出さない。
//   - widget 失敗時の error code は表示しない（固定文言で UI を返す）。
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
          "error-callback"?: () => void;
          "expired-callback"?: () => void;
          theme?: "light" | "dark" | "auto";
          appearance?: "always" | "execute" | "interaction-only";
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
  onError?: () => void;
  onExpired?: () => void;
};

const TURNSTILE_SCRIPT_URL = "https://challenges.cloudflare.com/turnstile/v0/api.js";

/** Cloudflare Turnstile widget。official script を動的読込み、widget を render する。 */
export function TurnstileWidget({
  sitekey,
  action = "upload",
  onVerify,
  onError,
  onExpired,
}: TurnstileWidgetProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const widgetIdRef = useRef<string | null>(null);
  const [scriptLoaded, setScriptLoaded] = useState(false);

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

  // script 読込み後に widget を render
  useEffect(() => {
    if (!scriptLoaded) return;
    if (!containerRef.current) return;
    if (!window.turnstile) return;
    if (widgetIdRef.current !== null) return; // 二重 render 防止

    const id = window.turnstile.render(containerRef.current, {
      sitekey,
      action,
      callback: (token: string) => {
        onVerify(token);
      },
      "error-callback": () => {
        onError?.();
      },
      "expired-callback": () => {
        onExpired?.();
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
  }, [scriptLoaded, sitekey, action, onVerify, onError, onExpired]);

  return <div ref={containerRef} aria-label="Turnstile widget" />;
}
