# Backend deploy runbook

> 設計: [`docs/plan/m2-backend-deploy-automation-plan.md`](../plan/m2-backend-deploy-automation-plan.md)
>
> Backend Cloud Run (`vrcpb-api`) の deploy / rollback 手順をまとめる。
> 本書は**実運用の手順書**であり、計画書ではない。
>
> **PR29 採用方式**: Cloud Build **trigger オブジェクトは作成しない**。
> ローカル CLI から `gcloud builds submit` で `cloudbuild.yaml` を直接 invoke する
> **manual submit 方式**を標準とする。trigger 化は §7 の後続タスクで再検討。

---

## 0. 前提

- GCP project: `project-1c310480-335c-4365-8a8`
- Cloud Run service: `vrcpb-api`（asia-northeast1）
- Artifact Registry repo: `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb`
- 標準 deploy 方式: **`gcloud builds submit` で manual invoke**（trigger オブジェクト無し）
- Cloud Build 専用 SA: `vrcpb-cloud-build@<PROJ>.iam.gserviceaccount.com`
- env / secretKeyRef は Cloud Run service に既設。`--image=` 単独更新で不変
- 既存ロールバック先 image / revision は **deploy 直前に必ず控える**

---

## 1. 標準 deploy 手順（PR29 採用: trigger 無し / `gcloud builds submit`）

### 1.1 事前確認

```bash
PROJ=project-1c310480-335c-4365-8a8
SA_EMAIL=vrcpb-cloud-build@${PROJ}.iam.gserviceaccount.com

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

### 1.2 build 起動（gcloud builds submit）

```bash
SHORT=$(git rev-parse --short=7 HEAD)

# source は backend/ ディレクトリのみアップロード（context 最小化）
# --service-account= で vrcpb-cloud-build@... を明示（default SA を使わない）
gcloud builds submit /home/erenoa6621/dev/vrc_photobook/backend \
  --config=/home/erenoa6621/dev/vrc_photobook/cloudbuild.yaml \
  --substitutions=SHORT_SHA=${SHORT} \
  --service-account=projects/${PROJ}/serviceAccounts/${SA_EMAIL} \
  --project=${PROJ}
```

> `--service-account=` は **必須**。指定しないと Cloud Build default SA（過剰権限）が
> 使われる。
>
> 同時に `cloudbuild.yaml` 側で `options.logging: CLOUD_LOGGING_ONLY` 必須。
> 指定 SA に GCS Writer 権限が無いため、default の GCS logging では即 fail する。

### 1.3 build 進捗確認

```bash
# 直近 build を確認
gcloud builds list --project=$PROJ --limit=5 \
  --format='value(id,status,createTime)'

# 詳細 logs（Secret 漏洩がないか軽く目視）
gcloud builds log <BUILD_ID> --project=$PROJ | tail -50
```

### 1.4 deploy 確認

```bash
# 新 revision + traffic 配分を確認
gcloud run services describe vrcpb-api \
  --region=asia-northeast1 --project=$PROJ \
  --format='value(spec.template.spec.containers[0].image,status.latestReadyRevisionName,status.traffic[0].revisionName,status.traffic[0].percent)'

# 期待: latestReadyRevisionName と traffic[0].revisionName が一致 / traffic[0].percent=100
# 不一致の場合は §5.7「build success だが traffic が旧 revision に残る」を確認

# smoke（cloudbuild.yaml の smoke step も実行されているが、念のため再実行）
curl -sS https://api.vrc-photobook.com/health
curl -sS https://api.vrc-photobook.com/readyz
curl -sS -o /dev/null -w "HTTP %{http_code}\n" \
  https://api.vrc-photobook.com/api/photobooks/00000000-0000-0000-0000-000000000000/edit-view
# 期待: /health 200 ok / /readyz 200 ready / edit-view 401 unauthorized
```

> **必ず確認**: `cloudbuild.yaml` の `traffic-to-latest` step により、deploy 直後に
> traffic が最新 revision に明示切替されている。万一この step が失敗していると、
> build SUCCESS でも traffic は旧 revision のままになり、smoke は旧 revision を見て
> 200 を返してしまう（実害が遅延発覚する）。最終確認は **revision 名一致**で行うこと。

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

> **重要**: `update-traffic --to-revisions=<X>=100` を実行すると、Cloud Run の traffic
> 設定は **「特定 revision に pin」状態**になる。この状態では、`gcloud run services
> update --image=...` で新しい revision を作っても traffic は移動しない。
>
> 通常運用に戻すには以下のいずれか:
>
> - 次の Cloud Build deploy（`cloudbuild.yaml` 末尾の `traffic-to-latest` step が自動で pin を解除）
> - 手動で `gcloud run services update-traffic vrcpb-api --to-latest --region=asia-northeast1 --project=$PROJ`
>
> 詳細は §5.7。

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

### 5.7 build success だが traffic が旧 revision に残る（rollback drill 後の traffic pin）

PR30 deploy 時に発生（`harness/failure-log/2026-04-28_cloudbuild-traffic-pin-not-switched.md`）。

#### 症状

- Cloud Build: build / push / deploy / smoke すべて SUCCESS
- 新 revision は作成済（`gcloud run revisions list` で見える）
- しかし `status.traffic[0].revisionName` が **旧 revision** のまま
- 独自ドメイン `https://api.vrc-photobook.com/...` は旧 revision を返している
- smoke も旧 revision を見て 200 を返してしまう（false positive）

