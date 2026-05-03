// ManageUrlSavePanel: 管理 URL を Provider 不要で保存するための導線。
//
// 設計参照:
//   - docs/plan/m2-email-provider-reselection-plan.md §7（採用候補 A）
//   - docs/adr/0006-email-provider-and-manage-url-delivery.md（メール送信なし MVP）
//
// 導線:
//   1. .txt ダウンロード（Blob、ファイル名は slug 由来 + sanitize）
//   2. mailto: で自分宛にメール作成（ブラウザの Mail App 起動、サーバー送信しない）
//   3. 「保存しました」確認チェックボックス（チェック前は閉じる導線を弱くする）
//
// セキュリティ:
//   - URL を console.log に出さない
//   - localStorage / IndexedDB / Service Worker に保存しない
//   - Blob URL は triggerTxtDownload 内で即 revoke
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - design `wf-screens-b.jsx:226-233` (M) / `:266-269` (PC) `保存方法 panel` 視覚整合
//   - design `wf-check` (`wireframe-styles.css:315-334`) で「保存しました」chk を表現
//   - design `wf-btn` (`wireframe-styles.css:228-251`) で .txt / mailto button
//   - .txt download / mailto handler / saved chk state / data-testid (manage-url-save-panel /
//     manage-url-save-txt / manage-url-save-mailto / manage-url-saved-check / manage-url-saved-checkbox)
//     は **触らない**
"use client";

import { useState } from "react";

import {
  buildMailtoHref,
  buildManageUrlTxtContent,
  buildManageUrlTxtFileName,
  triggerTxtDownload,
} from "@/lib/manageUrlSave";

type Props = {
  manageURL: string;
  /** ファイル名に使う slug（無い場合は default 名）。 */
  slug?: string;
  /** 保存確認チェックの状態と切り替えハンドラ（CompleteView から制御）。 */
  saved: boolean;
  onSavedChange: (next: boolean) => void;
};

export function ManageUrlSavePanel({ manageURL, slug, saved, onSavedChange }: Props) {
  const [downloadStatus, setDownloadStatus] = useState<"idle" | "ok" | "fail">("idle");

  const handleDownload = () => {
    try {
      const filename = buildManageUrlTxtFileName(slug);
      const content = buildManageUrlTxtContent(manageURL);
      triggerTxtDownload(filename, content);
      setDownloadStatus("ok");
    } catch {
      setDownloadStatus("fail");
    } finally {
      window.setTimeout(() => setDownloadStatus("idle"), 3_000);
    }
  };

  const mailtoHref = buildMailtoHref(manageURL);

  return (
    <div
      className="space-y-3 rounded-lg border border-divider-soft bg-surface p-4 shadow-sm"
      data-testid="manage-url-save-panel"
    >
      <h3 className="flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
        <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
        管理 URL の保存方法
      </h3>
      <p className="text-xs leading-[1.6] text-ink-medium">
        現在メール送信は提供していません。下のいずれかの方法で必ずお手元に保存してください。
      </p>

      <div className="grid gap-2 sm:grid-cols-2">
        <button
          type="button"
          onClick={handleDownload}
          className="inline-flex h-10 items-center justify-center rounded-md border border-divider bg-surface px-3 text-xs font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700"
          data-testid="manage-url-save-txt"
        >
          {downloadStatus === "ok"
            ? ".txt をダウンロードしました"
            : downloadStatus === "fail"
              ? ".txt 保存に失敗"
              : ".txt ファイルとして保存"}
        </button>
        <a
          href={mailtoHref}
          rel="noopener noreferrer"
          className="inline-flex h-10 items-center justify-center rounded-md border border-divider bg-surface px-3 text-center text-xs font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700"
          data-testid="manage-url-save-mailto"
        >
          自分宛にメールを書く
        </a>
      </div>

      <p className="text-[11px] leading-[1.5] text-ink-soft">
        ※ メールは「送信」ボタンを押すまでサーバー経由で送られません。お使いのメールアプリ
        が起動しない場合はコピー / .txt 保存をご利用ください。
      </p>

      {/* design wf-check: rounded box (18x18) + ✓ icon (`wireframe-styles.css:315-334`) */}
      <label
        className="flex cursor-pointer items-start gap-2.5 rounded-md border border-divider-soft bg-surface-soft px-3 py-2.5 text-xs leading-[1.55] text-ink-strong"
        data-testid="manage-url-saved-check"
      >
        <input
          type="checkbox"
          checked={saved}
          onChange={(e) => onSavedChange(e.currentTarget.checked)}
          className="mt-0.5 h-4 w-4 shrink-0 accent-teal-500"
          data-testid="manage-url-saved-checkbox"
        />
        <span>
          管理用 URL を安全な場所に保存しました（紛失すると再表示できません）。
        </span>
      </label>
    </div>
  );
}
