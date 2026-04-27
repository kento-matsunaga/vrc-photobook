import type { Config } from "tailwindcss";

// PR25b: design-system 第一弾を反映。
// 正典は design/design-system/{colors,typography,spacing,radius-shadow}.md。
// prototype 値の直接コピーではなく、本実装の用途に整理して採用する。
const config: Config = {
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./features/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          teal: "#14B8A6",
          "teal-hover": "#0FA094",
          "teal-soft": "#E6F7F5",
          violet: "#8B5CF6",
        },
        ink: {
          DEFAULT: "#0F172A",
          strong: "#334155",
          medium: "#64748B",
          soft: "#94A3B8",
        },
        surface: {
          DEFAULT: "#FFFFFF",
          soft: "#F7F9FA",
          raised: "#EEF2F4",
        },
        divider: {
          DEFAULT: "#E5EAED",
          soft: "#EEF1F3",
        },
        status: {
          error: "#EF4444",
          "error-soft": "#FEF2F2",
          warn: "#D97706",
          "warn-soft": "#FFF7ED",
        },
      },
      fontFamily: {
        sans: [
          "Hiragino Sans",
          "Noto Sans JP",
          "-apple-system",
          "BlinkMacSystemFont",
          "system-ui",
          "sans-serif",
        ],
        num: [
          "SF Pro Display",
          "-apple-system",
          "system-ui",
          "sans-serif",
        ],
      },
      fontSize: {
        h1: ["24px", { lineHeight: "1.35", fontWeight: "700" }],
        h2: ["18px", { lineHeight: "1.4", fontWeight: "700" }],
        body: ["14px", { lineHeight: "1.6", fontWeight: "400" }],
        sm: ["12px", { lineHeight: "1.5", fontWeight: "400" }],
        xs: ["11px", { lineHeight: "1.4", fontWeight: "500" }],
      },
      borderRadius: {
        sm: "8px",
        DEFAULT: "12px",
        lg: "16px",
        xl: "20px",
      },
      boxShadow: {
        sm: "0 1px 2px rgba(15, 23, 42, 0.04)",
        DEFAULT: "0 4px 12px rgba(15, 23, 42, 0.05)",
      },
    },
  },
  plugins: [],
};

export default config;
