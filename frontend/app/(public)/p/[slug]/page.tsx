// /p/[slug] 公開 Viewer ページ。
//
// 設計参照:
//   - docs/plan/m2-public-viewer-and-manage-plan.md §3 / §10
//
// セキュリティ:
//   - status / hidden / private を区別せず 404 / 410 を出すのは Backend 側
//   - presigned URL を console.log しない
//   - manage URL / draft URL / token を画面に出さない
//   - 全ページ noindex（middleware + metadata 両方）

import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { ErrorState } from "@/components/ErrorState";
import { ViewerLayout } from "@/components/Viewer/ViewerLayout";
import {
  fetchPublicPhotobook,
  isPublicLookupError,
  type PublicPhotobook,
} from "@/lib/publicPhotobook";

export const dynamic = "force-dynamic";

type Params = Promise<{ slug: string }>;

/**
 * SSR メタデータ。MVP は noindex 固定で SNS preview 用 OGP のみ出す
 * （noindex と OGP は両立、検索 vs SNS の責務分離）。
 *
 * og:image は `https://app.vrc-photobook.com/ogp/<photobook_id>?v=<version>` を
 * 指す（Cloudflare Workers proxy が R2 から PNG を返す）。OGP 未生成 / 公開不可 /
 * lookup 失敗時は `/og/default.png` の絶対 URL に倒す。
 */
export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { slug } = await params;
  // 失敗時は最小メタを返す（slug 値を出さない）
  let pb: PublicPhotobook | null = null;
  try {
    pb = await fetchPublicPhotobook(slug);
  } catch {
    pb = null;
  }
  const appBaseUrl = process.env.NEXT_PUBLIC_BASE_URL ?? "https://app.vrc-photobook.com";
  const ogpURL = pb
    ? `${appBaseUrl}/ogp/${pb.photobookId}?v=1`
    : `${appBaseUrl}/og/default.png`;
  const publicURL = pb ? `${appBaseUrl}/p/${pb.slug}` : appBaseUrl;
  const title = pb?.title ?? "VRC PhotoBook";
  const description = pb?.description ?? "VRC PhotoBook（非公式ファンメイドサービス）";
  return {
    title,
    description,
    robots: { index: false, follow: false },
    openGraph: {
      title,
      description,
      type: "website",
      url: publicURL,
      images: [{ url: ogpURL, width: 1200, height: 630 }],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
      images: [ogpURL],
    },
  };
}

export default async function PublicViewerPage({
  params,
}: {
  params: Params;
}) {
  const { slug } = await params;
  let photobook: PublicPhotobook;
  try {
    photobook = await fetchPublicPhotobook(slug);
  } catch (e) {
    if (isPublicLookupError(e)) {
      switch (e.kind) {
        case "not_found":
          notFound();
        case "gone":
          return <ErrorState variant="gone" />;
        case "server_error":
        case "network":
          return <ErrorState variant="server_error" />;
      }
    }
    return <ErrorState variant="server_error" />;
  }

  return <ViewerLayout photobook={photobook} />;
}
