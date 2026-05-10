# OGP 同期生成 + outbox fallback ハイブリッド 実装計画書

> **状態**: STOP α 計画書（**実装未着手**）。本書は ADR-0007（OGP delivery sync policy）と一体で
> 設計判断を固定する。実装着手は STOP β 別 STOP で別途承認する。
>
> **背景**: 2026-05-10 publish 直後に X で teal placeholder が表示された事象を起点に、
> 「ユーザが共有 URL を入手 → X paste → crawler 1st access」の瞬間に `status=generated` を
> 確実に返す状態へ移行する。

---

## 0. メタ情報

| 項目 | 値 |
|---|---|
| 起点 commit | `f4e7ba4 feat(manage): add safety baseline actions`（M-1a deploy 完了） |
| 関連 ADR | **ADR-0007（本書同時起票）** — OGP 同期生成 + outbox fallback ハイブリッド方式 |
| 関連 docs | `docs/design/cross-cutting/outbox.md` / `docs/design/cross-cutting/ogp-generation.md` / 業務知識 v4 §3.2 / §3.8 / §6.17 |
| 関連 plan | `docs/plan/vrc-photobook-final-roadmap.md` §1.3「Cloud Scheduler 作成」「Reconcile（OGP / R2 orphan）」 |
| 想定 STOP | 4 段（α 計画 / β Backend 実装 / γ Backend deploy + Scheduler / δ Frontend polling + crawler 実機） |
| 出力（本 STOP α） | docs/plan/m2-ogp-sync-publish-plan.md（本書）+ docs/adr/0007-ogp-sync-publish-fallback.md |

---

## 1. 受入基準（hard / soft 2 段階）

### Hard requirement
ユーザが共有 URL を X / Discord / Slack / LINE 等に paste し、当該プラットフォームの crawler が `/ogp/<photobook_id>` を 1st access した時点で `status=generated` の本物 PNG が返ること。

### Soft target
- Complete 画面で「共有」ボタンが押せるまでの待ち時間: **同期成功時 0 秒**、同期失敗 / timeout 時は **polling で最大 30 秒、それでも未 generated なら ready=false で fallback** + 「OGP は数十秒後に反映」ヒント
- publish API レスポンス追加レイテンシ: **p50 < 300 ms / p95 < 1000 ms / timeout 2.5 s**（後述 §3 数値根拠）

### 失敗時の整合性
- publish 自体は**常に成功**（業務知識 v4 §3.2「OGP 失敗でも公開自体は成功させる」を維持）
- 同期 OGP 失敗時は既存 outbox-worker 経路で `pending` → `generated` 化（Scheduler 1 min 化により最大 60 s 以内に generated 化）

---

## 2. 現状の課題（実装観測ベース）

### 2.1 publish UC は `photobook_ogp_images` 行を同 TX で INSERT していない

`backend/internal/photobook/internal/usecase/publish_from_draft.go` の `WithTx` 内で実行される 4 操作:

1. `photobookRepo.FindByID` + state check
2. `photobookRepo.PublishFromDraft` (UPDATE)
3. `revoker.RevokeAllDrafts` (DELETE)
4. `outboxRepo.Create(photobook.published, status=pending)`

→ **`photobook_ogp_images` 行の INSERT は無い**。pending 行は outbox-worker 側 `GenerateOgpForPhotobook.CreatePending`（`backend/internal/ogp/internal/usecase/generate_ogp.go`）で初めて作成される。

### 2.2 1st access で 「row 不在 → not_found → default redirect」が確定

`backend/internal/ogp/internal/usecase/get_public_ogp.go` および Workers proxy `frontend/app/ogp/[photobookId]/route.ts` は `photobook_ogp_images.status != 'generated'`（行不在も含む）のとき `/og/default.png` に 302 redirect する。

→ publish 直後 outbox-worker が起動するまでの間（**現状 Cloud Scheduler 未設定で運営手動 execute**）に X に貼ると、X crawler は default placeholder を取得し **数日〜1 週間 cache** する。X Card Validator で手動 refresh は user 操作が必要で現実的でない。

