import type { Config } from "tailwindcss";

// 2026-05-03 m2-design-refresh STOP β-1: design refresh の token 正典を反映。
// 値の正典は `design/source/project/wireframe-styles.css:7-51` の `:root` block。
// design archive の token を Tailwind class 値に直接 swap する（class 名は維持、値だけ更新）。
//
// 設計参照:
//   - docs/plan/m2-design-refresh-plan.md §0.1 正典方針 / §2 design token / §6 STOP β-1
//   - design/source/project/wireframe-styles.css:7-51 (:root)
//   - design/source/project/wf-shared.jsx (PC / Mobile shell)
//
// 9 段 teal ramp (`teal.50..800`) を新規追加。既存 `brand.teal` / `brand.teal-soft` /
// `brand.teal-hover` は design 値で値だけ swap (class 名不変)。
const config: Config = {
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./features/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // 既存 brand.* class 名は維持、値を design に swap (`wireframe-styles.css:13` `--teal-500` 等)
        brand: {
          teal: "#15B2A8",
          "teal-hover": "#0E988F",
          "teal-soft": "#EDFAF8",
          violet: "#8B5CF6",
        },
        // 9 段 teal ramp (新規追加、`wireframe-styles.css:8-16` の --teal-50..--teal-800)
        teal: {
          50: "#EDFAF8",
          100: "#D4F2EE",
          200: "#A8E5DD",
          300: "#6FD2C5",
          400: "#3CC1B1",
          500: "#15B2A8",
          600: "#0E988F",
          700: "#0A7A73",
          800: "#095F59",
        },
        // ink (`wireframe-styles.css:18-21` --ink / --ink-2 / --ink-3 / --ink-4)
        ink: {
          DEFAULT: "#0F2A2E",
          strong: "#2C4A4F",
          medium: "#5C7378",
          soft: "#8FA2A6",
        },
        // surface (`wireframe-styles.css:26-28` --bg / --paper / --soft)
        surface: {
          DEFAULT: "#FFFFFF",
          soft: "#F6F9FA",
          raised: "#F0F6F7",
        },
        // divider (`wireframe-styles.css:22-24` --line / --line-2 / --line-3)
        divider: {
          DEFAULT: "#E1E8EA",
          soft: "#ECF1F2",
        },
        // status は業界標準維持 (design は inline で warn 配色を出すが status semantic は維持)
        status: {
          error: "#EF4444",
          "error-soft": "#FEF2F2",
          warn: "#D97706",
          "warn-soft": "#FFF7ED",
          success: "#16A34A",
          "success-soft": "#ECFDF5",
        },
      },
      fontFamily: {
        // `wireframe-styles.css:54` の正典フォントスタック
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
        // h1: design `wireframe-styles.css:351-358` `.wf-h1` 30px / `.wf-h1.lg` 42px
        h1: ["30px", { lineHeight: "1.25", fontWeight: "800" }],
        "h1-lg": ["42px", { lineHeight: "1.18", fontWeight: "800" }],
        // h2: design `:359` `.wf-h2` 18px / 700
        h2: ["18px", { lineHeight: "1.4", fontWeight: "700" }],
        // body / sm / xs は既存値 (14 / 12 / 11 px) を維持。design は 13.5 / 11.5 / 10.5
        // だが component 高さ整合のため既存維持（plan §2.2 「body 14px 維持」）
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
        // shadow tone を ink-tone (`#0F2A2E` 系) に合わせて調整。
        // design `wireframe-styles.css:30-32` 参照。
        sm: "0 1px 2px rgba(15, 42, 46, 0.04)",
        DEFAULT: "0 4px 14px rgba(15, 42, 46, 0.06), 0 1px 2px rgba(15, 42, 46, 0.04)",
        // 新規: lg shadow (LP MockBook spread / 大型 card で使用、`wireframe-styles.css:32`)
        lg: "0 18px 40px -16px rgba(15, 42, 46, 0.18), 0 4px 14px rgba(15, 42, 46, 0.06)",
      },
    },
  },
  plugins: [],
};

export default config;
