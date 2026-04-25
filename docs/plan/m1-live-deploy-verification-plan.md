# M1 実環境デプロイ検証計画 — Cloudflare Workers + Cloud Run

> **位置付け**: M1 で残っている「実環境依存」の未確認事項を、安全な順序で検証するための**計画書**。
> 本書は計画であって本実装ではない。記載された手順は実行前に本書をレビューし、ユーザーの承認後に着手する。
>
> **対象 PoC**: `harness/spike/frontend/`（OpenNext + Cloudflare Workers）と `harness/spike/backend/`（Go chi + pgx + sqlc + R2 + Turnstile + Outbox）。本実装ディレクトリ `frontend/` / `backend/` は M2 まで触らない。
>
> **作成日**: 2026-04-26
>
> **上流**:
> - `docs/plan/m1-spike-plan.md` §13.0 / §13.1 残作業
> - `docs/adr/0001-tech-stack.md` §M1 検証結果（OpenNext / Cloud Run）
> - `docs/adr/0003-frontend-token-session-flow.md` §13 未解決事項 U2（Cookie Domain）
> - `docs/adr/0004-email-provider.md`（SendGrid 第一・Mailgun 第二・AWS SES 運用不可）
> - `docs/adr/0005-image-upload-flow.md` §R2 接続検証
> - `docs/design/cross-cutting/outbox.md` §13.3 / `reconcile-scripts.md` §3.7.5（U11）
> - `harness/spike/frontend/README.md` / `harness/spike/backend/README.md`
> - `.agents/rules/safari-verification.md`（必須ルール）
>
> **重要前提**:
> - **AWS SES は採用不可**（Amazon 側申請不通過、`docs/adr/0004-email-provider.md` 再選定後）。本書では AWS SES に関する記述は意図的に含めない。
> - メールは **SendGrid 第一・Mailgun 第二**。M1 実デプロイでは **SendGrid 実送信は行わない**。
> - **本実装ディレクトリには触れない**。M2 を汚さない。

---

## 1. 検証目的

M1 ローカル PoC で成立した構成（Frontend / Backend / R2 / Turnstile / Outbox）を**実環境**で再現し、以下の前提が崩れないことを確認する。崩れた場合は本書 §13「失敗時の判断」で代替案へ切り替え、ADR を更新する。

| 確認したいこと | 影響を受ける ADR / 設計 |
|---|---|
| Cloudflare Workers + Static Assets binding（OpenNext）が `*.workers.dev` 上で SSR / OGP / Cookie / redirect / ヘッダ制御を維持する | ADR-0001 §Frontend Deploy / ADR-0003 §SSR時のCookie検証 |
| Cloud Run（東京）で Backend PoC が起動し、`/healthz` / `/readyz` / R2 系 / Turnstile 系 / Outbox 系のエンドポイントが本番相当の挙動を返す | ADR-0001 §Backend Deploy / §クロスクラウド構成 |
| Frontend Workers → Backend Cloud Run の API 疎通（CORS / Origin / SameSite=Strict / Secure）が macOS Safari と iPhone Safari で成立する | ADR-0003 §CSRF / `.agents/rules/safari-verification.md` |
| Cookie Domain（U2）の最終判断材料が揃う（共通親ドメイン案 / 同一オリジン化案 / Cookie 共有しない案の比較） | ADR-0003 §13 未解決事項 U2 |
| Cloud Run（東京リージョン）↔ R2 のレイテンシが p50 200ms 以下を満たす | ADR-0001 §クロスクラウド構成 |
| Cloud Logging で slog JSON が安全に扱われる（severity マッピング / 禁止フィールドが出ていない） | ADR-0001 §Backend Deploy |
| Secret Manager から R2 / DB / Turnstile の Secret を Cloud Run に注入できる | ADR-0001 / ADR-0005 / `.agents/rules/security-guard.md` |
| Cloud Run Jobs + Cloud Scheduler で `outbox-worker --once` / `--retry-failed` を起動できる（U11） | `reconcile-scripts.md` §3.7.5 / §11 U11 |
| iPhone Safari の **24 時間後 / 7 日後 ITP 影響評価**を開始できる（Cookie 残存追跡の起点を作る） | ADR-0003 §M1 検証結果 §継続観察項目 |

---

## 2. 実環境構成

```
┌─────────────────────────┐         ┌──────────────────────────┐
│ User (iPhone Safari /   │  HTTPS  │ Cloudflare Workers       │
│ macOS Safari / Chrome)  │────────▶│ + Static Assets binding  │
│                         │         │ (Frontend PoC, OpenNext) │
│                         │         │ *.workers.dev            │
└─────────────────────────┘         └──────────┬───────────────┘
                                               │ fetch (CORS / credentials: include)
                                               ▼
                                    ┌──────────────────────────┐
                                    │ Cloud Run (asia-northeast1│
                                    │  Backend PoC, Go chi)    │
                                    │ *.run.app                │
                                    └─────┬─────────┬──────────┘
                                          │         │
                  Secret Manager 注入     │         │  presigned PUT 発行 / HeadObject
                                          ▼         ▼
                                    ┌──────────┐  ┌────────────────────────┐
                                    │ Cloud SQL│  │ Cloudflare R2          │
                                    │ Postgres │  │ vrcpb-spike-live バケット│
                                    │ (検証用) │  │ (S3 互換 API)          │
                                    └──────────┘  └────────────────────────┘

                                    ┌──────────────────────────┐
                                    │ Cloud Run Jobs           │
                                    │  (outbox-worker --once)  │
                                    │  Cloud Scheduler 起動     │
                                    └──────────────────────────┘
```

### 構成要素

- **Frontend**: `harness/spike/frontend/` を OpenNext (`@opennextjs/cloudflare`) でビルド → Cloudflare Workers + Static Assets binding にデプロイ。URL は `*.workers.dev`（カスタムドメインは M1 では使わない）。
- **Backend**: `harness/spike/backend/` を multi-stage Dockerfile（`golang:1.24-alpine` → `gcr.io/distroless/static-debian12:nonroot`、PoC で 12.4MB / 21.6MB 確認済み）でビルド → Artifact Registry → Cloud Run（`asia-northeast1`、東京リージョン）。
  - **イメージ構成（M1 第一案）**: 同一 Docker image に `cmd/api` と `cmd/outbox-worker` の 2 バイナリを含める。Cloud Run service は `api` を `--command` で起動、Cloud Run Jobs は `outbox-worker` を `--command` で起動。詳細は §6 Step 11.0 を参照。
- **DB**: Cloud SQL PostgreSQL 16（検証用、最小サイズ）。Cloud SQL Auth Proxy 経由 / Public IP + Cloud SQL connector のいずれかを採用（§6 デプロイ順序で決める）。
  - **費用観点（重要）**: Cloud SQL は本書で **最も課金リスクが高いリソース**。M1 では「最初から常時起動」しない方針を推奨する。詳細は **§4 費用見積もりと課金ガードレール**を参照。
  - **代替**: Cloud Run コンテナ内 sqlite を使わず PostgreSQL を保つ。Cloud SQL の起動コストが M1 検証に対して過剰な場合のみ、Compute Engine 上の単一インスタンス PostgreSQL を一時 DB として代替（§13 で扱う）。
