# 2026-04-27 Frontend ↔ Backend 実 token 結合確認 実施結果（PR16）

## 概要

`docs/plan/m2-frontend-workers-deploy-plan.md` PR16 計画に基づき、実 token を使った
本番相当フロー（`/draft/<token>` / `/manage/token/<token>` → Backend
`/api/auth/*-session-exchange` 200 → HttpOnly Cookie 発行 → 302 redirect）を
独自ドメイン `app.vrc-photobook.com` 上で確認した。

- 実施日時: 2026-04-27 02:14〜02:16 JST（proxy 起動〜cleanup まで約 2 分）
- 対象 URL:
  - `https://app.vrc-photobook.com/draft/<raw_draft_token>`
  - `https://app.vrc-photobook.com/manage/token/<raw_manage_token>`
- Backend: `https://api.vrc-photobook.com`（Cloud Run revision `vrcpb-api-00002-pdn`、DB あり）
- Cloud SQL: `vrcpb-api-verify`（PR17 完了後の判断まで残置継続）

## 前提

- PR15 で `app.vrc-photobook.com` Workers Custom Domain が稼働中
- PR12 で `api.vrc-photobook.com` Cloud Run Domain Mapping が稼働中
- PR15 までで invalid token 経路（401 → reason redirect）は確認済
- Cloud SQL Auth Proxy + ADC + Secret Manager `DATABASE_URL` 経由で DB 接続

## 一時 tokengen（コミット禁止）

- 配置: `backend/internal/photobook/_tokengen/main.go`（**作業終了後に削除済**）
- 内容: `CreateDraftPhotobook` UseCase で draft token を発行、もう 1 件 draft を作って
  `PublishFromDraft` で publish して manage token を発行。stdout には
  `DRAFT=<base64url 43>` / `MANAGE=<base64url 43>` の 2 行のみ出力。
- 起動方法: `DATABASE_URL=<proxy DSN> go -C backend run ./internal/photobook/_tokengen`
- 出力捕捉: 一時ファイル経由で受け取り、token 値そのものは echo / cat / chat に出さない
- `go build` の副産物 `backend/_tokengen` バイナリも cleanup で削除した

## 検証結果

### draft 実 token 経路 — `GET /draft/<raw_draft_token>`

- HTTP/2 **302** ✅
- `location: https://app.vrc-photobook.com/edit/<photobook_id (uuid)>` ✅
  - raw token は Location に含まれない（uuid のみ）
- `cache-control: no-store` ✅
- `referrer-policy: no-referrer`（sensitive path）✅
- `x-robots-tag: noindex, nofollow` ✅
- `set-cookie:` 1 行のみ
  - 名前: `vrcpb_draft_<photobook_id>` ✅
  - **`Domain=.vrc-photobook.com`** ✅
  - **`HttpOnly`** ✅
  - **`Secure`** ✅
  - **`SameSite=strict`** ✅
  - **`Path=/`** ✅
  - `Max-Age=604787`（≈ 7 日、TTL から経過した秒数を引いた値）
  - `Expires` も同期して 7 日後

### manage 実 token 経路 — `GET /manage/token/<raw_manage_token>`

- HTTP/2 **302** ✅
- `location: https://app.vrc-photobook.com/manage/<photobook_id (uuid)>` ✅
  - raw token は Location に含まれない
- `cache-control: no-store` / `referrer-policy: no-referrer` / `x-robots-tag: noindex, nofollow` ✅
- `set-cookie:` 1 行のみ
  - 名前: `vrcpb_manage_<photobook_id>` ✅
  - `Domain=.vrc-photobook.com` / `HttpOnly` / `Secure` / `SameSite=strict` / `Path=/` ✅
  - `Max-Age=604800`（= 7 日丁度）

### ヘッダ重複・本数確認

- draft / manage どちらも `x-robots-tag` / `referrer-policy` / `set-cookie` は **各 1 行のみ** ✅
- middleware と Route Handler の二重出力なし

### Backend (`api.vrc-photobook.com`) 200 確認

`gcloud logging read` で `vrcpb-api` の `/api/auth/*-session-exchange` 直近を確認:

| timestamp (Z) | endpoint | status |
|---|---|---|
| 17:15:43 | manage-session-exchange | **200** |
| 17:15:42 | draft-session-exchange | **200** |

- Workers (`app.vrc-photobook.com`) → Cloud Run (`api.vrc-photobook.com`) の
  HTTPS fetch + Backend 側の domain logic（hash 一致 → session 発行）が
  実環境で初めて 200 で成立した ✅
