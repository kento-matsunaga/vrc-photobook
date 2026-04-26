// /edit/[photobookId] の最小ページ（PR10 段階）。
//
// PR10 では Cookie 化された draft session の redirect 着地点として存在する。
// Backend protected API（`/api/photobooks/{id}` 等）の呼び出しは PR11 以降。
//
// セキュリティ:
//   - URL path の photobook_id 以外は **画面に出さない**（token / Cookie 値は当然出さない）

export const dynamic = "force-dynamic";

export default async function EditPage({
  params,
}: {
  params: Promise<{ photobookId: string }>;
}) {
  const { photobookId } = await params;
  return (
    <main className="mx-auto max-w-3xl p-8">
      <h1 className="mb-4 text-xl font-semibold">Draft 編集ページ（最小実装）</h1>
      <p className="text-sm text-gray-700">
        photobook_id: <code className="font-mono">{photobookId}</code>
      </p>
      <p className="mt-4 text-sm text-gray-500">
        PR10 段階のプレースホルダです。draft session Cookie が発行されていれば本ページに到達できます。
        編集 UI と Backend protected API は PR11 以降で実装します。
      </p>
    </main>
  );
}
