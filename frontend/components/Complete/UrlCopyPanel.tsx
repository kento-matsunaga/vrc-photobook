// UrlCopyPanel: 公開直後の URL コピーパネル。
//
// セキュリティ:
//   - URL 値はユーザーに表示する必要があるため画面に出すが、console.log は避ける
//   - 失敗時に値を含むエラー文を出さない
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - design `wf-screens-b.jsx:204-211` (M 公開 URL) / `:217-223` (M 管理 URL dashed) /
//     `:255-271` (PC wf-grid-2 で公開 URL + 管理 URL) 視覚整合
//   - URL 表示部を wf-input 風 (rounded-md + border-divider + monospace + 13px)
//   - copy button を wf-btn sm 風 (rounded-md + border-divider + hover teal)
//   - `kind="public"` は teal tone、`kind="manage"` は dashed border (再表示不可の視覚強調)
//   - testId / handleCopy / clipboard logic は **触らない**
"use client";

import { useState } from "react";

type Props = {
  label: string;
  url: string;
  /** "public" は teal、"manage" は dashed border + violet */
  kind: "public" | "manage";
  /** 任意の補足説明（例: 再表示できません） */
  helper?: string;
  testId?: string;
};

export function UrlCopyPanel({ label, url, kind, helper, testId }: Props) {
  const [copied, setCopied] = useState<"idle" | "ok" | "fail">("idle");
  const containerCls =
    kind === "public"
      ? "border border-teal-100 bg-teal-50"
      : "border-2 border-dashed border-divider bg-surface";
  // m2-design-refresh STOP β-6 cleanup (F-01): manage kind label を violet 旧 palette から
  // ink-strong に揃える。design refresh 後の manage URL 識別は dashed border (container 側) で
  // 担保、color 区別は不要。
  const labelCls = kind === "public" ? "text-teal-700" : "text-ink-strong";

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
      className={`rounded-lg px-4 py-3 ${containerCls}`}
      data-testid={testId}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-1.5">
          <div className={`text-xs font-bold ${labelCls}`}>{label}</div>
          <div className="break-all rounded-md border border-divider-soft bg-surface px-3 py-2 font-num text-[12px] text-ink-strong">
            {url}
          </div>
          {helper && <div className="text-[11px] leading-[1.5] text-ink-medium">{helper}</div>}
        </div>
        <button
          type="button"
          onClick={() => void handleCopy()}
          className="inline-flex h-9 shrink-0 items-center rounded-md border border-divider bg-surface px-3 text-xs font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700"
          aria-label={`${label} をコピー`}
          data-testid={testId ? `${testId}-copy` : undefined}
        >
          {copied === "ok" ? "コピーしました" : copied === "fail" ? "失敗" : "コピー"}
        </button>
      </div>
    </div>
  );
}
