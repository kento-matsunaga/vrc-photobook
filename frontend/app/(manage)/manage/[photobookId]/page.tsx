// /manage/[photobookId] 管理ページ（PR25b 本実装）。
//
// 設計参照:
//   - docs/plan/m2-public-viewer-and-manage-plan.md §4
//   - docs/plan/m2-design-refresh-stop-beta-3-plan.md §2 (β-4 で同設計を Manage にも適用)
//
// 認可:
//   - Cookie が無ければ Backend は 401 を返す → ErrorState(unauthorized) を表示
//   - Cookie はあるが photobook_id 不一致は Backend 側で 401（middleware）
//   - photobook_id 不存在は 404
//
// セキュリティ:
//   - Cookie 値 / manage_url_token / draft_edit_token を画面に出さない
//   - Cookie ヘッダは Backend にだけ転送（Workers の fetch は同 origin でないため
//     `credentials: include` ではなく Cookie ヘッダを手で組み立てる）
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - PublicTopBar 統合 (showPrimaryCta=false)。manage session 経路だが LP 戻り nav は妥当、
//     primary CTA「無料で作る」は管理ページ context で違和感のため非表示。
//   - ErrorState 全 3 経路 (unauthorized / not_found / server_error) でも TopBar wrap

import type { Metadata } from "next";
import { headers } from "next/headers";

import { ErrorState } from "@/components/ErrorState";
import { ManagePanel } from "@/components/Manage/ManagePanel";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import {
  fetchManagePhotobook,
  isManageLookupError,
  type ManagePhotobook,
} from "@/lib/managePhotobook";

export const dynamic = "force-dynamic";

type Params = Promise<{ photobookId: string }>;

export const metadata: Metadata = {
  title: "管理ページ | VRC PhotoBook",
  robots: { index: false, follow: false },
};

/**
 * Server Component が受信した Cookie ヘッダをそのまま Backend に転送する。
 *
 * Workers / OpenNext の SSR では、`next/headers` の cookies() で個別 Cookie が読めるが、
 * Backend に対する forwarding は単純に Cookie ヘッダを文字列で渡す方が安全。
 */
async function getRequestCookieHeader(): Promise<string> {
  const h = await headers();
  return h.get("cookie") ?? "";
}

export default async function ManagePage({ params }: { params: Params }) {
  const { photobookId } = await params;
  const cookieHeader = await getRequestCookieHeader();

  let photobook: ManagePhotobook;
  try {
    photobook = await fetchManagePhotobook(photobookId, cookieHeader);
  } catch (e) {
    if (isManageLookupError(e)) {
      switch (e.kind) {
        case "unauthorized":
          return (
            <>
              <PublicTopBar showPrimaryCta={false} />
              <ErrorState variant="unauthorized" />
            </>
          );
        case "not_found":
          return (
            <>
              <PublicTopBar showPrimaryCta={false} />
              <ErrorState variant="not_found" />
            </>
          );
        case "server_error":
        case "network":
          return (
            <>
              <PublicTopBar showPrimaryCta={false} />
              <ErrorState variant="server_error" />
            </>
          );
      }
    }
    return (
      <>
        <PublicTopBar showPrimaryCta={false} />
        <ErrorState variant="server_error" />
      </>
    );
  }

  const baseUrl = process.env.NEXT_PUBLIC_BASE_URL ?? "";
  return (
    <>
      <PublicTopBar showPrimaryCta={false} />
      <ManagePanel photobook={photobook} appBaseUrl={baseUrl} />
    </>
  );
}
