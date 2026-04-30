# PR36 UsageLimit / RateLimit 実装結果（2026-04-30 / 2026-05-01、完了）

> **状態**: PR36 完了。Backend deploy / Workers deploy / Safari 429 実機 smoke / cleanup まで全工程実施済。
> 最終 commit: `044899c feat(ops): add usage limit inspection commands` + closeout commit。

## 0. 本書のスコープ

PR36 全体の進行記録。実装 commit 1〜5、本番 deploy（STOP γ / δ）、Safari 実機 429 smoke（STOP ε）、smoke 由来データ cleanup と target photobook の hidden=true 復元、final closeout までを集約する。

計画書: [`docs/plan/m2-usage-limit-plan.md`](../../docs/plan/m2-usage-limit-plan.md)
runbook: [`docs/runbook/usage-limit.md`](../../docs/runbook/usage-limit.md)

## 1. 計画書承認 / 判断事項 A〜L 確定

2026-04-30 ユーザー承認:

| 項目 | 確定値 |
|---|---|
| A. Salt 方針 | `REPORT_IP_HASH_SALT_V1` 流用 |
| B. 初期対象 endpoint | report.submit / upload_verification.issue / publish.from_draft |
| C. RateLimit ストア | DB 単機 `usage_counters` |
| D. report.submit 閾値 | 同 IP 全体 1 時間 20 件 + 同 IP × photobook 5 分 3 件 |
| E. upload_verification.issue 閾値 | draft_session × photobook で 1 時間 20 件 |
| F. publish.from_draft 閾値 | 同 IP 1 時間 5 件（業務知識 v4 §3.7 確定）|
| G. cmd/ops | list / show を本 PR に入れる |
| H. cleanup | runbook 手動 SQL のみ |
| I. Scheduler | 作らない方針継続 |
| J. Safari 確認 | 429 文言 + Retry-After + Turnstile 区別、iPhone Safari |
| K. Cloud SQL | `vrcpb-api-verify` 継続 |
| L. fail-closed/open | fail-closed（MVP 既定）|

## 2. commit 1: migration 00018 + domain foundation

commit `bd29396 feat(backend): add usage limit domain foundation`

- `backend/migrations/00018_create_usage_counters.sql`（PRIMARY KEY (scope_type, scope_hash, action, window_start) + CHECK 制約 6 種 + INDEX 2 種 + 24h grace expires_at）
- `backend/internal/usagelimit/domain/`:
  - `entity/usage_counter.go` + test
  - `vo/scope_type/`（4 種 enum: source_ip_hash / draft_session_id / manage_session_id / photobook_id）
  - `vo/action/`（3 種 enum: report.submit / upload_verification.issue / publish.from_draft）
  - `vo/scope_hash/`（hex 8〜128 chars / `Redacted()` で先頭 8 文字 prefix 表示）
  - `vo/window/`（fixed window / `StartFor(now)` floor / `RetryAfterSeconds(now)` 切り上げ）

## 3. STOP α: Cloud SQL migration 適用済

Docker 一時 Postgres で goose up/down 検証 → Cloud SQL `vrcpb-api-verify` に v17 → v18 適用済（120.5ms）。既存 12 テーブルの件数完全一致、副作用なし、Secret 漏洩 0 件、cleanup 済み。

> 詳細は当該 STOP α 完了報告（チャット履歴）。

## 4. commit 2: Repository + UseCase

commit `e38d8a8 feat(backend): add usage limit repository and usecases`

- `sqlc.yaml` に PR36 set 追加 → `usage_counter.sql` の 4 query から `sqlcgen` 自動生成
- `infrastructure/repository/rdb/usage_counter_repository.go`:
  - `UpsertAndIncrement`（atomic +1）/ `GetByKey` / `ListByPrefix`（LIKE 'prefix%'）/ `DeleteExpired`
- `internal/usecase/`:
  - `CheckAndConsumeUsage`（fail-closed、ErrRateLimited / ErrUsageRepositoryFailed）
  - `GetUsageForOps` / `ListUsageForOps`（read-only、cmd/ops 用）
