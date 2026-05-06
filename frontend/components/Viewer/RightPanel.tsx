// RightPanel: PC 用の右サイドバー (About / Creator / Share / 作る CTA / 通報)。
//
// 採用元: TESTImage 完成イメージ「PC (デスクトップ)」右コラム
//
// 設計判断 (v2):
//   - Server Component。中の ShareActions のみ Client (内部で "use client")
//   - sticky top で縦スクロール中も見える
//   - Mobile では使わない (ViewerLayout 側で sm:hidden または別配置)
//
// セキュリティ:
//   - shareUrl は viewer URL のみ、raw photobook_id 含まない
//   - 通報リンクは /p/{slug}/report

import Link from "next/link";

import type { PublicPhotobook } from "@/lib/publicPhotobook";
import { ShareActions } from "@/components/Viewer/ShareActions";
import { TypeAccent } from "@/components/Viewer/TypeAccent";

type Props = {
  photobook: PublicPhotobook;
  shareUrl: string;
  shareText: string;
};

function formatPublishedDate(iso: string): string {
  const m = iso.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!m) return iso;
  return `${m[1]}.${m[2]}.${m[3]}`;
}

export function RightPanel({ photobook, shareUrl, shareText }: Props) {
  const date = formatPublishedDate(photobook.publishedAt);
  const reportHref = `/p/${photobook.slug}/report`;

  return (
    <aside
      data-testid="viewer-right-panel"
      className="sticky top-[88px] flex flex-col gap-5 self-start"
    >
      {/* About this photobook */}
      <section className="rounded-lg border border-divider-soft bg-surface p-5 shadow-sm">
        <h3 className="mb-3 flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
          <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
          About this photobook
        </h3>
        <h4 className="font-serif text-lg font-bold text-ink">{photobook.title}</h4>
        {photobook.description ? (
          <p className="mt-1 text-xs leading-relaxed text-ink-medium">
            {photobook.description}
          </p>
        ) : null}
        <div className="mt-3 flex flex-wrap gap-2">
          <TypeAccent type={photobook.type} />
        </div>
      </section>

      {/* Creator card */}
      <section
        data-testid="viewer-creator-card"
        className="rounded-lg border border-divider-soft bg-surface p-5 shadow-sm"
      >
        <h3 className="mb-3 flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
          <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
          Creator
        </h3>
        <div className="flex items-center gap-3">
          <span
            aria-hidden="true"
            className="grid h-10 w-10 shrink-0 place-items-center rounded-full border border-teal-200 bg-teal-50 font-serif text-base font-bold text-teal-700"
          >
            {photobook.creatorDisplayName.slice(0, 1)}
          </span>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-bold text-ink">{photobook.creatorDisplayName}</p>
            {photobook.creatorXId ? (
              <a
                href={`https://x.com/${encodeURIComponent(photobook.creatorXId)}`}
                rel="noopener noreferrer"
                target="_blank"
                className="font-num text-xs text-teal-600 underline hover:text-teal-700"
              >
                @{photobook.creatorXId}
              </a>
            ) : null}
          </div>
        </div>
        <p className="mt-3 text-[11px] text-ink-soft">
          公開日 <span className="font-num">{date}</span>
        </p>
      </section>

      {/* Share */}
      <section className="rounded-lg border border-divider-soft bg-surface p-5 shadow-sm">
        <h3 className="mb-3 flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
          <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
          Share
        </h3>
        <ShareActions shareUrl={shareUrl} shareText={shareText} />
      </section>

      {/* CTA: 自分でも作る */}
      <section className="rounded-lg border border-teal-100 bg-teal-50 p-5">
        <p className="text-xs leading-relaxed text-ink-medium">
          あなたも VRChat の写真を、こんなフォトブックにまとめてみませんか?
        </p>
        <Link
          href="/create"
          data-testid="viewer-create-cta-pc"
          className="mt-3 inline-flex h-11 w-full items-center justify-center rounded-[10px] bg-brand-teal px-5 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover"
        >
          このフォトブックを作る
        </Link>
      </section>

      {/* 通報リンク */}
      <p className="text-center text-[11px] text-ink-soft">
        <Link
          href={reportHref}
          data-testid="viewer-report-link-pc"
          className="text-ink-medium underline hover:text-status-error"
        >
          このフォトブックを通報する
        </Link>
      </p>
    </aside>
  );
}
