// Cover: 公開ビューアの表紙。
//
// デザイン参照:
//   - design 最終調整版 §1 Mobile cover_first / 軽め(light)
//   - design 最終調整版「表紙のコントラスト保証 (3 パターン)」
//
// 3 パターンのコントラスト保証:
//   A. グラデーション : cover 画像下部に黒の linear gradient overlay を必ず重ねる
//   B. 半透明パネル   : 画像とは別の半透明黒パネルにタイトルを置く
//                       (portfolio / world / large レイアウト等、写真主役を強める用途)
//   C. フォールバック : cover 画像が無いとき。タイポのみの紙質背景
//
// 2 variant:
//   - cover_first : 表紙ファーストビュー。画面いっぱいの画像 + overlay + 「読む」ボタン
//   - light       : 軽め。小さい header カードでタイトル + creator + 公開日のみ
//
// セキュリティ:
//   - photobookId / token / presigned URL を console / data-attr に出さない
//   - title / description / coverTitle / creator は React の自動エスケープに任せる
//   - innerHTML / dangerouslySetInnerHTML は使わない

import type { PublicPhotobook } from "@/lib/publicPhotobook";

type Variant = "cover_first" | "light";
type ContrastPattern = "gradient" | "panel" | "fallback";

type Props = {
  photobook: PublicPhotobook;
  /** "cover_first"（表紙ファーストビュー）/ "light"（軽め） */
  variant: Variant;
  /**
   * コントラスト保証パターン。省略時は photobook.cover の有無 + layout から自動判定:
   *   - cover なし                                    → "fallback"
   *   - layout=large or type=portfolio/world/avatar    → "panel"
   *   - それ以外                                       → "gradient"
   */
  pattern?: ContrastPattern;
};

export function Cover({ photobook, variant, pattern }: Props) {
  const resolvedPattern = pattern ?? autoPattern(photobook);
  const coverTitle = photobook.coverTitle?.trim() || photobook.title;
  const description = photobook.description;
  const creatorDisplayName = photobook.creatorDisplayName.trim() || "作者未設定";
  const creatorXId = photobook.creatorXId;
  const publishedAt = formatPublishedDate(photobook.publishedAt);

  if (variant === "light") {
    return (
      <CoverLight
        photobook={photobook}
        coverTitle={coverTitle}
        description={description}
        creatorDisplayName={creatorDisplayName}
        creatorXId={creatorXId}
        publishedAt={publishedAt}
        pattern={resolvedPattern}
      />
    );
  }
  return (
    <CoverFirst
      photobook={photobook}
      coverTitle={coverTitle}
      description={description}
      creatorDisplayName={creatorDisplayName}
      creatorXId={creatorXId}
      publishedAt={publishedAt}
      pattern={resolvedPattern}
    />
  );
}

type InnerProps = {
  photobook: PublicPhotobook;
  coverTitle: string;
  description?: string;
  creatorDisplayName: string;
  creatorXId?: string;
  publishedAt: string;
  pattern: ContrastPattern;
};

/** cover_first: 表紙ファーストビュー */
function CoverFirst({
  photobook,
  coverTitle,
  description,
  creatorDisplayName,
  creatorXId,
  publishedAt,
  pattern,
}: InnerProps) {
  const cover = photobook.cover;
  if (!cover || pattern === "fallback") {
    return (
      <CoverFallback
        coverTitle={coverTitle}
        description={description}
        creatorDisplayName={creatorDisplayName}
        creatorXId={creatorXId}
        publishedAt={publishedAt}
      />
    );
  }

  return (
    <section
      className="relative isolate overflow-hidden rounded-lg shadow-sm"
      data-testid="viewer-cover"
      data-cover-variant="cover_first"
      data-cover-pattern={pattern}
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={cover.display.url}
        alt=""
        width={cover.display.width}
        height={cover.display.height}
        loading="eager"
        decoding="async"
        className="block h-auto w-full"
      />

      {/* A. グラデーション: 画像下部に黒の linear-gradient overlay */}
      {pattern === "gradient" && (
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/75 via-black/30 to-transparent"
        />
      )}

      {/* B. 半透明パネル: 画像中央〜下部に独立パネル */}
      {pattern === "panel" && (
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 flex items-end"
        >
          <div className="m-4 w-full rounded-lg bg-black/55 backdrop-blur-sm sm:m-6" />
        </div>
      )}

      <div className="absolute inset-0 flex items-end">
        <div className="w-full px-5 pb-6 text-white sm:px-8 sm:pb-10">
          <h1 className="font-serif text-3xl font-bold leading-tight tracking-tight drop-shadow-sm sm:text-4xl">
            {coverTitle}
          </h1>
          {description && (
            <p className="mt-2 text-sm leading-relaxed text-white/90 sm:text-base">
              {description}
            </p>
          )}
          <p className="mt-4 text-sm font-medium text-white/95">
            <span>by {creatorDisplayName}</span>
            {creatorXId && (
              <span className="ml-2 font-num text-xs text-white/80">
                @{creatorXId}
              </span>
            )}
          </p>
          <p className="mt-1 text-[11px] text-white/75">公開日 {publishedAt}</p>

          <a
            href="#page-1"
            className="mt-5 inline-flex items-center gap-2 rounded-full bg-white/95 px-5 py-2 text-xs font-bold text-ink shadow-sm transition-colors hover:bg-white"
            data-testid="viewer-cover-read-cta"
          >
            <span aria-hidden="true">↓</span>
            読む
          </a>
        </div>
      </div>
    </section>
  );
}

