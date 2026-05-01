# create-entry optional fields hotfix 計画書（m2-create-entry-optional-fields-fix）

> 作成: 2026-05-01
> 状態: **STOP α/β（原因記録 + 実装 + tests + commit + push）**。STOP γ Backend deploy 承認待ちで停止
> 起点: m2-image-processor-job-automation の STOP ε smoke で、空欄 title / creator_display_name で submit すると `/create` 経路が HTTP 500 を返すことが発覚（2026-05-01 02:54:34 / 02:55:01 / 02:57:24 UTC）。create-entry PR (m2-create-entry, commit `dc05511` / `98c7155`) は production に出ているが、空欄入力で submit 不能の状態
> 関連 docs:
> - [`docs/plan/m2-create-entry-plan.md`](./m2-create-entry-plan.md)（create-entry PR 本体）
> - [`docs/plan/m2-image-processor-job-automation-plan.md`](./m2-image-processor-job-automation-plan.md)（本 hotfix 完了後 STOP ε を再開）
> - [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §3.1（title / creator_display_name は **任意**）
>
> 関連 rules: [`.agents/rules/coding-rules.md`](../../.agents/rules/coding-rules.md), [`.agents/rules/testing.md`](../../.agents/rules/testing.md), [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md), [`.agents/rules/pr-closeout.md`](../../.agents/rules/pr-closeout.md)

---

## 1. 原因

### 1.1 仕様の整合性が崩れた地点

| 層 | 設計意図（任意） | 実装 |
|---|---|---|
| 業務知識 v4 §3.1 | 任意 | — |
| `m2-create-entry-plan.md` §1.3 / §4 | 任意 | — |
| Frontend UI（`CreateClient.tsx`） | 「任意」「後で入力できます」を明示 | 空欄を許容（placeholder のみ） |
| Frontend lib（`createPhotobook.ts:88-89`） | 任意 | `title: input.title ?? ""` / `creator_display_name: input.creatorDisplayName ?? ""` で空文字を Backend に送信 |
| Backend handler（`create_handler.go:137-145`） | 任意 | length check のみで空文字を許容 |
| **Backend domain（`photobook.go:450-468`）** | **任意のはず** | **`if s == "" { return ErrEmptyTitle / ErrEmptyCreatorName }` で空文字を拒否** |

domain だけが「必須」設計のままだったため、handler 経由で空文字が UseCase に到達 → `domain.NewDraftPhotobook` で `ErrEmptyTitle` / `ErrEmptyCreatorName` を返却 → handler が `usecase.Execute` の error を 500 で固定マスク（`writeJSONStatus(w, 500, bodyServerError)`）→ Frontend が `server_error` kind に mapping して「一時的なエラーが発生しました」を表示。

### 1.2 二次的な不整合（同時に修正対象）

- handler `maxTitleLen = 100`（`create_handler.go:42`）vs domain `maxTitleLen = 80`（`photobook.go:73`）
- 80〜100 文字の title が handler を通過して domain で `ErrTitleTooLong` を返す → 同じ 500 経路に落ちる潜在バグ
- handler を 80 に揃え、超過時は handler 段階で 400 invalid_payload 固定にする

### 1.3 publish-time の必須要件は維持される（重要）

- `domain.CanPublish()`（`photobook.go:280`）が **独立して** `creatorDisplayName == ""` を check する設計
- `publish_handler.go:119-120` が `ErrEmptyTitle` / `ErrEmptyCreatorName` を 409 conflict にマッピング（情報漏洩抑止のため理由は区別しない）
- 本 hotfix では publish-time path を一切触らない。draft 作成時のみ空欄許容に切り替える

---

## 2. 修正方針（A + D）

### 2.1 A: domain validation 緩和（draft 作成のみ）

- `validateTitle(s string)` / `validateCreatorName(s string)` の `if s == ""` 早期 return を削除
- 残るのは length-only 検証（`len([]rune(s)) > maxTitleLen` / `> maxCreatorNameLen`）
- `ErrEmptyTitle` / `ErrEmptyCreatorName` 定数は publish-time path（`CanPublish`、`publish_handler` のエラーマッピング）が引き続き使用するため **保持**
- `validateTitle` / `validateCreatorName` の唯一の caller は `NewDraftPhotobook` であり、削除しても publish-time の挙動には影響しない（caller 確認済）

### 2.2 D: max length 整合

- handler `maxTitleLen` を **100 → 80** に変更（domain と一致）
- handler `maxCreatorDisplayNameLen = 50` は domain `maxCreatorNameLen = 50` と既に一致 → **不変**
- 80〜100 文字 title が handler 通過で domain 500 を返す経路を消す
- 81 文字以上は handler 段階で 400 invalid_payload 固定（既存挙動を 100 → 80 に閾値だけ移動）

### 2.3 Frontend は触らない（hotfix scope を最小に）

- `CreateClient.tsx` の `maxLength={100}` → `maxLength={80}` の整合は **本 hotfix では実施しない**（Workers redeploy を avoidance）
- 影響: ユーザは UI 上は 100 文字まで入力できるが 81 文字以上で submit 時 400 を受ける → UX 上のマイナーな不整合だが致命バグではない
- 後続: 別 PR or m2-image-processor-job-automation final closeout で frontend maxLength を 80 に揃える項目を roadmap に記録

---

## 3. 影響範囲

| 対象 | 変更内容 | テスト |
|---|---|---|
| `backend/internal/photobook/domain/photobook.go` | `validateTitle` / `validateCreatorName` の empty check 削除（length-only） | 既存 domain test を更新 |
| `backend/internal/photobook/domain/photobook_test.go` | 「異常_空title」/「異常_空creator_name」を「正常_空文字許容」に書換 | — |
| `backend/internal/photobook/internal/usecase/create_and_touch_test.go` | 同上の usecase 側テストを書換 | — |
| `backend/internal/photobook/interface/http/create_handler.go` | `maxTitleLen` を 100 → 80 | 既存 length test は `maxTitleLen+1` 形式で自動追従 |
| `backend/internal/photobook/interface/http/create_handler_test.go` | **追加**: 空欄 title / creator で 201 が返ることを fake repo 経由で検証する success-path test を新設 | — |
| Frontend | 変更なし | — |
| migration | 不要 | — |
| Workers redeploy | 不要 | — |

---

## 4. deploy / smoke 方針

- **STOP α/β**: 本書 + 実装 + tests + 単一 commit + push（本セッションで完了）
- **STOP γ**: Backend Cloud Build manual submit で hotfix を本番反映（既存 `cloudbuild.yaml` + runbook、要 user 承認）
- **STOP δ（smoke）**: deploy 後に空欄 title / 空欄 creator_display_name + valid Turnstile で 201 が返ることを Safari 実機で確認（要 user 承認）。raw token / photobook_id は記録しない
- **resume**: m2-image-processor-job-automation の STOP ε（upload smoke）を再開

---

## 5. raw token / ID 非記録方針

- 修正コード / テスト / commit message / 報告に raw `photobook_id` / `draft_edit_token` / `image_id` / Cookie / Secret / `DATABASE_URL` / R2 credentials / Cloud SQL instance ID を出さない
- handler / domain test 内の test fixture は既存 builder（`newID(t)` / `newDraftHash(t)`）経由
- Cloud Run logs / Cloud Build logs の Secret grep を STOP γ deploy 後に実施
- `.claude/scheduled_tasks.lock` は触らない

---

## 6. STOP 設計

| STOP | 内容 | 実行 |
|---|---|---|
| **α/β** | 本書 + 実装 + tests + 単一 commit + push（本セッションで完了） | 本セッション |
| **γ** | Backend Cloud Build deploy（image tag は新 SHORT_SHA） + Job / Scheduler image tag の追従更新（`vrcpb-image-processor` / `vrcpb-outbox-worker` は古い `vrcpb-api:98c7155` のままで OK か別途判断） | **要 user 承認** |
| **δ** | deploy 後の create-entry empty fields smoke（Safari 実機 1 周のみ、空欄 title / creator → 201 → /draft → /edit 到達） | **要 user 承認** |
| **resume** | m2-image-processor-job-automation の STOP ε（upload smoke）を再開 | — |

---

## 7. closeout で更新する資料（hotfix 完了時）

- `harness/work-logs/2026-05-XX_create-entry-optional-fields-fix-result.md`（新規）— 原因 / 修正範囲 / deploy 結果 / smoke 結果（redacted）
- `docs/plan/vrc-photobook-final-roadmap.md` — 本 hotfix を新 PR 番号で記録、create-entry PR との関係を明記
- `docs/plan/m2-create-entry-plan.md` — §1.3 / §4 に「title / creator_display_name は任意。domain の empty 拒否は本 hotfix で除去」と追記
- `harness/failure-log/2026-05-01_create-entry-empty-optional-fields-500.md`（**新規起票必須**）— layered validation 不整合（UI / lib / handler / domain）が production まで残った経緯と再発防止
- `.agents/rules/` 起票判断: 「複数層を跨ぐ任意項目の validation を実装する PR は、各層が同じ optional 制約を持つかを必ず横並びレビューする」をルール化するか
- frontend `CreateClient.tsx` `maxLength={100}` → `={80}` を後続 PR で実施する項目を roadmap に追記

---

## 8. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-01 | 初版作成（STOP α/β）。m2-image-processor-job-automation STOP ε で発覚した production の domain validation 不整合を hotfix scope として独立計画化 |
