# Backend deploy runbook

> 設計: [`docs/plan/m2-backend-deploy-automation-plan.md`](../plan/m2-backend-deploy-automation-plan.md)
>
> Backend Cloud Run (`vrcpb-api`) の deploy / rollback 手順をまとめる。
> 本書は**実運用の手順書**であり、計画書ではない。Cloud Build trigger を経由する
> 経路と、緊急時の手動経路の両方を記載する。

---

## 0. 前提

- GCP project: `project-1c310480-335c-4365-8a8`
- Cloud Run service: `vrcpb-api`（asia-northeast1）
- Artifact Registry repo: `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb`
- Cloud Build trigger: manual（GCP Console / `gcloud builds triggers run` で起動）
- Cloud Build 専用 SA: `vrcpb-cloud-build@<PROJ>.iam.gserviceaccount.com`
- env / secretKeyRef は Cloud Run service に既設。`--image=` 単独更新で不変
- 既存ロールバック先 image / revision は **deploy 直前に必ず控える**

---

## 1. Cloud Build manual trigger 経由 deploy（標準手順）

### 1.1 事前確認

```bash
PROJ=project-1c310480-335c-4365-8a8

# 現 revision / image を控える（rollback 時に使う）
gcloud run services describe vrcpb-api \
  --region=asia-northeast1 --project=$PROJ \
  --format='value(spec.template.spec.containers[0].image,status.latestReadyRevisionName)'

# rollback 用に直前 revision も列挙
gcloud run revisions list \
  --service=vrcpb-api --region=asia-northeast1 --project=$PROJ \
  --format='value(metadata.name,spec.containers[0].image,status.conditions[0].lastTransitionTime)' \
  --limit=5
```

### 1.2 trigger 実行

```bash
# Console: Cloud Build > Triggers > <trigger-name> > Run trigger
# CLI:
gcloud builds triggers run <trigger-name> \
  --branch=main --project=$PROJ
```

### 1.3 build 進捗確認

```bash
# 直近 build を確認
gcloud builds list --project=$PROJ --limit=5 \
  --format='value(id,status,createTime,source.repoSource.commitSha)'

# 詳細 logs（Secret 漏洩がないか軽く目視）
gcloud builds log <BUILD_ID> --project=$PROJ | tail -50
```

### 1.4 deploy 確認

```bash
# 新 revision を確認
gcloud run services describe vrcpb-api \
  --region=asia-northeast1 --project=$PROJ \
  --format='value(spec.template.spec.containers[0].image,status.latestReadyRevisionName)'

# smoke（cloudbuild.yaml の smoke step も実行されているが、念のため再実行）
curl -sS https://api.vrc-photobook.com/health
curl -sS https://api.vrc-photobook.com/readyz
curl -sS -o /dev/null -w "HTTP %{http_code}\n" \
  https://api.vrc-photobook.com/api/photobooks/00000000-0000-0000-0000-000000000000/edit-view
# 期待: /health 200 ok / /readyz 200 ready / edit-view 401 unauthorized
```

### 1.5 work-log 記録

`harness/work-logs/YYYY-MM-DD_*.md` に以下を記録:

- 実行 commit (SHORT_SHA)
- 新 revision name + image
- rollback 用に控えた前 revision name + image
- smoke 結果
- 異常があれば failure-log にも起票

---

## 2. Rollback 手順

### 2.1 前 revision に traffic を戻す

```bash
PROJ=project-1c310480-335c-4365-8a8

# 1) 戻したい前 revision を特定
gcloud run revisions list \
  --service=vrcpb-api --region=asia-northeast1 --project=$PROJ \
  --format='value(metadata.name,spec.containers[0].image)' --limit=5

# 2) traffic を 100% 戻す（例: vrcpb-api-00009-wdb）
gcloud run services update-traffic vrcpb-api \
  --to-revisions=vrcpb-api-00009-wdb=100 \
  --region=asia-northeast1 --project=$PROJ

# 3) smoke
curl -sS https://api.vrc-photobook.com/health
curl -sS https://api.vrc-photobook.com/readyz
```

### 2.2 rollback 後に新 revision に戻す

問題の原因を修正した後、`update-traffic` で再度新 revision を 100% に切替えるか、
新しい commit で Cloud Build trigger を回す。

```bash
# 新 revision に traffic を 100% 戻す
gcloud run services update-traffic vrcpb-api \
  --to-revisions=<NEW_REVISION>=100 \
  --region=asia-northeast1 --project=$PROJ
```

