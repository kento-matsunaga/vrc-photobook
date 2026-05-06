// Public Viewer の dev / test 用 sample データ。
//
// 用途:
//   - vitest で表示分岐 (cover 有無 / coverTitle 有無 / meta 有無 / layout 4 種) を確認
//   - dev 専用 route `/p/__sample__` で Backend 未拡張時の page_meta あり描画を確認
//
// 画像参照:
//   - frontend/scripts/build-sample-images.sh で TESTImage/ 配下の VRChat 実写 PNG を
//     圧縮して frontend/public/img/sample/ に配置 (cover P 1200×1800 / sample 01-07 L 1600×900)
//   - 本 fixture は `/img/sample/<stable>.jpg` (display) / `<stable>.thumb.jpg` (thumbnail)
//     を参照する。Cloudflare Workers ASSETS 経由で配信
//
// 制約:
//   - 本ファイルは production bundle に混入しても安全 (純データのみ、Secret なし)。ただし
//     dev-only route 側は process.env.NODE_ENV === "production" で notFound に倒す
//   - photobookId は raw 内部識別子の体裁で出さない。slug は dev-only 識別子を使う

import type {
  PublicPhotobook,
  PublicVariantSet,
} from "@/lib/publicPhotobook";

const FAR_FUTURE_EXPIRES_AT = "2099-12-31T23:59:59Z";

// 画像 stable name は frontend/scripts/build-sample-images.sh の MAPPING と一致させる
type SampleSpec = {
  slug: string;
  displayW: number;
  displayH: number;
  thumbW: number;
  thumbH: number;
};

const COVER_SPEC: SampleSpec = {
  slug: "sample-cover",
  displayW: 1200,
  displayH: 1800,
  thumbW: 300,
  thumbH: 450,
};

const LANDSCAPE_SPEC = (slug: string): SampleSpec => ({
  slug,
  displayW: 1600,
  displayH: 900,
  thumbW: 480,
  thumbH: 270,
});

function variantOf(spec: SampleSpec): PublicVariantSet {
  return {
    display: {
      url: `/img/sample/${spec.slug}.jpg`,
      width: spec.displayW,
      height: spec.displayH,
      expiresAt: FAR_FUTURE_EXPIRES_AT,
    },
    thumbnail: {
      url: `/img/sample/${spec.slug}.thumb.jpg`,
      width: spec.thumbW,
      height: spec.thumbH,
      expiresAt: FAR_FUTURE_EXPIRES_AT,
    },
  };
}

const COVER = variantOf(COVER_SPEC);
const SAMPLE_01 = variantOf(LANDSCAPE_SPEC("sample-01"));
const SAMPLE_02 = variantOf(LANDSCAPE_SPEC("sample-02"));
const SAMPLE_03 = variantOf(LANDSCAPE_SPEC("sample-03"));
const SAMPLE_04 = variantOf(LANDSCAPE_SPEC("sample-04"));
const SAMPLE_05 = variantOf(LANDSCAPE_SPEC("sample-05"));
const SAMPLE_06 = variantOf(LANDSCAPE_SPEC("sample-06"));
const SAMPLE_07 = variantOf(LANDSCAPE_SPEC("sample-07"));

/**
 * dev / test 用 sample (Sunset Memories)。
 *
 * - layout: "magazine" (PageHero の magazine 分岐確認用)
 * - openingStyle: "cover_first"
 * - 5 page 構成 (page 1-5)、各 page に meta + caption + 1〜4 photos
 * - cover 画像 + coverTitle あり (3 contrast pattern A グラデーション)
 * - type: "event" (TypeAccent event バッジ確認用)
 *
 * 画像 (TESTImage 由来):
 *   cover : 82E37915-... 縦長 portrait
 *   photos: VRChat_2026-04-29_*.png 7 枚 landscape (sample-01..07)
 */
export function sampleSunsetMemories(): PublicPhotobook {
  return {
    photobookId: "sample-sunset-redacted",
    slug: "dev-sample",
    type: "event",
    title: "Sunset Memories",
    description: "あの日の集い、忘れぬための記録",
    layout: "magazine",
    openingStyle: "cover_first",
    creatorDisplayName: "ERENOA",
    creatorXId: "Noa_Fortevita",
    coverTitle: "Sunset Memories",
    cover: COVER,
    publishedAt: "2026-04-29T12:00:00Z",
    pages: [
      {
        caption: "屋上に集まったメンバー、夕焼けを背に。",
        photos: [
          { caption: "屋上の集合", variants: SAMPLE_01 },
          { caption: "ENGINE が指差した先", variants: SAMPLE_02 },
          { caption: "夕焼けと逆光", variants: SAMPLE_03 },
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
          { caption: "夜の路地", variants: SAMPLE_04 },
          { caption: "ネオンの反射", variants: SAMPLE_05 },
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
          { caption: "ステージ全景", variants: SAMPLE_06 },
          { caption: "客席のサイリウム", variants: SAMPLE_07 },
          { caption: "間奏のシャウト", variants: SAMPLE_03 },
          { caption: "アンコール", variants: SAMPLE_01 },
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
          { caption: "改札前の別れ際", variants: SAMPLE_04 },
        ],
      },
      {
        caption: "翌朝の余韻",
        photos: [
          { caption: "朝焼けの東京", variants: SAMPLE_02 },
          { caption: "コーヒーと眠気", variants: SAMPLE_07 },
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
    slug: "dev-sample-coverless",
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
          { caption: undefined, variants: SAMPLE_05 },
        ],
      },
      {
        caption: "コーヒーと寝起きの自撮り",
        photos: [
          { caption: undefined, variants: SAMPLE_06 },
          { caption: undefined, variants: SAMPLE_07 },
        ],
      },
    ],
  };
}
