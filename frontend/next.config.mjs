/** @type {import('next').NextConfig} */
const nextConfig = {
  // PR4: 最小構成。
  // PR5 で middleware.ts に X-Robots-Tag / Referrer-Policy を一本化する方針。
  // M1 学習: next.config.mjs の headers() に X-Robots-Tag を書くと middleware と二重出力になるため
  //         本 config では headers() を持たない（harness/work-logs/2026-04-26_m1-live-deploy-verification.md）。
  reactStrictMode: true,

  // 検証用画像ホストの allowlist（必要に応じて拡張、PR5 以降で OGP / 画像読み込みに合わせて追加）。
  images: {
    remotePatterns: [],
  },
};

export default nextConfig;
