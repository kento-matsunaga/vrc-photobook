# PR36 UsageLimit / RateLimit 実装結果（2026-04-30、進行中）

> **状態**: PR36 実装中。commit 5（cmd/ops usage list/show + runbook）まで完了。
> Backend deploy（STOP γ）/ Workers redeploy（STOP δ）/ Safari 429 smoke（STOP ε）/ 最終 closeout は **未実施**。

## 0. 本書のスコープ

PR36 全体の進行記録。最終完了時点で本書を closeout モードに更新する。
現時点では **commit 1〜5 を main に push 済み、本番未 deploy** の状態を記録する。

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

## 11. **未実施（本 PR 完了までに必要）**

- **STOP γ**: Backend Cloud Build deploy（image build → Cloud Run revision 更新 → Cloud Run Job image 更新）
- **STOP δ**: Workers redeploy（cf:build + wrangler deploy）
- **STOP ε**: Safari 実機 429 smoke（report / upload / publish の rate_limited 表示確認、Turnstile 状態区別、iPhone Safari レイアウト）
- **最終 closeout**: 本 work-log を「完了」モードに更新、PR closeout チェック（stale-comments / Secret grep / 後続候補整理 / final commit）

## 12. 後続候補

- upload-verification 経路の RemoteIP × photobook 複合 scope（現在は session × photobook のみ）
- SubmitReport 専用の実 DB 副作用なしテスト追加
- session_repository_test.go の photobook seed 修正（既存テスト基盤の技術負債）
- `cmd/ops usage reset` / `cleanup --execute` / Cloud Run Job 化 / Scheduler
- `usage.abuse_detected` Outbox event（Phase 2）
- fail-open flag（`USAGE_LIMIT_FAIL_OPEN_ON_DB_ERROR`）
- 詳細は roadmap §1.3 / 計画書 §17 を参照

## 13. Secret / Privacy 取り扱い（PR 全期間）

- raw IP / source_ip_hash 完全値 / scope_hash 完全値 / token / Cookie / DATABASE_URL 値: いずれも commit / chat / work-log には **未含有**
- 用語 / Secret 名（`REPORT_IP_HASH_SALT_V1` / `TURNSTILE_SECRET_KEY` 等）の参照は許容
- cmd/ops 出力は redact（先頭 8 文字 prefix）形式で固定

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-30 | 初版（PR36 commit 5 時点）。STOP γ / δ / ε / 最終 closeout は未実施として記録 |
