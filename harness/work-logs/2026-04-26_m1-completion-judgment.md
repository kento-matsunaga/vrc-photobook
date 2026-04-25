# 2026-04-26 M1 完了判定表

> 上流: `docs/plan/m1-spike-plan.md` §13 M1 完了条件 / `docs/plan/m1-live-deploy-verification-plan.md` §12 成功条件
> 関連 work-logs:
> - `harness/work-logs/2026-04-26_m1-live-deploy-verification.md`（Step 7〜10 実機検証ログ）
> 関連 failure-logs（5 件、本日記録）:
> - `2026-04-25_cloudflare-next-on-pages-deprecated.md`（既存）
> - `2026-04-26_wsl-cwd-drift-recurrence.md`
> - `2026-04-26_gcloud-install-verification-mismatch.md`
> - `2026-04-26_sudo-noninteractive-shell-limit.md`
> - `2026-04-26_gcp-account-billing-mismatch.md`
> - `2026-04-26_cloud-run-healthz-intercepted.md`

## 結論（先出し）

| 項目 | 結論 |
|---|---|
| **M1 を完了扱いにできるか** | ✅ **Yes**（条件付き） |
| 条件 | (a) Cloud SQL / Cloud Run Jobs / Cloud Scheduler / SendGrid 実送信 / Turnstile 本番 widget / U2 独自ドメインを **M2 早期へ持ち越す**ことに合意。(b) 24h / 7 日後 Safari ITP は **継続観察項目**として M1 完了をブロックしない |

> M1 の目的は「本番実装前に、主要な技術的前提が実環境で成立するかを確認すること」。本番相当の全リソースを作ることではない。

---

## 1. M1 で確認済みのこと

### 1.1 Frontend（Next.js 15 + OpenNext + Cloudflare Workers）

| 項目 | 状態 | 根拠コミット / ログ |
|---|---|---|
| Next.js 15 App Router + OpenNext (`@opennextjs/cloudflare`) で実 deploy 成立 | ✅ | Workers Version `08756f3d-...`（`afa3c7b` 後）、URL `https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev` |
| Cloudflare Workers + Static Assets binding 構成成立 | ✅ | `wrangler.jsonc` で `assets binding=ASSETS`、Worker Startup 25 ms |
| `*.workers.dev` 上で SSR 動作（`x-opennext: 1` 付き） | ✅ | `/`、`/p/sample-slug` ともに 200、SSR HTML を確認 |
| `generateMetadata` で OGP / Twitter card メタタグを動的出力 | ✅ | `<meta property="og:title">` 等が動的生成 |
| **`og:image` の絶対 URL 解決**（C 修正後）| ✅ | `metadataBase` 設定で `https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev/og-sample.png` に展開（`afa3c7b`） |
| `<meta name="robots" content="noindex, nofollow">` HTML 出力 | ✅ | 全ページで確認 |
| **`X-Robots-Tag: noindex, nofollow` 一回のみ**（D 修正後）| ✅ | `next.config.mjs` の `headers()` 削除で重複解消（`afa3c7b`） |
| `Referrer-Policy` 出し分け（通常 strict-origin / sensitive no-referrer）| ✅ | middleware.ts で出し分け、Workers 上でも成立 |
| `/draft/{token}` → 302 + Set-Cookie + redirect → `/edit/{photobook_id}` | ✅ | URL から token 消去、Cookie 発行確認 |
| `/manage/token/{token}` → 302 + Set-Cookie + redirect → `/manage/{photobook_id}` | ✅ | 同上 |
| Cookie 属性: HttpOnly / Secure / SameSite=Strict / Path=/ | ✅ | DevTools 目視確認（macOS Safari / iPhone Safari）|
| Cookie Max-Age: draft 7 日 / manage 24 時間 | ✅ | ADR-0003 通り |
| `NEXT_PUBLIC_API_BASE_URL` の Workers bundle inline | ✅ | client chunk 内に Cloud Run URL inline 確認 |
| **macOS Safari 実機**（Workers 実環境）| ✅ 大きな問題なし | 2026-04-26 |
| **iPhone Safari 実機**（Workers 実環境）| ✅ 大きな問題なし | 2026-04-26 |

### 1.2 Backend（Go chi + pgx + sqlc + Cloud Run）

