"use client";

// ShareActions: X 共有 / URL コピーボタン。
//
// デザイン参照:
//   - design 最終調整版 §1 Mobile 「シェア・アクション」
//   - design 最終調整版 §2 PC 右サイドバー Share セクション
//
// セキュリティ:
//   - shareUrl は public URL（slug ベース）のみ。photobook_id を含めない
//   - clipboard 失敗時に値を console.log しない
//   - シェア文に作成者の自由テキストを含めるが、X 共有は intent URL 経由のため XSS リスクなし

import { useCallback, useState } from "react";

type Props = {
  /** 公開 URL（https://app.vrc-photobook.com/p/{slug}） */
  shareUrl: string;
  /** X 共有用の本文（タイトル + creator など） */
  shareText: string;
};

export function ShareActions({ shareUrl, shareText }: Props) {
  const [copied, setCopied] = useState(false);

  const onShareX = useCallback(() => {
    const u = new URL("https://twitter.com/intent/tweet");
    u.searchParams.set("text", shareText);
    u.searchParams.set("url", shareUrl);
    window.open(u.toString(), "_blank", "noopener,noreferrer");
  }, [shareText, shareUrl]);

  const onCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch {
      // 失敗時もユーザに具体的な値を出さない
      setCopied(false);
    }
  }, [shareUrl]);

  return (
    <div className="space-y-2" data-testid="share-actions">
      <button
        type="button"
        onClick={onShareX}
        className="flex w-full items-center justify-center gap-2 rounded-lg bg-ink px-4 py-2.5 text-sm font-bold text-white transition-colors hover:bg-ink-strong"
        data-testid="share-action-x"
      >
        <span aria-hidden="true" className="font-num text-base">𝕏</span>
        <span>で共有する</span>
      </button>
      <button
        type="button"
        onClick={onCopy}
        className="flex w-full items-center justify-center gap-2 rounded-lg border border-divider bg-surface px-4 py-2.5 text-sm font-semibold text-ink transition-colors hover:border-teal-300 hover:text-teal-700"
        data-testid="share-action-copy"
        aria-live="polite"
      >
        <span aria-hidden="true">🔗</span>
        <span>{copied ? "コピーしました" : "URL をコピー"}</span>
      </button>
    </div>
  );
}
