# PR31 outbox-worker 実装結果（2026-04-28）

## 概要

- 新正典 PR31 / `docs/plan/m2-outbox-plan.md` §6 / §7 / `docs/adr/0006-email-provider-and-manage-url-delivery.md`
  に従い、PR30 で作成した `outbox_events` を消化する **CLI worker** を実装
- handler 3 種は **すべて no-op + structured log**（email provider が ADR-0006 で
  再選定中のため副作用は未実装）
- Cloud Build manual submit で deploy 完了、`vrcpb-api-00013-l9s` に traffic 100%
- Cloud Run Jobs / Scheduler 作成は **本 PR では実施しない**（後続独立 PR で扱う）
- PR30 完了後の独立タスク A（`cloudbuild.yaml` の `traffic-to-latest` step 追加）の
  **初回実地検証も完了**

## ファイル追加 / 更新（commit `c75fe66`）

| ファイル | 役割 |
|---|---|
| `backend/cmd/outbox-worker/main.go` | CLI（--once / --all-pending / --max-events / --timeout / --worker-id / --dry-run / --release-stale-locks） |
| `backend/internal/outbox/internal/usecase/handler.go` | Handler interface + Registry + ErrUnknownEventType |
| `backend/internal/outbox/internal/usecase/handlers/photobook_published.go` | no-op + structured log |
| `backend/internal/outbox/internal/usecase/handlers/image_became_available.go` | no-op + structured log |
| `backend/internal/outbox/internal/usecase/handlers/image_failed.go` | no-op + structured log |
| `backend/internal/outbox/internal/usecase/worker.go` | claim / dispatch / MarkProcessed / MarkFailedRetry / MarkDead / sanitize |
| `backend/internal/outbox/internal/usecase/release_stale_locks.go` | locked_at < threshold で processing → pending 救出 |
| `backend/internal/outbox/internal/usecase/handler_test.go` | Registry 単体（DB 不要、3 ケース） |
| `backend/internal/outbox/internal/usecase/sanitize_test.go` | sanitizeLastError 単体（6 ケース、Secret パターン redact 検証） |
| `backend/internal/outbox/internal/usecase/worker_test.go` | DB 統合（7 ケース: processed / failed_retry / dead / unknown / dry-run / future / 並列 SKIP LOCKED / stale lock release） |
| `backend/internal/outbox/wireup/wireup.go` | cmd から呼ぶ Runner facade |
| `backend/internal/outbox/infrastructure/repository/rdb/queries/outbox.sql` | worker query 6 種追加 |
| `backend/internal/outbox/infrastructure/repository/rdb/sqlcgen/outbox.sql.go` | sqlc 再生成 |
| `backend/internal/outbox/infrastructure/repository/rdb/outbox_repository.go` | worker 系メソッド + PendingEventRow |
| `backend/internal/outbox/domain/event.go` | コメント更新（PR 番号削除） |
| `backend/internal/database/sqlcgen/health.sql.go` | sqlc 再生成（独立タスク B のコメント追従） |
| `backend/Dockerfile` | `outbox-worker` binary を build / COPY |
| `backend/README.md` | 実装履歴の整合更新 |

## CLI 仕様

| flag | 既定 | 動作 |
|---|---|---|
| `--once` | false | 1 件処理して終了（max-events を 1 に強制） |
| `--all-pending` | false | pending / failed が無くなるまで処理（max-events / timeout 上限を尊重） |
| `--max-events` | 50 | 1 起動で処理する最大件数 |
| `--timeout` | 60s | context timeout |
| `--worker-id` | hostname-pid-randomhex | locked_by 列に書く識別子 |
| `--dry-run` | false | claim 結果を log するだけで status を変えない（1 件 SELECT FOR UPDATE → rollback） |
| `--release-stale-locks=<dur>` | 0 | 指定時のみ実行: `processing` で locked_at < now-<dur> の行を pending に戻して終了 |

## status 遷移

```
producer (PR30 INSERT)
        │
        ▼
   pending ──────► processing ──────► processed
        ▲              │
        │              ├──► failed (attempts++、available_at = now+backoff)
        │              │       │
        │              │       └─► (次回 worker pickup で再 processing)
        │              │
        │              └──► dead (attempts >= MaxAttempts、available_at 不変)
        │
        └── (ReleaseStaleLocks: locked_at < threshold の processing を pending に救出)
```

主要な決定:

- **MaxAttempts=5**（既定）。attempts++ で 5 に達したら `dead`
- **backoff = 5min × 2^attempts**、上限 1h（`MaxBackoff`）
- pickup query は `status IN ('pending', 'failed') AND available_at <= now ORDER BY
  available_at ASC, created_at ASC FOR UPDATE SKIP LOCKED`
- claim TX 内で `MarkProcessingByIDs`（複数 ID 同時遷移可、現状は 1 件ずつ）
- handler 後の `MarkProcessed` / `MarkFailedRetry` / `MarkDead` は別 TX（pool 直）
- claim TX commit で row lock を解放、`status='processing'` が論理 lock として機能

## handler 方針（PR31 範囲）

