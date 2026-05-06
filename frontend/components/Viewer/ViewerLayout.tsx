// ViewerLayout: 公開ビューアの主レイアウト（リプレース版）。
//
// デザイン参照:
//   - design 最終調整版 §1 Mobile (cover_first / 軽め)
//   - design 最終調整版 §2 PC 3 カラム (PageNavSidebar / Main / RightPanel)
//
// 構造:
//   [PublicTopBar (既存)]
//   [PC: 3 カラム grid / Mobile: 1 カラム]
//     - [PageNavSidebar (PC のみ)]
//     - [Main col]
//         [Cover (variant=cover_first or light)]
//         各 page ごとに [PageHero (Page header + meta + hero + sub thumbs + caption + note)]
//         [Mobile: ShareActions + Report link + Make CTA]
//     - [RightPanel (PC のみ: About / Share / Make CTA / Report)]
//   [PublicPageFooter (既存)]
//
// セキュリティ:
//   - URL に raw token を出さない（route 側で担保）
//   - 編集 URL / draft URL / manage URL を表示しない
//   - 削除した機能（いいね / ブックマーク / 画像ダウンロード）が混入しないこと
//
// Lightbox は ViewerInteractionProvider 内に常駐させ、PageHero 内の各
// LightboxTrigger から open する。

import Link from "next/link";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import type { PublicPhotobook, PublicPhoto } from "@/lib/publicPhotobook";

import { Cover } from "./Cover";
import { PageHero } from "./PageHero";
import { PageNavSidebar } from "./PageNavSidebar";
import { RightPanel } from "./RightPanel";
import { ShareActions } from "./ShareActions";
import { TypeAccent } from "./TypeAccent";
import { ViewerInteractionProvider } from "./ViewerInteractionProvider";

type Props = {
  photobook: PublicPhotobook;
};

export function ViewerLayout({ photobook }: Props) {
  const variant = normalizeOpening(photobook.openingStyle);
  const layout = photobook.layout;

  // 全ページを通した flat photo 配列を Server で計算（Lightbox の index 整合のため）
  const flatPhotos: PublicPhoto[] = photobook.pages.flatMap((p) => p.photos);

  // 各ページの photoIndexBase（flat 配列内での開始 index）を事前計算
  const pageStartIndex: number[] = [];
  let cursor = 0;
  for (const page of photobook.pages) {
    pageStartIndex.push(cursor);
    cursor += page.photos.length;
  }

  const shareUrl = buildShareUrl(photobook.slug);
  const shareText = `${photobook.title} / by ${photobook.creatorDisplayName}`;

  return (
    <>
      <PublicTopBar showPrimaryCta={true} />
      <ViewerInteractionProvider photos={flatPhotos}>
        <main className="mx-auto w-full max-w-screen-md px-4 py-6 sm:px-6 sm:py-8 lg:max-w-[1280px] lg:px-8">
          <div className="lg:grid lg:grid-cols-[88px_minmax(0,1fr)_280px] lg:items-start lg:gap-8">
            {/* PC 左サイドバー (Mobile では非表示) */}
            <div className="hidden lg:block">
              <PageNavSidebar pages={photobook.pages} />
            </div>

            {/* Main col */}
            <div className="space-y-8">
              <div className="flex items-center gap-2">
                <TypeAccent type={photobook.type} />
              </div>
              <Cover photobook={photobook} variant={variant} />
              <div className="space-y-12">
                {photobook.pages.map((page, idx) => (
                  <PageHero
                    key={idx}
                    page={page}
                    pageNumber={idx + 1}
                    layout={layout}
                    photoIndexBase={pageStartIndex[idx]}
                  />
                ))}
              </div>

              {/* Mobile: ページ末尾の Share + 通報 + Make CTA。PC は RightPanel に集約 */}
              <div className="space-y-6 lg:hidden" data-testid="viewer-mobile-actions">
                <section className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm">
                  <p className="mb-3 text-[10px] font-bold tracking-[0.08em] text-ink-medium">
                    シェア・アクション
                  </p>
                  <ShareActions shareUrl={shareUrl} shareText={shareText} />
                  <p className="mt-4 text-xs text-ink-medium">
                    問題がある場合はこちらからご連絡ください
                  </p>
                  <Link
                    href={`/p/${photobook.slug}/report`}
                    className="mt-1 inline-flex items-center gap-1 text-sm font-semibold text-ink-medium hover:text-teal-700"
                    data-testid="viewer-report-link"
                  >
                    通報する
                    <span aria-hidden="true">🚩</span>
                  </Link>
                </section>

                <section
                  className="rounded-lg border border-amber-200 bg-amber-50/70 p-4 shadow-sm"
                  data-testid="viewer-make-cta-mobile"
                >
                  <p className="text-sm font-bold text-ink">
                    あなたの写真も、1 冊のフォトブックに。
                  </p>
                  <Link
                    href="/create"
                    className="mt-3 flex items-center justify-center gap-2 rounded-lg bg-teal-500 px-4 py-2.5 text-sm font-bold text-white transition-colors hover:bg-teal-600"
                  >
                    このフォトブックを作る
                    <span aria-hidden="true">→</span>
                  </Link>
                </section>
              </div>
            </div>

            {/* PC 右サイドバー */}
            <div className="hidden lg:block">
              <div className="sticky top-20">
                <RightPanel photobook={photobook} />
              </div>
            </div>
          </div>

          <PublicPageFooter />
        </main>
      </ViewerInteractionProvider>
    </>
  );
}

function normalizeOpening(opening: string): "cover_first" | "light" {
  return opening === "cover_first" ? "cover_first" : "light";
}

function buildShareUrl(slug: string): string {
  const base =
    typeof process !== "undefined" && process.env.NEXT_PUBLIC_BASE_URL
      ? process.env.NEXT_PUBLIC_BASE_URL
      : "https://app.vrc-photobook.com";
  return `${base.replace(/\/$/, "")}/p/${slug}`;
}
