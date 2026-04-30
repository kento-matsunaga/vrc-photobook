# SubmitReport visibility 緩和 実装結果（2026-05-01、進行中）

> **状態**: STOP β 実装完了、STOP γ Backend deploy 承認待ち。
> 設計判断: [`docs/plan/post-pr36-submit-report-visibility-decision.md`](../../docs/plan/post-pr36-submit-report-visibility-decision.md) 案 B 採用
> 起点 commit: `da9e637 docs(plan): decide submit report visibility policy`

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

## 4. STOP γ Backend deploy（承認待ち）

`docs/runbook/backend-deploy.md` §1.4 に従い実施予定:

- Cloud Build manual submit で `vrcpb-api:<short-sha>` build
- Cloud Run service `vrcpb-api` の image を新 SHA に更新（自動 traffic 100%）
- Cloud Run Job `vrcpb-outbox-worker` の image も同 SHA に bump
- env / secretKeyRef / args / cloudsql-instances は **触らない**
- 5〜10 分待機 → public route handler smoke（`/health` 200 / `/readyz` 200 / bad-slug HTTP 404 + JSON）
- Cloud Build / Cloud Run logs Secret grep（0 件期待）
- rollback 候補: `vrcpb-api-00022-g4r` / image `vrcpb-api:044899c`（PR36 final closeout 時点）

## 5. STOP δ Workers redeploy

**不要**（API 互換維持、Frontend / bundle 変更なし）。

## 6. STOP ε Safari 実機 smoke（承認待ち）

iPhone Safari で **unlisted smoke candidate**（visibility=unlisted, hidden=false, smoke 用既存 photobook）に対し:

- submit 1 回のみ → thanks view 表示確認
- report_id / token / scope_hash / raw URL の画面・URL 露出なし、レイアウト崩れなし
- Turnstile 失敗文言・rate_limited 文言と区別されている

副作用 cleanup（PR36 STOP ε と同流儀、`FOR UPDATE` lock + rowcount assert + ROLLBACK on mismatch）:
- baseline: submit 前に reports / outbox / usage_counters の件数を取得
- delta 検出: submit 後の件数差分から smoke 由来行を一意特定（target id_prefix / LIKE 条件は使わない）
- DELETE `outbox.report.submitted` pending（worker 処理せず削除）
- DELETE `reports`（smoke delta のみ）
- DELETE `usage_counters`（smoke delta、report.submit のみ）

target photobook は visibility / hidden を変更しない方針（unlisted のまま）。**public hidden target は触らない**（regression 回避）。

## 7. final closeout（後続）

PR closeout チェックリスト（`.agents/rules/pr-closeout.md` §6 準拠）:

- [ ] 実装 commit 単一化 + push 確認
- [ ] STOP α / β / γ / ε 結果記録
- [ ] Safari smoke 結果（unlisted submit 成功）
- [ ] DB 副作用と cleanup 結果
- [ ] target photobook 状態が変化していないこと
- [ ] Workers / Cloud Run Job / migration / Secret に変更なし
- [ ] roadmap / 業務知識 / m2-report-plan / runbook / 決定メモ §13 履歴 更新
- [ ] stale-comments 結果 4 区分分類
- [ ] Secret grep（0 件）+ raw id / hash / URL の redact
- [ ] failure-log 起票要否判断

## 8. Secret / Privacy 取り扱い

- raw slug / raw photobook_id / raw report_id / raw URL / token / Cookie / DATABASE_URL / Secret 値 / source_ip_hash 完全値 / scope_hash 完全値 / reporter_contact 実値 / detail 実値: chat / commit / docs / failure-log には **未含有**
- smoke target は redact ラベル（"unlisted smoke candidate" / "public hidden target"）で参照
- cmd/ops 出力は redact 形式で扱う（PR36 STOP ε で得た教訓を反映）

## 9. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-01 | 初版（STOP β 完了時点）。STOP γ / ε / final closeout は未実施として記録 |
