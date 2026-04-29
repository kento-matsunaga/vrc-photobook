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

# source は **repo root** をアップロードする。`.gcloudignore` で frontend / docs /
# harness 等の不要ディレクトリを除外して context を最小化する（PR29 commit `50f940c`、
# 実測 10 MiB / 約 300 files）。
# --service-account= で vrcpb-cloud-build@... を明示（default SA を使わない）
gcloud builds submit /home/erenoa6621/dev/vrc_photobook \
  --config=/home/erenoa6621/dev/vrc_photobook/cloudbuild.yaml \
  --substitutions=SHORT_SHA=${SHORT} \
  --service-account=projects/${PROJ}/serviceAccounts/${SA_EMAIL} \
  --project=${PROJ}
```

> **source は backend/ ではなく repo root**。`cloudbuild.yaml` の build step は
> `-f backend/Dockerfile` + context `backend` を参照するため、submit する source の
> 最上位に `backend/` ディレクトリが見える必要がある（§5.8 参照）。
>
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

### 1.4.1 deploy / traffic 切替直後の安定化待ち（必須）

> **`/health` `/readyz` だけを smoke の合格基準にしてはならない。** 公開 Viewer / Report
> 経路など `/api/public/photobooks/{slug}` の handler 到達まで含めて確認する。

`gcloud builds submit` 直後 / `gcloud run services update-traffic` 直後の Cloud Run は、
旧 / 新 revision の instance 切替で routing が短時間不安定になることがある（観測例:
deploy 直後 1〜3 分で `/api/public/photobooks/<slug>` GET が **chi default の plain text
"404 page not found" を返す**ケース、`harness/failure-log/2026-04-29_public-photobook-route-unregistered-after-report-guard-deploy.md`）。

そのため smoke を始める前に **必ず 5〜10 分待ってから**実施する:

```bash
# 5〜10 分待ってから次の smoke に進む
echo "wait 5-10 minutes for Cloud Run routing transient to settle..."
```

### 1.4.2 public route handler 到達 smoke（必須）

`/health` / `/readyz` が 200 でも、`/api/public/photobooks/{slug}` の **handler に到達して
JSON 応答が返ること**を確認する。chi default NotFound（plain text）の場合は **failed
判定**で扱う。

```bash
# A. 不在 slug（handler から JSON 404 期待）
RESP=$(curl -s -w "\n%{http_code}" \
  https://api.vrc-photobook.com/api/public/photobooks/aaaaaaaaaaaaaaaaaa)
BODY=$(echo "$RESP" | head -n 1)
CODE=$(echo "$RESP" | tail -n 1)
echo "bad-slug: HTTP=$CODE body=$BODY"
# 期待: HTTP=404 body={"status":"not_found"}
# 失敗例（NG）: HTTP=404 body=404 page not found  ← chi default、route 未到達

# B. hidden 対象 slug がある場合（handler から JSON 410 gone 期待）
# raw slug は work-log に書かない / コマンド履歴にも残さないように $SLUG 等の env 化
# RESP=$(curl -s -w "\n%{http_code}" \
#   "https://api.vrc-photobook.com/api/public/photobooks/${SLUG}")
# 期待: HTTP=410 body={"status":"gone"}（hidden_by_operator=true 時）

# C. published 対象 slug がある場合（handler から JSON 200 期待）
# 期待: HTTP=200 + view JSON（slug, title, pages 等）
```

#### 合否判定

- **OK**: A は `HTTP=404 + {"status":"not_found"}`。B / C があるなら同様に handler JSON
- **NG（chi default 落ち）**: A が `HTTP=404 + 404 page not found`（plain text、19 bytes）
  → handler 未到達。**追加操作せず 5 分待って再確認**。それでも NG なら traffic を直前
  revision に rollback する判断を行う（§2 Rollback）。

#### Secret 漏洩 grep（必須）

deploy / traffic 切替後の **Cloud Build logs + Cloud Run logs（新 revision 名）** に対し、
`salt` / `secret` / `password` / `cookie` / `manage_url` / `storage_key` / `reporter_contact`
/ `source_ip_hash` / `turnstile_token` / `DATABASE_URL` の値が出ていないか grep する
（パターンは `.agents/rules/security-guard.md`）。0 件であることを確認。

> **work-log / commit / chat への記録**: raw slug / raw photobook_id / raw URL / 任意の
> Secret 値は出さない。redact（先頭 8 文字 + `...` / `<redacted>`）に揃える。

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

# 3) smoke（§1.4.1 / §1.4.2 と同条件: 5〜10 分待ち + public route handler 到達確認）
curl -sS https://api.vrc-photobook.com/health
curl -sS https://api.vrc-photobook.com/readyz
# 公開 Viewer 復旧確認（必須）
curl -s -w "\nHTTP=%{http_code}\n" \
  https://api.vrc-photobook.com/api/public/photobooks/aaaaaaaaaaaaaaaaaa
# 期待: HTTP=404 body={"status":"not_found"}（chi default plain text なら failed）
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

# 4) smoke（§1.4.1 / §1.4.2 と同条件: 5〜10 分待ち + public route handler 到達確認）
curl -sS https://api.vrc-photobook.com/health
curl -sS https://api.vrc-photobook.com/readyz
curl -s -w "\nHTTP=%{http_code}\n" \
  https://api.vrc-photobook.com/api/public/photobooks/aaaaaaaaaaaaaaaaaa
# 期待: HTTP=404 body={"status":"not_found"}（chi default plain text なら failed）
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

### 5.8 build step #0 が `unable to prepare context: path "backend" not found` で失敗

`harness/failure-log/2026-04-29_runbook-backend-deploy-section-outdated.md`、
PR29 work-log §「修正 2」と同事象。

#### 症状

```
Cloud Build <BUILD_ID> FAILURE
Step #0 - "build": unable to prepare context: path "backend" not found
```

#### 原因

`gcloud builds submit` の source として `<repo-root>/backend/` ディレクトリだけを
渡したケース。`cloudbuild.yaml` の build step は `-f backend/Dockerfile` + context
`backend` を参照するため、submit する source の最上位に `backend/` が見える必要
がある（§1.2）。

#### 対処

- §1.2 のサンプル通り **repo root から submit**:
  `gcloud builds submit /home/erenoa6621/dev/vrc_photobook \ --config=/home/erenoa6621/dev/vrc_photobook/cloudbuild.yaml \ ...`
- `.gcloudignore` で frontend / docs / harness 等を除外しているため、repo root submit
  でも upload は最小化される（PR29 commit `50f940c`）

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
| 2026-04-29 | PR35b STOP ε2 で発見した「deploy 直後 transient で `/api/public/photobooks/{slug}` GET が chi default plain text 404 を返す」事象を踏まえ、§1.4.1 安定化待ち（5〜10 分）と §1.4.2 public route handler 到達 smoke を必須化。§2.1 / §3 の rollback / 緊急 deploy 経路にも反映。詳細: `harness/failure-log/2026-04-29_public-photobook-route-unregistered-after-report-guard-deploy.md` |
