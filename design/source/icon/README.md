# icon source

VRC PhotoBook のブラウザタブアイコン / Apple touch icon / PWA icon の生成元素材を保管する。

## ファイル

| ファイル | 内容 |
|---|---|
| `photobook-icon.png` | user-provided icon source (1254×1254 PNG)。`frontend/app/icon.png` ほかの生成元 |

## ライセンス・出所

- 出所: user-provided（プロジェクトオーナーが自作 / 自分で生成した素材）
- 公開サイト掲載・商用利用ともに権利上問題なし（2026-05-10 確認）

## 生成物

本素材から ImageMagick (Lanczos filter) でリサイズして以下を生成している。

| 生成先 | サイズ | 用途 |
|---|---|---|
| `frontend/app/icon.png` | 32×32 | Chrome / Firefox / Edge タブ favicon（Next.js 自動検出） |
| `frontend/app/icon1.png` | 512×512 | 高 DPI / PWA / Android ホーム（Next.js 自動検出、`icon{N}` 規約） |
| `frontend/app/apple-icon.png` | 180×180 | iOS ホーム画面追加（Next.js 自動検出） |
| `frontend/app/favicon.ico` | 16/32/48 multi | 旧ブラウザ・`/favicon.ico` 直接アクセス対応（Next.js 自動配信） |

> Next.js App Router では `app/` 配下のこれらのファイルは convention で自動的に
> `<link rel="icon">` / `<link rel="apple-touch-icon">` が `<head>` に挿入される。
> 命名規則上ダッシュ付き番号 (`icon-512.png`) は検出されないため、複数 PNG は
> `icon.png` / `icon1.png` のように数字サフィックスのみを使う。

再生成手順:

```bash
SRC=design/source/icon/photobook-icon.png
convert "$SRC" -filter Lanczos -resize 32x32   -strip frontend/app/icon.png
convert "$SRC" -filter Lanczos -resize 180x180 -strip frontend/app/apple-icon.png
convert "$SRC" -filter Lanczos -resize 512x512 -strip frontend/app/icon1.png
convert "$SRC" -filter Lanczos -resize 16x16   -strip /tmp/ico-16.png
convert "$SRC" -filter Lanczos -resize 32x32   -strip /tmp/ico-32.png
convert "$SRC" -filter Lanczos -resize 48x48   -strip /tmp/ico-48.png
convert /tmp/ico-16.png /tmp/ico-32.png /tmp/ico-48.png frontend/app/favicon.ico
```

## デザイン情報

- 構図: ダークティール (#0F2A2E ≒ design system `--ink`) 背景にクリーム色のフォトカードと
  ページめくりのモチーフ。波・太陽の図像入り
- design token (`design/source/project/wireframe-styles.css`) と色が一致しており、
  `frontend/app/layout.tsx` の `viewport.themeColor` も `#0F2A2E` で揃えている