- 単体 + 実 DB 統合テスト（concurrency 50 並列 → 最終 count=50 race-free 確認）

## 5. commit 3: endpoint 統合 + HTTP 429 mapping

commit `4ff9c01 feat(backend): enforce usage limits on write endpoints`

- `SubmitReport` / `IssueUploadVerificationSession` / `PublishFromDraft` の前段で `CheckAndConsumeUsage` を呼ぶ
- `mapUsageErr` / `MapUsageErr` / `MapPublishUsageErr` で wrapper 化（retry_after_seconds + cause）
- handler に `writeRateLimited` / `writePublishRateLimited` を追加（HTTP 429 + Retry-After + Cache-Control + X-Robots-Tag + body）
- wireup（report / uploadverification / photobook）に `usagelimitwireup.Check` を伝搬
- cmd/api/main.go で `usagelimitwireup.NewCheck(pool)` を組み立て、3 endpoint に inject

## 6. commit 3.5: 副作用テスト + scope_type 整理

commit `9d36bfe test(backend): cover usage limit endpoint side effects`

- `writeRateLimited` / `writePublishRateLimited` の unit テスト（Retry-After / Cache-Control / X-Robots-Tag / body の漏洩 grep）
- `mapUsageErr` / `MapUsageErr` / `MapPublishUsageErr` の unit テスト（threshold / fail-closed 60 秒 / その他透過）
- report.submit の scope_type を `photobook_id` → **`source_ip_hash`** に統一（複合 hash で表現）
- 計画書 §5.2 / §17.2 / §21 履歴に補足追加

## 7. commit 3.6: 実 DB 副作用なし統合テスト

commit `7205e3d test(backend): verify usage limit denies without side effects`

- `IssueUploadVerificationSession` 429 時 `upload_verification_sessions` 不変（実 DB 統合）
- `PublishFromDraft` 429 時 photobook status / outbox / draft session 不変（実 DB 統合）
- SubmitReport は photobook published seed が複雑なため uv/publish の同パターンで代表保証（mapUsageErr unit + writeRateLimited unit + L4 ガード unit）→ 後続候補として roadmap §1.3 に記録
- 既存 session_repository_test.go の FK 違反問題（テスト基盤の技術負債）を roadmap §1.3 に記録（failure-log 起票不要）

## 8. commit 4: Frontend 429 mapping + UI

commit `d2edd80 feat(frontend): show usage limit errors`

- `lib/report.ts` / `lib/upload.ts` / `lib/publishPhotobook.ts` に `kind: "rate_limited"; retryAfterSeconds: number` 追加
- `extractRetryAfterSeconds`（Retry-After header → body → 既定 60 秒、最低 1 秒）
- `lib/retryAfter.ts` で `formatRetryAfterDisplay`（短文化、iOS Safari レイアウト崩れ回避）
- `ReportForm` / `EditClient`（Upload UI）/ `Publish flow`（CompleteView 周辺）に文言追加
  - Turnstile 失敗（`turnstile_failed`）と完全に分離
- vitest 132 PASS（+19 追加）

## 9. commit 5: cmd/ops + runbook + 途中 work-log

commit `<commit 5 SHA>` `feat(ops): add usage limit inspection commands`

- `backend/cmd/ops/usage.go` 新規（`runUsage` / `cmdUsageList` / `cmdUsageShow` / `printUsageDetail`）
  - `usage list`: scope_type / scope_prefix / action / threshold-only / limit / offset
  - `usage show`: scope_type / scope_prefix（min 4 chars）/ action 必須、複数候補は曖昧として停止
  - **scope_hash 完全値は絶対に出さず、先頭 8 文字 prefix のみ表示**
  - reset / cleanup --execute は MVP 未実装
- `cmd/ops/main.go` の `usage` 文字列 + `runUsage` ディスパッチ追加
- `docs/runbook/usage-limit.md` 新設（12 章、PR36 計画書 §10 / §13 を運用化）
- 本 work-log 作成（途中記録）

## 10. test / build / vet 結果