| event_type | 副作用 | log 出力（structured） |
|---|---|---|
| `photobook.published` | **無し** | event_id / event_type / aggregate_type / aggregate_id / attempts / duration_ms / result=ok |
| `image.became_available` | **無し** | 同上 |
| `image.failed` | **無し** | 同上 |

将来（後続 PR で順次拡張）:

- `photobook.published` → OGP 再生成 / Analytics / email provider 確定後の通知
- `image.became_available` → OGP 再生成 / viewer cache refresh
- `image.failed` → cleanup / admin visibility / notification

## ログ方針

**出力する**: event_id / event_type / aggregate_type / aggregate_id / result / duration_ms / attempts / max_attempts / backoff / picked / processed / failed_retry / dead / skipped_unknown / worker_id

**出力しない（厳守）**: payload 全文 / token / Cookie / manage URL / presigned URL /
storage_key 完全値 / R2 credentials / DATABASE_URL / Secret 値

`worker.go` の `sanitizeLastError` で `last_error` 列に書く前に redact:
- 200 char 上限
- `postgres://` / `Bearer ` / `Set-Cookie` / `DATABASE_URL` / `R2_SECRET` /
  `TURNSTILE_SECRET` / `presigned` のいずれかを含む msg は `[REDACTED] error_type` に置換

## Test 結果

ローカル DB（docker-compose postgres + goose 適用済）で実行:

| package | ケース | 結果 |
|---|---|---|
| `internal/outbox/domain` | 既存 | ok（0.002s） |
| `internal/outbox/infrastructure/repository/rdb` | Create / CHECK 違反 / rollback（既存 4 件） | ok（0.093s） |
| `internal/outbox/internal/usecase` | Registry 3 / sanitize 6 / Worker 7（DB あり） + 並列 + stale lock | ok（0.235s） |

`go vet ./...` / `go build ./...` クリーン。

### 既存 test の flaky（PR31 範囲外）

`-p 1`（並列無効）で全 test を実行したところ、`internal/auth/session/infrastructure/repository/rdb` の 5 ケースが
`sessions_photobook_id_fkey` 違反で FAIL。

**git stash で PR31 変更を退避して同 test を実行 → 同様に FAIL を確認**。
よって PR31 起因ではなく、**既存 test の seed 不足 / DB state 依存**による flaky。
他 test（`get_public_photobook_test` の `truncateAll`）が photobooks を CASCADE で
TRUNCATE した状態で session_repository_test が走ると、photobooks に row が無いまま
`session.photobook_id` を INSERT して FK 違反する設計問題。

PR31 の責務外として記録し、後続で session_repository_test 側に photobook seed を
入れるなどの修正を別 PR で実施する想定。

## Cloud Build deploy（独立タスク A の初回実地検証も兼ねる）

### 実行内容

- 対象 commit: `c75fe66`（`feat(backend): add outbox worker`）
- Build ID: `11db1cef-63b8-479f-b907-bcd2f0c64d43`
- Build duration: 2M55S（build / push / deploy / **traffic-to-latest** / smoke すべて SUCCESS）
- 新 image: `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb/vrcpb-api:c75fe66`
- 新 revision: `vrcpb-api-00013-l9s`

### `traffic-to-latest` step の初回実地確認

PR30 完了後の独立タスク A（commit `2c54471`）で `cloudbuild.yaml` に追加した
`update-traffic --to-latest` step が **初めて実地で稼働**。step status SUCCESS。
新 revision に traffic 100% が自動的に切り替わったことを確認:

| 観点 | 値 |
|---|---|
| `latestReadyRevisionName` | `vrcpb-api-00013-l9s` |
| `traffic[0].revisionName` | `vrcpb-api-00013-l9s` |
| `traffic[0].percent` | 100 |

### deploy 後検証

| 観点 | 結果 |
|---|---|
| `https://api.vrc-photobook.com/health` | 200 |
| `https://api.vrc-photobook.com/readyz` | 200 |
| edit-view no Cookie | 401 |
| publish no Cookie | 401 |
| manage no Cookie | 401 |
| upload-intent no Cookie | 401 |
| env / secretKeyRef | 9 個維持（APP_ENV / ALLOWED_ORIGINS / DATABASE_URL / R2_* 5 / TURNSTILE_SECRET_KEY） |
| 新 image | `vrcpb-api:c75fe66` |
| Cloud Build logs Secret 漏洩 | 0 件（220 行） |
| Cloud Run logs Secret 漏洩（新 revision） | 0 件 |
| `outbox-worker` binary 同梱 | ✓（Cloud Build logs step 6 で `go build -o /out/outbox-worker ./cmd/outbox-worker`、step 10 で `COPY --from=build /out/outbox-worker /usr/local/bin/outbox-worker`） |
| Cloud Run service 動作 | 不変（CMD は `/usr/local/bin/api`、outbox-worker は **当面起動しない**） |

### Rollback 準備

- rollback 先: `vrcpb-api-00012-6g4`（PR30 image、outbox table 適用後互換）
- rollback 手順: `gcloud run services update-traffic vrcpb-api --to-revisions=vrcpb-api-00012-6g4=100 --region=asia-northeast1 --project=...`
- 本 deploy 中は rollback 不発火

