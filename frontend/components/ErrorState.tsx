// ErrorState: 公開 / 管理ページの 404 / 410 / 401 / 500 を表示する Server Component。
//
// セキュリティ:
//   - 失敗詳細を画面に出さない（kind と固定文言のみ）
//   - URL に raw token / Cookie 値が含まれていないことは Route Handler 側で担保

type Variant = "not_found" | "gone" | "unauthorized" | "server_error";

type Props = {
  variant: Variant;
};

const TITLE: Record<Variant, string> = {
  not_found: "ページが見つかりません",
  gone: "このページは現在閲覧できません",
  unauthorized: "アクセスできません",
  server_error: "一時的にエラーが発生しました",
};

const BODY: Record<Variant, string> = {
  not_found: "URL を確認するか、作成者にお問い合わせください。",
  gone: "運営により一時的に非表示になっています。時間をおいて再度ご確認ください。",
  unauthorized: "管理用リンクから入り直してください。",
  server_error: "しばらく待ってから再度お試しください。",
};

/**
 * 共通エラー表示コンポーネント。Public / Manage 共用。
 */
export function ErrorState({ variant }: Props) {
  return (
    <main
      className="mx-auto flex min-h-[60vh] max-w-screen-md items-center justify-center px-4"
      data-testid={`error-state-${variant}`}
    >
      <div className="rounded-lg border border-divider bg-surface p-6 text-center shadow-sm">
        <h1 className="text-h2 text-ink">{TITLE[variant]}</h1>
        <p className="mt-3 text-body text-ink-medium">{BODY[variant]}</p>
      </div>
    </main>
  );
}
