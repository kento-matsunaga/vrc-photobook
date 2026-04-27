# PR29 Backend deploy 自動化 実装結果（2026-04-28、進行中）

## 概要

- 新正典 [`docs/plan/vrc-photobook-final-roadmap.md`](../../docs/plan/vrc-photobook-final-roadmap.md)
  §3 PR29 / 計画書 [`docs/plan/m2-backend-deploy-automation-plan.md`](../../docs/plan/m2-backend-deploy-automation-plan.md)
  §4〜§8 に従い、Cloud Build manual trigger による Backend 自動 deploy を導入
- 計画書 §8 の **6 個の停止ポイント**で逐次ユーザー承認を得て進行
- 本 work-log は進行に応じて随時追記する（途中経過記録）

## ファイル追加（commit `755c939`）

- `cloudbuild.yaml`: docker build → AR push → `gcloud run services update --image=`
  → smoke /health + /readyz の 4 step。tag は `$SHORT_SHA`、`logging: CLOUD_LOGGING_ONLY`、
  `timeout: 1200s`
  - Secret 値 / substitutions に Secret を入れない
  - `--update-env-vars` / `--update-secrets` を使わず Cloud Run 既存 secretKeyRef を維持
- `docs/runbook/backend-deploy.md`: Cloud Build manual trigger 経由 deploy /
  rollback / 緊急手動 deploy / Secret 漏洩確認 / よくある失敗と対処
- `CLAUDE.md`: Backend deploy は Cloud Build manual trigger 経由が標準、と追記

## STOP 1: Cloud Build API 有効化（承認・実行済）

### 承認

- ユーザー承認受領: 2026-04-28
- 実行内容: `cloudbuild.googleapis.com` のみ有効化、他 API は変更しない

### 実行結果

```
$ gcloud services enable cloudbuild.googleapis.com --project=<PROJ>
Operation "operations/acf.p2-271979922385-..." finished successfully.
```

before / after の比較:

| API | before | after |
|---|---|---|
| `artifactregistry.googleapis.com` | enabled | enabled（変更なし）|
| `cloudbuild.googleapis.com` | **無効** | **enabled**（新規）|
| `logging.googleapis.com` | enabled | enabled（変更なし）|
| `run.googleapis.com` | enabled | enabled（変更なし）|

**余計な API 有効化なし**を確認。

### 課金影響（再確認）

- 月 120 build-min まで無料、超過は $0.003/build-min
- 想定: 月 ~90 build-min → 無料枠内
- Budget Alert は PR39 本番運用整備で再設計予定

## STOP 2: Cloud Build 専用 SA 作成 + IAM 付与（承認・実行済）

### 承認

- ユーザー承認受領: 2026-04-28
- 実行内容: `vrcpb-cloud-build@<PROJ>.iam` を新設し、計画書 §5.2 の最小権限を付与

### 実行結果

```
$ gcloud iam service-accounts create vrcpb-cloud-build ...
Created service account [vrcpb-cloud-build].

$ for role in artifactregistry.writer run.developer logging.logWriter cloudbuild.builds.builder; do
    gcloud projects add-iam-policy-binding ... --role=roles/$role
  done
(全 4 role 付与成功)

$ gcloud iam service-accounts add-iam-policy-binding 271979922385-compute@... \
    --member=serviceAccount:vrcpb-cloud-build@... \
    --role=roles/iam.serviceAccountUser
(成功)
```

### 検証結果

| 観点 | 結果 |
|---|---|
| SA `vrcpb-cloud-build@<PROJ>.iam` 存在 | ✓（disabled=false） |
| project-level role: `artifactregistry.writer` | ✓ |
| project-level role: `run.developer` | ✓ |
| project-level role: `logging.logWriter` | ✓ |
| project-level role: `cloudbuild.builds.builder` | ✓ |
| runtime SA への `iam.serviceAccountUser` | ✓（runtime SA = `271979922385-compute@developer.gserviceaccount.com`） |
| `secretmanager.secretAccessor` が付与されていないこと | ✓（grep blank） |
| `cloudsql.client` が付与されていないこと | ✓ |
| `editor` / `owner` が付与されていないこと | ✓ |

