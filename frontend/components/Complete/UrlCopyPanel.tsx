// UrlCopyPanel: 公開直後の URL コピーパネル。
//
// design 参照: design/mockups/prototype/screens-a.jsx の Complete / shared.jsx UrlRow
//
// セキュリティ:
//   - URL 値はユーザーに表示する必要があるため画面に出すが、console.log は避ける
//   - 失敗時に値を含むエラー文を出さない
"use client";

import { useState } from "react";

type Props = {
  label: string;
  url: string;
  /** "public" は teal、"manage" は violet */
  kind: "public" | "manage";
  /** 任意の補足説明（例: 再表示できません） */
  helper?: string;
  testId?: string;
};

export function UrlCopyPanel({ label, url, kind, helper, testId }: Props) {
  const [copied, setCopied] = useState<"idle" | "ok" | "fail">("idle");
  const tone =
    kind === "public" ? "text-brand-teal bg-brand-teal-soft" : "text-brand-violet bg-purple-50";
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied("ok");
    } catch {
      setCopied("fail");
    } finally {
      setTimeout(() => setCopied("idle"), 3_000);
    }
  };
  return (
    <div
      className={`rounded-lg border border-divider px-4 py-3 ${tone}`}
      data-testid={testId}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-1">
          <div className="text-xs font-medium">{label}</div>
          <div className="break-all font-num text-sm text-ink-strong">{url}</div>
          {helper && <div className="text-xs text-ink-medium">{helper}</div>}
        </div>
        <button
          type="button"
          onClick={() => void handleCopy()}
          className="shrink-0 rounded-md border border-divider bg-surface px-3 py-1 text-xs text-ink-medium hover:bg-surface-soft"
          aria-label={`${label} をコピー`}
          data-testid={testId ? `${testId}-copy` : undefined}
        >
          {copied === "ok" ? "コピーしました" : copied === "fail" ? "失敗" : "コピー"}
        </button>
      </div>
    </div>
  );
}
