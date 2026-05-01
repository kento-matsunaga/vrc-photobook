# M2 image-processor 自動実行基盤 PR 計画書（m2-image-processor-job-automation）

> 作成: 2026-05-01
> 状態: **STOP β（設計判断資料）** ユーザー承認済（option α 採用）。STOP γ（実 GCP リソース作成）承認待ちで停止
> 起点: 2026-05-01 作成導線 PR（m2-create-entry）の STOP ε smoke 中に発覚した「画像アップロード後『処理中』が永遠に終わらない」P0 課題（→ STOP α 調査報告で根因特定）
>
> 関連 docs:
> - [`docs/plan/m2-image-processor-plan.md`](./m2-image-processor-plan.md) — PR23 で image-processor 本体を実装した時の計画書（本書は **その実行基盤を後追いで自動化**する位置づけ）
> - [`docs/plan/m2-image-upload-plan.md`](./m2-image-upload-plan.md)
> - [`docs/plan/m2-frontend-upload-ui-plan.md`](./m2-frontend-upload-ui-plan.md)
> - [`docs/plan/m2-outbox-plan.md`](./m2-outbox-plan.md)
> - [`docs/plan/m2-create-entry-plan.md`](./m2-create-entry-plan.md) — 本 PR 完了後に再開予定の保留中 PR
> - [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md) — closeout で更新
> - [`docs/runbook/backend-deploy.md`](../runbook/backend-deploy.md) — Cloud Run service deploy 既存 runbook（本 PR ではこれと並ぶ image-processor-ops.md を新設予定）
> - [`docs/design/cross-cutting/reconcile-scripts.md`](../design/cross-cutting/reconcile-scripts.md) §3.7 — Cloud Run Jobs + Scheduler の MVP 基本案（既存 outbox-worker と同方針）
>
> 関連 ADR:
> - [`docs/adr/0001-tech-stack.md`](../adr/0001-tech-stack.md) — Cloud Run 第一候補
> - [`docs/adr/0005-image-upload-flow.md`](../adr/0005-image-upload-flow.md) — image-processor の責務
>
> 関連 rules:
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) — Secret / raw ID / token を docs に書かない
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md) — Safari smoke
> - [`.agents/rules/feedback-loop.md`](../../.agents/rules/feedback-loop.md) — failure-log 起票判断
> - [`.agents/rules/pr-closeout.md`](../../.agents/rules/pr-closeout.md) — PR 完了処理

---

## 0. 本計画書の使い方

- **設計の正典は ADR-0005 + `m2-image-processor-plan.md`**。本書はそれを「実運用で `processing` 滞留が顕在化した時点で再判断する」と先送りしていた **実行基盤の自動化** を、再判断のうえ実装する PR の計画として固定する。
- §1〜§3 で「なぜ処理中が終わらないか」を正式記録（STOP α 調査報告の plan 化）。
- §4〜§6 で「どう自動で終わらせるか」の Job / Scheduler / 既存滞留救済の spec 候補を提示。
- §7 で smoke 設計と SLO 候補。§8 でリスク。
- §9 で STOP 設計、§10 で closeout で更新する資料リスト。
- 実装・gcloud 操作は本 PR の STOP β 段階では行わない。STOP γ 以降で承認後に実施。

---

## 1. 目的

1. **`images.status='processing'` が本番で滞留しないよう、image-processor を自動実行する**。
2. アップロード後、`available` への遷移と display / thumbnail variant 生成が **production で完了する経路を確立**する。
3. 既存 draft の processing 滞留分は別 STOP（本書 §6）で救済し、新規 upload は §4–§5 の Job + Scheduler 経路で吸収する。
4. **migration / Backend service deploy / Secret 追加・変更を発生させない**範囲で完結させる（既存 image `vrcpb-api:98c7155` を流用）。

非ゴール（本 PR では扱わない）:

- P1 改善（preview 即時表示 / multiple file input / upload queue / progress UI / polling timeout）。これらは別 PR で扱う（→ §10 closeout で roadmap 追記）。
- 既存 outbox-worker の起動自動化。本 PR は image-processor 専用の Job + Scheduler を新設するに留め、outbox-worker 自動化は引き続き「PR33e で要否判断」とする。

---

## 2. 現状整理（STOP α 調査結果の plan 化、根拠は §11 に集約）

### 2.1 「処理中」が終わらない設計上のギャップ

| 工程 | 担当 | 現状 |
|---|---|---|
| upload-verification 取得 | Frontend → Backend handler | 同期的に完了 |
| presigned PUT 取得 | Frontend → Backend handler | 同期的に完了 |
| R2 への直接 PUT | Frontend → R2 | 同期的に完了 |
| `complete` 通知（`POST /api/photobooks/{id}/images/{imageId}/complete`） | Backend handler | **同期的に `MarkProcessing()` を呼んで `images.status='processing'` に DB UPDATE するのみ。response も `status:"processing"` を即返す**。Outbox event は emit しない |
| variant 生成（display 1600px / thumbnail 480px）+ `MarkAvailable` + `image.became_available` Outbox INSERT | **`/backend/cmd/image-processor` 独立 CLI** | **本番で誰も起動していない**。Cloud Run Job 未作成、Cloud Scheduler 未設定、ローカル CLI も非実行 |
| Frontend polling（5 秒間隔で `processingCount > 0` を watch） | Frontend `EditClient.tsx` | timeout / abort なし。`processingCount === 0` まで永遠に polling |