## STOP 3: manual trigger 作成（承認・実行済 / 案 A 採用）

### 承認

- ユーザー承認受領: 2026-04-28
- **案 A 採用**: Cloud Build trigger オブジェクトを作らず、`gcloud builds submit` で
  cloudbuild.yaml を直接 invoke する方式を採用（計画書 §6.3 通り）

### 採用理由

- 計画書 §6.3「Cloud Build の GitHub App 連携は **不要**（manual trigger なら）」と整合
- 永続的 trigger / GitHub App 接続を持たないため、最小構成
- Cloud Build SA を `--service-account=` で明示 invoke できる
- GCP Console での GitHub OAuth / GitHub App インストール作業を回避

### 確定方針（PR29）

- Cloud Build trigger は **作成しない**
- GitHub App 連携 / Cloud Build GitHub connection は **しない**
- tag trigger も **作らない**
- main push 自動 deploy も **作らない**
- 標準 deploy は `gcloud builds submit --config=cloudbuild.yaml` 経由のローカル CLI
- `--service-account=projects/<PROJ>/serviceAccounts/vrcpb-cloud-build@...` を明示
- `options.logging: CLOUD_LOGGING_ONLY` を維持（cloudbuild.yaml 既設）

### 先送り事項（必ず後で再検討、忘れないこと）

| 項目 | 後で検討する場所 |
|---|---|
| Cloud Build trigger オブジェクト作成 | PR40（ローンチ前運用整理） |
| GitHub App / Cloud Build GitHub connection | PR38（Public repo 化）+ PR40 |
| tag trigger（`release-*` / `v*`） | PR40 / PR41+ |
| main push 自動 deploy | PR41+（M2 完了後、e2e test 充実後） |
| Frontend Workers deploy 自動化 | PR41+ |
| WIF（Workload Identity Federation）/ branch protection | PR38 Public repo 化と統合 |

これらは新正典 §3 PR41+ + 計画書 §6.2 / §10 / §14 と整合させて記録する。

## STOP 4: 初回 build 実行（承認・実行済 / 1 段階方式）

### 承認

- ユーザー承認受領: 2026-04-28、1 段階方式（`--no-traffic` 入れない）
- 実行対象 commit: 修正中に `6092604` → `c1592ec` → `50f940c` と 2 度 fix
- 理由: 初回試行で 2 件の fix 必要だった（後述）

### 修正 1: cloudbuild.yaml の bash substitution エスケープ

```
ERROR: INVALID_ARGUMENT: key in the template "URL" is not a valid built-in substitution
```

原因: cloudbuild.yaml の smoke step で bash 変数 `$URL` / `$path` / `$code` を
そのまま書いていたが、Cloud Build substitution parser が予約変数として解釈。
修正: `$$` でエスケープ（`$$URL` / `$${URL}$${path}` 等）、commit `c1592ec`。

### 修正 2: build context path の不一致（`.gcloudignore` 追加）

```
unable to prepare context: path "backend" not found
```

原因: `gcloud builds submit backend/` で backend ディレクトリのみ upload。
cloudbuild.yaml は repo-root 相対 path（`backend/Dockerfile` + context `backend`）
を想定していたため矛盾。
修正: repo root から submit + `.gcloudignore` で frontend / design / docs / harness 等
build に不要な大きいディレクトリを除外。commit `50f940c`。

### 実行結果（成功）

```
$ gcloud builds submit /home/erenoa6621/dev/vrc_photobook \
    --config=cloudbuild.yaml \
    --substitutions=SHORT_SHA=50f940c \
    --service-account=projects/<PROJ>/serviceAccounts/vrcpb-cloud-build@... \
    --project=<PROJ>
...
ID: 18395bac-8ab0-4f6b-b5f1-839bccae53ef
DURATION: 3M50S
STATUS: SUCCESS
IMAGES: asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb/vrcpb-api:50f940c
```

