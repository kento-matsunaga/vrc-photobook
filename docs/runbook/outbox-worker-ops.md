# outbox-worker / OGP sync fallback 運用 runbook

> 設計参照:
> - [`docs/plan/m2-ogp-sync-publish-plan.md`](../plan/m2-ogp-sync-publish-plan.md)
> - [`docs/adr/0007-ogp-sync-publish-fallback.md`](../adr/0007-ogp-sync-publish-fallback.md)
> - [`docs/plan/m2-image-processor-job-automation-plan.md`](../plan/m2-image-processor-job-automation-plan.md) §5（同パターンの Scheduler 構成元ネタ）
>
> Cloud Run Job `vrcpb-outbox-worker` と Cloud Scheduler `vrcpb-outbox-worker-tick`
> の **日常運用 / 起動確認 / IAM 確認 / rollback** をまとめる。
>
> 本書は**運用手順書**であり、設計判断は ADR-0007 / m2-ogp-sync-publish-plan で行う。

---

## 0. 前提

- GCP project: `project-1c310480-335c-4365-8a8`
- Region: `asia-northeast1`
- Cloud Run Job 名: `vrcpb-outbox-worker`
- Cloud Scheduler 名: `vrcpb-outbox-worker-tick`
- Scheduler 発火 SA: `271979922385-compute@developer.gserviceaccount.com`（Compute Engine default SA）
- Scheduler schedule: `* * * * *`（1 分間隔、Asia/Tokyo）
- 認証方式: **OAuth**（Cloud Run Admin API は OIDC ではなく OAuth、`image-processor-tick` と同パターン）
- Job image tag: Backend Service `vrcpb-api` と同 SHA に同期する運用（`backend-deploy.md` §6 と同方針）

---

## 1. Scheduler / Job 構成（運用上の不変事項）

| 項目 | 値 | 備考 |
|---|---|---|
| Scheduler name | `vrcpb-outbox-worker-tick` | suffix `-tick` で Scheduler / Job の対応関係を明示 |
| schedule | `* * * * *` | ADR-0007 §3 (3) 採用、1 分間隔 |
| time-zone | `Asia/Tokyo` | log タイムスタンプを実運用と合わせる |
| target | Cloud Run Jobs run API、URI `https://asia-northeast1-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/<PROJ>/jobs/vrcpb-outbox-worker:run` | 同 region |
| HTTP method | POST | Cloud Run Jobs run API |
| body | `{}`（空） | args は Job 側に固定 |
| auth | OAuth、`oauthToken.scope=https://www.googleapis.com/auth/cloud-platform` | image-processor-tick と同方式（**OIDC ではない**） |
| oauthToken.serviceAccountEmail | `271979922385-compute@developer.gserviceaccount.com` | Job 側 IAM で `roles/run.invoker` 付与必須（§4） |
| attempt-deadline | `120s` | Cloud Run Jobs run API call 自体の deadline。Job 実行時間ではない |
| max-retry-attempts | `0` | Scheduler 側 retry は無効化、次の tick に任せる |
| Job args | `--once --max-events 1 --timeout 60s` | 1 起動で 1 event のみ処理（多重 lock 回避） |

---

## 2. 通常運用（自動起動の前提）

Scheduler が有効な間は **手動操作不要**。1 分ごとに Job が起動し、`outbox_events` の
`status='pending'` event を 1 件 pick して処理する。

- `image.became_available` event → 現状 no-op（PR33d で OGP 連携が wired 済、`photobook.published` event が来た場合のみ OGP 生成が動く）
- `photobook.published` event → OGP 生成（`photobook_ogp_images.status='generated'` に遷移、R2 PUT、`images` / `image_variants` row 作成）
- 失敗時 → Job exit 0 / event は再 pick されない既存仕様（reconcile は別経路）

OGP 同期化（ADR-0007）導入後は **publish handler 内の同期試行が p95 で成功**するため、
本 worker の役割は **同期失敗時の fallback** に変わった。Scheduler 1 min 化により、
publish 同期失敗から最大 60 s 以内に generated 化される SLO。

---

## 3. 起動確認手順

### 3.1 Scheduler の状態

```bash
PROJ=project-1c310480-335c-4365-8a8

gcloud scheduler jobs describe vrcpb-outbox-worker-tick \
  --location=asia-northeast1 --project=${PROJ} \
  --format='value(state,schedule,lastAttemptTime,status.code)'
# 期待: ENABLED / '* * * * *' / 直近 60 秒以内 / status.code 空 (success)
# 異常: status.code=7 → PERMISSION_DENIED（§4 IAM 確認へ）
```

### 3.2 直近 attempt の HTTP status

```bash
gcloud logging read \
  'resource.type=cloud_scheduler_job AND resource.labels.job_id=vrcpb-outbox-worker-tick AND jsonPayload.@type:"AttemptFinished"' \
  --project=${PROJ} --limit=10 --freshness=10m \
  --format='value(timestamp,severity,httpRequest.status)'
# 期待: 直近 10 件すべて 200
# 異常: 403 連続 → §4 IAM 確認 / §6 障害対応
```

