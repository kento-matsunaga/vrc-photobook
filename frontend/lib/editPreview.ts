// EditView -> PublicPhotobook 変換 helper (PreviewPane 用、STOP P-4)。
//
// 設計参照:
//   - docs/plan/m2-edit-page-split-and-preview-plan.md §6.9 / §7.4
//
// 用途:
//   /edit 画面の preview toggle で、現在の draft EditView をそのまま PreviewPane に
//   流し込むため、PublicPhotobook 形式 (公開 Viewer が期待する型) に変換する。
//
// 制約:
//   - draft session には creator_display_name / creator_x_id が無い。プレビュー用に
//     固定文言を入れる (OQ5 確定、publish 時に Backend が creator から取得して上書き)。
//   - Phase A は page_meta 未対応 (OQ4 / Phase B で対応)。常に undefined。
//   - slug は固定文字列で「本物の公開 slug」を持たない (OGP / public URL の解決には使えない)。
//   - publishedAt は変換時刻 (now) を入れる。draft では publish が確定していないため。
//
// 型互換性:
//   - EditPresignedURL <-> PublicPresignedURL: 形式互換 (`{url, width, height, expiresAt}`)
//   - EditVariantSet  <-> PublicVariantSet : 形式互換 (`{display, thumbnail}`)
//   - EditPhoto.caption / EditPage.caption は PublicPhoto / PublicPage と互換 (optional string)
//
// セキュリティ:
//   - raw photobook_id / image_id 等を新たに露出させない (EditView に既に含まれる ID を
//     PublicPhotobook の photobookId に渡すのみ、UI 表示で raw を出さないのは Viewer 側責務)。

import type { EditView } from "@/lib/editPhotobook";
import type { PublicPhotobook } from "@/lib/publicPhotobook";

/** preview 表示用の固定 slug。本物の公開 slug と区別するため明確な値にする。 */
export const PREVIEW_SLUG = "draft-preview";

/** preview 中の creator display name 固定文言。
 *  publish 時に Backend が photobook.creator から取得して PublicPhotobook の
 *  creator_display_name に入れるため、draft / preview では入らない (OQ5)。 */
export const PREVIEW_CREATOR_DISPLAY_NAME = "プレビュー (公開時に creator 名)";

/** title 未入力時の暫定表示文言。 */
export const PREVIEW_FALLBACK_TITLE = "(タイトル未設定)";

/**
 * EditView を PublicPhotobook 相当に変換する (PreviewPane 用)。
 *
 * @param v 現在の EditView (server ground truth)
 * @param now 変換時刻 (publishedAt placeholder、test では固定 ISO 文字列を渡せるよう注入)
 * @returns PublicPhotobook 形式の preview データ
 */
export function editViewToPreview(v: EditView, now: Date = new Date()): PublicPhotobook {
  return {
    photobookId: v.photobookId,
    slug: PREVIEW_SLUG,
    type: v.settings.type,
    title: v.settings.title || PREVIEW_FALLBACK_TITLE,
    description: v.settings.description,
    layout: v.settings.layout,
    openingStyle: v.settings.openingStyle,
    creatorDisplayName: PREVIEW_CREATOR_DISPLAY_NAME,
    creatorXId: undefined,
    coverTitle: v.settings.coverTitle,
    cover: v.cover,
    publishedAt: now.toISOString(),
    pages: v.pages.map((p) => ({
      caption: p.caption,
      photos: p.photos.map((ph) => ({
        caption: ph.caption,
        variants: ph.variants,
      })),
      meta: undefined,
    })),
  };
}
