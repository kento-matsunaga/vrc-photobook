# 2026-04-26 プロジェクト全体ロードマップ整理

> M1 完了承認直後の時点で、これまでの仕様 / ADR / 集約設計 / 横断設計 / M1 計画 / M1 完了判定表 / failure-log を一望できるようにまとめたロードマップ。
>
> 目的: M2 以降で道を見失わないために、現在地・確定事項・今後の進行順序を work-log として保存する。
>
> 関連:
> - `harness/work-logs/2026-04-26_m1-live-deploy-verification.md`（Step 7〜10 検証ログ）
> - `harness/work-logs/2026-04-26_m1-completion-judgment.md`（M1 完了判定表）

---

## §A 一目でわかる現在地

```
[Spec / 設計]   ✅ 完了   業務知識 v4 / ADR-0001〜0005 / 6 集約 / 3 横断設計 / 2 認可機構
                          全 P0 / P1 反映済（v4-change-summary §4 / §5）
                              │
[M1 PoC]        ✅ 完了   spike-frontend / spike-backend / R2 / Turnstile / Outbox（5 つ、テストなし方針）
                              │
[M1 実環境]     ✅ 完了   Workers + Cloud Run + R2 + CORS + Safari 実機（2026-04-26、コミット 4ff8687）
                              │
                          ←── ここがいま ──
                              │
[M2 早期]       ⏳ 未着   優先度 A 独自ドメイン + U2 → B Cloud SQL Step B → C メール / Turnstile widget
                              │
[M2〜M7]        ⏳ 未着   本実装ディレクトリ frontend/ backend/ への移植 + 全 25 API + 全 16 画面
                              │
[MVP リリース]
                              │
[Phase 2 以降]            ログイン / マイページ / 作者ページ / 運営 UI / AI / SNS 化 等
```

---

## §B ドキュメント体系マップ（参照優先順位）

### B-1 ヒエラルキーの上から下へ

| 階層 | ファイル / ディレクトリ | 役割 | 状態 |
|---|---|---|---|
| 1. 上位仕様（What / Why） | `docs/spec/vrc_photobook_business_knowledge_v4.md`（1357 行、第1〜7部 + 付録 A〜C）| MVP の業務ルール正本 | ✅ 確定、変更しない |
| 2. アーキテクチャ決定（Why this choice） | `docs/adr/0001-tech-stack.md` ほか 5 本 | 技術スタック / 運営 / 認可 / メール / 画像 | ✅ 全 Accepted（M1 検証結果反映済）|
| 3. 集約ドメイン設計（How / Domain）| `docs/design/aggregates/{photobook,image,report,usage-limit,manage-url-delivery,moderation}/{ドメイン設計,データモデル設計}.md`（合計 4174 行） | DDD 集約単位の不変条件 / エンティティ / 値オブジェクト / 状態遷移 / テーブル | ✅ 確定、本実装の正本 |
| 4. 認可機構設計 | `docs/design/auth/{session,upload-verification}/*` | 集約ではない認可機構（ADR-0003 / ADR-0005 の具現化）| ✅ 確定 |
| 5. 横断設計 | `docs/design/cross-cutting/{outbox,reconcile-scripts,ogp-generation}.md`（合計 1161 行） | 集約をまたぐ Outbox / Reconcile / OGP 生成 | ✅ 確定、PoC 結果反映済 |
| 6. M1 計画 | `docs/plan/m1-spike-plan.md`（835 行）/ `docs/plan/m1-live-deploy-verification-plan.md`（682 行） | M1 の検証計画 / 実環境デプロイ手順 / 費用ガード | ✅ M1 完了承認済 |
| 7. ハーネス（運用） | `harness/` | QUALITY_SCORE / failure-log（6 件）/ work-logs（2 件） | ✅ ループ機能中 |
| 8. ルール（自動執行 + 規範） | `.agents/rules/*.md`（7 本）+ `.agents/skills/*/SKILL.md`（6 本）+ `.claude/settings.json` | コード規約 / WSL シェル / Safari 検証 / フィードバックループ | ✅ M1 で補強済 |
| 9. 実装 | `harness/spike/{frontend,backend}/`（M1 PoC、本実装に流用しない）/ `frontend/`（空）/ `backend/`（未作成） | M2 以降で本実装 | ⏳ 未着 |

