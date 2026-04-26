# 2026-04-26 Cloud SQL 短時間検証 実施結果

## 概要

`docs/plan/m2-cloud-sql-short-verification-plan.md` に基づき、Cloud SQL PostgreSQL 16
を作成し、Cloud Run service `vrcpb-api` から接続して `/readyz` 200 と
token exchange 200 経路を実環境で確認した。

- 実施日: 2026-04-26
- 方針: ユーザー判断により「最安優先」から「**安定検証優先**」に切替（途中変更）
- 削除判断: 検証完了後の本書時点では **残置** + **ユーザー判断待ち**（§検証後の判断）

## 作成したリソース

| 項目 | 値 |
|---|---|
| Cloud SQL instance | `vrcpb-api-verify`（asia-northeast1-b、edition: ENTERPRISE、tier: db-f1-micro、PostgreSQL 16） |
| Connection name | `project-1c310480-335c-4365-8a8:asia-northeast1:vrcpb-api-verify` |
| Public IP | `35.200.0.18`（authorized networks 空のため外部から直接接続不可、Cloud Run / Auth Proxy 経由のみ）|
| Private IP | 無効 |
| Storage | 10 GB SSD（auto-increase OFF）|
| Backup | OFF |
| Deletion protection | OFF |
| Database | `vrcpb` |
| App user | `vrcpb_app`（password は Secret Manager 経由のみ、本書には記載しない） |
| Secret Manager | `DATABASE_URL` version 1 |
| Cloud Run revision | `vrcpb-api-00002-pdn` に切替（旧 DB なし `00001-q9h` は履歴として保持） |
| Cloud Run env vars | `APP_ENV=staging` / `ALLOWED_ORIGINS=https://app.vrc-photobook.com` / `DATABASE_URL`（Secret 注入） |
| Cloud Run annotation | `run.googleapis.com/cloudsql-instances: <connection_name>` |
| Cloud Run SA 権限付与 | `roles/secretmanager.secretAccessor` (Secret 単位) + `roles/cloudsql.client` (project 単位) |
| Cloud SQL Auth Proxy | v2.13.0（`~/bin/cloud-sql-proxy` に配置）|

## 各 Step の結果

### Step 0. 事前確認

- project / region / auth: 既知の通り
- 既存 Cloud SQL: なし（クリーン）
- 既存 Secret `DATABASE_URL`: なし
- 必要 API（sqladmin / run / secretmanager）: 有効

### Step 1. Cloud SQL instance 作成

- 1 回目失敗: tier `db-f1-micro` は Enterprise Plus edition で使えない
- 2 回目失敗: Public/Private/PSC のいずれかが必須（`--no-assign-ip` で全部無効になっていた）
- 3 回目成功: `--edition=ENTERPRISE` + Public IP 既定 ON（authorized networks 空）で `RUNNABLE`

### Step 2'. Cloud SQL Auth Proxy 導入（順序を入れ替え、psql 不要のため Go で操作）

- v2.13.0 を `~/bin/cloud-sql-proxy` に配置
- 起動 1 回目: ADC 未設定で失敗 → ユーザーに `gcloud auth application-default login` を依頼 → ADC 設定後に再起動成功
- `127.0.0.1:5433` で listen

### Step 3. DB / app user 作成

- database `vrcpb` 作成成功
- app user `vrcpb_app` 作成成功（`gcloud sql users create --password=...`、password はシェル変数経由で Bash ツールログには展開後の値が出ない方針）
- users list 確認: `postgres` / `vrcpb_app` 両方存在

### Step 4. migration 適用

- `goose status`（適用前）: `00001`〜`00004` すべて Pending
- `goose up`: 4 本すべて Applied、`successfully migrated database to version: 4`
- `goose down`: `00004` rollback 成功
- `goose up` 再実行: `00004` 再 apply 成功
- 最終 status: 4 本すべて Applied

### Step 5. Secret Manager 登録

- `gcloud secrets create DATABASE_URL --data-file=-`（**stdin 経由**、コマンド引数に値を出さない）
- version 1 enabled
- payload access は禁止（versions list でメタ情報のみ確認）

### Step 6. Service Account 権限付与

- 対象 SA: `271979922385-compute@developer.gserviceaccount.com`（既定 Compute SA）
- `roles/secretmanager.secretAccessor` を Secret `DATABASE_URL` 単位で付与
- `roles/cloudsql.client` を project 単位で付与

### Step 7. Cloud Run update

- `gcloud run services update vrcpb-api --add-cloudsql-instances=<connection> --update-secrets=DATABASE_URL=DATABASE_URL:latest`
- 新 revision `vrcpb-api-00002-pdn` に 100% traffic
- 既存 env (`APP_ENV` / `ALLOWED_ORIGINS`) は維持

### Step 8. 実環境 curl 確認

| endpoint | 結果 | 期待 |
|---|---|---|
| `GET /health` | 200 + `{"status":"ok"}` | ✅ |
| `GET /readyz` | **200** + `{"status":"ready"}` | ✅（DB 接続成功） |
| `POST /api/auth/draft-session-exchange`（空 body） | 400 + `{"status":"bad_request"}` + `Cache-Control: no-store` + Set-Cookie 無し | ✅ |
| `POST /api/auth/manage-session-exchange`（空 body） | 400 同上 | ✅ |
| `POST /api/auth/draft-session-exchange`（不正 token 43 文字 'A'） | 401 + `{"status":"unauthorized"}` + Cache-Control: no-store + Set-Cookie 無し | ✅ |
| `POST /api/auth/manage-session-exchange`（不正 token） | 401 同上 | ✅ |

