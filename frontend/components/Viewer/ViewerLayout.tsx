// ViewerLayout: 公開 Viewer のページ全体レイアウト (v2 redesign)。
//
// 採用元: TESTImage 完成イメージ「VRC PhotoBook 公開ビューア 完成イメージ (最終調整版)」
//
// 構成 (PC、3 col grid):
//   [PageNavSidebar] [Main col: Cover → SensitiveGate → 各 PageHero → Mobile-only Share/Report/CTA] [RightPanel]
// 構成 (Mobile、single col stack):
//   PublicTopBar → Cover → SensitiveGate → 各 PageHero → ShareActions → 通報リンク → 作る CTA → PublicPageFooter
//
// 設計判断 (v2):
//   - 落とし穴 #4 対応: pages.flatMap で flat photos 配列を作り、ViewerInteractionProvider に渡す
//     各 PageHero に photoIndexBase を計算して伝播 → Lightbox は全ページ横断 navigate 可能
//   - 落とし穴 #5 対応: ViewerInteractionProvider ("use client") が Server Component を
//     children として受け取る三角構造
//   - LP / About 等で共有される PublicTopBar / PublicPageFooter / TrustStrip は触らない
//
// セキュリティ:
//   - photobookId / image_id は DOM / data-* に出さない (slug は OK)
//   - presigned URL は <img src> 渡しのみ
//
// 業務違反 3 機能 (いいね / ブックマーク / 画像ダウンロード) は実装しない (ViewerLayout.test.tsx で guard)

import Link from "next/link";

import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { Cover } from "@/components/Viewer/Cover";
import { PageHero } from "@/components/Viewer/PageHero";
import { PageNavSidebar } from "@/components/Viewer/PageNavSidebar";
import { RightPanel } from "@/components/Viewer/RightPanel";
import { SensitiveGate } from "@/components/Viewer/SensitiveGate";
import { ShareActions } from "@/components/Viewer/ShareActions";
import { TypeAccent } from "@/components/Viewer/TypeAccent";
import { ViewerInteractionProvider } from "@/components/Viewer/ViewerInteractionProvider";
import type { PublicPhoto, PublicPhotobook } from "@/lib/publicPhotobook";

type Props = {
  photobook: PublicPhotobook;
  /** 既定 false。Backend が isSensitive を返すまで dead path */
  isSensitive?: boolean;
};

function buildShareUrl(slug: string): string {
  const base = process.env.NEXT_PUBLIC_BASE_URL ?? "https://app.vrc-photobook.com";
  return `${base.replace(/\/$/, "")}/p/${slug}`;
}

function buildShareText(photobook: PublicPhotobook): string {
  // X share text: タイトル + creator + ハッシュタグ (お任せ)
  const creator = photobook.creatorXId ? ` by @${photobook.creatorXId}` : "";
  return `${photobook.title}${creator} | VRC PhotoBook`;
}

export function ViewerLayout({ photobook, isSensitive = false }: Props) {
  // flat photos (落とし穴 #4 対応)
  const flatPhotos: PublicPhoto[] = photobook.pages.flatMap((p) => p.photos);

  // 各 page の photoIndexBase を事前計算
  const pageBases: number[] = [];
  let acc = 0;
  for (const page of photobook.pages) {
    pageBases.push(acc);
    acc += page.photos.length;
  }

  const variant: "cover_first" | "light" =
    photobook.openingStyle === "cover_first" ? "cover_first" : "light";

  const shareUrl = buildShareUrl(photobook.slug);
  const shareText = buildShareText(photobook);
  const reportHref = `/p/${photobook.slug}/report`;

  return (
    <ViewerInteractionProvider flatPhotos={flatPhotos}>
      <PublicTopBar />

      {/* Cover は full-bleed (max-width 制約なし) */}
      <Cover photobook={photobook} variant={variant} />

      <main
        data-testid="viewer-main"
        className="mx-auto w-full max-w-screen-md px-4 py-8 sm:px-9 sm:py-10 lg:max-w-[1280px]"
      >
        <SensitiveGate isSensitive={isSensitive}>
          {/* 上部メタ行 (TypeAccent + creator inline) — Mobile/PC 共通 */}
          <div
            data-testid="viewer-meta-strip"
            className="mb-6 flex flex-wrap items-center gap-3 sm:mb-8"
          >
            <TypeAccent type={photobook.type} />
            <span
              aria-hidden="true"
              className="block h-3 w-px bg-divider"
            />
            <span
              data-testid="viewer-creator-inline"
              className="text-xs text-ink-medium sm:text-sm"
            >
              by{" "}
              <span className="font-bold text-ink-strong">
                {photobook.creatorDisplayName.trim() || "作者未設定"}
              </span>
              {photobook.creatorXId ? (
                <span className="ml-1 font-num text-ink-medium">
                  @{photobook.creatorXId}
                </span>
              ) : null}
            </span>
          </div>

          {/* PC: 3 col grid (left nav / main / right panel) / Mobile: stack */}
          <div className="grid grid-cols-1 gap-8 lg:grid-cols-[200px_1fr_300px] lg:items-start lg:gap-10">
            {/* 左 nav (PC のみ) */}
            <div className="hidden lg:block">
              <PageNavSidebar pages={photobook.pages} />
            </div>

            {/* main col */}
            <div className="space-y-12 sm:space-y-16">
              {photobook.pages.map((page, idx) => (
                <PageHero
                  key={idx}
                  page={page}
                  pageNumber={idx + 1}
                  layout={photobook.layout}
                  photoIndexBase={pageBases[idx]}
                />
              ))}
            </div>

            {/* 右 panel (PC のみ) */}
            <div className="hidden lg:block">
              <RightPanel
                photobook={photobook}
                shareUrl={shareUrl}
                shareText={shareText}
              />
            </div>
          </div>

          {/* Mobile only: ShareActions + 通報 + 作る CTA */}
          <div className="mt-12 space-y-6 lg:hidden">
            <section className="rounded-lg border border-divider-soft bg-surface p-5 shadow-sm">
              <h3 className="mb-3 flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
                <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
                Share
              </h3>
              <ShareActions shareUrl={shareUrl} shareText={shareText} />
            </section>

            <section className="rounded-lg border border-teal-100 bg-teal-50 p-5 text-center">
              <p className="text-xs leading-relaxed text-ink-medium">
                あなたも VRChat の写真を、こんなフォトブックにまとめてみませんか?
              </p>
              <Link
                href="/create"
                data-testid="viewer-create-cta-mobile"
                className="mt-3 inline-flex h-12 w-full items-center justify-center rounded-[10px] bg-brand-teal px-5 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover"
              >
                このフォトブックを作る
              </Link>
            </section>

            <p className="text-center text-xs text-ink-soft">
              <Link
                href={reportHref}
                data-testid="viewer-report-link-mobile"
                className="text-ink-medium underline hover:text-status-error"
              >
                このフォトブックを通報する
              </Link>
            </p>
          </div>
        </SensitiveGate>

        <PublicPageFooter
          extraSlot={
            <Link
              href={reportHref}
              className="text-sm text-ink-medium underline hover:text-ink-strong"
              data-testid="viewer-report-link"
            >
              このフォトブックを通報
            </Link>
          }
        />
      </main>
    </ViewerInteractionProvider>
  );
}
