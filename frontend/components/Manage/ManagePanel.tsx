// ManagePanel: 管理ページの主パネル。
//
// design 参照: design/mockups/prototype/screens-b.jsx Manage / pc-screens-b.jsx PCManage
//
// セキュリティ:
//   - manage_url_token を画面に出さない（業務知識 v4: 発行直後のみ通知メール経由で表示、
//     再表示しない。再発行は PR32 で SendGrid 経由）
//   - draft_edit_token を画面に出さない
//   - 公開 URL は teal、Manage 行為が必要な区画は violet で示す

import type { ManagePhotobook } from "@/lib/managePhotobook";
import { UrlRow } from "@/components/UrlRow";
import { HiddenByOperatorBanner } from "@/components/Manage/HiddenByOperatorBanner";

type Props = {
  photobook: ManagePhotobook;
  appBaseUrl: string;
};

const STATUS_LABEL: Record<string, string> = {
  draft: "下書き（未公開）",
  published: "公開中",
  deleted: "削除済み",
};

export function ManagePanel({ photobook, appBaseUrl }: Props) {
  const fullPublicUrl =
    photobook.publicUrlPath !== undefined
      ? `${appBaseUrl.replace(/\/$/, "")}${photobook.publicUrlPath}`
      : null;
  const statusLabel = STATUS_LABEL[photobook.status] ?? photobook.status;

  return (
    <main className="mx-auto max-w-screen-md px-4 py-6 sm:px-6">
      <header className="space-y-2">
        <p className="text-xs font-medium text-ink-medium">管理ページ</p>
        <h1 className="text-h1 text-ink">{photobook.title}</h1>
        <p className="text-sm text-ink-medium">
          状態: <span className="font-medium text-ink-strong">{statusLabel}</span>
          {photobook.hiddenByOperator && (
            <span className="ml-2 rounded-sm bg-status-error-soft px-2 py-0.5 text-xs text-status-error">
              運営により一時非表示
            </span>
          )}
        </p>
      </header>

      {photobook.hiddenByOperator && (
        <div className="mt-6">
          <HiddenByOperatorBanner />
        </div>
      )}

      <section className="mt-6 space-y-4">
        {fullPublicUrl ? (
          <UrlRow kind="public" label="公開 URL" url={fullPublicUrl} />
        ) : (
          <div className="rounded-lg border border-dashed border-divider px-4 py-3 text-sm text-ink-medium">
            まだ公開されていません。下書きを編集して公開すると公開 URL が発行されます。
          </div>
        )}
      </section>

      <section className="mt-8 space-y-3 rounded-lg border border-divider bg-surface-soft p-4">
        <h2 className="text-h2 text-ink">情報</h2>
        <dl className="space-y-2 text-sm">
          <div className="flex justify-between gap-4">
            <dt className="text-ink-medium">公開写真の数</dt>
            <dd className="font-medium text-ink-strong">
              {photobook.availableImageCount}
            </dd>
          </div>
          <div className="flex justify-between gap-4">
            <dt className="text-ink-medium">公開設定</dt>
            <dd className="font-medium text-ink-strong">{photobook.visibility}</dd>
          </div>
          <div className="flex justify-between gap-4">
            <dt className="text-ink-medium">管理リンクのバージョン</dt>
            <dd className="font-medium text-ink-strong">
              {photobook.manageUrlTokenVersion}
            </dd>
          </div>
          {photobook.publishedAt && (
            <div className="flex justify-between gap-4">
              <dt className="text-ink-medium">公開日時</dt>
              <dd className="font-num text-ink-strong">{photobook.publishedAt}</dd>
            </div>
          )}
        </dl>
      </section>

      <section className="mt-8 space-y-3 rounded-lg border border-dashed border-divider px-4 py-4 text-sm text-ink-medium">
        <h2 className="text-h2 text-ink">管理リンクの再発行</h2>
        <p>
          管理リンクが流出した・紛失した場合は再発行できます。再発行すると
          以前の管理リンクは無効になります。
        </p>
        <button
          type="button"
          disabled
          className="cursor-not-allowed rounded-md border border-divider bg-surface-soft px-4 py-2 text-xs text-ink-soft"
          aria-disabled="true"
          data-testid="manage-reissue-button-placeholder"
        >
          再発行（後日対応）
        </button>
      </section>

      <footer className="mt-10 text-center text-xs text-ink-soft">
        VRC PhotoBook（非公式ファンメイドサービス）
      </footer>
    </main>
  );
}