- **R2**: M1 PoC とは**別バケット** `vrcpb-spike-live`（または同名のまま M1 検証専用）を使う。M1 PoC で使ったバケット / Token は R2 接続 PoC 完了後に Revoke 済みのため、新規 Token を発行する。
- **Secret Manager**: GCP Secret Manager。Cloud Run / Cloud Run Jobs に環境変数として注入。
- **SendGrid**: M2 早期で扱う。**M1 実デプロイでは実送信しない**（アカウント審査・DKIM/SPF/DMARC 設定・テスト送信は M2 早期の別タスクで実施）。本書では「Cloud Run 環境変数 `SENDGRID_API_KEY` のキー名だけは予約しておくが、Secret 値の登録は M1 ではしない」立て付け。
- **Turnstile**: **M1 実デプロイでは本番 widget を作成しない**。実環境検証は Cloudflare 公式テスト secret（always-pass / always-fail）で済ませ、token → upload-verification-session → consume の流れだけ確認する。本番 widget は Workers hostname または独自ドメインが確定したあと、M2 早期に Cloudflare Dashboard → Turnstile → Add widget で発行する（widget 名・hostname・Managed/Non-interactive の比較は M2 タスクで決める）。
- **Cloud Run Jobs + Cloud Scheduler**: `outbox-worker` バイナリを Job として登録し、Scheduler から `--once` / `--retry-failed` を起動できるかを確認する（U11）。

---

## 3. 使用する一時リソース

### 3.1 Cloudflare 側

| リソース | 用途 | 命名・備考 |
|---|---|---|
| **Workers + Static Assets binding** | Frontend PoC のホスティング | 名前: `vrcpb-spike-live`（仮）。`wrangler.jsonc` の `name` を変更してデプロイ |
| **`*.workers.dev` URL** | デプロイ後の検証用 URL | 例: `vrcpb-spike-live.<account>.workers.dev`。カスタムドメインは M1 では設定しない |
| **R2 バケット** | M1 PoC とは別の検証用バケット（または M1 PoC 用バケットを再利用） | 例: `vrcpb-spike-live`。M1 PoC のバケット（テストオブジェクト削除済）を再利用しても可 |
| **R2 API Token**（Object Read & Write、対象バケット限定、短期 TTL） | Cloud Run から R2 にアクセス | M1 PoC で使った Token は Revoke 済みのため、新規発行 |
| **Turnstile widget**（M1 では原則作成しない） | Frontend / Backend の Turnstile 実機検証 | **M1 では本番 widget を原則作成しない**。実環境デプロイ検証では Cloudflare 公式テスト secret（`1x0000000000000000000000000000000AA` / `2x0000000000000000000000000000000AA`）を使い、token / session / redirect の流れだけ確認する。本番 widget の作成は **Workers hostname または独自ドメイン方針が確定した後、M2 早期タスク**として実施（参照: M1 plan §10.5 / `harness/spike/backend/README.md` §次工程） |

### 3.2 Google Cloud 側

| リソース | 用途 | 備考 |
|---|---|---|
| **GCP プロジェクト** | M1 検証専用プロジェクト | 既存プロジェクトを再利用可。本実装プロジェクトとは分離するのが望ましい |
| **Artifact Registry** | Backend Docker image の格納 | リポジトリ名例: `vrcpb-spike` / リージョン: `asia-northeast1` |
| **Cloud Run service** | Backend API（`harness/spike/backend/cmd/api`） | サービス名例: `vrcpb-spike-api` / リージョン: `asia-northeast1` / min-instances=0 / max-instances=2 |
| **Cloud Run Jobs** | `outbox-worker --once` / `--retry-failed` の検証 | Job 名例: `vrcpb-spike-outbox-worker` |
| **Cloud Scheduler** | Cloud Run Jobs の cron 起動（U11 検証） | スケジュール例: `*/10 * * * *`（10 分ごと、検証用に短くする） |
| **Cloud SQL（PostgreSQL 16）** | 検証用 DB | インスタンス名例: `vrcpb-spike-pg` / リージョン: `asia-northeast1` / 最小スペック（db-f1-micro 相当） |
| **Secret Manager** | R2 / DB / Turnstile Secret の格納 | §5 のキー一覧に従って登録 |
| **Cloud Logging** | slog JSON のパース確認 | severity マッピング / 禁止フィールド漏洩チェック |
| **サービスアカウント** | Cloud Run / Cloud Run Jobs から Secret Manager / Cloud SQL にアクセス | 最小権限。`roles/secretmanager.secretAccessor` / `roles/cloudsql.client` のみ付与 |

---

## 4. 費用見積もりと課金ガードレール

> **このセクションは技術検証計画より優先する**。M1 実デプロイは本番運用ではなく短期検証であり、課金事故が起きた瞬間にプロジェクト全体が止まる。実行前に本セクションのチェックリストを完了させること。

### 4.1 基本方針

- M1 実環境デプロイは **本番運用ではなく短期検証**。目標は数時間〜1 日程度の短期確認
- 費用目標は **数百円〜数千円以内**（概算。正確な金額は実行直前に **Google Cloud Pricing Calculator** / 各公式料金ページで再確認する）
- 料金プランは変わる可能性があるため、本書には金額を断定的に書かず「概算」「実行前に再確認」と扱う
- 本セクションの方針と矛盾するステップを §6 デプロイ順序に見つけたら、§6 ではなく本セクションを優先する

### 4.2 低リスク寄りの項目

| 項目 | リスク評価 | 注意点 |
|---|---|---|
| **Cloud Run service**（リクエストベース課金、無料枠あり） | 低 | `min-instances=0` を厳守。`min-instances=1` 以上にすると常時課金リスクが上がるため M1 では避ける。max-instances も 2 程度に抑える |
| **Artifact Registry**（保存容量に対する課金、小容量は無料枠内） | 低 | M1 検証では Docker image 数個（合計 1GB 未満）に収める。検証完了後に古い image を削除 |
| **Cloud Scheduler**（1 ジョブ単位の少額課金） | 低（ただし削除忘れ注意） | 検証用に作った Scheduler は **検証後に必ず削除**。残すと Cloud Run Jobs の起動が継続し、付随コストが積み上がる |
| **Secret Manager**（少数 Secret なら無料枠内に近い） | 低 | 検証 Secret は M1 検証完了後に削除 |
| **Cloudflare R2**（保存容量＋クラス A/B オペレーション課金、エグレス無料） | 低 | テスト画像は数枚（合計 1MB 未満想定）。検証完了後にテストオブジェクト削除、API Token を Revoke |
| **Safari / iPhone Safari 実機検証** | なし | クラウド側に追加費用は発生しない（端末側の通信料のみ） |

### 4.3 高リスク寄りの項目

