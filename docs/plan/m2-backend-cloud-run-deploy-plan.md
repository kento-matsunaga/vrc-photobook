# M2 Backend Cloud Run Deploy 計画

> 作成日: 2026-04-26
> 位置付け: 本実装 `backend/` を Cloud Run service `vrcpb-api`（asia-northeast1）として
> deploy するための計画書。**Cloud Run deploy / Artifact Registry repo 作成 / Cloud SQL 作成 /
> Secret Manager 実値登録は本書の段階では一切実行しない**。
>
> 上流参照（必読、本書では再記載しない）:
> - [`docs/plan/m2-domain-mapping-execution-plan.md`](./m2-domain-mapping-execution-plan.md)（DNS / Domain Mapping 実施計画、§9 deploy 順序の起点）
> - [`docs/plan/m2-domain-purchase-checklist.md`](./m2-domain-purchase-checklist.md)（vrc-photobook.com 購入完了記録）
> - [`docs/plan/m2-implementation-bootstrap-plan.md`](./m2-implementation-bootstrap-plan.md)（M2 実装ブートストラップ）
> - [`docs/plan/m2-photobook-session-integration-plan.md`](./m2-photobook-session-integration-plan.md)
> - [`docs/adr/0001-tech-stack.md`](../adr/0001-tech-stack.md) / [`docs/adr/0002-ops-execution-model.md`](../adr/0002-ops-execution-model.md)
> - [`backend/Dockerfile`](../../backend/Dockerfile) / [`backend/docker-compose.yaml`](../../backend/docker-compose.yaml)
> - [`backend/internal/http/router.go`](../../backend/internal/http/router.go) / [`backend/cmd/api/main.go`](../../backend/cmd/api/main.go)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md) / [`safari-verification.md`](../../.agents/rules/safari-verification.md)
> - [Cloud Run mapping custom domains 公式](https://docs.cloud.google.com/run/docs/mapping-custom-domains)

---

## 0. 本計画書の使い方

- 本書は **計画書のみ**。Cloud Run / Artifact Registry / Secret Manager の実操作は **行わない**。
- §3 で DB 接続方針（DB なし vs Cloud SQL 即時作成）を決定 → §4-§7 で個別構成を確定 → §8 で deploy コマンド案を確認 → §12 のユーザー判断事項に答えてから実施 PR に進む。
- 実施 PR は本書の手順を 1 つずつ進めるたびに `gcloud` / `curl` / `docker` で **客観確認**（`wsl-shell-rules.md` §sudo / 検証準拠）。

---

## 1. 目的

- 本実装 `backend/` を **Cloud Run service `vrcpb-api`** として deploy する
- Domain Mapping 用に `api.vrc-photobook.com` で受けられる **service URL を確定**する（Domain Mapping は別 PR）
- `/health` / `/readyz` / `/api/auth/draft-session-exchange` / `/api/auth/manage-session-exchange` を **実環境で確認できる**状態にする
- 既存 `vrcpb-spike-api` は残し、本実装と並走させる（切戻しの参照点）

---

## 2. 今回決めること

| 項目 | 案 |
|---|---|
| Cloud Run service 名 | `vrcpb-api`（M2 本実装名） |
| Region | `asia-northeast1`（東京、Domain Mapping GA 対応） |
| Artifact Registry repo 名 | `vrcpb`（本実装用、§5 で詳細） |
| Docker image | `asia-northeast1-docker.pkg.dev/<project>/vrcpb/vrcpb-api:<tag>` |
| Service Account | 既定の Compute SA を使うか、専用 SA を作るか（§4） |
| Cloud Run env vars | `APP_ENV` / `ALLOWED_ORIGINS` /（DB ありなら `DATABASE_URL`、Secret 経由）|
| Secret Manager に入れる値 | `DATABASE_URL`（DB ありの段階で）/ 後続で SendGrid / Turnstile / R2 |
| DB 接続先 | §3 で決定（推奨: 案 B = DB なし開始）|
| min/max instances | min=0 / max=2 |
| allow unauthenticated | **許可**（公開 API、認可は token / Cookie で行う）|
| CORS / `ALLOWED_ORIGINS` | env として設定、middleware は本 PR では入れない（§7）|
| 既存 spike リソース | 残す（切戻し参照点）|

---

## 3. DB 接続方針（最重要）

### 3.1 案比較

| 案 | 内容 | 利点 | 欠点 |
|---|---|---|---|
| 案 A | **Cloud SQL PostgreSQL を本書段階で作成**、`vrcpb-api` から接続 | `/readyz` 200 / token exchange 200 経路まで実環境で確認可能 | Cloud SQL の費用（最小 db-f1-micro でも〜$10/月、停止しても課金あり）。後片付け / バックアップ / Secret Manager 設定が一気に増える |
| 案 B | **Cloud SQL は作らず**、`DATABASE_URL` 未設定で Cloud Run deploy。`/health` 200 / `/readyz` 503 db_not_configured を確認 | Cloud Run deploy の正常性だけ低コストで確認できる。Cloud SQL 課金が発生しない。Budget Alert ¥1,000 維持 | token exchange 200 経路は実環境で確認できない（PR9c の handler test と PR10.5 の Vitest で確認済を信じる）。`/api/auth/*` endpoint は **router 側で生やさない**（pool nil で nil 渡しのため、§9.1）|
| 案 C | 一時 PostgreSQL を Compute Engine / Cloud SQL Auth Proxy / VM 上で立てる | Cloud SQL 課金を抑えつつ DB ありで動く | セットアップ・ネットワーク（Cloud Run → VM の private 接続）が複雑。短期検証には合わない |

### 3.2 推奨: **案 B（DB なしで Cloud Run deploy）**

理由（ユーザー希望と整合）:

- いきなり Cloud SQL 常時起動は避けたい
- Budget Alert ¥1,000 維持
- まず低コストで Cloud Run deploy だけ確認したい
- token exchange 200 経路は **PR9c の実 DB handler test 13 件 + PR10.5 の Vitest 16 件** で確認済（Backend ロジック自体の正常性は担保済）
- Cloud Run の deploy 自体（image build / Artifact Registry / Secret Manager / env vars / `/health` / `/readyz`）に **deploy パイプラインの問題が無いこと**を先に確認するのが事故の少ない順序
- Cloud SQL 接続は次の段階（短時間検証）で別 PR として独立させる

### 3.3 案 B での挙動の確認

`backend/cmd/api/main.go` + `backend/internal/http/router.go` の現状（PR9c）:

- `DATABASE_URL` 空 → `pool == nil` → **`photobookHandlers` も nil**
- `NewRouter(pool, photobookHandlers)` で `photobookHandlers == nil` のとき、token exchange endpoint は **生やさない**（router.go 26-28 行）
- `/health` は常に登録 → 200 を返す
- `/readyz` は `pool == nil` → 503 `{"status":"db_not_configured"}`
- **`/api/auth/*` は 404**（endpoint 自体が無い、PR9c 仕様）

→ 案 B でも Cloud Run の `/health` / `/readyz` / 404 動作で deploy パイプラインの正常性は完全に確認できる。

### 3.4 後続段階（別 PR）

案 B で deploy 成功 → 確認完了したら、別 PR で:

1. **Cloud SQL PostgreSQL を作成**（最小スペック、検証期間中のみ起動）
2. **Secret Manager に `DATABASE_URL` 登録**
3. **Cloud Run revision 更新** で env / Secret を反映
4. **`/readyz` 200** + **token exchange 200 経路** を実環境で確認
5. 検証完了後 Cloud SQL を **停止 or 削除**（Budget Alert 維持）

→ 本書では Cloud SQL は範囲外、別 PR `m2-backend-cloud-sql-and-db-verification-plan.md`（仮）で扱う。

---

## 4. Cloud Run service 設計

### 4.1 設計値（本書での確定提案）

| 項目 | 値 | 根拠 |
|---|---|---|
| service 名 | `vrcpb-api` | M2 本実装名、計画と整合 |
| Region | `asia-northeast1` | 既存 spike と同、Domain Mapping GA |
| Image | `asia-northeast1-docker.pkg.dev/<project>/vrcpb/vrcpb-api:<tag>` | §5 で repo 命名 |
| Port | `8080` | Dockerfile / main.go 既定値 |
| min-instances | `0` | コスト優先、コールドスタート許容 |
| max-instances | `2` | M2 検証期間は 1〜2 で十分、暴走時の課金抑止 |
| CPU | `1` (= 1 vCPU) | Cloud Run 既定、最小コスト |
| Memory | `512Mi` | Go バイナリ + pgx pool で 512Mi で十分（M1 spike 同等） |
| Concurrency | `80` | Cloud Run 既定（Go の HTTP server で十分捌ける） |
| Timeout | `60s` | API は短時間、画像アップロードは presigned URL で R2 直送なので Cloud Run には掛からない |
| Ingress | `all`（パブリック）| 認可は token / Cookie で行う |
| Allow unauthenticated | **許可** | 公開 API、認証は内部の token / session で実施 |
| Service Account | 既定（Compute SA）| M2 検証段階。本番では専用 SA + IAM 限定（後続 PR で扱う） |
| Execution environment | First gen（既定） | OpenNext と違い特に第二世代は不要 |
| HTTP/2 end-to-end | 無効（既定） | 不要 |

### 4.2 env vars（DB なし版、案 B）

| キー | 値 | 種別 |
|---|---|---|
| `APP_ENV` | `staging` | 公開 |
| `PORT` | （未設定、Cloud Run が自動注入）| 自動 |
| `ALLOWED_ORIGINS` | `https://app.vrc-photobook.com` | 公開（CORS middleware 未実装でも env として用意しておく）|
| `DATABASE_URL` | **未設定** | （DB ありの段階で Secret Manager 経由）|

### 4.3 env vars（DB ありに切替え時の追加）

| キー | 値 | 種別 |
|---|---|---|
| `DATABASE_URL` | Cloud SQL DSN（`postgres://...`）| **Secret Manager 経由**で注入 |

### 4.4 起動 / 終了

- 起動: Cloud Run が SIGINT/SIGTERM 直後 10 秒 grace period（PR2 で実装した graceful shutdown と整合）
- ヘルスチェック: Cloud Run 既定の startup / liveness probe は `/` に対して TCP チェック。HTTP probe にする場合は revision 設定で `/health` を指定可能（M2 本実装で必要なら追加、PR9c 段階では不要）

---

## 5. Artifact Registry 方針

### 5.1 案比較

| 案 | 内容 | 利点 | 欠点 |
|---|---|---|---|
| 案 A | 既存 `vrcpb-spike` repo を再利用 | 1 つで済む | 名前が spike なので本実装の image と混在、後で分離が困難 |
| 案 B | **本実装用 `vrcpb` repo を新規作成**、image は `vrcpb/vrcpb-api` | 本実装と spike を物理的に分離、命名が一貫 | 新規 repo の作成 + IAM 設定の手間 |

### 5.2 推奨: **案 B（新規 `vrcpb` repo）**

理由:
- spike と本実装は別物（PoC 流用しない方針との整合）
- image push 時にどちらに入れるか迷わない
- 後片付けで spike repo を丸ごと削除しても本実装に影響しない

### 5.3 設計値

| 項目 | 値 |
|---|---|
| Repository name | `vrcpb` |
| Format | `docker` |
| Mode | `Standard` |
| Region | `asia-northeast1` |
| Description | `M2 production images for vrc-photobook.com` |
| Image: API | `asia-northeast1-docker.pkg.dev/<project>/vrcpb/vrcpb-api` |
| Image: Frontend (将来) | `asia-northeast1-docker.pkg.dev/<project>/vrcpb/vrcpb-frontend`（Workers なので image 不要、参考） |
| 後続 outbox-worker | `asia-northeast1-docker.pkg.dev/<project>/vrcpb/vrcpb-api`（同一 image、`--command` で切替）|

### 5.4 image tag 戦略

| tag | 用途 |
|---|---|
| `<git-sha>` | 不変、本番 deploy で常用 |
| `staging` | 最新 staging（mutable）|
| `latest` | 使わない（mutable で混乱の元）|

→ deploy は `<git-sha>` を指定、必要なら `staging` を更新。`latest` は使わない。

### 5.5 費用

- Artifact Registry: 0.5 GB/月まで無料、超過分 $0.10/GB/月
- Go バイナリ + distroless で image は ~25 MB。10 revision でも 250 MB、無料枠内
- 古い image の削除は別 PR で扱う（Cleanup policy 設定）

---

## 6. Secret Manager 方針

### 6.1 今すぐ必要

| Secret | 値 | 必要なタイミング |
|---|---|---|
| `DATABASE_URL` | Cloud SQL DSN | DB ありに切替する別 PR で登録 |

### 6.2 今は不要（後続 PR）

- `SENDGRID_API_KEY`（M2 後期、メール送信実装時）
- `TURNSTILE_SECRET_KEY`（Image upload PR）
- `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY`（同上）

### 6.3 Secret Manager に入れない値（公開 env で OK）

- `APP_ENV` / `ALLOWED_ORIGINS` / `PORT` / `NEXT_PUBLIC_*` / `COOKIE_DOMAIN`

### 6.4 注意

- Secret 値を **チャット / ログ / コミットメッセージ / failure-log に貼らない**（`security-guard.md`）
- Secret Manager 登録は **実施 PR で別操作**として扱う（本書では値を持たない）
- Cloud Run の Service Account に `roles/secretmanager.secretAccessor` を付与（実施 PR）
- env vars に Secret を渡す方法は 2 通り:
  - (a) Cloud Run の `--update-secrets` で Secret Manager から直接 mount
  - (b) Cloud Run の `--set-env-vars` で平文（NG）
  - **(a) を採用**（実施 PR で）

---

## 7. CORS 方針

### 7.1 現状（PR9c 時点）

- Backend には **CORS middleware が存在しない**
- chi router に登録されているのは `/health` / `/readyz` + 必要なら token exchange の 2 endpoint だけ
- preflight (`OPTIONS`) リクエストへの応答は chi の既定（404 / 405）

### 7.2 deploy 時に CORS が必要か

token exchange の流れを再確認:

```
ブラウザ → app.vrc-photobook.com/draft/[token] (Frontend Workers)
        → Frontend Route Handler が Backend `api.vrc-photobook.com/api/auth/*` を fetch
        → Backend が JSON で raw session_token 返却
        → Frontend Route Handler が Set-Cookie + 302 redirect
        → ブラウザ /edit/<id> へ遷移
```

- Backend を呼ぶのは **Frontend Route Handler（Server-side fetch）**、ブラウザではない
- Server-side fetch は **CORS の対象外**（CORS はブラウザの Same-Origin 制約）
- ブラウザから直接 `api.vrc-photobook.com` を叩く経路は本書段階では無い
- → **deploy 時点では CORS middleware 不要**

### 7.3 将来の必要性

以下の段階で CORS が必要になる:

- `/edit/<id>` / `/manage/<id>` の Server Component から Backend `/api/photobooks/{id}` を呼ぶ場合 → これも Server-side なので CORS 不要
- Client Component （`'use client'`）から `fetch('https://api.vrc-photobook.com/api/...', {credentials: 'include'})` を呼ぶ場合 → **CORS 必要**
- 例: 画像アップロードの状態ポーリング、楽観ロック衝突時の再取得 等

### 7.4 推奨: **本 deploy では CORS middleware を入れない、PR11 以降で Client Component が必要になったら追加**

理由:
- 現状の token exchange + Server Component 経由は CORS 不要で動く
- 不要な middleware を先に入れると debug 時の可動範囲が広がりすぎる
- middleware 追加は別 PR で test も含めて入れる方が安全

ただし `ALLOWED_ORIGINS=https://app.vrc-photobook.com` env は **deploy 時から設定**（middleware 接続時にすぐ使えるように）。

---

## 8. Deploy 手順案（実コマンド、まだ実行しない）

### 8.1 事前準備

```sh
# project / region 確認
gcloud config get-value project
gcloud config get-value run/region
# 期待: project-1c310480-335c-4365-8a8 / asia-northeast1（必要なら set）

# 必要 API 有効化
gcloud services enable run.googleapis.com artifactregistry.googleapis.com secretmanager.googleapis.com cloudbuild.googleapis.com

# 認証確認
gcloud auth list
gcloud auth configure-docker asia-northeast1-docker.pkg.dev
```

### 8.2 Artifact Registry 作成

```sh
gcloud artifacts repositories create vrcpb \
  --repository-format=docker \
  --location=asia-northeast1 \
  --description="M2 production images for vrc-photobook.com"
```

### 8.3 docker build / push

```sh
# repo root から（cd は使わない、wsl-shell-rules.md 準拠）
GIT_SHA=$(git rev-parse --short HEAD)
IMAGE="asia-northeast1-docker.pkg.dev/$(gcloud config get-value project)/vrcpb/vrcpb-api:${GIT_SHA}"

docker build \
  -f backend/Dockerfile \
  -t "${IMAGE}" \
  backend

docker push "${IMAGE}"
```

### 8.4 Cloud Run deploy（DB なし、案 B）

```sh
gcloud run deploy vrcpb-api \
  --image="${IMAGE}" \
  --region=asia-northeast1 \
  --platform=managed \
  --allow-unauthenticated \
  --port=8080 \
  --min-instances=0 \
  --max-instances=2 \
  --cpu=1 \
  --memory=512Mi \
  --concurrency=80 \
  --timeout=60 \
  --set-env-vars="APP_ENV=staging,ALLOWED_ORIGINS=https://app.vrc-photobook.com" \
  --no-cpu-throttling=false
```

### 8.5 service URL 確認

```sh
gcloud run services describe vrcpb-api \
  --region=asia-northeast1 \
  --format='value(status.url)'
# 期待: https://vrcpb-api-<hash>-an.a.run.app
```

### 8.6 動作確認 curl

```sh
URL=$(gcloud run services describe vrcpb-api --region=asia-northeast1 --format='value(status.url)')

# /health
curl -sI "${URL}/health"
# 期待: HTTP/2 200, Content-Type: application/json

curl -sS "${URL}/health"
# 期待: {"status":"ok"}

# /readyz（DB なしのため 503 db_not_configured）
curl -sI "${URL}/readyz"
curl -sS "${URL}/readyz"
# 期待: 503, {"status":"db_not_configured"}

# token exchange は pool nil で endpoint 自体が登録されていない（PR9c 仕様）
curl -sI -X POST "${URL}/api/auth/draft-session-exchange"
# 期待: 404 Not Found（chi router の既定動作）
```

### 8.7 logs 確認 + Secret 漏洩 grep

```sh
# 直近 100 行
gcloud run services logs read vrcpb-api \
  --region=asia-northeast1 \
  --limit=100

# Secret 漏洩確認（Cloud Logging に raw token / DSN / API_KEY / Cookie が出ていないこと）
gcloud run services logs read vrcpb-api \
  --region=asia-northeast1 \
  --limit=500 | \
  grep -iE "(SECRET|API_KEY|PASSWORD|PRIVATE|sk_live|sk_test|draft_edit_token|manage_url_token|session_token|set-cookie|DATABASE_URL=)"
# 期待: マッチなし
```

---

## 9. 検証項目

### 9.1 DB なし版（案 B、本書で進める想定）

- [ ] `gcloud run services list` に `vrcpb-api` が `Ready` で出る
- [ ] revision 数 1、min-instances=0、max-instances=2 が反映
- [ ] `/health` が 200 + `{"status":"ok"}`
- [ ] `/readyz` が **503** + `{"status":"db_not_configured"}`
- [ ] `/api/auth/draft-session-exchange` が **404**（pool nil で endpoint 未登録、PR9c 仕様）
- [ ] Cloud Run logs に `db not configured; /readyz will return 503 db_not_configured` の起動メッセージ
- [ ] Cloud Run logs に **Secret / token / Cookie が漏れていない**
- [ ] Cloud Run の課金が min=0 で受信時のみであること（数時間放置で課金確認）

### 9.2 DB あり版（別 PR で扱う、参考）

- [ ] Cloud SQL PostgreSQL の Private IP / Cloud SQL Auth Proxy で Cloud Run から接続
- [ ] `/readyz` が 200 + `{"status":"ready"}`
- [ ] `/api/auth/draft-session-exchange` が 400（空 body）/ 401（不正 token）/ 200（実 token）
- [ ] response に `Cache-Control: no-store`、`Set-Cookie` が **出ない**
- [ ] 実 token で 200 返却、body に raw `session_token`（仕様通り、ログには出ない）

---

## 10. 費用見積もり

### 10.1 案 B（DB なし）の月額目安

| 項目 | 費用 | 備考 |
|---|---|---|
| Cloud Run (min=0, ~100 req/月) | $0 | リクエスト 0 で課金ゼロ、200ms × ~100 で無料枠内 |
| Artifact Registry | $0 | 0.5 GB 無料枠内、image ~25 MB × 数 revision |
| Secret Manager | $0 | 6 secrets まで無料、本書段階では 0 secret |
| Cloud Logging | $0 | 50 GiB/月 無料枠、検証期間で十分 |
| **合計** | **$0** | |

### 10.2 Cloud SQL を作る場合（別 PR、参考）

- db-f1-micro: ~$10/月（最小スペック、停止しても課金あり、削除のみで停止）
- db-g1-small: ~$25/月
- 検証期間中のみ起動、終わったら **削除**（停止だけだと課金継続）

### 10.3 Budget Alert ¥1,000 維持

- 案 B では Budget Alert ¥1,000 を **十分維持できる**（実質 $0）
- 案 A（Cloud SQL）でも $10/月 = ¥1,500 程度で 1 ヶ月分超過の可能性 → Budget Alert 増額か Cloud SQL 短期検証で対応

### 10.4 検証完了後の削除候補

- Cloud Run service の revision 古いもの（gcloud で `--delete-old-revisions`）
- Artifact Registry の古い tag（cleanup policy で自動）
- Cloud Logging のログ（自動 30 日 retention で十分）
- 既存 spike service（本書段階では残す、別 PR で削除判断）

---

## 11. 失敗時切戻し

### 11.1 Cloud Run service 削除

```sh
gcloud run services delete vrcpb-api --region=asia-northeast1 --quiet
```

### 11.2 Artifact Registry image 削除

```sh
# 個別 tag
gcloud artifacts docker images delete \
  asia-northeast1-docker.pkg.dev/<project>/vrcpb/vrcpb-api:<tag> --quiet

# repo 自体を消す
gcloud artifacts repositories delete vrcpb --location=asia-northeast1 --quiet
```

### 11.3 Secret 削除（DB ありで作成済の場合のみ）

```sh
gcloud secrets delete DATABASE_URL --quiet
```

### 11.4 env / config 戻し

- Frontend `.env.production` の `NEXT_PUBLIC_API_BASE_URL` を旧 `https://vrcpb-spike-api-7eosr3jcfa-an.a.run.app` に戻す
- ただし本書段階で Frontend は本番 deploy していないため、影響範囲は本実装内のみ

### 11.5 旧 spike service 維持

- `vrcpb-spike-api` は本書範囲では **削除しない**
- 本実装 deploy 失敗時は spike URL で動作確認できる
- spike 削除は M2 後期の別 PR で扱う

### 11.6 Domain Mapping は影響なし

- 本書段階では Domain Mapping を作成しないため、DNS / `api.vrc-photobook.com` には影響なし
- 切戻し後は単に「Cloud Run service が消える」だけで、外部に何も公開していない状態に戻る

### 11.7 切戻しトリガー

- `gcloud run deploy` が異常終了して service が `Failed` 状態
- `/health` が 200 を返さない（image build 不備、port 不一致 等）
- Cloud Run logs に **Secret / token / Cookie 漏洩**を発見
- 課金が想定外（min=0 のはずなのに idle 課金が発生）

---

## 12. ユーザー判断事項

実施 PR 着手前に以下を確認してください。

### 12.1 DB 接続方針

- [ ] **案 B（DB なしで先に Cloud Run deploy）**（推奨、§3.2、ユーザー希望と整合）
- [ ] 案 A（Cloud SQL を本書段階で作成）
- [ ] 案 C（一時 PostgreSQL 等）

### 12.2 Cloud SQL を後続でどう作るか

- [ ] 別 PR で **短時間検証用に作成 → 検証完了後に削除**（推奨）
- [ ] M2 後期まで Cloud SQL は作らず、ローカル検証で押さえる（推奨度低、deploy 後の token exchange 200 経路が実環境で確認できないリスク）

### 12.3 Artifact Registry repo

- [ ] **新規 `vrcpb` repo 作成**（推奨、§5.2）
- [ ] 既存 `vrcpb-spike` repo を再利用

### 12.4 Cloud Run service 名

- [ ] **`vrcpb-api`**（推奨、本実装名と整合）
- [ ] 別名

### 12.5 allow unauthenticated

- [ ] **許可**（推奨、API 公開、認可は token / Cookie で）
- [ ] IAM 制限（社内検証等）

### 12.6 CORS middleware

- [ ] **本 deploy では入れない、PR11 以降で必要時に追加**（推奨、§7.4）
- [ ] 本 deploy 前に追加する別 PR を先に作る

### 12.7 既存 spike リソース

- [ ] **残す**（推奨、切戻し参照点）
- [ ] 本 deploy 成功後に削除する PR を別途作る

### 12.8 実施 PR の進め方

- [ ] **本書承認 → 実施 PR で §8.1〜§8.7 を 1 ステップずつ実行 + 各ステップで gcloud / curl 確認**（推奨）
- [ ] 一括スクリプト化して実行（本書では推奨しない、検証粒度が粗くなる）

---

## 13. 実施しないこと（再掲）

本書は **計画書のみ**。以下は **実行しない**:

- Artifact Registry repo 作成
- docker build / push
- Cloud Run service 作成 / deploy
- Cloud SQL 作成 / 接続テスト
- Secret Manager 実値登録
- Cloud Run Domain Mapping
- Cloudflare DNS 変更
- Workers Custom Domain 設定 / Workers deploy
- SendGrid 設定 / Turnstile 本番 widget 作成 / R2 変更
- 既存 spike リソース削除
- Budget Alert 変更

---

## 14. 関連ドキュメント

- [M2 Domain Mapping 実施計画](./m2-domain-mapping-execution-plan.md) §9 deploy 順序
- [M2 ドメイン購入チェックリスト + 購入記録](./m2-domain-purchase-checklist.md)
- [M2 実装ブートストラップ計画](./m2-implementation-bootstrap-plan.md)
- [M2 Photobook session 接続計画](./m2-photobook-session-integration-plan.md)
- [プロジェクト全体ロードマップ](../../harness/work-logs/2026-04-26_project-roadmap-overview.md)
- [`backend/Dockerfile`](../../backend/Dockerfile) / [`backend/cmd/api/main.go`](../../backend/cmd/api/main.go) / [`backend/internal/http/router.go`](../../backend/internal/http/router.go)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md) / [`safari-verification.md`](../../.agents/rules/safari-verification.md)
- [Cloud Run 公式](https://docs.cloud.google.com/run/) / [Artifact Registry 公式](https://docs.cloud.google.com/artifact-registry/)
