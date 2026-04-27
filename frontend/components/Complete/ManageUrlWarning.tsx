// ManageUrlWarning: manage URL は再表示できないことを強調する。
//
// 業務知識 v4 §6: manage URL は発行直後にユーザーに表示し、再表示しない。
// 紛失した場合は PR32 で SendGrid 経由メール再送（PR28 では placeholder）。
"use client";

export function ManageUrlWarning() {
  return (
    <div
      className="rounded-md border border-status-error bg-status-error-soft px-4 py-3 text-sm"
      role="alert"
      data-testid="manage-url-warning"
    >
      <p className="font-medium text-status-error">⚠ 管理用 URL は今だけ表示されます</p>
      <ul className="mt-2 list-disc space-y-1 pl-5 text-ink-strong">
        <li>このページを離れると、管理用 URL は二度と表示できません。</li>
        <li>必ずスクリーンショットや安全な場所への保存を行ってから次へ進んでください。</li>
        <li>紛失した場合の再発行はメール送信機能（後日対応）が必要です。</li>
      </ul>
    </div>
  );
}