帰結: **complete handler 直後に `processing` に固定された行が DB に残り続け、Frontend は 5 秒 polling で永遠に「処理中」を表示し続ける**。

### 2.2 outbox-worker は本問題の解決手段にならない

- outbox event `image.became_available` / `image.failed` の handler は **no-op + structured log のみ**（`backend/internal/outbox/internal/usecase/handlers/image_became_available.go`）。
- variant 生成・status 遷移は image-processor が **直接** `images.status='processing'` 行を `FOR UPDATE SKIP LOCKED` で claim する経路で実行されるため、outbox event は副産物に過ぎない。
- 既存 Cloud Run Job `vrcpb-outbox-worker` を手動 execute しても **image processing は進まない**（事実関係を §11 に収録）。

### 2.3 既存 docs 上の「先送り」記述（解除対象）

| 場所 | 既存記述 | 本 PR 完了後の状態 |
|---|---|---|
| `CLAUDE.md` 未実装欄 | "Cloud Scheduler 作成（outbox-worker 自動回し）→ 当面は手動 Job execute、PR33e で要否判断" | image-processor 側は本 PR で自動化、outbox-worker 側は引き続き保留（区別を明記） |
| `cmd/image-processor/main.go` 冒頭コメント | "現状は Cloud Run Job 未作成で、ローカル CLI / Cloud SQL Auth Proxy 経由で実行する。Cloud Run Job 化は実運用で processing 詰まりが顕在化した時点で再判断" | 状態ベース表現に書き換え（→ §10 closeout） |
| `docs/plan/m2-image-processor-plan.md` §15.1 / §15.2 | Cloud Run Jobs 化は別 PR、当面ローカル | 本 PR を相互参照する追記（または本書を後継として明記） |

---

## 3. 採用案（option α）

### 3.1 採用方針

- **Cloud Run Job `vrcpb-image-processor` を新設し、Cloud Scheduler から OIDC 認証で定期 trigger する**。
- 既存 image `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb/vrcpb-api:98c7155` を流用（image-processor binary は同梱済、entrypoint を `/usr/local/bin/image-processor` に切替）。
- env / Secret は既存 6 件（`DATABASE_URL` / `R2_ACCOUNT_ID` / `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_BUCKET_NAME` / `R2_ENDPOINT`）を `secretKeyRef` で再利用。
- migration / Backend service deploy / Secret 値変更 / Workers redeploy は **不要**。
- Job の image tag は今後の Backend deploy（`cloudbuild.yaml`）の SHORT_SHA に追従する手順を runbook 化（→ §10）。

### 3.2 採用しなかった案（理由をここに固定）

- **β（complete handler 同期化）**: Cloud Run の HTTP timeout / メモリ消費 / 同時 upload 503 risk / retry 設計が複雑化。MVP 非推奨。
- **γ 単独（ローカル CLI ワンショット）**: 自動化されないため運用継続性なし。本 PR では「既存滞留救済」用途に限定（§6）。
- **outbox-worker 経路への合流**: image.became_available handler を non-no-op 化しても、起動側の image-processor 実行を誰がやるかという根本問題は移動するだけ。設計上の責務分離（outbox = 副産物の通知 / image-processor = 一次処理）を維持する。

---

## 4. Cloud Run Job spec 案（`vrcpb-image-processor`）

### 4.1 確定パラメータ

| 項目 | 値 | 備考 |
|---|---|---|
| Job name | `vrcpb-image-processor` | 既存 `vrcpb-outbox-worker` と命名規約を揃える |
| region | `asia-northeast1` | 既存 service / Job と同じ |
| image | `asia-northeast1-docker.pkg.dev/<PROJ>/vrcpb/vrcpb-api:<SHORT_SHA>` | 初期値は `vrcpb-api:98c7155`。今後 Backend deploy ごとに更新（手順は §10 runbook） |
| command | `/usr/local/bin/image-processor` | Dockerfile 同梱の独立 binary |
| service account | `271979922385-compute@<PROJ>.iam.gserviceaccount.com` | 既存 outbox-worker と同 SA。`run.invoker` / `secretmanager.secretAccessor` を継承 |
| Cloud SQL 接続 | **`--add-cloudsql-instances` は付けない**（既存 outbox-worker と同方針）。`DATABASE_URL` の値で TCP / private IP に直接接続する | secretKeyRef を流用 |
| timeout (`--task-timeout`) | `90s`（初期値） | 既存 outbox-worker と同。実運用で増減判断 |
| `taskCount` / `parallelism` | `1` / `1` | 多重起動を最初は禁止。`FOR UPDATE SKIP LOCKED` の効果は §8 で別途検証 |
| max-retries (`--max-retries`) | `0` | Job 自体の retry は無効化。次の Scheduler tick に任せる |

