// /p/dev-sample-coverless — Dev 専用の Viewer プレビュールート (cover 無しフォールバック)。
//
// 目的:
//   - 3 contrast pattern C (typography only) + simple layout + light variant の確認
//   - meta 無しの Casual photobook の見え方確認
//
// folder 命名: `_` 始まりの folder は Next.js private 扱いで route 除外されるため、
//   `dev-sample-coverless` に rename。
//
// 動作:
//   - process.env.NODE_ENV === "production" の時は notFound() で 404

import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { ViewerLayout } from "@/components/Viewer/ViewerLayout";
import { sampleCoverlessCasual } from "@/lib/__fixtures__/publicPhotobookSample";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Sample Coverless | VRC PhotoBook",
  robots: { index: false, follow: false },
};

export default function SampleCoverlessViewerPage() {
  if (process.env.NODE_ENV === "production") {
    notFound();
  }
  const photobook = sampleCoverlessCasual();
  return <ViewerLayout photobook={photobook} />;
}
