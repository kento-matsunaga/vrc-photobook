# PR 終了時コメント整合チェックルール

## 適用範囲

すべての PR / 作業サイクルの **完了報告を出す直前**に必ず実施する。
PR の規模によらず（1 ファイル修正でも、20 ファイル横断 PR でも）対象とする。

## 原則

> **「コードは完成したけどコメントが過去のまま」を作らない。**

実装が進むほど、過去 PR のコメントが嘘になる確率が上がる。これを PR 完了処理として
強制的に確認しないと、AI 実装・レビュー・新規参画者の混乱源になる
（実例: 2026-04-28 独立タスク B で 9 既知 + 多数の追加発見を整理）。

## 1. PR 完了時に必ず実施すること

完了報告（チャット返信 / work-log 記録 / commit）の **直前**に、以下を順に行う。

1. **新規・更新したコードコメントを確認**: 自分が今 PR で書き加えたコメントを再読し、
   将来劣化しそうな PR 番号 + 未来形になっていないかをセルフレビュー
2. **PR 番号付きコメントを検索**: §2 の grep keyword で全体検索（自分の編集箇所外も含む）
3. **「後続 PR」「未実装」「未接続」「TODO」「FIXME」を検索**: 同上
4. **コメントが実装状況と矛盾していないか確認**: §3 で各ヒットを 4 区分に分類
5. **先送り事項がある場合は新正典に記録されているか確認**:
   `docs/plan/vrc-photobook-final-roadmap.md` / 該当 PR 計画書 / ADR / runbook /
   failure-log のいずれかに必ず残す
6. **README / docs / runbook / plan の古い記述も確認**: コメントだけでなく
   markdown 内の「PR9c までで未実装」のような時間経過で古くなる記述もチェック
7. **generated file は直接編集しない**: sqlcgen / .open-next 等は **生成元** を
   修正し、次回 generate で追従する。生成物に手を入れてはいけない

## 2. 検索キーワード（最低限これだけは検索する）

### 必須

| カテゴリ | キーワード |
|---|---|
| PR 番号系 | `PR[0-9]+` / `PR [0-9]+` / `PR9c` 等の固有番号表記 |
| 未来形 | `後続 PR` / `後続PR` / `後続` / `本 PR では` / `本PRでは` / `実装予定` |
| 未実装 | `未接続` / `未実装` / `TODO` / `FIXME` |
| 英語未来形 | `future` / `later` / `not connected` / `not implemented` / `placeholder` |
| 廃止前提 | `SendGrid` / `SES` |
| 機能名 | `Outbox INSERT` / `provider` / `再発行` |

### PR 内容に応じて追加

- 認可・session 系の PR → `Set-Cookie` / `dummy token`
- 画像系 PR → `presigned` / `R2 orphan` / `libheif`
- 集約追加 PR → 該当集約名 / FK 関連語

## 3. 分類ルール（各ヒットを 4 区分のどれかに振る）

### A. 修正する

- 実装済みなのに「未実装」「後続 PR で実装」と書いてある
- 現行方針 / ADR と矛盾している（例: SendGrid 採用前提の記述が ADR-0006 で破棄）
- 廃止された Provider / 設計を前提にしている
- PR 番号が古く、現状説明として不正確（例: 「PR8 では未接続」だが既に router 配線済）

### B. 残してよい（正しい TODO として残す）

- 本当に未実装で、新正典ロードマップに次の PR 番号と一緒に予定が残っている
- 機能としては「MVP 範囲外」「provider 確定後に追加」など状態説明として正しい
- 残すときは **状態ベース表現**（「未実装」「未確定」「ADR-0006 後続」等）にする。
  「PR32 で実装」のような **固定 PR 番号 + 未来形は避ける**

### C. 残してよい（過去経緯として正しい）

- migration ファイル（`-- PR9a:` 等）の歴史記述
- failure-log / work-log の過去記録
- 設定ファイルの段階的有効化メモ（`.env.example` 等）
- Dockerfile / docker-compose の経緯コメント

### D. 生成元を直す（generated file）

- sqlcgen / .open-next / OpenAPI generated client 等に古いコメント
- **直接編集禁止**。生成元 SQL / OpenAPI / spec 等を修正
- 今 PR で sqlc 再生成する場合は反映、しない場合は次回 generate 時に追従させる旨を
  完了報告に明記

## 4. コメント記法ルール（今後の新規コメント）

### 悪い例（書かないこと）

- `// PR8 では未接続`
- `// 後続 PR で実装`
- `// PR32 で SendGrid 連携`
- `// 本 PR では Outbox INSERT しない`
- `// 後で対応する`

### 良い例（こう書くこと）

