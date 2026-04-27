# Backend deploy 自動化（Cloud Build）実装計画（PR29 計画書）

> 作成日: 2026-04-28
> 位置付け: 新正典 [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md)
> §3 PR29 の本体。本書では計画のみ確定し、実装は次の PR で行う。
>
> 上流参照（必読）:
> - [新正典ロードマップ](./vrc-photobook-final-roadmap.md)
> - [PR28 publish flow 結果](../../harness/work-logs/2026-04-27_publish-flow-result.md)
> - [PR23 image-processor 結果](../../harness/work-logs/2026-04-27_image-processor-result.md)
> - [`backend/Dockerfile`](../../backend/Dockerfile)（distroless static / nonroot / api + image-processor 同梱）
> - [`.github/workflows/backend-ci.yml`](../../.github/workflows/backend-ci.yml)（vet / build / test、deploy はしない）
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
> - [`.agents/rules/wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)

---

## 0. PR29 と次 PR の流れ

| PR | 内容 |
|---|---|
| **PR29（本書）** | Cloud Build 自動 deploy の計画書のみ（実装しない、API 有効化もしない） |
| 次の実装 PR | `cloudbuild.yaml` 追加 / IAM / Cloud Build trigger 作成 / 初回 deploy / rollback 確認 |

> **PR28 残課題**: 「実画像を含む完全 visual Safari 確認」は本 PR とは独立した manual
> 作業として残す。詳細は §13 を参照。

---

## 1. 目的

- Backend の **build / push / Cloud Run revision update** を Cloud Build で自動化する
- 手動 `docker build` / `docker push` / `gcloud run services update --image` を撤廃し、
  人為ミス / cwd drift / 認証漏れを防ぐ
- **Secret 値を Cloud Build に直接渡さない**（現在の Cloud Run env の secretKeyRef
  方式を維持）
- rollback 手順を明確化し、本 PR では実 API 有効化 / trigger 作成は行わず、
  停止ポイントを置く
- 課金・権限の影響を事前に整理する

---

## 2. PR29 対象範囲

### 対象（本書で確定する）

- `cloudbuild.yaml` 設計（手動レビュー前提）
- Cloud Build trigger の方式（main push / tag / manual）と推奨案
- IAM service account 設計（Cloud Build SA に必要な最小権限）
- Artifact Registry tag 設計（`$SHORT_SHA` ベース、`latest` の扱い）
- Cloud Run deploy の更新方式（`--image=` 単独更新 / env / secretKeyRef 維持）
- rollback 設計（revision pin / `update-traffic`）
- 既存 GitHub Actions（`backend-ci.yml`）との役割分担
- 停止ポイント（Cloud Build API 有効化前 / IAM 付与前 / trigger 作成前 / 初回 build / 切替前 / rollback 確認前）
- 課金影響の評価
- security 要件

### 対象外（本書で決めない / 触らない）

- 実装本体（次の PR）
- **Cloud Build API の有効化（`cloudbuild.googleapis.com`）**
- **Cloud Build trigger の実作成**
- **Cloud Build SA の実作成 / IAM 付与**
- 本番 deploy 実行
- Frontend Workers deploy 自動化（PR41+ で評価）
- Cloud Run Jobs / Scheduler（PR31）
- SendGrid / Outbox / OGP / Moderation / Report / UsageLimit
- Cloud SQL 本番化（PR39）/ spike 削除（PR40）/ Public repo 化（PR38）

---

## 3. 現状整理

### 3.1 手動 deploy の現行手順（PR23 / PR25b / PR27 / PR28 で使用）

```bash
# 1) image build（commit hash で tag）
IMAGE=asia-northeast1-docker.pkg.dev/$PROJ/vrcpb/vrcpb-api:$(git rev-parse --short=7 HEAD)
docker build -f backend/Dockerfile -t "$IMAGE" backend

# 2) AR push
docker push "$IMAGE"

# 3) Cloud Run revision 更新（env / secretKeyRef は --image= で不変）
gcloud run services update vrcpb-api --image="$IMAGE" --region=asia-northeast1 --project=$PROJ

# 4) smoke
curl -sS https://api.vrc-photobook.com/health
curl -sS https://api.vrc-photobook.com/readyz
```

### 3.2 現状リソース

| 項目 | 値 |
|---|---|
| Cloud Run service | `vrcpb-api`（asia-northeast1） |
| 現 revision | `vrcpb-api-00010-7vz` |
| 現 image | `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb/vrcpb-api:3ec5080` |
| rollback revision | `vrcpb-api-00009-wdb` （image `vrcpb-api:2a93f8c`） |
| Artifact Registry repo | `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb`（DOCKER、~50MB、image 9 件） |
| Cloud Run env / secretKeyRef | DATABASE_URL / R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET_NAME / R2_ENDPOINT / TURNSTILE_SECRET_KEY（plain: APP_ENV / ALLOWED_ORIGINS） |
| Cloud Build API | **未有効**（`cloudbuild.googleapis.com` 無効） |
| Cloud Build SA | **未作成**（default compute SA のみ） |
| Cloud Logging | enabled（Cloud Run 経由で出力中） |
| GitHub Actions | `backend-ci.yml` で vet / build / test 実行（push / PR 両方）。**deploy はしない** |
| Image tag ルール | `git rev-parse --short=7 HEAD`（7 文字 SHA） |

### 3.3 課題

- 手動 `docker build` / `docker push` / `gcloud run services update` の手順が長い
- cwd drift で Bash hook が壊れたケースが過去にあった（`harness/failure-log/`）
- rollback 手順が work-log の毎回記載に依存している（runbook 化されていない）

---

## 4. Cloud Build 設計

### 4.1 cloudbuild.yaml 案

```yaml
# ※ 実ファイルは次の実装 PR で追加。本書では設計案のみ。
steps:
  # 1) docker build（Dockerfile は backend 配下、context も backend）
  - name: gcr.io/cloud-builders/docker
    id: build
    args:
      - build
      - -f
      - backend/Dockerfile
      - -t
      - asia-northeast1-docker.pkg.dev/$PROJECT_ID/vrcpb/vrcpb-api:$SHORT_SHA
      - backend

  # 2) AR push
  - name: gcr.io/cloud-builders/docker
    id: push
    args:
      - push
      - asia-northeast1-docker.pkg.dev/$PROJECT_ID/vrcpb/vrcpb-api:$SHORT_SHA

  # 3) Cloud Run revision 更新（env / secretKeyRef は不変）
  - name: gcr.io/google.com/cloudsdktool/cloud-sdk
    id: deploy
    entrypoint: gcloud
    args:
      - run
      - services
      - update
      - vrcpb-api
      - --image=asia-northeast1-docker.pkg.dev/$PROJECT_ID/vrcpb/vrcpb-api:$SHORT_SHA
      - --region=asia-northeast1
      - --project=$PROJECT_ID

  # 4) smoke（gcloud + curl 等で /health と /readyz を叩く）
  - name: gcr.io/google.com/cloudsdktool/cloud-sdk
    id: smoke
    entrypoint: bash
    args:
      - -lc
      - |
        URL="https://api.vrc-photobook.com"
        for path in /health /readyz; do
          code=$(curl -sS -o /dev/null -w "%{http_code}" "$URL$path")
          if [ "$code" != "200" ]; then
            echo "smoke failed: $path -> $code"
            exit 1
          fi
        done

# image を AR の build artifact として記録
images:
  - asia-northeast1-docker.pkg.dev/$PROJECT_ID/vrcpb/vrcpb-api:$SHORT_SHA

# Cloud Build 実行 logs は Cloud Logging のみ
options:
  logging: CLOUD_LOGGING_ONLY
  machineType: E2_HIGHCPU_8

timeout: 1200s  # 20 分
```

### 4.2 tag ルール

- **`$SHORT_SHA`**（Cloud Build 提供の 7 文字 SHA）を主 tag とする
- **`latest` は使わない**（rollback 時に「どの commit が現役か」が曖昧になるため）
- 過去 image は AR の retention policy（後段、PR40 で評価）で世代管理
- 手動 deploy 期と命名互換: `git rev-parse --short=7 HEAD` と同じ

### 4.3 失敗時の扱い

- どのステップで失敗しても **新 revision は traffic 100% を取らない**
  （`gcloud run services update --image=` は新 revision 作成 → traffic 切替を一連で行うため、
  deploy step が失敗すると traffic は前 revision のまま残る）
- ただし**新 revision が作成されてから smoke で fail する**場合、新 revision に traffic
  が一部行く可能性 → §7 rollback で `update-traffic` で旧 revision を 100% に戻す

### 4.4 Secret に関する制約（厳守）

- **`DATABASE_URL` / `R2_*` / `TURNSTILE_SECRET_KEY` を cloudbuild.yaml に書かない**
- `--update-env-vars` / `--update-secrets` を Cloud Build から呼ばない
  （Cloud Run 既存の env / secretKeyRef を維持する `--image=` 単独更新で完結）
- Cloud Build substitutions に Secret を入れない
- docker build context（`backend/`）に `.env` / `.env.local` を含めない（既に `.gitignore` 済）
- image 内に Secret を埋め込まない（distroless static + CGO_ENABLED=0 維持、
  外部から secret を引っ張る経路はランタイム env のみ）

---

## 5. IAM 設計

### 5.1 専用 Cloud Build SA を作る

**default compute SA（`<PROJ_NUM>-compute@developer.gserviceaccount.com`）を使い回さない**。
理由: 過剰権限、追跡しづらい、ローテーションしづらい。

新設: **`vrcpb-cloud-build@<PROJ>.iam.gserviceaccount.com`**

### 5.2 必要な role（最小権限）

| role | 必要性 |
|---|---|
| `roles/artifactregistry.writer` | AR push |
| `roles/run.developer` | Cloud Run service update |
| `roles/iam.serviceAccountUser`（Cloud Run runtime SA に対して） | revision 作成時に runtime SA を assume |
| `roles/logging.logWriter` | Cloud Build → Cloud Logging |
| `roles/cloudbuild.builds.builder` | Cloud Build 実行 |

### 5.3 与えない role

- `roles/secretmanager.secretAccessor` — Secret は **runtime（Cloud Run service の SA）が読む**ため、
  Cloud Build SA には不要
- `roles/cloudsql.client` — DB 接続も runtime のみ
- `roles/owner` / `roles/editor` — 過剰

### 5.4 Cloud Run runtime SA

- 現在は default compute SA が runtime に紐付いている（推測、`gcloud run services describe`
  で要確認）
- ベストプラクティスは Cloud Run 専用 SA だが、本 PR の範囲外（PR39 本番運用整備で再設計）
- 本 PR では **既存 runtime SA を変更しない**

---

## 6. Trigger 設計

### 6.1 候補比較

| 案 | 利点 | 欠点 |
|---|---|---|
| A: main push で自動 deploy | hands-free | 事故リスク高（merge 直後に本番反映、smoke 失敗時の影響範囲広） |
| B: tag push で deploy（`v*` / `release-*`） | 意図的、rollback 容易 | tag 運用ルールが必要 |
| **C: manual trigger のみ**（推奨） | 最も安全、初期段階の事故防止 | 完全自動化ではない |
| D: GitHub Actions → Cloud Build 呼び出し | GH Actions 一本化 | OIDC / WIF 設定が増える |

### 6.2 推奨: **案 C（manual trigger）+ 後続段階で B に拡張**

- **PR29 後の初期実装**: manual trigger のみ
  - GCP Console / `gcloud builds triggers run` で都度起動
  - 「何を deploy したか」が明示的になる
- **後続段階（M2 中盤〜後半）**: tag trigger に拡張
  - `release-*` tag push で発動
  - tag 運用ルールを runbook に追加
- **本格自動化（M2 完了後 / PR41+）**: main push trigger を検討
  - その頃には e2e test / smoke が充実している前提

### 6.3 GitHub 連携

- Cloud Build の GitHub App 連携は **不要**（manual trigger なら）
- 後続段階で tag trigger に移行する際に GitHub App を接続する判断
- 既存 `backend-ci.yml` は **vet / build / test 担当**として継続。deploy は Cloud Build 側に分離

---

## 7. Rollback 設計

### 7.1 戦略

Cloud Run の **revision pin + traffic split** を使う。

- 各 deploy で生成された revision は AR の image と紐づいて保持
- Cloud Run は過去 revision を保持（traffic 0% でも残る）
- rollback = `gcloud run services update-traffic --to-revisions=<old>=100`

### 7.2 rollback 手順（runbook 案）

```bash
# 直前 revision を確認
gcloud run revisions list \
  --service=vrcpb-api --region=asia-northeast1 --project=$PROJ \
  --format='value(metadata.name,spec.containers[0].image,status.conditions[0].lastTransitionTime)' \
  --limit=5

# 直前 revision に 100% traffic を戻す
gcloud run services update-traffic vrcpb-api \
  --to-revisions=vrcpb-api-00009-wdb=100 \
  --region=asia-northeast1 --project=$PROJ

# smoke
curl -sS https://api.vrc-photobook.com/health
curl -sS https://api.vrc-photobook.com/readyz

# 結果を harness/work-logs/ に記録
```

### 7.3 image 保持 / cleanup

- 本 PR では AR の retention policy は設定しない（手動運用継続）
- PR40 ローンチ前 cleanup で「過去 N 件のみ保持」ルールを評価（spike 削除と同時）

---

## 8. 停止ポイント（実装 PR で必ずユーザー承認）

各停止ポイントで **ユーザーの明示承認を得てから次へ進む**。

| # | 停止ポイント | 承認内容 |
|---|---|---|
| 1 | Cloud Build API 有効化前 | `cloudbuild.googleapis.com` を enable してよいか（**課金開始ポイント**） |
| 2 | Cloud Build SA 作成 + IAM 付与前 | 上記 §5.2 の role セットを付与してよいか |
| 3 | trigger 作成前 | manual trigger だけ作成、source は GitHub repo（OAuth or App 連携） |
| 4 | 初回 build 実行前 | trigger を 1 度だけ手動実行し、build → push → deploy の経路を確認 |
| 5 | Cloud Run traffic 切替前 | smoke が pass したら traffic 100% に切替（既定動作だが事前再確認） |
| 6 | rollback 確認前 | 1 度だけ rollback runbook を実行して動作確認 |

各 §ステップ後は work-log に記録し、次 step へ進む前に必ず chat / commit で承認を得る。

---

## 9. 課金影響

### 9.1 各サービスの課金

| サービス | 現状 | PR29 後 |
|---|---|---|
| Cloud Build | 未有効（無料） | 有効化 + 月 120 build-min まで無料、超過は $0.003/build-min |
| Artifact Registry | 50MB（無料枠 0.5GB 内） | image 増加で漸増（0.5GB 超で $0.10/GB/月） |
| Cloud Run revision | 課金変動なし（traffic ベース） | 同上 |
| Cloud Logging | 既存（無料枠 50GB/月） | Cloud Build logs 増加（小、月 100MB 程度想定） |
| Cloud Billing Budget API | 未有効 | PR39 本番運用整備で評価 |

### 9.2 予想費用

- 月 10〜30 build 想定（PR ごと + 後続 PR の deploy）
- 1 build あたり ~3 分（docker build + push + deploy + smoke）
- 月 ~90 build-min → **無料枠内（120 build-min）**で収まる想定
- Artifact Registry は当面 0.5GB 未満で無料枠内

### 9.3 抑制策

- 過去 image の cleanup（PR40）
- Cloud Build の `machineType: E2_HIGHCPU_8` を検討中（速度優先）。コスト気にするなら
  default `E2_MEDIUM` で開始
- Budget Alert は PR39 で再設計

---

## 10. 既存 GitHub Actions との関係

### 10.1 役割分担（PR29 後）

| 担当 | 何を | どこで |
|---|---|---|
| `backend-ci.yml` | vet / build / test（PR check） | GitHub Actions |
| `frontend-ci.yml` | typecheck / test / build | GitHub Actions |
| Cloud Build | docker build / push / deploy / smoke | Cloud Build（manual trigger） |
| `claude-*.yml` | AI レビュー | GitHub Actions |

### 10.2 PR check ルール

- main branch protection は現状未設定（Public repo 化前のため）
- PR29 では変更しない。PR38 Public repo 化と同時に整備

### 10.3 Public repo 化前後の扱い

- 現在 private repo → Cloud Build trigger に GitHub OAuth 接続でも可
- Public repo 化後（PR38）に **Cloud Build GitHub App + WIF** に移行検討（より安全）
- 本 PR では現状の private repo 前提で OAuth ベース trigger を採用

---

## 11. Security

### 11.1 守るべき不変条件

- **Secret 値を `cloudbuild.yaml` に書かない**
- Cloud Build substitutions に Secret を入れない
- Cloud Build logs に Secret 値が出ないこと（`set -x` 禁止 / `--quiet` 維持）
- docker build context（`backend/`）に `.env` / `.env.local` / `.env.production` を含めない（既に `.gitignore` 済）
- image 内に Secret を埋め込まない（distroless static、ランタイム env のみで取得）
- Cloud Build SA に **secretmanager.secretAccessor を付けない**（runtime SA のみが読む）
- raw token / Cookie / manage URL を Cloud Build logs に出さない
- Cloud Run logs / Cloud Build logs を grep で監視（実装 PR で確認）

### 11.2 grep 監査項目（実装 PR commit 前）

```
grep -RInE "DATABASE_URL=|PASSWORD=|SECRET=|SECRET_KEY|API_KEY|sk_live|sk_test|R2_SECRET_ACCESS_KEY=|TURNSTILE_SECRET_KEY=" \
  cloudbuild.yaml docs/runbook/*.md
```

実値ヒット 0 件。用語は許容。

---

## 12. PR29 実装範囲（次の PR で実施する checklist）

### 12.1 ファイル追加

- [ ] `cloudbuild.yaml`（repo root or `backend/` 配下のどちらかに置く、配置は実装 PR で確定）
- [ ] `docs/runbook/backend-deploy.md`（手動 deploy / Cloud Build trigger / rollback 手順を統合した runbook）
- [ ] CLAUDE.md / 新正典に「Backend deploy は Cloud Build trigger 経由」と追記

### 12.2 GCP 操作（停止ポイントごとにユーザー承認）

- [ ] `cloudbuild.googleapis.com` 有効化（§8 #1）
- [ ] Cloud Build SA `vrcpb-cloud-build@<PROJ>.iam.gserviceaccount.com` 作成（§8 #2）
- [ ] §5.2 の role 付与（§8 #2）
- [ ] Cloud Build trigger 作成（manual、source: GitHub repo `kento-matsunaga/vrc-photobook`）（§8 #3）
- [ ] 初回 build 実行（§8 #4）
- [ ] traffic 100% 切替（§8 #5）
- [ ] rollback runbook 1 度実行（§8 #6）

### 12.3 やらないこと（実装 PR でも）

- Frontend Workers deploy 自動化
- Cloud Run Jobs / Scheduler
- SendGrid / Outbox
- Cloud SQL 本番化
- spike 削除
- main push trigger（後続段階で評価）

---

## 13. PR28 残課題の扱い

### 13.1 残課題

PR28 で未実施の **「実画像を含む完全 visual Safari 確認」**:

- 実 Safari + 実画像 upload + image-processor + publish の e2e
- macOS Safari / iPhone Safari の visual 確認

### 13.2 PR29 との関係

- **PR29 とは独立**。Cloud Build 自動化と Safari 確認は影響しない
- PR29 計画書 / 実装 PR の作業中でも、ユーザー側で並行して実施可能
- 持ち越す場合は新正典 §1.3 の「未実装」リストに維持

### 13.3 manual 確認手順

`harness/work-logs/2026-04-27_publish-flow-result.md` §「Viewer visual Safari 確認結果」
の **§推奨次手順**を参照。要約:

1. 実 Safari で `/draft/<token>` → `/edit/<id>` 着地
2. 実画像（JPEG / PNG / WebP）を upload UI からアップロード
3. ローカルで `cloud-sql-proxy + go run ./cmd/image-processor --all-pending` を実行
   （または image-processor が処理するのを待つ）
4. edit ページで photo grid 表示 → 設定保存 → 「公開へ進む」
5. CompleteView で公開 URL コピー
6. macOS Safari / iPhone Safari で `/p/[slug]` を開いて display 画像表示確認
7. 結果を `harness/work-logs/` に追記

> 上記は Cloud Run Jobs（PR31）が無くても **ローカル CLI で完結**する。

---

## 14. ユーザー判断事項（実装 PR 着手前に確定）

| 判断項目 | 推奨 | 代替 |
|---|---|---|
| Trigger 方式 | **manual trigger のみ**（§6.2） | tag trigger（後続段階） |
| Cloud Build API 有効化 | **承認後に enable**（§8 #1）。月 120 build-min 無料枠内で運用 | M2 完了まで保留 |
| Cloud Build 専用 SA 作成 | **作成して最小権限**（§5.2） | default compute SA 流用（非推奨） |
| Cloud Run runtime SA | **既存維持**（PR39 で再設計） | PR29 で専用 SA 化 |
| main push 自動 deploy | **しない**（事故リスク） | 後続段階で評価 |
| rollback 確認 | **初回 deploy 時に必ず 1 度実行** | スキップ（非推奨） |
| GitHub Actions と Cloud Build の役割 | **GH Actions = test、Cloud Build = deploy** で分離 | 一本化 |
| Cloud SQL `vrcpb-api-verify` 残置 | PR39 まで継続 | 早期 rename |
| Public repo 化 | PR38 まで保留 | 早期公開 |
| AR image retention | PR40 で評価 | PR29 で導入 |
| machineType | デフォルト（E2_MEDIUM）から開始、遅ければ昇格 | 最初から E2_HIGHCPU_8 |

---

## 15. 完了条件

- 本計画書 review 通過
- §14 ユーザー判断事項が確定
- §12 checklist が次の実装 PR でそのまま使える状態
- §8 停止ポイントが明示されている

---

## 16. 次 PR への引き継ぎ事項

実装 PR 着手時に必ず参照する設計確定事項:

- §3 現状リソース
- §4 cloudbuild.yaml 案
- §5 IAM 最小権限セット
- §6 manual trigger 採用
- §7 rollback runbook
- §8 停止ポイント（6 個、各々ユーザー承認）
- §11 Security 不変条件
- §12 実装 PR checklist

---

## 17. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版作成。PR28 完了時点での Cloud Build 自動 deploy 計画 |
