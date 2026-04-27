# PR30 Outbox 実装結果（2026-04-28、進行中）

## 概要

- 新正典 §3 PR30 / `docs/plan/m2-outbox-plan.md` / `docs/design/cross-cutting/outbox.md` /
  `docs/adr/0006-email-provider-and-manage-url-delivery.md` に従い、メール非依存の
  Outbox 基盤を実装
- 対象 event は **3 種**（`photobook.published` / `image.became_available` / `image.failed`）
- ManageUrlReissued / ManageUrlDelivery* / SendGrid / SES 依存の event は ADR-0006 後続まで
  保留
- **commit `6b5e881`** push 済 / **migration `00012` Cloud SQL 適用済**
- Cloud Build manual submit 経由の deploy は STOP B 承認待ち

## ファイル追加 / 更新（commit `6b5e881`）

| ファイル | 役割 |
|---|---|
| `backend/migrations/00012_create_outbox_events.sql` | Outbox table（cross-cutting/outbox.md §3 通り）+ 索引 5 個 + CHECK 5 個 |
| `backend/sqlc.yaml` | outbox set 追加 |
| `backend/internal/outbox/domain/event.go` | Event entity + NewPendingEvent |
| `backend/internal/outbox/domain/payload.go` | PhotobookPublished / ImageBecameAvailable / ImageFailed payload struct |
| `backend/internal/outbox/domain/vo/event_type/event_type.go` | 3 種 VO |
| `backend/internal/outbox/domain/vo/aggregate_type/aggregate_type.go` | 5 種 VO（拡張余地）|
| `backend/internal/outbox/infrastructure/repository/rdb/outbox_repository.go` | Create のみ（TX-bound）|
| `backend/internal/outbox/infrastructure/repository/rdb/queries/outbox.sql` | CreateOutboxEvent |
| `backend/internal/outbox/infrastructure/repository/rdb/sqlcgen/*.go` | sqlc 生成物 |
| `backend/internal/outbox/domain/event_test.go` | VO + payload 禁止文字列 grep（10 ケース）|
| `backend/internal/outbox/infrastructure/repository/rdb/outbox_repository_test.go` | Repository.Create + CHECK 違反 + rollback（4 ケース）|
| `backend/internal/photobook/internal/usecase/publish_from_draft.go` | WithTx に photobook.published INSERT 追加 |
| `backend/internal/photobook/internal/usecase/publish_outbox_integration_test.go` | publish 成功で 1 行 INSERT 確認 |
| `backend/internal/photobook/internal/usecase/get_public_photobook_test.go` | truncateAll に outbox_events を追加 |
| `backend/internal/imageprocessor/internal/usecase/process_image.go` | MarkAvailable TX に image.became_available / failAndReturn を WithTx ラップ + image.failed 追加 |
| `backend/internal/imageprocessor/internal/usecase/process_image_outbox_test.go` | available / failed 各経路で 1 行 INSERT 確認 |

## DB 設計（migration 内容）

cross-cutting/outbox.md §3 を採用 + worker-friendly 列を追加:

- **Status 値**: `pending` / `processing` / `processed` / `failed` / `dead`（5 値）
- **Worker 列**: `available_at`（retry スケジュール）/ `attempts`（試行回数）/
  `last_error`（200 char CHECK）/ `locked_at` / `locked_by`（worker lock）
- **CHECK 制約**: aggregate_type 5 種許可 / event_type **PR30 では 3 種のみ**（誤投入防止）/
  status 5 値 / attempts ≥ 0 / payload は jsonb_typeof='object' / status と関連列の整合
- **索引 5 個**: pickup（status, available_at）/ aggregate / event_type+status / failed / locked_at
- **Cloud SQL 適用済**（後述）

## Event type / Payload 方針

PR30 で投入する 3 種:

| event_type | payload 主フィールド | 発火点（同 TX）|
|---|---|---|
| `photobook.published` | photobook_id / slug / visibility / type / cover_image_id | PublishFromDraft.Execute の WithTx |
| `image.became_available` | image_id / photobook_id / usage_kind / normalized_format / variant_count | ProcessImage MarkAvailable + AttachVariant×2 TX |
| `image.failed` | image_id / photobook_id / failure_reason | ProcessImage failAndReturn の新 WithTx |

Payload は **明示フィールドのみ**の struct で表現（map / interface{} を避け、Secret 混入事故防止）。

禁止フィールド（plan §5.5、test で grep 確認済）:
raw token / Cookie / hash bytea / presigned URL / storage_key 完全値 / R2 credentials /
DATABASE_URL / Secret / email address。

## 同一 TX 統合結果

