# SubmitReport visibility 要件の設計判断（PR36 STOP ε 由来）

> 作成: 2026-05-01
> 起点: PR36 STOP ε で実機 smoke 中に発見した **3 層に跨る仕様差分**
> **状態: 案 B 採用で確定（2026-05-01）**。実装は独立 PR で別サイクル
> 関連: [`harness/work-logs/2026-04-30_pr36-usage-limit-result.md`](../../harness/work-logs/2026-04-30_pr36-usage-limit-result.md) §13 / [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md) §1.3 後続候補

## 0. 決定（2026-05-01）

ユーザー判断により **案 B（Backend を `visibility != private` に緩和）** で確定。

確定事項:

1. **`unlisted` も Report submit 対象**にする
2. `private` は引き続き Report submit 不可
3. `hidden_by_operator=true` は引き続き不可
4. `status != published` は引き続き不可
5. **Frontend / Public API / Workers redeploy は原則不要**（API 互換維持）
6. **Backend UseCase + tests の独立 PR** として小さく実装
7. migration / Secret は不要
8. **業務知識 v4 §3.6 に 1 行追記**:「閲覧可能な published photobook（`public` / `unlisted`）は通報可能。`private` / `hidden` / `draft` は不可」と分かる文を入れる
9. **PR35 計画書 §17 #2 は `visibility != private` 方針で確定**（実装が「最小: public のみ」を採用していたものを「推奨: `!= private`」に戻す）

実装範囲とリリース方針は §6 推奨 + §9 STOP 設計（案 B）に従う。Safari 実機 smoke は §7.4 + §11 注意事項（PR36 STOP ε で得た rollover / cleanup ノウハウ）を踏まえて行う。

## 1. 何が起きているか — 3 層の不整合

公開 Viewer / Report ページ / SubmitReport の **3 層で visibility 受け入れ範囲がズレている**:

| 層 | 実装 | `public` | `unlisted` | `private` | `hidden=true` |
|---|---|---|---|---|---|
| ① 公開 Viewer (`get_public_photobook.go:244`) | `visibility != private` AND `hidden = false` | 200 ✓ | **200 ✓** | 404 ✗ | 410 ✗ |
| ② Viewer footer 通報リンク (`ViewerLayout.tsx:65`) | photobook が render される全件で `<Link href="/p/{slug}/report">このフォトブックを通報</Link>` | 表示 | **表示** | (viewer 自体 404) | (viewer 自体 410) |
| ③ `/p/[slug]/report` ページ (`report/page.tsx:48`) | `fetchPublicPhotobook(slug)` を使うため ① と同条件 | 200 + Form | **200 + Form** | notFound() | gone view |
| ④ **SubmitReport (`submit_report.go:163`)** | `visibility == 'public'` AND `hidden = false` AND `published` | 200 ✓ | **404 ✗** | 404 ✗ | 404 ✗ |

→ unlisted の挙動が **「Viewer 表示 → 通報リンク表示 → ReportForm 表示 → submit で `not_found`」** の **末端だけが拒否**になっている。PR36 STOP ε で実機 smoke 中に「通報を送信」を押したところ「通報対象のフォトブックが見つかりませんでした」（`SubmitReportError.kind=not_found` → `ReportForm.tsx:233`）が表示された経緯と一致。

業務知識 v4 §2.6 で MVP 既定 visibility は `unlisted` なので、**通常作成された photobook ほどこの不整合が起きる**。

## 2. 業務知識・既存計画との関係

### 2.1 業務知識 v4

- **§2.6**: 「公開範囲」= `public` / `unlisted` / `private` の 3 値。`unlisted` は **MVP 既定値**（公開 URL を知っている人のみ閲覧可能）。`public` は明示選択
- **§3.6 担うこと**: 「フォトブック閲覧ページから通報画面への遷移を受け付ける」「通報対象のフォトブックを特定する」
- **§3.6 守ること**: 「閲覧者は自分のログインや登録なしに通報を行える」
- §3.4 / §3.6 文脈: 「閲覧者」= ログイン不要・URL 到達可能な誰か（unlisted URL 保持者を排除する記述は無い）

