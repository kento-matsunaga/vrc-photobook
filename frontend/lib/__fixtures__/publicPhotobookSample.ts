// 開発・テスト確認用の PublicPhotobook サンプル。
//
// 用途:
//   - Viewer コンポーネントの SSR 文字列化テストでの input
//   - ローカルでデザインを目視確認したいときの mock データ
//
// セキュリティ:
//   - 本ファイルに本番の photobook_id / token / Cookie / Secret を書かない
//   - presigned URL は dummy（example.com）。本番では短命 R2 URL に差し替わる
//   - DB 接続なし。純粋に in-memory のサンプル

import type {
  PublicPhotobook,
  PublicPresignedURL,
  PublicVariantSet,
} from "@/lib/publicPhotobook";

const FAR_FUTURE = "2099-12-31T23:59:59Z";

function url(path: string, w: number, h: number): PublicPresignedURL {
  return {
    url: `https://example.com/${path}`,
    width: w,
    height: h,
    expiresAt: FAR_FUTURE,
  };
}

function variants(path: string, w: number, h: number): PublicVariantSet {
  return {
    display: url(`${path}/display.webp`, w, h),
    thumbnail: url(`${path}/thumbnail.webp`, Math.round(w / 4), Math.round(h / 4)),
  };
}

/**
 * デザイン画像「Sunset Memories」相当のサンプル。
 * - type: memory
 * - layout: magazine
 * - openingStyle: cover_first
 * - 12 ページ程度、page_meta あり
 */
export function sampleSunsetMemories(): PublicPhotobook {
  return {
    photobookId: "00000000-0000-0000-0000-000000000001",
    slug: "sample-sunset-memories",
    type: "memory",
    title: "Sunset Memories",
    description: "あの日見た、夕暮れの記憶",
    layout: "magazine",
    openingStyle: "cover_first",
    creatorDisplayName: "すずきさん",
    creatorXId: "suzuki_vrc",
    coverTitle: "Sunset Memories",
    cover: variants("sample/cover", 1600, 1200),
    publishedAt: "2025-05-12T10:00:00Z",
    pages: [
      {
        caption:
          "みんなで夕暮れの港町を散歩。きれいな景色に、思わず立ち止まってしまった。サクラが撮ってくれた一枚が特にお気に入り。",
        meta: {
          eventDate: "2025-05-03",
          world: "夕暮れの港町",
          castList: ["サクラ", "ミナ", "ルカ"],
          photographer: "すずきさん",
          note: "このワールドの夕焼け、本当に綺麗で何度でも来たい。またみんなで来ようね！",
        },
        photos: [
          { variants: variants("sample/p1/photo1", 1600, 900) },
          { variants: variants("sample/p1/photo2", 800, 800) },
          { variants: variants("sample/p1/photo3", 800, 800) },
          { variants: variants("sample/p1/photo4", 800, 800) },
          { variants: variants("sample/p1/photo5", 800, 800) },
        ],
      },
      {
        caption: "夜はキャンプ場で、焚き火を囲んで、たくさん話して笑って。こういう時間って、いちばん大切かも。",
        meta: {
          eventDate: "2025-05-02",
          world: "星降るキャンプ場",
          castList: ["ルカ", "ノノ", "ミオ"],
          photographer: "すずきさん",
        },
        photos: [
          { variants: variants("sample/p2/photo1", 1600, 1000) },
          { variants: variants("sample/p2/photo2", 800, 800) },
          { variants: variants("sample/p2/photo3", 800, 800) },
          { variants: variants("sample/p2/photo4", 800, 800) },
        ],
      },
      {
        caption: "ファンタジーホールにて、みんなで集合写真。",
        meta: {
          eventDate: "2025-05-02",
          world: "ファンタジーホール",
          castList: ["みんな"],
        },
        photos: [
          { variants: variants("sample/p3/photo1", 1600, 1100) },
          { variants: variants("sample/p3/photo2", 800, 800) },
          { variants: variants("sample/p3/photo3", 800, 800) },
        ],
      },
    ],
  };
}

/** type / layout / openingStyle を入れ替えた variant を返す */
export function sampleWithVariant(
  overrides: Partial<
    Pick<PublicPhotobook, "type" | "layout" | "openingStyle" | "cover" | "coverTitle" | "creatorXId">
  >,
): PublicPhotobook {
  return { ...sampleSunsetMemories(), ...overrides };
}
