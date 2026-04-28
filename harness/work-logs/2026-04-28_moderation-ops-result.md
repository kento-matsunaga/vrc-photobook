# PR34b Moderation / Ops 実装結果（2026-04-28〜2026-04-29 完了）

## 本書のスコープ

PR34b（Moderation / Ops MVP 実装）の最終結果。Cloud SQL migration 適用 + Backend
Cloud Build deploy + Cloud Run Job image 更新 + Frontend manage UI banner +
Workers redeploy + 本番 cmd/ops smoke + runbook 整備まで完了。Safari / iPhone Safari
実機の **manage UI banner レンダリング確認のみ未実施**（後続持ち越し、§13）。

## 概要

- 新正典 PR34 / `docs/plan/m2-moderation-ops-plan.md` PR34b 計画に従い、運営者が
  公開済み photobook を CLI で `hide / unhide / show / list-hidden` できる経路を
  整備
- Cloud SQL に migration `00014_create_moderation_actions.sql` /
  `00015_relax_outbox_event_type_check.sql` を適用
- Backend Cloud Build manual submit で `vrcpb-api:0db0d7c` を deploy、
  `vrcpb-api-00017-hbg` revision に traffic 100%
- Cloud Run Job `vrcpb-outbox-worker` の image を `vrcpb-api:0db0d7c` に更新
  （args / SA / Secret refs / cloudsql-instances / max-retries / parallelism /
  taskCount は維持）
- 初回 Cloud Build が `unable to prepare context: path "backend" not found` で
  失敗。原因は `docs/runbook/backend-deploy.md` §1.2 のサンプルが古く、
  source として backend/ を渡してしまった事象。**repo root submit に修正して再実行
  → SUCCESS**

## 完了済み（commit 1〜3）

### commit 1 `5576656 feat(backend): add moderation domain foundation`

- migration 2 本（`moderation_actions` 新設 + `outbox_events.event_type` CHECK 拡張）
- domain entity / VO 5 種（ActionID / ActionKind / ActionReason / OperatorLabel / ActionDetail）
- table-driven test、全 pass
- sqlc.yaml に moderation set 追加

### commit 2 `0db0d7c feat(backend): add moderation ops usecases and cli`

- `internal/moderation/internal/usecase`: Hide / Unhide / GetForOps / ListHidden の 4 種
  - 同一 TX 4 要素（photobooks UPDATE + moderation_actions INSERT + outbox_events INSERT）
  - version は上げない（編集 OCC を壊さない、計画書 §5.6 / ユーザー判断 #5）
  - status='published' 以外は拒否（計画書 §13 #4）
  - 冪等エラー（ErrAlreadyHidden / ErrAlreadyUnhidden）は cmd/ops が exit 0
- `internal/moderation/infrastructure/repository/rdb`: ModerationActionRepository
  （append-only Insert / ListRecentByPhotobook、sqlc query）
- `internal/moderation/wireup`: cmd/ops 用 facade（Input/Output 型 / sentinel エラー
  re-export）
- `internal/photobook/infrastructure/repository/rdb`: SetHiddenByOperator /
  GetForOps / ListHiddenForOps を追加（sqlc 再生成済）
- `internal/outbox`: payload に PhotobookHiddenPayload / PhotobookUnhiddenPayload
  追加。event_type VO に PhotobookHidden / PhotobookUnhidden 追加（migration
  00015 の CHECK 拡張と一致）。handlers に no-op + log 追加し wireup で Register
- `cmd/ops/main.go`: photobook show / list-hidden / hide / unhide。`--dry-run`
  既定、`--execute` 明示、確認プロンプト、`--yes` で CI skip、`--actor` 必須、
  raw token / Cookie / manage URL / storage_key 完全値は表示しない、
  DATABASE_URL は env 経由のみ
- domain test (commit 1) + outbox event_type test 拡張、全 pass

### commit 3（本書）

- 本 work-log の途中経過記録（commit 4 以降で frontend / runbook / smoke の章を追記）

## STOP α: Cloud SQL migration 適用結果

### ローカル goose 検証

| 観点 | 結果 |
|---|---|
| `goose status`（適用前） | local Postgres は v13 / 00014・00015 Pending |
| `goose up`（00014 / 00015）| 成功（61.48ms / 11.95ms）|
| `moderation_actions` table | column 10 / INDEX 7（PK 含む）/ CHECK 4 すべて期待通り |
| `outbox_events_event_type_check` | 5 種に拡張済（`photobook.published` / `photobook.hidden` / `photobook.unhidden` / `image.became_available` / `image.failed`）|
| `goose down` x2 | 成功（00015 で CHECK 3 種に戻り、00014 で table DROP）|
| `goose up` 再実行 | 成功（v15 復帰）|