| 項目 | リスク評価 | 詳細 |
|---|---|---|
| **Cloud SQL（PostgreSQL）** | **高** | インスタンス時間課金がある（最小スペックでも稼働時間に応じて課金）。数時間なら小さいが、**数日〜月単位で停止し忘れると費用が積み上がる**。M1 では「最初から常時起動」を避け、後段で必要時だけ短時間起動する方針（§4.4）を取る |
| **Cloud Run Jobs + Cloud Scheduler の組み合わせ** | 中（削除忘れに注意） | Job 自体の費用は Cloud Run と同枠で小さいが、**Scheduler の削除忘れで定期起動が継続**するリスクがある。検証完了後に Scheduler を必ず削除 |
| **Cloud Logging の保管容量** | 低〜中 | M1 検証期間ではほぼ無料枠内。ただしデバッグ用に大量出力すると保管課金が出る可能性がある。slog INFO レベルで運用 |

### 4.4 Cloud SQL 利用方針（段階化）— 費用ガード

現行 §6 デプロイ順序（旧 Step 3）では Cloud SQL を比較的早い段階で準備する流れになっているが、**費用ガードのため以下の段階化を推奨**する。実行時には §6 のステップ順を本セクションの段階化で上書きする。

#### Step A: DB 非依存 / DB 最小依存の検証を先に行う（Cloud SQL 未起動）

対象（本書 §6 の該当ステップを順序入れ替えで先に実施）:

- Cloud Run service 起動（DB 接続なしで起動できる構成、または DB 接続を遅延化する暫定設定）
- `/healthz`（DB 不要なので 200 が返る前提）
- R2 系エンドポイント（`/sandbox/r2-headbucket` / `/sandbox/r2-list` / `/sandbox/r2-presign-put` / `/sandbox/r2-headobject`）
- Turnstile（公式テスト secret / mock）
- CORS / Origin（`/sandbox/origin-check`）
- Frontend Workers との結合（`/integration/backend-check`）
- Safari / iPhone Safari 実機確認

> **注意**: 現行 Backend PoC は起動時に pgx pool の初期化を行うため、DB が無いと `/readyz` は 503 になる（`/healthz` は 200）。Step A の段階では `/readyz` は 503 を許容し、必要なら `DATABASE_URL` を空にして起動するか、Cloud SQL 起動前は `/readyz` を未確認のまま進める運用とする。

#### Step B: DB が必要な検証だけ Cloud SQL を短時間起動する

Step A で DB 非依存検証が完了したら、Cloud SQL を起動して以下を実施:

- `/readyz`（DB 接続込み）
- goose で migration 適用（00001 / 00002 / 00003）
- Outbox sandbox API の動作確認
- Cloud Run Jobs（`outbox-worker --once` / `--retry-failed`）
- Cloud Scheduler 連携
- DB 接続込みの結合確認

**Cloud SQL の連続稼働時間は短く抑える**（目安: 検証 1 セッションあたり 1〜2 時間以内）。完了次第 Step C へ。

#### Step C: DB 検証完了後、Cloud SQL を停止または削除する

- 即停止: `gcloud sql instances patch vrcpb-spike-pg --activation-policy=NEVER`
- 完全削除: `gcloud sql instances delete vrcpb-spike-pg`
- どちらを取るかは「再開頻度の見込み」で判断（同日中に再検証するなら停止、しばらく触らないなら削除）
- **停止 / 削除を確認するまで M1 検証完了扱いにしない**

### 4.5 予算アラート（Budget Alert）

実行前に GCP Billing で **Budget Alert を必ず設定**する。

- 推奨閾値（M1 短期検証の例）:
  - 500 円
  - 1,000 円
  - 3,000 円
- 通知先メールアドレス（プロジェクトのオーナー / 自分のメール）が正しく登録されていることを送信テストで確認
- **予算アラートが設定されていない状態で Cloud SQL を立てない**（高リスク項目なので一段ガードを増やす）
- 予算超過時は即座に対象リソースを停止 / 削除し、§13 失敗時の判断へ

### 4.6 実行前チェックリスト

§6 デプロイ順序を実行する **直前** に以下を全てチェックする。

- [ ] GCP Billing が有効であることを確認（Billing アカウントが紐付いている）
- [ ] **Budget Alert を設定**（500 円 / 1,000 円 / 3,000 円のいずれか以上、通知先メール確認済み）
- [ ] Cloud SQL を最初から使うか、§4.4 Step A → Step B の段階化で後段に分けるかを決定（推奨: 段階化）
- [ ] Cloud Run service の `min-instances=0` を確認（誤って 1 以上にしない）
- [ ] Cloud Scheduler の削除タイミングを §15 後片付け手順に組み込んでいることを確認
- [ ] Cloud SQL の停止 / 削除タイミングを §15 後片付け手順に組み込んでいることを確認
- [ ] R2 API Token Revoke 手順を §15 後片付け手順で確認
- [ ] Artifact Registry の検証 image 削除手順を §15 後片付け手順で確認
- [ ] Secret Manager の検証 Secret 削除手順を §15 後片付け手順で確認
- [ ] Pricing Calculator で本書 §3 リソース一覧の概算を一度通し計算した（金額を本書には書かないが、ユーザー側で確認した記録は残す）

### 4.7 公式料金ページ（実行前に再確認）

料金は変わる可能性があるため、本書に金額を断定的に書かず、実行前に以下を再確認する。

- Cloud Run pricing: https://cloud.google.com/run/pricing
- Cloud SQL pricing: https://cloud.google.com/sql/pricing
- Artifact Registry pricing: https://cloud.google.com/artifact-registry/pricing
- Cloud Scheduler pricing: https://cloud.google.com/scheduler/pricing
- Secret Manager pricing: https://cloud.google.com/secret-manager/pricing
- Cloudflare R2 pricing: https://www.cloudflare.com/developer-platform/r2/

---

## 5. 必要な Secret 一覧（**値は本書に書かない**）

すべて Secret Manager に登録し、Cloud Run / Cloud Run Jobs から環境変数として参照する。値は `.env.local` / Secret Manager 上でのみ管理し、チャット・ログ・README に出さない（`.agents/rules/security-guard.md` / ADR-0005）。

| 環境変数名 | 取得元 | M1 実デプロイで必要か |
|---|---|---|
| `DATABASE_URL` | Cloud SQL Auth Proxy 経由 / Cloud SQL connector の DSN（`postgres://...?sslmode=...`） | **必要** |
| `R2_ACCOUNT_ID` | Cloudflare Dashboard → R2 → Overview | **必要** |
| `R2_ACCESS_KEY_ID` | Cloudflare Dashboard → R2 → Manage R2 API Tokens（短期 TTL、対象バケット限定） | **必要** |
| `R2_SECRET_ACCESS_KEY` | 同上 | **必要** |
| `R2_BUCKET_NAME` | 検証用バケット名（例: `vrcpb-spike-live`） | **必要** |
| `R2_ENDPOINT` | `https://<R2_ACCOUNT_ID>.r2.cloudflarestorage.com` | **必要** |
| `TURNSTILE_SECRET_KEY` | **M1 では Cloudflare 公式テスト secret（always-pass: `1x0000000000000000000000000000000AA` / always-fail: `2x0000000000000000000000000000000AA`）を使う**。本番 widget の secret は M2 早期で扱う | **必要**（実機検証では always-pass を使う） |
| `ALLOWED_ORIGINS` | カンマ区切り。Workers の `*.workers.dev` URL を登録 | **必要** |
| `APP_ENV` | `staging` 等 | **必要**（値は固定で OK、Secret ではない） |
| `PORT` | Cloud Run が自動注入（`8080`） | 不要（Cloud Run 側で自動） |
| `IP_HASH_SALT_V1` | M1 はダミー固定値。本実装では Secret Manager の本番ソルト | **任意**（M1 では一旦不要、Backend PoC が要求しないなら設定しない） |
| `SESSION_TOKEN_HASH_PEPPER` | 候補のみ（必要なら検討） | **任意**（M1 では不要） |
| `SENDGRID_API_KEY` | M2 早期に SendGrid アカウント作成後に発行 | **不要**（M1 実デプロイでは登録しない。キー名だけ予約） |