### 2.3 OGP renderer は pure Go で軽量（実測値）

`backend/internal/ogp/infrastructure/renderer/renderer.go`:
- `image/draw` + `opentype` + `imaging` のみ、cgo / libvips 不使用
- フォント (`NotoSansJP-Regular/Bold.otf`) は `go:embed` で起動時 load
- cover image 取込みは現状 fallback teal flat のみ（`generate_ogp.go:166` "PR33b では取得しない（fallback 描画）"、本実装は別 STOP）

### 2.4 outbox-worker Cloud Run Job は手動 execute 運用

`CLAUDE.md` / `final-roadmap §1.3`「Cloud Scheduler 作成（outbox-worker 自動回し）→ 当面は手動 Job execute、PR33e で要否判断」。M-1a 完了時点で本 Scheduler は未作成。

---

## 3. 数値根拠（STOP α 計測結果）

### 3.1 renderer 実測（Go benchmark、Intel i7-14700F / Linux）

| Benchmark | ns/op | B/op | allocs/op | wall-time |
|---|---:|---:|---:|---:|
| `BenchmarkRender_WarmAsciiTitle` | 10,669,325 | 3,924,537 | 49 | **10.7 ms** |
| `BenchmarkRender_WarmJapaneseTitle` | 12,816,525 | 3,945,401 | 55 | **12.8 ms** |
| `BenchmarkRendererNew_Cold` | 278,773 | 725,371 | 128 | **0.28 ms** |

> 実測方法: `backend/internal/ogp/infrastructure/renderer/` に一時 `renderer_bench_test.go` を配置し
> `go -C backend test ./internal/ogp/infrastructure/renderer/... -run='^$' -bench=. -benchtime=2s -benchmem`
> を実行（測定後 file 削除、commit せず）。

### 3.2 Cloud Run 換算

Cloud Run service `vrcpb-api` は **1 vCPU / 512 MiB / startup-cpu-boost ON**。Intel i7-14700F の 28 threads と比較すると 1 vCPU は **約 2〜5 倍遅い**と保守的に見積もる:

| 操作 | 実機 | Cloud Run 換算 (warm) |
|---|---:|---:|
| renderer.Render | 11-13 ms | **25-65 ms** |
| renderer.New (cold) | 0.3 ms | 1-2 ms |

### 3.3 R2 PUT estimate

実測 log は欠落（structured log に `r2_put_duration_ms` フィールドなし、`docs/design/aggregates/image/` でも未計測）。**未測定、STOP β で計測する**。

参考: Cloudflare R2 同リージョン（asia-northeast1 → R2 ap）の小オブジェクト（~50 KB PNG）PUT は p50 **30-80 ms** / p95 **100-200 ms** が一般的目安。HEIC 等の大 object は対象外、OGP は固定 1200×630 PNG で約 50-100 KB。

### 3.4 同期 OGP 生成 total latency estimate

publish API レスポンス前に同期 OGP を入れた場合の追加 latency:

| Step | 内訳 | warm estimate | p95 想定 |
|---|---|---:|---:|
| 1. photobook_ogp_images pending INSERT (同 TX) | 同 TX 内 1 SQL | +5-15 ms | +30 ms |
| 2. WithTx commit | 既存通り | 0 | 0 |
| 3. renderer.Render | 上記 25-65 ms | 30-50 ms | 100 ms |
| 4. R2 PUT | 上記 estimate | 30-80 ms | 200 ms |
| 5. 完了 TX (images + image_variants INSERT + MarkGenerated) | 単発 TX | 20-50 ms | 100 ms |
| **合計** | | **85-195 ms** | **~430 ms** |

→ **timeout 2.5 s は十分余裕**。Plan agent の保守 estimate (200-540 ms warm / 500-1100 ms cold) より明らかに高速。

### 3.5 outbox-worker Cloud Run Job cold start

`gcloud run jobs executions list --job=vrcpb-image-processor` 観測値: execution wall-time **15-19 s**。これは Job cold start + multi-variant 処理 + 複数 R2 GET/PUT を含む全体時間で、OGP 単発処理ではない。

