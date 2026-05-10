# /edit Phase A + brand icon/themeColor 本番反映 準備ログ（deploy 前停止）

> 状態: **deploy 承認前停止**。本ログは work-log 作成 + γ verification + deploy / rollback / smoke
> 計画固定までを記録する。実 Backend / Workers deploy は未実施。
>
> 次サイクルで本ログを根拠に deploy 実行（各 STOP で停止 → user 承認 → 進行）。

## 0. 関連参照

- 新正典ロードマップ: [`docs/plan/vrc-photobook-final-roadmap.md`](../../docs/plan/vrc-photobook-final-roadmap.md)
- /edit Phase A 計画: [`docs/plan/m2-edit-page-split-and-preview-plan.md`](../../docs/plan/m2-edit-page-split-and-preview-plan.md)
- Backend deploy runbook: [`docs/runbook/backend-deploy.md`](../../docs/runbook/backend-deploy.md)
- 必須 ルール:
  - [`predeploy-verification-checklist.md`](../../.agents/rules/predeploy-verification-checklist.md)
  - [`pr-closeout.md`](../../.agents/rules/pr-closeout.md)
  - [`safari-verification.md`](../../.agents/rules/safari-verification.md)
  - [`cors-mutation-methods.md`](../../.agents/rules/cors-mutation-methods.md)
  - [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
  - [`security-guard.md`](../../.agents/rules/security-guard.md)

## 1. Phase A summary

`m2-edit-page-split-and-preview-plan.md` Phase A スコープを **実装・テストともに完結状態**で本番未反映。

### 1.1 Phase A 機能 (Backend / Frontend / UI / Test)

| # | 機能 | endpoint | response 方式 | UI wire | test |
|---|---|---|---|---|:-:|
| 1 | page caption 編集 | `PATCH /api/photobooks/{id}/pages/{pageId}/caption` | A 方式 (`{version: N+1}`) | `PageCaptionEditor` | ✓ |
| 2 | page split | `POST  /api/photobooks/{id}/pages/{pageId}/split` | B 方式 (更新後 `EditView`) | `PhotoActionBar` | ✓ |
| 3 | photo move-between-pages | `PATCH /api/photobooks/{id}/photos/{photoId}/move` | B 方式 | `PageMovePicker` | ✓ |
| 4 | page merge | `POST  /api/photobooks/{id}/pages/{pageId}/merge-into/{targetPageId}` | B 方式 | `PageActionBar` | ✓ |
| 5 | page reorder | `PATCH /api/photobooks/{id}/pages/reorder` | B 方式 | `ReorderControls` / `PageActionBar` | ✓ |
| – | 同画面 preview | (Backend 不要、`editPreview` helper) | – | `PreviewToggle` / `PreviewPane` | ✓ |

### 1.2 brand icon / themeColor (icon commit)

| 種別 | 内容 |
|---|---|
| icon assets | `frontend/app/icon.png` (32×32) / `icon1.png` (512×512) / `apple-icon.png` (180×180) / `favicon.ico` (16/32/48 multi) |
| source 保管 | `design/source/icon/photobook-icon.png` (1254×1254) + `README.md` |
| themeColor | `frontend/app/layout.tsx` `viewport.themeColor: "#0F2A2E"` (Next.js 15 規約) |
| Next.js auto-detect | `/icon.png` / `/icon1.png` / `/apple-icon.png` ルートは `npm run build` 出力で確認済 |

## 2. commit range

### 2.1 Phase A 6 commits（古い順）

| commit | 内容 | 主な変更 |
|---|---|---|
| `1ddb958` | feat(edit): add repository primitives for page split and photo move | `photobook_pages_repository.go` / `photobook_pages.sql` / sqlcgen |
| `0114847` | feat(edit): add page caption split and photo move mutations | `update_page_caption.go` / `split_page.go` / `move_photo_between_pages.go` / `edit_handler.go` / `router.go` / handler test |
| `01380fa` | feat(edit): add page merge and reorder mutations | `merge_pages.go` / `reorder_pages.go` / `edit_handler.go` / `router.go` / handler test |
| `1d5dead` | feat(edit): add edit-photobook lib functions and preview helper | `lib/editPhotobook.ts` / `lib/editPreview.ts` + tests |
| `eceda06` | feat(edit): wire page caption / split / move into edit UI | `EditClient.tsx` / `PageBlock.tsx` / `PageCaptionEditor.tsx` / `PageMovePicker.tsx` / `PhotoActionBar.tsx` / `PhotoGrid.tsx` + tests |
| `fb7b0d8` | feat(edit): wire merge / page reorder / preview into edit UI | `EditClient.tsx` / `PageActionBar.tsx` / `PageBlock.tsx` / `PreviewPane.tsx` / `PreviewToggle.tsx` + tests |

### 2.2 brand icon 1 commit

| commit | 内容 |
|---|---|
| `37d7744` | feat(brand): add app icons and theme color |

### 2.3 deploy 対象

- **Backend / Workers deploy target: 実行時の current HEAD（== `origin/main`）**
  - 上記 7 commit（Phase A 6 commit + icon 1 commit）+ 本 work-log commit + 以後追加される commit を含む
  - 本 plan の commit 後に新たな commit が加わった場合も、deploy 実行時の HEAD を tag として使う（37d7744 を固定 tag にしない）
- Backend 実装範囲: `1ddb958..01380fa`（3 commit）
- Frontend Workers 実装範囲: `1d5dead`, `eceda06`, `fb7b0d8` の 3 commit + icon commit `37d7744`

> `37d7744` は **icon commit の履歴 ID** として記録するのみ。deploy 対象 image tag は STOP D-1
> 実行時に `git rev-parse --short=7 HEAD` を取得して使う。

## 3. deploy scope

### 3.1 含む

- Backend Cloud Run service `vrcpb-api`: image tag を **deploy 実行時の current HEAD short SHA** に同期し新 revision を 100% traffic に切替（icon commit `37d7744` 以降の commit を含む）
- Cloud Run Jobs image tag 同期（`vrcpb-image-processor` / `vrcpb-outbox-worker`）: 既存 args / annotation / SA / Secret refs / max-retries / parallelism / task-count / `--add-cloudsql-instances` の有無は不変、image tag のみ更新
- Frontend Workers `vrcpb-frontend`: `cf:build` 出力 (`.open-next/`) を `wrangler deploy` で投入

### 3.2 含まない（明示）

- DB migration: **追加なし**。最新 migration は `00018_create_usage_counters.sql`、Phase A は `00009_create_photobook_page_metas.sql` 既存 schema を流用（plan §2.1）
- Secret 追加 / 値変更: **なし**
- env / binding 変更: Cloud Run env (10 entries) / Workers binding (`OGP_BUCKET (R2 vrcpb-images)` / `ASSETS`) は完全不変
- Cloud Scheduler 変更: `vrcpb-image-processor-tick` (`* * * * *` ENABLED) は不変
- CORS `AllowedMethods` 変更: **不要**（PATCH / POST は `a8fe0db` で既に追加済、`cors-mutation-methods` rule 適用済）
- 新 Cloud Run Job / Scheduler 作成: なし
- DNS / Cloudflare Worker route / R2 bucket policy 変更: なし

## 4. verification results（γ verification、2026-05-10 実施）

| check | 結果 |
|---|---|
| `git diff --check` | PASS (exit 0) |
| `bash scripts/check-stale-comments.sh` | hits は CLAUDE.md / README.md ロードマップ既存記述のみ。Phase A / icon と無関係 (区分 C: 過去経緯) |
| raw value grep（manage_url_token / draft_edit_token / Cookie / Set-Cookie / Bearer / storage_key / presigned / sk_live / sk_test / DATABASE_URL= / TURNSTILE_SECRET / R2_SECRET） | 4 file ヒットだが **すべて false positive**。実値混入なし。詳細は §4.2 |
| `go -C backend vet ./...` | PASS (exit 0) |
| `go -C backend build ./...` | PASS (exit 0) |
| `go -C backend test ./...` | PASS（76 ok packages / 65 no-test / **0 FAIL**） |
| `npm --prefix frontend run typecheck` | PASS (tsc --noEmit エラー 0) |
| `npm --prefix frontend run test` | PASS（490 tests / 50 files、約 1.2s） |
| `npm --prefix frontend run build` | PASS。`/icon.png` `/icon1.png` `/apple-icon.png` ルートが auto-detect、`/favicon.ico` は別経路で配信 |
| `npm --prefix frontend run cf:build` | PASS（`Worker saved in .open-next/worker.js`） |
| `npm --prefix frontend run cf:check` | PASS（wrangler dry-run、bindings 維持: `OGP_BUCKET (R2 vrcpb-images)` + `ASSETS`、Total Upload 5507.55 KiB / gzip 1302.97 KiB） |

### 4.1 既知 pre-existing fail の切り分け

| 既知問題 | Phase A 起因か | 状態 |
|---|---|---|
| `session_repository_test.go` の FK 違反（`sessions_photobook_id_fkey`） | **起因しない**（PR36 commit 3.6 以前から残置、roadmap §1.3 に記録済） | DATABASE_URL 未設定環境では Skip するため `go test ./...` で FAIL せず（今回も 0 FAIL） |
| PR36 SubmitReport 専用の実 DB 副作用なしテスト未追加 | 起因しない | 代表保証で済ませた既知件、後続 PR で追加 |

→ **Phase A が原因の test 失敗はゼロ**。

### 4.2 raw value grep ヒットの精査（false positive 確認）

| file | line | 内容 | 判定 |
|---|---|---|---|
| `backend/internal/photobook/infrastructure/repository/rdb/photobook_pages_repository_test.go` | 13 | local-dev DATABASE_URL 例示コメント（`postgres://vrcpb:vrcpb_local@localhost:5432/...?sslmode=disable`） | local-dev hint 文字列、本番 Secret ではない |
| `backend/internal/photobook/interface/http/attach_prepare_handler_test.go` | 398-399 | defensive test の guard リスト（`"sk_live_"`, `"sk_test_"` を response body に含まないことを assert） | 防御テストの否定 assertion 用文字列、実値ではない |
| `backend/internal/photobook/interface/http/edit_view_images_test.go` | 397-403 | 同上（`"sk_live_"`, `"sk_test_"`, `"draft_edit_token="`, `"manage_url_token="` を含まない assert） | 同上 |
| `frontend/lib/editPhotobook.ts` | 117 | SSR 用 `fetchEditView` で受け取った `cookieHeader` を `Cookie:` ヘッダにセットするコード | パラメータ転送ロジック、raw 値ハードコードなし |

→ **本番 Secret / token / Cookie 値の混入なし**。

## 5. deploy plan

### 5.1 順序（Backend → Workers）

1. **STOP D-1: Backend Cloud Run deploy**（Cloud Build manual trigger、`docs/runbook/backend-deploy.md` §1）
   - **deploy 実行時の current HEAD**（== `origin/main`、icon commit `37d7744` 以降の commit を含む）を build → 同 short SHA を image tag として push → `vrcpb-api` revision 更新
   - Cloud Build SUCCESS と新 revision active 100% を確認
   - 直前 revision を **rollback target** として記録（次回 STOP δ 含む)