### 2.2 PR35 計画書 (`docs/plan/m2-report-plan.md`)

- **§G4**: 「通報対象を published + visible + not hidden の photobook に限定」
- **§17 判断 #2**: 「**推奨: published + visible + not hidden のみ受付**（v4 ドメイン §6.1 は『削除済みも対応のため受付』だが MVP 簡素化）。**最小は public のみ**」

→ "visible" の解釈に "公開 Viewer の `!= private`" / "`visibility == 'public'`" の 2 通りがあり、コード実装は **「最小は public のみ」を採用**してしまった。Frontend は **「公開 Viewer の `!= private`」** を採用（`fetchPublicPhotobook` を経由）。**両者の解釈不一致が今回の根因**。

### 2.3 PR36 STOP ε の経緯

- production の published photobook 4 件のうち、**`visibility=public AND hidden=false`（= 通報可能） は 0 件**だった
- target を **public hidden target**（visibility=public, hidden=true）に切り替え、一時 unhide → smoke → 再 hide で実機 429 確認
- そのまま放置すると、本番で運用される大半の unlisted photobook で通報導線が機能しない

## 3. 「URL を知っていれば見られるものは通報できるべきか」という運営方針

業務知識からは「`unlisted` は限定共有（URL 保持者のみ）」だが「閲覧者は通報できる」とある。次の 2 つの解釈軸が存在:

| 軸 | 解釈 | 帰結 |
|---|---|---|
| **公開意思軸** | 公開意思を明示した `public` のみ通報対象。`unlisted` は限定共有なので運営介入対象外 | 案 A / C |
| **可視性軸** | URL に到達できる閲覧者は誰でも通報可能（abuse / minor_safety_concern を含む）| 案 B |

業務知識 v4 §3.6 の記述（「閲覧者は通報できる」）と MVP 既定値 (`unlisted`) を組み合わせると、**可視性軸の方が業務知識と整合する**。一方、unlisted を「秘匿性のあるリンク共有」と扱う運営判断もあり得る。

## 4. 選択肢

### 案 A — 現仕様（Backend public-only）維持 + Frontend を Backend に揃える

**変更**:
- Backend SubmitReport: 維持（`visibility == 'public'`）
- Backend 公開 API: 応答に `visibility` または `reportable: bool` を追加（`backend/internal/photobook/interface/http/public_handler.go` の `publicPhotobookPayload` を拡張）
- Frontend `lib/publicPhotobook.ts` `PublicPhotobook` 型: `visibility` または `reportable` フィールド追加
- Frontend `ViewerLayout.tsx`: `if (photobook.visibility === 'public' && !photobook.hidden) {…}` の条件で footer 通報リンクを表示
- Frontend `app/(public)/p/[slug]/report/page.tsx`: visibility 判定で「対象外」view に分岐、ReportForm を出さない
- 業務知識 v4 §3.6 / §2.6: 「通報は visibility=public のみ」を明文化（解釈の固定化）
- runbook `usage-limit.md` §11.3 更新

**メリット**:
- 公開意思軸を採用、限定共有ユーザーの意図を尊重
- submit 段階で 404 が出る現象を Frontend 段階で吸収、UX 上の「リンクを押して拒否される」を解消

**デメリット**:
- 業務知識 v4 §3.6「閲覧者は通報できる」の解釈を **狭める方向で改定**が必要
- Backend API の応答に visibility が露出（敵対者対策上の影響軽微、ただし既存方針「外部に状態区別を漏らさない」とは緊張関係）
- unlisted 経由の abuse / minor_safety_concern への通報経路が無くなる
- 実装インパクトが **Backend + Frontend 両層** に及ぶ（API 拡張 + 型 + Viewer + Report ページ + tests + Workers redeploy + Safari 確認）
- 業務影響: MVP 既定 unlisted で通報できないので、既定値を `public` に変更する圧力 / 業務知識改定の連鎖

### 案 B — Backend を緩和（`visibility != private` で受け入れ）

