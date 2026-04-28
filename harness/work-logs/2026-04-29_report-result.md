# PR35b Report 集約 / 通報受付 実装結果（2026-04-29、進行中）

## 本書のスコープ

PR35b（Report MVP 実装）の進行記録。本 commit（commit 3）時点では
**Backend 実装 + Cloud SQL migration 適用 + REPORT_IP_HASH_SALT_V1 Secret 注入 +
Backend Cloud Build deploy + Cloud Run Job image 更新まで完了**。Frontend
`/p/[slug]/report` 実装 / Workers redeploy / 本番 Report 送信 smoke / Safari 実機
確認 / runbook 追記 / closeout は **次セッションの commit 4 以降**で実施する。

PR35b 全体は 1 PR のまま維持し、commit 履歴で 1〜5 を順次積む構造。

## 概要

- 新正典 PR35 / `docs/plan/m2-report-plan.md` PR35b 計画に従い、公開 Viewer から
  受付る通報基盤を Backend MVP 範囲で実装
- Turnstile を `internal/turnstile/` 共通 package として抽出（既存
  upload-verification 利用元の 6 import 先を切替、旧 path を削除）
- migration 00016 / 00017 を Cloud SQL `vrcpb-api-verify` に適用（v15 → v17）
- 新 Secret `REPORT_IP_HASH_SALT_V1` を Secret Manager に登録、runtime SA に
  `secretAccessor` 付与、Cloud Run service `vrcpb-api` に注入（secretKeyRef
  9 件 → 10 件、Job には注入せず）
- Backend image を `vrcpb-api:f4427b1` で deploy（new revision `vrcpb-api-00019-jkj`、
  traffic 100%）+ Cloud Run Job `vrcpb-outbox-worker` image を同 SHA に更新
- 公開 endpoint `POST /api/public/photobooks/{slug}/reports` が稼働中、Turnstile
  必須（dummy token で 403 / token 欠如で 400 動作確認済）
- `cmd/ops report list / show` + `cmd/ops photobook hide --source-report-id` 拡張
  完了
- Outbox `report.submitted` handler 配線済（worker no-op + log、minor_safety_concern
  Warn）

## ユーザー判断確定（PR35a §16）

| # | 判断項目 | 確定 |
|---|---|---|
| 1 | Turnstile 必須 | **採用**（403 / 400 動作確認済）|
| 2 | 通報対象 published+visibility=public+hidden=false のみ | **採用**（UseCase で 4 経路拒否）|
| 3 | `REPORT_IP_HASH_SALT_V1` 新規 Secret | **採用**（version 1 enabled、runtime SA 付与）|
| 4 | IP 取得 Cf-Connecting-Ip 優先 + XFF fallback | **採用**（extractRemoteIP 実装、test 4 ケース pass）|
| 5 | Frontend `/p/[slug]/report` 別ページ | **次 commit 4 で実装**（commit 3 では未着手）|
| 6 | runbook は ops-moderation.md に追記 | **次 commit 5 で対応** |
| 7 | thanks view で report_id 表示しない | **次 commit 4 で実装** |
| 8 | Turnstile siteverify 共通 package | **採用**（`internal/turnstile/` 抽出 + uploadverification 切替）|
| 9 | source_ip_hash 表示は先頭 4 byte hex のみ | **採用**（`cmd/ops report show` 実装、handler / log で完全値非表示）|
| 10 | Outbox handler no-op + minor_safety_concern Warn | **採用**（test 2 ケース pass）|
| 11 | Cloud SQL は vrcpb-api-verify 継続 | **採用** |

## 完了済み（commit 1〜3）

### commit 1 `2a33284 feat(backend): add report domain foundation`

- `internal/turnstile/` 共通 package 抽出（6 import 切替、旧 path 削除）
- migration 00016 reports（13 カラム + 6 INDEX + CHECK 9）
- migration 00017 outbox event_type CHECK 拡張（`report.submitted` 追加）
- Report entity / VO 6 種（ReportID / ReportReason / ReportDetail / ReporterContact /
  ReportStatus / TargetSnapshot）
- domain test 全 pass
- sqlc.yaml に report set 追加

### commit 2 `f4427b1 feat(backend): add report submission and ops commands`

