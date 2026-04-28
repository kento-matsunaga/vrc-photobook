# PR34b Moderation / Ops 実装結果（2026-04-28、進行中）

## 本書のスコープ

PR34b（Moderation / Ops MVP 実装）の進行記録。本 commit（commit 3）時点では
**Cloud SQL migration 適用 + Backend Cloud Build deploy + Cloud Run Job image 更新まで完了**。
Frontend manage UI banner / Workers redeploy / runbook / 実機 hide-unhide smoke は
後続 commit で追記する。

## 概要

- 新正典 PR34 / `docs/plan/m2-moderation-ops-plan.md` PR34b 計画に従い、運営者が
  公開済み photobook を CLI で `hide / unhide / show / list-hidden` できる経路を
  整備
- Cloud SQL に migration `00014_create_moderation_actions.sql` /
  `00015_relax_outbox_event_type_check.sql` を適用
- Backend Cloud Build manual submit で `vrcpb-api:0db0d7c` を deploy、
  `vrcpb-api-00017-hbg` revision に traffic 100%
- Cloud Run Job `vrcpb-outbox-worker` の image を `vrcpb-api:0db0d7c` に更新
  （args / SA / Secret refs / cloudsql-instances / max-retries / parallelism /
  taskCount は維持）
- 初回 Cloud Build が `unable to prepare context: path "backend" not found` で
  失敗。原因は `docs/runbook/backend-deploy.md` §1.2 のサンプルが古く、
  source として backend/ を渡してしまった事象。**repo root submit に修正して再実行
  → SUCCESS**

## 完了済み（commit 1〜3）

### commit 1 `5576656 feat(backend): add moderation domain foundation`

- migration 2 本（`moderation_actions` 新設 + `outbox_events.event_type` CHECK 拡張）
- domain entity / VO 5 種（ActionID / ActionKind / ActionReason / OperatorLabel / ActionDetail）
- table-driven test、全 pass
- sqlc.yaml に moderation set 追加

### commit 2 `0db0d7c feat(backend): add moderation ops usecases and cli`

- `internal/moderation/internal/usecase`: Hide / Unhide / GetForOps / ListHidden の 4 種
  - 同一 TX 4 要素（photobooks UPDATE + moderation_actions INSERT + outbox_events INSERT）
  - version は上げない（編集 OCC を壊さない、計画書 §5.6 / ユーザー判断 #5）
  - status='published' 以外は拒否（計画書 §13 #4）
  - 冪等エラー（ErrAlreadyHidden / ErrAlreadyUnhidden）は cmd/ops が exit 0
- `internal/moderation/infrastructure/repository/rdb`: ModerationActionRepository
  （append-only Insert / ListRecentByPhotobook、sqlc query）
- `internal/moderation/wireup`: cmd/ops 用 facade（Input/Output 型 / sentinel エラー
  re-export）
- `internal/photobook/infrastructure/repository/rdb`: SetHiddenByOperator /
  GetForOps / ListHiddenForOps を追加（sqlc 再生成済）
- `internal/outbox`: payload に PhotobookHiddenPayload / PhotobookUnhiddenPayload
  追加。event_type VO に PhotobookHidden / PhotobookUnhidden 追加（migration
  00015 の CHECK 拡張と一致）。handlers に no-op + log 追加し wireup で Register
- `cmd/ops/main.go`: photobook show / list-hidden / hide / unhide。`--dry-run`
  既定、`--execute` 明示、確認プロンプト、`--yes` で CI skip、`--actor` 必須、
  raw token / Cookie / manage URL / storage_key 完全値は表示しない、
  DATABASE_URL は env 経由のみ
- domain test (commit 1) + outbox event_type test 拡張、全 pass

### commit 3（本書）

- 本 work-log の途中経過記録（commit 4 以降で frontend / runbook / smoke の章を追記）

## STOP α: Cloud SQL migration 適用結果

### ローカル goose 検証