## Cloud Run Jobs / Scheduler 判断（後続 STOP）

**採用方針: A（本 PR では作成しない）**。

### 理由

- 現状の outbox-worker handler は **no-op + log 中心**
- Cloud Run Jobs を稼働させると pending event を `processed` に進めてしまう
- 将来 OGP / 通知 / cleanup などの副作用を入れる前に既存 event を消費すると、後で
  「処理済み扱いだが実副作用は未実行」という不整合状態になる
- まずは binary 同梱 + deploy + traffic-to-latest 検証までで PR31 を閉じる
- pending event は consume されず積み上がる（PR30 deploy 以降と同じ挙動を継続）

### 後続独立 PR で扱う

| 項目 | 判断タイミング |
|---|---|
| Cloud Run Jobs 作成 | worker の副作用方針が固まった後続 PR（PR32 email provider 再選定 → handler 副作用を入れるタイミング） |
| Cloud Scheduler 作成 | Cloud Run Jobs 作成後、運用 cadence（例: 5 分おき）を確定する PR |
| 副作用 handler 実装 | PR32 以降、provider / OGP / cleanup の各実装 PR |

## PR closeout チェックリスト（pr-closeout.md §6）

- [x] **コメント整合チェック実施**: `bash scripts/check-stale-comments.sh` を
  PR31 範囲（`backend/cmd/outbox-worker/`、`backend/internal/outbox/`）に絞って
  分類実施
- [x] **古いコメントを修正した**:
  - A 修正: `backend/README.md` の「outbox-worker（PR31 で実装予定）」を実装済表記に
  - B 状態ベース表現に変更:
    - `backend/cmd/outbox-worker/main.go` 起動形態説明
    - `backend/internal/outbox/internal/usecase/handler.go` 「PR31 で扱う」→「現状扱う」
    - `backend/internal/outbox/internal/usecase/handlers/photobook_published.go` パッケージ説明
    - `backend/internal/outbox/internal/usecase/worker.go` handleFailure コメント
    - `backend/internal/outbox/internal/usecase/release_stale_locks.go` 起動形態
    - `backend/internal/outbox/wireup/wireup.go` NewRunner コメント
    - `backend/internal/outbox/domain/event.go` Event 状態遷移コメント / AvailableAt フィールドコメント
- [x] **残した TODO とその理由**:
  - C 過去経緯として残す: migration / queries の `-- PR30:` 系履歴コメント、
    `event_type.go` / `aggregate_type.go` の「PR30 で実体投入する」記述
  - B 状態ベース TODO: `outbox.sql` の `-- last_error は worker が sanitize` 等
  - D 生成元修正: `health.sql.go` の sqlcgen は本 PR の sqlc generate で自動追従済（独立タスク B のコメント追従）
- [x] **先送り事項がロードマップに記録済み**:
  - Cloud Run Jobs / Scheduler 作成 → 本 work-log + 新正典ロードマップ §3 PR31 を更新（後述）
  - 副作用 handler（OGP / 通知 / cleanup） → 各後続 PR に分散
  - 既存 session_repository_test の flaky → 本 work-log に記録、後続 PR で seed 追加
- [x] **generated file の未反映コメント**: 無し（sqlc 再生成済）
- [x] **Secret 漏洩 grep**: PR31 touched files に対して実行 → マッチは禁止リスト記述 / sanitize 対象リストのみ、実値 0 件

## Secret 漏洩なし

- DATABASE_URL は env 経由（Secret Manager → Cloud Run secretKeyRef）。本 work-log /
  commit / chat に実値を含まない
- payload 禁止リストを `event.go` / `payload.go` / `handler.go` / handlers 群 /
  worker.go / queries に明示
- worker.go の `sanitizeLastError` で last_error 列の実 Secret 値混入を redact
- Cloud Build logs / Cloud Run logs grep 実値ヒット 0 件
- 一時 log file は cleanup 済（`/tmp/cb-pr31.txt` / `/tmp/cr-pr31.txt`）

## 実施しなかったこと（PR31 範囲外）

- **Cloud Run Jobs / Scheduler 作成**（採用方針 A、後続 PR で実施判断）
- **副作用 handler**（メール送信 / OGP 再生成 / 通知 / cleanup）
- SendGrid / SES / Email Provider 実装（ADR-0006 で再選定中）
- ManageUrlReissued / ManageUrlDelivery* event の追加
- Moderation / Report / UsageLimit handler
- dead letter queue の本格運用（dead 化のみ実装、UI / 再投入機構は無し）
- Dashboard UI / 観測 alert 設計
- Public repo 化 / Cloud SQL 削除 / spike 削除
- 既存 session_repository_test の seed 修正（PR31 範囲外、別 PR）

## PR28 visual Safari 残課題

PR31 と独立。引き続き manual 残課題として継続。PR31 中の manual 実施はなし。

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（PR31）。worker 実装 + Dockerfile 同梱 + Cloud Build deploy + traffic-to-latest 初回実地検証 + Cloud Run Jobs 作成は採用方針 A で本 PR では実施せず |