- `go build ./...`: OK
- `go vet ./...`: OK
- `go test -count=1 ./...`: 全 PASS（DATABASE_URL 未設定下、regression 0）
- frontend `vitest run`: 132 PASS / `tsc --noEmit` OK / `next build` OK / `cf:build` OK
- 実 DB 統合テスト（Docker postgres）: uv + publish 429 副作用なし PASS

## 11. STOP γ: Backend Cloud Build deploy（完了）

- Cloud Build `db4b95c3-0ad8-4a19-9474-10b630e74072` SUCCESS（3M47S）→ image `vrcpb-api:044899c` (digest `sha256:ad2f04fe...`)
- Cloud Run service `vrcpb-api`: 新 revision `vrcpb-api-00022-g4r` 100% traffic（旧 `vrcpb-api-00021-vl9` / `vrcpb-api:540cd1f` を rollback target として保持）
- secretKeyRef 8 個（`DATABASE_URL` / `R2_ACCESS_KEY_ID` / `R2_ACCOUNT_ID` / `R2_BUCKET_NAME` / `R2_ENDPOINT` / `R2_SECRET_ACCESS_KEY` / `REPORT_IP_HASH_SALT_V1` / `TURNSTILE_SECRET_KEY`）。前 revision と完全一致、env 消失なし
- 7 分待機 → routing transient 沈静化 → smoke 全 OK
  - `/health` 200 / `/readyz` 200
  - public route handler: `/api/public/photobooks/<bad-slug>` → HTTP 404 + `{"status":"not_found"}`（chi default plain text 落ちなし）
- Cloud Build log Secret grep: 0 件
- Cloud Run log Secret grep: 起動時の `report endpoint enabled (turnstile + ip_hash_salt configured)` / `turnstile configured; upload-verifications endpoint enabled` のみ。**値は出ていない**
- Cloud Run Job `vrcpb-outbox-worker` の image を `vrcpb-api:540cd1f` → `vrcpb-api:044899c` に bump、args（`--once --max-events 1 --timeout 60s`）/ secretKeyRef 6 個（`DATABASE_URL` / `R2_*`）保持

## 12. STOP δ: Workers redeploy（完了）

- `cf:build`: 9 routes + middleware 34.1 kB、OpenNext bundle 生成成功
- `wrangler deploy`: Total Upload 4604.47 KiB / gzip 947.83 KiB、Worker Startup 35 ms
- 新 Worker version `ac2b884a-7c75-49d3-a21c-5c2a66c462ed`（100% active）。直前 active `ce64f95a-d4ce-405b-821a-f71c22a992db`（PR36-0、rollback 候補）
- bindings: `OGP_BUCKET (vrcpb-images)` / `ASSETS` 維持
- routes smoke: `/` / `/og/default.png` / `/help/manage-url` 200、`/manage/<dummy>` 200 SSR、`/p/<dummy>/report` / `/p/<dummy>` 404（slug 不在の Next.js 標準）、`/ogp/<dummy>?v=1` 302（fallback）
- PR36 文言・helper bundle 含有確認:
  - Report `短時間に通報を送信しすぎました` を `app/(public)/p/[slug]/report/page-*.js` に 1 件
  - Upload `短時間にアップロード` を `app/(draft)/edit/[photobookId]/page-*.js` に 1 件
  - Publish `公開操作の上限` を同 bundle に 1 件
  - retryAfter helper 出力 `分ほど` / `時間ほど` 両方含有
  - Turnstile 失敗 (`turnstile_failed`) は別エラー分岐として bundle に維持、rate_limited と混同なし
- Secret grep（deploy output / response headers / body）: 0 件
- `.open-next` / `.wrangler` git-ignored、tracked 0 件

## 13. STOP ε: Safari 実機 429 smoke（完了）

### 13.1 target 選定（visibility 仕様で 1 度切替）

production の published photobooks 4 件のうち（visibility=public 1 件 / unlisted 1 件 / private 2 件）、SubmitReport は **`visibility='public'` を厳格要求**するため target が限定された。詳細経緯:

1. 当初 `unlisted` 候補（id_prefix `019dca39`）に submit したが「通報対象のフォトブックが見つかりませんでした」（`ErrTargetNotEligibleForReport` → Frontend `not_found`）
2. 原因解析: `backend/internal/report/internal/usecase/submit_report.go:163` が `pb.Visibility().String() != "public"` で弾く実装。業務知識 v4 §2.6 の MVP 既定値（unlisted）と整合しない
3. target を **唯一の `visibility=public` photobook（id_prefix `019dd1bb`、PR33d OGP test、当時 hidden=true）** に切替。Option A1（`cmd/ops photobook unhide` で一時 unhide → smoke → 再 hide）を採用

### 13.2 smoke 実行（Plan F → Plan G）

| Step | 操作 | 結果 |
|---|---|---|
| 1 | DB baseline 取得 | `usage_counters` total 0、`outbox` pending 0、reports 1（PR35b smoke 由来 resolved_action_taken） |
| 2 | `cmd/ops photobook unhide` (`erroneous_action_correction`, ops_smoke) | 新 `moderation_action 019ddf57...`、target `hidden=false`、`/p/<slug>/report` HTTP 200 |
| 3 | iPhone Safari で 1 回正常 submit | thanks view 表示。reports +1 / outbox `report.submitted` pending +1 / usage_counters +2（5min × 3 limit / 1h × 20 limit） |
| 4 | DB diff 確認 | 期待値完全一致。outbox payload は `event_version, has_contact, occurred_at, reason, report_id, target_photobook_id` のみで PII clean（`reporter_contact` / `detail` / `source_ip_hash` 不在） |
| 5 (**Plan F**) | 狭粒度 (5min, count=3/limit=3) のみ threshold 化 | UPDATE 1 row、TX commit |
| 6a (Plan F 失敗) | iPhone Safari で 2 連続 submit | **両方とも thanks view（429 にならず reports +2）**。原因: 5 分窓 rollover（17:05-17:10 → 17:10-17:15、新窓 count=0 から増分） |
| 5' (**Plan G**) | 狭粒度 current 窓 + 広粒度 1h 窓を両方 threshold 化 | 2 行を UPSERT（1 INSERT + 1 UPDATE）、TX commit。rollover 跨ぎ耐性確保 |
| 6 | iPhone Safari で 1 回 submit | **rate_limited UI 表示成功**: 「短時間に通報を送信しすぎました。3 分ほど時間をおいて再度お試しください。」<br>Turnstile 失敗文言なし、レイアウト崩れなし、report_id / token / scope_hash 画面・URL 露出なし |
| 7 | 429 副作用検証 | reports 増分 0 / outbox `report.submitted` pending 増分 0 / 狭粒度 counter 3→4 [OVER_LIMIT]（PR36 §17.2「片方 consume 副作用」の設計通り）/ 広粒度 counter 20 不変（狭粒度で先弾き） |
| 8 | smoke 由来データ DELETE TX | `outbox_events` 3 行 / `reports` 3 行 / `usage_counters` 4 行 削除。PR35b `resolved_action_taken` 行は保持 |
| 9 | `cmd/ops photobook hide` (`policy_violation_other`, ops_smoke) | 新 `moderation_action 019ddf6c...`、target `hidden=true` 復元、API HTTP 410 + `{"status":"gone"}` |
| 10 | `outbox-worker --once --max-events 1` × 2 回 | `photobook.unhidden` / `photobook.hidden` 各 1 件処理 (no-op handler、PR34b 設計通り)、pending=0 |
| 11 | workspace cleanup | cloud-sql-proxy 停止、`/tmp/dsn-prod.txt` / `/tmp/target-pid.txt` / `/tmp/proxy.log` / baseline 削除、port 5433 解放 |

### 13.3 target 019dd1bb 最終状態

| 項目 | 開始時 | 終了時 |
|---|---|---|
| status | published | published |
| visibility | public | public（不変）|
| hidden_by_operator | true | **true**（復元）|
| version | 1 | 1（hide/unhide で OCC bump なし、設計通り）|

### 13.4 失敗と教訓（Plan F → Plan G）

