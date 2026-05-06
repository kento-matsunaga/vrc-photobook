// Public Viewer API client（Server-side fetch wrapper）。
//
// 設計参照:
//   - docs/plan/m2-public-viewer-and-manage-plan.md §6 / §12
//
// セキュリティ:
//   - presigned URL / R2 credentials / storage_key は console.log しない
//   - 失敗詳細は kind だけを返し、内容を画面に出さない（呼び出し元で UI 変換）

/** Backend のベース URL を取得する。 */
function getApiBaseUrl(): string {
  const url = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  if (url === "") {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return url.replace(/\/$/, "");
}

/** API のエラー種別。SSR / Client から呼ぶ際に画面ステータスへ変換する。 */
export type PublicLookupError =
  | { kind: "not_found" }
  | { kind: "gone" }
  | { kind: "server_error" }
  | { kind: "network" };

/** 1 写真分の variant URL set。 */
export type PublicVariantSet = {
  display: PublicPresignedURL;
  thumbnail: PublicPresignedURL;
};

/** Backend が返す短命 presigned URL。 */
export type PublicPresignedURL = {
  url: string;
  width: number;
  height: number;
  expiresAt: string;
};

/** ページ内の 1 写真。 */
export type PublicPhoto = {
  caption?: string;
  variants: PublicVariantSet;
};

/**
 * ページに紐づく任意メタ情報。photobook_page_metas（1 page につき 0..1）に対応。
 * Backend API が拡張されるまでは undefined のまま流れる。
 *
 * セキュリティ: world / cast / photographer / note は作成者入力の自由テキスト。
 * 表示時は React の自動エスケープに任せ、innerHTML で出さないこと。
 */
export type PublicPageMeta = {
  /** ISO 8601 日付（YYYY-MM-DD）。表示は viewer 側で整形 */
  eventDate?: string;
  world?: string;
  castList?: string[];
  photographer?: string;
  note?: string;
};

/** 1 ページ。 */
export type PublicPage = {
  caption?: string;
  photos: PublicPhoto[];
  /** 任意メタ情報。Backend 未拡張のうちは undefined */
  meta?: PublicPageMeta;
};

/** Public Viewer の photobook 全体。 */
export type PublicPhotobook = {
  /**
   * 公開識別子（業務知識 v4 §3.5）。OGP URL `/ogp/<photobookId>?v=<n>` の解決に使う。
   */
  photobookId: string;
  /** public_url_slug（公開済の場合のみ非空）。 */
  slug: string;
  type: string;
  title: string;
  description?: string;
  layout: string;
  openingStyle: string;
  creatorDisplayName: string;
  creatorXId?: string;
  coverTitle?: string;
  cover?: PublicVariantSet;
  publishedAt: string;
  pages: PublicPage[];
};

/**
 * GET /api/public/photobooks/{slug} を呼び出す。
 *
 * 失敗時は PublicLookupError を throw する（呼び出し元で notFound / gone / 500 に分岐）。
 * Cookie / credentials は付けない（public 経路）。
 */
export async function fetchPublicPhotobook(
  slug: string,
  signal?: AbortSignal,
): Promise<PublicPhotobook> {
  const url = `${getApiBaseUrl()}/api/public/photobooks/${encodeURIComponent(slug)}`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "GET",
      cache: "no-store",
      signal,
    });
  } catch {
    throw { kind: "network" } satisfies PublicLookupError;
  }

  if (res.status === 200) {
    const body = (await res.json()) as ApiPublicPhotobookPayload;
    return mapPublicPayload(body);
  }
  if (res.status === 404) {
    throw { kind: "not_found" } satisfies PublicLookupError;
  }
  if (res.status === 410) {
    throw { kind: "gone" } satisfies PublicLookupError;
  }
  throw { kind: "server_error" } satisfies PublicLookupError;
}

// === API レスポンス（snake_case）→ TS（camelCase）変換 ===

type ApiPresignedURL = {
  url: string;
  width: number;
  height: number;
  expires_at: string;
};

type ApiVariantSet = {
  display: ApiPresignedURL;
  thumbnail: ApiPresignedURL;
};

type ApiPhoto = {
  caption?: string;
  variants: ApiVariantSet;
};

type ApiPageMeta = {
  event_date?: string;
  world?: string;
  cast_list?: string[];
  photographer?: string;
  note?: string;
};

type ApiPage = {
  caption?: string;
  photos: ApiPhoto[];
  meta?: ApiPageMeta;
};

type ApiPublicPhotobookPayload = {
  photobook_id: string;
  slug: string;
  type: string;
  title: string;
  description?: string;
  layout: string;
  opening_style: string;
  creator_display_name: string;
  creator_x_id?: string;
  cover_title?: string;
  cover?: ApiVariantSet;
  published_at: string;
  pages: ApiPage[];
};

function mapPresignedURL(p: ApiPresignedURL): PublicPresignedURL {
  return {
    url: p.url,
    width: p.width,
    height: p.height,
    expiresAt: p.expires_at,
  };
}

function mapVariantSet(v: ApiVariantSet): PublicVariantSet {
  return {
    display: mapPresignedURL(v.display),
    thumbnail: mapPresignedURL(v.thumbnail),
  };
}

function mapPublicPayload(p: ApiPublicPhotobookPayload): PublicPhotobook {
  return {
    photobookId: p.photobook_id,
    slug: p.slug,
    type: p.type,
    title: p.title,
    description: p.description,
    layout: p.layout,
    openingStyle: p.opening_style,
    creatorDisplayName: p.creator_display_name,
    creatorXId: p.creator_x_id,
    coverTitle: p.cover_title,
    cover: p.cover ? mapVariantSet(p.cover) : undefined,
    publishedAt: p.published_at,
    pages: p.pages.map((page) => ({
      caption: page.caption,
      photos: page.photos.map((photo) => ({
        caption: photo.caption,
        variants: mapVariantSet(photo.variants),
      })),
      meta: page.meta ? mapPageMeta(page.meta) : undefined,
    })),
  };
}

function mapPageMeta(m: ApiPageMeta): PublicPageMeta {
  return {
    eventDate: m.event_date,
    world: m.world,
    castList: m.cast_list,
    photographer: m.photographer,
    note: m.note,
  };
}

/** Public 経路エラーが PublicLookupError 形か判定する型ガード。 */
export function isPublicLookupError(e: unknown): e is PublicLookupError {
  return typeof e === "object" && e !== null && "kind" in e;
}
