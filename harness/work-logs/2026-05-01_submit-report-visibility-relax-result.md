# SubmitReport visibility 緩和 実装結果（2026-05-01、完了）

> **状態**: STOP α / β / γ / ε / final closeout 全完了。
> 設計判断: [`docs/plan/post-pr36-submit-report-visibility-decision.md`](../../docs/plan/post-pr36-submit-report-visibility-decision.md) 案 B 採用
> 起点 commit: `da9e637 docs(plan): decide submit report visibility policy`
> 実装 commit: `773d5cc fix(report): allow report submit on unlisted photobooks`
> closeout commit: 本書反映 commit

## 0. 本書のスコープ

PR36 STOP ε で発見した SubmitReport visibility 不整合（公開 Viewer / Viewer footer 通報リンク / Report ページ / SubmitReport の 4 層で受入軸がズレ、unlisted で「見えるが送れない」）を解消するための独立 PR の進行記録。STOP α〜final closeout を通して記録する。

## 1. STOP α 確定事項（2026-05-01 ユーザー承認済）

| 項目 | 確定値 |
|---|---|
| 採用案 | 案 B（Backend を `visibility != private` に緩和）|
| `unlisted` の扱い | Report submit **対象** |
| `private` / `hidden=true` / `status != published` | 引き続き **不可** |
| Frontend / Public API / Workers redeploy | **不要**（API 互換維持）|
| migration / Secret | **不要** |
| 業務知識 v4 §3.6 への 1 行追記 | やる |
| PR35 計画書 §17 #2 | `visibility != private` 採用で確定 |
| commit 構成 | 単一 commit（推奨案 a）|
| 実 DB 統合テスト | 今 PR では追加せず Safari STOP ε で end-to-end 担保。後続候補は roadmap §1.3 に維持 |

## 2. STOP β 実装内容

### 2.1 Backend usecase

`backend/internal/report/internal/usecase/submit_report.go`:

- import 追加: `photobookdomain "vrcpb/backend/internal/photobook/domain"` / `"vrcpb/backend/internal/photobook/domain/vo/visibility"`
- 既存の `Execute` 内 lines 159-168 の 3 連 if（status / visibility / hidden）を **`assessReportEligibility(pb)` 関数として抽出**
- 抽出関数の判定:
  - `!pb.Status().IsPublished()` → `ErrTargetNotEligibleForReport`
  - `pb.Visibility().Equal(visibility.Private())` → `ErrTargetNotEligibleForReport`（**緩和済**：以前は `!= "public"` で unlisted も拒否していた）
  - `pb.HiddenByOperator()` → `ErrTargetNotEligibleForReport`
  - 上記すべて満たさなければ nil
- 判定理由は外部に区別なく漏らさない（`ErrPublicNotFound` / `ErrPublicGone` 既存ポリシーと整合）

### 2.2 単体テスト

`backend/internal/report/internal/usecase/submit_report_test.go`:

- `TestAssessReportEligibility` を追加（テーブル駆動、`.agents/rules/testing.md` 準拠）
- ケース 7 件:

| name | visibility | status | hidden | 期待 |
|---|---|---|---|---|
| 成功_public_published_visible | public | published | false | nil |
| 成功_unlisted_published_visible | unlisted | published | false | **nil（案 B で新規許可）** |
| 拒否_private_published | private | published | false | ErrTargetNotEligibleForReport |
| 拒否_public_hidden_by_operator | public | published | true | err |
| 拒否_unlisted_hidden_by_operator | unlisted | published | true | err |
| 拒否_draft | unlisted | draft | false | err |
| 拒否_deleted | public | deleted | false | err |

- 各ケースは `domain.RestorePhotobook` で必要最小付帯フィールド（slug / manage_url_token_hash / published_at / draft_edit_token_hash / draft_expires_at / deleted_at）を埋める helper `buildPhotobookForEligibility` を新設
- 既存 `TestSubmitReport_L4_BlankTurnstileToken_Rejected`（4 ケース）/ `TestMapUsageErr`（4 ケース）に regression なし

### 2.3 docs 更新

| ファイル | 変更 |
|---|---|
| `docs/spec/vrc_photobook_business_knowledge_v4.md` | §3.6「この機能が守ること」に 1 行追記（「通報受付対象の可視性条件」: published + hidden=false + visibility != private、`public` / `unlisted` は通報可能、`private` / `hidden` / `draft` / `deleted` / `purged` は対象外）|
| `docs/plan/m2-report-plan.md` §17 #2 | 「採用（PR36 後）: published + (visibility != private) + not hidden を受付」「当初実装は最小（public のみ）を採用、PR36 STOP ε で 3 層不整合判明後に案 B 確定」と追記 |
| `docs/plan/m2-report-plan.md` §5.2 / line 224 | 公開対象判定条件を `visibility != private` に修正 |
| `docs/runbook/usage-limit.md` §11.3 | 「smoke target は `visibility != private` 必須」に更新、`assessReportEligibility` 参照、設計判断メモへの link 追加 |
| `docs/plan/vrc-photobook-final-roadmap.md` §1.3 | 「SubmitReport の visibility 要件 再判断」を完了扱いに更新（取り消し線 + 案 B 採用への link）|
| `docs/plan/post-pr36-submit-report-visibility-decision.md` §13 履歴 | STOP β 実装完了行を追加 |