- request URL path には raw token が出ない（POST body のため access log には記録されない）

### 漏洩 grep

Backend Cloud Run logs（直近 500 行）:

```
gcloud run services logs read vrcpb-api --region=asia-northeast1 --limit=500 |
  grep -iE "(SECRET|API_KEY|PASSWORD|PRIVATE|sk_live|sk_test|draft_edit_token|
            manage_url_token|session_token|set-cookie|DATABASE_URL=)"
```

→ **マッチなし** ✅

curl response file（一時、cleanup で削除済）:

- `grep -F "${DRAFT_RAW}" /tmp/vrcpb-draft-e2e-response.txt /tmp/vrcpb-manage-e2e-response.txt` → **NO_LEAK** ✅
- `grep -F "${MANAGE_RAW}" /tmp/vrcpb-draft-e2e-response.txt /tmp/vrcpb-manage-e2e-response.txt` → **NO_LEAK** ✅
- raw token は Location / Set-Cookie value / response body のいずれにも含まれない

## 一時ファイル・一時コード削除

| 対象 | 結果 |
|---|---|
| `backend/internal/photobook/_tokengen/` | 削除済 ✅ |
| `backend/_tokengen`（go build 副産物バイナリ）| 削除済 ✅ |
| `/tmp/vrcpb-draft-e2e-response.txt` | 削除済 ✅ |
| `/tmp/vrcpb-manage-e2e-response.txt` | 削除済 ✅ |
| `/tmp/tmp.*`（token 一時ファイル）| 削除済 ✅ |
| Cloud SQL Auth Proxy プロセス | 停止済 ✅ |
| 環境変数 `DB_PASSWORD` / `DATABASE_URL_PROXY` / `DRAFT_RAW` / `MANAGE_RAW` | unset 済 ✅ |
| git status | clean（作業ログのみ追加予定）|

## 実施しなかったこと

- Safari / iPhone Safari 実機確認（PR17）
- Cloud SQL `vrcpb-api-verify` 削除（PR17 完了後の判断まで継続）
- Backend 変更 / Workers 再 deploy
- Cloudflare DNS 変更
- SendGrid / Turnstile / R2 設定
- 本番 router への debug endpoint 追加
- dummy token 成功経路の追加
- tokengen コードのコミット
- raw token / Cookie 値 / DATABASE_URL / DB password / Secret payload の表示・記録
- response body 全文の表示・記録

## 切戻し手順（参考、本書では実施しない）

実 token 結合自体には差し戻すべき変更がない（一時 tokengen は既に削除、Backend 変更なし、
Frontend 再 deploy なし）。問題が出た場合は以下を順に確認:

1. `app.vrc-photobook.com` の Worker 状態（PR14 の Version ID `9736fe88-...`）
2. `api.vrc-photobook.com` の Cloud Run revision（`vrcpb-api-00002-pdn`、DB あり）
3. Cloud SQL `vrcpb-api-verify` の RUNNABLE 状態
4. Secret Manager `DATABASE_URL` の latest enabled

## 費用

- 検証時間 ~2 分。Workers / Cloud Run / Cloud SQL ともに既存稼働の延長
- Cloud SQL 残置継続中、累計経過 ~5.5 時間 / ~¥13

## 次のステップ

PR17: Safari / iPhone Safari 実機確認
- macOS Safari 最新で `app.vrc-photobook.com/draft/<token>` → `/edit/<id>` 遷移後に
  HttpOnly Cookie が DevTools で確認できる
- iPhone Safari 最新で同経路を実機確認、redirect 後の Cookie 維持を目視
- 24 時間 / 7 日後の Cookie 残存（ITP 長期影響）は継続観察項目として記録
- `.agents/rules/safari-verification.md` に従って必須チェックリストを実施

PR17 完了後: Cloud SQL `vrcpb-api-verify` の保持/削除判定（必須判断）

## 関連

- [PR13 計画書](../../docs/plan/m2-frontend-workers-deploy-plan.md)
- [PR14 Frontend Workers Deploy 結果](./2026-04-27_frontend-workers-deploy-result.md)
- [PR15 Frontend Custom Domain 結果](./2026-04-27_frontend-custom-domain-result.md)
- [PR12 Backend Domain Mapping 結果](./2026-04-27_backend-domain-mapping-result.md)
- [Post-deploy Final Roadmap §A](./2026-04-27_post-deploy-final-roadmap.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