Job cold start 単独は推定 **4-8 s**（Cloud Run Job は service と異なり pre-warm されない）。outbox-worker を本案で同期 fallback として使う場合、Scheduler 1 min 化により**最大 60 s 内に新 instance** で picked → generated 化される。

---

## 4. 採用方針: H1 ハイブリッド（同期試行 + outbox fallback + Cloud Scheduler 1 min）

### 4.1 構成

| 構成要素 | 内容 |
|---|---|
| **publish 同 TX** | `photobook_ogp_images.pending` 行を**先行 INSERT**（OGP UC 側 `EnsureCreatedPending` 分離、worker 側冪等保持）。1st crawl で row 不在による fallback 経路を消す |
| **commit 後 best-effort 同期** | publish UC の `WithTx` commit 後・response 返却前に renderer + R2 PUT + `MarkGenerated` を **context timeout 2.5 s で best-effort 実行**。成功時 `status=generated`。**失敗・timeout は publish 200 を維持**、既存 outbox path に倒す（既に同 TX で `outbox_events` INSERT 済） |
| **Cloud Scheduler 1 min** | `vrcpb-outbox-worker-tick` を `* * * * *` ENABLED（`image-processor-tick` と同パターン）。手動 execute 運用を恒久解除。同期失敗時の最大遅延を 60 s 以内に SLO 化 |
| **Frontend Complete 画面 polling** | `/api/public/photobooks/{id}/ogp` を 2 s 間隔で polling（最大 30 s）、`generated` まで「共有」ボタン disable + spinner |

### 4.2 publish UC への変更（イメージ）

```go
// 既存 WithTx 内に追加（1 行 INSERT）
if err := ogpRepo.EnsureCreatedPending(ctx, tx, pb.ID(), now); err != nil {
    return fmt.Errorf("ogp ensure pending: %w", err)
}
// outbox INSERT, RevokeAllDrafts ... (既存)

// WithTx 終了後、response 返却前
syncCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
defer cancel()
if err := h.ogpSync.TryGenerate(syncCtx, pb.ID()); err != nil {
    // best-effort 失敗は warn log のみ。publish 200 は維持。
    h.log.Warn("ogp_sync_failed", slog.String("photobook_id", pb.ID().String()), slog.String("error", err.Error()))
}
// response 200 を返す（generated でも pending でも 200）
```

### 4.3 outbox handler 側の冪等性

既存 `backend/internal/outbox/internal/usecase/handlers/photobook_published.go` の handler は `GenerateOgpForPhotobook.Execute` を呼ぶ。本案で **publish 時に pending 行が既存** + **同期成功時に generated**になっていることを冪等に扱う:

- 同期成功 → handler は worker 起動時に `status=generated` 確認 → no-op 完了
- 同期失敗 → handler は `status=pending` 確認 → 通常通り render + R2 PUT → `status=generated`
- generate_ogp.go の `CreatePending` を `EnsureCreatedPending` に名前変更（既存・新規いずれの呼出も冪等化）

### 4.4 unpublish / soft_delete との独立性（M-1b 影響境界）

`get_public_ogp.go` の `publicAllowed` 判定は既存 `hiddenByOperator` / `status='published'` / `visibility != 'private'` で行う。M-1b で `hidden_by_owner` / `status='deleted'` が増えるときは同 path に bool 追加するだけで本案と独立。本書 §6 STOP δ smoke で「M-1b 着地時に再確認」と明記。

---

## 5. STOP 分割（4 段、user 確定）

### STOP α: 計画 + 計測 + ADR（**本 STOP、実装なし**）

| 項目 | 状態 |
|---|---|
| renderer bench 実測（10-13 ms warm） | ✓ 完了 |
| R2 PUT 実測 | 未測定（estimate 30-80 ms、STOP β で計測） |
| publish UC 実装確認（pending INSERT 不在の事実） | ✓ 完了 |
| 本書 `docs/plan/m2-ogp-sync-publish-plan.md` | ✓ 本 STOP で作成 |
| ADR-0007 ドラフト `docs/adr/0007-ogp-sync-publish-fallback.md` | ✓ 本 STOP で作成 |
| 既存 docs / runbook 追記方針（実装は別 STOP） | ✓ §7 で列挙 |
| commit 前停止、user 承認待ち | ✓ |