**変更**:
- Backend `submit_report.go:163`: `pb.Visibility().String() != "public"` → `pb.Visibility().Equal(visibility.Private())` に差し替え（公開 Viewer と同じ判定）
- Backend tests: `visibility=unlisted` ケースを成功 path に追加（`submit_report_test.go` テーブル駆動）
- Backend tests: 既存の `visibility=public` 成功 / `private` / `hidden=true` 拒否は維持
- Frontend: **変更なし**（既に `fetchPublicPhotobook` 経由で unlisted も到達可能。Backend を Frontend の挙動に合わせる方向）
- runbook `usage-limit.md` §11.3 を「`visibility != private` で受け入れ」に更新
- 業務知識 v4 §3.6 受入条件: 「`visibility != private` AND `hidden_by_operator = false` AND `status = published` の photobook を通報対象とする」を 1 行追記（解釈の固定化、任意だが推奨）
- PR35 計画書 §17 判断 #2: 採用案を「推奨: `published + visible + not hidden`」と明示する形で固定

**メリット**:
- 業務知識 v4 §3.6「閲覧者は通報できる」の自然な解釈と整合
- 公開 Viewer との挙動が完全整合（「Viewer で開ける ≡ 通報できる」）
- MVP 既定 `unlisted` での運用と整合、Report 機能が現実に有効化
- abuse / minor_safety_concern 用途への通報経路を確保
- PR35 計画書 §17 #2 の **推奨案**に整合（実装が「最小」案を採用した結果として現状が生まれており、推奨に戻す）
- 実装インパクトが **Backend のみ + 1 行差し替え + test 追加**で最小

**デメリット**:
- 仕様変更による行動変化（unlisted で誤通報・運営負荷増の可能性。ただし MVP では運営手動対応なので影響限定）
- Safari 実機で `visibility=unlisted` で submit 成功 path を再 smoke（5〜10 分、副作用 1 件 + cleanup）
- 業務知識を更新する場合 1 行追記、しない場合は計画書だけで意図が読める前提に依存

### 案 C — `public` only を正式仕様化（業務知識・UI・Backend API 全面改定）

**変更**:
- Backend SubmitReport: 維持
- Backend 公開 API: 案 A と同じく `visibility` または `reportable` を応答に追加
- Frontend: 案 A と同じく Viewer footer / Report ページで visibility 分岐
- ReportForm エラー mapping: `not_found` → 「対象外」文言（誤って submit された場合のフォールバック）
- **業務知識 v4 §3.6 を改定**: 「通報は `visibility=public` のみ」を明文化
- **業務知識 v4 §2.6 を改定**: `unlisted` の "通報されない" 性質を追加
- m2-report-plan.md §G4 / §17 #2: "public のみ" を正式仕様として確定

**メリット**:
- 仕様の意図を明文化、開発者・運営の認知負荷が減る
- 「リンクを押して拒否される」を Frontend で解消

**デメリット**:
- 業務知識の **改定** が必要（影響範囲が大きい）
- unlisted 経由の abuse 通報経路が永続的に無くなる（運営方針の根本判断）
- 実装インパクトが案 A と同等で大きい
- MVP 既定 unlisted のままでは Report 機能の現実的な有効性が失われる
- 「将来 visibility=public をデフォルトに変えるか」というさらなる業務判断を誘発

## 5. 影響範囲（案ごと）