| 統合点 | 既存 TX | 追加内容 |
|---|---|---|
| PublishFromDraft | 既存 WithTx あり | `outboxRepo.Create(photobook.published)` を末尾に追加 |
| ProcessImage MarkAvailable | 既存 WithTx あり（MarkAvailable + AttachVariant×2） | 同 TX に `outboxRepo.Create(image.became_available)` 追加 |
| ProcessImage failAndReturn | 既存は単独 UPDATE（TX 無し） | **`WithTx` ラップに書き換え** + `outboxRepo.Create(image.failed)` 追加 |

`outboxrdb.NewOutboxRepository(tx)` に同 TX の `pgx.Tx` を渡すことで、状態更新と outbox INSERT
の atomicity を保証。`failAndReturn` の race condition 経路（ErrConflict）は noOp フラグで
event 出さない判断を維持。

## Test 結果

実 DB（local docker postgres）で全 pass:

| package | 件数 | 観点 |
|---|---|---|
| `internal/outbox/domain` | VO + payload 禁止文字列 grep（10 ケース）| 全 pass |
| `internal/outbox/infrastructure/repository/rdb` | Create / CHECK 違反 / rollback（4 ケース）| 全 pass |
| `internal/photobook/internal/usecase` | 既存 + PublishFromDraft で photobook.published INSERT 確認 | 全 pass |
| `internal/imageprocessor/internal/usecase` | 既存 + available で 1 行 / failed で 1 行 | 全 pass |
| `internal/imageupload/internal/usecase` | 既存 | 全 pass |
| `internal/photobook/interface/http` | 既存 | 全 pass |

`go vet ./...` / `go build ./...` クリーン。

## Cloud SQL migration 適用結果（STOP A）

### 承認

ユーザー承認受領: 2026-04-28。Cloud SQL `vrcpb-api-verify` への適用許可。

### 手順

1. `cloud-sql-proxy --port 15432 <PROJ>:asia-northeast1:vrcpb-api-verify` を background 起動
2. `gcloud secrets versions access` で DATABASE_URL を env injection（chat に値出さず）
3. `goose status` で適用前 version を確認: 11（00001〜00011 適用済）
4. `goose up` で 00012 のみを適用: 225ms で SUCCESS
5. `goose status` で 12 が Applied になったことを確認
6. 直接 SQL で `outbox_events` の存在 / 索引数 / 既存 table の行数を確認
7. cloud-sql-proxy + 一時 DSN env + 一時 Go script + log すべて cleanup

### 検証結果

| 観点 | 結果 |
|---|---|
| migration version | 11 → 12 に更新（00012 のみ Applied） |
| outbox_events table 存在 | ✓（0 行） |
| 索引数 | 6（PK + CREATE INDEX 5 個） |
| 既存 table 行数 | photobooks 11 / images 6 / image_variants 0 / sessions 10 / upload_verification_sessions 5（変動なし） |
| 既存 revision (`vrcpb-api-00011-xfd`) への影響 | なし（旧 image は outbox_events を一切触らないコード） |

## STOP B: Cloud Build manual submit deploy（**完了**）

### 承認

ユーザー承認受領: 2026-04-28。

### 実行内容

- 対象 commit: `019f1d4`（実装 commit `6b5e881` + work-log `019f1d4`）
- Build ID: `1a9c9a35-5594-4852-9ae7-fa231ac5ccee`
- Build duration: 3M36S（build / push / deploy / smoke すべて SUCCESS）
- 新 image: `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb/vrcpb-api:019f1d4`
- 新 revision: `vrcpb-api-00012-6g4`

### Traffic 切替の補足（**新規発見、後続タスクで恒久対処**）

`gcloud run services update --image=` は新 revision を作成するが、PR29 STOP 6
ロールバックドリルで旧 `vrcpb-api-00011-xfd` に **traffic 100% を明示 pin** した
状態のため、**自動で latest に流れない**事象を初検知。承認のもと
`update-traffic --to-revisions=vrcpb-api-00012-6g4=100` で明示切替を実行。

恒久対処は PR30 完了後の独立タスク（後述「後続 deferred」参照）。

### 切替後検証