### Cloud SQL 適用

| 観点 | 値 |
|---|---|
| 接続 | cloud-sql-proxy v2.13.0 経由（127.0.0.1:5433）|
| 適用前 status | v13 / 00014・00015 Pending |
| `goose up` | 成功（00014: 244.97ms、00015: 43.05ms）|
| 適用後 status | 全 Applied（最新 v15）|
| 既存 outbox_events 行（PR33d 残骸 1 件） | 保持 |
| 既存 photobooks 行 | 12 件保持 |
| `moderation_actions` 既存行 | 0（適用直後の期待通り）|

### Cleanup

- cloud-sql-proxy 停止 / port 5433 解放
- 一時 DSN ファイル / 検証 Go script 削除
- DATABASE_URL 値 / R2 credentials / token / Cookie / storage_key 完全値: chat / log / commit に値出力 0 件

## STOP β: Backend Cloud Build deploy 結果

### 1 回目（FAILURE）

| 観点 | 値 |
|---|---|
| Build ID | `5ce38f50-2937-403a-9c51-ad33824f0694` |
| 結果 | FAILURE（step #0 build）|
| エラー | `unable to prepare context: path "backend" not found` |
| 原因 | runbook §1.2 のサンプルが古く、source として `/home/erenoa6621/dev/vrc_photobook/backend` を渡したが、cloudbuild.yaml は repo root context（`-f backend/Dockerfile` + context `backend`）前提のため不一致 |
| 副作用 | なし（image push 0、Cloud Run / Cloud SQL / Job / Secret 影響なし、既存 revision `vrcpb-api-00016-9ln` のまま）|

PR29 deploy automation の work-log §「修正 2: build context path の不一致」と同じ
事象で、commit `50f940c` で `.gcloudignore` を追加し repo root submit に切り替え
たことで一度解決済み。runbook §1.2 のサンプルだけが古い記述のまま残っていた。

### 2 回目（SUCCESS、修正版コマンド）

修正点: `gcloud builds submit` の source を `/home/erenoa6621/dev/vrc_photobook`（repo root）に変更（`.gcloudignore` で frontend / docs / harness 等を除外して context 最小化）。

| 観点 | 値 |
|---|---|
| Build ID | `a0f05816-a67a-48aa-b396-07da507ade1f` |
| Duration | 3M35S |
| 5 steps（build / push / deploy / traffic-to-latest / smoke） | **すべて SUCCESS** |
| Image tag | `vrcpb-api:0db0d7c` |
| 旧 revision | `vrcpb-api-00016-9ln`（PR33d、image `fe19ab5`、rollback 先）|
| 新 revision | `vrcpb-api-00017-hbg` |
| traffic 100% | `vrcpb-api-00017-hbg`（`latestReadyRevisionName == status.traffic[0].revisionName` 一致確認済）|

### Smoke 検証

| 項目 | 期待 | 結果 |
|---|---|---|
| `/health` | 200 | 200 ✓ |
| `/readyz` | 200 | 200 ✓ |
| `/api/public/photobooks/<unknown-slug>` | 404 | 404 ✓ |
| `/api/public/photobooks/<dummy-uuid>/ogp` | 404 + fallback | 404 ✓ |
| `/api/photobooks/<dummy>/edit-view` (no Cookie) | 401 | 401 ✓ |
| `/api/photobooks/<dummy>/manage-view` (no Cookie) | 401 | 401 ✓ |
| env / secretKeyRef 件数 | 9 件維持 | **9 件**（APP_ENV / ALLOWED_ORIGINS / DATABASE_URL / R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET_NAME / R2_ENDPOINT / TURNSTILE_SECRET_KEY）|
| Cloud Build logs Secret 漏洩 grep | 0 件 | 0 件 |
| Cloud Run logs Secret 漏洩 grep（新 revision 直近 50 行） | 0 件 | 0 件 |

### Cloud Run Job vrcpb-outbox-worker image 更新

`--image=` 単独で更新、他は不変:

| 観点 | 値 |
|---|---|
| 更新前 image | `vrcpb-api:fe19ab5`（PR33d）|
| 更新後 image | `vrcpb-api:0db0d7c` |
| command | `/usr/local/bin/outbox-worker`（維持）|
| args | `--once --max-events 1 --timeout 60s`（維持）|
| serviceAccountName | `271979922385-compute@developer.gserviceaccount.com`（維持）|
| Secret refs | DATABASE_URL + R2_* 全 6 件（維持）|
| `run.googleapis.com/cloudsql-instances` annotation | `<PROJ>:asia-northeast1:vrcpb-api-verify`（維持）|
| maxRetries / parallelism / taskCount | 0 / 1 / 1（維持）|
| Job execution 数 | 2（PR33d 時の `vrcpb-outbox-worker-jdfh9` + `znx4v`、新規実行なし）|
| Cloud Scheduler | 未作成（gcloud scheduler jobs list で 0 件、計画書 §3.2 通り）|

## STOP γ: Frontend manage UI banner + Workers redeploy

### 実装内容（commit 4）

| ファイル | 変更 |
|---|---|
| `frontend/components/Manage/HiddenByOperatorBanner.tsx`（新規）| hiddenByOperator=true 用 banner（`role="status"` / `data-testid="hidden-by-operator-banner"` / `status-warning` トーン）|
| `frontend/components/Manage/ManagePanel.tsx`（編集）| hiddenByOperator=true で banner 表示。既存の compact status badge は残す。編集 / 再発行 placeholder はブロックしない |
| `frontend/components/Manage/__tests__/HiddenByOperatorBanner.test.tsx`（新規）| `renderToStaticMarkup` で SSR 検証。banner 文言 / aria role / true→表示・false→非表示 / 既存 UI 不変、5 ケース全 pass |
| `frontend/vitest.config.ts`（編集）| `esbuild.jsx: "automatic"` を追加（vitest 用 JSX runtime） |

### test 結果

| 項目 | 結果 |
|---|---|
| `npm run typecheck` | クリーン |
| `npm run test`（vitest）| 97 tests / 10 files 全 pass（既存 92 + 新規 5）|
| `npm run build`（Next.js）| 全ルート成功 |
| `npm run cf:build`（OpenNext）| Worker 出力 `.open-next/worker.js` 生成成功 |

### Workers redeploy 結果

| 観点 | 値 |
|---|---|
| `wrangler deploy` | SUCCESS |
| 新 Worker version ID | `e97148fe-283f-4c64-9765-37ad10bdd29e` |
| 旧 Worker version ID（rollback 先）| `b966c234-2605-4343-b03a-1ca6cbb0c534`（PR33c）|
| Upload | 1 new + 24 既存 / 4434.98 KiB / gzip 922.15 KiB |
| Worker bindings | `env.OGP_BUCKET (vrcpb-images)` / `env.ASSETS` 両方維持 |

### Smoke 検証（既存ルート不変確認）

| 経路 | 結果 |
|---|---|
| `https://app.vrc-photobook.com/` | 200 |
| `https://app.vrc-photobook.com/help/manage-url` | 200 |
| `https://app.vrc-photobook.com/p/<bad-slug>` | 404 |
| `https://app.vrc-photobook.com/og/default.png` | 200 / image/png / 15886 bytes |
| `https://app.vrc-photobook.com/ogp/<dummy>?v=1` | 302 / Location: /og/default.png / x-robots-tag: noindex,nofollow |
| `/manage/<dummy>` Cookie 無し | unauthorized 表示 HTML |
| Worker response Secret 漏洩 grep | 0 件 |

## STOP δ: 本番 cmd/ops 実機 hide / unhide smoke

### 採用方針

PR33d STOP κ で作成済の test photobook（`hidden_by_operator=true` 状態）を流用し、
**unhide → 公開挙動確認 → hide で元に戻す**。新規 photobook は作成せず、最終状態は
PR34b 開始前と同じ hidden=true。

### 実行順 + 結果（成功）

