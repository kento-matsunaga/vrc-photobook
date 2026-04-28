# Backend deploy runbook §1.2 のサンプルが古く Cloud Build context path failure 再発（2026-04-29）

## 発生状況

- PR34b STOP β（`vrcpb-api:0db0d7c` を Cloud Run に deploy）の初回実行
- 実行コマンド: `gcloud builds submit /home/erenoa6621/dev/vrc_photobook/backend ...`
- runbook `docs/runbook/backend-deploy.md` §1.2 のサンプルに従い、source として `backend/` ディレクトリを指定

## 失敗内容

```
Cloud Build 5ce38f50-2937-403a-9c51-ad33824f0694 FAILURE
Step #0 - "build": unable to prepare context: path "backend" not found
```

- Cloud Build step 0（docker build）で context path 不一致により即 fail
- image push 0、Cloud Run / Cloud SQL / Job / Secret に副作用なし

## 根本原因

**runbook §1.2 のサンプルが過去の運用と乖離**していた。

| 観点 | runbook §1.2（誤）| 実運用（正、PR29 PR30 PR33b PR33c PR33d で実証）|
|---|---|---|
| `gcloud builds submit` の source | `/home/erenoa6621/dev/vrc_photobook/backend` | `/home/erenoa6621/dev/vrc_photobook`（**repo root**）|
| `--config` の値 | `/home/erenoa6621/dev/vrc_photobook/cloudbuild.yaml` | 同上（一致）|

`cloudbuild.yaml` step 0 は:

```yaml
- name: gcr.io/cloud-builders/docker
  id: build
  args:
    - build
    - -f
    - backend/Dockerfile
    - -t
    - asia-northeast1-docker.pkg.dev/$PROJECT_ID/vrcpb/vrcpb-api:$SHORT_SHA
    - backend
```

つまり **repo root 直下に `backend/` がある前提**で `-f backend/Dockerfile` + context `backend` を参照する。runbook §1.2 のように source を `backend/` に絞ると、Cloud Build workspace の root が `backend` ディレクトリの中身になり、`backend/Dockerfile` は見つからない。

`.gcloudignore`（PR29 commit `50f940c`）が `frontend/` `docs/` `harness/` 等を除外しているため、repo root submit でも upload は最小化される（実測 10 MiB / 304 files）。

## 影響範囲

- 直接影響: PR34b STOP β 初回 build fail（17 秒 / image push 0）
- 副作用範囲: なし（Cloud Run / Cloud SQL / Job / Secret 不変、既存 revision `vrcpb-api-00016-9ln` のまま）
- Secret 漏洩: 0 件（Cloud Build logs は `unable to prepare context` のみ）
- 復旧: 修正版コマンド（repo root submit）で再実行 → SUCCESS（build `a0f05816-a67a-48aa-b396-07da507ade1f` / 3M35S / `vrcpb-api:0db0d7c` / 新 revision `vrcpb-api-00017-hbg`）

## 同事象の過去発生

PR29 deploy automation 初回時に同じ事象が発生し、commit `50f940c` で `.gcloudignore` 追加 + repo root submit に切替えて解決済（`harness/work-logs/2026-04-28_backend-deploy-automation-result.md` §「修正 2: build context path の不一致」）。

しかし:
- runbook §1.2 のサンプル文字列だけが **PR29 解決前の記述のまま残置**されていた
- そのため PR34b 着手時の照合作業で「runbook 通り」と誤読し、初回 build fail を引き起こした

## 対策

### 1. runbook §1.2 を実運用に合わせて修正（本失敗ログ起票と同 commit）

- `gcloud builds submit /home/erenoa6621/dev/vrc_photobook/backend` →
  `gcloud builds submit /home/erenoa6621/dev/vrc_photobook`（repo root）に修正
- なぜ repo root から submit するのかの根拠（cloudbuild.yaml の `-f backend/Dockerfile` + context `backend` 前提、`.gcloudignore` で context 最小化）を runbook 本文にも明記
- §5「よくある失敗と対処」に新しい節（5.8 相当）を追加し、誤って backend/ を渡したときの症状と対処を記録

### 2. 運用ルールの再周知

- `.agents/rules/pr-closeout.md` で既に「runbook と実コマンドが不一致なら必ず停止」を運用しているが、**runbook 自体が古い場合がある**ことを再認識
- PR34b 計画書 §0 に従い「runbook と実コマンドの照合」STOP は今後も徹底（本失敗で実際に停止して照合した結果、再発を 1 回で抑え込めた）

### 3. 自動検出（任意）

- 将来的に `cloudbuild.yaml` の context 部と `docs/runbook/backend-deploy.md` のサンプルが整合しているかを CI / lint で検証する案
- 手作業では runbook update 漏れを防ぎきれない（PR29 で解決した修正が runbook §1.2 にだけ反映されなかった事実）
- ただし MVP では実装しない（lint コスト > 効果、本ルール運用で十分と判断）

## 関連

- [`.agents/rules/feedback-loop.md`](../../.agents/rules/feedback-loop.md) — 失敗 → ルール / runbook 修正の運用
- [`.agents/rules/pr-closeout.md`](../../.agents/rules/pr-closeout.md) — runbook と実コマンドの照合
- [`docs/runbook/backend-deploy.md`](../../docs/runbook/backend-deploy.md) §1.2 — 本 commit で修正
- [`harness/work-logs/2026-04-28_backend-deploy-automation-result.md`](../work-logs/2026-04-28_backend-deploy-automation-result.md) §「修正 2」 — PR29 で同事象が起きた記録
- [`harness/work-logs/2026-04-28_moderation-ops-result.md`](../work-logs/2026-04-28_moderation-ops-result.md) — PR34b 進行記録（初回失敗 → 修正版で再実行 SUCCESS の経緯）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-29 | 初版。PR34b STOP β 初回失敗を起点に、runbook §1.2 と実運用の乖離を明文化し、同 commit で runbook を修正 |
