import type { Metadata } from "next";
import type { ReactNode } from "react";

import "./globals.css";

// PR4: 最小レイアウト。
// PR5 で metadataBase（OGP og:image の絶対 URL 解決）/ middleware による
// X-Robots-Tag / Referrer-Policy 出し分け / robots metadata を整備する。
export const metadata: Metadata = {
  title: "VRC PhotoBook",
  description: "VRChat 向けフォトブックサービス（非公式ファンメイド、開発中）",
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