### 2.4 build / vet / test 結果

- `go -C backend build ./...` : **OK**
- `go -C backend vet ./...` : **OK**
- `go -C backend test -count=1 ./internal/report/internal/usecase/...` : **OK**（既存 8 ケース + 新規 7 ケース PASS）

## 3. 影響範囲（実装後の確認）

| 領域 | 状態 |
|---|---|
| Backend SubmitReport usecase | 1 行差し替え + extract function + 7 行 import 追加 |
| Backend submit_report_test.go | テーブル駆動 7 ケース + helper 1 個追加 |
| Backend 公開 API レスポンス（`publicPhotobookPayload`）| 変更なし（API 互換維持）|
| Frontend `PublicPhotobook` 型 / `ViewerLayout` / `/p/[slug]/report` / `ReportForm` / `lib/report.ts` / `lib/publicPhotobook.ts` | 変更なし |
| migration / Secret / Cloud Run env / secretKeyRef / Job args / cloudsql-instances | 変更なし |
| 業務知識 v4 §3.6 / m2-report-plan §17 #2 / §5.2 | 1 行追記 / 採用方針確定 / 公開対象判定 wording |
| runbook `usage-limit.md` §11.3 | smoke target 条件を `!= private` に更新 |
| roadmap §1.3 | 後続候補を完了扱いに |

## 4. STOP γ Backend deploy（完了）

| 項目 | 値 |
|---|---|
| Cloud Build ID | `c77ad798-6ee1-4ebb-b338-8516444254c8` SUCCESS（3M40S）|
| 生成 image | `vrcpb-api:773d5cc` (digest `sha256:e93a02ca...`) |
| Cloud Run service `vrcpb-api` | revision `vrcpb-api-00023-pwv` 100% traffic、image `:773d5cc` |
| 直前 active（rollback 候補）| `vrcpb-api-00022-g4r` / `vrcpb-api:044899c`（PR36 final closeout 時点）|
| secretKeyRef | 8 個（`DATABASE_URL` / `R2_*` ×5 / `REPORT_IP_HASH_SALT_V1` / `TURNSTILE_SECRET_KEY`、rev 22 と完全一致）|
| plain env | 2 個（`ALLOWED_ORIGINS` / `APP_ENV`、不変）|
| env 消失 | なし |
| 7 分待機 | 18:25:46Z → 18:32:46Z |
| `/health` | HTTP 200 |
| `/readyz` | HTTP 200 |
| public route handler bad-slug | HTTP 404 + `{"status":"not_found"}`（chi default plain text 落ちなし）|
| Cloud Build log Secret grep | 0 件 |
| Cloud Run log Secret grep | 起動時 `report endpoint enabled (turnstile + ip_hash_salt configured)` / `turnstile configured; upload-verifications endpoint enabled` のみ（**設定確認の文字列、値ではない**、PR36 STOP γ と同パターン）|
| Cloud Run log raw UUID 形式 | 0 件 |
| Cloud Run Job `vrcpb-outbox-worker` image | `vrcpb-api:773d5cc`（args / secretKeyRef 6 個 不変）|
| Job 実行 | 未実行（execute コマンド未投入）|

## 5. STOP δ Workers redeploy（不要、未実施）

API 互換維持・bundle 変更なしのため Workers redeploy は不要と判断。実施なし。Worker version は PR36 STOP δ 由来 `ac2b884a-7c75-49d3-a21c-5c2a66c462ed` のまま 100% active。

## 6. STOP ε Safari 実機 smoke（完了）

iPhone Safari で **unlisted smoke candidate** に対し submit 1 回 → **thanks view 成立**。

| 確認項目 | 結果 |
|---|---|
| ReportForm 表示 | OK |
| Turnstile 完了 | OK |
| submit 1 回（連打なし）| OK |
| thanks view 表示 | OK（visibility 緩和前は `not_found` 阻害だった経路が成立）|
| `report_id` / `token` / `scope_hash` の画面・URL 露出 | なし |
| レイアウト崩れ | なし |
| Turnstile 失敗 / `not_found` / `rate_limited` / `internal_error` 文言 | なし |

### 6.1 副作用 delta（baseline_now_utc を cutoff として一意特定）