### 3.3 Job executions 件数（直近 5 分）

```bash
gcloud run jobs executions list --job=vrcpb-outbox-worker \
  --region=asia-northeast1 --project=${PROJ} --limit=10 \
  --format='value(metadata.name,status.startTime,status.completionTime,status.succeededCount,status.failedCount)'
# 期待: 直近 5 分以内に 4-5 件、すべて succeededCount=1
# 異常: 0 件 → Scheduler 側 attempt が失敗している（§3.2 / §4 へ）
# 異常: failedCount=1 → Job 内部エラー（§3.4 Job logs へ）
```

### 3.4 Job log 健全性

```bash
gcloud logging read \
  'resource.type=cloud_run_job AND resource.labels.job_name=vrcpb-outbox-worker' \
  --project=${PROJ} --limit=15 --freshness=5m \
  --format='value(timestamp,severity,jsonPayload.msg,textPayload)'
# 期待 (起動時): "outbox-worker starting"
# 期待 (OGP 配線確認): "outbox-worker: ogp generator wired (photobook.published will trigger OGP generation)"
# 期待 (処理結果): "outbox processed" → "outbox-worker finished" → "Container called exit(0)"
```

### 3.5 Secret 漏洩 grep（運用観測）

```bash
gcloud logging read \
  'resource.type=cloud_run_job AND resource.labels.job_name=vrcpb-outbox-worker' \
  --project=${PROJ} --freshness=30m --limit=200 \
  --format='value(textPayload,jsonPayload)' \
  | grep -iE 'DATABASE_URL=|R2_SECRET|R2_ACCESS_KEY_ID=[A-Za-z0-9]{8}|TURNSTILE_SECRET|REPORT_IP_HASH_SALT=|sk_live_|sk_test_|password=[a-zA-Z]|manage_url=[a-zA-Z]|storage_key=[a-zA-Z0-9]|reporter_contact=[a-zA-Z@]|source_ip_hash=|turnstile_token=[a-zA-Z]|salt=[a-zA-Z]'
# 期待: 0 件
```

---

## 4. IAM 確認手順

Scheduler が `vrcpb-outbox-worker` Job を起動するには、Scheduler 発火 SA に
**Job 単位**の `roles/run.invoker` が必要。

> **重要**: image-processor 側に `roles/run.invoker` を付与しても、outbox-worker 側には
> 自動継承されない。Job ごとに付与する。

### 4.1 現在の IAM 状況確認

```bash
PROJ=project-1c310480-335c-4365-8a8
COMPUTE_SA=271979922385-compute@developer.gserviceaccount.com

gcloud run jobs get-iam-policy vrcpb-outbox-worker \
  --region=asia-northeast1 --project=${PROJ}
# 期待:
# bindings:
# - members:
#   - serviceAccount:271979922385-compute@developer.gserviceaccount.com
#   role: roles/run.invoker
# 異常 (空 / etag のみ): roles/run.invoker 未付与 → §4.2 で付与
```

### 4.2 `roles/run.invoker` 付与

```bash
gcloud run jobs add-iam-policy-binding vrcpb-outbox-worker \
  --region=asia-northeast1 --project=${PROJ} \
  --member="serviceAccount:${COMPUTE_SA}" \
  --role="roles/run.invoker"

# verify
gcloud run jobs get-iam-policy vrcpb-outbox-worker \
  --region=asia-northeast1 --project=${PROJ}
```

付与は冪等。複数回実行しても問題ない。1〜2 分後に §3.2 で attempt が 200 化することを確認。

### 4.3 関連: image-processor 側の IAM（参考）

```bash
gcloud run jobs get-iam-policy vrcpb-image-processor \
  --region=asia-northeast1 --project=${PROJ}
# image-processor 側は STOP γ 以前から compute SA に roles/run.invoker 付与済
```

---

## 5. Rollback / pause / delete 手順

### 5.1 一時停止（Scheduler のみ pause、Job は残す）

```bash
gcloud scheduler jobs pause vrcpb-outbox-worker-tick \
  --location=asia-northeast1 --project=${PROJ}

# 再開
gcloud scheduler jobs resume vrcpb-outbox-worker-tick \
  --location=asia-northeast1 --project=${PROJ}
```

publish 同期化を維持したまま fallback だけ止める場合に使う（OGP 同期は publish handler
内で動くため、Scheduler が止まっていても新規 publish は影響を受けない）。ただし同期失敗
case の OGP は **手動 execute するまで `pending` のまま**になることに注意。

### 5.2 間隔変更（1 min ↔ 5 min）

```bash
# 1 min → 5 min（負荷下げる）
gcloud scheduler jobs update http vrcpb-outbox-worker-tick \
  --schedule='*/5 * * * *' \
  --location=asia-northeast1 --project=${PROJ}

# 5 min → 1 min（fallback latency を下げる）
gcloud scheduler jobs update http vrcpb-outbox-worker-tick \
  --schedule='* * * * *' \
  --location=asia-northeast1 --project=${PROJ}
```

