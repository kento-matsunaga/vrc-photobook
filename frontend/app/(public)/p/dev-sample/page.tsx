// /p/dev-sample — Dev 専用の Viewer プレビュールート (page_meta あり)。
//
// folder 命名に関する注記:
//   - Next.js App Router は `_` 始まりの folder を **private folder** として route から
//     除外する (https://nextjs.org/docs/app/building-your-application/routing/colocation)
//   - 当初 `__sample__` で作ったが、上記仕様により `[slug]` 動的 route に流れて 500 に
//     なったため `dev-sample` に rename した
//
// 目的:
//   - Backend が page_meta を返す前でも、人間が実機 / dev で「フォトブックっぽさ」
//     を確認できるようにする (要件 B)
//   - 表紙パターン A グラデーション + magazine layout + cover_first variant + 5 page
//
// 動作:
//   - process.env.NODE_ENV === "production" の時は notFound() で 404
//   - dev / test 環境では sampleSunsetMemories() を ViewerLayout に流し込む
//
// セキュリティ:
//   - production bundle にも本ファイルは含まれるが、production ガードで route 自体が 404
//   - fixture data は local public asset 参照のみ (presigned / Secret 含まず)

import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { ViewerLayout } from "@/components/Viewer/ViewerLayout";
import { sampleSunsetMemories } from "@/lib/__fixtures__/publicPhotobookSample";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Sample | VRC PhotoBook",
  robots: { index: false, follow: false },
};

export default function SampleViewerPage() {
  if (process.env.NODE_ENV === "production") {
    notFound();
  }
  const photobook = sampleSunsetMemories();
  return <ViewerLayout photobook={photobook} />;
}
