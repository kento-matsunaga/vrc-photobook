# 2026-04-25: @cloudflare/next-on-pages が deprecated、OpenNext adapter 推奨

## 発生状況

- 何をしようとしていたか: M1 PoC frontend (`harness/spike/frontend/`) で `npm install` を実行
- どのファイル/モジュールで発生したか: `package.json` の `@cloudflare/next-on-pages: ^1.13.0` 依存
- 関連: M1 スパイク検証計画 [`docs/plan/m1-spike-plan.md`](../../docs/plan/m1-spike-plan.md) §「OpenNext / next-on-pages の選択」

## 失敗内容

`npm install` 実行時、deprecation 警告が出力された:

```
npm warn deprecated @cloudflare/next-on-pages@1.13.16:
  Please use the OpenNext adapter instead: https://opennext.js.org/cloudflare
```

ビルド・実行自体はブロックされていない（`added 576 packages in 22s`、exit 0）。
ただし Cloudflare 公式が **OpenNext adapter** を Cloudflare Pages の推奨パスに切り替えていることが明確になった。

## 根本原因

- M1 計画と本 PoC の `package.json` は「@cloudflare/next-on-pages を第一候補」として書いたが、これは私（Claude Code）の知識カットオフ時点では公式推奨だった
- 実際には Cloudflare が `@opennextjs/cloudflare`（OpenNext adapter）に方針を切り替えており、`@cloudflare/next-on-pages` は deprecated 扱いになっている
- M1 計画 §「OpenNext / next-on-pages の選択」で「@cloudflare/next-on-pages を第一候補、OpenNext は比較対象」と書いたが、**事実関係が逆転していた**

## 影響範囲

- `harness/spike/frontend/package.json` の依存選定
- `harness/spike/frontend/README.md` の「OpenNext / next-on-pages の選択」セクション（§OpenNext 比較メモ）
- M1 計画 §「優先順位 1 検証手順」（OpenNext を先行検証する方針へ修正）
- ADR-0001 §M1 で必要なスパイク（OpenNext を第一候補として記載すべき）

## 対策種別

- [x] ADR / 計画への反映（ADR-0001 / M1 計画 / PoC README を更新）
- [x] PoC の依存切替検討（OpenNext adapter `@opennextjs/cloudflare` への移行）
- [ ] ルール化: 不要（個別の技術選定の検証結果）
- [ ] テスト追加: 不要

## 暫定対応（M1 検証中の判断）

**選択肢**:

### 案A: PoC を OpenNext adapter に切り替える（推奨）

- `package.json` から `@cloudflare/next-on-pages` を削除し、`@opennextjs/cloudflare` を追加
- `package.json` の `pages:build` / `pages:preview` スクリプトを OpenNext 用に書き換え
- README の OpenNext 比較メモを「OpenNext を採用、next-on-pages は deprecated のため見送り」に更新
- 残りの検証（OGP/Cookie/redirect/ヘッダ制御）はそのまま実行可能

### 案B: deprecation 警告を許容して next-on-pages のまま検証を続ける

- 動作はブロックされていないため、M1 検証目的（SSR/Cookie/redirect/ヘッダ）の達成は可能
- ただし将来的に動かなくなるリスクがあり、M2 本実装で再度切り替えが必要
- M1 PoC の検証結果が「next-on-pages 上で成立した」と記録されてしまうため、本実装側で再検証コストが発生

### 案C: 両方で検証する

- next-on-pages と OpenNext の両方でビルド・動作確認し、比較結果を ADR-0001 / M1 計画に反映
- 工数は増えるが、M1 計画 §6.1 案A〜D の判断材料が揃う

**推奨**: 案A（OpenNext に切り替え）。理由:

- Cloudflare 公式推奨に従う方が長期保守性が高い
- M1 PoC の検証結果が M2 本実装に直接適用できる
- 検証コストは package.json と数行の設定変更のみ

ただし、最終判断はユーザーに委ねる。ユーザーが案A〜C のいずれかを指示することで対応する。

## 反映先

検証結果が確定した時点で以下を更新する:

- [ ] `harness/spike/frontend/package.json`: 依存切替（案A 採用時）
- [ ] `harness/spike/frontend/README.md`: 「OpenNext / next-on-pages の選択」セクションを実際の状況に合わせて書き換え
- [ ] `docs/plan/m1-spike-plan.md`: §「OpenNext / next-on-pages の比較」記述を更新
- [ ] `docs/adr/0001-tech-stack.md`: §M1 で必要なスパイク → 検証結果セクションに「OpenNext adapter を採用」を追記

## 学び

- 知識カットオフ後に技術選定の推奨先が変わることがある
- M1 計画策定時に「公式推奨を最新ドキュメントで確認する」ステップを入れるべきだった（次回計画時の改善点）
- Cloudflare Pages 関連の推奨パスは 2024-2025 年で大きく動いているので、M1 PoC 全体で同種の問題が他にもある可能性がある

## ステータス

- 検出: 2026-04-25（M1 PoC `npm install` 時）
- 検証完了: 2026-04-25（next-on-pages 上で SSR / OGP / Cookie / redirect / ヘッダ制御がすべて成立することを curl 検証で確認）
- 対応方針: **案A 採用**（OpenNext adapter 版 PoC を別途検証 → M2 本実装は OpenNext 第一候補）
- 反映先:
  - `harness/spike/frontend/README.md` §検証結果（2026-04-25 next-on-pages）に記録
  - `docs/plan/m1-spike-plan.md` §「OpenNext / next-on-pages の選択」に再評価を追記
  - `docs/adr/0001-tech-stack.md` §M1 で必要なスパイク → 検証結果セクションに記録
- 残作業:
  - OpenNext adapter 版 PoC の追加検証（M1 中、本ファイル参照）
  - macOS Safari / iPhone Safari 実機検証
  - Cloudflare Pages 実環境デプロイ検証
  - 24h / 7 日後の Cookie 残存検証（ITP 影響評価）
- 完了判定: OpenNext 版 PoC 検証完了 + 実機 Safari 検証完了で本ログを完了扱いにする
