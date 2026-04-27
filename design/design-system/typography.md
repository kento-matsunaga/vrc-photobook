# Typography（design-system 第一弾、PR25b）

> 抽出元: `design/mockups/prototype/styles.css` のクラス `.t-h1` / `.t-h2` / `.t-body` / `.t-sm` / `.t-xs`

## フォントファミリ

| 用途 | font-family |
|---|---|
| 日本語（本文・見出し） | `"Hiragino Sans", "Noto Sans JP", -apple-system, BlinkMacSystemFont, system-ui, sans-serif` |
| 数字・URL | `"SF Pro Display", -apple-system, system-ui, sans-serif` |

`tailwind.config.ts` で `fontFamily.sans` / `fontFamily.mono`（数値・URL 用）に対応。

## サイズ階層

| 用途 | size (px) | line-height | weight |
|---|---|---|---|
| 見出し H1（ページタイトル） | 24 | 1.35 | 700 |
| 見出し H2（セクション） | 18 | 1.4 | 700 |
| 本文 | 14 | 1.6 | 400 |
| 補助小（注釈・キャプション） | 12 | 1.5 | 400 |
| 極小（ラベル） | 11 | 1.4 | 500 |

## 強調 / 弱調

- **強調**: `text-ink` + `font-bold` または H1/H2
- **本文**: `text-ink-strong`
- **補助**: `text-ink-medium`
- **無効** / placeholder: `text-ink-soft`

## Tailwind config への反映

```ts
fontSize: {
  "h1":   ["24px", { lineHeight: "1.35", fontWeight: "700" }],
  "h2":   ["18px", { lineHeight: "1.4",  fontWeight: "700" }],
  "body": ["14px", { lineHeight: "1.6",  fontWeight: "400" }],
  "sm":   ["12px", { lineHeight: "1.5",  fontWeight: "400" }],
  "xs":   ["11px", { lineHeight: "1.4",  fontWeight: "500" }],
},
```

`fontFamily`:

```ts
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
```

## 使い分けの原則

- 公開 URL / manage URL は `font-num` を使う（数字 / URL 字幅安定のため）
- 本文長文は `body`、リスト row は `sm`
- 「VRC」の固有表記は強調しない（ロゴ表記は別途）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-27 | 初版（PR25b）。prototype `styles.css` から抽出 |