### STOP β: Backend 実装 + verification

| 項目 | DoD |
|---|---|
| `publish_from_draft.go` の WithTx に `EnsureCreatedPending` 追加（pending 行先行 INSERT） | unit / integration test PASS |
| OGP UC 側に `EnsureCreatedPending` method 分離（冪等化） | 既存 `CreatePending` 呼出を全部 ensure 化、worker 側で再呼出しても duplicate にならない |
| publish UC 内に commit 後 best-effort 同期 path 追加（context timeout 2.5 s、失敗は warn log）| 同期成功 / timeout / R2 失敗 / 完了 TX 失敗の 4 シナリオで publish 200 が返ることを test で固定 |
| R2 PUT 実測（既存 logs に structured field 追加 or test 内 metric 取得） | p50 / p95 を `docs/plan/m2-ogp-sync-publish-plan.md` §3.3 に追記 |
| Backend smoke plan（後 STOP γ で実施） | smoke コマンドを本書 §6 に固定 |
| commit 前停止 | user 承認後 STOP γ |

**変更ファイル想定**:
- `backend/internal/photobook/internal/usecase/publish_from_draft.go`（WithTx 拡張、commit 後同期 path）
- `backend/internal/photobook/internal/usecase/ports.go`（新 port: `OgpEnsurePending` / `OgpSyncGenerator`）
- `backend/internal/ogp/internal/usecase/generate_ogp.go`（`CreatePending` → `EnsureCreatedPending` 冪等化 + 新 method `SyncGenerate`）
- `backend/internal/ogp/wireup/*` および photobook wireup（DI 追加）
- 必要なら adapter（`backend/internal/photobook/infrastructure/ogp_adapter/`）新設
- tests: publish UC + ogp UC + adapter
- migration: **不要**（既存 schema で完結）

### STOP γ: Backend deploy + Cloud Scheduler 作成

| 項目 | DoD |
|---|---|
| Backend Cloud Build manual submit | revision 100% 切替、`predeploy-verification-checklist.md` 準拠 |
| Cloud Run Jobs (`vrcpb-image-processor` / `vrcpb-outbox-worker`) image tag 同期 | 両 Job `:<new SHA>` |
| **`vrcpb-outbox-worker-tick` Cloud Scheduler 新規作成** | `* * * * *` ENABLED、OIDC 認証、image-processor-tick と同パターン |
| Backend smoke（既存 routes + publish→OGP lookup の同期成功確認） | §6 smoke plan 全 PASS |
| Scheduler 起動ログ観測（最初の 5 分で 5 回 Job 起動を確認） | `gcloud run jobs executions list --job=vrcpb-outbox-worker` で観測 |
| `docs/runbook/outbox-worker-ops.md` 新規作成 | image-processor-job-automation-plan.md の Scheduler 章を流用 |
| 完了報告 + commit / push（runbook） | rollback target 控え（Scheduler は削除コマンドで巻戻し可） |

### STOP δ: Frontend polling + crawler 実機確認

| 項目 | DoD |
|---|---|
| Frontend Complete 画面で OGP polling 実装（2 s 間隔 / 最大 30 s） | EditClient or CompleteView の publish 後 state 拡張、共有ボタン disable / spinner |
| 共有ボタンの ready / not-ready 表示 | generated 確認後 enable、30 s 経過時は info で「OGP は数十秒後に反映」+ 共有ボタン enable |
| Workers redeploy | `cf:build` + `wrangler deploy`、Total Upload ≤ 9 MiB target 維持 |
| Workers smoke | 既存 routes + Manage / Edit regression + Complete 画面 polling 動作 |
| **X Card Validator / Discord / Slack / LINE preview 実機確認** | テスト用 photobook（hide → unhide 経由）で 4 種 crawler が `generated` PNG を取得することを user 確認、raw slug は work-log で redact |
| iOS Safari / macOS Safari 実機確認 | `safari-verification.md` 該当（OGP メタ / 共有ボタン timing / モバイル UI）|
| 完了報告 | work-log 起票（`harness/work-logs/2026-05-XX_ogp-sync-publish-result.md`）|

