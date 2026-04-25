/** @type {import('next').NextConfig} */
const nextConfig = {
  // M1 PoC: OpenNext for Cloudflare で Workers + Static Assets binding 上で動かす。
  // Edge runtime は指定しない（OpenNext は Workers 上 Node.js 互換ランタイムで動作）。
  reactStrictMode: true,

  // 検証用画像ホストの allowlist（必要に応じて拡張）。
  images: {
    remotePatterns: [],
  },

  // X-Robots-Tag / Referrer-Policy は middleware.ts に一本化する。
  // 過去に next.config.mjs の headers() と middleware.ts の両方で X-Robots-Tag を
  // 出していたため、Workers 実環境で値が `noindex, nofollow, noindex, nofollow` と
  // 二重出力された（2026-04-26 確認）。本実装でも middleware 一本化を維持する。
  // 詳細: harness/work-logs/2026-04-26_m1-live-deploy-verification.md
};

export default nextConfig;
