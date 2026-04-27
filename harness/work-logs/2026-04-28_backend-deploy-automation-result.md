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

## STOP 4: 初回 build 実行（**承認待ち**）

予定: 後段 §STOP 4 セクションで詳細を提示してから承認受け待つ。

## STOP 5〜6 / smoke / traffic 切替 / rollback

未実施。STOP 4 承認後に順次進行。

## PR28 残課題の継続

「実画像を含む完全 visual Safari 確認」は本 PR と独立して継続。
PR29 進行中も並行して manual 実施可能（手順は
[`harness/work-logs/2026-04-27_publish-flow-result.md`](./2026-04-27_publish-flow-result.md)
§推奨次手順）。

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（PR29 進行中）。STOP 1 承認・実行を記録。STOP 2 承認待ち |