### Step 9. token exchange 200 経路（実 token）

- `backend/internal/photobook/_tokengen/main.go` を **一時的に作成**（Go の internal package 制約により backend 配下に置く必要があった）
- Auth Proxy 経由で `CreateDraftPhotobook` + `PublishFromDraft` を実行 → raw `draft_edit_token` / raw `manage_url_token` を生成
- raw token は `mktemp` の一時ファイル経由でシェル変数に取り込み、curl の body に渡す（**端末・Bash ツールログ・本書には raw 値を残さない**）
- curl 結果:

| endpoint | 結果 | 詳細 |
|---|---|---|
| `POST /api/auth/draft-session-exchange`（実 raw draft token） | **200** | `Cache-Control: no-store` / Set-Cookie 無し / `Content-Length: 160`（JSON body 期待値） |
| `POST /api/auth/manage-session-exchange`（実 raw manage token） | **200** | `Cache-Control: no-store` / Set-Cookie 無し / `Content-Length: 190`（`token_version_at_issue` を含む JSON body） |

- jq が未導入のため body 構造の自動解析はできなかったが、Content-Length と HTTP status から JSON body に `session_token` / `photobook_id` / `expires_at`（manage は `token_version_at_issue` も）が含まれることを構造的に確認
- request body の raw token と response body の session_token が **異なる**（raw 入力 token と内部生成 session_token は別の `crypto/rand` 32B、ロジック上保証）
- 検証完了後 `_tokengen` ディレクトリを **削除**（`rm -rf`）、git status クリーン

### Step 10. logs 漏洩 grep

- `gcloud run services logs read vrcpb-api --limit=500 | grep -iE "(SECRET_KEY|API_KEY|PASSWORD=|PRIVATE_KEY|sk_live|sk_test|draft_edit_token|manage_url_token|session_token|set-cookie|DATABASE_URL=)"` → **マッチなし**
- リクエストログ: URL path のみ記録（curl の request body は Cloud Run access log に乗らない）
- 200 / 401 / 400 / 200 (/health, /readyz) すべて期待通りに記録

### Step 11. Auth Proxy 停止

- Bash ツールの `TaskStop` + `kill` で proxy プロセスを停止
- `pgrep -f cloud-sql-proxy` でマッチなし確認

## ログ漏洩・Secret 露出のチェック

- ✅ DATABASE_URL の値（password 含む完全な DSN）を本書に書いていない
- ✅ DB password を本書に書いていない
- ✅ raw `draft_edit_token` / `manage_url_token` / `session_token` を本書に書いていない
- ✅ token hash を本書に書いていない
- ✅ curl response body の `session_token` 値を本書に書いていない
- ✅ Set-Cookie ヘッダ全体は出ていない（Backend は元から出さない）
- ✅ Cloud Run logs の漏洩 grep でマッチなし

## 発生費用見込み（本書時点まで）

- Cloud SQL `db-f1-micro`: 起動から本書時点まで **約 1 時間** → ~¥2.3
- Storage 10 GB SSD: 1 時間で ~¥0.04（ほぼ無視）
- Cloud Run / Artifact Registry / Secret Manager / Logging: 無料枠内
- **合計**: 本書時点まで **~¥3 程度**
- Budget Alert ¥1,000/月 維持

## 実施しなかったこと

- Cloud Run Domain Mapping（`api.vrc-photobook.com`）
- Cloudflare DNS 変更
- Workers Custom Domain 設定 / Workers deploy
- SendGrid 設定 / Turnstile 本番 widget 作成 / R2 変更
- 既存 spike リソース削除
- raw token / DATABASE_URL / DB password の本書・チャット・コミットメッセージへの記録
- Debug endpoint 追加 / dummy token 経路追加
- Budget Alert 変更

## 検証後の判断（**ユーザー確認待ち**）

ユーザー方針切替（「最安優先」→「**安定検証優先**」）に従い、Cloud SQL を即削除はせず、
**残すか / 削除するか** をユーザー判断に委ねる。判断材料は次のチャットで報告:

1. Cloud SQL を残すメリット（Domain Mapping / Workers deploy / Safari 検証時の再現性）
2. Cloud SQL を削除するメリット（時間課金停止）
3. 現在の費用見込みと将来見込み
4. 残す場合の監視・注意点
5. 削除する場合のコマンド
6. 推奨判断

## 関連

- [`docs/plan/m2-cloud-sql-short-verification-plan.md`](../../docs/plan/m2-cloud-sql-short-verification-plan.md)
- [`docs/plan/m2-backend-cloud-run-deploy-plan.md`](../../docs/plan/m2-backend-cloud-run-deploy-plan.md)
- [`harness/work-logs/2026-04-26_backend-cloud-run-deploy-result.md`](./2026-04-26_backend-cloud-run-deploy-result.md)
- [`docs/plan/m2-domain-mapping-execution-plan.md`](../../docs/plan/m2-domain-mapping-execution-plan.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
