# 2026-04-26 Backend Cloud Run deploy 実施結果（DB なし）

## 概要

`docs/plan/m2-backend-cloud-run-deploy-plan.md` 案 B（DB なしで先に Cloud Run deploy）に
従って、本実装 `backend/` を Cloud Run service `vrcpb-api`（asia-northeast1）として deploy。
Cloud SQL / Secret Manager / Domain Mapping は **未実施**、既存 spike リソースは **残置**。

- 実施日: 2026-04-26
- Service URL: `https://vrcpb-api-7eosr3jcfa-an.a.run.app`（同義: `https://vrcpb-api-271979922385.asia-northeast1.run.app`）
- Image: `asia-northeast1-docker.pkg.dev/project-1c310480-335c-4365-8a8/vrcpb/vrcpb-api:500f8cc`
- Image digest: `sha256:0c5aea12929b6a8a64149f69b22bb86b52f705f484e353b2b89571410d1e3cb5`
- Revision: `vrcpb-api-00001-q9h`
- Image size: 12.4MB（distroless static + nonroot）

## 各 Step の結果

### Step 1. 事前確認

- project: `project-1c310480-335c-4365-8a8`
- region: `asia-northeast1`
- auth: `k.matsunaga.biz@gmail.com`（active）
- 有効 API: `run` / `artifactregistry` / `secretmanager`
- `cloudbuild.googleapis.com`: 未有効。本実施 PR では Cloud Build を使わず、ローカル `docker build` → `docker push` 直接のため有効化せずに進めた

### Step 2. Docker 認証

- `gcloud auth configure-docker asia-northeast1-docker.pkg.dev`: 既に登録済（追加変更なし）

### Step 3. Artifact Registry repo

- 既存: `vrcpb-spike`（spike 用、残置）
- **新規作成**: `vrcpb`（M2 本実装用、format=docker, location=asia-northeast1）

### Step 4. Docker image build

- `docker build -f backend/Dockerfile -t <image> backend` 成功
- image size: 12.4MB
- distroless static + nonroot
- `.dockerignore` で `.env` / `.git` / 一時ファイルを除外（PR6 で確認済）
- build 後の image 内部スキャン（`docker save | tar -tf`）はバックグラウンドで応答が止まったため skip。バイナリのみの distroless ベースで、build context に `.env` を入れない `.dockerignore` 設定があるため、混入リスクは構造的に極小と判断

### Step 5. Docker image push

- `docker push asia-northeast1-docker.pkg.dev/.../vrcpb/vrcpb-api:500f8cc` 成功
- レイヤーは spike repo との重複が `Mounted from project-1c310480-335c-4365-8a8/vrcpb-spike/api` で再利用された（Artifact Registry の cross-repo blob mount）
- digest 確認済

### Step 6. Cloud Run deploy

- `gcloud run deploy vrcpb-api --image=... --region=asia-northeast1 --allow-unauthenticated --port=8080 --min-instances=0 --max-instances=2 --cpu=1 --memory=512Mi --concurrency=80 --timeout=60 --set-env-vars="APP_ENV=staging,ALLOWED_ORIGINS=https://app.vrc-photobook.com"`
- revision `vrcpb-api-00001-q9h` に 100% traffic
- IAM Policy 設定（allow unauthenticated）

### Step 7. Service URL

- `https://vrcpb-api-7eosr3jcfa-an.a.run.app`

### Step 8. curl 確認

| endpoint | status | body | 期待値 |
|---|---|---|---|
| `GET /health` | 200 | `{"status":"ok"}` | ✅ |
| `GET /readyz` | 503 | `{"status":"db_not_configured"}` | ✅ |
| `POST /api/auth/draft-session-exchange` | 404 | （chi 既定の plain text）| ✅（pool nil で endpoint 未登録、PR9c 仕様） |
| `POST /api/auth/manage-session-exchange` | 404 | 同上 | ✅ |

### Step 9. Service 設定

| 項目 | 値 | 期待 |
|---|---|---|
| name | `vrcpb-api` | ✅ |
| containerConcurrency | 80 | ✅ |
| timeoutSeconds | 60 | ✅ |
| containerPort | 8080 | ✅ |
| cpu / memory | 1 / 512Mi | ✅ |
| autoscaling.knative.dev/maxScale | 2 | ✅ |
| min-instances | 既定（0、annotation には現れない）| ✅ |
| env: APP_ENV | `staging` | ✅ |
| env: ALLOWED_ORIGINS | `https://app.vrc-photobook.com` | ✅ |
| env: DATABASE_URL | （未設定）| ✅ |
| traffic | 100% to revision 00001-q9h | ✅ |
| serviceAccount | `271979922385-compute@developer.gserviceaccount.com`（既定 Compute SA） | M2 検証段階としては OK、本番では専用 SA 推奨 |