2. **STOP D-2: Cloud Run Jobs image tag 同期**
   - `vrcpb-image-processor` および `vrcpb-outbox-worker` の image を新 tag に揃える（args / annotation / SA / Secret refs は不変）
   - `gcloud run jobs describe --format=export` で env / secretKeyRef / max-retries / parallelism / task-count / `--add-cloudsql-instances` の有無を pre/post snapshot 比較
3. **STOP D-3: Backend routing 安定化 wait**（5〜10 分、`predeploy-verification-checklist.md` §2）
4. **STOP D-4: Backend post-deploy smoke**（§6.1）
5. **STOP D-5: Workers Frontend deploy**（`npm --prefix frontend run cf:build` → `npx wrangler deploy --cwd frontend` 等価操作）
   - 直前 Workers version を **rollback target** として記録
6. **STOP D-6: Workers post-deploy smoke**（§6.2）
7. **STOP D-7: Safari 実機 smoke**（§6.3）
8. **STOP D-8: 完了報告**（`pr-closeout.md` §6 + `predeploy-verification-checklist.md` §1〜§8）

### 5.2 deploy 中の制約

- raw photobook_id / image_id / token / Cookie / Secret / storage_key / presigned URL は **報告 / 完了ログ / smoke コマンドの出力**に出さない
- 各 STOP で停止 → user 承認 → 次 STOP（自動進行禁止）
- `wsl-shell-rules.md` 遵守: `cd` 不使用、`npm --prefix` / `go -C` / `--cwd` を使う
- 失敗発生時は即 `harness/failure-log/` 起票