**Secret 取り扱いルール**（本書外でも徹底）:

- Secret 値は `.env.local`（gitignore 対象）／ Secret Manager のいずれにのみ存在させる。
- Claude Code は値を `cat` / `printenv` / 値を引数にした `grep` などで表示しない（M1 PoC で確立した運用を継続）。
- Cloud Run / Cloud Run Jobs のログ（slog JSON）に Secret が出ていないことを `grep` で確認する。
- 検証完了後、API Token は **Revoke**、検証用 Secret は Secret Manager から削除する（§15 後片付け手順）。

---

## 6. デプロイ順序

**重要**: 各ステップで失敗 / 仕様差異が見つかったら即停止し、§13 で代替案を検討する。「動くまで雑に進める」のは禁止。**実行前に §4 費用見積もりと課金ガードレールの実行前チェックリストを完了させる**こと。

### Step 1. GCP プロジェクト / リージョン確認

- 検証用プロジェクト ID を確定（既存 or 新規）。
- リージョン: **`asia-northeast1`（東京）固定**（クロスクラウドレイテンシ計測を意味あるものにするため）。
- 必要 API 有効化: Cloud Run / Cloud Run Jobs / Cloud Scheduler / Cloud SQL Admin / Secret Manager / Artifact Registry / Cloud Logging。

### Step 2. Artifact Registry 準備

- リポジトリ作成: `gcloud artifacts repositories create vrcpb-spike --repository-format=docker --location=asia-northeast1`
- 認証: `gcloud auth configure-docker asia-northeast1-docker.pkg.dev`

### Step 3. Cloud SQL（または一時 DB）準備

- 第一案: Cloud SQL PostgreSQL 16、最小スペック。Public IP + Cloud SQL Auth Proxy または Cloud SQL connector for Go。
- 起動後、`harness/spike/backend/migrations/` を `goose` で適用（00001 / 00002 / 00003）。
- 接続方式は **Cloud SQL connector for Go** を第一候補（pgx と素直に組める）、難航したら Auth Proxy を sidecar で動かす案へ切替。
- **代替**: Cloud SQL の起動コストが過剰な場合、Compute Engine `e2-micro` に PostgreSQL 16 を一時的に立てる（§13 で詳細）。

### Step 4. Secret Manager 登録

- §5 一覧の Secret を登録（**M1 では SendGrid は登録しない**）。
- Cloud Run / Cloud Run Jobs 用サービスアカウントに `roles/secretmanager.secretAccessor` を付与。

### Step 5. Backend PoC を Cloud Run へデプロイ

```sh
# ビルド・push（実コマンド例。実行前にユーザー承認）
docker build -t asia-northeast1-docker.pkg.dev/<PROJECT>/vrcpb-spike/api:m1-live harness/spike/backend
docker push asia-northeast1-docker.pkg.dev/<PROJECT>/vrcpb-spike/api:m1-live

# Cloud Run デプロイ
gcloud run deploy vrcpb-spike-api \
  --image=asia-northeast1-docker.pkg.dev/<PROJECT>/vrcpb-spike/api:m1-live \
  --region=asia-northeast1 \
  --platform=managed \
  --allow-unauthenticated \
  --min-instances=0 --max-instances=2 \
  --set-env-vars=APP_ENV=staging \
  --set-secrets=DATABASE_URL=DATABASE_URL:latest,\
R2_ACCOUNT_ID=R2_ACCOUNT_ID:latest,\
R2_ACCESS_KEY_ID=R2_ACCESS_KEY_ID:latest,\
R2_SECRET_ACCESS_KEY=R2_SECRET_ACCESS_KEY:latest,\
R2_BUCKET_NAME=R2_BUCKET_NAME:latest,\
R2_ENDPOINT=R2_ENDPOINT:latest,\
TURNSTILE_SECRET_KEY=TURNSTILE_SECRET_KEY:latest,\
ALLOWED_ORIGINS=ALLOWED_ORIGINS:latest
```

> **注意**: 上記は計画上の例。Secret 名やリビジョンは Secret Manager 登録後に確定。

### Step 6. Backend ヘルスチェック

- `curl https://vrcpb-spike-api-xxx.run.app/healthz` → 200
- `/readyz` → 200（DB 接続確認）
- `/sandbox/r2-headbucket` → 200（R2 接続確認）
- `/sandbox/r2-list` → 200
- `/sandbox/turnstile/verify`（mock または always-pass で）→ 200

### Step 7. Frontend PoC を Cloudflare Workers へデプロイ

- `harness/spike/frontend/wrangler.jsonc` の `name` を `vrcpb-spike-live` に変更（M1 検証用）。
- `harness/spike/frontend/.env.local` の `NEXT_PUBLIC_API_BASE_URL` を Step 5 で得た Cloud Run URL に設定（環境別の `wrangler.jsonc` `vars` で上書きしてもよい）。
- ビルド + デプロイ:
  ```sh
  npm --prefix harness/spike/frontend run cf:build
  npx wrangler deploy --cwd harness/spike/frontend
  ```
- デプロイ後の `*.workers.dev` URL を控える。

### Step 8. Backend の `ALLOWED_ORIGINS` 更新

- Step 7 で得た `*.workers.dev` URL を `ALLOWED_ORIGINS` Secret に追記（Secret Manager の新リビジョン作成 + Cloud Run の Secret 参照を `:latest` で再展開、または `gcloud run services update --set-secrets`）。

### Step 9. Frontend からの動作確認

- `https://vrcpb-spike-live.<account>.workers.dev/` にアクセス（chrome / Safari / iPhone Safari）。
- `/p/sample-slug`（OGP / noindex / Referrer-Policy）。
- `/draft/sample-draft-token` → `/edit/sample-photobook-id` redirect。
- `/manage/token/sample-manage-token` → `/manage/sample-photobook-id` redirect。
- `/integration/backend-check` で Cloud Run の `/healthz` / `/sandbox/session-check` / `/sandbox/origin-check` を実機 fetch。

### Step 10. Safari / iPhone Safari 実機確認

§8 のチェックリストを **macOS Safari 最新 + iPhone Safari 最新** で実施。`.agents/rules/safari-verification.md` の必須項目を踏襲。