- queries/report.sql + sqlcgen
- `ReportRepository`（Create / GetByID / List / MarkResolvedActionTaken）
- `SubmitReport` UseCase: 同 TX 2 要素（reports INSERT + outbox INSERT）/
  Turnstile siteverify / 公開対象判定 / source_ip_hash 算出
- `GetReportForOps` / `ListReportsForOps`（cmd/ops 参照系）
- 公開 endpoint `POST /api/public/photobooks/{slug}/reports`
- `cmd/ops report list / show` + `cmd/ops photobook hide --source-report-id`
- Moderation `HideInput.SourceReportID` 拡張、同 TX 5 要素で reports.status
  自動遷移
- Outbox `ReportSubmittedPayload` + event_type + handler（no-op + log、
  minor_safety_concern Warn 以上）
- Cloud Run service env に `REPORT_IP_HASH_SALT_V1` を読み込む config 拡張
- test: HashSourceIP / extractRemoteIP / report_submitted handler / outbox
  event_type Parse 拡張、全 pass

### commit 3（本書）

- 本 work-log の途中経過記録（commit 4 以降で frontend / smoke / runbook /
  closeout を追記）

## STOP α: Cloud SQL migration 適用結果

### ローカル goose 検証

| 観点 | 結果 |
|---|---|
| `goose status`（適用前） | local Postgres v15 / 00016・00017 Pending |
| `goose up` | 成功（00016: 46.51ms、00017: 5.66ms）|
| `reports` table | column 15 / INDEX 7（PK 含む、minor_safety_concern 部分 INDEX、source_ip_hash 部分 INDEX）/ CHECK 9 |
| `outbox_events_event_type_check` | 6 種に拡張済（`report.submitted` 追加確認） |
| `goose down` x2 | 成功（00017 で CHECK 5 種に戻り、00016 で table DROP）|
| `goose up` 再実行 | 成功（v17 復帰）|

### Cloud SQL 適用

| 観点 | 値 |
|---|---|
| 接続 | cloud-sql-proxy v2 経由（127.0.0.1:5433）|
| 適用前 status | v15 / 00016・00017 Pending |
| `goose up` | 成功（00016: 138.74ms、00017: 31.71ms、v17 到達）|
| 適用後 status | 全 Applied、最新 v17 |
| 既存 photobooks 行 | 12 件保持 |
| 既存 outbox_events 行 | 3 件保持（PR34b smoke の photobook.published / hidden / unhidden）|
| 既存 moderation_actions 行 | 2 件保持（PR34b STOP δ smoke の hide+unhide）|
| `reports` 既存行 | 0（適用直後の期待通り）|

### Cleanup

- cloud-sql-proxy 停止 / port 5433 解放
- 一時 DSN ファイル / 一時 Go script 削除
- DATABASE_URL 値 / R2 credentials / token / Cookie / source_ip_hash / reporter_contact 値:
  chat / log / commit に未含有

## STOP β: REPORT_IP_HASH_SALT_V1 Secret 作成・注入結果

### Secret 状態

| 観点 | 値 |
|---|---|
| Secret 名 | `REPORT_IP_HASH_SALT_V1` |
| Secret resource | `projects/271979922385/secrets/REPORT_IP_HASH_SALT_V1` |
| 作成日時 | `2026-04-28T21:21:37Z` |
| version 1 | enabled（`2026-04-28T21:21:40Z`）|
| 値生成 | ユーザー対話シェルで `openssl rand -hex 32` を stdin pipe で `gcloud secrets versions add --data-file=-` に渡す形で登録（値は chat / log / 履歴 / commit に未含有、Claude Code が値を持たない）|
| replication | automatic |

### IAM 付与

| 観点 | 状態 |
|---|---|
| runtime SA `271979922385-compute@developer.gserviceaccount.com` | `roles/secretmanager.secretAccessor` 付与済 |
| Cloud Build SA `vrcpb-cloud-build@...` | **付与せず**（build 時に Secret 値を使う必要なし、最小権限維持）|

### Cloud Run service `vrcpb-api` 注入結果