| 観点 | 結果 |
|---|---|
| `goose status`（適用前） | local Postgres は v13 / 00014・00015 Pending |
| `goose up`（00014 / 00015）| 成功（61.48ms / 11.95ms）|
| `moderation_actions` table | column 10 / INDEX 7（PK 含む）/ CHECK 4 すべて期待通り |
| `outbox_events_event_type_check` | 5 種に拡張済（`photobook.published` / `photobook.hidden` / `photobook.unhidden` / `image.became_available` / `image.failed`）|
| `goose down` x2 | 成功（00015 で CHECK 3 種に戻り、00014 で table DROP）|
| `goose up` 再実行 | 成功（v15 復帰）|

### Cloud SQL 適用

| 観点 | 値 |
|---|---|
| 接続 | cloud-sql-proxy v2.13.0 経由（127.0.0.1:5433）|
| 適用前 status | v13 / 00014・00015 Pending |
| `goose up` | 成功（00014: 244.97ms、00015: 43.05ms）|
| 適用後 status | 全 Applied（最新 v15）|
| 既存 outbox_events 行（PR33d 残骸 1 件） | 保持 |
| 既存 photobooks 行 | 12 件保持 |
| `moderation_actions` 既存行 | 0（適用直後の期待通り）|

### Cleanup

- cloud-sql-proxy 停止 / port 5433 解放
- 一時 DSN ファイル / 検証 Go script 削除
- DATABASE_URL 値 / R2 credentials / token / Cookie / storage_key 完全値: chat / log / commit に値出力 0 件

## STOP β: Backend Cloud Build deploy 結果

### 1 回目（FAILURE）

| 観点 | 値 |
|---|---|
| Build ID | `5ce38f50-2937-403a-9c51-ad33824f0694` |
| 結果 | FAILURE（step #0 build）|
| エラー | `unable to prepare context: path "backend" not found` |
| 原因 | runbook §1.2 のサンプルが古く、source として `/home/erenoa6621/dev/vrc_photobook/backend` を渡したが、cloudbuild.yaml は repo root context（`-f backend/Dockerfile` + context `backend`）前提のため不一致 |
| 副作用 | なし（image push 0、Cloud Run / Cloud SQL / Job / Secret 影響なし、既存 revision `vrcpb-api-00016-9ln` のまま）|

PR29 deploy automation の work-log §「修正 2: build context path の不一致」と同じ
事象で、commit `50f940c` で `.gcloudignore` を追加し repo root submit に切り替え
たことで一度解決済み。runbook §1.2 のサンプルだけが古い記述のまま残っていた。

### 2 回目（SUCCESS、修正版コマンド）

修正点: `gcloud builds submit` の source を `/home/erenoa6621/dev/vrc_photobook`（repo root）に変更（`.gcloudignore` で frontend / docs / harness 等を除外して context 最小化）。

| 観点 | 値 |
|---|---|
| Build ID | `a0f05816-a67a-48aa-b396-07da507ade1f` |
| Duration | 3M35S |
| 5 steps（build / push / deploy / traffic-to-latest / smoke） | **すべて SUCCESS** |
| Image tag | `vrcpb-api:0db0d7c` |
| 旧 revision | `vrcpb-api-00016-9ln`（PR33d、image `fe19ab5`、rollback 先）|
| 新 revision | `vrcpb-api-00017-hbg` |
| traffic 100% | `vrcpb-api-00017-hbg`（`latestReadyRevisionName == status.traffic[0].revisionName` 一致確認済）|

### Smoke 検証

| 項目 | 期待 | 結果 |
|---|---|---|
| `/health` | 200 | 200 ✓ |
| `/readyz` | 200 | 200 ✓ |
| `/api/public/photobooks/<unknown-slug>` | 404 | 404 ✓ |
| `/api/public/photobooks/<dummy-uuid>/ogp` | 404 + fallback | 404 ✓ |
| `/api/photobooks/<dummy>/edit-view` (no Cookie) | 401 | 401 ✓ |
| `/api/photobooks/<dummy>/manage-view` (no Cookie) | 401 | 401 ✓ |
| env / secretKeyRef 件数 | 9 件維持 | **9 件**（APP_ENV / ALLOWED_ORIGINS / DATABASE_URL / R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET_NAME / R2_ENDPOINT / TURNSTILE_SECRET_KEY）|
| Cloud Build logs Secret 漏洩 grep | 0 件 | 0 件 |
| Cloud Run logs Secret 漏洩 grep（新 revision 直近 50 行） | 0 件 | 0 件 |

