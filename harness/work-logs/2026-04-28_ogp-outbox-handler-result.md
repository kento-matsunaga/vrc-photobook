# PR33d Outbox handler 連携 + 公開 OGP 配信実機検証 結果（2026-04-28）

## 概要

- 新正典 PR33 / `docs/plan/m2-ogp-generation-plan.md` PR33d 計画に従い、
  outbox-worker の `photobook.published` handler を **no-op から OGP 生成副作用に接続**
- Cloud Run Jobs `vrcpb-outbox-worker` を asia-northeast1 に作成（STOP θ）
- UseCase 経由でテスト用 public photobook を作成（STOP ι、CLI helper 追加）
- Job を手動実行し、副作用 handler 初回稼働を実機で検証（STOP κ、初回失敗→patch→再実行 SUCCESS）
- generated OGP の **公開配信実機 + SNS card 形式 + Safari noindex** 全てクリア
- 本番 photobook 1 件はテスト目的のため、検証直後に `hidden_by_operator=true` で公開停止
  （履歴: outbox event / OGP rows / R2 object はすべて保持）

## ファイル追加 / 更新

### Backend

| ファイル | 役割 |
|---|---|
| `backend/internal/outbox/contract/ogp_generator.go`（新規） | internal package boundary を超えるための contract package。`OgpGenerator` interface / `OgpGenerateResult` / `ErrNotPublishedSkippable` sentinel を outbox / ogp 両 tree から見える位置に置く |
| `backend/internal/outbox/internal/usecase/handlers/photobook_published.go` | no-op から `contract.OgpGenerator` 呼び出しに変更。`ErrNotPublishedSkippable` を nil 返却で processed に倒す |
| `backend/internal/outbox/internal/usecase/handlers/photobook_published_test.go`（新規） | 5 ケース（success / NotPublishedSkippable→nil / payload broken→error / invalid UUID→error / transient error→propagate）テーブル駆動 |
| `backend/internal/outbox/wireup/wireup.go` | `Config.OgpGenerator` 追加。nil の場合 handler は **登録しない**（R2 未設定環境で Job が誤動作しない安全策） |
| `backend/internal/ogp/wireup/outbox_adapter.go`（新規） | `OutboxOgpAdapter`（contract.OgpGenerator 実装）。`ogpusecase.ErrNotPublished` → `contract.ErrNotPublishedSkippable` 変換 |
| `backend/cmd/outbox-worker/main.go` | OGP adapter を組み立てて outbox wireup に注入。R2 未設定なら adapter を nil で渡す |
| `backend/internal/photobook/wireup/wireup.go` | CLI / batch 用 helper（`BuildCreateDraftPhotobook` / `BuildPublishFromDraft` / `CreateAndPublishCLIInput` / `CreateAndPublishCLIOutput` / `CreateAndPublishForCLI`）を追加。raw token は helper 内で破棄 |
| `.gitignore` | `/backend/cmd/_test-publish-public/` を追加（STOP ι 用テンポラリ CLI、commit せず検証後削除） |

### test 結果

| 範囲 | 結果 |
|---|---|
| `internal/outbox/internal/usecase/handlers/...` test（DB 不要） | 5 ケース全 pass |
| `internal/outbox/...` test（DB あり） | 全 pass |
| `internal/ogp/...` test（DB あり、`-p 1`） | 全 pass |
| `internal/photobook/...` test（DB あり） | 全 pass |
| `go vet ./...` / `go build ./...` | クリーン |

## STOP θ: Cloud Run Jobs 作成（asia-northeast1）

| 観点 | 結果 |
|---|---|
| Job name | `vrcpb-outbox-worker` |
| region | asia-northeast1 |
| image | `asia-northeast1-docker.pkg.dev/<project>/vrcpb/vrcpb-api:fe19ab5` |
| args | `--once --max-events 1 --timeout 60s` |
| serviceAccountName | `271979922385-compute@developer.gserviceaccount.com` |
| Secret refs（6 件） | DATABASE_URL / R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET_NAME / R2_ENDPOINT |
| max-retries | 0（副作用 handler の **double-fire を避けるため**） |
| parallelism | 1 |
| task-count | 1 |
| Cloud Scheduler | 未作成（STOP λ で要否判断） |

> **STOP θ の課題（後続 STOP κ で発覚）**: 本ステップで `--set-cloudsql-instances`
> annotation を指定し忘れた。詳細は STOP κ 失敗の章 + failure-log。

## STOP ι: テスト用 public photobook 作成（UseCase 経由 / 直接 SQL を使わない）