| Step | 内容 | 結果 |
|---|---|---|
| 1 | cloud-sql-proxy 起動（127.0.0.1:5433）+ 一時 DSN ファイル（chmod 600）| ✓ |
| 2 | `cmd/ops photobook show --id <PID>` | status=published / hidden=true / version=1 / mod_actions 0 件 |
| 3 | dry-run unhide（reason=erroneous_action_correction / actor=ops-1）| DB 更新なし、planned summary 表示 |
| 4 | execute unhide（プロンプト yes）| 成功、新 action_id 生成 |
| 5a | DB 検証（unhide 後）| photobooks: hidden=false / version=1（不変）/ mod_actions: unhide 1 行 / outbox: photobook.unhidden pending +1 |
| 5b | 公開挙動（unhide 後）| `/api/public/photobooks/<SLUG>` 200 / OGP `status=generated` / Workers `/ogp` 200 image/png / `/p` 通常 og:title・twitter:card・noindex |
| 8 | dry-run hide（reason=policy_violation_other）| DB 更新なし、unhide が直近 mod history に表示される |
| 9 | execute hide（プロンプト yes）| 成功、新 action_id 生成 |
| 10a | DB 検証（hide 後）| photobooks: hidden=true / version=1（不変）/ mod_actions: hide+unhide 2 行（append-only）/ outbox: photobook.hidden pending +1 |
| 10b | 公開挙動（hide 後）| `/api/public/photobooks/<SLUG>` 410 gone / OGP `not_public` + fallback / Workers `/ogp` 302 → `/og/default.png` / `/p` gone テンプレ + noindex |
| 13a | `cmd/ops photobook list-hidden` | target が hidden 一覧に含まれる |
| 13b | `cmd/ops photobook show` 最終 | mod_actions 直近 2 件に hide / unhide 表示 |
| 14a | Job execute 1 回目（`vrcpb-outbox-worker-wpn5q`、`--once --max-events 1`）| picked=1 / processed=1 / handler `outbox handler: photobook.unhidden (no-op)` 動作 |
| 14b | Job execute 2 回目（`vrcpb-outbox-worker-x54nm`）| picked=1 / processed=1 / handler `outbox handler: photobook.hidden (no-op)` 動作 |
| 14c | pending event 残数 | 0 |
| 14d | 全 outbox event status | photobook.published / unhidden / hidden 全 processed（3/3）|
| 15 | cleanup | proxy 停止 / 一時 DSN ファイル削除 / 一時 Go script 削除 / 一時 PNG 削除 |

### 最終 photobook 状態

PR34b 開始前と同じ hidden_by_operator=true。version 不変（1）。R2 OGP object も保持。

### Job logs Secret 漏洩 grep

- `wpn5q` / `x54nm` 両 execution の Cloud Run Job logs を grep（DATABASE_URL=postgres / R2_SECRET_ACCESS_KEY= / TURNSTILE_SECRET_KEY= / sk_live / raw_token / Bearer / manage_url_token / storage_key）→ **0 件**

## closeout 実施内容（commit 5、本 commit）

- **`harness/failure-log/2026-04-29_runbook-backend-deploy-section-outdated.md` 起票**
  （PR34b STOP β 初回失敗の根本原因として、runbook §1.2 が PR29 解決前の記述のまま残置されていた事実を記録）
- **`docs/runbook/backend-deploy.md` §1.2 を実運用（repo root submit）に修正** + §5.8 に「`unable to prepare context: path "backend" not found` 失敗時の対処」を追加
- **`docs/runbook/ops-moderation.md` 新規作成**（cmd/ops 運用手順、reason 許容セット、rollback、Secret 漏洩確認、よくある失敗）
- **`docs/plan/vrc-photobook-final-roadmap.md` を PR34b 完了状態に更新**
- 本 work-log を完成
- stale-comments チェック + Secret grep 実施

## 後続持ち越し（PR34b 範囲外）

| 項目 | 持ち越し先 |
|---|---|
| **macOS Safari / iPhone Safari で hidden=true manage page banner の実機表示確認** | PR34c（reissue_manage_url 実装後）または運用フェーズで新 photobook 出現時 |
| **横幅崩れ / タップ可能要素崩れの実機確認** | 同上 |
| `soft_delete` / `restore`（論理復元） / `purge` UseCase | PR34 拡張 / 別 PR |
| `reissue_manage_url`（管理URL 再発行 + Session revoke + ManageUrlDelivery）| Email Provider 確定後（PR32c 以降）|
| Report 集約 / 自動連動 | PR35 |
| Web admin UI / dashboard / HTTP endpoint | MVP 範囲外（v4 §6.19）|
| 作成者通知メール（hide / unhide 時）| Email Provider 確定後 |
| OGP 自動再生成 / R2 stale cleanup | PR33e（任意）|
| 複数運営者対応（OperatorId 化）| 単一運用前提を脱するときに再検討 |
| Cloud Scheduler 作成（outbox-worker 自動回し）| 当面は手動 Job execute、PR33e で要否判断 |

### Safari 実機未確認の判断理由（§13）

