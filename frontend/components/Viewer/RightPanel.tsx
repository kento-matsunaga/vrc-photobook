// RightPanel: PC 右サイドバー。
//
// デザイン参照:
//   - design 最終調整版 §2 PC 右サイドバー
//
// 構造:
//   - About this photobook
//   - Title + description
//   - Creator card
//   - 公開日
//   - Share アクション (Client)
//   - 「あなたの写真も、1 冊のフォトブックに」CTA
//   - 通報リンク
//
// セキュリティ:
//   - photobook_id を DOM に出さない（slug のみで導線生成）
//   - 管理 URL / draft URL は出さない

import Link from "next/link";

import type { PublicPhotobook } from "@/lib/publicPhotobook";

import { ShareActions } from "./ShareActions";

type Props = {
  photobook: PublicPhotobook;
  /** ページ末尾の作る CTA を別途出す場合は false にできる */
  showMakeCta?: boolean;
};

export function RightPanel({ photobook, showMakeCta = true }: Props) {
  const creatorDisplayName = photobook.creatorDisplayName.trim() || "作者未設定";
  const creatorInitial = creatorDisplayName.slice(0, 1).toUpperCase();
  const publishedAt = formatDate(photobook.publishedAt);
  const shareUrl = buildShareUrl(photobook.slug);
  const shareText = `${photobook.title} / by ${creatorDisplayName}`;

  return (
    <aside className="space-y-3" data-testid="viewer-right-panel">
      {/* About */}
      <section className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm">
        <p className="mb-2 text-[10px] font-bold tracking-[0.08em] text-ink-medium">
          ABOUT THIS PHOTOBOOK
        </p>
        <h2 className="font-serif text-lg font-bold leading-tight text-ink">
          {photobook.title}
        </h2>
        {photobook.description && (
          <p className="mt-1 text-xs leading-relaxed text-ink-medium">
            {photobook.description}
          </p>
        )}

        <div className="mt-4 flex items-center gap-3 border-t border-divider-soft pt-4">
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
        {publishedAt && (
          <p className="mt-3 text-[11px] text-ink-medium">公開日 {publishedAt}</p>
        )}
      </section>

      {/* Share */}
      <section className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm">
        <p className="mb-3 text-[10px] font-bold tracking-[0.08em] text-ink-medium">
          SHARE
        </p>
        <ShareActions shareUrl={shareUrl} shareText={shareText} />
      </section>

      {/* Make CTA */}
      {showMakeCta && (
        <section
          className="rounded-lg border border-amber-200 bg-amber-50/70 p-4 shadow-sm"
          data-testid="viewer-make-cta-panel"
        >
          <p className="text-sm font-bold text-ink">あなたの写真も、</p>
          <p className="text-sm font-bold text-ink">1 冊のフォトブックに。</p>
          <p className="mt-1 text-[11px] leading-relaxed text-ink-medium">
            無料でかんたんに作れます。
          </p>
          <Link
            href="/create"
            className="mt-3 flex items-center justify-center gap-2 rounded-lg bg-teal-500 px-4 py-2 text-sm font-bold text-white transition-colors hover:bg-teal-600"
          >
            このフォトブックを作る
            <span aria-hidden="true">→</span>
          </Link>
        </section>
      )}

      {/* Report */}
      <section className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm">
        <p className="text-xs font-semibold text-ink-strong">
          問題がある場合はこちら
        </p>
        <Link
          href={`/p/${photobook.slug}/report`}
          className="mt-2 inline-flex items-center gap-1 text-sm font-semibold text-ink-medium hover:text-teal-700"
          data-testid="viewer-report-link-pc"
        >
          通報する
          <span aria-hidden="true">🚩</span>
        </Link>
      </section>
    </aside>
  );
}

function buildShareUrl(slug: string): string {
  const base =
    typeof process !== "undefined" && process.env.NEXT_PUBLIC_BASE_URL
      ? process.env.NEXT_PUBLIC_BASE_URL
      : "https://app.vrc-photobook.com";
  return `${base.replace(/\/$/, "")}/p/${slug}`;
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const yyyy = d.getUTCFullYear();
  const mm = String(d.getUTCMonth() + 1).padStart(2, "0");
  const dd = String(d.getUTCDate()).padStart(2, "0");
  return `${yyyy}.${mm}.${dd}`;
}
