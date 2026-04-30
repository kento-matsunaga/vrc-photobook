# UsageLimit / RateLimit 運用 runbook（PR36 MVP）

> Backend Cloud Run （`vrcpb-api`）の UsageLimit / RateLimit 運用手順をまとめる。
> 通報・upload-verification・publish の 3 endpoint に対する利用上限の確認・運営対応を扱う。

## 0. 前提

- 業務知識 v4 §3.7「同一作成元 1 時間 5 冊まで」の確定値が publish の上限値
- 計画書: [`docs/plan/m2-usage-limit-plan.md`](../plan/m2-usage-limit-plan.md)
- ルール: [`.agents/rules/turnstile-defensive-guard.md`](../../.agents/rules/turnstile-defensive-guard.md) /
  [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)

## 1. UsageLimit の目的と Turnstile との違い

| 観点 | Turnstile | UsageLimit |
|---|---|---|
| 目的 | **bot か人間か** を判定 | **人間でも回数が多すぎる** を抑制 |
| 配置 | 公開操作前 / upload-intent 前 | 既存 UseCase の前段（payload 検証 + Turnstile siteverify の後） |
| 失敗時 | HTTP 403 / `turnstile_failed` | HTTP 429 / `rate_limited` |
| 合否 | siteverify を 1 回呼んで判定 | DB の固定窓 bucket を atomic increment して判定 |
| 関係 | **直列**配置（両方独立に評価） | 同上 |

両者は別軸の防御で、Frontend / 運営 UI でも別文言・別カテゴリとして扱う。

## 2. 対象 endpoint と閾値

| Endpoint | scope（複合キー） | window | limit |
|---|---|---|---|
| `POST /api/public/photobooks/{slug}/reports` | `source_ip_hash`（複合：sha256(ip_hash || pid)）| 5 分 | 3 |
| 同上（全体）| `source_ip_hash`（IP hash hex）| 1 時間 | 20 |
| `POST /api/photobooks/{id}/upload-verifications` | `draft_session_id`（複合：sha256(session_id || pid)）| 1 時間 | 20 |
| `POST /api/photobooks/{id}/publish` | `source_ip_hash`（IP hash hex）| 1 時間 | **5（業務知識 v4 §3.7 確定）** |

> `report.submit` は 2 本のレートリミットを直列で消費するため、1 本目で deny されたあとに 2 本目には到達しない。一方、1 本目が成功して 2 本目で deny されると **1 本目だけ count が進む副作用**がある（PR36 計画書 §17.2 で MVP 許容）。

## 3. HTTP 429 の見方

```http
HTTP/2 429
Retry-After: 1380
Cache-Control: private, no-store, must-revalidate
X-Robots-Tag: noindex, nofollow
Content-Type: application/json

{"status":"rate_limited","retry_after_seconds":1380}
```

- **`Retry-After`** header を最優先で読む（数値秒）
- header が無い場合は body の `retry_after_seconds` を使う（fallback）
- どちらも無い場合は Frontend は **既定 60 秒**で扱う
- **scope_hash / count / limit / IP / token は body / header に出ない**（敵対者対策）

## 4. Frontend 表示

| 経路 | 文言 |
|---|---|
| ReportForm | 「短時間に通報を送信しすぎました。N 分ほど時間をおいて再度お試しください。」 |
| Upload UI | 「短時間にアップロード操作を繰り返しています。N 分ほど時間をおいて再度お試しください。」 |
| Publish flow | 「公開操作の上限に達しました。1 時間あたりの公開数には上限があります。N 分ほど時間をおいて再度お試しください。」 |

`N 分ほど` は `frontend/lib/retryAfter.ts` の `formatRetryAfterDisplay` で整形（60 秒未満は「1 分ほど」、60 分超は「1 時間 M 分ほど」等）。Turnstile 失敗（`turnstile_failed`）と完全に別 UI 状態として扱う。

## 5. cmd/ops usage list / show

### 5.1 一覧表示

```bash
# 全件（scope_type / action フィルタ無し、最大 50 件）
$OPS_BIN usage list

# action 指定
$OPS_BIN usage list --action=publish.from_draft

# scope_type + action
$OPS_BIN usage list --scope-type=source_ip_hash --action=report.submit

# threshold 超過のみ（count > limit_at_creation）
$OPS_BIN usage list --threshold-only

# scope_hash prefix 検索（前方一致 LIKE 'prefix%'）
$OPS_BIN usage list --scope-prefix=0123abcd
```

出力例（**scope_hash は先頭 8 文字 prefix のみ表示**、完全値は出さない）:

```
scope_type=source_ip_hash scope_prefix=0ba17413... action=report.submit window_start=2026-04-30T05:00:00Z window_secs=3600 count=18 limit=20 expires=2026-05-01T06:00:00Z
scope_type=source_ip_hash scope_prefix=0123abcd... action=publish.from_draft window_start=2026-04-30T05:00:00Z window_secs=3600 count=6 limit=5 expires=2026-05-01T06:00:00Z [OVER_LIMIT]
```

### 5.2 個別表示

```bash
$OPS_BIN usage show \
  --scope-type=source_ip_hash \
  --scope-prefix=0ba17413 \
  --action=report.submit
```