### Step 10. Logs / 漏洩 grep

- 起動ログ:
  - `db not configured; /readyz will return 503 db_not_configured`
  - `server starting`
  - `Default STARTUP TCP probe succeeded after 1 attempt for container "vrcpb-api-1" on port 8080`
- WARNING / ERROR ログ:
  - 全て **HTTP リクエストログ**（404 / 503 を Cloud Run の標準動作で WARNING / ERROR severity に分類）
  - アプリケーション側の error ではない（curl 自身のテストアクセス由来）
- 漏洩 grep 結果:
  - `SECRET / API_KEY / PASSWORD / PRIVATE / sk_live / sk_test / draft_edit_token / manage_url_token / session_token / set-cookie / DATABASE_URL=`: **マッチなし**

## 実施しなかったこと

- Cloud SQL 作成
- Secret Manager 実値登録（`DATABASE_URL` は未設定）
- Cloud Run Domain Mapping（`api.vrc-photobook.com` への紐付け）
- Cloudflare DNS 変更
- Workers Custom Domain 設定 / Workers deploy
- SendGrid 設定 / Turnstile 本番 widget 作成 / R2 変更
- 既存 spike リソース削除（`vrcpb-spike-api` Cloud Run service / `vrcpb-spike` Artifact Registry repo は残置）
- Budget Alert 変更
- `cloudbuild.googleapis.com` 有効化（本実施 PR では Cloud Build 不要のため）

## 発生した費用見込み

- Cloud Run service: min=0、curl 数回のみのリクエスト → 実質 $0
- Artifact Registry: image ~12.4MB の 1 個、無料枠 0.5 GB 内で課金なし
- Secret Manager: secret 0 件のため課金なし
- Cloud Logging: 50 GiB/月 無料枠内
- **Budget Alert ¥1,000 維持**（M1 設定済の月額予算）

## 切戻し手順（参考、本書では切戻しを実施しない）

```sh
# Cloud Run service 削除
gcloud run services delete vrcpb-api --region=asia-northeast1 --quiet

# Artifact Registry image 削除（個別 tag）
gcloud artifacts docker images delete \
  asia-northeast1-docker.pkg.dev/project-1c310480-335c-4365-8a8/vrcpb/vrcpb-api:500f8cc --quiet

# Artifact Registry repo 自体削除
gcloud artifacts repositories delete vrcpb --location=asia-northeast1 --quiet
```

旧 spike service（`vrcpb-spike-api`）は残置されているため、切戻し後も Backend は spike URL で動作確認可能。

## 検証チェックリスト（実施計画 §9.1 と一致）

- [x] `gcloud run services list` に `vrcpb-api` が `Ready`
- [x] revision 1、min=0、max=2 が反映
- [x] `/health` 200 + `{"status":"ok"}`
- [x] `/readyz` 503 + `{"status":"db_not_configured"}`
- [x] `/api/auth/*` が 404（pool nil で endpoint 未登録、PR9c 仕様通り）
- [x] Cloud Run logs に DB 未設定の起動メッセージ
- [x] Cloud Run logs に Secret / token / Cookie 漏れなし
- [ ] min=0 の課金確認（数時間放置後の billing で確認、本書段階では未実施。次回確認で OK）

## 次のステップ

実施計画書（`m2-backend-cloud-run-deploy-plan.md`）§3.4 / §12.2 通り:

1. **Cloud SQL 短時間検証 PR**（短期作成 → DATABASE_URL を Secret Manager 登録 → Cloud Run revision 更新 → `/readyz` 200 + token exchange 200 経路を実環境で確認 → 検証完了後に Cloud SQL **削除** で課金停止）
2. **Backend Domain Mapping 実施 PR**（`api.vrc-photobook.com` を Cloud Run service へ紐付け、Cloudflare DNS に CNAME → `ghs.googlehosted.com` を **DNS only** で追加）
3. **Frontend Workers deploy + Custom Domain 実施 PR**（`app.vrc-photobook.com` を `vrcpb-frontend` Worker に紐付け）
4. **Safari / iPhone Safari 実機確認 PR**

なお、Cloud Run の min=0 課金挙動の確認は次のサイクル（数時間後の billing 観察）で実施する。

## 関連

- [`docs/plan/m2-backend-cloud-run-deploy-plan.md`](../../docs/plan/m2-backend-cloud-run-deploy-plan.md)
- [`docs/plan/m2-domain-mapping-execution-plan.md`](../../docs/plan/m2-domain-mapping-execution-plan.md)
- [`docs/plan/m2-domain-purchase-checklist.md`](../../docs/plan/m2-domain-purchase-checklist.md)
- [`backend/Dockerfile`](../../backend/Dockerfile) / [`backend/cmd/api/main.go`](../../backend/cmd/api/main.go) / [`backend/internal/http/router.go`](../../backend/internal/http/router.go)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