| 項目 | 状態 | 根拠 |
|---|---|---|
| Cloud Run service（asia-northeast1）に digest 指定で deploy 成立 | ✅ | revision `vrcpb-spike-api-00003-mxl`、URL `https://vrcpb-spike-api-7eosr3jcfa-an.a.run.app` |
| 同一 Docker image に `cmd/api` + `cmd/outbox-worker` の 2 バイナリを含める構成（M1 第一案） | ✅ | コミット `9e6a4f6` Dockerfile / `36a1e93` `.dockerignore`、image 26.3MB |
| `.dockerignore` で `.env.local` / Secret 系を build context から除外 | ✅ | コミット `36a1e93` |
| **Cloud Run / 本番監視用 `/health` 200**（GFE intercept 対策）| ✅ | コミット `e8f7029`、`failure-log/2026-04-26_cloud-run-healthz-intercepted.md` |
| `/healthz` はローカル互換用に並存登録、Cloud Run 上では GFE 404 で許容 | ✅ | 同上 |
| `/readyz` 503 `db_not_configured`（DB 未設定時の想定挙動） | ✅ | Step A 段階化通り |
| **DB 未設定でも Backend が起動継続**（pool nil / queries nil ガード）| ✅ | main.go / config.go / 各 handler で実装済 |
| Secret Manager から R2 系 5 個の Secret を Cloud Run に注入（`--set-secrets` + Secret 単位の IAM Accessor）| ✅ | A-3 / A-5-1、IAM bindings 確認済 |
| Cloud Run から R2 への HeadBucket / List / presigned PUT / HeadObject 動作 | ✅ | A-5-2 検証で R2 系 4 endpoint すべて 200 |
| graceful shutdown / slog JSON / Cloud Logging severity マッピング | ✅ | 起動ログが Cloud Logging に正しく取り込まれることを確認 |

### 1.3 Frontend / Backend 結合

| 項目 | 状態 | 根拠 |
|---|---|---|
| Cloud Run `ALLOWED_ORIGINS` を Workers URL に設定（`--update-env-vars`、既存 Secret 維持）| ✅ | A-5 → Step 8、revision 003 |
| 起動ログ「`CORS allowed origins configured count=1`」 | ✅ | 反映確認 |
| `POST /sandbox/origin-check` from Workers Origin → 200 `{"origin_allowed":true}` | ✅ | Access-Control-Allow-Origin / Allow-Credentials / vary: Origin 反射 |
| 許可外 Origin → 403 `{"error":"origin_not_allowed"}` | ✅ | CORS ヘッダ無し（ブラウザは応答破棄）|
| OPTIONS preflight 成立（`Allow-Methods` / `Allow-Headers` / `Max-Age`）| ✅ | 204 応答確認 |
| `/integration/backend-check` で `/health` 200 / `origin-check` 200 を実機確認 | ✅ | Step 9（ユーザー側ブラウザ実機）|
| `session-check` が Workers ↔ Cloud Run 別オリジンで `false / false` | ✅ **想定通り**（U2 確定材料）| ADR-0003 §M1 検証結果に追記済 |
| ブラウザコンソールに CORS エラーなし | ✅ | Step 9 |

### 1.4 R2（Cloudflare R2 / S3 互換 API）

| 項目 | 状態 | 根拠 |
|---|---|---|
| Cloudflare Dashboard で R2 バケット作成 + 短期 API Token 発行 | ✅ | M1 priority 4（コミット `83cf628`）|
| HeadBucket / List / presigned PUT 発行 / R2 直接 PUT / HeadObject | ✅ | ローカル PoC で 8 ケース成立、Cloud Run 上で再確認（A-5）|
| `Content-Length` を SignedHeaders に含める挙動（不一致時 403 SignatureDoesNotMatch）の実機把握 | ✅ | 本実装の Frontend で `byte_size = file.size` 直結を必須化する根拠 |
| バリデーション 8 ケース（10MB+ / SVG / GIF / path traversal / prefix invalid / 存在しない key / byte_size=0 / filename 空）| ✅ | すべて期待通り |
| ログ漏洩 0 ヒット（presigned URL / R2 認証情報 / storage_key）| ✅ | grep で確認 |

### 1.5 Turnstile + upload_verification_session

