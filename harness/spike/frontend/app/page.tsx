/**
 * M1 PoC トップページ
 *
 * 検証用ルートへのリンク集を提供する。
 */
export default function Home() {
  return (
    <main>
      <h1>VRC PhotoBook — M1 Spike Frontend PoC</h1>
      <p>
        Next.js 15 App Router + Cloudflare Pages の SSR / Cookie / Header
        制御を最小 PoC で検証するためのリンク集です。詳細は{" "}
        <code>README.md</code> を参照してください。
      </p>

      <h2>検証用ルート</h2>
      <ul>
        <li>
          <a href="/p/sample-slug">/p/sample-slug</a> —
          公開ページ（SSR / OGP / noindex / strict-origin-when-cross-origin）
        </li>
        <li>
          <a href="/draft/sample-draft-token">/draft/sample-draft-token</a> —
          draft 入場（token → session Cookie 交換 + redirect）
        </li>
        <li>
          <a href="/edit/sample-photobook-id">/edit/sample-photobook-id</a> —
          draft session 検証（Cookie 読取）
        </li>
        <li>
          <a href="/manage/token/sample-manage-token">
            /manage/token/sample-manage-token
          </a>{" "}
          — manage 入場（token → session Cookie 交換 + redirect）
        </li>
        <li>
          <a href="/manage/sample-photobook-id">/manage/sample-photobook-id</a>{" "}
          — manage session 検証（Cookie 読取）
        </li>
        <li>
          <a href="/integration/backend-check">/integration/backend-check</a> —
          Backend API 結合検証（CORS / Cookie / Origin）
        </li>
      </ul>

      <h2>注意</h2>
      <ul>
        <li>本ページは PoC 用。本実装には流用しない。</li>
        <li>Cookie 値そのものは画面に表示しない（存在の有無のみ表示）。</li>
        <li>token 値は console / log に出さない。</li>
      </ul>
    </main>
  );
}
