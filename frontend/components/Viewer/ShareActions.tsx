"use client";

// ShareActions: X 共有 + URL コピー の Client Component。
//
// 採用元: TESTImage 完成イメージ「Share / X で共有する / URL をコピー」セクション
//
// 設計判断 (v2):
//   - navigator.share が使える環境 (iPhone Safari) ではネイティブシートを優先
//   - clipboard 経由は navigator.clipboard.writeText、失敗時は execCommand fallback なし
//     (Safari / Chrome 共に MVP 範囲では成功する想定、失敗時はトーストで通知)
//   - X (Twitter) intent URL は web intent (https://x.com/intent/tweet?text=...&url=...)
//
// セキュリティ:
//   - shareUrl は public viewer の URL のみ。raw photobook_id 含まない
//   - clipboard 書き込みは user gesture 内のみ (button click)

import { useCallback, useState } from "react";

type Props = {
  shareUrl: string;
  shareText: string;
};

type CopyState = "idle" | "copied" | "failed";

export function ShareActions({ shareUrl, shareText }: Props) {
  const [copyState, setCopyState] = useState<CopyState>("idle");

  const onShare = useCallback(async () => {
    // navigator.share があれば優先 (iPhone Safari でネイティブシート)
    if (typeof navigator !== "undefined" && typeof navigator.share === "function") {
      try {
        await navigator.share({ title: shareText, text: shareText, url: shareUrl });
        return;
      } catch {
        // user キャンセル含む。X intent fallback に移る
      }
    }
    // fallback: X (Twitter) intent
    const intent = `https://x.com/intent/tweet?text=${encodeURIComponent(shareText)}&url=${encodeURIComponent(shareUrl)}`;
    window.open(intent, "_blank", "noopener,noreferrer");
  }, [shareUrl, shareText]);

  const onCopy = useCallback(async () => {
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      try {
        await navigator.clipboard.writeText(shareUrl);
        setCopyState("copied");
        setTimeout(() => setCopyState("idle"), 2000);
        return;
      } catch {
        setCopyState("failed");
        setTimeout(() => setCopyState("idle"), 2000);
        return;
      }
    }
    setCopyState("failed");
    setTimeout(() => setCopyState("idle"), 2000);
  }, [shareUrl]);

  return (
    <div data-testid="viewer-share-actions" className="flex flex-col gap-2 sm:flex-row sm:gap-3">
      <button
        type="button"
        onClick={onShare}
        data-testid="viewer-share-x"
        className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-[10px] bg-ink px-5 text-sm font-bold text-white shadow-sm transition-colors hover:bg-ink-strong sm:w-auto sm:min-w-[160px]"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
          <path d="M18.244 2H21l-6.46 7.39L22 22h-6.91l-4.59-5.99L4.94 22H2.18l6.93-7.93L2 2h7.06l4.13 5.46L18.244 2zm-2.43 18h1.85L7.32 4h-1.9l10.39 16z" />
        </svg>
        X で共有する
      </button>
      <button
        type="button"
        onClick={onCopy}
        data-testid="viewer-share-copy"
        className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-[10px] border border-divider bg-surface px-5 text-sm font-bold text-ink-strong shadow-sm transition-colors hover:border-teal-300 hover:text-teal-700 sm:w-auto sm:min-w-[160px]"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <rect x="9" y="9" width="13" height="13" rx="2" />
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
        </svg>
        {copyState === "copied" ? "コピーしました" : copyState === "failed" ? "コピー失敗" : "URL をコピー"}
      </button>
    </div>
  );
}