| 項目 | 状態 | 根拠 |
|---|---|---|
| Cloudflare 公開サンドボックス secret（always-pass / always-fail）で siteverify 実呼び出し | ✅ | コミット `53fa568` |
| 32B 乱数 → base64url → SHA-256 → bytea として `upload_verification_sessions` に SHA-256 のみ保存 | ✅ | raw token は DB に残さない |
| 単一 SQL UPDATE 5 条件 AND の原子消費 | ✅ | 逐次 21 回 / 100 並列 race（`success=20 / forbidden=80`）|
| 拒否カテゴリの集約（`consume_rejected` 単一）| ✅ | 攻撃者が脱落条件判別不可 |
| secret / verification_session_token / Turnstile token のログ漏洩 0 | ✅ | grep で確認 |

> **Note**: Turnstile 系は DB 必須（`upload_verification_sessions` への INSERT）。Cloud Run 上での実機検証は Cloud SQL を立てる Step B 以降のため、M1 では DB 必要 endpoint の実 deploy 検証は **未実施**。siteverify 自体（外部 API 呼び出し）はローカルで成立確認済。

### 1.6 Outbox + 自動 reconciler 起動基盤

| 項目 | 状態 | 根拠 |
|---|---|---|
| `outbox_events` migration / sqlc 生成 / sandbox API / `cmd/outbox-worker --once` / `--retry-failed` | ✅ | コミット `91be6de` |
| CTE + `FOR UPDATE SKIP LOCKED` で claim、enqueue → claim → processed / failed → retry | ✅ | ローカル PoC |
| 30 件 + 2 並列 process-once で event_ids overlap=0、最終 processed=30 | ✅ | 二重処理防止確認 |
| `ImageIngestionRequested` event_type を扱える | ✅ | 5 件で processed |
| payload / Secret / `last_error` のログ漏洩 0 | ✅ | grep |

> **Note**: Cloud Run Jobs / Cloud Scheduler 経由の起動は **未実施**（DB 必須 + Cloud SQL 起動を伴うため、計画書 §4.4 段階化 Step B 以降）。

### 1.7 Email Provider 選定（ADR-0004 Accepted）

| 項目 | 状態 | 根拠 |
|---|---|---|
| 4 候補比較完了（Resend / AWS SES / SendGrid / Mailgun）+ 参考 2 候補（Postmark / Cloudflare Email Service）| ✅ | コミット `cf56df1` → `20f22f5` |
| **第一候補 SendGrid / 第二候補 Mailgun**（AWS SES は Amazon 申請落ちで運用不可、再選定済）| ✅ | ADR-0004 Accepted |
| 実装方針整理（EmailSender ポート抽象化 / 本文最小化 / Outbox payload に管理 URL を入れない 等）| ✅ | ADR-0004 §実装方針 |

> **Note**: SendGrid 実送信 PoC（API Key 発行 + 1 通テスト送信 + bounce/complaint webhook）は **未実施**（M2 早期 TODO）。

### 1.8 GCP / インフラ / ガードレール

| 項目 | 状態 | 根拠 |
|---|---|---|
| GCP プロジェクト `project-1c310480-335c-4365-8a8`（My First Project）/ Billing 有効 | ✅ | Step 1 |
| 必須 API 7 個 + 既存 logging を有効化 | ✅ | Step 1（artifactregistry / run / iam / cloudresourcemanager / secretmanager / sqladmin / cloudscheduler）|
| Artifact Registry リポジトリ `vrcpb-spike` 作成、image push（9.2MB）| ✅ | A-2 / A-4、無料枠 0.5GB の 1.8% |
| Secret Manager R2_* 5 個（active version 1 ずつ）| ✅ | A-3、約 $0.30/month |
| Secret Manager IAM 付与は **Secret 単位** で `secretAccessor`（プロジェクト全体に付けない）| ✅ | A-5-1、最小権限 |
| Cloud Run min=0 / max=2 / 1 vCPU / 256Mi | ✅ | 課金ガード |
| **Budget Alert 1,000 円**設定済 | ✅ | ユーザー側 GCP Console |
| Cloud SQL **未作成**（計画書 §4.4 段階化通り）| ✅ | `gcloud sql instances list` → 0 件 |
| Cloud Run Jobs **未作成** | ✅ | 0 件 |
| Cloud Scheduler **未作成** | ✅ | 0 件 |
| Cloud Logging で Secret / presigned URL / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / X-Amz-Signature / access_token の grep 0 ヒット | ✅ | 全期間 |

