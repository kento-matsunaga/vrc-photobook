# 2026-04-29 PR35b STOP ε で Turnstile widget verification 完了前に Report 送信が成立した

## 発生状況

- **何をしようとしていたか**: PR35b（Report 集約 / 公開通報窓口）STOP ε として、Cloudflare Workers 経由で本番環境の `/p/[slug]/report` に iPhone Safari からアクセスし、harassment_or_doxxing で 1 件だけ通報送信する正常系 smoke を実施しようとした。
- **どのファイル/モジュールで発生したか**:
  - `frontend/components/Report/ReportForm.tsx`（PR35b commit 4 で新設）
  - `frontend/lib/report.ts`（PR35b commit 4 で新設）
  - `backend/internal/report/internal/usecase/submit_report.go`（PR35b commit 2）
  - `backend/internal/report/interface/http/handler.go`（PR35b commit 2）

## 失敗内容

- iPhone Safari 上で `/p/[slug]/report` を開き、reason=harassment_or_doxxing / detail / reporter_contact を入力したのち、**Cloudflare Turnstile widget の verification が完了せずスピナーが回り続けている状態**で送信ボタンが押下可能になり、submit が成立した。
- 結果として、Backend は `report_submit` action 紐付きの Turnstile siteverify を**通過しないまま** reports 1 行 + outbox `report.submitted` 1 行を INSERT した（同一 TX 2 要素）。
- ユーザーから「STOP ε は NG / 正常系 smoke として続行しない」「過去に同じ事象がある可能性、メモが残っていないならハーネスエンジニアリング失格」という指摘を受け、`harness/work-logs/2026-04-27_frontend-upload-ui-result.md`（PR22 frontend-upload-ui）に同種の対策（L1-L4 多層 Turnstile ガード）を実装した記録があるにも関わらず、本 PR の Report 経路に水平展開されていなかったことが確認された。

> 詳細: `harness/work-logs/2026-04-29_report-result.md`（本 cleanup の進行記録）

## 根本原因

1. **L1（送信ボタン disable 条件）が空白許容**: `ReportForm.tsx` は `const canSubmit = turnstileToken !== "" && formState !== "submitting";` で disable 判定していたため、空文字でなければ tap で送信できる。Turnstile widget の `data-callback` で得たトークンは検証完了前から非空文字列が一時的にセットされる経路がありうる（例: widget が中断されたあとに古い state が残る場合等）。一方 PR22 の Upload では `if (typeof turnstileToken !== "string" || turnstileToken.trim() === "") { ... reject ... }` を **L1 / L2 / L3 の 3 層**で噛ませてあり、Report ではこれが踏襲されていなかった。
2. **L2（onSubmit 冒頭の再評価ガード）が無い**: `onSubmit` の最初で再度 `turnstileToken.trim()` を確認して early return / reject する処理が無く、`canSubmit` 判定だけに依存していた。
3. **L3（API client lib の defensive guard）が無い**: `lib/report.ts` の `submitReport` は引数 `turnstileToken` をそのまま POST body に詰めるだけで、空文字 / whitespace を `SubmitReportError("turnstile_failed")` として弾いていない。`lib/upload.ts` には L3 ガードが実装済（PR22）だが、Report ではコピーされなかった。
4. **L4（Backend 側の trim 後空文字拒否）が不徹底**: `usecase/submit_report.go` および `interface/http/handler.go` で `if in.TurnstileToken == ""` のみ確認しており、whitespace のみのトークン（空白 / タブ / 改行）は素通りする。Cloudflare siteverify は空白を含む不正トークンには 400 を返すが、Backend 側で先に validate しないと Cloudflare 経由の挙動に依存することになる。
5. **既存対策の水平展開漏れ**: PR22 で確立した L1-L4 多層 Turnstile ガードは `harness/work-logs/2026-04-27_frontend-upload-ui-result.md` に書かれた**実装ノウハウ**でしかなく、`.agents/rules/` のルール化はされていなかった。そのため PR35b の Report 経路で**同じパターンを踏襲する強制力が無く**、新設フォームで自然に劣化版が再発した。

## 影響範囲

- **本番データへの影響**: NG 由来の reports 1 行 + outbox `report.submitted` 1 行を Cloud SQL 上に作成してしまった。**raw 値は work-log / commit / chat に出さない方針** で、本セッション内 cleanup TX により signature 一致 1 件のみを安全に DELETE 済（target_photobook_id 配下 reports count: 1 → 0、outbox `report.submitted` 全体: 1 → 0）。Moderation hide は別途 unhide → hide で復元済（target photobook の hidden=true 維持）。outbox `photobook.unhidden` / `photobook.hidden` の pending は no-op handler 経由で processed 済。
- **設計への影響**: 「Turnstile 必須」は ADR-0005 で確定済の前提条件であり、これが破られると Report の仕様（不正大量送信防止 / source_ip_hash の意義）が成立しなくなる。本失敗は同等の劣化が **任意の Turnstile 経路（upload / report / 将来追加される form）で再発しうる**ことを示している。
- **harness エンジニアリング上の影響**: 「過去 PR の対策は work-log 記録だけでは水平展開されない」という、ハーネス運用の構造的な弱点を露呈した。失敗の再発防止には**ルール化が必須**である（feedback-loop.md §Step 2 のとおり）。

