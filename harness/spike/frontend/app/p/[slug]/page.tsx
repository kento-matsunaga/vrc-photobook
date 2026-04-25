import type { Metadata } from "next";

/**
 * M1 PoC: 公開ページ検証ルート
 *
 * 検証目的:
 *  - SSR が動くか
 *  - generateMetadata で OGP メタタグが SSR HTML に動的に出力されるか
 *  - <meta name="robots" content="noindex"> が出るか
 *  - X-Robots-Tag, Referrer-Policy: strict-origin-when-cross-origin が middleware から付与されるか
 */

// OpenNext for Cloudflare は Workers 上の Node.js 互換ランタイムで動作する。
// `runtime = 'edge'` を指定すると別バンドルが必要になりビルドエラーになるため、
// 明示指定はしない（Next.js デフォルト = nodejs ランタイム想定で OK）。
// 参考: next-on-pages 版（コミット c7ba16b）では `runtime = 'edge'` 必須だった点が逆転している。

type Params = Promise<{ slug: string }>;

export async function generateMetadata({
  params,
}: {
  params: Params;
}): Promise<Metadata> {
  const { slug } = await params;
  const title = `Sample Photobook (${slug}) — M1 Spike`;
  const description =
    "M1 PoC verification page. SSR + OGP rendering check. Not for production.";
  // og:image はダミー（PoC では画像実体は不要、HTML 出力の検証が目的）
  const ogImageUrl = "/og-sample.png";

  return {
    title,
    description,
    openGraph: {
      title,
      description,
      type: "article",
      images: [{ url: ogImageUrl, width: 1200, height: 630 }],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
      images: [ogImageUrl],
    },
    robots: {
      index: false,
      follow: false,
    },
  };
}

export default async function PublicPhotobookPage({
  params,
}: {
  params: Params;
}) {
  const { slug } = await params;

  return (
    <main>
      <h1>公開ページ PoC</h1>
      <p>
        slug: <code>{slug}</code>
      </p>
      <p>
        このページは SSR でレンダリングされ、<code>generateMetadata</code>{" "}
        で OGP メタタグが HTML に動的に挿入される。
      </p>

      <h2>検証手順</h2>
      <ol>
        <li>
          ブラウザで <kbd>View Source</kbd> し、以下を確認する:
          <ul>
            <li>
              <code>&lt;meta property="og:title"&gt;</code>
            </li>
            <li>
              <code>&lt;meta property="og:description"&gt;</code>
            </li>
            <li>
              <code>&lt;meta property="og:image"&gt;</code>
            </li>
            <li>
              <code>&lt;meta name="twitter:card" content="summary_large_image"&gt;</code>
            </li>
            <li>
              <code>&lt;meta name="robots" content="noindex"&gt;</code>
            </li>
          </ul>
        </li>
        <li>
          DevTools → Network → Response Headers で以下を確認する:
          <ul>
            <li>
              <code>X-Robots-Tag: noindex, nofollow</code>
            </li>
            <li>
              <code>Referrer-Policy: strict-origin-when-cross-origin</code>
            </li>
          </ul>
        </li>
      </ol>

      <p>
        <a href="/">← トップへ戻る</a>
      </p>
    </main>
  );
}