## 6. smoke plan

### 6.1 Backend post-deploy smoke

#### 既存 routes regression

```bash
URL=https://api.vrc-photobook.com
curl -sS  "${URL}/health"
curl -sS  "${URL}/readyz"
# chi default plain text ではなく JSON 404 を返すこと
curl -s -w "\nHTTP=%{http_code}\n" \
  "${URL}/api/public/photobooks/aaaaaaaaaaaaaaaaaa"
# 期待: HTTP=404 body={"status":"not_found"}
```

#### Phase A 5 endpoint preflight（CORS）

```bash
ORIGIN=https://app.vrc-photobook.com
DUMMY_PB=00000000-0000-0000-0000-000000000000
DUMMY_PAGE=00000000-0000-0000-0000-000000000000
DUMMY_PHOTO=00000000-0000-0000-0000-000000000000

# 各 method の preflight が Access-Control-Allow-Methods に含まれるか
for METHOD_PATH in \
  "PATCH /api/photobooks/${DUMMY_PB}/pages/${DUMMY_PAGE}/caption" \
  "POST  /api/photobooks/${DUMMY_PB}/pages/${DUMMY_PAGE}/split" \
  "PATCH /api/photobooks/${DUMMY_PB}/photos/${DUMMY_PHOTO}/move" \
  "POST  /api/photobooks/${DUMMY_PB}/pages/${DUMMY_PAGE}/merge-into/${DUMMY_PAGE}" \
  "PATCH /api/photobooks/${DUMMY_PB}/pages/reorder"; do
  M=$(echo "$METHOD_PATH" | awk '{print $1}')
  P=$(echo "$METHOD_PATH" | awk '{print $2}')
  curl -sS -i -X OPTIONS \
    -H "Origin: ${ORIGIN}" \
    -H "Access-Control-Request-Method: ${M}" \
    "${URL}${P}" \
    | grep -iE '^HTTP|^access-control'
done
# 期待: 各 200 + access-control-allow-methods に該当 method を含む
```