## 対策種別

- [x] ルール化（`.agents/rules/turnstile-defensive-guard.md` 新設）
- [ ] スキル化
- [x] テスト追加（L1 / L3 / L4 の単体テストを Frontend / Backend 双方に追加）
- [ ] フック追加（将来 grep ベースの hook 化を検討。現時点はルールのみ）

## 取った対策

### 1. データ後始末（cleanup）

- cloud-sql-proxy 経由で Serializable TX を張り、SELECT で signature 一致 1 件と pending outbox 1 件を確認したうえで `DELETE outbox_events WHERE event_type='report.submitted' AND ...` → `DELETE FROM reports WHERE target_photobook_id=$1 AND status='submitted' AND reason='harassment_or_doxxing'` → COMMIT。期待件数と異なれば ROLLBACK して停止する組み立て。
- 影響件数: outbox 1 / reports 1。raw report_id / target_photobook_id / detail / reporter_contact / source_ip_hash は一切標準出力に出さず、redact 出力（prefix 8 文字 + 件数）のみで判定。
- moderation hide は cmd/ops 経由で `--reason policy_violation_other --execute`（`--source-report-id` なし）で hidden=true を復元済。outbox `photobook.unhidden` / `photobook.hidden` の pending 2 件は Cloud Run Jobs `vrcpb-outbox-worker --once --max-events 1` を 2 回実行して no-op handler で processed 化。

### 2. ルール化（再発防止）

`.agents/rules/turnstile-defensive-guard.md` を新規作成し、Turnstile を使う**全 Frontend form** + **全 Backend endpoint** に L1-L4 多層ガードを必須化。

### 3. 実装修正（PR35b 内で水平展開）

- `frontend/components/Report/ReportForm.tsx`: L1 を `turnstileToken.trim() !== ""` に強化、L2 として onSubmit 冒頭で `if (turnstileToken.trim() === "") return` を追加。
- `frontend/lib/report.ts`: L3 として `if (typeof turnstileToken !== "string" || turnstileToken.trim() === "") { reject SubmitReportError("turnstile_failed") }` を追加。
- `backend/internal/report/internal/usecase/submit_report.go` + `backend/internal/report/interface/http/handler.go`: L4 として `strings.TrimSpace(in.TurnstileToken) == ""` を early-return（403 turnstile_failed）に変更。
- 各層に対応するユニットテスト（whitespace-only / 空文字 / 正常）を Frontend（vitest）+ Backend（go test）の双方に追加。

### 4. STOP γ2 / δ2 / ε2

修正反映のため Backend Cloud Build deploy（STOP γ2）+ Workers redeploy（STOP δ2）+ Safari 実機 1 件 smoke（STOP ε2）を実施する。

## 横展開すべき領域（次の PR ラインで必ず拾う）

> **upload-verification 側の同種リスクは PR35b の修正範囲には含まれていない。**
> 既知のリスクなので、次に着手する PR ライン（PR36 以降）の冒頭で必ず拾う。
> 後送り（先送り）にはしない。`docs/plan/vrc-photobook-final-roadmap.md` §1.3 運用 / インフラ 冒頭にも明記済。

具体的にやること:

- `backend/internal/uploadverification/interface/http/handler.go` の `if req.TurnstileToken == ""` を `strings.TrimSpace(...) == ""` に強化（L4 handler）
- `backend/internal/uploadverification/internal/usecase/issue.go`（または該当ファイル）にも UseCase 入口で同条件の早期 return を追加（L4 UseCase）
- `frontend/lib/upload.ts` `issueUploadVerification` の既存 L3 ガード（`turnstileToken.trim() === ""`）が **本ルール準拠であること**をセルフレビューし、不足あれば追加
- Frontend `components/Upload/*` の送信ボタン disable / onSubmit が L1+L2 構成として trim 済であることを確認
- 各層に whitespace-only / 空文字 / 正常の単体テスト追加（テーブル駆動 + description 必須）
- Safari 実機（macOS + iPhone）で「Turnstile widget スピナー中に upload 開始ボタンを押下しても submit されない」ことを確認

その他:

- 今後 Turnstile を使う form を新設する場合（将来の問い合わせ / 再発行依頼 / 公開コメント等）は `.agents/rules/turnstile-defensive-guard.md` を必ず参照する。
- `.agents/rules/safari-verification.md` 改訂時の必須確認項目に「Turnstile 完了前 submit が成立しないこと」を追加検討（後続）。

## 関連

- `.agents/rules/turnstile-defensive-guard.md`（本失敗を起点に新設）
- `.agents/rules/feedback-loop.md`（失敗 → ルール化 → テスト の運用原則）
- `.agents/rules/safari-verification.md`（Turnstile を含む submit フロー Safari 検証）
- `harness/work-logs/2026-04-27_frontend-upload-ui-result.md`（PR22 で L1-L4 が初回実装された記録、未ルール化のままだった）
- `docs/adr/0005-turnstile-action-binding.md`（Turnstile action 厳密一致と必須化の基盤）
- `docs/plan/m2-report-plan.md`（PR35b 計画書）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-29 | 初版作成。PR35b STOP ε NG（Turnstile bypass）を契機に L1-L4 多層ガードを正典ルール化 |