| 領域 | 案 A | 案 B | 案 C |
|---|---|---|---|
| Backend SubmitReport usecase | なし | 1 行 + import | なし |
| Backend SubmitReport tests | なし | 1〜2 ケース追加 | なし |
| Backend 公開 API レスポンス（`publicPhotobookPayload`）| `visibility` or `reportable` 追加 | なし | 同左 |
| Backend 公開 API tests | 拡張 | なし | 同左 |
| Frontend `PublicPhotobook` 型 | フィールド追加 | なし | 同左 |
| Frontend `ViewerLayout.tsx` footer | 条件分岐 | なし | 同左 |
| Frontend `/p/[slug]/report/page.tsx` | 「対象外」view 追加 | なし | 同左 |
| Frontend `ReportForm.tsx` 文言 | 必要に応じて | なし | 必要に応じて |
| Frontend tests | 各 component の visibility 分岐 | なし | 同左 |
| 業務知識 v4 §2.6 / §3.6 | **改定**（狭める）| 1 行追記（任意）| **改定**（狭める）|
| m2-report-plan.md | §17 #2 を「最小: public のみ」採用と明文化 | §17 #2 を「推奨: `!= private`」採用と明文化 | 案 A と同方向 |
| roadmap §1.3 | 後続項目を解消 | 後続項目を解消 | 後続項目を解消 |
| runbook `usage-limit.md` §11.3 | 更新（public のみ）| 更新（`!= private`）| 更新（public のみ）|
| Safari 実機 | unlisted で通報リンクが出ない確認 | unlisted で submit 成功 + 既存 public regression | unlisted で対象外 view 確認 |
| UsageLimit との関係 | 変化なし | 変化なし | 変化なし |
| `hidden_by_operator` との関係 | 変化なし（hidden=true は引き続き拒否）| 変化なし | 変化なし |
| OGP / public lookup | 変化なし | 変化なし | API 拡張のみ波及 |
| migration / Secret | 不要 | 不要 | 不要 |
| Workers redeploy 必要性 | **必要**（Frontend 変更）| 不要 | **必要**（Frontend 変更）|
| Backend deploy 必要性 | **必要**（API 変更）| **必要**（usecase 変更）| **必要**（API 変更）|

## 6. 推奨案

**推奨: 案 B（Backend を `visibility != private` に緩和）**

理由:

1. **業務知識 v4 §3.6 と整合**。「閲覧者は通報できる」の自然な解釈と一致
2. **公開 Viewer / Frontend の挙動と Backend を揃えるのが clean**。現状は Backend だけが厳格で、3 層の整合が取れていない不整合の解消
3. **MVP 既定値 (`unlisted`) との整合**。本番で大半の photobook が通報可能になる
4. **abuse 用途への通報経路を確保**。`minor_safety_concern` / `harassment_or_doxxing` 等は unlisted でも起きうる
5. **PR35 計画書 §17 #2 の「推奨」案に整合**。実装が「最小」案を採用したことで生まれた現状を「推奨」に戻す
6. **実装インパクトが最小**（Backend 1 行 + test、Frontend / API 拡張 / Workers redeploy / 業務知識改定すべて不要）
7. **Safari 確認も最小**（既存 unlisted photobook で submit 成功 path を 1 回確認すれば十分）
8. **失敗時の rollback が容易**（1 commit revert で戻せる、API 変更がないので Frontend 互換は維持）

## 7. 必要なテスト（案 B 採用時）

### 7.1 Backend 単体テスト

`backend/internal/report/internal/usecase/submit_report_test.go` のテーブル駆動に追加:

| name | description | photobook visibility | photobook hidden | photobook status | 期待 |
|---|---|---|---|---|---|
| 成功_unlisted_visible_published | Given: unlisted/hidden=false/published, When: submit, Then: 通報受付成功 + DB 行 + outbox event | unlisted | false | published | success |
| 既存: 成功_public_visible_published | （既存）| public | false | published | success |
| 既存: 拒否_private | （既存）| private | false | published | not eligible |
| 既存: 拒否_hidden | （既存）| any | true | published | not eligible |
| 既存: 拒否_draft | （既存）| any | * | draft | not eligible |

### 7.2 Backend 実 DB 統合テスト

PR36 commit 3.6 と同パターンで、`visibility=unlisted` photobook を seed して submit 成功 / DB 行 + outbox INSERT 確認。

### 7.3 Frontend tests

**変更なし**。`fetchPublicPhotobook` / `ReportForm` / `ViewerLayout` は元から unlisted を許容しており、submit 結果が「成功」になるだけなので新規テスト不要。

### 7.4 Safari 実機 smoke 注意事項

PR36 STOP ε で得た知見を踏まえて以下を厳守する:

