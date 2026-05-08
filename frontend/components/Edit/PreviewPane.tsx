// PreviewPane: editViewToPreview(view) を ViewerLayout に流し込む thin wrapper。
//
// 設計参照:
//   - docs/plan/m2-edit-page-split-and-preview-plan.md §6.8 / §6.9
//
// 責務:
//   - EditView を PublicPhotobook 形式 (PreviewPane 用) に変換
//   - ViewerLayout を再利用して draft 状態を公開後と同じ見た目で確認
//
// 制約:
//   - read-only (preview 中は mutation を投げない、CTA も表示する別経路はないため安全)
//   - presigned URL は EditView と同じものを使う (15 分有効、expired 時は edit に戻って
//     reload を user に促す = §6.8 方針、本 component では特別な expiry 処理を行わない)
//   - creator name / slug / publishedAt は preview 用 placeholder (editViewToPreview の責務)
"use client";

import { ViewerLayout } from "@/components/Viewer/ViewerLayout";
import { editViewToPreview } from "@/lib/editPreview";
import type { EditView } from "@/lib/editPhotobook";

type Props = {
  view: EditView;
};

export function PreviewPane({ view }: Props) {
  const photobook = editViewToPreview(view);
  return (
    <div data-testid="preview-pane">
      <ViewerLayout photobook={photobook} />
    </div>
  );
}