#### Phase A endpoint auth ガード（unauth で 401 / Cookie なしで弾く）

```bash
# Cookie なしで PATCH → 401 / 403 系（draft session middleware 通過必須）
curl -s -w "\nHTTP=%{http_code}\n" -X PATCH \
  -H "Content-Type: application/json" \
  -d '{"caption":"test","expected_version":1}' \
  "${URL}/api/photobooks/${DUMMY_PB}/pages/${DUMMY_PAGE}/caption"
# 期待: HTTP=401 / 403 系（middleware 経由）。raw token の漏出がないこと
```

> 認可成功経路（Cookie 付き）の e2e は §6.3 Safari 実機 smoke で検証する。

#### `/edit-view` regression（Phase A の getEditView は不変だが、response shape を再確認）

```bash
# Cookie なし → 401 系
curl -s -w "\nHTTP=%{http_code}\n" \
  "${URL}/api/photobooks/${DUMMY_PB}/edit-view"
```

### 6.2 Workers post-deploy smoke

#### 既存 routes regression

```bash
APP=https://app.vrc-photobook.com

# 公開ページ（200）
for P in / /create /about /terms /privacy /help/manage-url; do
  curl -sS -o /dev/null -w "${P} HTTP=%{http_code}\n" "${APP}${P}"
done

# 認可必須ページ（Cookie なしで error UI / redirect）
curl -sS -o /dev/null -w "/edit/<dummy> HTTP=%{http_code}\n" \
  "${APP}/edit/00000000-0000-0000-0000-000000000000"
curl -sS -o /dev/null -w "/prepare/<dummy> HTTP=%{http_code}\n" \
  "${APP}/prepare/00000000-0000-0000-0000-000000000000"

# OGP redirect
curl -sS -o /dev/null -w "/ogp/<dummy> HTTP=%{http_code}\n" \
  "${APP}/ogp/00000000-0000-0000-0000-000000000000"
# 期待: 302 → /og/default.png
```

