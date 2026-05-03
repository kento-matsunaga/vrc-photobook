// ManageUrlWarning: manage URL は再表示できないことを強調する。
//
// 業務知識 v4 §6: manage URL は発行直後にユーザーに表示し、再表示しない。
// メール再発行は ADR-0006 で provider 再選定中。MVP は本警告 + Complete 画面の
// 保存導線（ManageUrlSavePanel）でユーザー側に保管してもらう運用。
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - design `wf-screens-b.jsx:213-215` (M) / `:274-276` (PC) `wf-note.warn`
//     (`wireframe-styles.css:420-425`) 視覚整合: border-l warn + bg warn-soft + ! icon
//   - role="alert" / data-testid="manage-url-warning" / data-testid="manage-url-faq-link" 維持
//   - 文言・FAQ link は **触らない**
"use client";

export function ManageUrlWarning() {
  return (
    <div
      className="flex items-start gap-2.5 rounded-lg border-l-[3px] border-status-error bg-status-error-soft p-4 text-sm"
      role="alert"
      data-testid="manage-url-warning"
    >
      <span
        aria-hidden="true"
        className="grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full bg-status-error font-serif text-xs font-bold italic leading-none text-white"
      >
        !
      </span>
      <div className="flex-1">
        <p className="font-bold text-status-error">⚠ 管理用 URL は今だけ表示されます</p>
        <ul className="mt-2 list-disc space-y-1 pl-5 text-xs leading-[1.6] text-ink-strong">
          <li>このページを離れると、管理用 URL は二度と表示できません。</li>
          <li>下の保存メニューから .txt 保存 / 自分宛メール / コピーのいずれかで必ず保管してください。</li>
          <li>
            紛失すると、編集や公開停止ができなくなります（メール再送は現在提供していません、
            <a
              href="/help/manage-url"
              className="text-teal-600 underline hover:text-teal-700"
              data-testid="manage-url-faq-link"
            >
              よくある質問
            </a>
            ）。
          </li>
        </ul>
      </div>
    </div>
  );
}
