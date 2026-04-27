# Radius / Shadow（design-system 第一弾、PR25b）

> 抽出元: `design/mockups/prototype/styles.css` の `--radius-*` / `--shadow*` token

## Radius

| 用途 | px | token |
|---|---|---|
| ピル / ボタン | 12 | `radius` |
| 小要素（バッジ / chip） | 8 | `radius-sm` |
| カード / モーダル | 16 | `radius-lg` |
| 大カード / バナー | 20 | `radius-xl` |

Tailwind config:

```ts
borderRadius: {
  sm:  "8px",
  DEFAULT: "12px",
  lg:  "16px",
  xl:  "20px",
}
```

## Shadow

| 用途 | token | css |
|---|---|---|
| 軽い浮き（list row hover） | `shadow-sm` | `0 1px 2px rgba(15, 23, 42, 0.04)` |
| 通常カード | `shadow` | `0 4px 12px rgba(15, 23, 42, 0.05)` |

Tailwind config:

```ts
boxShadow: {
  sm:      "0 1px 2px rgba(15, 23, 42, 0.04)",
  DEFAULT: "0 4px 12px rgba(15, 23, 42, 0.05)",
}
```

## 使い分けの原則

- カード / モーダル: `radius-lg` + `shadow`
- ボタン / chip: `radius` + shadow なし
- 写真表示: `radius-lg`、shadow なし（画像が主役、影は弱める）
- 警告 / エラー: shadow を出さず、border 強調で表現

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-27 | 初版（PR25b） |
