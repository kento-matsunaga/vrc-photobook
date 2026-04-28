# PR33c OGP public 配信経路 実装結果（2026-04-28）

## 概要

- 新正典 PR33 / `docs/plan/m2-ogp-generation-plan.md` PR33c 計画に従い、SNS crawler が
  R2 public OFF を維持したまま OGP 画像を取得できる経路を実装
- **Backend**: `/api/public/photobooks/{id}/ogp` lookup endpoint + `GenerateOgp` 完了化
  （images / image_variants + MarkGenerated）+ public payload に photobook_id / slug 追加
- **Frontend**: `/ogp/[photobookId]` Workers proxy（R2 binding 経由）+ `generateMetadata`
  更新（og:image / twitter:card）+ default OGP placeholder
- **Workers**: `wrangler.jsonc` に `OGP_BUCKET` R2 binding 追加（bucket は public OFF
  を維持、binding は IAM 完結）
- STOP δ (Workers redeploy) / STOP ε (Backend Cloud Build deploy) 完了
- **STOP ζ（実 OGP 生成 + public 取得 PoC）はスキップ**（後述）
- **generated OGP の public 配信実機確認 / SNS validator / Safari 実機確認は PR33d 持ち越し**

## ファイル追加 / 更新（commit `6eaf855`）

### Backend

| ファイル | 役割 |
|---|---|
| `backend/internal/ogp/internal/usecase/generate_ogp.go` | UseCase に完了化（images / image_variants 同 TX 作成 + MarkGenerated）を追加 |
| `backend/internal/ogp/internal/usecase/get_public_ogp.go`（新規） | photobook_id → status / version / storage_key を返す UseCase。public 配信判定（status=published / visibility=public / hidden=false）|
| `backend/internal/ogp/interface/http/public_handler.go`（新規） | GET `/api/public/photobooks/{photobookId}/ogp` ハンドラ。fallback JSON / image_url_path 構築 |
| `backend/internal/ogp/wireup/http_wireup.go`（新規） | cmd/api 用 facade |
| `backend/internal/ogp/infrastructure/repository/rdb/queries/ogp.sql` | `CreateOgpImageRecord` / `CreateOgpImageVariant` / `GetOgpDeliveryByPhotobookID` 追加 |
| `backend/internal/ogp/infrastructure/repository/rdb/ogp_repository.go` | `CreateOgpImageAndVariant` / `GetDeliveryByPhotobookID` / `OgpDelivery` 構造体追加 |
| `backend/internal/photobook/internal/usecase/get_public_photobook.go` | `PublicPhotobookView` に `PhotobookID` / `Slug` 追加 |
| `backend/internal/photobook/interface/http/public_handler.go` | `publicPhotobookPayload` に `photobook_id` / `slug` JSON フィールド追加 |
| `backend/internal/http/router.go` | `OgpPublicHandlers` 受け取り + `/api/public/photobooks/{photobookId}/ogp` ルート追加 |
| `backend/cmd/api/main.go` | OgpPublicHandlers を組み立てて router に渡す |
| `backend/cmd/ogp-generator/main.go` | コメントを PR33c 完了仕様に更新（PR33b 持ち越し記述削除） |
| `backend/README.md` | photobook_ogp_images の実装済表記に更新 |

### Frontend

| ファイル | 役割 |
|---|---|
| `frontend/wrangler.jsonc` | `r2_buckets: [{binding: OGP_BUCKET, bucket_name: vrcpb-images}]` 追加 |
| `frontend/cloudflare-env.d.ts`（新規） | `CloudflareEnv.OGP_BUCKET` の最小型 + `OgpR2Bucket` interface（@cloudflare/workers-types 未 install のため local declare） |
| `frontend/app/ogp/[photobookId]/route.ts`（新規） | Workers Route Handler。Backend lookup → R2 list+get → image/png + Cache-Control 86400 / fallback 302 → `/og/default.png` |
| `frontend/app/(public)/p/[slug]/page.tsx` | `generateMetadata` で og:image / og:url / twitter:card summary_large_image を 1200×630 で出力 |
| `frontend/lib/publicPhotobook.ts` | `PublicPhotobook` 型に `photobookId` / `slug` 追加、payload mapper 更新 |
| `frontend/public/og/default.png`（新規） | 1200×630 デフォルト OGP（Backend renderer で wordmark のみ生成、15886 bytes、OFL Noto Sans JP） |