### 5.3 Scheduler 削除（rollback / 構成変更時）

```bash
gcloud scheduler jobs delete vrcpb-outbox-worker-tick \
  --location=asia-northeast1 --project=${PROJ}
```

Job 自体は残るため、手動 execute（`gcloud run jobs execute vrcpb-outbox-worker`）は引き続き可能。

### 5.4 IAM 付与の rollback

通常は不要。`roles/run.invoker` の付与は冪等で再付与可能、削除しても再付与で復旧。
削除コマンド（必要な場合のみ）:

```bash
gcloud run jobs remove-iam-policy-binding vrcpb-outbox-worker \
  --region=asia-northeast1 --project=${PROJ} \
  --member="serviceAccount:${COMPUTE_SA}" \
  --role="roles/run.invoker"
```

### 5.5 Job image tag を旧 SHA に戻す（Backend rollback と連動）

```bash
OLD_SHA=<前 deploy 時に控えた SHA>
gcloud run jobs update vrcpb-outbox-worker \
  --image=asia-northeast1-docker.pkg.dev/${PROJ}/vrcpb/vrcpb-api:${OLD_SHA} \
  --region=asia-northeast1 --project=${PROJ}
```

`backend-deploy.md` §2.1 の Service rollback と必ずセットで実施する（Service と Job の
image tag を 揃える運用方針）。

---

## 6. 障害対応

### 6.1 Scheduler 403 連続（PERMISSION_DENIED）

**症状**: §3.2 で `httpRequest.status=403` が連続、§3.3 で executions が増えない。

**主要原因**: `vrcpb-outbox-worker` Job に compute SA の `roles/run.invoker` が無い。

**対応**: §4.2 で付与。1〜2 分後に attempt が 200 化することを確認。

**詳細**: `harness/failure-log/2026-05-11_outbox-worker-job-iam-missing-on-creation.md`

### 6.2 Job executions が 0 件のまま

**症状**: §3.3 で 0 件、§3.1 で `state=ENABLED` だが §3.2 で attempt log すら出ない。

**主要原因**:
- Scheduler URI が間違っている（typo / region 違い / Job 名違い）→ §3.1 で `httpTarget.uri` を確認
- Scheduler 自体が一時的に backend で詰まっている → 10 分待って §3.2 再確認

### 6.3 Job 内部エラー（failedCount=1）

**症状**: §3.3 で `failedCount=1`、§3.4 で error log。

**主要原因と対応**:
- DATABASE_URL 不通: Cloud SQL Auth Proxy / VPC connector 問題（`backend-deploy.md` §5 参照）
- R2 認証失敗: Secret rotation 後の Job 側未更新 → Job image tag 再 update（Secret は `secretKeyRef` で渡るため image 同梱ではない、`gcloud run jobs update` で再注入）
- panic / unexpected error: §3.4 で stack trace を確認、`failure-log/` 起票

### 6.4 OGP が `pending` のまま長時間残る

**症状**: publish 後 60 s 経っても X / Discord crawler が default placeholder を取得。

**確認手順**:
1. §3.2 で Scheduler attempt が 200 か
2. §3.3 で Job が直近 1-2 分以内に起動しているか
3. §3.4 で `outbox processed` log が出ているか
4. publish 経路の `event=ogp_sync_result` ログ（Service 側）で `outcome` を確認:
   - `success` なら crawler cache 問題（X Card Validator で refresh）
   - `timeout` / `error` なら worker fallback で generated 化するはず、§3.3 で確認
5. それでも generated 化しない場合は手動 execute:
   ```bash
   gcloud run jobs execute vrcpb-outbox-worker \
     --region=asia-northeast1 --project=${PROJ} --wait
   ```

### 6.5 報告 / 記録時の注意

- raw photobook_id / slug / token / Secret / storage_key は **記録に出さない**
- Job execution name (`vrcpb-outbox-worker-xxxxx`) / revision 名 / attempt timestamp は運用情報として OK
- logs grep で hit した content を共有する場合は redact (`<redacted>` / 先頭 8 文字 + `...`) する

---

## 7. 関連資料

- `docs/runbook/backend-deploy.md` — Backend Service / Cloud Build / Cloud Run Jobs deploy 全般
- `docs/runbook/ops-moderation.md` — `cmd/ops photobook` 系運営コマンド
- `docs/plan/m2-image-processor-job-automation-plan.md` — `vrcpb-image-processor-tick` 側 Scheduler の構成元
- `harness/failure-log/2026-05-11_outbox-worker-job-iam-missing-on-creation.md` — STOP γ で発生した IAM 不足事故
- `.agents/rules/predeploy-verification-checklist.md` — deploy 前後の verification チェックリスト
- `.agents/rules/security-guard.md` — Secret / raw 値の禁止リスト

---

## 8. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-11 | 初版作成。STOP γ で `vrcpb-outbox-worker-tick` を ENABLED 化したタイミングで起票。IAM 不足事故も §4 / §6.1 として組み込み |
