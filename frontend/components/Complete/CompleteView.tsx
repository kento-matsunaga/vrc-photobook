// CompleteView: publish 直後に表示する完了画面。
//
// design 参照: design/mockups/prototype/screens-a.jsx の Complete / pc-screens-a.jsx PCComplete
//
// セキュリティ:
//   - manage URL は raw token を含むため、URL bar / log / localStorage に出さない
//   - 本コンポーネントの prop で manage_url_path を受けるが、URL 遷移しない（display only）
//   - reload で props は失われる（intentional、再表示防止）
//
// Provider 不要 改善（PR32b、ADR-0006）:
//   - ManageUrlSavePanel: .txt download / mailto / 「保存しました」確認チェックを集約
//   - チェック前は閉じる導線（編集ページに戻る / 公開ページを開く）に注意文を出し、
//     ボタン自体は disable しない（誤誘導防止のため警告中心、選択肢は奪わない）
"use client";

import { useState } from "react";

import { ManageUrlSavePanel } from "./ManageUrlSavePanel";
import { ManageUrlWarning } from "./ManageUrlWarning";
import { UrlCopyPanel } from "./UrlCopyPanel";

type Props = {
  appBaseUrl: string;
  publicUrlPath: string;
  manageUrlPath: string;
  onBackToEdit: () => void;
};

// extractSlugFromManagePath は public_url_path から slug を取り出す（無ければ undefined）。
// 想定形式: "/p/<slug>" または "/manage/<...>"。manage_url_path は token 含むので使わない。
function extractSlugFromPublicPath(publicUrlPath: string): string | undefined {
  const m = publicUrlPath.match(/^\/p\/([^/?#]+)/);
  return m ? m[1] : undefined;
}

export function CompleteView({
  appBaseUrl,
  publicUrlPath,
  manageUrlPath,
  onBackToEdit,
}: Props) {
  const base = appBaseUrl.replace(/\/$/, "");
  const publicURL = `${base}${publicUrlPath}`;
  const manageURL = `${base}${manageUrlPath}`;
  const slug = extractSlugFromPublicPath(publicUrlPath);

  const [savedConfirmed, setSavedConfirmed] = useState(false);

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
        <ManageUrlSavePanel
          manageURL={manageURL}
          slug={slug}
          saved={savedConfirmed}
          onSavedChange={setSavedConfirmed}
        />
      </section>

      <section
        className="flex flex-col gap-3 border-t border-divider pt-4 sm:flex-row sm:items-center sm:justify-between"
        data-testid="complete-actions"
      >
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

      {!savedConfirmed && (
        <div
          className="rounded-md border border-status-warn bg-status-warn-soft px-4 py-3 text-xs text-ink-strong"
          role="status"
          data-testid="complete-save-reminder"
        >
          管理用 URL の保存をまだ確認していません。上の保存方法のいずれかで保管したら、
          チェックボックスをオンにしてください。
        </div>
      )}

      <footer className="border-t border-divider-soft pt-6 text-center text-xs text-ink-soft">
        <p>VRC PhotoBook（非公式ファンメイドサービス）</p>
        <p className="mt-1">
          管理 URL の保存方法・紛失時のご案内は{" "}
          <a href="/help/manage-url" className="underline" data-testid="complete-faq-link">
            よくある質問
          </a>
          。
        </p>
      </footer>
    </main>
  );
}