### test 修正

`backend/internal/ogp/internal/usecase/generate_ogp_test.go` の `TestGenerate_Success`
期待値を PR33b 仕様（Generated=false / status=pending）から PR33c 仕様（Generated=true /
status=generated / image_id 設定 / generated_at 設定）に更新。

## STOP δ: Workers R2 binding + Workers redeploy 結果

| 観点 | 結果 |
|---|---|
| `npm run cf:build` | 成功（OpenNext worker 出力 `.open-next/worker.js`） |
| `wrangler deploy` | 成功 |
| 新 Worker Version ID | `b966c234-2605-4343-b03a-1ca6cbb0c534` |
| upload | 4 new + 21 既存 / total 4433 KiB / gzip 921 KiB |
| Worker bindings | `env.OGP_BUCKET (vrcpb-images)` R2 Bucket / `env.ASSETS` Assets（OGP_BUCKET 認識） |
| `https://app.vrc-photobook.com/og/default.png` | 200 / image/png / 15886 bytes |
| `https://app.vrc-photobook.com/ogp/<dummy-uuid>` | 302 → `Location: /og/default.png` / `x-robots-tag: noindex, nofollow`（fallback 動作確認） |
| `/` / `/help/manage-url` / `/p/<bad-slug>` | 200 / 200 / 404（既存 route 影響なし） |
| R2 bucket public 設定 | **OFF を維持**（PR33c で bucket 設定変更なし、binding 追加のみ） |
| Secret / R2 credentials / storage_key 漏洩 | 0 件（binding は IAM 完結、deploy ログ / Workers レスポンスに値出力なし） |
| Dashboard 操作 | 不要（wrangler.jsonc 設定で完結） |

## STOP ε: Backend Cloud Build deploy 結果

| 観点 | 値 |
|---|---|
| Build ID | `204982ce-0367-4508-870f-dd9d52bdcada` |
| Build duration | 3M5S |
| Cloud Build 5 steps | build / push / deploy / **traffic-to-latest** / smoke すべて SUCCESS |
| 新 image | `vrcpb-api:6eaf855` |
| 新 revision | `vrcpb-api-00015-j8t`（traffic 100%、`latestReadyRevisionName == status.traffic[0].revisionName`） |
| /health / /readyz | 200 / 200 |
| 認可 4 経路 no Cookie | 全 401 |
| **新 OGP endpoint（zero uuid）** | 404 + `{"status":"not_found","version":0,"image_url_path":"/og/default.png"}` |
| **新 OGP endpoint（bad uuid）** | 404 + 同上 fallback JSON |
| 既存 `/api/public/photobooks/<unknown-slug>` | 404 不変 |
| env / secretKeyRef | **9 個維持** |
| ogp-generator binary 同梱 | ✓（Build logs Step 6/14 + Step 11/14 で確認、不変） |
| Cloud Build logs Secret 漏洩 | 0 件（229 行） |
| Cloud Run logs Secret 漏洩（新 revision） | 0 件（24 行） |

## STOP ζ: 実 OGP 生成 + public 取得 PoC（**スキップ**）

### スキップ理由

ローカル CLI で対象 photobook 選定を試行したところ、本番 Cloud SQL `vrcpb-api-verify` の
**published photobook の visibility 内訳**は以下:

| 状態 | 件数 |
|---|---|
| published / **public** | **0** |
| published / unlisted | 1 |
| published / private | 2 |

PR33c の OGP lookup endpoint（`internal/ogp/internal/usecase/get_public_ogp.go`）は
`status=published AND visibility=public AND hidden_by_operator=false` のみを generated
配信し、それ以外は `status='not_public'` で fallback に倒す設計（計画書 §11、cross-cutting/
ogp-generation.md §6）。本番 DB に **public な published photobook が 0 件**のため、
generated 経路の実機検証ができない状況。

### 検討した選択肢と判断