### Step 11. Cloud Run Jobs + Cloud Scheduler 検証

#### Step 11.0 イメージ構成方針（着手時に確定）

Cloud Run Jobs では、M1 検証時点で以下のどちらかを着手時に選ぶ。

- **案1（M1 第一案）: 同一 Docker image に `api` / `outbox-worker` の 2 バイナリを含める**
  - Cloud Run service（`vrcpb-spike-api`）は `api` バイナリを起動
  - Cloud Run Jobs（`vrcpb-spike-outbox-worker`）は `outbox-worker` バイナリを `--command` で起動
  - 利点: 同一イメージなので Artifact Registry の管理が 1 本で済む / ビルド時間短縮 / Secret 注入の Cloud Run / Cloud Run Jobs 両方で同じ環境変数体系を使える
- **案2: `outbox-worker` 専用の Dockerfile または専用ビルドターゲットを用意する**
  - Cloud Run service と Cloud Run Jobs で別イメージ・別タグ
  - 利点: バイナリサイズが小さくなる / 役割が明確
  - 欠点: M1 検証ではメリットが薄い

**M1 第一案は「案1: 同一 image に 2 バイナリを含める」**。ただし、現行の `harness/spike/backend/Dockerfile` は `cmd/api` 単体ビルドのため、**Cloud Run Jobs 着手前に Dockerfile を最小修正**して `cmd/outbox-worker` も同じ image にビルドし、両バイナリを `/usr/local/bin/spike-api` / `/usr/local/bin/spike-outbox-worker`（など）として配置する。`ENTRYPOINT` は M1 では設定せず、Cloud Run service 側 / Cloud Run Jobs 側で `--command` を明示的に指定する。

#### Step 11.1 outbox-worker 用 Cloud Run Jobs 登録

- 上記の Dockerfile 最小修正後、`api:m1-live` 同一タグでビルド・push（service 用と Jobs 用で同じイメージを使う）。
- Cloud Run Jobs に登録（**案1 を採用、`--command` で `outbox-worker` バイナリを明示**）:
  ```sh
  gcloud run jobs create vrcpb-spike-outbox-worker \
    --image=asia-northeast1-docker.pkg.dev/<PROJECT>/vrcpb-spike/api:m1-live \
    --region=asia-northeast1 \
    --command="/usr/local/bin/spike-outbox-worker" \
    --args="--once,--limit=50" \
    --set-secrets=DATABASE_URL=DATABASE_URL:latest
  ```
  > `--command` / `--args` の値は Dockerfile 最小修正で決まったバイナリパスに合わせる。`--retry-failed` を起動したい場合は別の Cloud Run Jobs として `vrcpb-spike-outbox-worker-retry` を分けてもよい。

- Cloud Scheduler から呼び出し:
  ```sh
  gcloud scheduler jobs create http vrcpb-spike-outbox-trigger \
    --schedule="*/10 * * * *" \
    --uri="https://<region>-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/<PROJECT>/jobs/vrcpb-spike-outbox-worker:run" \
    --http-method=POST \
    --oauth-service-account-email=<sched-sa>@<PROJECT>.iam.gserviceaccount.com
  ```
- 10 分待って Cloud Run Jobs の実行履歴を確認、`outbox_failed_retry` が動いていることを `/sandbox/outbox/list` で確認。

### Step 12. 結果を ADR / M1 計画に反映

- ADR-0001 §M1 検証結果に「Cloud Run 実環境デプロイ成立 / コールドスタート / Cloud Logging slog JSON / 東京 ↔ R2 レイテンシ」を追記。
- ADR-0003 §13 U2 に Cookie Domain 検証結果（§7 のどの案を採るか）を追記。
- `reconcile-scripts.md` §11 U11 に Cloud Run Jobs + Cloud Scheduler 採用可否を追記。
- `m1-spike-plan.md` §13.0 の残作業をクローズ。

### Step 13. 不要リソース停止・削除

§15「後片付け手順」を実行。

---

## 7. Cookie Domain U2 の検証案

ADR-0003 §13 未解決事項 U2「Frontend = `*.workers.dev` / Backend = `*.run.app` の異なるホスト構成下での Cookie Domain」を確定するための比較案。M1 ではどこまで実機検証し、どこから M2 判断に回すかを明示する。

### 案A: 共通親ドメインに Frontend / Backend を載せ、Cookie Domain で共有

- 構成: `app.example.com`（Frontend Workers）と `api.example.com`（Backend Cloud Run）を共通親ドメイン下に置き、Cookie に `Domain=.example.com` を付ける。
- 利点: ADR-0003 の token → session 交換方式をそのまま使える。`SameSite=Strict` で CSRF 一次対策が効き続ける。
- 欠点: **独自ドメインの取得・DNS 設定が必要**（Cloudflare DNS / Workers Custom Domains / Cloud Run Domain mappings）。M1 検証期間に独自ドメインを用意するのは過剰。
- M1 での扱い: **M1 では実装しない**。M2 早期に独自ドメインを取得した時点で再検証する。

### 案B: Backend を Workers の `/api/*` 経由でプロキシし、同一オリジン化

- 構成: Frontend Workers の同一 Worker 内で `/api/*` パスを Cloud Run へリバースプロキシ（`fetch()` で Cloud Run URL に転送）。Cookie は `*.workers.dev` 上で Frontend が発行・受信し、`/api/*` 経由のリクエストは同一オリジンになる。
- 利点: Cookie Domain の問題が消える（同一オリジンになる）。`SameSite=Strict` を維持しつつ Frontend/Backend の物理分離も保てる。
- 欠点: Workers が API 経路を抱えるためレイテンシが +1 hop（Workers → Cloud Run）。Workers の制限（CPU 時間 / リクエストサイズ）に巻き込まれる。CORS は不要だが、Workers の `fetch` が Cloud Run の `Set-Cookie` をそのまま転送するか / Cookie 名衝突が起きないかを実機確認する必要がある。
- M1 での扱い: **設計検討は M1 で行うが、Workers 経由のプロキシ実装は M1 では避ける**（PoC コードを増やしすぎない）。実装を伴う検証は M2 へ。

### 案C: Frontend が Backend を直接呼び、Cookie 共有はしない

- 構成: Frontend `*.workers.dev` と Backend `*.run.app` をそのまま使う。Cookie は Frontend ホストにのみ発行。Backend への API 呼び出しは `credentials: omit` で Cookie を渡さず、Frontend Server Component / Route Handler 側で session 検証 → Backend へ署名付きヘッダ等で委譲する。
- 利点: 独自ドメインが不要、最小構成で動く。
- 欠点: ADR-0003 の「Backend が Cookie session で認可」前提が崩れる。Backend の認可方式を Cookie 以外に再設計する必要がある（HMAC ヘッダ / 短命 JWT 等）。**ADR-0003 全面見直しに近い**。
- M1 での扱い: **本案は採用しない方針**。本書では「案C を選ぶ場合は ADR-0003 の根本変更が必要」と明示するに留める。

### M1 での確定範囲

