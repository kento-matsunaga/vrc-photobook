// ViewerLayout: 公開 Viewer のページ全体レイアウト。
//
// セキュリティ:
//   - URL に raw token を出さない（route 側で担保済み）
//   - 編集 URL / draft URL / manage URL を表示しない（業務知識 v4）
//
// m2-design-refresh STOP β-5 (本 commit、visual のみ):
//   - design `wf-screens-c.jsx:4-33` (Viewer M) / `:34-76` (Viewer PC wf-grid-2-1) 視覚整合
//   - PC は `lg:grid-cols-[1fr_300px]` (design wf-grid-2-1 1fr / 1fr の代替で sidebar 固定幅)
//     左 col: cover + h1 + description + creator info + page grids / 右 col: sticky 右 panel
//   - Q-G PC sticky 右 panel: Creator card + Report link card (design `wf-screens-c.jsx:55-70`)
//   - Mobile: 縦 stack (sticky panel 非表示、報告リンクは footer extraSlot 経由)
//   - PublicTopBar 統合 (showPrimaryCta=true / 公開 viewer は visitor が新規作成へ進む導線として
//     primary CTA「無料で作る」が UX 適合、design WFBrowser 通り)
//   - 既存 data-testid="viewer-report-link" 維持 (footer extraSlot)
//   - PC sticky panel に追加で data-testid="viewer-report-link-pc" / "viewer-creator-card"
//   - public fetch / data mapping / production data (title / description / creator / cover) は **触らない**
//   - PC sticky 右 panel の OGP annotation は design では「OGP: /ogp/{id}?v=1」と技術 annotation
//     だが、production user に raw photobook_id を露出させないため UI には出さない (design 注釈を
//     production 補完で削除、§安全側方針)

import Link from "next/link";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import type { PublicPhotobook } from "@/lib/publicPhotobook";

import { PhotoGrid } from "./PhotoGrid";

type Props = {
  photobook: PublicPhotobook;
};

/**
 * Viewer の主レイアウト。
 *
 * MVP は magazine / card / large の layout 差を最小骨格で表現せず、
 * 共通の縦並びで提示する。type / layout に応じた装飾は PR41+ で。
 */
export function ViewerLayout({ photobook }: Props) {
  const coverTitle = photobook.coverTitle ?? photobook.title;
  const creatorDisplayName = photobook.creatorDisplayName.trim() || "作者未設定";
  // creator avatar placeholder (initial 1 文字、design `wf-screens-c.jsx:13` round border 風)
  const creatorInitial = creatorDisplayName.slice(0, 1).toUpperCase();
  return (
    <>
      <PublicTopBar />
      <main className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9 lg:max-w-[1120px]">
        {/* design `wf-screens-c.jsx:38` PC `wf-grid-2-1` / Mobile 単 col */}
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-[1fr_300px] lg:items-start lg:gap-8">
          {/* Main col */}
          <div className="space-y-8">
            <header className="space-y-4">
              {photobook.cover && (
                <div className="overflow-hidden rounded-lg border border-divider-soft bg-surface-soft shadow-sm">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    src={photobook.cover.display.url}
                    alt=""
                    width={photobook.cover.display.width}
                    height={photobook.cover.display.height}
                    loading="eager"
                    decoding="async"
                    className="block h-auto w-full"
                  />
                </div>
              )}
              <div className="space-y-2">
                <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">{coverTitle}</h1>
                {photobook.description && (
                  <p className="text-body leading-[1.7] text-ink-strong">
                    {photobook.description}
                  </p>
                )}
              </div>
              {/* design `wf-screens-c.jsx:12-16` (M) creator inline (avatar circle + name + @x_id) */}
              <div
                className="flex items-center gap-3"
                data-testid="viewer-creator-inline"
              >
                <span
                  aria-hidden="true"
                  className="grid h-8 w-8 shrink-0 place-items-center rounded-full border-[1.5px] border-divider bg-surface-soft text-xs font-bold text-ink-medium"
                >
                  {creatorInitial}
                </span>
                <span className="text-sm font-bold text-ink">
                  {creatorDisplayName}
                </span>
                {photobook.creatorXId ? (
                  <span className="font-num text-xs text-ink-medium">
                    @{photobook.creatorXId}
                  </span>
                ) : null}
              </div>
            </header>

            <div className="space-y-8">
              {photobook.pages.map((page, idx) => (
                <PhotoGrid key={idx} page={page} pageNumber={idx + 1} />
              ))}
            </div>
          </div>

          {/* PC sticky 右 panel (Q-G、design `wf-screens-c.jsx:55-70`)。
              Mobile では hidden、報告リンクは footer extraSlot 経由 */}
          <aside className="hidden lg:block">
            <div className="sticky top-20 space-y-3">
              <section
                className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm"
                data-testid="viewer-creator-card"
              >
                <h2 className="mb-3 flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
                  <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
                  Creator
                </h2>
                <div className="flex items-center gap-3">
                  <span
                    aria-hidden="true"
                    className="grid h-9 w-9 shrink-0 place-items-center rounded-full border-[1.5px] border-divider bg-surface-soft text-sm font-bold text-ink-medium"
                  >
                    {creatorInitial}
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-bold text-ink">
                      {creatorDisplayName}
                    </p>
                    {photobook.creatorXId && (
                      <p className="truncate font-num text-xs text-ink-medium">
                        @{photobook.creatorXId}
                      </p>
                    )}
                  </div>
                </div>
              </section>

              <Link
                href={`/p/${photobook.slug}/report`}
                className="block rounded-lg border border-divider-soft bg-surface p-4 text-center text-xs font-semibold text-ink-strong shadow-sm transition-colors hover:border-teal-300 hover:text-teal-700"
                data-testid="viewer-report-link-pc"
              >
                このフォトブックを通報 →
              </Link>
            </div>
          </aside>
        </div>

        <PublicPageFooter
          extraSlot={
            <Link
              href={`/p/${photobook.slug}/report`}
              className="text-sm text-ink-medium underline hover:text-ink-strong"
              data-testid="viewer-report-link"
            >
              このフォトブックを通報
            </Link>
          }
        />
      </main>
    </>
  );
}