- `// Draft session middleware は router.go で配線済`
- `// Email provider は ADR-0006 で再選定中。決着後に追加`
- `// 本 method は同一 TX 内で outbox event を INSERT する`
- `// Manage URL の再送経路は provider 依存。MVP は完了画面 1 回表示が標準`
- `// MVP 範囲外（衝突発生時は handler が 409）`

### 共通原則

- **固定 PR 番号より、現在の責務 / 機能名 / 状態を書く**
- 後続予定をコメントに残す場合は「未実装（〜が決まり次第追加）」のような **状態ベース表現**
- 直近の commit 文脈（「本 PR では」）に依存しない言い回しにする
- 日本語 / 英語のどちらでも同じ原則

## 5. 先送り事項の扱い

PR 中に「今は実装しない」と判断した項目は、必ず以下のいずれかに記録する。

| 種別 | 記録先 |
|---|---|
| ロードマップレベル（後続 PR で実装予定） | `docs/plan/vrc-photobook-final-roadmap.md` |
| その PR 固有の判断（小さい枝葉） | 対応する PR 計画書 / 実装書 |
| 設計判断 | ADR（必要なら新規作成 or 既存更新） |
| 運用手順 | runbook（`docs/runbook/`） |
| 失敗から派生した先送り | `harness/failure-log/` |
| 進捗スナップショット | `harness/work-logs/` |

「今はやらない」と判断したら、必ず **「いつ・どの PR 以降で再検討するか」** を書く。
判断時刻だけ書いて再検討タイミングを書かないのは禁止（永久に忘れられる）。

## 6. PR 完了報告に含める項目

今後のすべての PR 完了報告（チャット返信 / work-log）に **以下のチェックリスト**を含める。

- [ ] コメント整合チェック実施（§1 / §2 の grep を実行）
- [ ] 古いコメントを修正した（または「該当なし」と明記）
- [ ] 残した TODO とその理由（§3 の B / C / D 区分）
- [ ] 先送り事項がロードマップ等に記録済み（§5 の記録先を明記）
- [ ] generated file に未反映コメントがあるか（次回 generate で追従する旨も）
- [ ] Secret 漏洩 grep 結果（`.agents/rules/security-guard.md` の禁止リスト）

このチェックリストが**埋まっていない完了報告は完了扱いにしない**。

## 7. スクリプト補助

`scripts/check-stale-comments.sh` を補助ツールとして用意（本ルール作成時に追加）。

```bash
bash scripts/check-stale-comments.sh
```

挙動:

- backend / frontend / docs / .agents / CLAUDE.md / README.md を grep
- harness/work-logs / harness/failure-log / node_modules / .next / .open-next /
  .wrangler / sqlcgen / migrations は除外
- stale 候補を出力（0 件でなくても exit 0）
- **判断は人間 / Claude Code の責務**。スクリプトは補助で、各ヒットを §3 の
  4 区分に分類することはユーザー確認 + Claude Code が行う

## Why（なぜこのルールが必要か）

2026-04-28 独立タスク B で実際に発生した問題:

- `publish_from_draft.go`「Outbox INSERT は本 PR では行わない」が PR30 で実装済なのに残存
- `router.go`「PR8 未接続のまま」が PR9c 以降の接続後も残存
- `manage_handler.go` / `get_manage_photobook.go`「PR32 で SendGrid + Outbox 経由」
  が ADR-0006 で SendGrid 採用破棄後も残存
- `frontend/middleware.ts`「PR5 段階では /draft, /manage, /edit のルートは未実装」が
  PR15+ で実装済なのに残存
- `backend/README.md` 全体が PR9c 時点の記述で停止
- `health.sql.go` 生成物に古いコメントが残存（生成元 SQL も古かった）

これらは AI 実装時に「未実装と書いてあるから実装する」「SendGrid を呼ぶコードを足す」
等の誤動作リスク、レビュー時の混乱、新規参画者への嘘情報提供を引き起こす。

PR 完了処理として体系化することで、各 PR の完了直前に検出 → 修正 or 状態ベース
表現への置換 → 先送り事項の正典記録の一連の動きを習慣化する。

## 関連

- `.agents/rules/coding-rules.md` — 明示的 > 暗黙的（コメントも同様）
- `.agents/rules/feedback-loop.md` — 失敗の再発防止としての本ルールの位置付け
- `.agents/rules/security-guard.md` — Secret 漏洩 grep の禁止リスト
- `docs/plan/vrc-photobook-final-roadmap.md` §0 — コードコメント記法ルールの本体
- `harness/work-logs/2026-04-28_outbox-result.md` — 独立タスク B 実施記録（参考事例）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版作成。PR30 完了後の独立タスク B（古いコメント整理）の知見を運用ルール化 |