---

## 6. smoke plan（STOP γ / δ で実行、ここで固定）

### Backend smoke（STOP γ）

```bash
URL=https://api.vrc-photobook.com
ORIGIN=https://app.vrc-photobook.com
DUMMY_PB=11111111-2222-3333-4444-555555555555

# 1. 既存 routes regression
curl -sS "${URL}/health"           # 期待: 200 ok
curl -sS "${URL}/readyz"           # 期待: 200 ready
curl -s -w "\nHTTP=%{http_code}\n" "${URL}/api/public/photobooks/aaaaaaaaaaaaaaaaaa"
# 期待: 404 + {"status":"not_found"}

# 2. M-1a manage endpoint preflight 維持
# (前 STOP の項目を維持)

# 3. publish 同期 OGP smoke
#    実 publish は user 操作（test photobook を draft 作成 → publish）、
#    publish 直後に /api/public/photobooks/{id}/ogp を curl で確認
curl -sS "${URL}/api/public/photobooks/${PUBLISHED_PB_DUMMY}/ogp"
# 期待: 200 + status=generated（同期成功 path）
#       または 200 + status=pending（同期失敗 path、Scheduler 1 min 内に generated 化）

# 4. Cloud Scheduler 確認
gcloud scheduler jobs describe vrcpb-outbox-worker-tick \
  --location=asia-northeast1 --project=$PROJ --format="value(name,schedule,state)"
# 期待: schedule '* * * * *' / state ENABLED

# 5. Job execution 観測（5 分待った後）
gcloud run jobs executions list --job=vrcpb-outbox-worker \
  --region=asia-northeast1 --project=$PROJ --limit=5
# 期待: 直近 5 件、1 min 間隔で起動
```

### Workers smoke（STOP δ）

```bash
APP=https://app.vrc-photobook.com

# 1. 公開ページ regression（5 page 200）
for P in / /about /terms /privacy /help/manage-url; do
  curl -sS -o /dev/null -w "${P} HTTP=%{http_code}\n" "${APP}${P}"
done

# 2. M-1a Manage bundle marker 維持
# (前 STOP の項目を維持)

# 3. OGP polling bundle marker
# /edit / Complete 画面 chunk に polling 関連の testid / 文言が含まれること

# 4. icon / theme-color regression（M-1a 後不変）

# 5. raw token / Secret grep 全 0 件
```

### Crawler 実機 smoke（STOP δ、user 実施）

| crawler | URL | 期待 |
|---|---|---|
| X Card Validator | `https://cards-dev.twitter.com/validator` （内部 preview 可なら） | 1st access で `generated` PNG が表示、teal default が出ない |
| Discord | テスト用 photobook の公開 URL を Discord に貼る | embed preview に `generated` PNG |
| Slack | 同上 | unfurl preview に `generated` PNG |
| LINE | 同上 | LINE preview に `generated` PNG |

raw slug は work-log で redact（実 photobook_id / slug は dummy 表記）。

---

## 7. 既存 docs / runbook 追記方針（STOP β / γ / δ で実施、ここで列挙のみ）