### B-2 ナビゲーション索引

- `docs/README.md` — docs/ の使い分け（spec / design / business / adr）
- `docs/ディレクトリマッピング.md` — コード ↔ docs 対応、`sync-check` スキル参照
- `docs/design/aggregates/README.md` — 6 集約一覧 + 関係図
- `docs/design/auth/README.md` — session / upload-verification 認可機構
- `docs/design/v4-change-summary.md` — v3→v4 全変更点 + P0 / P1 達成チェックリスト

---

## §C 設計のヒエラルキー（変更したら下流すべて見直し）

```
業務知識 v4 §1〜7 + 付録C
   │ 矛盾する場合は v4 を正とする
   ▼
ADR-0001 技術スタック / 0002 運営 / 0003 認可 / 0004 メール / 0005 画像
   │ 業務制約を技術選択へ写像、変更時は ADR-NNNN を新規 or 改訂
   ▼
集約ドメイン設計 6 本 + 認可機構 2 本 + 横断設計 3 本
   │ ADR と業務知識の具体化、不変条件・状態遷移・テーブルを確定
   ▼
M1 計画 / M1 実環境計画
   │ 検証は不変条件を破壊しない範囲で、結果は ADR / 設計に必ず反映
   ▼
M1 PoC（harness/spike/）
   │ コード品質はラフでよい、本実装に流用しない方針
   ▼
M2〜 本実装（backend/ + frontend/）
   │ domain-standard.md / testing.md 厳守、PoC コード再利用しない
```

**重要前提**: 業務知識 v4 と ADR / 集約設計に矛盾を感じたら、まず v4 と ADR を読み直す（この順が原則。`docs/design/aggregates/README.md` 設計原則 §1）。

---

## §D M1 で完了したこと（要約）

詳細は `harness/work-logs/2026-04-26_m1-completion-judgment.md` §1（9 カテゴリで網羅）。

### D-1 設計フェーズ
- 業務知識 v4 / ADR-0001〜0005 / 全集約・横断設計の整備
- v4 の P0（15 項目）/ P1（11 項目）すべて反映済（`v4-change-summary.md` §4 / §5）
- AWS SES 第一候補不採用 → SendGrid 第一 / Mailgun 第二に再選定（ADR-0004 Accepted）

### D-2 PoC フェーズ（5 つ）
- `spike-frontend`: OpenNext + Workers + Static Assets binding、SSR / OGP / Cookie / redirect / Safari 実機
- `spike-backend`: Go chi + pgx + sqlc + goose、distroless image 12.4MB、graceful shutdown
- `spike-r2-upload`: aws-sdk-go-v2 で S3 互換 API、presigned PUT 15min、HeadObject、バリデーション 8 ケース
- `spike-turnstile-upload-verification`: 公式テスト secret、SHA-256 のみ DB 保存、100 並列 race（success=20 / forbidden=80）
- `spike-outbox-reconciler`: `FOR UPDATE SKIP LOCKED`、二重処理防止、cmd/outbox-worker `--once` / `--retry-failed`

### D-3 実環境フェーズ（Step 1〜10、本日完了）
- GCP プロジェクト + Billing + Budget Alert 1,000円
- Artifact Registry + 同一 image に api / outbox-worker 2 バイナリ
- Secret Manager R2_* 5 個（Secret 単位で IAM Accessor、最小権限）
- Cloud Run service 起動、`/health` 採用（`/healthz` GFE intercept 回避、failure-log 化）
- Workers `vrcpb-spike-frontend` deploy、URL = `https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev`
- ALLOWED_ORIGINS で CORS / Origin 結合確認
- macOS Safari / iPhone Safari 実機確認（大きな問題なし）
- C+D 修正（`metadataBase` 設定 + `X-Robots-Tag` 重複解消）

### D-4 ハーネス強化
- failure-log 6 件（`.gitignore` 修正で git 管理化）
- `wsl-shell-rules.md` 新規作成（cwd drift / sudo / install 検証 / hook 整合）
- `track-edit.sh` 拡張子拡張（.md / Dockerfile / .json / .yaml / .sql / 等、`.env.local` は明示除外）
- QUALITY_SCORE.md 初回反映（5 モジュール F→B）

---

## §E 「いま」確定した不変事項（変更には ADR 改訂が必要）

