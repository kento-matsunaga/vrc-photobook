# design/

フロントエンドのデザイン資産を一元管理する場所。

## ディレクトリ構成

| パス | 目的 | 主な内容 |
|------|------|---------|
| `mockups/` | 画面モックアップ | PNG/JPG/PDF エクスポート、画面単位の完成イメージ |
| `wireframes/` | ワイヤーフレーム設計入力 | 画面一覧、導線、状態、API 対応 |
| `design-system/` | デザインシステム定義 | カラー・タイポ・余白・UIコンポーネント仕様（Markdown/JSON） |
| `figma-exports/` | Figma原本エクスポート | `.fig` ファイル、Figma から書き出した SVG 等 |
| `assets/` | デザイン素材 | ロゴ、アイコン、画像素材（原本） |

## 命名規則

- モックアップ: `{screen-name}_{state}_{yyyymmdd}.png`
  - 例: `photobook-editor_default_20260424.png`
- デザインシステムドキュメント: `{topic}.md`
  - 例: `colors.md`, `typography.md`, `spacing.md`, `components.md`

## 運用

1. Figma で編集 → `figma-exports/` に原本を配置
2. 画面ごとのモックアップを `mockups/` に書き出し
3. デザインシステムの変更は `design-system/` のドキュメントに反映
4. フロント実装は `design-system/` を Single Source of Truth とする

## ワイヤーフレーム作成

- 画面一覧・導線・状態整理: [`wireframes/system-wireframe-brief.md`](./wireframes/system-wireframe-brief.md)
- 探索モック: [`mockups/prototype/`](./mockups/prototype/)