| 案 | 内容 | 判断 |
|---|---|---|
| A. unlisted で実行（DB / images / R2 PUT のみ確認、配信は not_public 倒し） | 内部整合だけ確認、実導線（Backend → Workers → og:image 200）は未達成 | **採用しない** |
| B. unlisted の photobook を一時的に visibility='public' に UPDATE → PoC → 元に戻す | 作成者意図と異なる公開範囲への一時変更で、たとえ短時間でもリスクあり | **採用しない**（ユーザー判断） |
| C. テスト用 photobook を新規 publish（visibility='public'）で作成 | 本番 DB に運営判断 photobook を 1 件追加することになり、PR33c 範囲として過剰 | **採用しない**（ユーザー判断） |
| **D. STOP ζ をスキップして PR33c 完了、PR33d で OGP 自動生成 + 実 publish 経路と組で検証** | 最も低リスク、検証タイミングを後ろ倒しするだけ | **採用** |

### スキップ前のクリーンアップ

cloud-sql-proxy 停止 / 一時 Go script (`/tmp/stopz-pick.go` / `/tmp/stopz-vis.go`) 削除 /
env から DATABASE_URL / R2_* / PG_* / DSN_LOCAL 全 unset / shell history clear すべて完了。
本番 DB / R2 への書き込みは **行っていない**（visibility=public な対象不在で UseCase 実行を
そもそも起動していない）。

## PR33d への引き継ぎ事項

PR33d（Outbox handler 連携 + Cloud Run Jobs 作成）の着手時に以下を併せて実施:

1. generated OGP の public 配信実機確認（publish 経路または運営判断のテスト photobook 作成）
   - `cmd/ogp-generator --photobook-id <UUID>` 実行 → status='generated' 確認
   - `curl https://api.vrc-photobook.com/api/public/photobooks/<UUID>/ogp` → 200 + status=generated + image_url_path=/ogp/<UUID>?v=1
   - `curl -A "Twitterbot/1.0" -I https://app.vrc-photobook.com/ogp/<UUID>?v=1` → 200 + image/png + Cache-Control: public, max-age=86400
   - `/p/<slug>` HTML に正しい og:image / twitter:card が含まれること
2. SNS validator（X Card Validator / Discord / Slack / LINE）プレビュー確認
3. Safari / iPhone Safari 実機確認（`.agents/rules/safari-verification.md` の §2 観点）

これらは新正典 §3 PR33 と本計画書 §12 の確認項目に組み込まれており、PR33d で消化する。

## 検証可能な範囲（curl 自動実行）

PR33c のコード経路自体は STOP δ + STOP ε の deploy で全て smoke 通過済:

- `/og/default.png` 200 / image/png / 15886 bytes
- `/ogp/<dummy-uuid>` 302 → `/og/default.png`（fallback 動作）
- `/api/public/photobooks/<zero-uuid>/ogp` 404 + fallback JSON
- `/api/public/photobooks/<bad-uuid>/ogp` 404 + fallback JSON
- 既存 route（`/`, `/help/manage-url`, `/p/<bad-slug>`, `/health`, `/readyz`, 認可 4 経路）影響なし
- Workers binding `env.OGP_BUCKET (vrcpb-images)` 認識
- env / secretKeyRef 9 個維持

## Test 結果

| 範囲 | 結果 |
|---|---|
| Backend `go vet ./...` / `go build ./...` | クリーン |
| Backend `internal/photobook/...` test（DB あり） | 全 pass |
| Backend `internal/ogp/...` test（DB あり、`-p 1`） | 全 pass（VO / renderer / Repository / UseCase Success+NotPublished+R2PutFailure+DryRun） |
| Frontend `npm run typecheck`（tsc --noEmit） | クリーン |
| Frontend `npm run test`（vitest） | 9 files / 92 tests 全 pass |
| Frontend `npm run build`（Next.js） | `/ogp/[photobookId]` route 認識 / `/help/manage-url` 等の既存 static / dynamic 不変 |
| Frontend `npm run cf:build`（OpenNext） | Worker 出力 `.open-next/worker.js` 生成成功 |

## PR closeout チェックリスト（pr-closeout.md §6）