| 観点 | baseline | post-Safari | post-cleanup |
|---|---|---|---|
| `reports` total | 1 | 2（+1）| **1**（baseline 一致）|
| `reports` status=submitted (last 1h) | 0 | 1（+1）| 0 |
| `reports` status=resolved_action_taken（PR35b 由来）| 1 | 1（不変）| 1（保持）|
| `outbox.report.submitted` pending | 0 | 1（+1）| **0**（baseline 一致）|
| `usage_counters` total | 0 | 2（+2、5min narrow + 1h broad）| **0**（baseline 一致）|

smoke 由来行の特徴（redact 済）:
- report row: status=submitted, reason=harassment_or_doxxing, detail_len=4 (smoke marker), contact_len=16, ip_hash 32 byte hex
- outbox payload: 6 keys (`event_version` / `has_contact` / `occurred_at` / `reason` / `report_id` / `target_photobook_id`) のみ。**`reporter_contact` / `detail` / `source_ip_hash` は payload 不在**（PII clean、設計通り）
- usage_counters: 5min 狭粒度 (count=1, limit=3) + 1h 広粒度 (count=1, limit=20)、両者 created_at 同一（同 TX）

### 6.2 cleanup（delta-based、`FOR UPDATE` lock + rowcount assert）

- 対象は `submitted_at` / `created_at > baseline_now_utc` で一意特定（raw target id_prefix / LIKE 条件は使用せず）
- DELETE `outbox_events`: 1 行（rowcount assert 一致）
- DELETE `reports`: 1 行（rowcount assert 一致）
- DELETE `usage_counters`: 2 行（複合 key で個別 DELETE、合計 rowcount assert 一致）
- TX COMMIT 成功
- `report.submitted` outbox は **worker 処理せず削除**（指示通り、本物の通報として残さない）
- PR35b 由来 `resolved_action_taken` 行は不変（保持確認）

### 6.3 target photobook 最終状態

| 項目 | 開始時 | 終了時 |
|---|---|---|
| status | published | published |
| visibility | unlisted | **unlisted**（不変）|
| hidden_by_operator | false | false（不変）|
| version | 1 | 1（不変）|

`cmd/ops photobook hide / unhide` 不使用、moderation_actions 行も追加せず。

### 6.4 final invariants

```
reports by status: resolved_action_taken: 1
outbox by type/status:
  photobook.hidden          processed  6
  photobook.published       processed  1
  photobook.unhidden        processed  6
  report.submitted          processed  1
outbox pending total: 0
usage_counters total: 0
```

### 6.5 workspace cleanup

- cloud-sql-proxy 停止、port 5433 クリア
- `/tmp/dsn-prod.txt` / `/tmp/target-pid.txt` / `/tmp/target-slug.txt` / `/tmp/proxy.log` / `/tmp/stop-eps-relax-baseline.json` / `/tmp/build-stop-gamma-relax.log` 全削除
- DSN / raw target id / raw slug は端末履歴・work-log・commit に未残存

## 7. final closeout（完了）

PR closeout チェックリスト（`.agents/rules/pr-closeout.md` §6 準拠、本 commit 完了時点）:

- [x] 実装 commit 単一化 + push 確認（`773d5cc`、origin/main 同期）
- [x] STOP α / β / γ / ε 結果記録（本 work-log §1〜§6）
- [x] Safari smoke 結果（unlisted submit 成功）
- [x] DB 副作用と cleanup 結果（§6.1〜§6.2）
- [x] target photobook 状態が変化していないこと（§6.3）
- [x] Workers / Cloud Run Job / migration / Secret に変更なし（Job image bump のみ、env 不変）
- [x] roadmap / 業務知識 / m2-report-plan / runbook / 決定メモ §13 履歴 更新
- [x] stale-comments 結果 4 区分分類（A: 既修正 / B: 後続候補（既存）/ C: 過去経緯 / D: 該当なし）
- [x] redact 対象値 grep 0 件（commit / docs / work-log / failure-log にて未含有）
- [x] failure-log 起票要否判断 = **不要**（仕様意図的緩和、設計判断メモで網羅）

## 8. Secret / Privacy 取り扱い（PR 全期間）

- redact 対象値（raw slug / raw photobook_id / raw report_id / raw URL / token / Cookie / DATABASE_URL / Secret 値 / source_ip_hash 完全値 / scope_hash 完全値 / reporter_contact 実値 / detail 実値）: chat / commit / docs / work-log / failure-log には **未含有**（grep 0 件）
- smoke target は redact ラベル（"unlisted smoke candidate" / "public hidden target"）で参照
- cmd/ops 出力 / Cloud Run logs / Cloud Build logs にも値漏えいなし
- Safari 用 URL は chat 一回限り提示の規律で扱った

## 9. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-01 | 初版（STOP β 完了時点）。STOP γ / ε / final closeout は未実施として記録 |
| 2026-05-01 | STOP γ / ε / final closeout 完了を反映。本書を完了モードに更新 |