### 採用案

- **case ι-2**: `BuildCreateDraftPhotobook` → `BuildPublishFromDraft` を 1 関数化した
  `CreateAndPublishForCLI` を `photobook/wireup` に追加し、temp CLI から呼び出す
- raw token (draft / manage) は helper 内で受け取って即破棄、戻り値には含めない
- temp CLI は `backend/cmd/_test-publish-public/`（`.gitignore` に登録、検証後削除）

### 実行結果

| 観点 | 値 |
|---|---|
| photobook_id | `019dd1bb-774f-7341-91a4-fd0fbd279320` |
| slug | `uqfwfti7glarva5saj` |
| outbox_pending_count | 1（同一 TX で `photobook.published` event INSERT を確認） |
| visibility | public |
| status | published |
| hidden_by_operator | false（後で true に） |

raw token は helper 戻り値に含めず、stdout / chat / log / work-log に未表記。

## STOP κ: Job 実行 + 公開 OGP 配信実機検証

### 1 回目（失敗、`vrcpb-outbox-worker-jdfh9`）

- 実行コマンド: `gcloud run jobs execute vrcpb-outbox-worker --region=asia-northeast1 --wait`
- 結果: `Container called exit(1)` / `dial unix /cloudsql/<INSTANCE>/.s.PGSQL.5432: connect: no such file or directory`
- 原因: Job spec に **`run.googleapis.com/cloudsql-instances` annotation が欠落**
  （STOP θ で `--set-cloudsql-instances` を指定し忘れ）
- 副作用: なし（DB 接続前に exit、event は status=pending / attempts=0 のまま）
- 詳細 + 再発防止: [`harness/failure-log/2026-04-28_cloud-run-job-missing-cloudsql-annotation.md`](../failure-log/2026-04-28_cloud-run-job-missing-cloudsql-annotation.md)

### Job spec patch（最小修正）

```bash
gcloud run jobs update vrcpb-outbox-worker \
  --region=asia-northeast1 \
  --project=<project> \
  --set-cloudsql-instances=<project>:asia-northeast1:vrcpb-api-verify
```

patch 後の確認（`gcloud run jobs describe --format=export`）:

- `metadata.annotations."run.googleapis.com/cloudsql-instances"` 追加 ✅
- image / args / serviceAccountName / Secret refs / max-retries / parallelism / task-count 全て不変 ✅

### 再実行前 DB 状態確認

| 観点 | 値 |
|---|---|
| pending event 数（available_at <= now()）| 1 |
| failed event 数（available_at <= now()）| 0 |
| ListPending #1（古い順） | STOP ι event（aggregate_id=019dd1bb-...）|
| target event の status / attempts | pending / 0 |
| public published photobook 件数 | 1（STOP ι 作成分のみ）|

→ 古い pending 不在、target event のみ pickup される確証あり。

### 2 回目（成功、`vrcpb-outbox-worker-znx4v`）

Job logs（msg のみ抜粋、フィールド値は含めず Secret なしを確認）:

```
outbox-worker: ogp generator wired (photobook.published will trigger OGP generation)
outbox-worker starting
ogp rendered and uploaded
ogp marked generated
outbox handler: photobook.published (ogp generated) [generated=true, duration_ms=407, result=ok]
outbox processed
outbox-worker finished [picked=1, processed=1, failed_retry=0, dead=0]
Container called exit(0).
```

handler `Handle` の実行時間: **407 ms**（renderer + R2 PUT + images / image_variants / MarkGenerated 同 TX）。

### 事後 DB rows 検証

| 観点 | 値 |
|---|---|
| `outbox_events.status` | `processed`（attempts=0、processed_at=2026-04-28T10:48:59+09:00、last_error=null）|
| `photobook_ogp_images.status` | `generated`（version=1、image_id 設定、generated_at=同上、last_error=null）|
| `images` (linked) | usage_kind=ogp / status=available / source_format=png / available_at=同上 |
| `image_variants` (kind='ogp') | mime_type=image/png / **width=1200 / height=630** / byte_size=23839 / storage_key 設定（長さのみ確認、完全値は出さず）|

### 公開配信実機検証

