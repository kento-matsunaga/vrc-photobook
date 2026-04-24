# frontend/

フロントエンドアプリケーションのコード格納先。

## 予定スタック

技術選定は仕様確定後に決定。候補:
- Next.js / Astro / Vite + React / SvelteKit
- Cloudflare Pages デプロイ想定

## ディレクトリ方針（実装開始後に整備）

```
frontend/
├── src/
│   ├── components/     # UIコンポーネント
│   ├── features/       # 機能単位のモジュール
│   ├── pages/          # ルーティング
│   ├── lib/            # 共通ユーティリティ
│   └── styles/         # グローバルスタイル
├── public/             # 静的アセット
└── package.json
```

## デザインとの連携

- デザイン原本: `design/` を参照
- デザインシステム定義: `design/design-system/`
- コンポーネント実装時は `design/mockups/` の該当モックと照合