| 領域 | 確定事項 | 根拠 |
|---|---|---|
| Backend 言語 / フレームワーク | Go 1.24+ / chi v5 / pgx v5 / sqlc v1.30 / goose v3.22 | ADR-0001 |
| Backend Deploy 第一候補 | Cloud Run（asia-northeast1） | ADR-0001 §M1 検証結果 |
| Frontend FW / Adapter | Next.js 15 App Router / OpenNext (`@opennextjs/cloudflare`) | ADR-0001 §M1 検証結果 |
| Frontend Deploy | Cloudflare Workers + Static Assets binding | ADR-0001 |
| 必須実装ルール | `runtime = "edge"` 不使用 / ヘッダ制御は middleware 一本化 / `metadataBase` 設定必須 | ADR-0001 §M1 検証結果 |
| Cloud Run ヘルスチェック | **`/health`** を正式採用、`/healthz` はローカル互換並存 | ADR-0001 + failure-log 2026-04-26 |
| Storage | Cloudflare R2（S3 互換 API、エグレス無料）| ADR-0001 / 0005 |
| DB | PostgreSQL 16 | ADR-0001 |
| ID | UUIDv7（DB 主キー）/ public_url_slug 別途 / Cookie session token は 256bit 乱数（UUIDv7 不使用） | ADR-0001 / 0003 |
| 認可方式 | token → session 交換、HttpOnly+Secure+SameSite=Strict Cookie、DB は SHA-256 のみ保存 | ADR-0003 |
| Cookie Domain U2 | **M2 早期に独自ドメイン取得 → 案 A（共通親ドメイン + Domain=.example.com）採用**（一次方針）| ADR-0003 §M1 検証結果 |
| 画像アップロード | R2 presigned URL 方式、upload-intent + complete の 2 段、Turnstile セッション化（30 分 / 20 intent） | ADR-0005 |
| Email Provider | **SendGrid 第一 / Mailgun 第二 / AWS SES 運用不可**、本文最小化 / Outbox payload に管理 URL を入れない | ADR-0004 |
| 横断機構 | Transactional Outbox / OGP 独立管理 / Reconcile（自動 4 種 + 手動 6 種）すべて MVP から | v4 §6.11 / 6.12 / 6.16 / 6.17 + 横断設計 3 本 |
| MVP スコープ | 6 機能（作成 / 公開 / 閲覧 / 管理 / 控えメール / 通報 + 利用制限 + X 共有 + LP + 画像）| v4 §5.1 |
| MVP 範囲外（Phase 2+）| ログイン / マイページ / 作者ページ / SNS 機能 / AI / 運営 UI / creator_avatar / 動画 / 印刷 | v4 §5.2 |
| 全ページ noindex | MVP は全 noindex / `/manage/`・`/draft/` は Disallow | v4 §7.6 |
| 運営操作 | `cmd/ops` Go CLI + `scripts/ops/*.sh`、HTTP API 化しない | ADR-0002 / v4 §6.19 |
| Safari 検証ルール | Cookie / redirect / OGP / ヘッダ / モバイル UI 変更時に macOS Safari + iPhone Safari 必須確認 | `.agents/rules/safari-verification.md` |
| WSL 運用 | `cd` 不使用、`-C` / `-f` / 絶対パス、sudo は対話シェルでまとめて | `.agents/rules/wsl-shell-rules.md` |
| 失敗 → ルール化 | すべての失敗を `harness/failure-log/` に記録、再発防止策をルール / スキルに | `.agents/rules/feedback-loop.md` |

---

## §F M2 早期：4 ブロックの順序と内容

`harness/work-logs/2026-04-26_m1-completion-judgment.md` §3 を再構成。優先順位は **A → B → C → D** で確定。

### F-1 優先度 A：独自ドメイン + U2（**最優先**、次にやる入口）