### 4.2 args 候補（要 user 判断、初期値の確定）

| 候補 | args | 想定挙動 | 備考 |
|---|---|---|---|
| **A（推奨）** | `--all-pending --max-images 10 --timeout 60s` | 1 起動で最大 10 枚を順次処理。pending 行が無くなれば即 exit | scheduler 1 min 間隔と組合せ、瞬発負荷を抑える |
| B | `--once --max-images 1 --timeout 60s` | 1 起動 1 枚。outbox-worker と完全対称 | 大量 backlog 時の捌き速度が scheduler 間隔依存になる |
| C | `--all-pending --max-images 100 --timeout 5m` | 1 起動で最大 100 枚を 5 分以内に捌く | 1 起動の Cloud Run wall-time / メモリピークが大きい |

**推奨理由（A）**:
- 通常時の per-tick 1 起動コストを抑え、滞留時は max 10 まで連続処理できる
- Cloud Run Job の 90s timeout 内で 10 枚 × 数秒 / 枚で収まる想定
- §6 の既存滞留救済では一時的に C 相当（または直接 CLI）を別 STOP で実施

### 4.3 secretKeyRef / env

既存 `vrcpb-outbox-worker` の secretKeyRef を完全踏襲（追加・削除なし）:

| name | secretKeyRef.name | secretKeyRef.key | 必須 |
|---|---|---|---|
| `DATABASE_URL` | `DATABASE_URL` | `latest` | yes |
| `R2_ACCOUNT_ID` | `R2_ACCOUNT_ID` | `latest` | yes |
| `R2_ACCESS_KEY_ID` | `R2_ACCESS_KEY_ID` | `latest` | yes |
| `R2_SECRET_ACCESS_KEY` | `R2_SECRET_ACCESS_KEY` | `latest` | yes |
| `R2_BUCKET_NAME` | `R2_BUCKET_NAME` | `latest` | yes |
| `R2_ENDPOINT` | `R2_ENDPOINT` | `latest` | yes |

plain env:

| name | value |
|---|---|
| `APP_ENV` | `production` |

不要 secret（image-processor では使わない、追加しない）: `TURNSTILE_SECRET_KEY` / `REPORT_IP_HASH_SALT_V1`。

### 4.4 リソース

| 項目 | 初期値 | 根拠 |
|---|---|---|
| CPU | `1` | outbox-worker と同。decode / encode で CPU 律速 |
| memory | `512Mi` | outbox-worker と同。1600px JPEG decode / encode は通常数十 MiB で済む見込み。実測で **不足が観測されたら 1Gi に昇格**（§8 risk） |

### 4.5 Job 作成時の gcloud コマンド（雛形、実行は STOP γ）

> **本 PR では実行しない**。STOP γ 承認後に runbook 経由で実施。`<PROJ>` / `<SHORT_SHA>` は実値に置換、Secret 値は表示しない。

```bash
PROJ=<gcp-project-id>
SHORT_SHA=98c7155

gcloud run jobs create vrcpb-image-processor \
  --image=asia-northeast1-docker.pkg.dev/${PROJ}/vrcpb/vrcpb-api:${SHORT_SHA} \
  --region=asia-northeast1 \
  --project=${PROJ} \
  --service-account=271979922385-compute@${PROJ}.iam.gserviceaccount.com \
  --command=/usr/local/bin/image-processor \
  --args=--all-pending,--max-images,10,--timeout,60s \
  --task-timeout=90s \
  --tasks=1 \
  --parallelism=1 \
  --max-retries=0 \
  --cpu=1 \
  --memory=512Mi \
  --set-env-vars=APP_ENV=production \
  --set-secrets=DATABASE_URL=DATABASE_URL:latest,\
R2_ACCOUNT_ID=R2_ACCOUNT_ID:latest,\
R2_ACCESS_KEY_ID=R2_ACCESS_KEY_ID:latest,\
R2_SECRET_ACCESS_KEY=R2_SECRET_ACCESS_KEY:latest,\
R2_BUCKET_NAME=R2_BUCKET_NAME:latest,\
R2_ENDPOINT=R2_ENDPOINT:latest
```

### 4.6 実行ログの見方

- 確認場所: GCP Console > Cloud Run > Jobs > `vrcpb-image-processor` > Executions、または `gcloud run jobs executions describe <EXEC_ID>`。
- structured log の主要 key（image-processor 側で固定済、§11 根拠）:
  - `image-processor starting`（once / all_pending / dry_run / max_images / timeout）
  - `image-processor finished`（picked / success / failed）
