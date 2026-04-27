# Colors（design-system 第一弾、PR25b）

> 抽出元: `design/mockups/prototype/styles.css` の `:root` token
>
> 本ファイルは VRC PhotoBook の **Light テーマ正典**。`frontend/tailwind.config.ts`
> の `theme.extend.colors` と本ファイルを同期させる。

## ベース

| 用途 | token | hex |
|---|---|---|
| 背景（カード / 主文） | `surface` | `#FFFFFF` |
| 背景（弱、ページ全体） | `bg` | `#FFFFFF` |
| 背景（補助、セクション区切り） | `bg-2` | `#F7F9FA` |
| 背景（補助、外周フレーム等） | `bg-3` | `#EEF2F4` |
| 罫線 | `border` | `#E5EAED` |
| 罫線（弱） | `border-2` | `#EEF1F3` |

## アクセント（teal）

teal はサービスのプライマリ。CTA / 公開 URL の identifier に使う。

| 用途 | token | hex |
|---|---|---|
| Primary | `teal` | `#14B8A6` |
| Primary hover / pressed | `teal-2` | `#0FA094` |
| Primary highlight | `teal-3` | `#5DCFC3` |
| Primary 背景（弱） | `teal-soft` | `#E6F7F5` |

## テキスト

| 用途 | token | hex |
|---|---|---|
| 本文（最強） | `fg` | `#0F172A` |
| 本文（強） | `fg-2` | `#334155` |
| 本文（中） | `fg-3` | `#64748B` |
| 本文（弱） | `fg-4` | `#94A3B8` |

## ステータス

| 用途 | token | hex |
|---|---|---|
| エラー / 警告 | `red` | `#EF4444` |
| エラー 背景（弱） | `red-soft` | `#FEF2F2` |
| Manage URL（差別化） | `violet` | `#8B5CF6` |

## 使い分けの原則

- **公開 URL**: teal を識別色とする（mockup `UrlRow` に準拠）
- **管理 URL**: violet を識別色とする（mockup `UrlRow` の `kind="manage"`）
- **エラー**: `red` + `red-soft` を 1 セットで使う
- **背景階層**: 主面 = `surface`、外周 = `bg-2` → `bg-3`（深い順）
- **罫線**: 主用途は `border`。`border-2` は弱い区切り（リスト row 等）に限定

## Tailwind config への反映

`frontend/tailwind.config.ts` の `theme.extend.colors` に下記キーで反映する:

```ts
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
    soft:   "#94A3B8",
  },
  surface: {
    DEFAULT: "#FFFFFF",
    soft:    "#F7F9FA",
    raised:  "#EEF2F4",
  },
  divider: {
    DEFAULT: "#E5EAED",
    soft:    "#EEF1F3",
  },
  status: {
    error: "#EF4444",
    "error-soft": "#FEF2F2",
  },
}
```

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-27 | 初版（PR25b）。prototype `styles.css` から抽出 |