| 観点 | 結果 |
|---|---|
| Backend `/api/public/photobooks/<PID>/ogp` | HTTP 200 / `{"status":"generated","version":1,"image_url_path":"/ogp/<PID>?v=1"}` |
| Workers `/ogp/<PID>?v=1` | HTTP 200 / `image/png` / **23839 bytes**（DB と一致）/ `Cache-Control: public, max-age=86400, s-maxage=86400` / `x-robots-tag: noindex, nofollow` |
| PNG 寸法（`file` + struct.unpack 検証） | **1200 × 630, 8-bit/color RGB, non-interlaced** |
| `/p/<SLUG>` HTML | HTTP 200 / `<title>OGP test (PR33d)</title>` / `og:title` / `og:description` / `og:url`（絶対 URL）/ `og:image=https://app.vrc-photobook.com/ogp/<PID>?v=1` / `og:image:width=1200` / `og:image:height=630` / `og:type=website` / `twitter:card=summary_large_image` / `twitter:title` / `twitter:description` / `twitter:image` / `<meta name="robots" content="noindex, nofollow">` |
| `/p/<SLUG>` レスポンスヘッダ | `Cache-Control: private, no-cache, no-store, max-age=0, must-revalidate` / `Referrer-Policy: strict-origin-when-cross-origin` / `X-Robots-Tag: noindex, nofollow` |
| Cloud Run Job logs Secret 漏洩 grep | 0 件（`storage_key` 完全値 / `postgres://` URL / Bearer token / R2 access key 値 / Cookie / raw token：いずれも不在。`secretKeyRef` / `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` のヒットは Secret **名** のみで値なし）|

### 公開停止後検証（`hidden_by_operator=true` の効果）

クリーンアップとして対象 photobook を `hidden_by_operator=true` に変更。
status / visibility / version / event / OGP rows / R2 object はすべて保持。

| 観点 | 結果 |
|---|---|
| Backend `/api/public/photobooks/<SLUG>` | **HTTP 410 / `{"status":"gone"}`** ✅ |
| Backend `/api/public/photobooks/<PID>/ogp` | `{"status":"not_public","version":1,"image_url_path":"/og/default.png"}` ✅ |
| Workers `/ogp/<PID>?v=1` | **HTTP 302 / Location: /og/default.png** ✅（fallback 動作）|
| `/p/<SLUG>?_=<bust>` HTML | HTTP 200 / `<title>VRC PhotoBook</title>` / `og:title="VRC PhotoBook"` / body に `gone` 文字列 ✅（hidden 専用テンプレ動作） |

公開停止が **API / Workers / OGP fallback の各経路で正しく動作**することを確認。
履歴データ（outbox processed event / photobook_ogp_images generated row / R2 PNG）は
保持し、後の冪等性 / monitoring 観察に使えるようにしてある。

## SNS validator / Safari 実機確認

`hidden_by_operator=true` 設定後は外部 validator / Safari でも `/p/<SLUG>` は `gone`
テンプレ + デフォルト OGP に倒れる。**SNS validator / Safari 実機での generated OGP 表示
確認は、本番運用での `visibility=public` 公開 photobook が初めて publish された段階で
実施する**（手順は roadmap PR33 章に追記、`.agents/rules/safari-verification.md` の
ルールに従って Cookie / redirect / OGP / 構造化データ / モバイル UI を確認）。

> 本 PR33d で確認した HTTP / 構造化データ層は **SNS card validator が要求する条件を
> 全て満たしている**（og:image 1200×630 絶対 URL、twitter:card=summary_large_image、
> og:image:width / height、Cache-Control: public max-age=86400）。実環境ユーザー向けの
> 公開 photobook が出てからの validator 実機確認は形式手続きとなる見込み。

## PR33d 後回し事項 / 懸念事項 / 未検証事項

| 区分 | 内容 | 再開・解消条件 |
|---|---|---|
| 後回し（運用） | SNS validator（X Card Validator / Discord / Slack / LINE）プレビュー実機確認 | 一般ユーザーが `visibility=public` の photobook を publish したタイミング、または運営判断で別途公開 photobook を 1 件作成して確認するタイミング |
| 後回し（運用） | macOS Safari / iPhone Safari 実機確認（generated OGP 表示） | 同上。公開 photobook 出現時に `.agents/rules/safari-verification.md` §2 の観点で確認 |
| 後回し（運用） | Cloud Scheduler 作成判断（STOP λ） | 副作用 handler の自動回しを始めるかどうかの運営判断後。本 PR33d 終了時点では **手動 Job execute のみ**で運用 |
| 懸念（運用） | 万一 OGP 生成が transient error で `failed` → `pending` retry 中に `dead` まで attempts を消費する場合の運用手順 | reconcile / `ogp_stale.sh` を入れる PR33e（任意）で整備。MVP は手動 Job 再実行で対応 |
| 懸念（コード） | `cmd/outbox-worker/main.go` で R2 未設定時に handler を **登録しない**（events 残る）ため、長期間気付かないと outbox table が肥大化する | Cloud Run Jobs に env / Secret 6 件が必須として注入されているため運用上は登録漏れしない設計。reconcile / metrics は PR33e 以降 |
| 未検証 | 本番で同 PID に **複数の photobook.published event** が発生したときの冪等性（`MarkGenerated` の OCC や R2 上書き） | 実装は `image_variants` (image_id, kind) UNIQUE と `photobook_ogp_images` の OCC で冪等担保。実機 PoC は本 PR33d スコープ外、別 PR で publish 経路の再 publish 機能が入った時に検証 |

