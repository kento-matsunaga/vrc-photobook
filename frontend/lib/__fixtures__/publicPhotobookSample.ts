// Public Viewer の dev / test 用 sample データ。
//
// 用途:
//   - vitest で表示分岐 (cover 有無 / coverTitle 有無 / meta 有無 / layout 4 種) を確認
//   - dev 専用 route `/p/__sample__` で Backend 未拡張時の page_meta あり描画を確認
//
// 制約:
//   - 本ファイルは production bundle に混入しても安全 (純データのみ、Secret なし、
//     presigned URL は dummy 文字列で raw URL 形式)。ただし dev-only route では
//     middleware で production 時 404 にする
//   - photobookId は raw 内部識別子の体裁で出さない。slug は dev-only 識別子を使う

import type {
  PublicPhotobook,
  PublicVariantSet,
} from "@/lib/publicPhotobook";

const FAR_FUTURE_EXPIRES_AT = "2099-12-31T23:59:59Z";

function dummyVariant(slug: string, w: number, h: number): PublicVariantSet {
  return {
    display: {
      url: `https://images.example.invalid/${slug}.jpg`,
      width: w,
      height: h,
      expiresAt: FAR_FUTURE_EXPIRES_AT,
    },
    thumbnail: {
      url: `https://images.example.invalid/${slug}.thumb.jpg`,
      width: Math.round(w / 4),
      height: Math.round(h / 4),
      expiresAt: FAR_FUTURE_EXPIRES_AT,
    },
  };
}

/**
 * dev / test 用 sample (Sunset Memories)。
 *
 * - layout: "magazine" (PageHero の magazine 分岐確認用)
 * - openingStyle: "cover_first"
 * - 5 page 構成 (page 1-5)、各 page に meta + caption + 1〜4 photos
 * - cover 画像 + coverTitle あり (3 contrast pattern A グラデーション)
 * - type: "event" (TypeAccent event バッジ確認用)
 */
export function sampleSunsetMemories(): PublicPhotobook {
  return {
    photobookId: "sample-sunset-redacted",
    slug: "__sample__",
    type: "event",
    title: "Sunset Memories",
    description: "あの日の集い、忘れぬための記録",
    layout: "magazine",
    openingStyle: "cover_first",
    creatorDisplayName: "ERENOA",
    creatorXId: "Noa_Fortevita",
    coverTitle: "Sunset Memories",
    cover: dummyVariant("sample-cover", 1600, 2400),
    publishedAt: "2026-04-29T12:00:00Z",
    pages: [
      {
        caption: "屋上に集まったメンバー、夕焼けを背に。",
        photos: [
          {
            caption: "屋上の集合 (1/3)",
            variants: dummyVariant("sample-01", 1600, 1066),
          },
          {
            caption: "ENGINE が指差した先",
            variants: dummyVariant("sample-02", 1200, 1600),
          },
          {
            caption: "夕焼けと逆光",
            variants: dummyVariant("sample-03", 1200, 1600),
          },
        ],
        meta: {
          eventDate: "2026-04-29",
          world: "Sunset Rooftop",
          castList: ["@Noa_Fortevita", "@friend_a", "@friend_b"],
          photographer: "ERENOA",
          note: "ふと撮った 1 枚が、その日の空気をぜんぶ覚えていた。",
        },
      },
      {
        caption: "ストリート散策",
        photos: [
          {
            caption: "夜の路地",
            variants: dummyVariant("sample-04", 1200, 1600),
          },
          {
            caption: "ネオンの反射",
            variants: dummyVariant("sample-05", 1600, 1066),
          },
        ],
        meta: {
          eventDate: "2026-04-29",
          world: "Midnight Boulevard",
          castList: ["@Noa_Fortevita", "@friend_a"],
          photographer: "ERENOA",
        },
      },
      {
        caption: "ライブ会場の興奮",
        photos: [
          {
            caption: "ステージ全景",
            variants: dummyVariant("sample-06", 1600, 900),
          },
          {
            caption: "客席のサイリウム",
            variants: dummyVariant("sample-07", 1200, 1600),
          },
          {
            caption: "間奏のシャウト",
            variants: dummyVariant("sample-08", 1200, 1600),
          },
          {
            caption: "アンコール",
            variants: dummyVariant("sample-09", 1600, 1066),
          },
        ],
        meta: {
          eventDate: "2026-04-29",
          world: "Live Stage Hall",
          castList: ["@Noa_Fortevita", "@friend_a", "@friend_b", "@friend_c"],
          photographer: "ERENOA",
          note: "音と歓声で空気が震えていた。",
        },
      },
      {
        caption: "終演後、駅前にて",
        photos: [
          {
            caption: "改札前の別れ際",
            variants: dummyVariant("sample-10", 1200, 1600),
          },
        ],
      },
      {
        caption: "翌朝の余韻",
        photos: [
          {
            caption: "朝焼けの東京",
            variants: dummyVariant("sample-11", 1600, 1066),
          },
          {
            caption: "コーヒーと眠気",
            variants: dummyVariant("sample-12", 1200, 1600),
          },
        ],
        meta: {
          eventDate: "2026-04-30",
          note: "また会おうね、と誰も言わなかった。\nそれが、この日のいちばん良いところ。",
        },
      },
    ],
  };
}

/**
 * cover 無しフォールバック確認用 sample (3 contrast pattern C)。
 * type: "casual"、layout: "simple"、openingStyle: "light"。
 */
export function sampleCoverlessCasual(): PublicPhotobook {
  return {
    photobookId: "sample-coverless-redacted",
    slug: "__sample_coverless__",
    type: "casual",
    title: "おはツイまとめ 2026.04",
    description: undefined,
    layout: "simple",
    openingStyle: "light",
    creatorDisplayName: "ERENOA",
    creatorXId: "Noa_Fortevita",
    coverTitle: undefined,
    cover: undefined,
    publishedAt: "2026-04-30T08:00:00Z",
    pages: [
      {
        caption: "今朝の空",
        photos: [
          {
            caption: undefined,
            variants: dummyVariant("sample-cl-01", 1200, 1600),
          },
        ],
      },
      {
        caption: "コーヒーと寝起きの自撮り",
        photos: [
          {
            caption: undefined,
            variants: dummyVariant("sample-cl-02", 1200, 1600),
          },
          {
            caption: undefined,
            variants: dummyVariant("sample-cl-03", 1200, 1600),
          },
        ],
      },
    ],
  };
}
