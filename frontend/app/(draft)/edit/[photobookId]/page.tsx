// /edit/[photobookId] ページ。
//
// PR22: Server Component で photobook_id と Turnstile sitekey を解決し、
// Client Component (UploadClient) に渡す。upload UI / Turnstile widget / API client は
// すべて Client 側で実装。
//
// セキュリティ:
//   - URL path の photobook_id 以外は画面に出さない
//   - Turnstile sitekey は公開値（NEXT_PUBLIC_TURNSTILE_SITE_KEY）

import { UploadClient } from "./UploadClient";

export const dynamic = "force-dynamic";

export default async function EditPage({
  params,
}: {
  params: Promise<{ photobookId: string }>;
}) {
  const { photobookId } = await params;
  const turnstileSiteKey = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY ?? "";
  return <UploadClient photobookId={photobookId} turnstileSiteKey={turnstileSiteKey} />;
}
