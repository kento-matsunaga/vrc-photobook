// ManageUrlNoticeBanner: 管理ページ常設の info tone 注意喚起バナー (M-1a)。
//
// 設計参照:
//   - docs/plan/m-1-manage-mvp-safety-plan.md §3.3.1
//   - 業務知識 v4 §3.4 (「管理 URL は他人に共有してはならない」旨の注意喚起を表示する)
//
// 方針:
//   - warning ではなく info / note tone（怖くしすぎない）
//   - 共有禁止 / SNS 公開禁止 / 共有端末では session revoke を使う、を 1 まとめで提示
//   - manage_url_token の raw 値は出さない（既定）

type Props = {
  testId?: string;
};

export function ManageUrlNoticeBanner({ testId = "manage-url-notice-banner" }: Props) {
  return (
    <section
      role="note"
      aria-live="polite"
      data-testid={testId}
      className="flex items-start gap-2.5 rounded-lg border-l-[3px] border-teal-300 bg-teal-50 p-3.5"
    >
      <span
        aria-hidden="true"
        className="grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full bg-teal-500 font-serif text-xs font-bold italic leading-none text-white"
      >
        i
      </span>
      <div className="flex-1 text-xs leading-[1.7] text-ink-strong">
        <p className="font-bold">管理 URL の取り扱いについて</p>
        <ul className="mt-1.5 list-disc space-y-1 pl-5 marker:text-teal-600 text-ink-medium">
          <li>管理 URL は他人と共有しないでください。共有された相手も編集や削除ができてしまいます。</li>
          <li>SNS や公開チャットに貼らないでください。</li>
          <li>共有端末で開いた後は、下の「この端末の管理権限を削除」を使って Cookie を消してください。</li>
        </ul>
      </div>
    </section>
  );
}