| 項目 | M1 で行うこと | M2 へ持ち越すこと |
|---|---|---|
| 案A 実機検証 | 行わない（独自ドメインなし） | 独自ドメイン取得後に実機検証 |
| 案B 実機検証 | 行わない（Workers プロキシ実装は PoC 範囲外） | Workers `/api/*` プロキシ PoC を別タスクで実施 |
| 案C 採用判断 | **不採用と確定**（ADR-0003 根本変更を避ける） | — |
| `*.workers.dev` ↔ `*.run.app` 別オリジン下での **Cookie 共有不可の実機確認** | **実施**（CORS / `credentials: include` でも Cookie が渡らないことを Safari / iPhone Safari で確認）| — |
| Cookie Domain（U2）の最終結論 | **「M2 で独自ドメインを取得して案A を採る」を一次方針として ADR-0003 §13 U2 に追記** | M2 早期に独自ドメイン取得 → 案A 実機検証 → U2 解消 |

---

## 8. Safari / iPhone Safari 検証項目

`.agents/rules/safari-verification.md` の必須項目に従う。**Chrome / Edge のみで完了とすることは禁止**。

### 8.1 macOS Safari（最新）

- [ ] `https://<workers.dev>/` トップページ表示
- [ ] `/p/sample-slug` でレスポンスヘッダ確認: `X-Robots-Tag: noindex, nofollow` / `Referrer-Policy: strict-origin-when-cross-origin`
- [ ] `/p/sample-slug` View Source で OGP メタタグ（`og:title` / `og:description` / `og:image` / `twitter:card` / `meta name="robots" noindex`）が出力
- [ ] `/draft/sample-draft-token` → `/edit/sample-photobook-id` に redirect、URL から token が消える
- [ ] redirect 後 `draft session found` 表示
- [ ] Web Inspector → Storage → Cookies で `vrcpb_draft_sample-photobook-id` の HttpOnly / Secure / SameSite=Strict / Path=/ を目視
- [ ] `/manage/token/sample-manage-token` → `/manage/sample-photobook-id` redirect、`manage session found` 表示
- [ ] `/edit/*` / `/manage/*` のレスポンスヘッダで `Referrer-Policy: no-referrer`
- [ ] `/integration/backend-check` で `GET /healthz`（200）、`POST /sandbox/origin-check (credentials: include)`（200 + `origin_allowed:true`）
- [ ] `GET /sandbox/session-check (credentials: include)` で **別オリジンのため Cookie が渡らないこと**を Network タブで確認（U2 検証の核）
- [ ] ページ再読込後も session found 維持
- [ ] レスポンスヘッダの `X-Robots-Tag` 重複出力がない（OpenNext 既知問題、middleware 一本化済みであることの確認）

### 8.2 iPhone Safari（最新、可能なら 1 世代前も）

- [ ] 上記 macOS Safari の全項目を実機で再現
- [ ] redirect 後の表示が成立
- [ ] ページ再読込後も session 維持
- [ ] モバイルレイアウトに破綻がない（タップ可能領域 / 横画面）
- [ ] OGP プレビュー（X / iMessage プレビュー）で `og:image` の絶対 URL が解決される（`metadataBase` を `NEXT_PUBLIC_BASE_URL` に設定済みであることを確認）

### 8.3 継続観察（運用開始後、24h / 7 日後に再アクセス）

- [ ] **24 時間後の Cookie 残存**（ITP 影響評価）
- [ ] **7 日後の Cookie 残存**（ITP 影響評価）
- [ ] プライベートブラウジングでの動作（参考）
- [ ] iOS Safari 1 世代前 / iPad Safari（推奨）

> **記録**: 24h / 7 日後の継続観察は本デプロイ完了日を起点として `harness/work-logs/` に記録する。`.agents/rules/safari-verification.md` §履歴 に追記する。

### 8.4 禁止事項（再掲）

- Chrome / Edge のみで検証完了とすること
- iPhone Safari を「Chrome 互換」として扱うこと
- Cookie 値・raw token を console / 画面 / スクリーンショットに出すこと

---

## 9. R2 実環境確認

ローカル PoC（M1 priority 4）で実接続成立済み（コミット `83cf628`）。Cloud Run から再確認する。

### 9.1 確認項目

- [ ] `GET /sandbox/r2-headbucket` 200
- [ ] `GET /sandbox/r2-list` 200
- [ ] `POST /sandbox/r2-presign-put`（filename / content_type / byte_size）→ 200 / 519 bytes 程度の `upload_url`
- [ ] presigned URL に対する `curl -X PUT`（**ローカル PoC と同じく `byte_size` 宣言値と body サイズを一致させる**）→ 200
- [ ] `GET /sandbox/r2-headobject?key=<storage_key>` → 200 + `content_length` / `content_type` / `etag`
- [ ] バリデーション 8 ケース（10MB+ / SVG / GIF / path traversal / prefix invalid / 存在しない key / byte_size=0 / filename 空）が期待通り
- [ ] **Cloud Logging に Secret / presigned URL / storage_key が出ていない**（ログ漏洩 grep）

### 9.2 Cloud Run 東京 ↔ R2 レイテンシ計測

- [ ] `time curl https://vrcpb-spike-api-xxx.run.app/sandbox/r2-headbucket` を 5 回計測、p50 を記録（目標: p50 < 200ms）
- [ ] `time curl https://vrcpb-spike-api-xxx.run.app/sandbox/r2-presign-put -d '...'` を 5 回計測（presign は内部 SDK 演算なので R2 通信は発生しない想定だが、HeadObject / List は通信あり）
- [ ] HeadObject の p50 を記録

### 9.3 既知事項の再確認

- [ ] `aws-sdk-go-v2` の presign は `Content-Length` を SignedHeaders に含めるため、宣言サイズと実 PUT サイズが不一致だと `403 SignatureDoesNotMatch` になる挙動を Cloud Run 上でも確認（M2 本実装の Frontend 側で「`File.size` を `byte_size` に直結」を必須化する根拠を厚くする）

---

## 10. Outbox / Cloud Run Jobs 確認

### 10.1 Cloud Run Jobs での `outbox-worker --once` 起動

> **前提**: §6 Step 11.0 のとおり、M1 第一案として **同一 Docker image に `api` / `outbox-worker` の 2 バイナリを含める**方針を採用する。Cloud Run service と Cloud Run Jobs はどちらも同じイメージを参照し、`--command` で起動するバイナリを切り替える。

- [ ] Dockerfile が `cmd/api` / `cmd/outbox-worker` の両方をビルドし、最終ステージに 2 バイナリを配置している（Jobs 着手前の最小修正がコミット済み）
- [ ] Cloud Run Jobs に登録できる（`--command="/usr/local/bin/spike-outbox-worker"` / `--args="--once,--limit=50"`）
- [ ] `gcloud run jobs execute vrcpb-spike-outbox-worker --region=asia-northeast1` で手動起動 → 成功
- [ ] 実行ログ（Cloud Logging）に `claimed=N / processed=M / failed=K` が slog INFO で出る
- [ ] 失敗時の exit code が **non-zero** で Cloud Run Jobs 側で「失敗」として記録される
  - `cmd/outbox-worker` 側で DB 接続失敗・claim 失敗時に `os.Exit(1)` を返す実装になっているかを確認、なっていなければ最小修正