- **ログに raw image_id / storage_key / R2 credentials / DATABASE_URL / file 内容を出さない**前提が image-processor 側で実装済（plan §10B.2 / `cmd/image-processor/main.go` セキュリティ節）。本 PR の closeout 時に grep で再確認。

---

## 5. Cloud Scheduler spec 案

### 5.1 確定パラメータ

| 項目 | 値 | 備考 |
|---|---|---|
| Scheduler name | `vrcpb-image-processor-tick` | Job 名と suffix `-tick` で対応関係を明示 |
| region | `asia-northeast1` | Job と同一 region 推奨（latency / billing 整合） |
| target | Cloud Run Jobs invoke（HTTP target、URI: `https://asia-northeast1-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/<PROJ>/jobs/vrcpb-image-processor:run`） | `gcloud scheduler jobs create http ... --uri=...` 経由 |
| auth | OIDC token、`--oidc-service-account-email=271979922385-compute@<PROJ>.iam.gserviceaccount.com`、`--oidc-token-audience=https://asia-northeast1-run.googleapis.com/` | SA に `roles/run.invoker` 付与必須（既存付与済か STOP γ で確認） |
| HTTP method | POST | Cloud Run Jobs run API |
| body | `{}`（空） | args は Job 側に固定 |
| time_zone | `Asia/Tokyo` | log タイムスタンプを実運用と合わせる |

### 5.2 interval 候補比較（要 user 判断）

| 候補 | cron | 体感 latency（最悪） | per-day Job 起動回数 | 課金影響（推定） |
|---|---|---|---|---|
| **A. 1 分** | `* * * * *` | 1 分 + 処理時間（数十秒） | 1440 | Scheduler 自体は free tier 内（3/月以内 free、超過は $0.10/job/月）、Cloud Run Job CPU/memory 課金は 1 起動あたり数秒の課金で 1 日 1440 起動でも軽微（要 Budget 確認） |
| **B. 5 分** | `*/5 * * * *` | 5 分 + 処理時間 | 288 | A の 1/5。実利用初期はこちらで開始するのが堅実 |

**推奨**: **B（5 分間隔）から開始**。upload UX として 5 分以内に variant が出ることが SLO（§7）として現実的。実運用で「5 分待ちが UX を損ねている」と判定したら A に切替。切替は scheduler 1 行更新で完了（§5.4）。

### 5.3 retry policy

| 項目 | 値 | 備考 |
|---|---|---|
| `max-retry-attempts` | `0` | Scheduler 側 retry は無効化。次の tick に任せる |
| `attempt-deadline` | `120s` | Cloud Run Jobs run API call 自体の deadline。Job 実行時間ではない |

理由: image-processor は Job 内で行を `FOR UPDATE SKIP LOCKED` で claim するため、Scheduler retry で多重起動した場合でも同一 image を重複処理しない設計。だが余計な起動は無駄なので retry を 0 にする。

### 5.4 pause / disable / 切替手順（runbook 化、本 PR §10）

```bash
# 一時停止
gcloud scheduler jobs pause vrcpb-image-processor-tick \
  --location=asia-northeast1 --project=${PROJ}

# 再開
gcloud scheduler jobs resume vrcpb-image-processor-tick \
  --location=asia-northeast1 --project=${PROJ}

# 間隔変更（5 min → 1 min）
gcloud scheduler jobs update http vrcpb-image-processor-tick \
  --schedule='* * * * *' \
  --location=asia-northeast1 --project=${PROJ}

# Scheduler 削除（rollback 用）
gcloud scheduler jobs delete vrcpb-image-processor-tick \
  --location=asia-northeast1 --project=${PROJ}
```

### 5.5 cost impact（推定、要 Budget 確認 STOP γ）

- Cloud Scheduler: 3 jobs/月以内 free tier。本 PR で +1 job → free tier 内に収まる（既存 0 件想定）。
- Cloud Run Jobs: 1 起動あたり CPU 1 / memory 512Mi、想定実行時間は idle 時 1 秒未満（pending 0 件で即 exit）、滞留時 数秒〜数十秒。1 day 288 起動（5 min interval）でも **月コスト < $5 想定**（Budget Alert で確認）。
- ネットワーク egress: R2 GET（original）+ R2 PUT（display + thumbnail）で **同 region は 0 円**（Cloudflare R2 仕様）。
- Cloud SQL: connection 数増（Job ごとに pool 1 個）。既存 service の connection 数と合わせて instance 上限内で OK か STOP γ で確認。

---

## 6. 既存 processing 滞留救済（option γ、本 PR §6 で範囲を確定、実施は STOP δ で別承認）

### 6.1 位置づけ