| 観点 | 結果 |
|---|---|
| traffic 100% on `vrcpb-api-00012-6g4` | ✓ |
| `https://api.vrc-photobook.com/health` | 200 |
| `https://api.vrc-photobook.com/readyz` | 200 |
| `GET /api/photobooks/{id}/edit-view` no Cookie | 401 |
| `POST /api/photobooks/{id}/publish` no Cookie | 401 |
| `GET /api/manage/photobooks/{id}/` no Cookie | 401 |
| `POST /api/photobooks/{id}/images/upload-intent` no Cookie | 401 |
| env / secretKeyRef 9 個維持 | ✓（APP_ENV / ALLOWED_ORIGINS / DATABASE_URL / R2_* 5 個 / TURNSTILE_SECRET_KEY）|
| Cloud Build logs Secret 漏洩 grep | 0 件（204 行スキャン）|
| Cloud Run logs Secret 漏洩 grep | 0 件（新 revision logs スキャン）|
| 新 revision startup probe | TCP probe SUCCESS、AUTOSCALING / DEPLOYMENT_ROLLOUT 起因 INFO のみ |
| WARNING 6 件 | すべて私の認可テスト curl 由来 401 / 404（Cloud Run は 4xx を WARNING に分類する標準動作）|

### Rollback 準備

- rollback 先: `vrcpb-api-00011-xfd`（migration 00012 適用後も後方互換、既存コードは
  `outbox_events` を一切触らない）
- rollback 手順: `gcloud run services update-traffic vrcpb-api --to-revisions=vrcpb-api-00011-xfd=100 --region=asia-northeast1`
- 本 deploy 中は rollback 発火なし

## 実施しなかったこと（PR30 範囲外）

- `cmd/outbox-worker`（PR31）
- worker 用 sqlc query 群（PR31）
- retry / dead letter / Cloud Run Jobs / Scheduler
- メール送信 / SendGrid / SES（**ADR-0006 で MVP 必須から外し済**）
- `ManageUrlReissued` / `ManageUrlDelivery*` event（**ADR-0006 後続**）
- OGP 自動生成（PR33）
- Moderation（PR34）/ Report（PR35）/ UsageLimit（PR36）
- Cloud SQL 削除 / spike 削除 / Public repo 化

## 後続 deferred（PR30 完了後、PR31 着手前に独立タスクで実施）

### A. Cloud Build / Cloud Run の traffic pin 問題（**今回新規発見**）

- 事象: `cloudbuild.yaml` の `gcloud run services update --image=` だけでは traffic
  pin 状態の場合に新 revision に traffic が流れない
- 原因: PR29 STOP 6 ロールバックドリルで `00011-xfd` に traffic 100% を明示 pin した
  まま、`--to-latest` に戻していなかった
- 必要対処:
  - `cloudbuild.yaml` に `--to-latest` 追加 or 明示的な `update-traffic` step 追加を検討
  - `docs/runbook/backend-deploy.md` に「rollback drill 後は traffic pin 状態になる」
    注意を追記
  - 本事象を `harness/failure-log/2026-04-28_cloudbuild-traffic-pin-not-switched.md`
    に記録（`.agents/rules/feedback-loop.md` 準拠）
- 注意: PR30 deploy 完了に集中するため本タスクと混ぜない。独立 PR で扱う

### B. 古いコードコメント整理ターン（**ユーザー指示**）

- PR 番号付きコメント / 後続 PR コメント / 未接続・未実装コメントの全検索
- `publish_from_draft.go` / `router.go` / `session_auth.go` / `main.go` / `slug.go` /
  `health.sql.go` / `manage_handler.go` / `get_manage_photobook.go` / `photobook.go`
  を含めて確認
- 実装済なのにコメントが古い箇所を一覧化
- 修正のみの小 PR（動作変更なし、コメント・ドキュメント整合のみ）として実施
- 完了後に PR31 outbox-worker に進む

## Secret 漏洩なし

- migration 適用に使った DATABASE_URL は env 経由（`$()` 展開のみ、chat / log には出さず）
- cloud-sql-proxy log は cleanup 済
- 一時 Go script は cleanup 済
- 本 work-log にも実値（`postgres://USER:PASS@...`）を含まない

grep で確認: backend/internal/outbox / migration 配下に実値ヒット 0 件
（test docstring の localhost dev DSN / test 内の禁止リスト文字列のみが用語として登場）。

## PR28 visual Safari 残課題

PR30 と独立、引き続き manual 残課題として継続。
PR30 中の manual 実施はなし。

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（PR30 進行中）。実装 + ローカル test 完了、commit `6b5e881` push 済 |
| 2026-04-28 | STOP A 承認・実行記録（Cloud SQL に migration 00012 適用、検証完了） |
| 2026-04-28 | STOP B 承認・実行記録（Cloud Build manual submit + 明示 traffic 切替で `vrcpb-api-00012-6g4` 100% 反映、smoke / 認可 / Secret grep すべて OK）。後続 deferred として `cloudbuild.yaml` traffic pin 問題と古いコメント整理を追記 |