### 1.9 ハーネス / フィードバックループ

| 項目 | 状態 |
|---|---|
| `.agents/rules/wsl-shell-rules.md` 新規作成（cwd drift / sudo / install verification / hook 整合性）| ✅ コミット `10eb2c8` |
| `track-edit.sh` 拡張子拡張（.md / .json / .yaml / .sql / Dockerfile / .dockerignore / 等、`.env.local` は明示除外）| ✅ |
| `harness/failure-log/*.md` を git 管理対象に変更（`.gitignore` 修正）| ✅ |
| failure-log 6 件記録（cwd drift / gcloud install / sudo / GCP account / Cloud Run /healthz / next-on-pages deprecated）| ✅ |
| `harness/QUALITY_SCORE.md` 初回反映（M1 PoC 5 モジュールを F → B に）| ✅ |
| ADR-0001 §M1 検証結果に Cloud Run 実環境デプロイ + `/health` 採用反映 | ✅ コミット `e8f7029` |
| ADR-0003 §M1 検証結果に Workers + Cloud Run 別オリジンの U2 確認結果反映 | ✅ コミット `6e38354` |
| 計画書 §M1 完了条件 / §13.0 残作業を実環境デプロイ完了状態に更新 | ✅ 同上 |

---

## 2. M1 で意図的にやらないこと（M2 早期に持ち越す）

| 項目 | M2 へ回す理由 |
|---|---|
| **Cloud SQL 実環境作成** | 計画書 §4.4 段階化通り。ローカル PoC で migration / Outbox / Turnstile DB 動作はすべて成立済。Cloud SQL は時間課金が最大リスクのため、M2 早期に必要時のみ短時間起動 |
| **Cloud Run Jobs 実行** | 同一 image に `outbox-worker` バイナリは含めて push 済（A-1）。Cloud Run Jobs 自体の起動は Cloud SQL 起動と一体で M2 早期に実施 |
| **Cloud Scheduler 実行** | Cloud Run Jobs と一体で M2 早期。U11（reconcile-scripts.md §11）の最終確定はこのタイミング |
| **Turnstile 本番 widget 作成** | Workers hostname `vrcpb-spike-frontend.k-matsunaga-biz.workers.dev` は確定済だが、本番では独自ドメインで運用予定。widget 発行は U2 独自ドメイン確定後の M2 早期が妥当 |
| **SendGrid 実送信 PoC** | ADR-0004 で Accepted、M2 早期 TODO として明記済。M1 はインフラ実環境検証に集中したため、メール実送信は M1 範囲外 |
| **U2 独自ドメイン / Cookie Domain 案 A 採用** | 独自ドメイン取得（DNS / Cloudflare DNS / Workers Custom Domain / Cloud Run Domain Mapping）が必要なため M2 早期。M1 では「`session-check` が別オリジンで false/false」を確定材料として記録 |
| **24h / 7 日後 Safari ITP 長期観察完了** | 起点 2026-04-26 で開始済の継続観察。完了は時間経過依存（最大 7 日）のため M1 完了をブロックしない |
| **本実装 `frontend/` / `backend/` への移植** | M1 はあくまで PoC 検証。本実装は M2〜M7 の実装フェーズで `domain-standard.md` 構造に従って書き直す（PoC コードは流用しない方針、`m1-spike-plan.md` §9 触らないもの） |
| **本実装 DB migration 作成** | 各集約のドメイン設計（`docs/design/aggregates/`）に従い M3 で作成 |
| **コールドスタート / SIGTERM / Cloud Run 東京 ↔ R2 レイテンシ詳細計測** | M1 で接続成立は確認済（p50 詳細値は未計測）。M2 早期に Cloud SQL 起動と同時に計測 |

---

## 3. M2 早期タスク一覧（優先順位付き）

### 優先度 A（M2 着手と同時または直前に必須）

