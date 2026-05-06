// Public Viewer の表紙 (Cover) コンポーネント。
//
// 採用元 (TESTImage 完成イメージ 最終調整版):
//   - Mobile (iPhone) cover_first 列 / Mobile light 列
//   - 表紙のコントラスト保証 3 パターン (A グラデーション / B 半透明パネル / C 画像なし)
//
// 設計判断 (v2 独自):
//   - 内部分岐でパターン A/B/C を決定し、外から制御 props を増やさない
//     (プロンプト §5「判定ロジックは Cover.tsx 内に閉じ込め」)
//   - パターン B (半透明パネル) は portfolio / world type と large layout で発動。
//     情報量が多い表紙では画像と分離してテキスト可読性を優先
//   - 「読む」CTA は cover_first のみ。anchor `#page-1` で PageHero (Stage 3) にジャンプ
//
// セキュリティ:
//   - presigned URL (cover.display.url) は <img src> に渡すのみ、console / data-* 不可
//   - photobookId は受け取らず slug 系メタも UI 露出させない (v2: viewer 全体で raw ID 不出)
//
// 制約:
//   - Server Component。フェッチしない、Cookie 触らない
//   - Tailwind のみ。font-serif は system Georgia / Hiragino Mincho fallback (Q2 確認)

import type { PublicPhotobook } from "@/lib/publicPhotobook";

type Props = {
  photobook: PublicPhotobook;
  variant: "cover_first" | "light";
};

type ContrastPattern = "A" | "B" | "C";

function determineContrastPattern(photobook: PublicPhotobook): ContrastPattern {
  if (!photobook.cover) {
    return "C";
  }
  // B: 情報量の多い type / 大型 layout は半透明パネルで可読性を優先
  if (
    photobook.type === "portfolio" ||
    photobook.type === "world" ||
    photobook.layout === "large"
  ) {
    return "B";
  }
  return "A";
}

function formatPublishedDate(iso: string): string {
  // 2026-04-29T12:00:00Z → 2026.04.29
  const m = iso.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!m) return iso;
  return `${m[1]}.${m[2]}.${m[3]}`;
}

function effectiveTitle(p: PublicPhotobook): string {
  // coverTitle 優先、無ければ photobook.title (creator が cover に別文言を設定可能)
  return p.coverTitle && p.coverTitle.trim() !== "" ? p.coverTitle : p.title;
}

export function Cover({ photobook, variant }: Props) {
  const pattern = determineContrastPattern(photobook);
  const title = effectiveTitle(photobook);
  const date = formatPublishedDate(photobook.publishedAt);

  const heightCls =
    variant === "cover_first"
      ? "min-h-[78vh] sm:min-h-[86vh]"
      : "min-h-[300px] sm:min-h-[440px]";

  return (
    <section
      data-testid="viewer-cover"
      data-cover-pattern={pattern}
      data-cover-variant={variant}
      className={`relative isolate w-full overflow-hidden ${heightCls}`}
    >
      {pattern === "C" ? (
        <CoverFallback
          title={title}
          description={photobook.description}
          creator={photobook.creatorDisplayName}
          creatorXId={photobook.creatorXId}
          date={date}
          variant={variant}
        />
      ) : (
        <CoverWithImage
          title={title}
          description={photobook.description}
          creator={photobook.creatorDisplayName}
          creatorXId={photobook.creatorXId}
          date={date}
          variant={variant}
          pattern={pattern}
          coverUrl={photobook.cover?.display.url ?? ""}
          coverWidth={photobook.cover?.display.width ?? 1600}
          coverHeight={photobook.cover?.display.height ?? 2400}
        />
      )}
    </section>
  );
}

type WithImageProps = {
  title: string;
  description?: string;
  creator: string;
  creatorXId?: string;
  date: string;
  variant: "cover_first" | "light";
  pattern: "A" | "B";
  coverUrl: string;
  coverWidth: number;
  coverHeight: number;
};

function CoverWithImage({
  title,
  description,
  creator,
  creatorXId,
  date,
  variant,
  pattern,
  coverUrl,
  coverWidth,
  coverHeight,
}: WithImageProps) {
  return (
    <>
      <img
        src={coverUrl}
        alt=""
        aria-hidden="true"
        width={coverWidth}
        height={coverHeight}
        loading="eager"
        decoding="async"
        className="absolute inset-0 h-full w-full object-cover"
      />
      {pattern === "A" ? (
        // A: 下から上に黒グラデーションを重ねてタイトル領域の可読性を確保
        <span
          aria-hidden="true"
          className="absolute inset-0 bg-gradient-to-t from-black/75 via-black/35 to-black/0"
        />
      ) : (
        // B: 全体に薄黒、タイトル枠は半透明パネルで分離
        <span
          aria-hidden="true"
          className="absolute inset-0 bg-black/40"
        />
      )}
      {pattern === "A" ? (
        <CoverTextBlockOnImage
          title={title}
          description={description}
          creator={creator}
          creatorXId={creatorXId}
          date={date}
          variant={variant}
        />
      ) : (
        <CoverPanel
          title={title}
          description={description}
          creator={creator}
          creatorXId={creatorXId}
          date={date}
          variant={variant}
        />
      )}
    </>
  );
}

