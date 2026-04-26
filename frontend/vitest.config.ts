// PR10.5: Frontend 用 Vitest 設定。
//
// 方針:
//   - Route Handler の GET を直接呼び出す形でテストする
//   - global.fetch を mock して Backend response を差し替える
//   - 重い E2E（実ブラウザ / Playwright）は本 PR では入れない
//
// セキュリティ:
//   - テスト内で raw token / session token / Cookie 値を console / log に出さない
//   - 固定 43 文字 token を repo に書かない（テストごとに動的生成）
import { defineConfig } from "vitest/config";
import path from "node:path";

export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(__dirname),
    },
  },
  test: {
    environment: "node",
    include: ["**/*.test.ts", "**/*.test.tsx"],
    exclude: ["node_modules/**", ".next/**", ".open-next/**", ".wrangler/**"],
  },
});
