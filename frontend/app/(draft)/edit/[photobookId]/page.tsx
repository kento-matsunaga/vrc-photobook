// /edit/[photobookId] ページ（PR27 本格編集 UI）。
//
// 設計参照:
//   - docs/plan/m2-frontend-edit-ui-fullspec-plan.md §3 / §6
//
// 役割:
//   - Server Component で edit-view を fetch（draft Cookie を Backend に転送）
//   - 401 / 404 / 409 → ErrorState、200 → EditClient
//
// セキュリティ:
//   - Cookie 値 / raw token を画面に出さない
//   - photobook_id は URL から取得、その他値は画面に直接出さない

import type { Metadata } from "next";
import { headers } from "next/headers";

import { ErrorState } from "@/components/ErrorState";
import {
  fetchEditView,
  isEditApiError,
  type EditView,
} from "@/lib/editPhotobook";
import { EditClient } from "./EditClient";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "編集 | VRC PhotoBook",
  robots: { index: false, follow: false },
};

type Params = Promise<{ photobookId: string }>;

async function getRequestCookieHeader(): Promise<string> {
  const h = await headers();
  return h.get("cookie") ?? "";
}

export default async function EditPage({ params }: { params: Params }) {
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
          return <ErrorState variant="unauthorized" />;
        case "not_found":
          return <ErrorState variant="not_found" />;
        case "version_conflict":
          // 編集系は published 等で 409 を返すため、ユーザー向けには「閲覧不可」の扱い
          return <ErrorState variant="not_found" />;
        case "bad_request":
        case "server_error":
        case "network":
          return <ErrorState variant="server_error" />;
      }
    }
    return <ErrorState variant="server_error" />;
  }

  return <EditClient initial={view} turnstileSiteKey={turnstileSiteKey} />;
}
