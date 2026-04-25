import { cookies } from "next/headers";

/**
 * M1 PoC: manage session 検証ページ
 *
 * 検証目的:
 *  - /manage/token/{token} からの redirect 後、Server Component で manage Cookie を読めるか
 *  - draft session（vrcpb_draft_*）と manage session（vrcpb_manage_*）が混同なく動くか
 */

// OpenNext for Cloudflare では `runtime = 'edge'` を指定しない（v2 切替時の修正）。
// 詳細は app/p/[slug]/page.tsx のコメント参照。

type Params = Promise<{ photobook_id: string }>;

export default async function ManagePage({ params }: { params: Params }) {
  const { photobook_id } = await params;
  const cookieStore = await cookies();
  const cookieName = `vrcpb_manage_${photobook_id}`;
  const sessionCookie = cookieStore.get(cookieName);
  const hasSession = sessionCookie !== undefined;

  return (
    <main>
      <h1>manage session 検証ページ</h1>
      <p>
        photobook_id: <code>{photobook_id}</code>
      </p>
      <p>
        Cookie 名: <code>{cookieName}</code>
      </p>

      <h2>結果</h2>
      <p
        style={{
          padding: "0.75rem 1rem",
          background: hasSession ? "#e8f5e9" : "#ffebee",
          border: `1px solid ${hasSession ? "#a5d6a7" : "#ef9a9a"}`,
          borderRadius: 4,
        }}
      >
        {hasSession ? (
          <strong>manage session found</strong>
        ) : (
          <strong>manage session missing</strong>
        )}
      </p>

      <p>
        <small>
          ※ Cookie 値そのものは表示しない。存在の有無のみ確認する PoC。
        </small>
      </p>

      <h2>検証手順</h2>
      <ol>
        <li>
          まず{" "}
          <a href="/manage/token/sample-manage-token">
            /manage/token/sample-manage-token
          </a>{" "}
          にアクセスする（Cookie 発行 + 本ページへ redirect）。
        </li>
        <li>
          リダイレクト後、URL に token が含まれていない（
          <code>/manage/{photobook_id}</code> のみ）ことを確認する。
        </li>
        <li>
          上記の「結果」が <code>manage session found</code>{" "}
          であることを確認する。
        </li>
        <li>
          ページ再読み込み後も <code>manage session found</code>{" "}
          のままか確認する。
        </li>
        <li>
          DevTools → Application → Cookies で以下を確認:
          <ul>
            <li>Cookie 名 <code>{cookieName}</code> が独立して存在</li>
            <li>HttpOnly / Secure / SameSite=Strict / Path=/</li>
          </ul>
        </li>
      </ol>

      <p>
        <a href="/">← トップへ戻る</a>
      </p>
    </main>
  );
}
