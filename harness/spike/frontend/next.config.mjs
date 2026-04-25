/** @type {import('next').NextConfig} */
const nextConfig = {
  // M1 PoC: Cloudflare Pages では @cloudflare/next-on-pages がビルドを変換するため、
  // ここでは Next.js 標準の挙動のままにしておく。Edge runtime 指定は各 route ファイルで行う。
  reactStrictMode: true,

  // 検証用画像ホストの allowlist（必要に応じて拡張）。
  images: {
    remotePatterns: [],
  },

  // 共通ヘッダ。X-Robots-Tag は middleware.ts でも上書きするが、ベースラインとしてここでも noindex を立てる。
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Robots-Tag", value: "noindex, nofollow" },
        ],
      },
    ];
  },
};

export default nextConfig;