| # | タスク | 期待アウトプット |
|---|---|---|
| A-1 | 独自ドメイン取得（例: `vrcphotobook.com` 等、命名は別途決定）| DNS が引ける状態 |
| A-2 | Cloudflare DNS 設定 + Workers Custom Domain（`app.example.com`）| Workers が独自ドメインで配信可能 |
| A-3 | Backend API のドメイン方針決定（`api.example.com` / Cloud Run Domain Mapping or Workers `/api/*` プロキシ）| Cookie Domain 戦略確定 |
| A-4 | **U2 Cookie Domain 案 A 採用 + 実機確認**（`Domain=.example.com` で Workers ↔ Cloud Run の Cookie 共有成立を確認）| ADR-0003 §13 U2 解消 |
| A-5 | 本実装ディレクトリ `frontend/` / `backend/` への移植計画策定（`domain-standard.md` 準拠、PoC コード流用しない方針の再確認） | 移植計画ドキュメント |

### 優先度 B（DB 必須機能の検証）

| # | タスク | 期待アウトプット |
|---|---|---|
| B-1 | Cloud SQL（PostgreSQL 16）作成 + 最小スペック起動 | 起動成立、コスト把握 |
| B-2 | goose で migration 適用（00001 / 00002 / 00003）| `_test_alive` / `upload_verification_sessions` / `outbox_events` 反映 |
| B-3 | `DATABASE_URL` を Secret Manager に登録 + Cloud Run service に注入 | `/readyz` 200 |
| B-4 | Backend の DB 必須 endpoint 確認（Turnstile / upload-intent / Outbox sandbox）| Cloud Run 上で成立 |
| B-5 | Cloud Run Jobs に outbox-worker を `--command` で登録 | Job 登録成立 |
| B-6 | Cloud Scheduler から Cloud Run Jobs を `*/10 * * * *` 等で起動 | 起動成立、U11 解消可否判定 |
| B-7 | コールドスタート / SIGTERM / 東京 ↔ R2 レイテンシ計測 | 数値記録 |
| B-8 | Cloud SQL **停止 / 削除**（B-1〜7 完了時、計画書 §4.4 Step C）| 課金停止 |

### 優先度 C（メール / Turnstile / 継続観察）

| # | タスク | 期待アウトプット |
|---|---|---|
| C-1 | SendGrid アカウント作成 + 審査通過 + 送信ドメイン認証 + DKIM/SPF/DMARC | 実送信前提が揃う |
| C-2 | SendGrid scoped API Key 発行 + Secret Manager 登録 | Secret 注入確立 |
| C-3 | 1 通テスト送信 + Email Activity Feed / API ログに本文・管理 URL が出ないことを実機確認 | ADR-0004 §M2 以降 TODO の主要項目達成 |
| C-4 | bounce / complaint webhook 受信エンドポイントの設計 | M3 / M6 設計準備 |
| C-5 | Turnstile 本番 widget 発行（Cloudflare Dashboard、独自ドメインの hostname 登録）| 本番 widget secret 取得 → Secret Manager |
| C-6 | iPhone Safari でメール内管理 URL を開き、token → session redirect 成立確認 | `.agents/rules/safari-verification.md` 履歴に追記 |
| C-7 | 24h / 7 日後 Safari ITP 観察結果を `.agents/rules/safari-verification.md` 履歴へ追記（起点 2026-04-26）| 継続観察結果が ADR-0003 §13 に反映 |

### 優先度 D（M2 中盤以降、本実装フェーズ）

| # | タスク |
|---|---|
| D-1 | `docs/design/aggregates/` 各集約の sqlc クエリ + Repository 実装 |
| D-2 | UseCase / ApplicationService 層の実装（`testing.md` 準拠のテーブル駆動 + Builder） |
| D-3 | image-processor（cgo + libheif）コンテナ構築 |
| D-4 | UI 本実装（`design/mockups/prototype/` を参考、Tailwind） |
| D-5 | reconcile スクリプト（`scripts/ops/reconcile/*.sh`）実装 |

---

## 4. 後片付け判断（M1 完了後）

### 4.1 残してよいもの（理由 + 月額）