- 本 PR の §4–§5（Job + Scheduler）が稼働すれば、**作成時点以降の新規 upload は自動で available 化される**。
- ただし Scheduler 稼働開始**より前**に作成された draft の `processing` 行は、Job 起動時に `FOR UPDATE SKIP LOCKED` で順次拾われて自然解消する想定。
- もし古すぎる行（`storage_key` が R2 に既に無い等）が混じっていた場合、image-processor が `failed` 化（failure_reason: `object_not_found` 等）して整理する。
- **本 PR では §6 の手動救済は基本不要**。Scheduler 起動後の挙動を観測してから判断する。
- ただし「Job が動いても失敗ばかり」「件数が多すぎて自然解消に長時間かかる」場合の **緊急避難として手順を文書化**する。

### 6.2 実施判断基準（STOP δ 承認の要件）

以下のいずれかが満たされた時に STOP δ を提案:

1. STOP γ で Job + Scheduler を起動した後、24 時間経過しても `processing` 件数が減らない。
2. Frontend smoke で「処理中が消えない」が依然再現する。
3. Job execution の log で同一 `<image-id-redacted>` を繰り返し失敗 → `failed` 化されているが Frontend 側 polling が止まらない（image-processor の責務外、frontend 側 fix が別 PR で必要、`failed` 行の grid 表示を改善）。

### 6.3 事前件数確認（読み取りのみ、Cloud SQL Auth Proxy 経由）

```bash
# Cloud SQL Auth Proxy 起動（別端末）
cloud-sql-proxy <PROJ>:asia-northeast1:<INSTANCE> --port 5433

# 件数確認（read-only、SELECT のみ）
psql "host=127.0.0.1 port=5433 dbname=<DB> user=<USER> sslmode=disable" \
  -c "SELECT count(*) FROM images WHERE status='processing';"
```

> **値（DATABASE_URL / instance 名 / user / DB 名）はここに書かない**。実値は Secret Manager / runbook 別ファイル（権限制御）で参照。
> **件数結果のみ work-log に記録**（例: "処理中滞留 N 件、redacted")。raw image_id / photobook_id / storage_key は記録しない。

### 6.4 救済実行（書き込み伴う、要 STOP δ 承認）

```bash
# ローカル CLI で all-pending + 大きめ batch で一括処理
DATABASE_URL=<取得経路は runbook 別ファイル> \
R2_ACCOUNT_ID=... R2_ACCESS_KEY_ID=... R2_SECRET_ACCESS_KEY=... \
R2_BUCKET_NAME=... R2_ENDPOINT=... APP_ENV=production \
  /home/erenoa6621/dev/vrc_photobook/backend/cmd/image-processor/<built-binary> \
  --all-pending --max-images 200 --timeout 10m
```

> 実値（DATABASE_URL / R2 credentials）は本書に書かない。実行は **対話シェル**で env 変数を export 後に CLI を 1 行で実行する。
> stdout / stderr の log は **redact してから** work-log に貼り付ける（image_id / storage_key / R2 endpoint URL は伏せる）。

### 6.5 実行後確認

- 同 §6.3 の SELECT 件数が **減少** していること。
- 残った件数を分類: `processing` のまま残ったもの / `failed` に遷移したもの / `available` になったもの。**件数のみ**記録。
- Frontend 側でも該当 photobook の `processingCount` が 0 になることをスポット smoke（ユーザ確認、raw URL は手元のみ）。

### 6.6 raw ID を記録しない運用

- work-log / failure-log / chat / commit message に出してよいのは **件数 / 種別比率 / 時刻 / Job execution ID** のみ。
- 出してはいけないもの: `image_id` / `photobook_id` / `slug` / `storage_key` / R2 URL / `DATABASE_URL` / `R2_*` 値 / Cookie / Bearer / draft_edit_token / manage_url_token。

### 6.7 rollback 不能性の明示

- image-processor の `MarkAvailable` は **冪等ではあるが反転不能**（一度 `available` に進んだ image を `processing` に戻す API はない、ADR-0005 状態遷移ガード）。
- `MarkFailed` 経由で `failed` 化された行も MVP では **再 processing しない**（recovery は別 PR、現状は `failure_reason` の手動再 upload 案内）。
- したがって §6.4 を実行すると、**大量の状態遷移が一度に確定**する。**dry-run（`--dry-run`）で件数確認 → 通常実行**の 2 段階が必須。STOP δ 提案時に dry-run 結果を redact 件数で添付する。

---

## 7. smoke 設計

### 7.1 smoke ケース（STOP ε で実施）

