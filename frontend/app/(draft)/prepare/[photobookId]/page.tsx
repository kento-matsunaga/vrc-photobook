// /prepare/[photobookId] ページ（Upload Staging 画面、Server Component）。
//
// 設計参照:
//   - docs/plan/m2-upload-staging-plan.md §6 / §13
//   - docs/plan/m2-design-refresh-stop-beta-3-plan.md §2
//
// 役割:
//   - Server Component で edit-view を fetch（draft Cookie を Backend に転送）
//   - 401 / 404 / 409 → ErrorState、200 → PrepareClient
//   - m2-design-refresh STOP β-3: PublicTopBar 統合 (showPrimaryCta=false)。
//     draft session 経路だが、LP / 他公開ページに戻る nav は a11y 上有用。
//     primary CTA「無料で作る」は draft 中の文脈に違和感のため非表示 (Q-3-3 確定)。
//
// セキュリティ:
//   - Cookie 値 / raw token を画面に出さない
//   - photobook_id は URL から取得、その他値は画面に直接出さない

import type { Metadata } from "next";
import { headers } from "next/headers";

import { ErrorState } from "@/components/ErrorState";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { fetchEditView, isEditApiError, type EditView } from "@/lib/editPhotobook";

import { PrepareClient } from "./PrepareClient";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "写真を追加 | VRC PhotoBook",
  robots: { index: false, follow: false },
};

type Params = Promise<{ photobookId: string }>;

async function getRequestCookieHeader(): Promise<string> {
  const h = await headers();
  return h.get("cookie") ?? "";
}

export default async function PreparePage({ params }: { params: Params }) {
  const { photobookId } = await params;
  const turnstileSiteKey = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY ?? "";
  const cookieHeader = await getRequestCookieHeader();

  let view: EditView;
  try {
    view = await fetchEditView(photobookId, cookieHeader);
  } catch (e) {
    if (isEditApiError(e)) {
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
        case "version_conflict":
          return (
            <>
              <PublicTopBar showPrimaryCta={false} />
              <ErrorState variant="not_found" />
            </>
          );
        case "bad_request":
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

  if (turnstileSiteKey === "") {
    return (
      <>
        <PublicTopBar showPrimaryCta={false} />
        <ErrorState variant="server_error" />
      </>
    );
  }

  return (
    <>
      <PublicTopBar showPrimaryCta={false} />
      <PrepareClient
        photobookId={photobookId}
        turnstileSiteKey={turnstileSiteKey}
        initialView={view}
      />
    </>
  );
}