| # | タスク | 期待アウトプット | 参照 |
|---|---|---|---|
| A-1 | 独自ドメイン取得（命名は別途、個人情報を含めない） | DNS が引ける | — |
| A-2 | Cloudflare DNS + Workers Custom Domain（`app.<domain>`）| Workers が独自ドメイン配信 | wrangler.jsonc / Cloudflare Dashboard |
| A-3 | Backend API ドメイン方針決定（`api.<domain>` Cloud Run Domain Mapping or Workers `/api/*` プロキシ）| Cookie Domain 戦略確定 | ADR-0003 §13 U2 |
| A-4 | **U2 案 A 採用 + 実機確認**（`Domain=.<domain>` で Workers ↔ Cloud Run の Cookie 共有成立） | ADR-0003 §13 U2 解消 | ADR-0003 §M1 検証結果 |
| A-5 | 本実装 `frontend/` / `backend/` 移植計画策定（`domain-standard.md` 構造、PoC コード流用しない）| 移植計画書 | `.agents/rules/domain-standard.md` |

**先に A をやる理由**: ドメインが決まらないと Cookie Domain / Turnstile widget hostname / SendGrid 送信ドメイン / OGP の絶対 URL すべてが暫定運用のまま。後戻りコストが最大。

### F-2 優先度 B：Cloud SQL Step B / U11 起動基盤

| # | タスク | 期待アウトプット |
|---|---|---|
| B-1 | Cloud SQL（PostgreSQL 16）作成、最小スペック | 起動成立、コスト把握 |
| B-2 | goose で migration 適用（00001 / 00002 / 00003）| `_test_alive` / `upload_verification_sessions` / `outbox_events` |
| B-3 | `DATABASE_URL` を Secret Manager 登録 + Cloud Run 注入 | `/readyz` 200 |
| B-4 | DB 必須 endpoint 確認（Turnstile / consume / Outbox sandbox）| Cloud Run 上で成立 |
| B-5 | Cloud Run Jobs に outbox-worker を `--command` 登録 | Job 登録成立 |
| B-6 | Cloud Scheduler から Cloud Run Jobs を `*/10 * * * *` で起動 | U11 解消可否判定 |
| B-7 | コールドスタート / SIGTERM / 東京 ↔ R2 レイテンシ計測 | 数値記録 |
| B-8 | **Cloud SQL 停止 / 削除**（計画書 §4.4 Step C） | 課金停止 |

**Step C を必ずやる**: Cloud SQL の時間課金が最大の課金リスク（計画書 §4.3）。起動 → 検証 → 停止/削除を一気通貫で。

### F-3 優先度 C：メール / Turnstile / 継続観察

| # | タスク |
|---|---|
| C-1 | SendGrid アカウント作成 + 審査通過 + 送信ドメイン認証 + DKIM / SPF / DMARC |
| C-2 | scoped API Key 発行 + Secret Manager 登録 |
| C-3 | 1 通テスト送信 + Email Activity Feed / API ログ / Webhook payload に本文 / 管理 URL が出ないことを実機確認 |
| C-4 | bounce / complaint webhook 設計（M3 / M6 準備） |
| C-5 | **Turnstile 本番 widget** 発行（独自ドメイン hostname 登録）|
| C-6 | iPhone Safari でメール内管理 URL 開いて token → session redirect 成立確認 |
| C-7 | **24h / 7 日後 Safari ITP 観察結果**を `.agents/rules/safari-verification.md` §履歴へ追記（起点 2026-04-26）|

### F-4 優先度 D：本実装移植（M2 中盤〜）

| # | タスク |
|---|---|
| D-1 | `backend/{module}/{domain,infrastructure,internal}/` の本実装着手（`domain-standard.md` 厳守、PoC コード流用しない）|
| D-2 | `frontend/` 本実装（Next.js 15、Tailwind、`design/mockups/prototype/` を参考）|
| D-3 | `testing.md` 準拠のテーブル駆動 + Builder パターンの徹底 |
| D-4 | image-processor（cgo + libheif）コンテナ構築 |
| D-5 | `scripts/ops/reconcile/*.sh` 実装 |

---

## §G M2〜M7 本実装フェーズの全体像

各集約のドメイン設計 §「次集約への引き継ぎ事項」/ §「次工程への引き継ぎ事項」から推定したマイルストーン構造。