| ケース | 操作 | 期待 |
|---|---|---|
| **A. 通常 1 枚 upload smoke** | `/create` → `/draft/<token>` → `/edit/<photobookId>` で画像 1 枚 upload | (1) complete handler が 200 / status=processing を返す。(2) `processingCount=1` になる。(3) Scheduler tick 後、Job 1 起動。(4) Job 内で image-processor が claim → R2 GET → variant 生成 → R2 PUT × 2 → MarkAvailable → Outbox INSERT。(5) edit-view 再取得で `processingCount=0` 観測、grid に variant 表示 |
| B. 大きい画像 upload | 5MB 程度の JPEG | A と同様だが処理時間が増える。Job 90s timeout 内に収まること |
| C. 失敗ケース（壊れた JPEG） | 故意に壊した image | image-processor が `decode_failed` で `MarkFailed`。Outbox に `image.failed` INSERT。Frontend は `processing` の行が `failed` にならない（既存 frontend は `failed` を表示しない問題 → 別 PR で改善、本 PR では fact のみ記録） |
| D. 複数枚同時 upload | 既存 UI の制約により 1 枚ずつしか upload できない（§2.1）→ smoke ではユーザが立て続けに 2 枚 upload する操作で代替 | 2 枚とも processing → Scheduler 1 tick で max 10 枚 batch 内に収まり順次 available 化 |
| E. dry-run（観測のみ） | Cloud Console から Job を `--args=--dry-run` で 1 度実行 | log に `picked=N success=0 failed=0` と claim 結果が出る。DB 状態は変わらない |

### 7.2 SLO 候補（要 user 判断）

| 観点 | SLO 候補 |
|---|---|
| upload 完了から variant 表示までの **max wait（P99）** | A. **5 分**（Scheduler 5 min 間隔 + Job 数十秒）<br>B. **2 分**（Scheduler 1 min 間隔 + Job 数十秒）<br>C. **10 分**（バッファ込み、初期運用の安全側） |
| **失敗時の確定までの max wait** | 5 分以内（同上） |
| **滞留警告ライン** | `processing` が 30 分以上滞留している件数が 0 件であること。failure-log 起票判断ライン |

**推奨初期 SLO**: **C（max 10 分）**。Scheduler 5 min から開始 + 観測 + 必要なら 1 min 化。

### 7.3 観測手段

- Cloud Run Jobs Executions 一覧（GCP Console）。
- Cloud Logging で `severity=INFO` かつ `image-processor finished` を時系列にプロット（picked / success / failed の合計が日次で保たれているか）。
- DB の `images.status` 分布（手動 SELECT、§6.3 の手順）。

---

## 8. リスク

| # | リスク | 影響 | 緩和 |
|---|---|---|---|
| R1 | Scheduler 課金 | 軽微（free tier 内 + Job 実行費月数 USD） | Budget Alert（PR39）で監視。STOP γ で初期 Budget 設定確認 |
| R2 | Job 多重起動（Scheduler retry / 手動 execute 同時など） | 同一 image を 2 つの Job が拾う | image-processor は `FOR UPDATE SKIP LOCKED` で claim する（PR23 で実装済）。Scheduler retry を 0 に固定（§5.3）。手動 execute 時は必ず Scheduler を pause（runbook §10） |
| R3 | R2 / Cloud SQL 負荷（連続起動） | DB connection 上限 / R2 rate limit | 初期 max-images=10 / parallelism=1 で抑制。観測しながら調整 |
| R4 | 大きい画像 / 異常画像で処理失敗 | `decode_failed` / `unsupported_format` / OOM | image-processor の `failure_reason` 12 種で分類済（PR23）。memory 512Mi 不足が観測されたら 1Gi に昇格（§4.4） |
| R5 | `failed` 化された行が Frontend で「処理中」のまま見える | UX 問題 | 既知のギャップ。**本 PR では fix しない**（frontend 修正 = P1 別 PR）。closeout で roadmap に追記 |
| R6 | Cloud SQL 接続のための `--add-cloudsql-instances` を付けるべきか論点 | 既存 outbox-worker と非対称 | 既存 outbox-worker は付けていない（DATABASE_URL TCP / private IP 接続）。**同方針を踏襲**。STOP γ 実行前に既存 service の接続方式を再確認 |
| R7 | image tag drift（Backend deploy ごとに Job image を更新する手順を忘れる） | 古い image-processor binary が動き続ける | runbook 化（§10、`docs/runbook/image-processor-ops.md` 新設）。Backend deploy runbook §1.4 と相互参照 |
| R8 | Idempotency（既に `available` になった行を再処理しない） | DB UPDATE 競合 | image domain の遷移ガード（`processing → available` のみ許可、`available → ...` は拒否）で担保。PR23 unit test で確認済 |
| R9 | `cloudbuild.yaml` の `traffic-to-latest` step 系の自動化が Job には無い | 手動更新時のヒューマンエラー | runbook §10 の手順を chat / commit で再現可能にしておく |
| R10 | option γ 実行時の DATABASE_URL / R2 credentials を端末で扱うリスク | Secret 漏洩 | env 変数経由で 1 行 export → CLI 実行 → unset、log redact、`harness/failure-log/` に手順を起票しない（実値が残る恐れ） |

### 8.1 失敗化条件（image-processor 側で既存実装済、参考）

- `decode_failed`：image library が decode 不能
- `unsupported_format`：HEIC / RAW 等、MVP 対応外（HEIC 本対応は別 PR、CLAUDE.md 未実装欄）
- `object_not_found`：R2 に original が無い（upload 中断 / R2 cleanup 後）
- `dimension_too_small` / `dimension_too_large`：サイズ制約
- `file_too_large`：byte size 制約
- 他 7 種は ADR-0005 / domain VO `failure_reason` 参照

