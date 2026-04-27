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
      className="space-y-3 rounded-lg border border-divider bg-surface px-4 py-3"
      data-testid="manage-url-save-panel"
    >
      <p className="text-sm font-medium text-ink-strong">管理 URL の保存方法</p>
      <p className="text-xs text-ink-medium">
        現在メール送信は提供していません。下のいずれかの方法で必ずお手元に保存してください。
      </p>

      <div className="grid gap-2 sm:grid-cols-2">
        <button
          type="button"
          onClick={handleDownload}
          className="rounded-md border border-divider bg-surface px-3 py-2 text-sm text-ink-strong hover:bg-surface-soft"
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
          className="rounded-md border border-divider bg-surface px-3 py-2 text-center text-sm text-ink-strong hover:bg-surface-soft"
          data-testid="manage-url-save-mailto"
        >
          自分宛にメールを書く
        </a>
      </div>

      <p className="text-xs text-ink-soft">
        ※ メールは「送信」ボタンを押すまでサーバー経由で送られません。お使いのメールアプリ
        が起動しない場合はコピー / .txt 保存をご利用ください。
      </p>

      <label
        className="flex cursor-pointer items-start gap-2 rounded-md border border-divider-soft bg-surface-soft px-3 py-2 text-sm text-ink-strong"
        data-testid="manage-url-saved-check"
      >
        <input
          type="checkbox"
          checked={saved}
          onChange={(e) => onSavedChange(e.currentTarget.checked)}
          className="mt-1 h-4 w-4"
          data-testid="manage-url-saved-checkbox"
        />
        <span>
          管理用 URL を安全な場所に保存しました（紛失すると再表示できません）。
        </span>
      </label>
    </div>
  );
}
