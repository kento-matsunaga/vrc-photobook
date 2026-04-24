# design/mockups

VRC PhotoBook のデザインモックアップとプロトタイプ。

## サブディレクトリ

| ディレクトリ | 種別 | 内容 |
|------------|-----|-----|
| `prototype/` | 動くプロトタイプ | React（Babel standalone）で動作する MVP プロトタイプ。iOS フレーム＋PC スクリーンを単一 HTML で表示 |
| `concept-images/` | コンセプト画像 | ChatGPT Image 生成によるコンセプトビジュアル 15 枚 |

## prototype/ の起動

外部ビルド不要。`VRC PhotoBook.html` をブラウザで直接開けば動く。

```bash
# 簡易HTTPサーバー経由で開くとCORS問題を避けられる
cd design/mockups/prototype
python3 -m http.server 8000
# → http://localhost:8000/VRC%20PhotoBook.html
```

## ファイル一覧（prototype/）

| ファイル | 役割 |
|--------|-----|
| `VRC PhotoBook.html` | エントリ HTML（React/Babel の CDN 読み込み） |
| `design-canvas.jsx` | モック全体の土台となるキャンバスレイアウト |
| `ios-frame.jsx` | iOS の端末フレームコンポーネント |
| `screens-a.jsx` / `screens-b.jsx` | モバイル画面モック群 |
| `pc-screens-a.jsx` / `pc-screens-b.jsx` | PC 画面モック群 |
| `shared.jsx` / `pc-shared.jsx` | 共通コンポーネント |
| `styles.css` / `pc-styles.css` | スタイル定義 |

## コンセプト画像（concept-images/）

命名: `concept-NN.png`（連番）。元ファイル名は `ChatGPT Image ...` 形式だったが、日本語・全角カッコ・スペースが CI/Git で扱いづらいため整理済み。

これらは**業務知識定義書 v4 の内部タイプ分類**（event / daily / portfolio / avatar / world / memory / free）を視覚化する参考資産。

## 設計システムとの関係

- カラー・タイポ等の確定仕様は `/design/design-system/` に別途記述予定
- ここに置かれるモックは「探索段階の成果物」であり、唯一の正典ではない
- 本実装のフロントエンドは `/frontend/` で構築し、ここで探ったアイデアを取り込む
