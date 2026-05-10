// ManageEditResumeButton: 「編集を再開」ボタン (M-1a 案 A)。
//
// 設計参照:
//   - docs/plan/m-1-manage-mvp-safety-plan.md §3.2.5
//   - 業務知識 v4 §3.4 (フォトブック内容編集の導線を提供する)
//
// 振る舞い:
//   - クリック → /manage/<id>/issue-draft (Workers Route Handler) を POST で呼ぶ
//   - 成功 → response の edit_url に window.location.replace で遷移
//   - 失敗 (not_draft) → メッセージ「現在 MVP では公開済みフォトブックの再編集はできません」
//   - 失敗 (unauthorized / network / server_error) → 個別文言
//
// セキュリティ:
//   - raw token / Cookie を画面に出さない / console に出さない
//   - 失敗時の reason は固定文言にマップ（敵対者観測抑止 + ユーザ向け説明性）
"use client";

import { useState } from "react";

import {
  isManageMutationError,
  issueDraftSessionFromManage,
  type ManageMutationError,
} from "@/lib/managePhotobook";

type Props = {
  photobookId: string;
};

const ERROR_LABEL: Record<ManageMutationError["kind"], string> = {
  unauthorized: "管理セッションが期限切れです。管理 URL から再度入場してください。",
  not_found: "対象のフォトブックが見つかりません。",
  version_conflict: "他の編集が反映されました。ページを再読込して再度お試しください。",
  public_change_not_allowed: "この操作はサポートされていません。",
  not_draft: "現在の MVP では、公開済みフォトブックの再編集はできません（公開停止機能の提供後に利用可能になります）。",
  invalid_payload: "リクエストが不正です。",
  server_error: "一時的なエラーが発生しました。しばらくしてから再度お試しください。",
  network: "通信に失敗しました。ネットワークをご確認のうえ再度お試しください。",
};

export function ManageEditResumeButton({ photobookId }: Props) {
  const [busy, setBusy] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const onClick = async () => {
    if (busy) return;
    setBusy(true);
    setErrorMsg(null);
    try {
      const out = await issueDraftSessionFromManage(photobookId);
      // 編集画面に置換（履歴を残さない、戻るで manage に戻れる）
      window.location.replace(out.editUrl);
    } catch (e) {
      if (isManageMutationError(e)) {
        setErrorMsg(ERROR_LABEL[e.kind] ?? ERROR_LABEL.server_error);
      } else {
        setErrorMsg(ERROR_LABEL.server_error);
      }
      setBusy(false);
    }
  };

  return (
    <div className="space-y-2" data-testid="manage-edit-resume-section">
      <button
        type="button"
        data-testid="manage-edit-resume-button"
        onClick={onClick}
        disabled={busy}
        className="inline-flex h-10 w-full items-center justify-center rounded-md border border-divider bg-surface px-4 text-sm font-semibold text-ink-strong shadow-sm transition-colors hover:border-teal-300 hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-60 sm:w-auto sm:min-w-[200px]"
      >
        {busy ? "処理中…" : "編集を再開"}
      </button>
      {errorMsg !== null && (
        <p
          role="alert"
          data-testid="manage-edit-resume-error"
          className="text-xs leading-[1.6] text-status-warning"
        >
          {errorMsg}
        </p>
      )}
    </div>
  );
}