| マイルストーン | 主な内容 | 主な参照 |
|---|---|---|
| **M2** | M2 早期（§F の A〜C 完了）+ 本実装プロジェクト立ち上げ（`backend/` / `frontend/` 骨組み、CI、Dockerfile 整備）| ADR-0001 §M1 検証結果 / `domain-standard.md` |
| **M3** | DB 層実装：全集約の goose migration / sqlc クエリ / Repository（`photobooks` / `images` / `image_variants` / `sessions` / `upload_verification_sessions` / `reports` / `usage_windows` / `manage_url_deliveries` / `moderation_actions` / `outbox_events` / `photobook_ogp_images`）| 各集約データモデル設計 / Outbox §3 / OGP §（独立テーブル）|
| **M4** | UseCase / ApplicationService 層：CreateDraftPhotobook / PublishPhotobook / CreateImageUploadIntent / CompleteImageUpload / SubmitReport / ExchangeTokenForSession / Moderation 7 種 / RequestManageUrlDelivery など | Photobook §10 / Image §6 / Report §6 / Moderation §6 / ManageUrlDelivery §6 / session §6 / upload-verification §6 |
| **M5** | API 層：全 25 API の handler、Cookie 認可 middleware、Origin 検証、CORS、CSRF（破壊的操作のワンタイムトークン、U4）| ADR-0003 / 各集約「他集約との境界」|
| **M6** | ワーカー層：`outbox-dispatcher` / 自動 reconciler 4 種（`outbox_failed_retry` / `draft_expired` / `stale_ogp_enqueue` / `delivery_expired_to_permanent`）/ image-processor（cgo + libheif、HEIC / EXIF 除去 / variant 生成）/ ManageUrlDelivery メール送信ハンドラ（SendGrid）| Outbox §11 / reconcile-scripts §3.7 / Image §6 / ADR-0005 / ADR-0004 §実装方針 |
| **M7** | Frontend 全 16 画面：作成 / 編集 / 公開 / 閲覧 / 管理 / 通報 / X 共有 / LP、Tailwind + react-hook-form + zod、Turnstile widget 統合、楽観ロック競合 UX（U3）| プロトタイプ `design/mockups/prototype/` / ADR-0001 / Photobook §10 |

> **注**: M2〜M7 の番号は計画書 §後続候補から推測した粗い構造。本実装着手時に `docs/plan/m2-implementation-plan.md` 等を新規作成して詳細化する想定。

---

## §H Phase 2 以降への持ち越し（v4 §5.5）

MVP リリース後の拡張順:

- **Phase 2**: 任意ユーザー機能の本格化（ログイン、マイページ、作者ページ、デコレーション強化、運営 UI、creator_avatar、AWS SES 再申請検討）
- **Phase 3**: AI 生成補助（タイトル / 説明文 / X 投稿文の自動生成）
- **Phase 4**: AI 画像加工（背景除去等）
- **Phase 5+**: 新レイアウト、タイプ別作り込み（BOOTH 連携、ポートフォリオ機能）、SNS 機能、有料プラン、物理印刷発注

---

## §I 反れないための「現在地マーカー」と「次の 1 サイクル」

### I-1 現在地マーカー（コミット粒度、本ロードマップ作成時点）

```
4ff8687 docs(plan): mark M1 spike as completed     ← M1 完了承認、★いまここ
256e3d1 docs(spike): add M1 completion judgment
afa3c7b fix(spike-frontend): set metadataBase and dedupe robots headers
6e38354 docs(spike): record live frontend backend integration results
0f6d817 fix(spike-frontend): prepare Workers deploy with Cloud Run backend URL
e8f7029 fix(spike-backend): add Cloud Run safe health endpoint
10eb2c8 chore(harness): strengthen feedback loop before live deploy steps
36a1e93 chore(spike-backend): exclude secrets from Docker build context
9e6a4f6 chore(spike-backend): bundle outbox-worker binary into Cloud Run image
1e6446b docs(plan): add cost guardrails and Cloud SQL staging to M1 live deploy plan
a4e4dbd docs(plan): add M1 live deploy verification plan for Cloudflare Workers and Cloud Run
（以下 M1 PoC 系）
```

### I-2 次の 1 サイクル（最小単位、推奨）

**「§F-1 優先度 A の入口計画書を 1 本作る」**。コード変更も実リソース作成もせず、計画だけ。