**失敗**: 5 分固定窓 counter のみを threshold 化したため、ユーザー操作との時間ズレで window rollover が起きて smoke が再発火。reports / outbox / counter が想定の 1 / 1 / 2 から 3 / 3 / 4 に膨らんだ（後段 cleanup で全削除）。

**教訓**:

1. **手動 smoke で固定窓 counter を threshold 化する場合、短窓だけに依存しない**。必ず長窓（1 時間）を **同時に** threshold 化して rollover 跨ぎ耐性を確保する
2. **current window の判定はサーバ now 基準**で行う（クライアント / Python `now()` ではなく PostgreSQL `now() AT TIME ZONE 'UTC'`）
3. **失敗 submit 後の追加 submit は禁止**。次の DB 調整完了まで明確に手戻り合図を出す
4. cmd/ops 出力の redact は **sed 後段に依存しない**。出力元（cmd/ops 自身）が redact 形式で出すため、それを尊重し、`[ok]` line など末尾サマリも redact 対象に含める。今回 sed regex が `[ok] photobook_id=...` line をすり抜けて 1 度 raw UUID が chat に出た（work-log には書かない）。詳細は failure-log: `harness/failure-log/2026-04-30_usage-limit-smoke-window-rollover.md`

## 14. 後続候補

- **SubmitReport の visibility 要件再判断**（最重要）:
  - 業務知識 v4 §2.6: `unlisted` は MVP 既定値、`public` は明示選択
  - 現実装: SubmitReport は `visibility=='public'` を厳格要求
  - 衝突: MVP の既定が unlisted なので、通常作成された photobook は **通報不可**
  - 候補: A 仕様維持 / B SubmitReport を `!= 'private'` に緩和して unlisted も対象 / C 業務知識を更新して「通報は public のみ」と明文化
- **report.submit 2 本制限の片方 consume 副作用**（PR36 §17.2 で MVP 許容、後続で CheckOnly + Consume 分離 / reservation 方式 / stored procedure を検討）
- **Upload / Publish 実機 rate_limited smoke**（公開可能な test photobook が増えた段階で実施。upload は draft_session × photobook で 1h × 20、publish は IP × 1h × 5 のため副作用が重く、bundle 文言確認 + Backend 単体・実 DB 統合テスト保証で代替中）
- upload-verification 経路の RemoteIP × photobook 複合 scope（現在は session × photobook のみ）
- SubmitReport 専用の実 DB 副作用なしテスト追加
- session_repository_test.go の photobook seed 修正（既存テスト基盤の技術負債）
- `cmd/ops usage reset` / `cleanup --execute` / Cloud Run Job 化 / Scheduler
- `usage.abuse_detected` Outbox event（Phase 2）
- fail-open flag（`USAGE_LIMIT_FAIL_OPEN_ON_DB_ERROR`）
- Cloud Armor / WAF / Redis 分散 rate-limit（Phase 2 / scale 到達時）
- Legal / privacy policy（PR37）への rate-limit 文言反映
- 詳細は roadmap §1.3 / 計画書 §17 を参照

## 15. Secret / Privacy 取り扱い（PR 全期間）

- raw IP / source_ip_hash 完全値 / scope_hash 完全値 / token / Cookie / DATABASE_URL 値 / reporter_contact 実値 / detail 実値: commit / chat / work-log には **未含有**
- 用語 / Secret 名（`REPORT_IP_HASH_SALT_V1` / `TURNSTILE_SECRET_KEY` 等）の参照は許容
- cmd/ops 出力は redact（先頭 8 文字 prefix）形式で固定
- target photobook の slug / photobook_id は chat に **smoke 中 1 回限り**で提示（work-log では `id_prefix=019dd1bb` / `slug_len=18` のみ記録）
- Step 9 cmd/ops `[ok]` 行で raw photobook_id 完全値が一度だけ chat に出た事実は failure-log に記録（work-log には raw 値を残さない）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-30 | 初版（PR36 commit 5 時点）。STOP γ / δ / ε / 最終 closeout は未実施として記録 |
| 2026-05-01 | STOP γ / δ / ε 完了および final closeout を反映。Plan F→G の教訓 + SubmitReport visibility 後続判断を追記 |
