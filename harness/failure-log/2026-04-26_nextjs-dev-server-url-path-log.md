# 2026-04-26 Next.js dev server のリクエストログに raw token を含む URL path が出る

## 発生状況

PR10 の Frontend Route Handler `/draft/[token]` / `/manage/token/[token]` を実装し、ローカル E2E 確認のため `npm run dev` 起動した frontend に対して `curl http://localhost:3000/draft/AAA...` を投げた。

その結果、frontend dev server (`next dev` 標準のリクエストロガー) が以下のように URL path をそのまま stdout に出力した。

```
 GET /draft/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA 302 in 11054ms
 GET /manage/token/BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB 302 in 492ms
```

## 失敗内容

- URL path に乗っている raw `draft_edit_token` / `manage_url_token` が **dev server のログに直接出る**
- Next.js dev server 標準のロガーで、`middleware.ts` よりも前段で発火しているため middleware 側で URL を redact できない
- これは Next.js の仕様（next dev の console output）

## 根本原因

- `next dev` は HTTP request を受けると最初に「method + path + status + duration」を stdout に書く
- token を URL path に持つ設計（ADR-0003）と組み合わせると、dev server ログに raw token が露出する
- 設計上の前提: 本来 raw token は Cookie に乗せるべきで、URL は redirect で消すが、入場 GET の URL は token を含まざるを得ない

## 影響範囲

- **本番（OpenNext / Workers）では発生しない**: `next dev` のロガーは production build で無効化される
- **ローカル開発時**に開発者の terminal / log にだけ raw token が残る
- 開発者の手元の shell history / scrollback に raw token が落ちる可能性
- 共有 PC で開発する場合のリスク

## 対策種別

- [x] **対策しない（許容）**: 本番影響なし、開発者は raw token の取り扱いに自律的に注意する
- [x] frontend/README.md と本 failure-log で開発者へ周知
- [ ] Next.js dev server の logger を差し替える（`next.config.mjs` の experimental option / カスタム logger）→ コスト高、今回は保留
- [ ] tail / less でログを保存しない運用ルール → 必要なら別途検討

## 補足: Backend 側はログに出ない

PR9c の backend handler は slog 経由で **token / hash / Cookie をログ出力しない方針** で実装済。
dev server の素朴な logger と区別する必要がある。本問題は frontend dev server だけ。

## 再発防止メモ

- PR で Frontend Route Handler を増やすときは、本問題が発生する旨を必ず frontend/README に明記
- 本番 deploy 前に Workers のログ設定を確認し、URL path の自動ロギングが無効化されていることを再確認（`safari-verification.md` の確認項目に追加検討）
- 共有 PC では frontend dev server のターミナル出力を tail / less でファイル化しない

## 関連

- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
- [`docs/adr/0003-frontend-token-session-flow.md`](../../docs/adr/0003-frontend-token-session-flow.md)
- [`docs/plan/m2-photobook-session-integration-plan.md`](../../docs/plan/m2-photobook-session-integration-plan.md) §12