→ 上記は roadmap (`docs/plan/vrc-photobook-final-roadmap.md`) PR33d / PR33e 章にも反映。

## クリーンアップ

| 項目 | 状態 |
|---|---|
| `backend/cmd/_test-publish-public/`（テンポラリ CLI） | 削除済 |
| `/tmp/check-pending.go` / `/tmp/check-all-events.go` / `/tmp/check-post*.go` / `/tmp/hide-test-pb.go` | 削除済 |
| `/tmp/dsn-local.txt` / `/tmp/stopi-pid.txt` / `/tmp/stopi-slug.txt` / `/tmp/ogp.png` / `/tmp/ogp.headers` / `/tmp/ogp-lookup.json` / `/tmp/p-slug*.html` / `/tmp/p-slug.headers` / `/tmp/job-logs.json` | 削除済 |
| `cloud-sql-proxy` プロセス | 停止済 |
| 環境変数 (DSN_LOCAL / TARGET_PID / DATABASE_URL / R2_*) | unset 済（すべてインライン使用、main shell に export していない） |
| shell history | 当該セッションは `history -c` クリア済 |

## PR closeout チェックリスト（pr-closeout.md §6）

- [x] **コメント整合チェック実施**: `bash scripts/check-stale-comments.sh --extra "OGP|og:image|twitter:card|SNS|crawler|R2 public|public URL|Cloud Run Jobs|Scheduler|outbox-worker|ogp-generator|cloudsql-instances"`
- [x] **古いコメント修正**: 該当なし（PR33c で stale 整理済、PR33d で新規追加するコメントは状態ベース表現で記述）
- [x] **残した TODO とその理由**: B 状態ベース（後回し事項表）が work-log §「PR33d 後回し事項」に集約。コードコメントには未来 PR 番号や「後で実装」を新規追加していない
- [x] **先送り事項の記録**: 全て work-log §「PR33d 後回し事項」+ roadmap PR33d/PR33e 章に明記
- [x] **generated file 未反映コメント**: 該当なし（sqlc 再生成不要、sqlcgen 触っていない）
- [x] **Secret 漏洩 grep 0 件**: Cloud Run Job logs / chat / work-log / commit / failure-log すべて値ベース 0 件

## Secret 漏洩がないこと

- DATABASE_URL 完全値: 一時 `/tmp/dsn-local.txt`（chmod 600）に置いて Go script に渡し、実値を chat / log / work-log / git に書かなかった。検証完了後ファイル削除
- R2 credentials 実値: 一切扱っていない（R2 PUT は Cloud Run Job 内で完結、handler logs / Job logs に値出力なし）
- raw draft / manage token: STOP ι helper 内で破棄、stdout / chat / log / work-log / commit に出さず
- storage_key 完全値: 一切表示せず（`octet_length()` で長さのみ確認、105 文字）
- Cookie / Set-Cookie / presigned URL / Bearer token: 該当なし（PoC は API / Workers / Backend lookup の HTTP 経路のみで cookie は不要）
- Cloud Run Job logs Secret grep: `secretKeyRef` の名前ヒットのみ（環境変数定義の構造、値なし）
- shell history / tmp file: すべて削除済

## 実施しなかったこと（PR33d 範囲外、別 PR ライン）

- **Cloud Scheduler 作成**（STOP λ）: 本 PR33d は **手動 Job execute** で運用、Scheduler は別判断
- **Reconcile / `ogp_stale.sh`**: PR33e（任意）
- **SNS validator / Safari 実機確認**: 公開 photobook 初出のタイミング（運用フェーズ）
- **Cloud SQL 削除 / spike 削除 / Public repo 化**: 別 PR ライン
- **Email Provider 連携 / ManageUrlDelivery**: PR32c 以降

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版。PR33d 完了記録（STOP θ / ι / κ、初回失敗→patch→再実行 SUCCESS 含む） |
