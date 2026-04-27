// CompleteView: publish 直後に表示する完了画面（PR28）。
//
// design 参照: design/mockups/prototype/screens-a.jsx の Complete / pc-screens-a.jsx PCComplete
//
// セキュリティ:
//   - manage URL は raw token を含むため、URL bar / log には出さない
//   - 本コンポーネントの propには manage_url_path を受けるが、URL 遷移しない（display only）
//   - reload で props は失われる（intentional、再表示防止）
"use client";

import { ManageUrlWarning } from "./ManageUrlWarning";
import { UrlCopyPanel } from "./UrlCopyPanel";

type Props = {
  appBaseUrl: string;
  publicUrlPath: string;
  manageUrlPath: string;
  onBackToEdit: () => void;
};

export function CompleteView({
  appBaseUrl,
  publicUrlPath,
  manageUrlPath,
  onBackToEdit,
}: Props) {
  const base = appBaseUrl.replace(/\/$/, "");
  const publicURL = `${base}${publicUrlPath}`;
  const manageURL = `${base}${manageUrlPath}`;
  return (
    <main className="mx-auto max-w-screen-md space-y-6 p-4 sm:p-6" data-testid="complete-view">
      <header className="space-y-2 text-center">
        <p className="text-xs font-medium uppercase text-brand-teal">公開完了</p>
        <h1 className="text-h1 text-ink">フォトブックを公開しました</h1>
        <p className="text-sm text-ink-medium">
          公開 URL を SNS や友人にシェアできます。管理用 URL は再発行までの間、
          フォトブックを編集できる唯一の鍵です。
        </p>
      </header>

      <section className="space-y-3">
        <UrlCopyPanel
          kind="public"
          label="公開 URL"
          url={publicURL}
          helper="このページを VRChat やフレンドに共有できます。"
          testId="complete-public-url"
        />
      </section>

      <section className="space-y-3">
        <ManageUrlWarning />
        <UrlCopyPanel
          kind="manage"
          label="管理用 URL（再表示できません）"
          url={manageURL}
          helper="このリンクを失うと、編集や公開停止ができなくなります。安全に保管してください。"
          testId="complete-manage-url"
        />
      </section>

      <section className="flex flex-col gap-3 border-t border-divider pt-4 sm:flex-row sm:items-center sm:justify-between">
        <a
          href={publicURL}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center justify-center rounded-md bg-brand-teal px-4 py-2 text-sm font-medium text-white hover:bg-brand-teal-hover"
          data-testid="complete-open-viewer"
        >
          公開ページを開く
        </a>
        <button
          type="button"
          onClick={onBackToEdit}
          className="rounded-md border border-divider bg-surface px-4 py-2 text-sm text-ink-medium hover:bg-surface-soft"
          data-testid="complete-back-to-edit"
        >
          編集ページに戻る
        </button>
      </section>

      <footer className="border-t border-divider-soft pt-6 text-center text-xs text-ink-soft">
        VRC PhotoBook（非公式ファンメイドサービス）
      </footer>
    </main>
  );
}
