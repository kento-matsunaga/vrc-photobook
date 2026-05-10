// ManageSessionRevokeButton: 「この端末の管理権限を削除」ボタン (M-1a)。
//
// 設計参照:
//   - docs/plan/m-1-manage-mvp-safety-plan.md §3.2.4 / §3.3.1
//   - 業務知識 v4 §3.4 (この端末から管理権限を削除する明示破棄 UI)
//
// 振る舞い:
//   - クリック → 確認 dialog（簡易）
//   - 承認 → /manage/<id>/revoke-session (Workers Route Handler) を POST
//   - 成功 → / にリダイレクト（reason=session_revoked クエリ）
//   - 別端末からの再入場は阻害しない（manage_url は失効させない）
//
// セキュリティ:
//   - raw token / Cookie 値は出さない
"use client";

import { useState } from "react";

import {
  isManageMutationError,
  type ManageMutationError,
  revokeManageSession,
} from "@/lib/managePhotobook";

type Props = {
  photobookId: string;
};

const ERROR_LABEL: Record<ManageMutationError["kind"], string> = {
  unauthorized: "既にセッションが期限切れです。",
  not_found: "対象が見つかりません。",
  version_conflict: "他の編集が反映されました。ページを再読込してください。",
  public_change_not_allowed: "操作できません。",
  not_draft: "操作できません。",
  invalid_payload: "リクエストが不正です。",
  server_error: "一時的なエラーが発生しました。",
  network: "通信に失敗しました。",
};

export function ManageSessionRevokeButton({ photobookId }: Props) {
  const [busy, setBusy] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const onClick = async () => {
    if (busy) return;
    // 簡易 confirm（破壊系ではないが、誤クリック防止）
    const ok = typeof window !== "undefined"
      ? window.confirm(
          "この端末の管理権限を削除します。別端末からは引き続き管理 URL でアクセスできます。続行しますか？",
        )
      : true;
    if (!ok) return;
    setBusy(true);
    setErrorMsg(null);
    try {
      await revokeManageSession(photobookId);
      // 成功 → 公開トップに戻す（reason 付きで「権限を削除しました」表示余地）
      window.location.replace("/?reason=session_revoked");
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
    <div className="space-y-2" data-testid="manage-session-revoke-section">
      <button
        type="button"
        data-testid="manage-session-revoke-button"
        onClick={onClick}
        disabled={busy}
        className="inline-flex h-10 w-full items-center justify-center rounded-md border border-divider bg-surface px-4 text-xs font-semibold text-ink-strong shadow-sm transition-colors hover:border-status-warning hover:text-status-warning disabled:cursor-not-allowed disabled:opacity-60 sm:w-auto sm:min-w-[220px]"
      >
        {busy ? "処理中…" : "この端末の管理権限を削除"}
      </button>
      <p className="text-xs leading-[1.6] text-ink-medium">
        この端末の管理セッションのみを破棄します。管理 URL 自体は引き続き有効で、別端末からは再入場できます。
      </p>
      {errorMsg !== null && (
        <p
          role="alert"
          data-testid="manage-session-revoke-error"
          className="text-xs leading-[1.6] text-status-warning"
        >
          {errorMsg}
        </p>
      )}
    </div>
  );
}
