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
 * SSR メタデータ。MVP は noindex 固定（OGP 本実装は PR33）。
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
    },
    twitter: {
      card: "summary",
      title,
      description,
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