### Cloud Run Job vrcpb-outbox-worker image 更新

`--image=` 単独で更新、他は不変:

| 観点 | 値 |
|---|---|
| 更新前 image | `vrcpb-api:fe19ab5`（PR33d）|
| 更新後 image | `vrcpb-api:0db0d7c` |
| command | `/usr/local/bin/outbox-worker`（維持）|
| args | `--once --max-events 1 --timeout 60s`（維持）|
| serviceAccountName | `271979922385-compute@developer.gserviceaccount.com`（維持）|
| Secret refs | DATABASE_URL + R2_* 全 6 件（維持）|
| `run.googleapis.com/cloudsql-instances` annotation | `<PROJ>:asia-northeast1:vrcpb-api-verify`（維持）|
| maxRetries / parallelism / taskCount | 0 / 1 / 1（維持）|
| Job execution 数 | 2（PR33d 時の `vrcpb-outbox-worker-jdfh9` + `znx4v`、新規実行なし）|
| Cloud Scheduler | 未作成（gcloud scheduler jobs list で 0 件、計画書 §3.2 通り）|

## 後続 commit で実施予定（本 commit 以降）

- **frontend manage UI banner**: `app/(manage)/manage/[photobookId]/page.tsx` に
  hiddenByOperator=true のとき banner 表示（編集ブロックしない、再発行もしない）
- **STOP γ**: Workers redeploy 前停止
- **runbook**: `docs/runbook/ops-moderation.md` 整備
- **STOP δ**: 実機 hide / unhide smoke 前停止
- **closeout 必須対応**:
  - **`harness/failure-log/2026-04-28_runbook-backend-deploy-section-outdated.md` 起票**
    （runbook §1.2 のサンプルが古かった件、PR29 で同事象が起きていた）
  - **`docs/runbook/backend-deploy.md` §1.2 を実運用に合わせて修正**
    （`gcloud builds submit /home/erenoa6621/dev/vrc_photobook \ --config=cloudbuild.yaml \ ...`）
  - work-log の本書を完成させ、stale-comments / Secret grep を実行
- **roadmap 更新**: PR34a → PR34b 完了状態の反映

## Secret 漏洩がないこと（commit 1〜3 範囲）

- DATABASE_URL 完全値 / password: 一時 `/tmp/dsn-prod.txt`（chmod 600）に置いて
  Go script に渡し、検証完了後ファイル削除。chat / log / work-log / git に値を
  書いていない
- R2 credentials 実値: 一切扱っていない（hide / unhide では R2 操作なし）
- raw draft / manage token: 一切扱っていない（cmd/ops は token を使わない設計）
- storage_key 完全値: 一切表示せず（Repository / cmd/ops 出力ホワイトリストで除外）
- Cookie / Set-Cookie / presigned URL / Bearer token: 該当なし
- Cloud Build logs / Cloud Run logs Secret grep: 0 件（forbidden patterns）
- shell history / tmp file: cleanup 済

## 実施しなかったこと（commit 1〜3 時点）

- Frontend manage UI banner（commit 4 で実装）
- Workers redeploy（STOP γ で停止予定）
- runbook `docs/runbook/ops-moderation.md`（commit 5 で作成）
- 実機 hide / unhide smoke（STOP δ で停止予定）
- failure-log 起票 / runbook §1.2 修正（closeout で実施）
- Cloud Scheduler 作成（PR34b 範囲外、計画書 §3.2）
- soft_delete / restore（論理復元） / purge / reissue_manage_url（PR34b 範囲外）
- Web admin UI（v4 §6.19、MVP 範囲外）
- 作成者通知メール（Email Provider 未確定、ADR-0006）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（途中経過）。STOP α / β 完了 + Cloud Run Job image 更新までを記録。frontend / runbook / STOP δ 結果は後続 commit で追記 |
