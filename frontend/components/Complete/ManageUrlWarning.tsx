// ManageUrlWarning: manage URL は再表示できないことを強調する。
//
// 業務知識 v4 §6: manage URL は発行直後にユーザーに表示し、再表示しない。
// メール再発行は ADR-0006 で provider 再選定中。MVP は本警告 + Complete 画面の
// 保存導線（ManageUrlSavePanel）でユーザー側に保管してもらう運用。
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
        <li>下の保存メニューから .txt 保存 / 自分宛メール / コピーのいずれかで必ず保管してください。</li>
        <li>
          紛失すると、編集や公開停止ができなくなります（メール再送は現在提供していません、
          <a
            href="/help/manage-url"
            className="underline"
            data-testid="manage-url-faq-link"
          >
            よくある質問
          </a>
          ）。
        </li>
      </ul>
    </div>
  );
}