### 8.2 retry / idempotency の取り扱い

- Job 自体の retry: `--max-retries=0`（§4.1）。
- Scheduler retry: `--max-retry-attempts=0`（§5.3）。
- image-processor 側の retry: **しない**。`MarkFailed` 後は再 processing しない（MVP）。
- 結果としてシステム全体は「次の Scheduler tick で次の pending を拾う」という単純なループで idempotent。

---

## 9. STOP 設計（実施タイミング）

| STOP | 内容 | 実 GCP 操作 | 課金影響 | 必要承認 |
|---|---|---|---|---|
| **α**（完了） | 調査報告（処理中が終わらない原因特定）、option α 採用判断 | なし | なし | 完了 |
| **β**（**本 PR、ここまで実施**） | 計画書（本書）の作成、必要な script / docs / tests の追加、commit + push | なし | なし | ユーザー指示で着手済 |
| **γ** | Cloud Run Job `vrcpb-image-processor` 作成 + Cloud Scheduler `vrcpb-image-processor-tick` 作成。SA に `roles/run.invoker` の事前付与確認 | あり（gcloud） | **発生**（Scheduler 1 + Job 起動費） | **要承認**（次のステップ） |
| **δ**（任意） | §6 の既存 processing 滞留救済（dry-run → 実行）。発動条件は §6.2 | あり（ローカル CLI + Cloud SQL Auth Proxy） | なし（軽微） | **要承認**（条件付き） |
| **ε** | §7 の smoke。Safari macOS / iPhone 実機で 1 枚 upload → variant 出るまで | スポット観察のみ | 微 | **要承認**（実 upload を行う） |
| **final** | work-log / roadmap / runbook / CLAUDE.md / failure-log 判断 / commit + push、本 PR closeout | なし | なし | 完了報告 |

### 9.1 各 STOP の終了条件

- β 終了: 本書 push + 該当 commit が main に到達 + STOP γ 承認待ちで停止 → 本セッションの目標
- γ 終了: Job / Scheduler 作成完了 + 起動 1 回成功（idle で `picked=0`）+ secretKeyRef / SA / 接続性確認
- δ 終了: 件数（redacted）が想定通り減少 + work-log に記録
- ε 終了: smoke A〜E のうち最低 A をユーザ実機で OK + 観測結果が SLO 内
- final 終了: §10 の closeout を完了

---

## 10. closeout で更新する資料

| ファイル | 更新内容 |
|---|---|
| `harness/work-logs/2026-05-XX_image-processor-job-automation-result.md`（新規） | β 計画書 push / γ で作成した Job / Scheduler ID / ε smoke 結果（redact 済）/ SLO 観測値 |
| `docs/plan/vrc-photobook-final-roadmap.md` | 本 PR を新 PR 番号で正式登録（§1.x 該当箇所）、create-entry PR との依存関係（本 PR が先、create-entry STOP ε 再開は本 PR 完了後）を明記 |
| `docs/runbook/image-processor-ops.md`（**新規**） | Job 手動 execute / Scheduler pause-resume / image tag 更新（Backend deploy 連動）/ 既存滞留救済手順（§6 抜粋）/ 監視 query |
| `docs/runbook/backend-deploy.md` | §1.4 / §1.5 の最後に「image-processor Job の image tag も同じ SHORT_SHA で `gcloud run jobs update vrcpb-image-processor --image=...:<SHORT_SHA>` で揃えること」を追記（`vrcpb-outbox-worker` についても既存の手順と並べる） |
| `CLAUDE.md` 未実装欄 | 「Cloud Scheduler 作成（outbox-worker 自動回し）」を **「outbox-worker 側のみ未実装、image-processor 側は本 PR で自動化済」** に書き換え。「主要動線で実装済の機能」欄に「image-processor Cloud Run Job + Scheduler 自動実行」を追加 |
| `docs/plan/m2-image-processor-plan.md` §15.1 / §15.2 | 「Cloud Run Jobs 化は別 PR」記述を「本 PR（m2-image-processor-job-automation）で自動化」に更新 |
| `cmd/image-processor/main.go` 冒頭コメント | 「Cloud Run Job 未作成で、ローカル CLI / Cloud SQL Auth Proxy 経由で実行する」記述を状態ベース表現に更新（PR 番号は書かない、`.agents/rules/pr-closeout.md`） |
| `harness/failure-log/2026-05-XX_image-processor-job-automation-gap.md`（**新規起票判断**） | 「Cloud Run Job 未作成で processing が滞留した」事象を 失敗 → ルール化 の対象として記録するか判断。記録する場合の対策ルール例: 「非同期 worker / Job が必要な aggregate を実装した PR では、起動基盤の自動化を同 PR / 直近 PR で必ず integrate する（手動運用前提の merge を避ける）」 |
| `docs/plan/m2-create-entry-plan.md` | 末尾に「STOP ε は本 PR の final 後に再開」のメモを追記 |