```
1. 計画書 docs/plan/m2-early-domain-and-cookie-plan.md（仮）を新規作成
   - 独自ドメインの命名候補（個人情報を含めない 3〜5 案）
   - 取得元（Cloudflare Registrar 推奨）+ DNS 設定方針
   - Workers Custom Domain の手順
   - Backend API ドメイン方針（Cloud Run Domain Mapping or Workers /api/* プロキシ）
   - U2 Cookie Domain 案 A の実装手順（Domain=.example.com）
   - 切替後の curl + Safari 実機検証手順
   - 旧 *.workers.dev / *.run.app との併存・廃止タイミング
   - 課金影響（Cloudflare Registrar / Cloud Run Domain Mapping は無料）
2. レビュー → 承認 → 着手判断
```

なぜこの粒度か:

- ドメイン名は個人情報・命名権の判断が入るため**ユーザー判断必須**、当方は提案まで
- いきなり取得すると後戻り（取得後のキャンセルは難しい）
- 計画書を一度通すと、A→B→C→D の各ブロックが「次に何をやるか」明確になる
- 計画書だけならコミット 1 つで済み、リソース課金ゼロ

### I-3 一つだけ反れやすいポイント

> **「Cloud SQL を立てる前に、必ず U2（独自ドメイン + Cookie Domain）を解消する」**

理由: U2 が解消する前に Cloud SQL を立てて Turnstile / Outbox / DB endpoint を実機検証しても、Frontend ↔ Backend の認可フローが「別オリジン」のままなので、本物のエンドツーエンド検証ができない。Cloud SQL は最大の課金リスクなので、立てるなら確度の高い検証で 1 ターン完結させたい。

---

## §J 落とし穴・運用注意事項（過去 6 件の failure-log + 直接の経験から）

| # | 落とし穴 | 防止策 / 既存ルール |
|---|---|---|
| 1 | Bash の `cd` で cwd drift → hook 失敗 | `.agents/rules/wsl-shell-rules.md` / `failure-log/2026-04-26_wsl-cwd-drift-recurrence.md`、`-C` / `-f` / 絶対パス使用 |
| 2 | sudo パスワードが Bash ツールに渡らない | 同上 / `failure-log/2026-04-26_sudo-noninteractive-shell-limit.md`、ユーザー対話シェルで `! ...` |
| 3 | install / セットアップ「完了」報告と実態の乖離 | 同上 / `failure-log/2026-04-26_gcloud-install-verification-mismatch.md`、`which` / `--version` / 設定ファイル存在を必ず客観確認 |
| 4 | GCP プロジェクト所有者と CLI ログインアカウントの不一致 | `failure-log/2026-04-26_gcp-account-billing-mismatch.md`、`config set project` 直後に `projects list --filter` / `billing describe` で 3 点確認 |
| 5 | Cloud Run / GFE が `/healthz` を intercept | `failure-log/2026-04-26_cloud-run-healthz-intercepted.md`、Cloud Run 上は **`/health` を正式採用** |
| 6 | `@cloudflare/next-on-pages` が deprecated | `failure-log/2026-04-25_cloudflare-next-on-pages-deprecated.md`、`@opennextjs/cloudflare` を採用済 |
| 7 | `.env.local` が Docker build context に混入 | `harness/spike/backend/.dockerignore` / `harness/spike/frontend/.gitignore` |
| 8 | Secret 値をログ / curl 出力 / git diff に出してしまう | `.agents/rules/security-guard.md`、`grep` で必ず事前 sanity check |
| 9 | Cloud SQL の停止忘れ / Cloud Scheduler の削除忘れ | 計画書 §14 後片付け、Budget Alert 1,000 円 |
| 10 | `NEXT_PUBLIC_*` を wrangler runtime env で渡そうとして失敗 | Next.js は build 時 inline、`.env.production` で渡す |
| 11 | Workers ↔ Cloud Run の別オリジン Cookie 不通 | 設計通りの想定、U2 案 A で解消（M2 早期）|
| 12 | `docs/`・`harness/failure-log/`・`harness/work-logs/` が `.gitignore` で除外されると共有不能 | `.gitignore` を 2026-04-26 修正済（`*.draft.md` だけ除外）|

---

## §K 履歴

| 日付 | 変更 |
|---|---|
| 2026-04-26 | 初版作成。M1 完了承認直後、M2 早期の入口に立った時点で全体ロードマップを保存。優先順位 A → B → C → D を確定、次の 1 サイクルとして `docs/plan/m2-early-domain-and-cookie-plan.md` 作成を §I-2 に明記 |