### 検証結果

| 観点 | 結果 |
|---|---|
| Cloud Build 成功 | ✓ build `18395bac-...` 3 分 50 秒 |
| AR push | ✓ `vrcpb-api:50f940c` push 完了（2026-04-28 02:44 UTC） |
| Cloud Run 新 revision | ✓ `vrcpb-api-00011-xfd` 作成 |
| traffic 100% 新 revision | ✓ 100% を `vrcpb-api-00011-xfd` に向いている |
| /health 200 | ✓ `{"status":"ok"}` |
| /readyz 200 | ✓ `{"status":"ready"}` |
| edit-view no Cookie 401 | ✓ `{"status":"unauthorized"}` |
| env / secretKeyRef 維持 | ✓ APP_ENV / ALLOWED_ORIGINS / DATABASE_URL / R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET_NAME / R2_ENDPOINT / TURNSTILE_SECRET_KEY すべて存在 |
| Cloud Build smoke step output | `smoke ok: /health -> HTTP 200` / `smoke ok: /readyz -> HTTP 200` |
| Cloud Build logs Secret leak | 0 件（grep 空） |
| Cloud Run logs Secret leak | 0 件（grep 空） |

### Rollback 用に控え

- **rollback 先**: `vrcpb-api-00010-7vz`（image `vrcpb-api:3ec5080`）
- その前: `vrcpb-api-00009-wdb`（image `vrcpb-api:2a93f8c`）

## STOP 5: traffic 切替後の確認（承認・実行済 / rollback 不要判定）

> 1 段階方式のため STOP 5 は「切替**後**の確認・rollback 要否判断」として扱った。

### 承認

- ユーザー承認受領: 2026-04-28、rollback 不要判定
- `vrcpb-api-00011-xfd` を現行 revision として維持

### 確認済み事項

- Cloud Build SUCCESS / AR push 済 / 新 revision 100% traffic
- /health 200 / /readyz 200 / edit-view no Cookie 401
- env / secretKeyRef 9 件維持
- Cloud Build logs / Cloud Run logs に Secret 漏洩なし

## STOP 6: rollback 確認 1 回実行（承認・実行済）

### 承認

- ユーザー承認受領: 2026-04-28、rollback 訓練 1 回実行
- 影響範囲評価: 両 revision は Backend Go コード差分なし、機能上等価で影響最小

### 実行結果

| Step | コマンド | 結果 |
|---|---|---|
| 1 | 訓練前 state 確認 | traffic 100% = `vrcpb-api-00011-xfd` |
| 2 | `gcloud run services update-traffic --to-revisions=vrcpb-api-00010-7vz=100` | **成功**（旧 revision に切替） |
| 3 | 旧 revision で smoke | /health 200 / /readyz 200 / edit-view no Cookie 401 |
| 4 | `gcloud run services update-traffic --to-revisions=vrcpb-api-00011-xfd=100` | **成功**（新 revision に戻る） |
| 5 | 新 revision で smoke 再確認 | /health 200 / /readyz 200 / edit-view no Cookie 401 |
| 6 | env / secretKeyRef 不変確認 | 9 件維持（APP_ENV / ALLOWED_ORIGINS / DATABASE_URL / R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET_NAME / R2_ENDPOINT / TURNSTILE_SECRET_KEY） |

### Runbook 検証結果

`docs/runbook/backend-deploy.md` §2.1 の rollback 手順
（`gcloud run services update-traffic vrcpb-api --to-revisions=<NAME>=100`）が
**実環境で 1 度の操作で機能することを確認**。

訓練の所要時間: ~30 秒（traffic 切替 2 回 + smoke 2 回）。

## PR29 完了サマリ

### 達成事項

