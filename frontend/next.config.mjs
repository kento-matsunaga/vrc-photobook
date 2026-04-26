/** @type {import('next').NextConfig} */
const nextConfig = {
  // OpenNext for Cloudflare Workers + Static Assets binding を最終ターゲットとする（ADR-0001）。
  //
  // 重要な実装ルール（M1 で確定、v4 §7.6 / ADR-0003 / failure-log の経験を反映）:
  //   - X-Robots-Tag / Referrer-Policy は middleware.ts に一本化する
  //     （next.config.mjs の headers() に X-Robots-Tag を書くと middleware と二重出力になる）
  //   - 各ページ / Route Handler で `export const runtime = "edge"` を指定しない
  //     （OpenNext は Workers 上 Node.js 互換ランタイムで動作）
  reactStrictMode: true,

  // 検証用画像ホストの allowlist（必要に応じて拡張、後続 PR で OGP / 画像読み込みに合わせて追加）。
  images: {
    remotePatterns: [],
  },
};

export default nextConfig;