#### 原因

`gcloud run services update-traffic --to-revisions=<X>=100` を実行すると Cloud Run の
traffic 設定は「特定 revision に pin」状態になる。この状態では `gcloud run services
update --image=...` だけでは traffic が新 revision に流れない（pin が優先される）。

PR29 STOP 6 のロールバックドリル後に traffic が pin 状態のまま残っており、PR30 の
Cloud Build deploy で初めて顕在化した。

#### 恒久対策（PR30 完了後の独立タスクで適用済）

`cloudbuild.yaml` の deploy step 直後に `update-traffic --to-latest` step
（id: `traffic-to-latest`）を追加。これにより:

- pin 状態でも必ず latest revision に traffic 100% を向ける
- 通常運用（pin なし）でも冪等動作で副作用なし
- smoke は traffic 切替後に走るため、必ず新 revision を検証する

#### 暫定対処（cloudbuild.yaml 修正前 / 別事故で再発した場合）

```bash
gcloud run services update-traffic vrcpb-api \
  --to-latest \
  --region=asia-northeast1 --project=$PROJ
```

または明示的に新 revision を指定:

```bash
gcloud run services update-traffic vrcpb-api \
  --to-revisions=<NEW_REVISION>=100 \
  --region=asia-northeast1 --project=$PROJ
```

#### 必ず確認

- deploy 完了報告では **`status.latestReadyRevisionName == status.traffic[0].revisionName`**
  を必須チェック項目とする
- build SUCCESS だけで「deploy 成功」とみなさない（false positive 防止）

---

## 6. 後続タスク（PR29 で先送りした項目、忘れないこと）

PR29 では **manual submit 方式**を採用したため、以下の項目を意図的に未実施として残している。
**忘れないために本書 + 新正典 + 計画書 + work-log すべてに同じ記録を残す**。

| 項目 | 後で検討する PR / 場所 | 備考 |
|---|---|---|
| Cloud Build trigger オブジェクト作成（GCP Console からワンクリック起動） | PR40（ローンチ前運用整理）| 現状は CLI のみ |
| GitHub App / Cloud Build GitHub connection（2nd gen） | PR38（Public repo 化）+ PR40 | private repo のままなら不要 |
| tag trigger（`release-*` / `v*` push で自動 deploy） | PR40 / PR41+ | tag 運用ルール策定が必要 |
| main push 自動 deploy | PR41+（M2 完了後、e2e test 充実後） | 事故リスク高、初期段階では避ける |
| Frontend Workers deploy 自動化 | PR41+ | 現状 `npm run cf:build` + `wrangler deploy` 手動 |
| WIF（Workload Identity Federation）/ branch protection | PR38 + PR40 | Public repo 化と統合 |
| Artifact Registry retention policy（過去 image 自動 cleanup） | PR40 | 現状は手動 |
| Cloud Build machineType の昇格（速度改善） | PR41+ | 必要になったら E2_HIGHCPU_8 |
| Budget Alert 再設計 | PR39（本番運用整備） | 現状 Budget API 未有効 |

### 移行時の参照点

trigger 化に進むとき:

1. Cloud Build > Triggers > Connect Repository（GitHub App or 2nd gen connection）
2. trigger 作成: `gcloud builds triggers create manual --name=vrcpb-api-deploy --build-config=cloudbuild.yaml`
3. 本書 §1 の `gcloud builds submit` 経路は緊急用に残す（CLI なら trigger 無くても動く）

---

## 7. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（PR29）。manual submit 方式 (`gcloud builds submit` + 専用 SA) を採用。trigger / GitHub App / tag trigger / main push auto-deploy / frontend deploy 自動化 は後続タスクとして §6 に記録 |
| 2026-04-28 | PR30 完了後の独立タスクで `cloudbuild.yaml` に `traffic-to-latest` step を追加。§1.4 deploy 確認に traffic 一致チェックを追記、§2.2 rollback 後の pin 効果を明記、§5.7 の FAQ を追加 |