- **unlisted smoke candidate**（visibility=unlisted, hidden=false）に対して submit 成功（thanks view）を確認
- **public hidden target**（visibility=public, 元 hidden=true）は **触らない**（regression リスクなし、検証コスト省略。必要なら別 STOP に分けて unhide → smoke → 再 hide サイクルで実施）
- **submit は 1 回だけ**。連打禁止。DB 調整完了や確認合図なしに追加 submit しない
- thanks view を確認、`report_id` / `token` / raw slug / raw URL / hash 完全値が画面・URL に出ないことを確認
- iPhone Safari でレイアウト崩れがないことを確認、Turnstile 失敗文言と区別される
- smoke 由来 `reports` / `outbox.report.submitted` / `usage_counters` は **一意確認して cleanup**:
  - `FOR UPDATE` lock + rowcount 一致 assert + 不一致なら ROLLBACK
  - **`outbox.report.submitted` は worker 処理せず DELETE する方針**を推奨（本物の通報として残さないため）
  - `usage_counters` は 24h grace で expire されるが、明示 DELETE で確実に消す
- raw photobook_id / raw slug / raw URL は work-log / commit / failure-log に書かない（chat 一回限りは可、redact 形式で記録）

## 8. rollback 方針（案 B）

- 1 commit に分離（usecase 1 行 + test）
- 不具合観測時:
  1. `git revert <commit>` で usecase 変更を取り消し
  2. Cloud Build で再 deploy
  3. Frontend は変更していないため Workers 互換は自動維持
- DB / API レスポンスの互換性は維持されるため、運用中の在庫データへの影響なし
- failure-log 起票 + 業務知識への記載修正

## 9. 実装に進む場合の STOP 設計

### 案 B 採用時

| STOP | 内容 | 必要性 |
|---|---|---|
| α | 計画書承認 + v4 §3.6 解釈確定（unlisted を含めるか） + 計画書 §17 #2 の採用方針確定 | 必要（小規模だが仕様変更）|
| β | Backend usecase 1 行差し替え + 単体テスト 1〜2 ケース + 実 DB 統合テスト | 必要 |
| γ | Backend Cloud Build deploy（Cloud Run revision 更新 + Cloud Run Job image 更新）| 必要 |
| δ | Workers redeploy | **不要**（Frontend 変更なし）|
| ε | Safari 実機: unlisted で submit 成功（thanks view）+ 既存 public で regression なし | 必要 |
| final closeout | work-log / roadmap §1.3 / runbook §11.3 / 業務知識 §3.6（任意 1 行）/ failure-log（必要なら）| 必要 |

migration / Secret は **不要**。

### 案 A / C 採用時（参考）

| STOP | 内容 |
|---|---|
| α | 業務知識 v4 §3.6 / §2.6 改定方針確定 + 計画書承認 + API 拡張形式（`visibility` か `reportable`）確定 |
| β1 | Backend 公開 API 応答に visibility/reportable 追加 + handler tests + 公開 viewer query 改修なし |
| β2 | Frontend `PublicPhotobook` 型拡張 + ViewerLayout / report page 条件分岐 + render テスト |
| γ | Backend Cloud Build deploy |
| δ | Workers redeploy（Frontend 反映）|
| ε | Safari 実機: unlisted で通報リンクが出ない / public で出る / report page 直叩きで対象外 view 表示 確認 |
| final closeout | work-log / roadmap / 業務知識改定 / m2-report-plan §17 #2 確定 |

## 10. 出してはいけないもの（再掲）

- raw slug / raw photobook_id / raw report_id / raw public URL
- token / Cookie / DATABASE_URL / Secret 値
- source_ip_hash 完全値 / scope_hash 完全値
- reporter_contact 実値 / detail 実値 / manage URL / storage_key 完全値

## 11. ユーザー判断 — 確定済（2026-05-01）

§0「決定」と整合。以下を全項目確定済として記録する:

1. **採用案: 案 B**（Backend `visibility != private` に緩和）
2. **業務知識 v4 §3.6 の解釈**: 「閲覧者は通報できる」の "閲覧者" に **unlisted URL 保持者を含める**（案 B）
3. **`hidden_by_operator=true` は引き続き拒否で変更なし**（3 案共通、確認）
4. **`status != published`（draft / deleted / purged）は引き続き拒否で変更なし**（3 案共通、確認）
5. **業務知識 v4 §3.6 への 1 行追記**: **やる**（public / unlisted は通報可能、private / hidden / draft は不可と分かる文）
6. **PR35 計画書 §17 #2 採用方針**: **`visibility != private`** で確定
7. **実装 PR の切り出し**: **独立 PR**（小さく実装、追跡容易、rollback 容易）
8. **Frontend / Public API / Workers redeploy**: 原則 **不要**（API 互換維持）
9. **migration / Secret**: **不要**