| # | 項目 | 結果 |
|---|---|---|
| 1 | cloudbuild.yaml 追加 | 4 step（build / push / Cloud Run update / smoke）|
| 2 | runbook 追加 | `docs/runbook/backend-deploy.md`（manual submit + rollback + 緊急手動 + 後続タスク）|
| 3 | Cloud Build API 有効化 | `cloudbuild.googleapis.com` enabled、他 API 変動なし |
| 4 | 専用 SA 作成 | `vrcpb-cloud-build@<PROJ>.iam` 新設 |
| 5 | IAM 最小権限付与 | 5 role（artifactregistry.writer / run.developer / iam.serviceAccountUser / logging.logWriter / cloudbuild.builds.builder）。secretmanager.secretAccessor は付与しない |
| 6 | 初回 build 成功 | image `vrcpb-api:50f940c`、revision `vrcpb-api-00011-xfd` |
| 7 | smoke 全 pass | /health 200 / /readyz 200 / edit-view 401 |
| 8 | rollback 訓練成功 | runbook §2 通りに動作確認 |
| 9 | Secret 漏洩なし | Cloud Build logs / Cloud Run logs / docs / commit すべて grep 0 件 |

### 確定方針（manual submit 方式）

- 標準 deploy: `gcloud builds submit --config=cloudbuild.yaml --service-account=vrcpb-cloud-build@... <repo-root>`
- Cloud Build trigger オブジェクトは作らない（後続 PR40 で再検討）
- main push 自動 deploy はしない（PR41+ で再検討）
- env / secretKeyRef は `--image=` 単独更新で維持

### 先送り項目（PR40 / PR41+ で必ず再検討、忘れないこと）

| 項目 | 後で検討する PR |
|---|---|
| Cloud Build trigger オブジェクト作成（GCP Console ワンクリック） | PR40 |
| GitHub App / Cloud Build GitHub connection | PR38 + PR40 |
| tag trigger（`release-*`） | PR40 / PR41+ |
| main push 自動 deploy | PR41+ |
| Frontend Workers deploy 自動化 | PR41+ |
| WIF / branch protection 整備 | PR38 |
| Artifact Registry retention policy | PR40 |
| Cloud Build machineType 昇格 | PR41+ |

これらは runbook §6 / 計画書 §6.4 / 新正典 §1.3 / §3 PR40 / §3 PR41+ に同期記録済。

### PR28 残課題（独立して継続）

「実画像を含む完全 visual Safari 確認」は PR29 と独立。引き続き **manual 残課題**として
新正典 §1.3 / [`harness/work-logs/2026-04-27_publish-flow-result.md`](./2026-04-27_publish-flow-result.md)
§推奨次手順を参照。PR29 進行中も並行実施可能だったが、本 PR では実施していない。

## 実施しなかったこと（計画通り）

- Cloud Build trigger オブジェクト作成
- GitHub App / Cloud Build GitHub connection
- tag trigger
- main push 自動 deploy
- Frontend Workers deploy 自動化
- Cloud Run Jobs / Scheduler（PR31）
- SendGrid / Outbox / OGP 自動生成
- Cloud SQL 本番化 / spike 削除 / Public repo 化

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（PR29 開始）。STOP 1 承認・実行記録 |
| 2026-04-28 | STOP 2 承認・実行記録（SA 作成 + IAM 付与） |
| 2026-04-28 | STOP 3 承認・実行記録（manual submit 方式採用 / 先送り項目 4 ファイル明記） |
| 2026-04-28 | STOP 4 承認・実行記録（cloudbuild.yaml fix 2 件 + 初回 build 成功） |
| 2026-04-28 | STOP 5 承認（rollback 不要判定） |
| 2026-04-28 | STOP 6 承認・実行記録（rollback 訓練成功、runbook §2 検証完了）|
| 2026-04-28 | PR29 完了 |

## PR28 残課題の継続

「実画像を含む完全 visual Safari 確認」は本 PR と独立して継続。
PR29 進行中も並行して manual 実施可能（手順は
[`harness/work-logs/2026-04-27_publish-flow-result.md`](./2026-04-27_publish-flow-result.md)
§推奨次手順）。

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（PR29 進行中）。STOP 1 承認・実行を記録。STOP 2 承認待ち |