| 観点 | 値 |
|---|---|
| 注入前 revision | `vrcpb-api-00017-hbg`（PR34b、image `vrcpb-api:0db0d7c`、secretKeyRef 9 件）|
| 注入直後 revision | `vrcpb-api-00018-65p`（同 image `vrcpb-api:0db0d7c` + REPORT_IP_HASH_SALT_V1 注入の env-only 変動）|
| secretKeyRef 件数 | **10 件**（既存 9 + 新 `REPORT_IP_HASH_SALT_V1`）|
| 既存 9 件不変 | APP_ENV / ALLOWED_ORIGINS / DATABASE_URL / R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET_NAME / R2_ENDPOINT / TURNSTILE_SECRET_KEY |
| 新規 1 件 | REPORT_IP_HASH_SALT_V1 |
| `/health` / `/readyz` | 200 / 200 |
| Cloud Run logs Secret 漏洩 grep | 0 件（実値非含有）|

### Cloud Run Job `vrcpb-outbox-worker`（不変）

- 案 A 採用通り **`REPORT_IP_HASH_SALT_V1` を Job には注入せず**
- 既存 secretKeyRef 7 件（APP_ENV + DATABASE_URL + R2_* 5 件）維持
- 将来 Reconcile / batch で salt が必要になった時点で追加注入する方針

## STOP γ: Backend Cloud Build deploy 結果

### deploy 実行（修正版コマンド、runbook §1.2 準拠）

| 観点 | 値 |
|---|---|
| Build ID | `2e68aff3-3bff-489c-a757-3a7d0c039012` |
| Duration | 3M42S |
| 5 steps（build / push / deploy / traffic-to-latest / smoke）| **すべて SUCCESS** |
| Image tag | `vrcpb-api:f4427b1` |
| 注入前 revision（rollback 先）| `vrcpb-api-00018-65p`（PR34b image + Salt 注入後の env-only revision）|
| 新 revision | `vrcpb-api-00019-jkj`（image `vrcpb-api:f4427b1`、PR35b 完全反映）|
| traffic 100% | `vrcpb-api-00019-jkj`（`latestReadyRevisionName == status.traffic[0].revisionName` 一致確認）|

### Smoke 検証

| 項目 | 期待 | 結果 |
|---|---|---|
| `/health` | 200 | **200** ✓ |
| `/readyz` | 200 | **200** ✓ |
| `/api/public/photobooks/<unknown-slug>` | 404 | **404** ✓ |
| `/api/public/photobooks/<dummy-uuid>/ogp` | 404 + fallback | **404** ✓ |
| **POST `/api/public/photobooks/<bad-slug>/reports` token なし** | 400 invalid_payload | **400 / `{"status":"invalid_payload"}`** ✓ |
| **POST 同 endpoint dummy token** | 403 turnstile_failed | **403 / `{"status":"turnstile_failed"}`** ✓ |
| `/api/photobooks/<dummy>/edit-view` no Cookie | 401 | **401** ✓ |
| `/api/photobooks/<dummy>/manage-view` no Cookie | 401 | **401** ✓ |
| `/api/auth/draft-session-exchange` POST 空 body | 400 | **400** ✓（既存挙動、認可前 body validation）|
| env / secretKeyRef 件数 | 10 件維持 | **10 件**（APP_ENV / ALLOWED_ORIGINS / DATABASE_URL / R2_* 5 / TURNSTILE_SECRET_KEY / REPORT_IP_HASH_SALT_V1）|
| Cloud Build logs Secret 漏洩 grep | 0 件 | **0 件** |
| Cloud Run logs Secret 漏洩 grep（新 revision 直近 50 行）| 0 件 | **0 件** |

### Cloud Run Job vrcpb-outbox-worker image 更新

| 観点 | 値 |
|---|---|
| 更新前 image | `vrcpb-api:0db0d7c`（PR34b）|
| 更新後 image | `vrcpb-api:f4427b1` |
| command | `/usr/local/bin/outbox-worker`（維持）|
| args | `--once --max-events 1 --timeout 60s`（維持）|
| serviceAccountName | `271979922385-compute@developer.gserviceaccount.com`（維持）|
| Secret refs | APP_ENV + DATABASE_URL + R2_* 5 件 = **7 件**（維持、`REPORT_IP_HASH_SALT_V1` 注入なし、案 A 通り）|
| `cloudsql-instances` annotation | 維持 |
| maxRetries / parallelism / taskCount | 0 / 1 / 1（維持）|
| Job execution 数 | 4（PR33d + PR34b 累積、新規実行なし）|
| Cloud Scheduler | 未作成（gcloud scheduler jobs list で 0 件）|

## 現在の本番状態（commit 3 時点）