### 2.3 work-log + failure-log 記録

- rollback 理由 / 切戻し先 revision / smoke 結果を `harness/work-logs/` に記録
- 障害情報は `harness/failure-log/` に起票し、再発防止策を `.agents/rules/` 化検討

---

## 3. 緊急時の手動 deploy（Cloud Build trigger が使えない場合）

Cloud Build API 障害 / IAM 失効時の fallback。

```bash
PROJ=project-1c310480-335c-4365-8a8
SHORT=$(git rev-parse --short=7 HEAD)
IMAGE=asia-northeast1-docker.pkg.dev/$PROJ/vrcpb/vrcpb-api:$SHORT

# 1) build
docker build -f backend/Dockerfile -t "$IMAGE" backend

# 2) push（事前に gcloud auth configure-docker asia-northeast1-docker.pkg.dev 必要）
docker push "$IMAGE"

# 3) Cloud Run revision 更新
gcloud run services update vrcpb-api \
  --image="$IMAGE" --region=asia-northeast1 --project=$PROJ

# 4) smoke
curl -sS https://api.vrc-photobook.com/health
curl -sS https://api.vrc-photobook.com/readyz
```

> 緊急時の手動 deploy も work-log に記録すること。

---

## 4. Secret 漏洩確認（deploy 前後）

```bash
# Cloud Build logs に Secret が出ていないか
gcloud builds log <BUILD_ID> --project=$PROJ \
  | grep -iE "DATABASE_URL=|R2_SECRET_ACCESS_KEY=|TURNSTILE_SECRET_KEY=|sk_live|sk_test" \
  || echo "no leak in build logs"

# Cloud Run logs に Secret が出ていないか（直近 revision）
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=vrcpb-api" \
  --project=$PROJ --limit=50 \
  | grep -iE "DATABASE_URL=|R2_SECRET_ACCESS_KEY=|TURNSTILE_SECRET_KEY=" \
  || echo "no leak in cloud run logs"
```

---

## 5. よくある失敗と対処

### 5.1 Cloud Build API が無効

```
ERROR: ... Cloud Build API has not been used in project ...
```

対処:
```bash
gcloud services enable cloudbuild.googleapis.com --project=$PROJ
# ※ 課金開始ポイント。必ずユーザー承認後に実行。
```

### 5.2 Cloud Build SA に IAM 不足

build step で `Permission denied` / `403`:

- `roles/artifactregistry.writer`（push 失敗時）
- `roles/run.developer`（gcloud run services update 失敗時）
- `roles/iam.serviceAccountUser`（Cloud Run runtime SA に対して、revision 作成失敗時）
- `roles/logging.logWriter`（Cloud Build logs 出力失敗時）
- `roles/cloudbuild.builds.builder`（build 自体の実行失敗時）

確認:
```bash
gcloud projects get-iam-policy $PROJ \
  --flatten='bindings[].members' \
  --filter='bindings.members:vrcpb-cloud-build@*' \
  --format='value(bindings.role)'
```

### 5.3 Artifact Registry push 失敗

- AR repo 未作成 → `gcloud artifacts repositories create vrcpb --repository-format=docker --location=asia-northeast1`
- docker auth 未設定（手動 deploy 時）→ `gcloud auth configure-docker asia-northeast1-docker.pkg.dev`

### 5.4 Cloud Run deploy 権限不足

- Cloud Build SA に `roles/run.developer` 不足
- runtime SA に対する `roles/iam.serviceAccountUser` 不足

### 5.5 smoke 失敗

- 新 revision の起動失敗（image 内 panic / config 不足）
- env / secretKeyRef が消えていないか確認:

```bash
gcloud run services describe vrcpb-api --region=asia-northeast1 --project=$PROJ \
  --format='yaml(spec.template.spec.containers[0].env)' \
  | sed -E 's|(value:).*|\1 <REDACTED>|; s|(secretKeyRef:).*|\1 <ref>|'
```

### 5.6 env / secretKeyRef が消えた場合（最重要）

`--image=` 単独更新で env / secretKeyRef が消えるのは想定外。万一発生したら:

1. **即 rollback**（§2）
2. Cloud Build trigger / cloudbuild.yaml を見直し、`--update-env-vars=` / `--clear-env-vars` /
   `--update-secrets=` を使っていないか確認
3. `failure-log/` に起票して再発防止ルール化

---

## 6. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（PR29）。Cloud Build manual trigger 経由 deploy + 緊急手動経路 + rollback |
