# iPhone Safari /create が一時的に Turnstile 403 連発、観測 log を deploy したら自然消失

## 発生日

2026-05-09 〜 2026-05-10。Safari smoke 中に user が再現報告。

## 症状

`https://app.vrc-photobook.com/create` で iPhone Safari (iOS 18.7 / Safari Version/26.4)
からの `POST /api/photobooks` が **403 turnstile_failed を連発**。

Cloud Run access logs（24h 抜粋）:

| 時刻 (UTC) | UA | 結果 |
|---|---|---|
| 2026-05-09T10:59:17 | iPhone Safari | 403 |
| 2026-05-09T10:59:23 | iPhone Safari | 403 |
| 2026-05-09T11:06:17 | iPhone Chrome (CriOS) | 201 ✓ |
| 2026-05-10T06:03:05 | iPhone Safari | 403 |
| 2026-05-10T06:36:42 | iPhone Safari | **201 ✓**（観測 log deploy 後） |
| 2026-05-10T06:37:02 | iPhone Safari | **201 ✓** |

同時刻帯の同 iPhone で **CriOS は 201 成功 / Safari は 403** という Safari 固有パターン。
今回の Workers icon/themeColor deploy（commit `37d7744` / `56acf36` 前後）よりも前から
発生していたため deploy 起因ではない。

## 根本原因

**未確定**（diagnostic を deploy した直後に再現性消失したため）。

Cloudflare Turnstile siteverify が `success=false` を返すこと自体は handler 側で 403 に
変換されている動作で、原因として最もありえるのは:

- Safari ITP / Lockdown Mode が Cloudflare challenge の cookie / storage を block
  → token が invalid / fingerprint 不整合で reject
- 短時間の連続再試行で Cloudflare 側が `timeout-or-duplicate` 系で reject
- iOS 18.7 + Safari Version/26.4 の特定ビルドの Turnstile JS 互換性問題

deploy 完了後 30 分以内で iPhone Safari からの 201 成功が連続しているため、上記いずれかの
state が自然回復した可能性が高い。**根本原因を特定するための diagnostic は本番に常設**
（後述）。

## 観測 log hotfix（再発時の自動診断データ取得）

commit `4e935a9` で `backend/internal/photobook/interface/http/create_handler.go` の
Turnstile siteverify 失敗パスに `slog.Warn` を追加。

出力 fields（raw token / Cookie / IP / UA 全文は出さない）:

- `event`: `"turnstile_verify_failed"`
- `route`: `"/api/photobooks"`
- `error`: error chain の文字列（package error 名のみ）
- `error_codes`: Cloudflare 公開 enum の `[]string`（例: `["timeout-or-duplicate"]`）
- `got_hostname` / `got_action`: siteverify response が返した値
- `ua_class`: `"ios-safari"` / `"ios-chromium"` / `"macos-safari"` / `"other"`

検索クエリ（再発時の診断用）:

```
resource.type=cloud_run_revision
AND jsonPayload.event="turnstile_verify_failed"
AND jsonPayload.ua_class="ios-safari"
```

## 影響範囲

- **本番データへの影響**: なし（403 は client error、副作用 commit なし）
- **ユーザ影響**: 一部 iPhone Safari ユーザが /create に進めない時間帯があった
- **deploy 影響**: なし（観測 log のみ追加、ロジック不変）

## 対策

- [x] **観測 log 追加**（`backend/internal/photobook/interface/http/create_handler.go` +
  `create_handler_test.go`、commit `4e935a9`）。常設で残す方針
- [x] **production smoke** で synthetic dummy token → 403 + 観測 log entry 確認済
- [ ] 再発時に `error_codes` を取得 → 根本原因確定 → 必要なら個別対策（例:
  challenge state hint を Frontend に追加 / Lockdown Mode 検知 / 等）

## 関連

- `.agents/rules/turnstile-defensive-guard.md` L0-L4
- `harness/failure-log/2026-04-29_turnstile-widget-remount-loop.md`（widget 安定 mount）
- `harness/failure-log/2026-05-03_turnstile-upload-verification-race.md`（single-use token race）
- commit `4e935a9` `fix(observability): log turnstile verification failure codes`

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-10 | 初版作成。観測 log を本番常設、再発時の error_codes 取得経路を確立 |