| リソース | 残す理由 | 月額（概算） |
|---|---|---|
| **Cloud Run service `vrcpb-spike-api`**（revision 003） | M2 早期で再 deploy せず B-1〜B-7 で再利用できる。min=0 で待機中は 0 円 | 無料枠内（リクエスト無し時） |
| **Cloudflare Workers `vrcpb-spike-frontend`** | 同様、M2 早期 U2 案 A 切替まで使用 | 無料枠内 |
| **Artifact Registry image `api:m1-live-health`**（9MB） | M2 早期で同一 image を再利用、または digest 指定で deploy できる | 無料枠 0.5GB の 1.8% |
| **Secret Manager R2_* 5 個** | M2 早期で `DATABASE_URL` を追加するときに Secret Manager 構成が既にある状態が便利 | 約 $0.30/month |
| **Budget Alert 1,000 円** | M2 早期も低水準で進めるため、引き続きガードレールとして必要 | 無料 |
| **R2 バケット `vrcpb-spike` + テストオブジェクト 1 件**（1024B）| M2 早期で再アップロード検証に使える | ほぼ 0 円 |
| **R2 API Token**（短期 TTL）| 期限内なら M2 早期にも使える。期限切れ時は再発行 | 無料 |
| **失敗 / 検証 / 完了判定の各 work-logs / failure-logs / ADR / 計画書** | M2 移行時の主要参照。git 管理 | 無料 |

→ **M1 完了後の月額予測: 約 $0.30〜数十円**（Cloud SQL 未作成のため低水準を維持）。

### 4.2 削除候補（**今は削除しない、判断表のみ**）

| リソース | M1 完了時に削除？ | M2 早期に削除？ | 備考 |
|---|---|---|---|
| Cloud Run の **旧 revision 001 / 002**（traffic 0%）| 任意 | 任意 | 課金ゼロのため放置可。整理したい場合 `gcloud run revisions delete vrcpb-spike-api-00001-8mc` 等 |
| Artifact Registry の **古いタグ `api:m1-live`**（digest `a5d0e58f...`）| 残してよい | M2 で完全切替後に削除 | M1 で `m1-live` は使われなくなったが、容量微小 |
| Cloud Run service `vrcpb-spike-api` | 残す | M2 完了後に整理 | 上記 §4.1 通り |
| Workers `vrcpb-spike-frontend` | 残す | U2 案 A 切替後に旧 Worker を削除 | 同上 |
| R2 Secret Manager（R2_*）| 残す | M2 完了後に整理 | 同上 |
| R2 API Token | 期限切れまで残す | 期限内であれば M2 早期で再利用 | Cloudflare Dashboard で TTL 確認 |
| R2 テストオブジェクト | 残してよい | 任意 | 1024B、課金ほぼ無し |
| Budget Alert | 残す | 残す | 課金ガードは継続必須 |

> **削除は実行しない**。判断表のみ。実削除は M1 完了承認後にユーザー側で別タイミング（または M2 移行時）に実施。

---

## 5. M1 完了判定（最終結論）

### 5.1 M1 は完了扱いにできるか

✅ **Yes**

### 5.2 理由

1. **計画書 §M1 完了条件の必須項目**（`m1-spike-plan.md` §13.1）のうち、**実環境デプロイ依存項目はすべて成立**：
   - Frontend Workers (`*.workers.dev`) ✅
   - Backend Cloud Run (`*.run.app`) ✅
   - Frontend / Backend 結合確認（CORS / Origin / Cookie 引き渡し U2 確定材料）✅
   - macOS Safari / iPhone Safari 実機確認 ✅
2. **計画書 §12 成功条件の 12 項目**のうち、達成可能な項目はすべて達成（`/health` 200、R2 系、CORS、Cloud Logging Secret 漏洩なし、min-instances=0、後片付け手順整備済 等）
3. **既知問題はすべて消化済**:
   - C: og:image 解消（`afa3c7b`）
   - D: X-Robots-Tag 二重出力解消（同上）
   - U2: 別オリジン Cookie 不通を確定材料として記録、案 A を M2 一次方針として確定
   - GFE intercept: `/health` 採用で迂回（`e8f7029`）
4. **ハーネスのフィードバックループが機能**：6 件の failure-log + `wsl-shell-rules.md` + `track-edit.sh` 拡張が、M1 後半で再発防止に効いた（`docker build -f` 形式採用、cd 不使用）

### 5.3 M1 完了をブロックしない残項目