確認済み事項（API + コード経由で代替）:
- Backend `manage_handler.go` line 43 / 93 が JSON に `hidden_by_operator` を含めて creator に返すこと
- Frontend `lib/managePhotobook.ts` line 91 / 108 が `hiddenByOperator: boolean` で camelCase mapping すること
- `ManagePanel` が `hiddenByOperator=true` のとき banner 表示 / false で非表示にすることをユニットテスト 5 ケースで保証
- 既存 UI（status badge / 編集 / 再発行 placeholder）が壊れていないことをユニットテスト + Workers redeploy smoke で確認

未確認事項:
- 実機 Safari でのモバイル UI 表示崩れ / 横画面 / タップ可能要素

スコープ判断:
- raw manage URL は PR33d STOP ι で生成・破棄済、再表示経路（reissue_manage_url）は PR34b 範囲外
- creator session を作るために PR34b 内で reissue_manage_url を MVP 拡張する選択肢もあるが、Email Provider 未確定（ADR-0006）+ Session revoke 機構の test 拡張が必要でスコープ拡大が大きい
- 機能の核（hide / unhide / audit log / 同 TX 4 要素 / 公開導線への効果）は本 STOP δ smoke で完全に検証済
- Safari 実機は **副次的な UI 確認**のため、後続作業に持ち越しても本番リスクは小さい

## PR closeout チェックリスト（pr-closeout.md §6）

- [x] **コメント整合チェック**: `bash scripts/check-stale-comments.sh --extra "Moderation|Ops|hidden_by_operator|Report|abuse|admin|operator|restore|takedown|PhotobookHidden|PhotobookUnhidden"` 実行
- [x] **古いコメント修正**: PR34b で新規追加するコードコメントは固定 PR 番号 + 未来形を避けて状態ベースで記述。既存コメントの stale ヒットは全て C 区分（過去経緯 / ルール文書例示）として残置
- [x] **runbook §1.2 修正**: 実運用（repo root submit）に揃え、§5.8 に対処手順追加
- [x] **failure-log 起票**: `2026-04-29_runbook-backend-deploy-section-outdated.md`
- [x] **新規 runbook**: `docs/runbook/ops-moderation.md` 作成（cmd/ops MVP 手順）
- [x] **roadmap 更新**: PR34b 完了反映 + Safari 後続持ち越し記録
- [x] **残した TODO**: 全項目を §13 後続持ち越しに集約、固定 PR 番号 + 未来形コメントを新規追加していない
- [x] **先送り事項記録**: 計画書 / roadmap / 本 work-log / failure-log に網羅的に記録
- [x] **generated file 未反映**: 該当なし（sqlcgen は本 PR で再生成済、commit 2 に同梱）
- [x] **Secret 漏洩 grep 0 件**: 値ベース 0 件、`secretKeyRef` / `R2_*` 等の名前ヒットのみ

## Secret 漏洩がないこと（commit 1〜5 範囲）

- DATABASE_URL 完全値 / password: 一時 `/tmp/dsn-prod.txt`（chmod 600）に置いて
  Go script に渡し、検証完了後ファイル削除。chat / log / work-log / git に値を
  書いていない
- R2 credentials 実値: 一切扱っていない（hide / unhide では R2 操作なし）
- raw draft / manage token: 一切扱っていない（cmd/ops は token を使わない設計）
- storage_key 完全値: 一切表示せず（Repository / cmd/ops 出力ホワイトリストで除外）
- Cookie / Set-Cookie / presigned URL / Bearer token: 該当なし
- Cloud Build logs / Cloud Run logs Secret grep: 0 件（forbidden patterns）
- shell history / tmp file: cleanup 済

## 実施しなかったこと（commit 1〜3 時点）

- Frontend manage UI banner（commit 4 で実装）
- Workers redeploy（STOP γ で停止予定）
- runbook `docs/runbook/ops-moderation.md`（commit 5 で作成）
- 実機 hide / unhide smoke（STOP δ で停止予定）
- failure-log 起票 / runbook §1.2 修正（closeout で実施）
- Cloud Scheduler 作成（PR34b 範囲外、計画書 §3.2）
- soft_delete / restore（論理復元） / purge / reissue_manage_url（PR34b 範囲外）
- Web admin UI（v4 §6.19、MVP 範囲外）
- 作成者通知メール（Email Provider 未確定、ADR-0006）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（途中経過、commit 3）。STOP α / β 完了 + Cloud Run Job image 更新までを記録 |
| 2026-04-29 | PR34b 完了（commit 5）。STOP γ / δ 結果 + closeout（runbook 整備 / failure-log / roadmap 更新）を追記。Safari 実機 manage UI 確認のみ後続持ち越し |