/** light: 軽めの header カード */
function CoverLight({
  photobook,
  coverTitle,
  description,
  creatorDisplayName,
  creatorXId,
  publishedAt,
  pattern,
}: InnerProps) {
  const cover = photobook.cover;
  // light は基本的にカード型。pattern=fallback の場合は画像なし、それ以外は cover の縦長サムネ的扱い
  return (
    <section
      className="rounded-lg border border-divider-soft bg-surface p-4 shadow-sm sm:p-6"
      data-testid="viewer-cover"
      data-cover-variant="light"
      data-cover-pattern={pattern}
    >
      <div className="flex items-start gap-4">
        {cover && pattern !== "fallback" && (
          <div className="relative w-24 shrink-0 overflow-hidden rounded sm:w-32">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={cover.thumbnail.url}
              alt=""
              width={cover.thumbnail.width}
              height={cover.thumbnail.height}
              loading="eager"
              decoding="async"
              className="block h-auto w-full"
            />
          </div>
        )}
        <div className="min-w-0 flex-1">
          <h1 className="font-serif text-2xl font-bold leading-tight tracking-tight text-ink sm:text-3xl">
            {coverTitle}
          </h1>
          {description && (
            <p className="mt-2 text-sm leading-relaxed text-ink-strong">
              {description}
            </p>
          )}
          <p className="mt-3 text-sm font-medium text-ink">
            <span>by {creatorDisplayName}</span>
            {creatorXId && (
              <span className="ml-2 font-num text-xs text-ink-medium">
                @{creatorXId}
              </span>
            )}
          </p>
          <p className="mt-1 text-[11px] text-ink-medium">公開日 {publishedAt}</p>
        </div>
      </div>
    </section>
  );
}

/** C. フォールバック: cover 画像なしのタイポのみ表紙 */
function CoverFallback({
  coverTitle,
  description,
  creatorDisplayName,
  creatorXId,
  publishedAt,
}: Omit<InnerProps, "photobook" | "pattern">) {
  return (
    <section
      className="rounded-lg border border-divider-soft bg-gradient-to-br from-surface-soft via-surface to-surface-soft p-8 text-center shadow-sm sm:p-12"
      data-testid="viewer-cover"
      data-cover-variant="cover_first"
      data-cover-pattern="fallback"
    >
      <span aria-hidden="true" className="mb-4 inline-block text-2xl">📖</span>
      <h1 className="font-serif text-3xl font-bold leading-tight tracking-tight text-ink sm:text-4xl">
        {coverTitle}
      </h1>
      {description && (
        <p className="mt-3 text-sm leading-relaxed text-ink-strong sm:text-base">
          {description}
        </p>
      )}
      <p className="mt-5 text-sm font-medium text-ink">
        <span>by {creatorDisplayName}</span>
        {creatorXId && (
          <span className="ml-2 font-num text-xs text-ink-medium">
            @{creatorXId}
          </span>
        )}
      </p>
      <p className="mt-1 text-[11px] text-ink-medium">公開日 {publishedAt}</p>
    </section>
  );
}

function autoPattern(photobook: PublicPhotobook): ContrastPattern {
  if (!photobook.cover) return "fallback";
  if (
    photobook.layout === "large" ||
    photobook.type === "portfolio" ||
    photobook.type === "world" ||
    photobook.type === "avatar"
  ) {
    return "panel";
  }
  return "gradient";
}

/** ISO 8601 → "YYYY.MM.DD" 表示。invalid 入力は空文字 */
function formatPublishedDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const yyyy = d.getUTCFullYear();
  const mm = String(d.getUTCMonth() + 1).padStart(2, "0");
  const dd = String(d.getUTCDate()).padStart(2, "0");
  return `${yyyy}.${mm}.${dd}`;
}
