import type { Metadata } from "next";
import type { ReactNode } from "react";

// metadataBase は OGP / Twitter card の相対 URL（例: /og-sample.png）を絶対 URL に展開する基底。
// NEXT_PUBLIC_BASE_URL が設定されていればそれを使い、未設定なら localhost にフォールバック。
// - 本番 / Workers 実環境: .env.production で Workers URL を渡す
// - ローカル `next dev`: 既定の http://localhost:3000
// 詳細経緯: ADR-0001 §M1 検証結果 / harness/work-logs/2026-04-26_m1-live-deploy-verification.md
const baseUrl = process.env.NEXT_PUBLIC_BASE_URL || "http://localhost:3000";

export const metadata: Metadata = {
  metadataBase: new URL(baseUrl),
  title: "VRC PhotoBook M1 Spike",
  description: "M1 spike PoC. Not for production.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="ja">
      <body
        style={{
          fontFamily:
            "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
          maxWidth: 720,
          margin: "2rem auto",
          padding: "0 1rem",
          lineHeight: 1.6,
        }}
      >
        {children}
      </body>
    </html>
  );
}