prefix 一致候補が **複数あれば曖昧として停止**し、より長い prefix を要求するメッセージが出る。
**`--scope-prefix` は 4 文字以上**を要求（広いマッチによる誤特定防止）。

出力（redact 済み）:

```
scope_type:           source_ip_hash
scope_hash_prefix:    0ba17413...
action:               report.submit
window_start:         2026-04-30T05:00:00Z
window_seconds:       3600
reset_at:             2026-04-30T06:00:00Z
count:                18
limit_at_creation:    20
over_limit:           no
created_at:           2026-04-30T05:00:01Z
updated_at:           2026-04-30T05:42:13Z
expires_at:           2026-05-01T06:00:00Z
```

### 5.3 出さない値（厳守）

- **scope_hash 完全値**（先頭 8 文字 prefix のみ）
- **IP 生値**（保存していない、salt+sha256 hash のみ）
- **session token / draft token / Cookie / DATABASE_URL / Secret**

## 6. 手動 cleanup SQL

MVP では `cmd/ops usage cleanup --execute` / Cloud Run Job 化を **実装しない**（PR36 計画書 §11）。retention は 24 時間 grace。

期限切れ行を削除する場合は cloud-sql-proxy 経由で直接実行:

```bash
DSN="postgres://..."
psql "$DSN" -c "DELETE FROM usage_counters WHERE expires_at < now() - interval '7 days'"
```

## 7. 未実装機能

| 機能 | 状態 | 後続対応 |
|---|---|---|
| `cmd/ops usage reset` | 未実装 | 後続 PR（必要になったタイミング）|
| `cmd/ops usage cleanup --dry-run` / `--execute` | 未実装 | 後続 PR |
| Cloud Run Job 化（自動 cleanup）| 未実装 | PR33e / PR41+ |
| Cloud Scheduler 連動 | 未実装、作らない方針継続 | 必要時に再判断 |
| `usage.abuse_detected` Outbox event | 未実装 | Phase 2 |
| Email 通知 | 未実装 | Email Provider 確定後 |
| Web admin dashboard | 未実装 | MVP 範囲外（v4 §6.19）|

## 8. false positive（誤ブロック）対応

NAT / IPv6 prefix / モバイル回線で複数ユーザーが同 IP hash を共有する場合、誤って 429 になる可能性がある。

### 確認手順

1. ユーザーから「公開・通報・アップロードができない」報告が来たら、**raw IP / token / Cookie を聞かない**こと
2. ユーザー側で **時間をおいて再試行**するよう案内（Retry-After 表示通り）
3. 運営側で `cmd/ops usage list --threshold-only` を実行し、**直近 1 時間で over_limit 行があるか**確認
4. 必要なら scope_prefix を絞って `cmd/ops usage show` で詳細確認
5. **対応**:
   - 真の abuse → 監視継続、必要なら moderation hide 検討
   - 誤ブロック → ユーザーに「時間をおいて再試行」を案内（直接 reset は MVP 未実装）

### scope_hash の取り扱い（必須）

- 運営内部メモにも **完全値を残さない**（prefix 8 文字までに redact）
- 外部（ユーザー / 第三者）に hash 値を提供しない
- 個人関連情報になり得るため `harness/work-logs/` / `chat` / `failure-log/` に出さない

## 9. fail-closed 方針

UsageLimit Repository（PostgreSQL `usage_counters`）への書き込み失敗時は、**安全側に倒して 429 で deny** する。

- 通常: `INSERT ... ON CONFLICT DO UPDATE` で atomic increment、戻り値 count > limit なら 429
- DB 障害: handler が 429（Retry-After 60 秒）で返す
- Cloud SQL 障害時に正常利用者も巻き込まれる可能性 → 業務影響が大きい場合は traffic を直前 revision に rollback して回避

将来 fail-open flag（`USAGE_LIMIT_FAIL_OPEN_ON_DB_ERROR=true`）を実装する余地は残してある（計画書 §17.3、本 PR では未実装）。

## 10. Cloud SQL write 増加への注意

- `usage_counters` テーブルへの write は public report submit / upload verification / publish のたびに 1〜2 件発生
- INDEX 最小化（PRIMARY KEY + 2 INDEX）+ ON CONFLICT DO UPDATE で write-amplification を抑制
- PostgreSQL の VACUUM / ANALYZE 圧迫が観測されたら **autovacuum 設定** or **partitioning / Redis 切替**を後続 PR で検討

## 11. 関連

- 計画書: [`docs/plan/m2-usage-limit-plan.md`](../plan/m2-usage-limit-plan.md)
- ルール: [`.agents/rules/turnstile-defensive-guard.md`](../../.agents/rules/turnstile-defensive-guard.md) / [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md)
- 横断: [`docs/design/cross-cutting/outbox.md`](../design/cross-cutting/outbox.md)
- runbook: [`docs/runbook/backend-deploy.md`](./backend-deploy.md) / [`docs/runbook/ops-moderation.md`](./ops-moderation.md)
- migration: `backend/migrations/00018_create_usage_counters.sql`

## 12. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-30 | 初版（PR36 commit 5）。MVP scope（report / upload_verification / publish の 3 endpoint × DB 単機 fixed window）の運用手順を集約 |