- [x] **コメント整合チェック実施**: `bash scripts/check-stale-comments.sh --extra "OGP|og:image|twitter:card|SNS|crawler|R2 public|public URL|Cloud Run Jobs|Scheduler|ogp-generator|Workers proxy"`
- [x] **古いコメント修正**: `cmd/ogp-generator/main.go` の PR33b 持ち越し記述（「renderer + R2 PUT までで停止」「images row 作成 / MarkGenerated は PR33c で」）を PR33c 完了仕様に更新（A 修正）。`backend/README.md` の `photobook_ogp_images はまだ未実装` を実装済表記に更新（A 修正）
- [x] **残した TODO とその理由**:
  - C 過去経緯: `internal/image/.../storage_key.go:132` 「PR18 では ogp_id は別集約」（既存、変更なし）
  - C 過去経緯: queries/ogp.sql の `PR33b: ...` `PR33c: ...` 履歴コメント
  - B 状態ベース: queries/ogp.sql の `PR33d で worker 連携時に再評価`
  - D generated: 該当なし（sqlc 再生成済）
- [x] **先送り事項の記録**:
  - PR33d（Outbox handler + Cloud Run Jobs）: 新正典 §3 PR33 / `m2-ogp-generation-plan.md` 履歴 / 本 work-log に明記
  - **STOP ζ スキップ理由 + PR33d 持ち越し検証項目**: 同上に明記
  - SNS validator / Safari 実機確認: 同上、`.agents/rules/safari-verification.md` の運用ルールでカバー
- [x] **generated file 未反映コメント**: 該当なし（sqlc 再生成済）
- [x] **Secret 漏洩 grep**: 実値 0 件（全て禁止リスト記述コメント）

## Secret 漏洩がないこと

- DATABASE_URL / R2 credentials 実値: STOP δ / ε / ζ いずれでも一時 env 経由のみ、unset 済、commit / chat / log / work-log に未含有
- 対象 photobook UUID（STOP ζ で選定試行したが 0 件で実行せず）: 環境変数すら使わず終了、shell history clear
- storage_key 完全値: 一切生成していない（STOP ζ で UseCase 実行せず、R2 PUT も発生せず）
- Workers binding は IAM 完結で credentials を露出しない（deploy ログ / レスポンスに 0 件）
- Cloud Build logs / Cloud Run logs Secret grep 0 件
- 新規追加コードに実値の Secret は **0 件**（全て禁止リスト記述コメント / sanitize 対象列挙）
- shell history / tmp file 全削除済

## 実施しなかったこと（PR33c 範囲外、PR33d 以降）

- **STOP ζ 実 OGP 生成 + public 取得 PoC**（本番に public な published photobook が 0 件のためスキップ）
- generated 状態での Backend lookup → Workers proxy → og:image 200 の実機検証（PR33d 持ち越し）
- SNS validator（X Card Validator / Discord / Slack / LINE）プレビュー確認（PR33d 持ち越し）
- macOS Safari / iPhone Safari 実機確認（PR33d 持ち越し）
- Outbox handler 接続（`photobook.published` consume → OGP 自動生成）→ PR33d
- Cloud Run Jobs 作成 → PR33d（**副作用 handler 初回稼働の最重要 STOP**）
- Cloud Scheduler 作成 → PR33d（別 STOP）
- Email Provider 連携 / ManageUrlDelivery → 別 PR ライン（PR32c 以降）
- Reconcile（stale_ogp_enqueue / 手動 ogp_stale.sh）→ PR33e（任意）
- Cloud SQL 削除 / spike 削除 / Public repo 化

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（PR33c 一部完了）。Backend public OGP endpoint + GenerateOgp 完了化 + Workers R2 binding + `/ogp/<id>` proxy + Frontend metadata + default OGP placeholder。STOP δ (Workers redeploy) / STOP ε (Backend deploy `vrcpb-api-00015-j8t`) すべて完了。**STOP ζ はスキップ**（本番 DB に public な published photobook が 0 件で generated 配信実機検証不可、unlisted 強制公開 / テスト photobook 新規作成は採用しない判断）。**generated OGP の public 配信実機確認 / SNS validator / Safari 実機確認は PR33d 持ち越し** |