#### icon / themeColor smoke（今回 commit 由来）

```bash
# icon 200
for P in /icon.png /apple-icon.png /favicon.ico /icon1.png; do
  curl -sSI "${APP}${P}" | head -3
done
# 期待: HTTP/2 200 + content-type 適切

# theme-color が <head> に含まれるか
curl -sS "${APP}/" | grep -oE '<meta name="theme-color"[^>]*>' | head -1
# 期待: content="#0F2A2E"
```

#### production bundle marker grep（Phase A wire 確認）

```bash
# /edit chunk を fetch して Phase A 関数名 / Preview marker が含まれることを確認
# (chunk path は cf:build / wrangler deploy 後に Workers 側で生成される実 path を使う)
# 期待 marker:
#   - splitPage / mergePages / movePhoto / reorderPages / updatePageCaption
#   - PreviewPane / PreviewToggle
# 旧 antipattern marker（含まれないこと）:
#   - 「公開条件に合致しません。最新を取得して再度確認してください。」固定文言
#   - raw token / Cookie / R2 credentials のリーク
```

### 6.3 Safari 実機 smoke（macOS Safari + iPhone Safari、`safari-verification.md`）

`/edit/[photobookId]` の主要動線を実 photobook（draft session で入場済）で確認:

| # | 操作 | 期待 |
|---|---|---|
| 1 | `/edit/<id>` SSR 表示 | 既存 EditView レンダリング、Cookie 維持、画像 display variant 表示 |
| 2 | page caption 編集 → 保存 | success notification、view.version+1 反映 |
| 3 | photo の「ここで分ける」（split） | 新 page が末尾に追加、photos 配分が plan §3.4.2 通り |
| 4 | photo の page picker dropdown で別 page に move | source / target pages の photos が更新、cover image は変わらない |
| 5 | 隣接 page の「上と結合」（merge） | 結合後 page に photos 統合、source page 削除 |
| 6 | page の上下移動（reorder） | display_order が連続更新 |
| 7 | 「編集 ⇄ プレビュー」トグル | preview mode で v2 ViewerLayout が render、編集 UI が hide |
| 8 | reload 後の state 復元 | server ground truth から再取得、編集中の draft state がロスしない（`state-restore-on-reload.md` 適用） |
| 9 | publish flow regression | rights_agreed / version_conflict / publish_precondition の reason 別 UI 文言が出る（`publish-precondition-ux.md` 適用） |
| 10 | tab icon | Safari タブに新 photobook icon、ホーム画面追加で apple-icon、ステータスバー color #0F2A2E |

**Safari 実機 smoke は user 実施**。完了報告には raw photobook_id / token を含めず、操作結果のみ記録する。

## 7. rollback plan

### 7.1 Backend rollback

直前 revision に traffic を 100% 戻す（`docs/runbook/backend-deploy.md` §2）:

```bash
PROJ=<gcp-project-id>
gcloud run services update-traffic vrcpb-api \
  --to-revisions=<PREV_REVISION>=100 \
  --region=asia-northeast1 --project=$PROJ
```

直前 revision 名は STOP D-1 完了時に記録する。

