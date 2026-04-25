import { cookies } from "next/headers";

/**
 * M1 PoC: draft session 検証ページ
 *
 * 検証目的:
 *  - /draft/{token} からの redirect 後、Server Component で Cookie を読めるか
 *  - Cookie の存在有無で表示が分岐するか
 *  - Cookie 値そのものは画面に出さない（存在のみ表示）
 */

export const runtime = "edge";

type Params = Promise<{ photobook_id: string }>;

export default async function EditPage({ params }: { params: Params }) {
  const { photobook_id } = await params;
  const cookieStore = await cookies();
  const cookieName = `vrcpb_draft_${photobook_id}`;
  const sessionCookie = cookieStore.get(cookieName);
  const hasSession = sessionCookie !== undefined;

  return (
    <main>
      <h1>draft session 検証ページ</h1>
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
          <strong>draft session found</strong>
        ) : (
          <strong>draft session missing</strong>
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
          まず <a href="/draft/sample-draft-token">/draft/sample-draft-token</a>{" "}
          にアクセスする（Cookie 発行 + 本ページへ redirect）。
        </li>
        <li>
          リダイレクト後、URL に token が含まれていない（
          <code>/edit/{photobook_id}</code> のみ）ことを確認する。
        </li>
        <li>
          上記の「結果」が <code>draft session found</code> であることを確認する。
        </li>
        <li>
          ページ再読み込み（F5 / Safari の更新）後も{" "}
          <code>draft session found</code> のままか確認する。
        </li>
        <li>
          DevTools → Application → Cookies で以下を確認する:
          <ul>
            <li>HttpOnly = true（JavaScript から読めない）</li>
            <li>Secure = true</li>
            <li>SameSite = Strict</li>
            <li>Path = /</li>
          </ul>
        </li>
      </ol>

      <p>
        <a href="/">← トップへ戻る</a>
      </p>
    </main>
  );
}