- [ ] payload / Secret / `last_error` 詳細がログに漏れていない

### 10.2 Cloud Scheduler 連携

- [ ] Cloud Scheduler から HTTP 起動できる（OAuth サービスアカウント経由）
- [ ] `*/10 * * * *` で 10 分後 / 20 分後 に Cloud Run Jobs の実行履歴が増えている
- [ ] スケジューラ重複起動が発生したケースで `FOR UPDATE SKIP LOCKED` が衝突しないこと（PoC では同プロセス並列で確認済み、Cloud Run Jobs では多重起動シナリオ自体がレアだが観察する）

### 10.3 U11 解消可否の判断

- [ ] Cloud Run Jobs + Cloud Scheduler が **MVP 基本案として成立**するなら U11 を解消（`reconcile-scripts.md` §11 を「Accepted」に更新）
- [ ] advisory lock の必要性は **M2 へ持ち越し可**と判定（PoC で SKIP LOCKED が機能していることを根拠に）
- [ ] 指数バックオフ・保持期間（processed=30 日）クリーンアップ・実 `ImageIngestionRequested` ハンドラは **M2 / M6 持ち越し**と確認

---

## 11. Email Provider の扱い

**AWS SES は採用不可**（Amazon 申請落ち、`docs/adr/0004-email-provider.md` 再選定後）。

### 11.1 M1 実環境デプロイでは SendGrid 実送信を行わない

- 理由: アカウント審査・DKIM/SPF/DMARC 設定・送信ドメイン認証は M1 検証期間に押し込むには重い。本書は「インフラ実環境を Workers + Cloud Run + R2 + Cloud SQL で動かす」ことに集中する。
- M1 では `SENDGRID_API_KEY` を Secret Manager に**登録しない**（キー名のみ予約）。

### 11.2 M2 早期で実施する SendGrid PoC（参照: ADR-0004 §M2 以降の TODO）

- [ ] SendGrid アカウント作成 + 審査通過（Amazon SES と同様の運用上不可リスクがあるため早期確認）
- [ ] 送信ドメイン認証 + DKIM / SPF / DMARC DNS 設定
- [ ] scoped API Key（Mail Send 専用）発行 → Secret Manager 登録
- [ ] 1 通テスト送信（自宅アドレス）
- [ ] Email Activity Feed / API ログに本文・管理 URL が見えないことを実機確認
- [ ] bounce / complaint webhook 受信エンドポイントの設計（Outbox + ManageUrlDelivery）
- [ ] iPhone Safari でメール内管理 URL を開き、token → session cookie → redirect 成立確認（`.agents/rules/safari-verification.md`）

### 11.3 SendGrid アカウント審査が落ちた場合のフォールバック

- 第二候補 Mailgun へ切替（ADR-0004 §第二候補）
- DKIM / SPF / DMARC を Mailgun 用に再設定
- Domain settings で **retention 0 day** を選択し、本文非保持を実現
- ADR-0004 を「第一 Mailgun / 第二 — 」に更新する手順は同 ADR §M2 以降の TODO §「SendGrid → Mailgun 切替判断 / 手順テンプレート」に従う

---

## 12. 成功条件

以下を**全て満たした時点で M1 実環境デプロイ検証は完了**とする。1 つでも満たさない場合は §13 失敗時の判断へ。

| # | 条件 |
|---|---|
| 1 | Cloud Run `/healthz` が 200、`/readyz` が 200（DB 接続成立） |
| 2 | Frontend Workers URL（`*.workers.dev`）でトップ / `/p/{slug}` / `/draft/*` / `/manage/token/*` の各ページが表示される |
| 3 | OGP メタタグ / `noindex` / `Referrer-Policy` / `X-Robots-Tag` が期待値で出る（重複なし） |
| 4 | `/draft/{token}` → `/edit/{photobook_id}` redirect が macOS Safari / iPhone Safari で成立 |
| 5 | redirect 後の `vrcpb_draft_*` Cookie が HttpOnly / Secure / SameSite=Strict / Path=/ で発行されている |
| 6 | Cloud Run から R2 への HeadBucket / List / presign / PUT / HeadObject 全成立、p50 < 200ms |
| 7 | `/integration/backend-check` で Workers → Cloud Run の CORS / preflight が成立、`origin_allowed:true` を返す |
| 8 | Cloud Run Jobs で `outbox-worker --once` / `--retry-failed` が動作、Cloud Scheduler 起動も成立 |
| 9 | Cloud Logging に Secret / token / presigned URL / Cookie 値が出ていない（grep 0 ヒット） |
| 10 | Cookie Domain（U2）の暫定方針が決まる（M1 では「案A を M2 で採る」を一次方針として確定） |
| 11 | iPhone Safari の **24 時間後 / 7 日後 ITP 観察**の起点（デプロイ日時）を `harness/work-logs/` に記録 |
| 12 | §15 後片付け手順が実行可能な状態である（`.workers.dev` / Cloud Run / Cloud Run Jobs / Cloud Scheduler / Cloud SQL / Artifact Registry / Secret Manager / R2 のリソースを停止・削除する手順がメモされている） |

---

## 13. 失敗時の判断

| 症状 | 第一対応 | 代替案 |
|---|---|---|
| **Cloud Run から R2 へのレイテンシが p50 > 500ms** | アクセスパターンの再計測（warm vs cold）。リージョンを `asia-northeast1` で固定しているか確認 | Cloud Run のリージョン変更は MVP 性能制約に引っかかるため避ける。R2 jurisdiction 設定の有無を確認。継続的に遅い場合は ADR-0001 §クロスクラウド構成に「実測 p50=Xms」を追記し、M2 で改善案検討 |
| **Cookie Domain が期待通り動かない**（`*.workers.dev` ↔ `*.run.app` の Cookie 共有を試みて失敗） | これは**期待通りの挙動**（別オリジン下では Cookie 共有不可）。むしろ U2 確定材料として記録 | 案A（独自ドメイン + 共通親ドメイン）を M2 早期で実施するため、独自ドメイン取得の Issue を起こす |
| **Safari ITP で session が短時間で消える** | ローカル PoC と同じテスト経路で再現するか確認 | ADR-0003 §13 U2 に「ITP で `*.workers.dev` 単独でも消える」事実を記録。案A 独自ドメイン取得を前倒し |
| **Workers → Cloud Run の CORS / preflight が複雑すぎる** | `ALLOWED_ORIGINS` を Workers の URL に正確に登録できているか / `Access-Control-Allow-Credentials: true` が返っているか確認 | 案B（Workers `/api/*` プロキシ）の検討を前倒し（ただし M1 では実装しない、Issue 起票のみ） |
| **Cloud SQL 接続が重い / 起動コストが過剰** | Cloud SQL connector for Go から Auth Proxy sidecar 方式へ切替を試す | Compute Engine `e2-micro` 上の単一 PostgreSQL に切替（**SLA なし、検証用のみ**）。M2 本実装では Cloud SQL に戻す |
| **Cloud Run Jobs + Cloud Scheduler が複雑 / 課金が読めない** | Scheduler の頻度を `0 * * * *`（1 時間ごと）に下げる | reconcile-scripts.md §3.7.5 §代替「GitHub Actions cron」「Cloud Run service 同居 worker」を ADR で再評価。M2 で最終決定する旨を記録 |
| **OpenNext build / deploy が `*.workers.dev` 上で失敗** | `wrangler 4.x` のバージョン / `nodejs_compat` フラグ / `assets` バインディング設定を確認 | ローカルで再現、`harness/failure-log/` に記録、必要なら ADR-0001 を「Frontend Deploy 第二候補」に切替（案: Cloud Run 上の Next.js Node 起動）。これは M1 失敗扱い |
| **Backend Cloud Run コールドスタートが 5 秒超** | コンテナサイズ / pgx 初期化 / chi 起動の各時間を slog で計測 | min-instances=1 を試す（課金増を許容）。それでも遅ければ ADR-0001 §結果デメリットを更新 |