実装は次の独立 PR で行う。実装範囲は §6（推奨）/ §7（必要なテスト）/ §9（STOP 設計、案 B）に従う。

## 12. 参考リンク

- 業務知識 v4: [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §2.6 / §3.6
- PR35 計画書: [`docs/plan/m2-report-plan.md`](./m2-report-plan.md) §G4 / §17 判断 #2
- 該当コード:
  - `backend/internal/report/internal/usecase/submit_report.go:163`
  - `backend/internal/photobook/internal/usecase/get_public_photobook.go:244`
  - `backend/internal/photobook/interface/http/public_handler.go:64-76`
  - `frontend/components/Viewer/ViewerLayout.tsx:61-72`
  - `frontend/app/(public)/p/[slug]/report/page.tsx:48-62`
  - `frontend/lib/publicPhotobook.ts:55-71`
  - `frontend/lib/report.ts:24, 114`
  - `frontend/components/Report/ReportForm.tsx:232-233`
- PR36 STOP ε 経緯: [`harness/work-logs/2026-04-30_pr36-usage-limit-result.md`](../../harness/work-logs/2026-04-30_pr36-usage-limit-result.md) §13.1
- runbook: [`docs/runbook/usage-limit.md`](../runbook/usage-limit.md) §11.3
- roadmap 後続候補: [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md) §1.3
- `.agents/rules/safari-verification.md` / `.agents/rules/security-guard.md` / `.agents/rules/pr-closeout.md`

## 13. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-01 | 初版作成。3 層（Viewer / Report ページ / SubmitReport）の不整合を整理、案 A/B/C 比較・推奨 B・STOP 設計・必要テスト・rollback 方針を整理。実装には進まない |
| 2026-05-01 | ユーザー判断で **案 B 採用**で確定（§0 / §11）。raw photobook id_prefix を "public hidden target" / "unlisted smoke candidate" に redact。Safari smoke 注意事項を §7.4 に統合 |
| 2026-05-01 | **STOP β 実装完了**: `submit_report.go` に `assessReportEligibility` を抽出し `visibility != private` に緩和。`submit_report_test.go` に `TestAssessReportEligibility`（テーブル駆動 7 ケース、Builder 不要の `RestorePhotobook` ベース）を追加。業務知識 v4 §3.6 / m2-report-plan §17 #2 / m2-report-plan §5.2 / runbook usage-limit.md §11.3 / roadmap §1.3 を更新。STOP γ Backend deploy 承認待ち |
| 2026-05-01 | **STOP γ Backend deploy 完了**: Cloud Build `c77ad798-...` SUCCESS → `vrcpb-api:773d5cc`、Cloud Run rev `vrcpb-api-00023-pwv` 100% traffic、secretKeyRef 8 個・plain env 2 個維持、`/health` `/readyz` `bad-slug handler JSON 404` smoke 全 OK、Cloud Run Job image も `:773d5cc` に bump（Job 未実行）、redact 対象値 grep 0 件 |
| 2026-05-01 | **STOP ε Safari smoke 完了**: unlisted smoke candidate に対し iPhone Safari で submit 1 回 → thanks view 成立（緩和の核心動作を production で検証）。delta は reports +1 / outbox.report.submitted pending +1 / usage_counters +2 で期待一致。`FOR UPDATE` lock + rowcount assert 付き TX で全行 cleanup、target photobook 状態（visibility=unlisted / hidden=false）は不変。pending outbox 0、usage_counters 0、PR35b 由来 resolved_action_taken 行保持。proxy / DSN / tmp 全削除 |
| 2026-05-01 | **final closeout 完了**: 本書 §13 履歴更新、work-log を完了モードに更新、roadmap §1.1 を新 deploy 状態に追記、stale-comments 4 区分分類済、redact 対象値 grep 0 件、failure-log 起票不要（仕様意図的緩和、設計判断メモで網羅）|
