import type { Config } from "tailwindcss";

// Tailwind v3 最小設定。
// PR5 以降でデザインシステム（design/design-system/）と整合する theme 拡張を行う想定。
const config: Config = {
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./features/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {},
  },
  plugins: [],
};

export default config;
