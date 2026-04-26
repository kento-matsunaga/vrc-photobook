# M2 Cloud SQL 短時間検証 計画

> 作成日: 2026-04-26
> 位置付け: Cloud Run service `vrcpb-api`（DB なし deploy 済）に **Cloud SQL を短時間だけ
> 接続**して、`/readyz` 200 と token exchange 200 経路を実環境で確認するための計画書。
> **Cloud SQL 作成 / Secret Manager 登録 / Cloud Run update / migration 適用は本書段階では一切実行しない**。
>
> 上流参照（必読、本書では再記載しない）:
> - [`docs/plan/m2-backend-cloud-run-deploy-plan.md`](./m2-backend-cloud-run-deploy-plan.md) §3 DB 接続方針 / §3.4 後続段階
> - [`harness/work-logs/2026-04-26_backend-cloud-run-deploy-result.md`](../../harness/work-logs/2026-04-26_backend-cloud-run-deploy-result.md)（DB なし deploy 完了記録）
> - [`docs/plan/m2-domain-mapping-execution-plan.md`](./m2-domain-mapping-execution-plan.md) §8 環境変数 / Secret Manager
> - [`docs/plan/m2-implementation-bootstrap-plan.md`](./m2-implementation-bootstrap-plan.md) §10
> - [`backend/migrations/`](../../backend/migrations/)（00001〜00004）
> - [`backend/cmd/api/main.go`](../../backend/cmd/api/main.go) / [`backend/internal/database/pool.go`](../../backend/internal/database/pool.go)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
> - [Cloud Run + Cloud SQL（Unix socket）公式](https://docs.cloud.google.com/sql/docs/postgres/connect-run)
> - [Cloud SQL Auth Proxy 公式](https://docs.cloud.google.com/sql/docs/postgres/sql-proxy)

---

## 0. 本計画書の使い方

- 本書は **計画書のみ**。Cloud SQL / Secret / Cloud Run update / migration の実操作は **行わない**。
- §3 で接続方式を確定 → §4-§6 で個別計画を確認 → §8 で update コマンド案を確認 → §12 のユーザー判断事項に答えてから実施 PR に進む。
- 実施 PR は **「作成 → 検証 → 削除」を同じ作業セッションで完結させる**（§10 費用ガード）。途中で中断する場合も Cloud SQL は削除して終了する。

---

## 1. 目的

- 短時間（**目安 1〜2 時間以内**）だけ Cloud SQL PostgreSQL 16 を起動し、`vrcpb-api` から接続できる状態を作る
- `backend/migrations/` を Cloud SQL に適用
- `DATABASE_URL` を Secret Manager に登録
- Cloud Run `vrcpb-api` revision に Secret を注入して reload
- `/readyz` 200 と token exchange 200 経路を実環境で確認
- 検証完了後、**Cloud SQL を削除**して課金を完全停止する

---

## 2. 今回確認すること

| 項目 | 確認内容 |
|---|---|
| Cloud Run → Cloud SQL 接続 | Unix socket（`--add-cloudsql-instances`）経由で接続成功 |
| migration 適用 | `goose up` で `00001`〜`00004` がすべて適用、テーブル 3 + FK 1 が作成 |
| `/readyz` 200 | DB 接続成功で `{"status":"ready"}` |
| token exchange 登録 | pool != nil で `photobookHandlers` が組み立てられ、`/api/auth/*` が router に登録 |
| 空 body | 400 `{"status":"bad_request"}` |
| 不正 token | 401 `{"status":"unauthorized"}` |
| 実 token | 200 + `{"session_token":..., "photobook_id":..., "expires_at":...}` |
| manage 経路 | 同様 + `token_version_at_issue:0` |
| `Set-Cookie` 出さない | curl 確認 |
| `Cache-Control: no-store` | curl 確認 |
| Cloud Run logs | Secret / token / Cookie / DSN 漏洩なし |
| Cloud SQL 接続ログ | `pg_isready` 相当で異常なし、SASL 認証成功 |

---

## 3. 接続方式の比較

### 3.1 候補

| 案 | 内容 | 設定 |
|---|---|---|
| **案 A** | **Cloud SQL Unix socket（Cloud Run の `--add-cloudsql-instances`）**| `DATABASE_URL=postgres://user:pass@/db?host=/cloudsql/<INSTANCE_CONNECTION_NAME>` |
| 案 B | Public IP + SSL | `DATABASE_URL=postgres://user:pass@<public-ip>:5432/db?sslmode=verify-ca&sslrootcert=...` |
| 案 C | Private IP + Serverless VPC Access Connector | `DATABASE_URL=postgres://user:pass@<private-ip>:5432/db?sslmode=disable` + VPC Connector 設定 |

### 3.2 比較

| 観点 | 案 A（Unix socket）| 案 B（Public IP）| 案 C（Private IP + VPC）|
|---|---|---|---|
| 設定の簡単さ | ★★★（`--add-cloudsql-instances` 1 つ） | ★★（公開 IP / SSL 設定 / 認可ネットワーク） | ★（VPC Connector 作成 + Private IP 有効化） |
| セキュリティ | ★★★（IAM 経由で認証、外部公開なし） | ★（パブリック IP、GCP で SSL は必須） | ★★★（VPC 内のみ） |
| コスト | ★★★（Cloud SQL 標準のみ） | ★★★ | ★（VPC Connector が **常時 $7/月程度**、削除しないと残る） |
| 後片付け | ★★★（Cloud SQL 削除のみで完了） | ★★（IP / authorized networks 整理）| ★（VPC Connector / Private IP 周りを個別削除） |
| M2 短時間検証との相性 | ★★★ | ★★ | ★（追加設定の手間が大きい） |
| 本番移行しやすさ | ★★★（公式推奨、Cloud Run + Cloud SQL の標準パターン）| ★（公開 IP は本番非推奨） | ★★（本番でも一般的、ただし VPC Connector 課金あり） |

### 3.3 推奨: **案 A（Cloud SQL Unix socket、`--add-cloudsql-instances`）**

理由:
- M2 短時間検証で最小工数
- Public IP を立てず、VPC Connector の常時課金もない
- Cloud Run の標準パターン、本番移行時もそのまま使える
- Cloud SQL 削除だけで全リソースが消える（後片付けが単純）

---

## 4. Cloud SQL instance 設計

### 4.1 設計値

| 項目 | 値 | 根拠 |
|---|---|---|
| instance 名 | `vrcpb-api-verify`（短期検証用、本番名は別途）| 本番 `vrcpb-api-db` 等とは別の名前にして「**検証用 / 削除前提**」を明示 |
| Database version | `PostgreSQL 16` | `backend/docker-compose.yaml` の postgres:16-alpine と同 |
| Region | `asia-northeast1` | Cloud Run と同リージョン（Unix socket 接続のレイテンシ最小） |
| Tier | `db-f1-micro` | 最小スペック、月額 ~$10。検証期間は 1〜2 時間で実質 ~¥30 |
| Storage | `10 GB` SSD（最小）| 検証では数 KB しか使わない |
| Storage auto increase | OFF | 検証用、自動拡張不要 |
| Backup | **OFF**（短期検証）| backup 自動作成は課金 + 削除が複雑になる |
| Point-in-time recovery | OFF | 同上 |
| Deletion protection | **OFF** | 検証後すぐ削除するため OFF（M1 PoC でも OFF にして削除した経緯と整合） |
| Public IP | OFF（無効）| 案 A Unix socket 接続なので不要 |
| Private IP | OFF | 同上 |
| Connection: `cloudsqlsuperuser` (postgres) | デフォルト Postgres user 自動作成 | パスワードを Secret に保存 |
| Connection: app user | `vrcpb_app`（新規作成）| 最小権限で運用、`vrcpb` database に DDL/DML 権限 |
| Database name | `vrcpb` | `backend/docker-compose.yaml` と整合 |
| Timezone / charset | デフォルト UTC / utf8 | 設定不要（PostgreSQL 16 既定） |
| Maintenance window | デフォルト | 1〜2 時間で削除するので影響なし |
| Connection name | `<project>:asia-northeast1:vrcpb-api-verify` | Cloud Run の `--add-cloudsql-instances` で指定 |

### 4.2 重要な制約

- **検証後は停止ではなく削除**（停止しても課金が完全には止まらない、tier コスト + ストレージ）
- **deletion protection を OFF**（短期検証では削除を阻害させない）
- **作成前に必ずユーザー承認**（M1 で「ユーザー判断なしの実リソース作成」を起こさないルール）
- 作業セッション内で「作成 → 検証 → 削除」を完結させる目標時間 **1〜2 時間**（最大でも 半日 / 4 時間以内）
- **Budget Alert ¥1,000 維持**

---

## 5. Secret Manager 方針

### 5.1 Secret 設計

| Secret 名 | 値の形式 | アクセス経路 |
|---|---|---|
| `DATABASE_URL` | `postgres://vrcpb_app:<password>@/vrcpb?host=/cloudsql/<project>:asia-northeast1:vrcpb-api-verify` | Cloud Run の `--update-secrets=DATABASE_URL=DATABASE_URL:latest` で env vars に注入 |

### 5.2 セキュリティ運用

- **Secret 値（パスワード含む完全な DSN）を チャット / ログ / コミットメッセージに貼らない**（`security-guard.md`）
- 値の登録は `gcloud secrets versions add DATABASE_URL --data-file=-` で **stdin 経由**（コマンドラインに値を出さない）
- `gcloud run services update --set-env-vars=DATABASE_URL=...` で **平文渡しは禁止**。必ず `--update-secrets` で Secret Manager から mount
- パスワード生成は `openssl rand -base64 32` 等で生成、ターミナル履歴に残らないよう `read -s` 等で読み込む
- Cloud Run の Service Account に `roles/secretmanager.secretAccessor` を付与（既定 Compute SA に対して）

### 5.3 検証後の削除

```sh
# Secret 全 version 削除
gcloud secrets delete DATABASE_URL --quiet
```

---

## 6. migration 適用方針

### 6.1 候補比較

| 案 | 内容 | 利点 | 欠点 |
|---|---|---|---|
| **案 A** | **ローカル WSL から Cloud SQL Auth Proxy 経由で `goose up`** | 既存の goose ワークフロー（PR3 / PR6 で確立）をそのまま使える、設定追加最小 | Auth Proxy バイナリの導入が必要 |
| 案 B | Cloud Run Jobs で migration 実行 | 本番運用と整合（後の Outbox Worker と同じパターン）| Cloud Run Jobs / image / IAM の追加設定が必要、検証用には重い |
| 案 C | 一時的な Cloud Build / コンテナ実行 | CI 化しやすい | 検証用には重い、Cloud Build 有効化も必要 |

### 6.2 推奨: **案 A（Cloud SQL Auth Proxy + ローカル `goose up`）**

理由:
- M2 短時間検証では最小工数
- PR3 / PR6 で確立した `goose up` フローと同じ操作感
- Auth Proxy はバイナリ 1 つを `~/bin/` 等に置けば動く（root 権限不要）
- Cloud SQL Jobs / Cloud Build は本番運用フェーズで導入

### 6.3 Cloud SQL Auth Proxy の導入確認

```sh
# v2 を推奨（v1 は EOL に近い）
# 本書段階ではダウンロードしない。実施 PR で:
curl -o ~/bin/cloud-sql-proxy https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.x.x/cloud-sql-proxy.linux.amd64
chmod +x ~/bin/cloud-sql-proxy
~/bin/cloud-sql-proxy --version
```

`~/bin/` を PATH に入れる（`.bashrc`）か、フルパスで起動。

### 6.4 Auth Proxy 起動コマンド（実施 PR で）

```sh
# port 5433 で待ち受け（ローカルの 5432 は docker-compose で使用中の可能性があるため避ける）
~/bin/cloud-sql-proxy \
  --port=5433 \
  <project>:asia-northeast1:vrcpb-api-verify
# バックグラウンドで動かしたい場合は & 付け、Ctrl+C で停止
```

### 6.5 ローカルから goose 適用

```sh
# Auth Proxy 経由のローカル DSN（パスワードは Secret から取り出すか、別途控える）
# DATABASE_URL は **stdin / 環境変数** で渡し、ターミナル履歴に残さない
read -s -p "DB password: " PG_PASSWORD; echo
export DATABASE_URL="postgres://vrcpb_app:${PG_PASSWORD}@localhost:5433/vrcpb?sslmode=disable"

# repo root から（cd 不使用）
go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 \
  -dir migrations postgres "$DATABASE_URL" status

# 正常なら up
go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 \
  -dir migrations postgres "$DATABASE_URL" up

# 確認
go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 \
  -dir migrations postgres "$DATABASE_URL" status
# 期待: 00001..00004 すべて Applied
```

migration `down` は **検証後 Cloud SQL 削除する前提**のため、必須ではない。ただし「`goose down` が動作することを 1 度だけ確認」しておくと、本番運用時の rollback 経路が担保できる（ユーザー判断）。

### 6.6 実 token を生成（200 経路確認用、§7）

migration 適用後、同じ Auth Proxy 経由で `backend/cmd/api` を一時起動し、Go の helper で draft / publish した photobook の raw token を生成する。**手順は §7 に集約**。

---

## 7. token exchange 200 経路確認方法

### 7.1 やってよいこと

- **一時的なローカル専用スクリプト**で `CreateDraftPhotobook` UseCase を呼んで raw token を取得（コミットしない）
- Go test 内で生成して `t.Log` に出さず、stdout / 環境変数経由で渡す
- 取得した raw token は **コピペ用に 1 度だけターミナルに表示**、すぐにシェル変数に格納してターミナルログから消す
- token を curl で `POST /api/auth/draft-session-exchange` に送り、200 + JSON body の確認

### 7.2 やってはいけないこと

- **本番 router に debug token 発行 endpoint を作る**
- dummy token で成功する経路を作る
- 固定 token を repo にコミット
- raw token を README / 作業ログ / コミットメッセージ / failure-log に貼る
- raw token を Cloud Run logs に出す（Backend は元から出さない構造、PR9c）
- ターミナル履歴 / シェル history file に raw token を残す（`HISTCONTROL=ignorespace` + 行頭スペース で抑止 or `unset HISTFILE` で当該セッションだけ history 無効化）

### 7.3 raw token を取得する手段（実施 PR で選択）

| 方法 | 内容 | コミット |
|---|---|---|
| (a) | Auth Proxy 経由で **ローカル backend を一時起動** + 既存テスト用ヘルパ + curl で `CreateDraftPhotobook` を呼ぶ HTTP endpoint を **一時的に追加してすぐ revert**（**非推奨**：本番コードに混入リスク） | NG |
| (b) | Auth Proxy 経由でローカル PostgreSQL のように扱い、**Go の test 経由でローカル backend が UseCase を実行 + raw token を環境変数で吐く一時プログラム**（PR10 で `/tmp/vrcpb_e2e/main.go` として作りかけ、internal package 制約で挫折）| NG |
| (c) | **Auth Proxy + 既存 PR9b の TX 統合 test を流す**（`TestPublishFromDraft_TxCommit_*` 等）。test 内で生成された raw token は test ログに出ない（assert に substring 検査のみ）。raw token を取り出すには test を一時的に変更する必要があり、これはコミットしない | NG |
| (d) | **Cloud SQL Auth Proxy 経由で psql 直接 INSERT** で `photobooks` テーブルに raw token hash を入れて、対応する raw token を別経路で生成。逆算は不可なので、**raw 生成 + hash 保存** の流れを 1 度に行う Go ワンショット script を `~/scratch/` 等の repo 外に置く | NG（repo 外） |

→ **推奨: (d) repo 外の Go ワンショット**。`~/scratch/vrcpb-token-gen/main.go` 等に置き、コミット対象外。raw token は stdout に出して、シェル変数にすぐ取り込む。

ただし内部 package 制約があるため、 `cd backend/` 経由でないと internal package を import できない（PR10 で経験済）。代替は backend 配下に `_e2e_test.go` を一時作成 → 実行 → 削除（コミットしない）。実施 PR で具体手順を確定する。

### 7.4 確認対象（curl）

```sh
URL=https://vrcpb-api-7eosr3jcfa-an.a.run.app
DRAFT_RAW='<取得した raw draft token、43 文字 base64url>'
MANAGE_RAW='<取得した raw manage token、43 文字 base64url>'

# 空 body
curl -sS -i -X POST -H "Content-Type: application/json" "${URL}/api/auth/draft-session-exchange" -d ''
# 期待: 400 bad_request

# 不正 token
curl -sS -i -X POST -H "Content-Type: application/json" \
  -d '{"draft_edit_token":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}' \
  "${URL}/api/auth/draft-session-exchange"
# 期待: 401 unauthorized

# 実 token
curl -sS -i -X POST -H "Content-Type: application/json" \
  -d "{\"draft_edit_token\":\"${DRAFT_RAW}\"}" \
  "${URL}/api/auth/draft-session-exchange"
# 期待: 200 + {"session_token":"...","photobook_id":"...","expires_at":"..."}
#       Set-Cookie: なし
#       Cache-Control: no-store

# manage も同様
curl -sS -i -X POST -H "Content-Type: application/json" \
  -d "{\"manage_url_token\":\"${MANAGE_RAW}\"}" \
  "${URL}/api/auth/manage-session-exchange"
```

### 7.5 確認後

- 取得した raw token / session_token は **シェル変数を `unset`** で破棄
- `history -d` でターミナル履歴から該当行を削除（または `unset HISTFILE` でセッション履歴無効化）
- スクラッチ Go プログラムは `rm -rf ~/scratch/vrcpb-token-gen/` で削除

---

## 8. Cloud Run update 手順案（実施 PR、まだ実行しない）

### 8.1 instance connection name 取得

```sh
INSTANCE_CONN=$(gcloud sql instances describe vrcpb-api-verify \
  --format='value(connectionName)')
echo "${INSTANCE_CONN}"
# 期待: project-1c310480-335c-4365-8a8:asia-northeast1:vrcpb-api-verify
```

### 8.2 Service Account に Secret アクセス権付与

```sh
PROJECT_ID=$(gcloud config get-value project)
SA="${PROJECT_ID//[^0-9]/}-compute@developer.gserviceaccount.com"
# 既定の Compute SA を使うと service account email は <project_number>-compute@... 形式
# 厳密には deploy 時に gcloud run services describe で取得
SA=$(gcloud run services describe vrcpb-api --region=asia-northeast1 --format='value(spec.template.spec.serviceAccountName)')

gcloud secrets add-iam-policy-binding DATABASE_URL \
  --member="serviceAccount:${SA}" \
  --role="roles/secretmanager.secretAccessor"

# Cloud SQL クライアント権限
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${SA}" \
  --role="roles/cloudsql.client"
```

### 8.3 Cloud Run revision update

```sh
gcloud run services update vrcpb-api \
  --region=asia-northeast1 \
  --add-cloudsql-instances="${INSTANCE_CONN}" \
  --update-secrets="DATABASE_URL=DATABASE_URL:latest"
# 既存 env (APP_ENV / ALLOWED_ORIGINS) は変更されない（add ベースのため）
```

### 8.4 revision 確認

```sh
gcloud run services describe vrcpb-api --region=asia-northeast1 \
  --format='yaml(spec.template.spec.containers[0].env,spec.template.metadata.annotations)' | head -30
# 期待:
#   - env に DATABASE_URL が secretKeyRef 経由で出る（値そのものは出ない）
#   - annotation に run.googleapis.com/cloudsql-instances: <project>:asia-northeast1:vrcpb-api-verify
```

### 8.5 rollback 方法（Cloud Run revision 戻し）

```sh
# 現 revision 一覧
gcloud run revisions list --service=vrcpb-api --region=asia-northeast1

# 旧 revision（DB なし、vrcpb-api-00001-q9h）に traffic 100%
gcloud run services update-traffic vrcpb-api --region=asia-northeast1 --to-revisions=vrcpb-api-00001-q9h=100
```

`/readyz` が再び 503 db_not_configured に戻ることを確認。

---

## 9. 検証チェックリスト

### 9.1 Before（DB 接続前、現在の状態）

- [x] `/health` 200
- [x] `/readyz` 503 db_not_configured
- [x] `/api/auth/draft-session-exchange` 404
- [x] `/api/auth/manage-session-exchange` 404

（既に `2026-04-26_backend-cloud-run-deploy-result.md` で確認済）

### 9.2 After（DB 接続後）

- [ ] `/health` 200 + `{"status":"ok"}`
- [ ] `/readyz` **200** + `{"status":"ready"}`
- [ ] `/api/auth/draft-session-exchange` 空 body → 400 bad_request
- [ ] 不正 token → 401 unauthorized
- [ ] 実 token → 200 + JSON body（session_token / photobook_id / expires_at）
- [ ] `/api/auth/manage-session-exchange` 同様 + `token_version_at_issue:0`
- [ ] response に `Set-Cookie` ヘッダが **含まれない**
- [ ] response に `Cache-Control: no-store`
- [ ] Cloud Run logs に raw token / session_token / hash / Cookie / DSN 平文が **出ていない**
- [ ] Cloud SQL 接続ログ（`gcloud sql instances logs`）で SASL 認証成功、エラーなし
- [ ] migration 4 本（`00001`〜`00004`）が `goose status` で `Applied`

### 9.3 Cleanup

- [ ] Cloud Run revision を DB なし版（`vrcpb-api-00001-q9h`）に戻す → `/readyz` 503 確認
- [ ] Cloud Run の `--add-cloudsql-instances` を削除（`gcloud run services update --remove-cloudsql-instances=...`）or revision 戻しで自動的に外れる
- [ ] Secret Manager `DATABASE_URL` を削除
- [ ] Cloud SQL instance を **削除**（`gcloud sql instances delete vrcpb-api-verify --quiet`）
- [ ] Cloud SQL Auth Proxy プロセスをローカルで kill
- [ ] スクラッチ Go プログラムを `rm`
- [ ] シェル変数 `DATABASE_URL` / `DRAFT_RAW` / `MANAGE_RAW` を `unset`
- [ ] 翌日 Billing 画面で課金が想定範囲内（数十円以内）であること確認

---

## 10. 費用見積もりとガード

### 10.1 Cloud SQL の課金

- `db-f1-micro`: 約 **$0.0150/時間**（~¥2.3/時間、為替 ¥150）
- ストレージ 10GB: 約 **$0.17/月**（時間割で実質 ~¥0.04/時間、無視可能）
- Backup: OFF のため $0
- **目安**: 1 時間 ~¥2.5、4 時間 ~¥10、24 時間 ~¥60

### 10.2 課金停止条件

- **削除のみ**で課金完全停止
- 停止（`gcloud sql instances patch ... --activation-policy=NEVER`）でも tier 課金は継続
- 検証後は **必ず削除**

### 10.3 ガード

- 作業セッションを開始したら、**§9 の Before / After / Cleanup を 1 セッションで完結**
- 中断する場合も Cloud SQL は **削除**してから終了
- 想定時間 1〜2 時間（最大 4 時間）
- Budget Alert ¥1,000 / 月は十分に維持できる規模
- 翌日 Billing 画面（`https://console.cloud.google.com/billing/`）で実課金確認

### 10.4 作成前のユーザー承認必須

- Cloud SQL は作成した瞬間から課金開始
- **§12 ユーザー判断事項に答えていただいてから実施 PR 着手**
- 作業セッション開始前に、削除予定時刻を共有

---

## 11. 失敗時切戻し

### 11.1 Cloud Run revision を DB なし版に戻す

```sh
gcloud run services update-traffic vrcpb-api \
  --region=asia-northeast1 \
  --to-revisions=vrcpb-api-00001-q9h=100
```

`/readyz` が 503 db_not_configured に戻ることを `curl` で確認。

### 11.2 DATABASE_URL Secret 削除

```sh
gcloud secrets delete DATABASE_URL --quiet
```

### 11.3 Cloud SQL instance 削除

```sh
gcloud sql instances delete vrcpb-api-verify --quiet
```

### 11.4 Cloud SQL 接続設定解除（Cloud Run 側）

revision を戻せば自動的に外れるが、明示的に外す場合:

```sh
gcloud run services update vrcpb-api \
  --region=asia-northeast1 \
  --remove-cloudsql-instances="<project>:asia-northeast1:vrcpb-api-verify" \
  --remove-secrets="DATABASE_URL"
```

### 11.5 確認

- `curl /readyz` → 503 db_not_configured
- `curl /api/auth/*` → 404
- `gcloud sql instances list`: 該当 instance が消えている
- `gcloud secrets list`: `DATABASE_URL` が消えている
- Cloud Run logs に Secret / token 漏洩なし

### 11.6 費用確認

翌日（or 数時間後）の Billing で:

- Cloud SQL 課金が「instance 削除時刻まで」で停止
- ストレージ課金も停止
- Budget Alert ¥1,000 を超えない

---

## 12. ユーザー判断事項

実施 PR 着手前に以下を確認してください。

### 12.1 Cloud SQL 短時間作成

- [ ] **作成して良い**（推奨、§3-§4）
- [ ] 作成しない（token exchange 200 経路の実環境確認は諦める。ローカル test で代替）

### 12.2 接続方式

- [ ] **案 A: Unix socket（`--add-cloudsql-instances`）**（推奨、§3.3）
- [ ] 案 B: Public IP
- [ ] 案 C: Private IP + VPC Connector

### 12.3 instance 名

- [ ] **`vrcpb-api-verify`**（推奨、検証用と明示）
- [ ] `vrcpb-api-db`（本番想定の名前で短期検証）
- [ ] その他

### 12.4 tier

- [ ] **`db-f1-micro`**（推奨、最小コスト）
- [ ] `db-g1-small`（より高性能、約 2.5 倍）

### 12.5 app user / database 名

- [ ] **app user: `vrcpb_app` / database: `vrcpb`**（推奨、既存 docker-compose と整合）
- [ ] 別名

### 12.6 Secret 名

- [ ] **`DATABASE_URL`**（推奨）
- [ ] 別名

### 12.7 migration 適用方式

- [ ] **案 A: ローカル WSL から Cloud SQL Auth Proxy 経由で `goose up`**（推奨、§6.2）
- [ ] 案 B: Cloud Run Jobs
- [ ] 案 C: Cloud Build / 一時コンテナ

### 12.8 検証後の Cloud SQL 削除

- [ ] **検証完了後すぐ削除**（推奨、§10）
- [ ] 数日残して観察したい（Budget Alert に注意）

### 12.9 実 token 取得方法

- [ ] **repo 外の Go ワンショット（`~/scratch/...`）**（推奨、§7.3 案 d）
- [ ] backend/ 配下に `_e2e_test.go` を一時追加 → 実行 → 削除（コミットしない）
- [ ] 200 経路は確認しない（400 / 401 のみで OK とする）

### 12.10 `goose down` の動作確認

- [ ] **1 度確認する**（推奨、本番運用時の rollback 担保）
- [ ] 確認しない（検証後 Cloud SQL 削除なので必須ではない）

---

## 13. 実施しないこと（再掲）

本書は **計画書のみ**。以下は実行しない:

- Cloud SQL instance 作成
- Cloud SQL Auth Proxy ローカル導入
- Secret Manager `DATABASE_URL` 登録
- Cloud Run service update（`--add-cloudsql-instances` / `--update-secrets`）
- migration 適用
- 実 token 生成
- Cloud Run Domain Mapping（`api.vrc-photobook.com`）
- Cloudflare DNS 変更
- Workers Custom Domain 設定 / Workers deploy
- SendGrid 設定 / Turnstile 本番 widget 作成 / R2 変更
- 既存 spike リソース削除
- Budget Alert 変更

---

## 14. 関連ドキュメント

- [M2 Backend Cloud Run Deploy 計画](./m2-backend-cloud-run-deploy-plan.md)
- [Backend Cloud Run deploy 実施結果（2026-04-26）](../../harness/work-logs/2026-04-26_backend-cloud-run-deploy-result.md)
- [M2 Domain Mapping 実施計画](./m2-domain-mapping-execution-plan.md)
- [M2 ドメイン購入チェックリスト + 購入記録](./m2-domain-purchase-checklist.md)
- [M2 実装ブートストラップ計画](./m2-implementation-bootstrap-plan.md)
- [`backend/migrations/`](../../backend/migrations/)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
- [Cloud Run + Cloud SQL（Unix socket）公式](https://docs.cloud.google.com/sql/docs/postgres/connect-run)
- [Cloud SQL Auth Proxy 公式](https://docs.cloud.google.com/sql/docs/postgres/sql-proxy)