> **重要**: `update-traffic --to-revisions=<X>=100` は revision pin 状態になる。
> 通常運用に戻すには次の Cloud Build deploy（`cloudbuild.yaml` 末尾の `traffic-to-latest`
> step が pin 解除）または `gcloud run services update-traffic vrcpb-api --to-latest`。

#### Cloud Run Jobs rollback

`vrcpb-image-processor` / `vrcpb-outbox-worker` を直前 image tag に戻す:

```bash
gcloud run jobs update vrcpb-image-processor \
  --image=asia-northeast1-docker.pkg.dev/${PROJ}/vrcpb/vrcpb-api:<PREV_SHORT_SHA> \
  --region=asia-northeast1 --project=$PROJ
# 同じ操作を vrcpb-outbox-worker にも実施
```

### 7.2 Workers rollback

直前 version ID に戻す:

```bash
# 直近 deployment 一覧から PREV_VERSION_ID を取得
npx wrangler --cwd frontend deployments list --name vrcpb-frontend
# rollback
npx wrangler --cwd frontend rollback --name vrcpb-frontend <PREV_VERSION_ID>
```

直前 Workers version は STOP D-5 完了時に記録する。

### 7.3 DB rollback

**不要**（migration 追加なし）。万一 schema が壊れた事象は本 deploy では発生しない設計。

### 7.4 rollback 判断基準

- Backend 5〜10 分 routing 安定化後も `/health` / `/readyz` / `/api/public/photobooks/<dummy>` が想定 status を返さない
- Phase A 5 endpoint preflight が `Access-Control-Allow-Methods` に欠落
- Workers が新 chunk を配信せず `/edit` が描画破綻
- icon / themeColor が反映されない（cache propagation を 10 分待っても改善しない）
- Safari 実機 smoke で Phase A 動線のいずれかが破綻

## 8. open items

deploy 完了後に別 PR で扱う想定。本 PR の deploy 範囲には含めない。

| 項目 | 種別 | roadmap 参照 |
|---|---|---|
| 業務知識 v4 §3.1 への `rights_agreed 同 TX 取得` 追記 | docs 1 行追記 | ロードマップ §1.3 STOP α 長期方針 |
| `/edit` creator_display_name 入力欄追加（B 案） | 影響 8〜10 ファイル + test | ロードマップ §1.3 STOP α 長期方針 |
| PR36 SubmitReport 実 DB 副作用なしテスト | test 追加 | ロードマップ §1.3 PR36 後続候補 |
| `router_test.go` の `chi.Walk` route registration test | test 追加 | ロードマップ §1.3 運用/インフラ（PR40 安全性強化） |
| `session_repository_test.go` FK 違反 fix | test 修正 | ロードマップ §1.3 運用/インフラ |

## 9. 報告チェックリスト（deploy 完了時に再掲する）

- [ ] HEAD == origin/main 確認
- [ ] Backend revision / image tag / rollback target 記録
- [ ] Workers version ID / rollback target 記録
- [ ] Cloud Run Jobs image tag 同期（image-processor / outbox-worker）pre/post snapshot
- [ ] env / secretKeyRef / annotation / SA / args / parallelism / task-count / max-retries が pre/post で完全一致（image tag のみ差分）
- [ ] Backend smoke 全 PASS（既存 regression + Phase A 5 endpoint preflight + auth ガード）
- [ ] Workers smoke 全 PASS（既存 regression + icon / themeColor + Phase A bundle marker）
- [ ] Safari 実機 smoke 全 PASS（user 実施分）
- [ ] Cloud Build / Cloud Run / wrangler logs Secret grep 0 件
- [ ] raw photobook_id / image_id / token / Cookie / Secret / storage_key / presigned URL を一切出していない（dummy 値のみ）
- [ ] follow-up 項目を §8 open items / 新正典ロードマップに反映

## 10. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-10 | 初版作成。Phase A 6 commit + icon 1 commit を本番反映する deploy plan を確定（deploy 未実施） |