---

## 14. 後片付け手順

**すべての検証が完了したら、課金停止と漏洩リスク低減のため以下を必ず実施する**。手順は順不同だが、Secret は最後に削除する（途中で必要になる可能性があるため）。

> **検証完了の判定**: 以下のすべてが満たされるまで「M1 実環境デプロイ検証完了」とは扱わない。
> - **Cloud SQL を停止または削除した**（最も課金リスクが高い、§4.3）
> - **Cloud Scheduler を削除した**（残すと定期起動が継続）
> - **Cloud Run Jobs を削除した**
> - **Artifact Registry の検証 image を削除した**
> - **Cloud Run service の `min-instances=0` を確認した**（誤って 1 以上にしたまま放置していない）
> - **R2 API Token を Revoke した**
> - **Secret Manager の検証 Secret を削除した**
> - **Budget Alert は必要なら残してよいが、検証用リソースが消えていることを Billing 画面で目視確認した**

### 14.1 Cloudflare 側

- [ ] Cloudflare Dashboard → Workers & Pages → `vrcpb-spike-live` Worker を停止または削除
- [ ] R2 バケット（`vrcpb-spike-live` または検証用）のテストオブジェクトを削除
- [ ] R2 → Manage R2 API Tokens で検証用 Token を **Revoke**
- [ ] Turnstile 本番 widget は **M1 では発行しない方針**のため、通常は対応不要。誤って発行した場合のみ widget を削除する

### 14.2 Google Cloud 側

- [ ] **Cloud SQL `vrcpb-spike-pg` インスタンスを停止**（`gcloud sql instances patch ... --activation-policy=NEVER`）**または削除**（**最優先**、§4.3 高リスク項目）
- [ ] **Cloud Scheduler `vrcpb-spike-outbox-trigger` を削除**（残すと毎 10 分定期起動が継続するため、Cloud SQL と同じく最優先）
- [ ] Cloud Run Jobs `vrcpb-spike-outbox-worker` を削除
- [ ] Cloud Run service `vrcpb-spike-api` を停止（min-instances=0 にしておけば追加対応不要だが、不要なら削除）。**`min-instances` が誤って 1 以上のまま残っていないことを必ず確認**
- [ ] Artifact Registry の検証イメージ（`api:m1-live` 等）を削除（保存容量課金の停止）
- [ ] Cloud Logging の検証ログは保持期限内に自動削除されるが、含まれる Secret / token がないことを確認した上で必要なら手動削除
- [ ] Secret Manager の検証用 Secret を削除（`DATABASE_URL` / `R2_*` / `TURNSTILE_SECRET_KEY` / `ALLOWED_ORIGINS`）
- [ ] サービスアカウントの IAM 権限が残っていないことを確認、検証用 SA は削除
- [ ] **Billing 画面で検証用リソースが消えていることを目視確認**（Budget Alert は残してよい）

### 14.3 ローカル

- [ ] `harness/spike/frontend/.env.local` / `harness/spike/backend/.env.local` の検証用値をクリア（`*.workers.dev` URL / Cloud Run URL を残してもよいが、Secret 値は消す）
- [ ] `harness/work-logs/` に検証完了サマリを記録（`harness/work-logs/2026-XX-XX_m1-live-deploy-verification.md` のような命名）
- [ ] M1 計画 `docs/plan/m1-spike-plan.md` §13.0 / §13.1 のチェックボックスを更新

### 14.4 SendGrid（M1 では触らない）

- [ ] M1 実デプロイ範囲では SendGrid アカウント作成・API Key 発行を行わないため、後片付け対象外

---

## 15. 参照

- `docs/plan/m1-spike-plan.md` §13.0 / §13.1（M1 残作業）
- `docs/adr/0001-tech-stack.md` §M1 検証結果 / §クロスクラウド構成
- `docs/adr/0003-frontend-token-session-flow.md` §13 未解決事項 U2
- `docs/adr/0004-email-provider.md`（SendGrid 第一・Mailgun 第二・AWS SES 運用不可）
- `docs/adr/0005-image-upload-flow.md` §R2 接続検証
- `docs/design/cross-cutting/outbox.md` §13.3 / §14（Outbox / Reconciler PoC）
- `docs/design/cross-cutting/reconcile-scripts.md` §3.7 / §11 U11
- `harness/spike/frontend/README.md` §結合検証 PoC
- `harness/spike/backend/README.md` §Cloud Run へ進めるうえでの未確認事項
- `.agents/rules/safari-verification.md`
- `.agents/rules/security-guard.md`
- `.agents/rules/feedback-loop.md`

---

## 16. 履歴

| 日付 | 変更 |
|---|---|
| 2026-04-26 | 初版作成。M1 実環境デプロイ検証計画として、Cloudflare Workers + Cloud Run + Cloud SQL + R2 + Cloud Run Jobs + Cloud Scheduler の検証手順 / Cookie U2 案 A/B/C / Safari チェックリスト / 後片付け手順を整理 |
| 2026-04-26 | レビュー反映：(1) Cloud Run Jobs の outbox-worker 起動方式を「同一 Docker image に api/outbox-worker の 2 バイナリを含めて --command で切替」を M1 第一案として §2 / §6 Step 11.0 / §10.1 に明記。Dockerfile の最小修正が前提であることも追記。(2) Turnstile 本番 widget は M1 では原則作成しない方針を §2 / §3.1 / §5 / §14.1 で再強調。本番 widget は M2 早期タスクへ（節番号は本日 2 度目の修正で再採番済み） |
| 2026-04-26 | 費用ガード追記：新セクション **§4 費用見積もりと課金ガードレール**を追加（基本方針 / 低リスク・高リスク分類 / Cloud SQL の段階化 Step A→B→C / Budget Alert 必須化 / 実行前チェックリスト / 公式料金ページ）。既存 §4〜§15 を §5〜§16 に再採番。§14 後片付け手順に費用観点の最優先項目（Cloud SQL 停止 / Scheduler 削除 / Cloud Run min-instances=0 確認 / Billing 目視確認）を追加 |