### 10.1 PR closeout チェックリスト（`.agents/rules/pr-closeout.md` 準拠、本 PR 専用）

- [ ] `bash scripts/check-stale-comments.sh` 実行 + ヒットを 4 区分に分類
- [ ] 古い「PR 番号 + 未来形」記述を状態ベース表現に書き換え（§10 該当ファイル）
- [ ] 先送り事項（outbox-worker 自動化 / Frontend `failed` 表示改善 / P1 改善 / multiple upload）を roadmap に **「いつ・どの PR 以降で再検討するか」付き**で記録
- [ ] generated file（sqlcgen / OpenAPI）に未反映が無いこと（本 PR は migration なし → generated 影響なし想定、再確認）
- [ ] Secret / raw ID grep（`.agents/rules/security-guard.md` 禁止リスト）を本書 / commit 全体で実行 → 0 件
- [ ] failure-log 起票判断（§10 該当行）

---

## 11. 根拠資料（事実 vs 推測の分離）

本書の各記述の根拠ファイル / 行番号を集約。

### 11.1 確認済の事実（コード根拠）

| 記述 | 根拠ファイル:行 |
|---|---|
| complete handler は同期的に MarkProcessing するのみ、Outbox event は emit しない | `backend/internal/imageupload/internal/usecase/complete_upload.go:58-163` |
| variant 生成（display + thumbnail）+ MarkAvailable + Outbox INSERT は image-processor の同一 TX 内 | `backend/internal/imageprocessor/internal/usecase/process_image.go:111-347, 300-321` |
| image-processor は独立 CLI（HTTP endpoint なし） | `backend/cmd/image-processor/main.go:1-128` |
| Dockerfile に api / outbox-worker / image-processor を同梱 | `backend/Dockerfile:41-49` |
| Cloud Run service の CMD は `/usr/local/bin/api` 固定 | `backend/Dockerfile:49` |
| 既存 `vrcpb-outbox-worker` Job の secretKeyRef / SA / timeout 構成 | `gcloud run jobs describe vrcpb-outbox-worker` 出力（2026-05-01 取得） |
| outbox `image.became_available` / `image.failed` handler は no-op | `backend/internal/outbox/internal/usecase/handlers/image_became_available.go:11-36` |
| frontend polling は `processingCount > 0` 間 5 秒間隔、timeout / abort なし | `frontend/app/(draft)/edit/[photobookId]/EditClient.tsx:121-127, 462-468, 289` |
| 「Cloud Scheduler 作成（outbox-worker 自動回し）」が CLAUDE.md 未実装欄に明記、image-processor 側も同方針 | `CLAUDE.md` |
| image-processor の args（`--once` / `--all-pending` / `--dry-run` / `--max-images` / `--timeout`） | `backend/cmd/image-processor/main.go:45-53` |
| image domain の status 6 値と遷移ガード | `backend/internal/image/domain/vo/image_status/image_status.go:19-67`, `.../entity/image.go:174-245` |
| reconcile-scripts.md に「Cloud Run Jobs + Cloud Scheduler が MVP 基本案」と明記 | `docs/design/cross-cutting/reconcile-scripts.md:292,294,429,444,459,460,472` |

### 11.2 推測（実 GCP / DB 観測なしで判断、STOP γ 以降で検証）

| 推測 | 検証ステップ |
|---|---|
| 既存の `processing` 滞留件数 | STOP δ §6.3 で SELECT count（実値は redact） |
| 大きい image での memory 512Mi 妥当性 | STOP ε で B ケース実測、Cloud Logging の memory metric |
| Scheduler 5 min 間隔の UX 受容性 | STOP ε でユーザ実機操作の体感 |
| Cloud SQL connection 数増の影響 | STOP γ で `gcloud sql instances describe` の current connections と max compare |

---

## 12. 制約遵守

- 本書に raw `photobook_id` / `image_id` / `slug` / token / Cookie / `storage_key` / upload URL / R2 endpoint URL の実値 / `DATABASE_URL` / Secret 値 / Bearer / sk_live / sk_test を記載していない（§6 は手順 placeholder のみ）。
- 本書作成時点で実 GCP 操作（Job / Scheduler 作成、Job execute、Scheduler enable）を実施していない。
- production DB 書き込み、Secret / env / secretKeyRef / cloudsql-instances 変更は STOP γ / δ の承認後に限定。
- `.claude/scheduled_tasks.lock` は触らない。

---

## 13. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-01 | 初版作成（STOP β）。create-entry PR の STOP ε smoke 中に発覚した P0「processing 終わらない」を根拠に、option α（Cloud Run Job + Cloud Scheduler）採用、image-processor 自動実行基盤を本 PR として独立計画化 |