type TextBlockProps = {
  title: string;
  description?: string;
  creator: string;
  creatorXId?: string;
  date: string;
  variant: "cover_first" | "light";
};

function CoverTextBlockOnImage({
  title,
  description,
  creator,
  creatorXId,
  date,
  variant,
}: TextBlockProps) {
  // パターン A: 画像下端寄りに白文字、cover_first は CTA を含む
  return (
    <div className="absolute inset-x-0 bottom-0 z-10 flex flex-col gap-3 px-6 pb-10 pt-16 text-white sm:gap-4 sm:px-12 sm:pb-16">
      <h1 className="max-w-3xl whitespace-pre-line font-serif text-3xl font-bold leading-[1.15] drop-shadow-md sm:text-5xl">
        {title}
      </h1>
      {description ? (
        <p className="max-w-2xl text-sm leading-relaxed text-white/85 sm:text-base">
          {description}
        </p>
      ) : null}
      <CoverByline
        creator={creator}
        creatorXId={creatorXId}
        date={date}
        tone="onImage"
      />
      {variant === "cover_first" ? (
        <a
          href="#page-1"
          data-testid="viewer-cover-cta"
          className="mt-3 inline-flex h-12 w-full items-center justify-center rounded-[10px] bg-white px-6 text-sm font-bold text-ink shadow-md transition-colors hover:bg-white/90 sm:mt-4 sm:w-auto sm:min-w-[200px]"
        >
          読む
        </a>
      ) : null}
    </div>
  );
}

function CoverPanel({
  title,
  description,
  creator,
  creatorXId,
  date,
  variant,
}: TextBlockProps) {
  // パターン B: 中央に半透明白パネル、ink 文字
  return (
    <div className="relative z-10 mx-auto flex h-full w-full max-w-screen-md items-center px-4 py-16 sm:px-6 sm:py-24">
      <div className="w-full rounded-lg border border-white/40 bg-white/85 p-6 shadow-xl backdrop-blur sm:p-10">
        <h1 className="whitespace-pre-line font-serif text-2xl font-bold leading-tight text-ink sm:text-4xl">
          {title}
        </h1>
        {description ? (
          <p className="mt-3 text-sm leading-relaxed text-ink-medium sm:text-base">
            {description}
          </p>
        ) : null}
        <div className="mt-4">
          <CoverByline
            creator={creator}
            creatorXId={creatorXId}
            date={date}
            tone="onPanel"
          />
        </div>
        {variant === "cover_first" ? (
          <a
            href="#page-1"
            data-testid="viewer-cover-cta"
            className="mt-5 inline-flex h-12 w-full items-center justify-center rounded-[10px] bg-brand-teal px-6 text-sm font-bold text-white shadow-md transition-colors hover:bg-brand-teal-hover sm:w-auto sm:min-w-[200px]"
          >
            読む
          </a>
        ) : null}
      </div>
    </div>
  );
}

type FallbackProps = {
  title: string;
  description?: string;
  creator: string;
  creatorXId?: string;
  date: string;
  variant: "cover_first" | "light";
};

function CoverFallback({
  title,
  description,
  creator,
  creatorXId,
  date,
  variant,
}: FallbackProps) {
  // パターン C: 画像なし。teal-50 → surface のグラデーション + タイポグラフィのみ
  return (
    <div className="relative h-full w-full bg-gradient-to-br from-teal-50 via-surface to-teal-100">
      <div className="mx-auto flex h-full w-full max-w-screen-md flex-col justify-end gap-3 px-6 pb-10 pt-16 sm:gap-4 sm:px-12 sm:pb-16">
        <span
          aria-hidden="true"
          className="block h-1 w-12 rounded-full bg-teal-500"
        />
        <h1 className="whitespace-pre-line font-serif text-3xl font-bold leading-tight text-ink sm:text-5xl">
          {title}
        </h1>
        {description ? (
          <p className="max-w-2xl text-sm leading-relaxed text-ink-medium sm:text-base">
            {description}
          </p>
        ) : null}
        <CoverByline
          creator={creator}
          creatorXId={creatorXId}
          date={date}
          tone="onSurface"
        />
        {variant === "cover_first" ? (
          <a
            href="#page-1"
            data-testid="viewer-cover-cta"
            className="mt-3 inline-flex h-12 w-full items-center justify-center rounded-[10px] bg-brand-teal px-6 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover sm:mt-4 sm:w-auto sm:min-w-[200px]"
          >
            読む
          </a>
        ) : null}
      </div>
    </div>
  );
}

type BylineProps = {
  creator: string;
  creatorXId?: string;
  date: string;
  tone: "onImage" | "onPanel" | "onSurface";
};

function CoverByline({ creator, creatorXId, date, tone }: BylineProps) {
  const nameCls =
    tone === "onImage" ? "text-white/90" : "text-ink-strong";
  const subCls =
    tone === "onImage" ? "text-white/70" : "text-ink-medium";
  const dateCls =
    tone === "onImage" ? "text-white/85" : "text-ink-medium";
  return (
    <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 text-xs sm:text-sm">
      <span className={`font-bold ${nameCls}`}>by {creator}</span>
      {creatorXId ? (
        <span className={`font-num ${subCls}`}>@{creatorXId}</span>
      ) : null}
      <span className={`font-num ${dateCls}`}>· {date}</span>
    </div>
  );
}