| レイヤ | 状態 |
|---|---|
| Cloud SQL `vrcpb-api-verify` | v17（reports + outbox CHECK 拡張、適用済）|
| Cloud Run service `vrcpb-api` | revision `vrcpb-api-00019-jkj`、image `vrcpb-api:f4427b1`、traffic 100%、secretKeyRef 10 件 |
| Cloud Run Job `vrcpb-outbox-worker` | image `vrcpb-api:f4427b1`、secretKeyRef 7 件（Salt 未注入）|
| Cloudflare Workers | version `e97148fe-...`（PR34b、PR35b 未反映）|
| 公開 endpoint `POST /api/public/photobooks/{slug}/reports` | 稼働中（Turnstile 必須）|
| `cmd/ops report list / show` | 利用可 |
| `cmd/ops photobook hide --source-report-id` | 利用可 |
| Outbox `report.submitted` handler | 配線済（no-op + log）|
| **Frontend `/p/[slug]/report`** | **未実装**（次セッション commit 4）|
| **本番 `report.submitted` event** | **0 件**（正規 Frontend 経由送信は STOP ε で実施予定）|
| **Safari 実機確認** | **未実施**（STOP ε で commit 4 実装後に実施）|

## 後続持ち越し（次セッション）

| 項目 | 持ち越し先 |
|---|---|
| Frontend `/p/[slug]/report` 別ページ | 次セッション commit 4 |
| Viewer から「通報」リンク | 同上 |
| Turnstile widget（既存 `TurnstileWidget` 流用検討、action="report-submit"）| 同上 |
| thanks view（report_id 非表示）| 同上 |
| frontend tests（vitest renderToStaticMarkup）| 同上 |
| Workers redeploy（cf:build + wrangler deploy）| 次セッション STOP δ |
| 本番 Report 送信 smoke（test photobook unhide → POST → DB 検証 → cmd/ops show / list → hide --source-report-id → reports.status='resolved_action_taken' 自動遷移確認 → 最終 hidden=true 復元）| 次セッション STOP ε |
| macOS Safari / iPhone Safari 実機確認 | 次セッション STOP ε で兼ねる |
| `docs/runbook/ops-moderation.md` § Report 連携 追記 | 次セッション commit 5 |
| failure-log 起票要否判断 | 次セッション closeout |
| stale-comments + Secret grep 最終 | 次セッション closeout |
| roadmap PR35 章 完了反映 | 次セッション closeout |

### 次セッション開始指示（参考）

```
PR35b commit 3 (work-log) push 済み。次は commit 4 から再開する。
- main HEAD: <commit 3 の SHA>
- Cloud SQL v17 / Cloud Run service vrcpb-api-00019-jkj / Job vrcpb-api:f4427b1
- 公開 endpoint POST /api/public/photobooks/{slug}/reports 稼働中、Turnstile 必須
- Frontend /p/[slug]/report は未実装
- 計画書: docs/plan/m2-report-plan.md
- ユーザー判断 11 件は PR35a で確定済み
- 次は: Frontend 別ページ + Viewer 通報リンク + form + Turnstile widget +
  thanks view + tests → STOP δ → STOP ε → commit 5 closeout
```

## Secret 漏洩がないこと（commit 1〜3 範囲）

- DATABASE_URL 完全値: 一時 `/tmp/dsn-prod.txt`（chmod 600）に置いて Go script に渡し、
  検証完了後ファイル削除。chat / log / work-log / commit に値出力なし
- `REPORT_IP_HASH_SALT_V1` 値: ユーザー対話シェルで `openssl rand -hex 32` を stdin pipe
  で Secret Manager に登録。Claude Code は値を一切受け取らず、chat / log / work-log /
  commit に未含有。Cloud Run logs にも 0 件
- R2 credentials 実値: 一切扱っていない
- raw token / Cookie / manage URL / storage_key 完全値: 一切扱わず（cmd/ops 設計時除外）
- reporter_contact / source_ip_hash 実値: 本番 0 件（送信なし、Frontend 未配線）
- Cloud Build / Cloud Run logs / Cloud Run Job logs Secret 漏洩 grep: 0 件
- shell history / tmp file: cleanup 済

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-29 | 初版（途中経過、commit 3）。STOP α / β / γ 完了 + Cloud Run service / Job image 更新までを記録。Frontend / Workers / smoke / Safari / closeout は次セッション commit 4 以降 |
