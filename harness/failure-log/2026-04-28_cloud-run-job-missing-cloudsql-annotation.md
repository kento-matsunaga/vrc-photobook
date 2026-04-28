# Cloud Run Job 作成時に `--set-cloudsql-instances` を指定し忘れて Job 実行が DB 接続エラーで即落ち（2026-04-28）

## 発生状況

- PR33d STOP κ で Cloud Run Job `vrcpb-outbox-worker` を初回実行
  - コマンド: `gcloud run jobs execute vrcpb-outbox-worker --region=asia-northeast1 --project=... --wait`
  - 実行 ID: `vrcpb-outbox-worker-jdfh9`
- Job spec は STOP θ で `gcloud run jobs create` により作成済（image / args / Secret refs / SA / max-retries / parallelism は意図通り）
- 実行直前検証では outbox event 1件のみ pending（STOP ι event）/ 古い pending=0 / spec の意図と DB の状態は OK

## 失敗内容

```text
ERROR: (gcloud.run.jobs.execute) The execution failed.
```

Cloud Run Job logs:

```text
Container called exit(1).
{
  "msg": "run failed",
  "error": "begin tx: failed to connect to `user=vrcpb_app database=vrcpb`: /cloudsql/<project>:asia-northeast1:vrcpb-api-verify/.s.PGSQL.5432 (...): dial error: dial unix /cloudsql/<project>:asia-northeast1:vrcpb-api-verify/.s.PGSQL.5432: connect: no such file or directory"
}
```

DB 接続自体ができず、ListPending を 1 件も叩かないまま Job が exit(1)。
`picked=0 / processed=0 / failed_retry=0 / dead=0`、outbox event は **`status=pending` /
`attempts=0` のまま不変**で副作用ゼロ。

## 根本原因

**Cloud Run Job spec に `run.googleapis.com/cloudsql-instances` annotation が欠落**。

- 同じ Cloud SQL インスタンスを使う Cloud Run **service** `vrcpb-api` には同 annotation が
  設定済 (`run.googleapis.com/cloudsql-instances: <project>:asia-northeast1:vrcpb-api-verify`)
- しかし Cloud Run **Jobs** は別リソースタイプであり、`gcloud run jobs create` 時に
  `--set-cloudsql-instances` を指定しない限り annotation が付かない
- `gcloud run deploy`（service 用）と `gcloud run jobs create`（Job 用）でデフォルトの
  Cloud SQL 接続 wiring が異なる、という Cloud Run の前提を見落としていた
- Cloud SQL Auth Proxy の Unix socket (`/cloudsql/<INSTANCE>/.s.PGSQL.5432`) は当該 annotation
  が付いた pod でしか mount されない

## 影響範囲

- 直接影響: STOP κ 1 回目の Job 実行が失敗（Job 実行時間 ~17 秒、DB 状態は不変）
- 副作用範囲: なし（DB rows は一切作成 / 更新されず、R2 に PUT も発生せず、
  Secret 漏洩なし、event は pending のまま）
- 復旧: `gcloud run jobs update --set-cloudsql-instances=...` で annotation 追加 + 再実行
  （`vrcpb-outbox-worker-znx4v` 成功、`processed=1`）
- 学び: 本番副作用 handler の **初回実行 STOP** で Job の DB 接続テストが事前に
  入っていなかったため、最初の execute まで気付かなかった

## 対策

### 1. ルール化（必須）

- `.agents/rules/` 直下には新規ルール追加せず、Job 作成手順に必須項目として刻む
  （副作用 handler の 1st-class テンプレートになる）

### 2. ロードマップ・計画書更新

- `docs/plan/vrc-photobook-final-roadmap.md` PR33d 章 / STOP θ 手順に
  「Job 作成時 `--set-cloudsql-instances` 必須」「同 region 内 Cloud SQL を使う Job は
  service 側 annotation と一致させる」を明記
- `docs/runbook/backend-deploy.md` または PR で参照される計画書に Job 作成テンプレート
  を残す

### 3. 自動検出（次の Job 追加時）

Job 作成 / 更新の際は以下を **`gcloud run jobs describe`** で必ず確認:

| 確認項目 | 期待値 |
|---|---|
| `metadata.annotations."run.googleapis.com/cloudsql-instances"` | 当該インスタンスの connection name 文字列 |
| `spec.template.metadata.annotations` | service 側 annotation と齟齬なし（cloudsql-instances / vpc-access） |
| 環境変数 secretKeyRef | DATABASE_URL / R2_* が service 側と完全一致 |

### 4. 再発防止コマンドテンプレート（Job 作成時の標準形）

```bash
gcloud run jobs create <JOB_NAME> \
  --region=<REGION> \
  --project=<PROJECT> \
  --image=<IMAGE> \
  --command=<COMMAND> \
  --args=<ARGS> \
  --service-account=<SA> \
  --set-cloudsql-instances=<INSTANCE_CONNECTION_NAME> \  # ← 必須、忘れると即 Job 起動失敗
  --set-env-vars=APP_ENV=production \
  --set-secrets=DATABASE_URL=DATABASE_URL:latest \
  --set-secrets=R2_*=R2_*:latest \
  --max-retries=0 \
  --parallelism=1 \
  --task-count=1
```

## 関連

- [`.agents/rules/feedback-loop.md`](../../.agents/rules/feedback-loop.md) — 失敗 → ルール化
- [`.agents/rules/wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md) — gcloud / 外部 CLI の cwd / 引数管理
- [`docs/plan/vrc-photobook-final-roadmap.md`](../../docs/plan/vrc-photobook-final-roadmap.md) PR33 章
- [`harness/work-logs/2026-04-28_ogp-outbox-handler-result.md`](../work-logs/2026-04-28_ogp-outbox-handler-result.md) — STOP κ 実施記録（初回失敗 → patch → 再実行 → 全検証成功 を含む）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版。PR33d STOP κ 1 回目失敗を起点に、Job 作成時の必須 annotation を明文化 |
