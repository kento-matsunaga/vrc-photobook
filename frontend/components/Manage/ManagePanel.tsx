// ManagePanel: 管理ページの主パネル。
//
// セキュリティ:
//   - manage_url_token を画面に出さない（業務知識 v4: 発行直後のみ通知メール経由で表示、
//     再表示しない。再発行は ADR-0006 後続）
//   - draft_edit_token を画面に出さない
//   - 公開 URL は teal、Manage 行為が必要な区画は violet で示す
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - design `wf-screens-b.jsx:319-364` (M) / `:365-411` (PC) `WFManage` 視覚整合
//   - PC: wf-grid-2-1 (`:379` left content 2fr + right 再発行 1fr)
//   - status / hidden を wf-badge.dark / wf-badge で表示 (`wireframe-styles.css:372-389`)
//   - 情報 panel を wf-box + 4 数値表示 (PC は wf-grid-4 / Mobile は縦)
//   - 再発行 panel を wf-box.dashed + disabled button placeholder
//   - HiddenByOperatorBanner を wf-note.warn 視覚で表示 (上記 component で対応)
//   - data-testid="manage-reissue-button-placeholder" / appBaseUrl prop / 「公開 URL」「管理リンクの再発行」
//     文言は **完全維持** (test 互換)
//   - manage URL 再表示なし方針 / reissue placeholder disabled は **触らない**

import type { ManagePhotobook } from "@/lib/managePhotobook";
import { UrlRow } from "@/components/UrlRow";
import { HiddenByOperatorBanner } from "@/components/Manage/HiddenByOperatorBanner";
import { ManageEditResumeButton } from "@/components/Manage/ManageEditResumeButton";
import { ManageSessionRevokeButton } from "@/components/Manage/ManageSessionRevokeButton";
import { ManageUrlNoticeBanner } from "@/components/Manage/ManageUrlNoticeBanner";
import { ManageVisibilitySensitiveForm } from "@/components/Manage/ManageVisibilitySensitiveForm";

type Props = {
  photobook: ManagePhotobook;
  appBaseUrl: string;
};

const STATUS_LABEL: Record<string, string> = {
  draft: "下書き（未公開）",
  published: "公開中",
  deleted: "削除済み",
};

const VISIBILITY_LABEL: Record<string, string> = {
  public: "公開",
  unlisted: "限定公開",
  private: "非公開",
};

