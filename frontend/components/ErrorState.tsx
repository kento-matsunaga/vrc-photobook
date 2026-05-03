// ErrorState: 公開 / 管理ページの 404 / 410 / 401 / 500 を表示する Server Component。
//
// セキュリティ:
//   - 失敗詳細を画面に出さない（kind と固定文言のみ）
//   - URL に raw token / Cookie 値が含まれていないことは Route Handler 側で担保
//
// m2-design-refresh STOP β-5 (本 commit、visual のみ):
//   - design `wf-screens-c.jsx:445-475` `WFErrorStates` (4 variant shell) 視覚整合
//   - design `wireframe-styles.css:547-562` `.wf-error-shell` (round teal-50 icon + title + msg + center btn)
//   - 各 variant に対応する HTTP code (`401 / 404 / 410 / 500`) を large icon として表示
//   - 既存 variant API (`not_found / gone / unauthorized / server_error`) と props は **不変**
//   - 既存 TITLE / BODY 文言は **完全維持**
//   - data-testid={`error-state-${variant}`} 維持
//   - 「トップへ戻る」link を design 通り追加 (server_error 時の「再試行」は static link 化が
//     困難なため、動線として / に戻す方が UX として安全)

import Link from "next/link";

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

// design `wf-screens-c.jsx:447-450` の HTTP code mapping
const CODE: Record<Variant, string> = {
  unauthorized: "401",
  not_found: "404",
  gone: "410",
  server_error: "500",
};

/**
 * 共通エラー表示コンポーネント。Public / Manage 共用。
 */
export function ErrorState({ variant }: Props) {
  return (
    <main
      className="mx-auto flex min-h-[60vh] max-w-screen-md items-center justify-center px-4 py-12"
      data-testid={`error-state-${variant}`}
    >
      {/* design `wireframe-styles.css:547-562` `.wf-error-shell` 整合: center stack + round teal icon */}
      <div className="w-full max-w-md text-center">
        <div className="mx-auto grid h-16 w-16 place-items-center rounded-full border-2 border-teal-200 bg-teal-50 font-num text-xl font-extrabold text-teal-700">
          {CODE[variant]}
        </div>
        <h1 className="mt-5 text-h2 font-extrabold text-ink">{TITLE[variant]}</h1>
        <p className="mt-3 text-sm leading-[1.7] text-ink-medium">{BODY[variant]}</p>
        <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
          <Link
            href="/"
            className="inline-flex h-10 items-center justify-center rounded-md border border-divider bg-surface px-5 text-xs font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700"
          >
            トップへ戻る
          </Link>
        </div>
      </div>
    </main>
  );
}