- **24h / 7 日後 Safari ITP 観察**: 継続観察項目（時間依存）
- **Cloud SQL Step B / Cloud Run Jobs / Cloud Scheduler**: 計画書 §4.4 段階化通り、M2 早期へ持ち越し合意済
- **SendGrid 実送信 / Turnstile 本番 widget**: M2 早期 TODO（ADR-0004 / ADR-0005 で記録済）
- **U2 独自ドメイン**: M2 早期で取得 → 案 A 採用、ADR-0003 §13 で確定方針
- **コールドスタート / レイテンシ詳細値**: 接続は成立、数値計測は M2 早期で

### 5.4 M2 早期に必ず着手する項目

優先度 A の **A-1〜A-5（独自ドメイン取得 + U2 解消 + 本実装移植計画）** から着手。
B-1〜B-8 は A の進捗に応じて並行可能。

### 5.5 コスト上の注意

- M1 完了時点での月額予測: **約 $0.30〜数十円**
- Budget Alert 1,000 円に対して十分余裕
- **Cloud SQL を立てたら最大の課金リスクが入る**（計画書 §4.3）→ 必ず Step C（停止 / 削除）まで一気通貫で進める運用

### 5.6 Safari 継続観察

- 起点: **2026-04-26**（Workers 実環境デプロイ完了時点）
- 観察対象 URL: `https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev/edit/sample-photobook-id` / `/manage/sample-photobook-id`
- 確認項目:
  - 24 時間後アクセスで `draft session found` / `manage session found` 表示維持
  - 7 日後アクセスで draft Cookie の Max-Age 限界（604800 秒）まで残るか
- 観察結果は `.agents/rules/safari-verification.md` §履歴に追記（ユーザー側で実施）

### 5.7 次にユーザーが判断すべきこと

1. **本判定表のレビュー**: §1〜§4 の整理に異論がないか
2. **M1 完了承認**: 「M1 を完了扱いにする」を明示承認
3. **承認後の計画書反映**: `m1-spike-plan.md` §13 完了条件の M1 完了マーク + `m1-live-deploy-verification-plan.md` §16 履歴に M1 完了の行を追加（次コミット候補）
4. **M2 早期タスクの優先順位再確認**: 優先度 A（独自ドメイン + U2）から着手するか、別の起点（例: SendGrid 先行）にするか
5. **後片付けの実行タイミング**: M1 完了直後 / M2 早期で再利用してから / M2 完了後にまとめて、のどれにするか

---

## 6. 関連リンク

- 計画書 / ADR
  - `docs/plan/m1-spike-plan.md`（§13 M1 完了条件）
  - `docs/plan/m1-live-deploy-verification-plan.md`（§4 費用ガード / §6 デプロイ順序 / §12 成功条件）
  - `docs/adr/0001-tech-stack.md`（§M1 検証結果 / `/health` 採用）
  - `docs/adr/0003-frontend-token-session-flow.md`（§M1 検証結果 / U2 確定方針）
  - `docs/adr/0004-email-provider.md`（SendGrid 第一 / Mailgun 第二 / AWS SES 運用不可）
  - `docs/adr/0005-image-upload-flow.md`（R2 検証結果 / Turnstile 検証結果）
- ハーネス
  - `harness/QUALITY_SCORE.md`（M1 PoC スコア初回反映）
  - `harness/work-logs/2026-04-26_m1-live-deploy-verification.md`（Step 7-10 検証ログ）
  - `harness/failure-log/2026-04-26_*.md`（5 件、本日記録）
  - `harness/failure-log/2026-04-25_cloudflare-next-on-pages-deprecated.md`（既存）
- ルール
  - `.agents/rules/wsl-shell-rules.md`（cwd / sudo / install verification / hook 整合性）
  - `.agents/rules/safari-verification.md`（Safari 検証必須ルール）
  - `.agents/rules/feedback-loop.md`（失敗 → ルール化）

## 7. 履歴

| 日付 | 変更 |
|---|---|
| 2026-04-26 | 初版作成。M1 完了判定として ✅ Yes（条件付き）を結論。確認済み 9 カテゴリ + M2 早期持ち越し 9 項目 + 後片付け判断表 |
| 2026-04-26 | **ユーザー承認済**：本判定表の結論「M1 を完了扱いにする」を承認。`docs/plan/m1-spike-plan.md` §13.4「M1 完了承認」と `docs/plan/m1-live-deploy-verification-plan.md` §16 履歴に M1 完了承認の行を追加（同コミットで反映）|