export function ManagePanel({ photobook, appBaseUrl }: Props) {
  const fullPublicUrl =
    photobook.publicUrlPath !== undefined
      ? `${appBaseUrl.replace(/\/$/, "")}${photobook.publicUrlPath}`
      : null;
  const statusLabel = STATUS_LABEL[photobook.status] ?? photobook.status;
  const visibilityLabel = VISIBILITY_LABEL[photobook.visibility] ?? photobook.visibility;

  return (
    <main className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9 lg:max-w-[1120px]">
      <header className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          {/* design `wf-screens-b.jsx:324-326` `wf-badge.dark` PUBLISHED + `wf-badge` hidden */}
          <span className="inline-flex h-6 items-center rounded-full bg-ink px-3 text-[10.5px] font-semibold uppercase tracking-wide text-white">
            {photobook.status.toUpperCase()}
          </span>
          {photobook.hiddenByOperator && (
            <span className="inline-flex h-6 items-center rounded-full border border-status-warning bg-status-warning-soft px-3 text-[10.5px] font-semibold uppercase tracking-wide text-status-warning">
              hidden
            </span>
          )}
        </div>
        <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">{photobook.title}</h1>
        <p className="text-sm text-ink-medium">
          状態: <span className="font-bold text-ink-strong">{statusLabel}</span>
        </p>
      </header>

      {photobook.hiddenByOperator && (
        <div className="mt-5">
          <HiddenByOperatorBanner />
        </div>
      )}

      {/* M-1a: 常設注意喚起バナー（warn ではなく info tone） */}
      <div className="mt-5">
        <ManageUrlNoticeBanner />
      </div>

      {/* design `wf-screens-b.jsx:379` PC `wf-grid-2-1`、Mobile は単 col */}
      <div className="mt-7 grid grid-cols-1 gap-5 lg:grid-cols-[2fr_1fr] lg:items-start lg:gap-5">
        {/* Left col: 公開 URL + 情報 panel */}
        <div className="space-y-4">
          <section className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm sm:p-5">
            <SectionTitle>公開 URL</SectionTitle>
            {fullPublicUrl ? (
              <UrlRow kind="public" label="公開 URL" url={fullPublicUrl} />
            ) : (
              <div className="rounded-md border-2 border-dashed border-divider-soft bg-surface-soft px-4 py-3 text-xs text-ink-medium">
                まだ公開されていません。下書きを編集して公開すると公開 URL が発行されます。
              </div>
            )}
          </section>

          <section className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm sm:p-5">
            <SectionTitle>情報</SectionTitle>
            {/* design `wf-screens-b.jsx:387-393` PC wf-grid-4 / Mobile 縦 dl */}
            <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <InfoCell label="公開写真の数" value={String(photobook.availableImageCount)} numeric />
              <InfoCell label="公開設定" value={visibilityLabel} />
              <InfoCell
                label="管理リンクのバージョン"
                value={`v${photobook.manageUrlTokenVersion}`}
                numeric
              />
              {photobook.publishedAt && (
                <InfoCell label="公開日時" value={photobook.publishedAt} numeric />
              )}
            </dl>
          </section>

          {/* M-1a: 公開設定（visibility unlisted/private + sensitive 切替） */}
          {photobook.status === "published" && (
            <section className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm sm:p-5">
              <SectionTitle>公開設定</SectionTitle>
              <ManageVisibilitySensitiveForm
                photobookId={photobook.photobookId}
                initialVersion={photobook.version}
                initialVisibility={
                  (photobook.visibility === "public" ||
                  photobook.visibility === "unlisted" ||
                  photobook.visibility === "private")
                    ? photobook.visibility
                    : "unlisted"
                }
                initialSensitive={photobook.sensitive}
              />
            </section>
          )}

          {/* M-1a: 編集を再開
              Backend `GetEditView` は status='draft' 以外を ErrEditNotAllowed で reject する
              ため、ボタンを押しても /edit で詰まる UX を避ける。draft 状態のときのみボタンを
              表示し、それ以外は「未対応」案内のみ出す。Backend `IssueDraftSessionFromManage`
              endpoint / UseCase / `ManageEditResumeButton` component は M-1b の unpublish
              着地後に活用される plumbing として保持する。 */}
          <section
            data-testid="manage-edit-resume-section-wrapper"
            className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm sm:p-5"
          >
            <SectionTitle>編集を再開</SectionTitle>
            {photobook.status === "draft" ? (
              <>
                <p className="mb-3 text-xs leading-[1.6] text-ink-medium">
                  編集画面に戻り、内容を修正します。
                </p>
                <ManageEditResumeButton photobookId={photobook.photobookId} />
              </>
            ) : (
              <p
                data-testid="manage-edit-resume-not-supported"
                className="text-xs leading-[1.6] text-ink-medium"
              >
                公開済みのフォトブックの再編集は MVP 範囲では未対応です。公開停止機能の提供後に利用可能になります。
              </p>
            )}
          </section>
        </div>

        {/* Right col: M-1a session revoke + 管理リンクの再発行 placeholder */}
        <div className="space-y-4">
          {/* M-1a: この端末の管理権限を削除 */}
          <section className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm sm:p-5">
            <SectionTitle>この端末の管理権限</SectionTitle>
            <ManageSessionRevokeButton photobookId={photobook.photobookId} />
          </section>

          <section className="space-y-3 rounded-lg border-2 border-dashed border-divider-soft bg-surface-soft p-4 sm:p-5">
            <SectionTitle>管理リンクの再発行</SectionTitle>
            <p className="text-xs leading-[1.6] text-ink-medium">
              管理リンクが流出した・紛失した場合は再発行できます。再発行すると
              以前の管理リンクは無効になります。
            </p>
            <button
              type="button"
              disabled
              className="inline-flex h-10 w-full cursor-not-allowed items-center justify-center rounded-md border border-divider bg-surface-soft px-4 text-xs font-semibold text-ink-soft"
              aria-disabled="true"
              data-testid="manage-reissue-button-placeholder"
            >
              再発行（後日対応）
            </button>
          </section>
        </div>
      </div>

      <footer className="mt-10 border-t border-divider-soft pt-6 text-center text-xs text-ink-soft">
        VRC PhotoBook（非公式ファンメイドサービス）
      </footer>
    </main>
  );
}

function SectionTitle({ children }: { children: string }) {
  return (
    <h2 className="mb-3 flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
      <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
      {children}
    </h2>
  );
}

function InfoCell({ label, value, numeric }: { label: string; value: string; numeric?: boolean }) {
  return (
    <div className="flex flex-col gap-1 border-t border-divider-soft pt-3 first:border-t-0 first:pt-0 sm:border-t-0 sm:pt-0">
      <dt className="text-[11px] text-ink-medium">{label}</dt>
      <dd
        className={`text-sm font-bold text-ink-strong ${numeric ? "font-num" : ""}`}
      >
        {value}
      </dd>
    </div>
  );
}
