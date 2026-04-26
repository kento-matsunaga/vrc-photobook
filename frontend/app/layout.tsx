import type { Metadata } from "next";
import type { ReactNode } from "react";

import "./globals.css";

// metadataBase は OGP / Twitter card の相対 URL（例: /og-default.png）を絶対 URL に展開する基底。
// NEXT_PUBLIC_BASE_URL（Workers の独自ドメイン or 暫定 URL）が設定されていればそれを使い、
// 未設定なら localhost にフォールバックする（`next dev` 用）。
//
// 仕組み:
//   - NEXT_PUBLIC_* は Next.js のビルド時に bundle に inline される
//   - wrangler runtime env では Frontend bundle に届かないため、必ず .env.production を使う
//   - 詳細経緯: M2 実装ブートストラップ計画 §6 / harness/work-logs/2026-04-26_m1-live-deploy-verification.md
const baseUrl = process.env.NEXT_PUBLIC_BASE_URL || "http://localhost:3000";

export const metadata: Metadata = {
  metadataBase: new URL(baseUrl),
  title: "VRC PhotoBook",
  description: "VRChat 向けフォトブックサービス（非公式ファンメイド、開発中）",
  // MVP は全ページ noindex（業務知識 v4 §7.6）。
  // X-Robots-Tag ヘッダは middleware.ts でも付与する（HTML meta + ヘッダの両方で noindex を担保）。
  robots: {
    index: false,
    follow: false,
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="ja">
      <body className="min-h-screen bg-white text-gray-900 antialiased">
        {children}
      </body>
    </html>
  );
}
