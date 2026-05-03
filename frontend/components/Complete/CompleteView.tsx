// CompleteView: publish 直後に表示する完了画面。
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
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - design `wf-screens-b.jsx:197-246` (M) / `:247-293` (PC) `WFComplete` 視覚整合
//   - eyebrow「Status: PUBLISHED」+ h1「フォトブックを公開しました」(design 通り)
//   - PC は wf-grid-2 で公開 URL + 管理 URL 並列、Mobile は縦 stack
//   - PublicTopBar 統合 (showPrimaryCta=false、draft → published 完了画面で違和感回避)
//   - design footer FAQ link / wf-note 視覚整合
//   - 全 data-testid (complete-view / complete-actions / complete-open-viewer /
//     complete-back-to-edit / complete-save-reminder / complete-faq-link) **完全維持**
//   - savedConfirmed state / extractSlugFromPublicPath / onBackToEdit handler /
//     ManageUrlSavePanel / ManageUrlWarning / UrlCopyPanel logic は **触らない**
"use client";

import { useState } from "react";

import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";

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
    <>
      {/* draft → published 完了画面、PublicTopBar 統合 (showPrimaryCta=false) */}
      <PublicTopBar showPrimaryCta={false} />
      <main
        className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9"
        data-testid="complete-view"
      >
        <header className="space-y-2">
          <SectionEyebrow>Status: PUBLISHED</SectionEyebrow>
          <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">
            フォトブックを公開しました
          </h1>
          <p className="text-sm leading-[1.7] text-ink-medium">
            公開 URL を SNS や友人にシェアできます。管理用 URL は再発行までの間、
            フォトブックを編集できる唯一の鍵です。
          </p>
        </header>

        {/* design `wf-screens-b.jsx:254-272` PC wf-grid-2 / Mobile 縦 stack で
            公開 URL + 管理 URL を並列表示 */}
        <div className="mt-7 grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-5">
          <UrlCopyPanel
            kind="public"
            label="公開 URL"
            url={publicURL}
            helper="このページを VRChat やフレンドに共有できます。"
            testId="complete-public-url"
          />
          <UrlCopyPanel
            kind="manage"
            label="管理用 URL（再表示できません）"
            url={manageURL}
            helper="このリンクを失うと、編集や公開停止ができなくなります。安全に保管してください。"
            testId="complete-manage-url"
          />
        </div>

        <div className="mt-5">
          <ManageUrlWarning />
        </div>

        <div className="mt-5">
          <ManageUrlSavePanel
            manageURL={manageURL}
            slug={slug}
            saved={savedConfirmed}
            onSavedChange={setSavedConfirmed}
          />
        </div>

        <section
          className="mt-7 flex flex-col gap-3 border-t border-divider-soft pt-5 sm:flex-row sm:items-center sm:justify-between"
          data-testid="complete-actions"
        >
          <a
            href={publicURL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex h-12 items-center justify-center rounded-[10px] bg-brand-teal px-6 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover"
            data-testid="complete-open-viewer"
          >
            公開ページを開く
          </a>
          <button
            type="button"
            onClick={onBackToEdit}
            className="inline-flex h-12 items-center justify-center rounded-[10px] border border-divider bg-surface px-6 text-sm font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700"
            data-testid="complete-back-to-edit"
          >
            編集ページに戻る
          </button>
        </section>

        {!savedConfirmed && (
          <div
            className="mt-5 flex items-start gap-2.5 rounded-lg border-l-[3px] border-status-warn bg-status-warn-soft p-3.5"
            role="status"
            data-testid="complete-save-reminder"
          >
            <span
              aria-hidden="true"
              className="grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full bg-status-warn font-serif text-xs font-bold italic leading-none text-white"
            >
              !
            </span>
            <p className="flex-1 text-xs leading-[1.6] text-ink-strong">
              管理用 URL の保存をまだ確認していません。上の保存方法のいずれかで保管したら、
              チェックボックスをオンにしてください。
            </p>
          </div>
        )}

        <footer className="mt-10 border-t border-divider-soft pt-6 text-center text-xs text-ink-soft">
          <p>VRC PhotoBook（非公式ファンメイドサービス）</p>
          <p className="mt-1">
            管理 URL の保存方法・紛失時のご案内は{" "}
            <a
              href="/help/manage-url"
              className="text-teal-600 underline hover:text-teal-700"
              data-testid="complete-faq-link"
            >
              よくある質問
            </a>
            。
          </p>
        </footer>
      </main>
    </>
  );
}
