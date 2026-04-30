# 失敗: PR36 STOP ε 実機 429 smoke で固定窓 rollover により smoke が再発火

> 発生日: 2026-05-01（UTC 2026-04-30）
> 関連: PR36 STOP ε / [`harness/work-logs/2026-04-30_pr36-usage-limit-result.md`](../work-logs/2026-04-30_pr36-usage-limit-result.md) §13
> ルール化: [`docs/runbook/usage-limit.md`](../../docs/runbook/usage-limit.md) §11 に反映

## 1. 発生状況

- PR36 STOP ε で iPhone Safari × ReportForm × 429 UI 実機確認を実施
- target photobook は `019dd1bb`（visibility=public、PR33d OGP test）を一時 unhide して使用
- Step 3 で 1 回正常 submit → reports +1 / outbox +1 / usage_counters +2（5 分窓 narrow / 1 時間窓 broad）
- Step 5 (Plan F) で **狭粒度 (5 分窓) counter のみ** を `count=limit_at_creation=3` に UPDATE して 429 を期待

## 2. 失敗内容

- ユーザーが iPhone Safari で 2 連続 submit したが **両方とも thanks view が表示**された
- 期待: 1 回目で 429 rate_limited 文言が出る
- 実際: reports / outbox 共に +2、usage_counters は **新しい 5 分窓に新規 counter** が作成され、count=0 から増分
- 副作用: 想定 reports +1 / outbox +1 / counter +2 のところ、reports +3 / outbox +3 / counter +4 に膨張（後段 cleanup TX で全削除済）

## 3. 根本原因

**5 分固定窓の rollover**:

- Step 5 UPDATE 対象: `9c936c25...` / window_start=`17:05:00` / count=3 / limit=3
- ユーザー Step 6 submit 時刻: `17:12:18` 〜 `17:12:53` → window_start=`17:10:00`（**新窓**）
- 新窓は count=0 から始まるため、increment 後 count=1, 2 共に limit=3 内 → 通常 submit が成立

PostgreSQL の固定窓 counter は `INSERT ... ON CONFLICT DO UPDATE` で `(scope_type, scope_hash, action, window_start)` の 4 列複合キーで row を識別する。`window_start` が変われば別 row なので、旧窓の count UPDATE は新窓に **影響しない**。

人間の操作時間（cmd/ops UPDATE → ユーザー画面操作 → submit）と 5 分窓の境界がぶつかる確率は十分高く、smoke 設計上の見落としだった。

## 4. 影響範囲

- 本番 DB: `reports` / `outbox_events` / `usage_counters` に smoke 由来行が 3 + 3 + 4 件発生
- 後段 cleanup TX で全削除（PR35b 由来の `resolved_action_taken` 行 / 既存 photobook.* outbox は保持）
- target photobook: 最終的に hidden=true / visibility=public に復元（開始時と同一）
- 観測上の漏えい: なし。raw scope_hash / source_ip_hash / report_id / slug / IP は chat / work-log に未記録（cmd/ops `[ok]` 行で raw photobook_id が **chat に 1 度だけ表示**された事実は本書 §6 と work-log §15 に redact 形式で記録）

## 5. 対策（Plan F → Plan G）

復旧手順は **狭粒度 current 窓 + 広粒度 1 時間窓の両方を threshold 化**（Plan G）:

- narrow `9c936c25...` / window=`17:20:00`（現窓）/ count=3 / limit=3 を UPSERT
- broad `4d68f33b...` / window=`17:00:00` / count=20 / limit=20 を UPDATE
- 次回 submit:
  - 狭粒度 inc → 4 > 3 → 429（17:25 までヒット）
  - 仮に狭粒度が rollover（17:25-）した場合でも、広粒度 inc → 21 > 20 → 429
- 結果: 1 回 submit で 429 文言「短時間に通報を送信しすぎました。3 分ほど時間をおいて再度お試しください。」を確認

## 6. 再発防止策

### 6.1 ルール化

`docs/runbook/usage-limit.md` §11.1〜§11.5 として以下を明記:

1. **手動 smoke で固定窓 counter を threshold 化する場合、短窓だけに依存しない**。必ず長窓も同時に threshold 化して rollover 跨ぎ耐性を確保
2. current window はサーバ now 基準で計算（`now() AT TIME ZONE 'UTC'` を取得）
3. SubmitReport smoke target は visibility=public 必須（unlisted は通報不可）
4. cleanup は `FOR UPDATE` lock + rowcount 想定値一致確認 + 不一致なら ROLLBACK
5. cmd/ops 出力の redact は sed 後段だけに依存せず、出力元の redact 形式を尊重し、`[ok]` 行など末尾サマリも redact 対象に含める

### 6.2 後続 PR で検討するもの

- `cmd/ops usage seed`（smoke 専用 counter 作成支援、本番では未実装）
- `cmd/ops usage reset`（smoke 後の cleanup 自動化、未実装）
- CheckOnly + Consume 分離 / reservation 方式（片方 consume 副作用解消）

## 7. 対策種別

- [x] ルール化（runbook §11 追加）
- [x] failure-log 起票（本書）
- [ ] スキル化（不要、PR 限定の実機 smoke）
- [ ] テスト追加（不要、unit + 実 DB 統合は既に commit 3.5/3.6 で網羅）
- [ ] フック追加（不要）

## 8. 関連

- [`harness/work-logs/2026-04-30_pr36-usage-limit-result.md`](../work-logs/2026-04-30_pr36-usage-limit-result.md) §13.4
- [`docs/runbook/usage-limit.md`](../../docs/runbook/usage-limit.md) §11
- [`docs/plan/m2-usage-limit-plan.md`](../../docs/plan/m2-usage-limit-plan.md) §17.2（片方 consume 副作用、MVP 許容）
- [`.agents/rules/feedback-loop.md`](../../.agents/rules/feedback-loop.md)