| ファイル | 追記内容 | 担当 STOP |
|---|---|---|
| `docs/adr/0007-ogp-sync-publish-fallback.md` | 本書同時起票（**本 STOP α**） | α |
| `docs/design/cross-cutting/outbox.md` §6.2 | 「`PhotobookPublished` は publish handler 内で**同期試行 + 非同期 fallback**」追記、ADR-0007 link | β |
| `docs/design/cross-cutting/ogp-generation.md` §5.1 | 「publish 同 TX で `photobook_ogp_images.pending` を INSERT」追加（`EnsureCreatedPending` 責務） | β |
| `.agents/rules/predeploy-verification-checklist.md` §3 / §5 / §8 | publish→OGP lookup smoke / Scheduler bindings / Job exec log 確認 | γ |
| `.agents/rules/safari-verification.md` OGP / Twitter card 表 | publish-time OGP 同期化を変更対象として追記 | δ |
| `docs/runbook/backend-deploy.md` §1.4.2 | publish API smoke（publish 直後 OGP lookup） | γ |
| **新規** `docs/runbook/outbox-worker-ops.md` | Scheduler / Job 起動確認 / 障害対応 | γ |
| `CLAUDE.md` 未実装欄 | 「Cloud Scheduler 作成（outbox-worker 自動回し）→ PR33e で要否判断」を削除（恒久 ENABLED 化）| γ |
| `docs/plan/vrc-photobook-final-roadmap.md` §1.3 | OGP 同期化を完了マーク、Cloud Scheduler 完了マーク、R2 orphan reconciler 優先度更新 | δ |

---

## 8. 先送り項目

| 項目 | 記録先 | 理由 |
|---|---|---|
| R2 orphan reconciler（render 後 R2 PUT 後 DB 失敗の orphan 回収、7 日 cleanup） | `final-roadmap` §1.3「Reconcile（OGP / R2 orphan）」既存項目維持、本案で発生頻度増の可能性を注記 | 本案の hard requirement に直接効かない、優先度上げて後続 STOP |
| OGP renderer の cover image 取込み本実装（`Input.CoverPNG` 経路） | `final-roadmap §1.3` に「OGP renderer cover 本実装は同期化後に再評価」記録 | render +400-800 ms 増の可能性、まず renderer 構造のまま同期化を成立させる |
| 同期成功率 KPI 未達時の対応（Cloud Run CPU 増 / timeout 短縮 / 等） | ADR-0007 §運用 KPI として宣言、3 か月後再評価を `final-roadmap` カレンダーに記録 | KPI 観測には実 production traffic が必要 |
| M-1b unpublish / soft_delete 後の OGP 扱い | `m-1-manage-mvp-safety-plan.md` の M-1b plan 着手時に確認項目化 | 本案と独立（`get_public_ogp.go` の `publicAllowed` に bool 追加するだけ）|

---

## 9. Open Questions（STOP β 着手前に確定）

1. **同期 timeout の最終値**: 2.5 s で進めるか / 1.5 s / 3.0 s
   - 推奨: 2.5 s（renderer 65 ms + R2 PUT 200 ms p95 + 完了 TX 100 ms = 365 ms 想定に対し、network jitter / Cloud Run autoscale 等の余裕を含めて 7 倍弱の予算）
2. **同期 path の slog 観測項目**: `ogp_sync_failed` / `ogp_sync_success_ms` / `ogp_sync_timeout` 等のフィールド名・粒度
   - 推奨: `event=ogp_sync_result` + `outcome=success|timeout|error` + `duration_ms` + `photobook_id`（dummy 化なし、operational identifier）
3. **Frontend polling: 失敗時 fallback の UX 文言**: 「OGP は数十秒後に反映されます」「公開後しばらくお待ちください」等の具体文言
   - 推奨: 「公開直後の共有では OGP（プレビュー画像）の反映に数十秒かかる場合があります。X や Discord 等で確認できない場合は数分後に再度お試しください。」
4. **`EnsureCreatedPending` の冪等戦略**: ON CONFLICT DO NOTHING / 既存 row check 後 INSERT / row count 確認
   - 推奨: ON CONFLICT DO NOTHING（photobook_id UNIQUE、status='pending' は冪等）
5. **同期失敗時の photobook_ogp_images.failure_reason 記録要否**: 同期 path だけ違う reason を残すか
   - 推奨: 同期失敗は **記録しない**（pending のまま、worker が後で generated にする想定）。記録すると worker 側で「failure 履歴あり」と誤読される

---

## 10. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-11 | 初版作成（STOP α）。renderer bench 実測 + publish UC 実装確認 + 4-STOP 分割固定 + Open Questions 5 件起票 |
