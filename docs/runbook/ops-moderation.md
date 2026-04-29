# Ops Moderation runbook（PR34b）

> 設計: [`docs/plan/m2-moderation-ops-plan.md`](../plan/m2-moderation-ops-plan.md) /
> [`docs/design/aggregates/moderation/`](../design/aggregates/moderation/)
>
> Moderation MVP（hide / unhide / show / list-hidden）の **運営者向け実運用手順書**。
> 本書は手順書であり、計画書ではない。
>
> **採用方式**: ローカル CLI `cmd/ops` を Cloud SQL Auth Proxy 経由で実行する。
> Cloud Run Job 化 / Web admin UI / HTTP endpoint は **作らない**（業務知識 v4 §6.19、
> 計画書 §3.2、ADR-0002）。

---

## 0. 前提

- GCP project: `project-1c310480-335c-4365-8a8`
- Cloud SQL: `vrcpb-api-verify`（asia-northeast1）/ DB: `vrcpb` / user: `vrcpb_app`
- Cloud SQL Auth Proxy: `~/bin/cloud-sql-proxy` v2.13.0+ を使用
- 運営者（actor）: 単一運用前提（kento-matsunaga）/ `--actor` ラベルは個人情報を含まない
  運営内識別子（例: `ops-1`、`legal-team`）。VO 側で `^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}[a-zA-Z0-9]$`
  を強制
- 実行マシンは **`vrcpb-api-verify` への認可が通る GCP アカウント**でログイン済
  （`gcloud auth login` / `gcloud auth application-default login`）

---

## 1. 標準運用フロー

### 1.1 cloud-sql-proxy 起動

```bash
~/bin/cloud-sql-proxy --port=5433 \
  project-1c310480-335c-4365-8a8:asia-northeast1:vrcpb-api-verify &
sleep 3
ss -tln | grep 5433   # 期待: 127.0.0.1:5433 LISTEN
```

### 1.2 DATABASE_URL を一時 DSN として組み立て

```bash
# Secret Manager から取り出す。値は端末履歴 / コミット / log に出さない。
DATABASE_URL_VAL="$(gcloud secrets versions access latest \
  --secret=DATABASE_URL \
  --project=project-1c310480-335c-4365-8a8)"
PG_PASSWORD="$(echo "$DATABASE_URL_VAL" | sed -E 's#^postgres(ql)?://[^:]+:([^@]+)@.*$#\2#')"
DSN_LOCAL="postgres://vrcpb_app:${PG_PASSWORD}@127.0.0.1:5433/vrcpb?sslmode=disable"

# 一時ファイル経由で cmd/ops に渡す（chmod 600）
umask 077
printf '%s' "$DSN_LOCAL" > /tmp/dsn-prod.txt
chmod 600 /tmp/dsn-prod.txt
unset DATABASE_URL_VAL PG_PASSWORD DSN_LOCAL
```

### 1.3 cmd/ops の起動共通形

```bash
DATABASE_URL="$(cat /tmp/dsn-prod.txt)" \
  go -C /home/erenoa6621/dev/vrc_photobook/backend run ./cmd/ops <subcommand> [flags...]
```

> **注意**: DATABASE_URL は env 経由のみ。CLI 引数や stdout に値を出さない。

### 1.4 photobook show（参照のみ）

```bash
DATABASE_URL="$(cat /tmp/dsn-prod.txt)" \
  go -C /home/erenoa6621/dev/vrc_photobook/backend run ./cmd/ops photobook show \
  --id <PHOTOBOOK_UUID>
# または
DATABASE_URL="$(cat /tmp/dsn-prod.txt)" \
  go -C /home/erenoa6621/dev/vrc_photobook/backend run ./cmd/ops photobook show \
  --slug <PUBLIC_URL_SLUG>
```

出力に含まれる情報:
- `photobook_id` / `slug` / `title` / `creator_display_name`
- `type` / `visibility` / `status` / `hidden_by_operator` / `version`
- `published_at` / `created_at` / `updated_at`
- 直近 moderation_actions 概要 ≤ 5 件（kind / reason / actor_label / executed_at / action_id）

**含めない情報**: `draft_edit_token_hash` / `manage_url_token_hash` / `storage_key` 完全値 / DATABASE_URL / R2 credentials。

### 1.5 photobook list-hidden（hidden=true 一覧）

```bash
DATABASE_URL="$(cat /tmp/dsn-prod.txt)" \
  go -C /home/erenoa6621/dev/vrc_photobook/backend run ./cmd/ops photobook list-hidden \
  [--limit 20] [--offset 0]
```

`--limit` は 1〜200 にクランプ（既定 20）。

### 1.6 photobook hide（運営による一時非表示）

```bash
# 1) dry-run（DB 更新なし、planned summary を確認）
DATABASE_URL="$(cat /tmp/dsn-prod.txt)" \
  go -C /home/erenoa6621/dev/vrc_photobook/backend run ./cmd/ops photobook hide \
  --id <PHOTOBOOK_UUID> \
  --reason <REASON> \
  --actor <ACTOR_LABEL> \
  --detail "<内部参照用メモ、個人情報は書かない>"

# 2) 実行（--execute + 確認プロンプト yes）
DATABASE_URL="$(cat /tmp/dsn-prod.txt)" \
  go -C /home/erenoa6621/dev/vrc_photobook/backend run ./cmd/ops photobook hide \
  --id <PHOTOBOOK_UUID> \
  --reason <REASON> \
  --actor <ACTOR_LABEL> \
  --detail "<...>" \
  --execute
# プロンプト: Type 'yes' to proceed
# 入力: yes
```

#### `--reason` の MVP 運用許容セット（DB CHECK は v4 設計通り 9 種）

| reason | 用途 |
|---|---|
| `policy_violation_other` | その他規約違反（最も汎用、迷ったらこれ）|
| `report_based_harassment` | 通報経由: 嫌がらせ・晒し（PR35 接続前は通報を伴わなくても受け付け）|
| `report_based_unauthorized_repost` | 通報経由: 無断転載 |
| `report_based_sensitive_violation` | 通報経由: センシティブ違反 |
| `report_based_minor_related` | 未成年関連（v4 §7.4 最優先対応） |
| `rights_claim` | 権利侵害申立て（v4 §7.3、Report 非経由でも可）|
| `erroneous_action_correction` | unhide で誤 hide を戻すときに使う |

PR34b 範囲外（CHECK は受け入れるが UseCase 未実装、使わない）:
`report_based_subject_removal` / `creator_request_manage_url_reissue`

#### 制約

- 対象 photobook は **status='published' のみ**。draft / deleted / purged は拒否
  （`ErrInvalidStatusForHide`）
- 既に `hidden_by_operator=true` の場合は冪等で no-op（exit 0）
- `version` は上げない（編集 OCC を壊さない、計画書 §5.6）

#### 同 TX 4 要素（v4 P0-19）

`hide --execute` は単一 TX で次を実行:

1. `SELECT photobooks` 現状確認（`GetForOps`）
2. `UPDATE photobooks SET hidden_by_operator=true, updated_at=now() WHERE status='published' AND hidden_by_operator=false`
3. `INSERT moderation_actions (kind='hide', target_photobook_id, actor_label, reason, detail, executed_at)`
4. `INSERT outbox_events (event_type='photobook.hidden', payload, status='pending')`

いずれか失敗で全 rollback。outbox-worker 側 handler は **no-op + log のみ**（CDN purge / OGP cache invalidation は将来 PR、計画書 §7.4）。

#### Hide が公開導線に与える影響

| 経路 | 期待結果 |
|---|---|
| `https://api.vrc-photobook.com/api/public/photobooks/<SLUG>` | **HTTP 410** / `{"status":"gone"}` |
| `https://api.vrc-photobook.com/api/public/photobooks/<PID>/ogp` | `{"status":"not_public", "image_url_path":"/og/default.png"}` |
| `https://app.vrc-photobook.com/ogp/<PID>?v=1` | **HTTP 302** / Location: `/og/default.png` / `x-robots-tag: noindex, nofollow` |
| `https://app.vrc-photobook.com/p/<SLUG>` | HTTP 200 / gone テンプレ表示（`<title>VRC PhotoBook</title>` / 既定 OGP）|

R2 object は **削除しない**（unhide 時に復活させるため、計画書 §7.3 / ユーザー判断 #8）。

### 1.7 photobook unhide（誤 hide / 合意解消の戻し）

```bash
# dry-run + execute は hide と同じ手順
DATABASE_URL="$(cat /tmp/dsn-prod.txt)" \
  go -C /home/erenoa6621/dev/vrc_photobook/backend run ./cmd/ops photobook unhide \
  --id <PHOTOBOOK_UUID> \
  --reason erroneous_action_correction \
  --actor <ACTOR_LABEL> \
  --detail "<...>" \
  --correlation <PREVIOUS_HIDE_ACTION_ID>  # 任意、直前の hide action id
```

#### 制約

- 対象は status='published' AND hidden_by_operator=true
- 既に hidden=false なら冪等で no-op
- `--correlation` は任意（指定すると moderation_actions の correlation_id に記録、直前 hide とのペア追跡）

#### Unhide 後の挙動

公開 viewer / OGP / Workers / `/p/<SLUG>` HTML 全て通常状態に戻る（R2 object は流用、OGP 再生成は **不要**）。

### 1.8 cleanup

```bash
# proxy 停止
PIDS=$(pgrep -f "cloud-sql-proxy --port=5433")
[ -n "$PIDS" ] && kill $PIDS
sleep 1
ss -tln | grep 5433 || echo "(cleared)"

# 一時 DSN ファイル削除
rm -f /tmp/dsn-prod.txt

# 端末履歴クリア（DATABASE_URL 値は env 経由なので通常は履歴に残らないが念のため）
history -c
```

---

## 2. 実機 smoke 手順（リリース前 / 障害復旧後）

PR34b STOP δ で実施した手順を再現する。

| Step | 内容 |
|---|---|
| 1 | proxy 起動 + DSN 構築（§1.1, §1.2） |
| 2 | `cmd/ops photobook show --id <PID>` で現状確認 |
| 3 | `cmd/ops photobook hide --id <PID> --reason policy_violation_other --actor ops-smoke` 実行（dry-run → execute）|
| 4 | Backend `/api/public/photobooks/<SLUG>` → 410 / OGP `not_public` / Workers `/ogp/<PID>` → 302 fallback / `/p/<SLUG>` → gone |
| 5 | `cmd/ops photobook unhide --id <PID> --reason erroneous_action_correction --actor ops-smoke` 実行 |
| 6 | Backend `/api/public/photobooks/<SLUG>` → 200 / OGP `generated` / Workers `/ogp/<PID>` → 200 image/png / `/p/<SLUG>` → 通常 og:title |
| 7 | `cmd/ops photobook show --id <PID>` で moderation_actions に hide / unhide が記録されたことを確認 |
| 8 | （任意）outbox-worker Job を `gcloud run jobs execute vrcpb-outbox-worker --wait` で 1 件ずつ手動実行し、`photobook.hidden` / `photobook.unhidden` event を no-op consume |
| 9 | cleanup（§1.8） |

---

## 3. Rollback 方針

| 状況 | 手順 |
|---|---|
| `hide --execute` 後に対象を間違えた | 即 `cmd/ops photobook unhide --id <PID> --reason erroneous_action_correction --actor <YOU> --execute` で戻す。`--correlation` で直前 hide action id を指定 |
| `unhide --execute` 後に再 hide 必要 | 通常の `hide --execute` を再度実行 |
| `moderation_actions` の行を修正したい | **不可**（append-only、UPDATE / DELETE 禁止）。誤入力した detail / reason は新しい補正アクション（unhide / erroneous_action_correction 等）として追記する |
| Backend revision 自体を rollback | `docs/runbook/backend-deploy.md` §2 |

---

## 4. Secret 漏洩確認

```bash
# 直近 cmd/ops 実行の標準出力 / log に Secret が出ていないか目視
# DATABASE_URL=postgres / R2_SECRET_ACCESS_KEY= / TURNSTILE_SECRET_KEY= /
# Bearer xxx / sk_live xxx / raw_token xxx / manage_url_token xxx /
# storage_key 完全値 が含まれていないこと

# Cloud Run Job logs Secret 漏洩 grep（outbox-worker 経由 hide/unhide event 処理時）
gcloud logging read \
  'resource.type=cloud_run_job AND resource.labels.job_name=vrcpb-outbox-worker' \
  --project=project-1c310480-335c-4365-8a8 --limit=50 --format=json \
  | grep -iE "DATABASE_URL=postgres|R2_SECRET_ACCESS_KEY=|TURNSTILE_SECRET_KEY=|sk_live|raw_token|Bearer [A-Za-z0-9]{30,}|manage_url_token|storage_key" \
  || echo "no leak"
```

---

## 5. よくある失敗と対処

### 5.1 `DATABASE_URL not set`

cmd/ops は env から DATABASE_URL を読む。CLI 引数では受け付けない。

```
DATABASE_URL not set (export via env, do not pass on CLI)
```

対処: §1.2 / §1.3 の通り `DATABASE_URL="$(cat /tmp/dsn-prod.txt)"` を前置する。

### 5.2 `db connect failed: ...`

cloud-sql-proxy が起動していない / 別ポート使用中。

対処: §1.1 で `127.0.0.1:5433 LISTEN` を確認。proxy が落ちていれば再起動。

### 5.3 `photobook is not 'published'; hide requires status='published'`

draft / deleted / purged を hide しようとした。MVP では published のみ受け付ける（計画書 §13 #4）。

対処: 対象 status を `cmd/ops photobook show --id <PID>` で確認。published に遷移してから再実行。

### 5.4 `already hidden (no-op).` / `already not hidden (no-op).`

冪等動作。状態は変わっていない、追加の moderation_actions 行も増えない。これは **エラーではない**（exit 0）。

### 5.5 `invalid --actor`

`--actor` が正規表現 `^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}[a-zA-Z0-9]$` に合致しない。

対処: `ops-1` / `legal-team` / `admin_2` のような 3〜64 char、英数 + `. _ -` のみ、先頭末尾英数。**個人名・メールアドレス・実名は書かない**。

### 5.6 `--detail` が長すぎる（rune > 2000）

対処: 短く要約する。**個人情報を `detail` に書かない**運用ガイドを優先。

---

## 5.7 Report 連携（PR35b 追加、`hide --source-report-id` で監査チェーン形成）

### Report list / show（参照、redact 必須）

```bash
# 状態フィルタ付きで一覧（status=submitted / under_review / resolved_action_taken /
# resolved_no_action / dismissed のいずれか）
$OPS_BIN report list --status=submitted --limit=20

# 詳細表示。raw report_id / target_*_snapshot / reporter_contact / detail は
# work-log / chat に出さない方針（運用上必要なら redact 表示にする）
$OPS_BIN report show --id "<RID>"
```

`report show` の出力には以下が含まれる:

| 列 | 内容 | 表示 / 取り扱い |
|---|---|---|
| `report_id` | UUID v7 | redact `<redacted-uuid>` |
| `status` | submitted / under_review / resolved_* / dismissed | そのまま（カテゴリ） |
| `reason` | 6 種の VO 値 | そのまま（カテゴリ） |
| `submitted_at` | 受付時刻 UTC | そのまま |
| `target_photobook_id` / `target_slug_snapshot` / `target_title_snapshot` / `target_creator_snapshot` | 通報時刻時点の identifier / メタ | redact `<redacted>` |
| `reporter_contact` | 通報者の連絡先実値 | **work-log / chat / 公開記録に絶対に書かない**（DB 内のみ） |
| `detail` | 通報本文（複数行可） | **同上、絶対に書かない**（複数行 redact 必要、`sed` だけで安心しない） |
| `source_ip_hash_prefix4` | salt+sha256 の先頭 4 byte hex | そのまま（4 byte で同一 IP の重複検知のみ可、復元不可） |
| `resolved_at` / `resolved_by_moderation_action_id` | 監査チェーン用 | UUID は redact |

### Report → Hide の監査チェーン形成（同一 TX 5 要素）

通報を受けて運営が hide する場合、`hide --source-report-id` を渡すと **同一 TX で
5 要素**を更新する（PR34b の 4 要素 + reports 1 行）:

```bash
# dry-run（DB 更新なし、計画表示）
$OPS_BIN photobook hide \
  --id "<PID>" \
  --actor "ops-1" \
  --reason "policy_violation_other" \
  --detail "<運用判断の理由、機微情報を書かない>" \
  --source-report-id "<RID>"

# execute（confirm prompt が出る、--yes 不使用で手動 yes 推奨）
$OPS_BIN photobook hide \
  --id "<PID>" \
  --actor "ops-1" \
  --reason "policy_violation_other" \
  --detail "<運用判断の理由>" \
  --source-report-id "<RID>" \
  --execute
# Proceed to HIDE the photobook above?
# Type 'yes' to proceed: yes
# [ok] hidden. action_id=<UUID> photobook_id=<UUID> hidden_at=<UTC>
```

#### 同 TX 5 要素の内訳

1. `photobooks.hidden_by_operator = true`
2. `moderation_actions` に kind=hide / actor / reason / detail / `source_report_id=<RID>` で 1 行 INSERT
3. `reports.status = 'resolved_action_taken'`
4. `reports.resolved_by_moderation_action_id = <新 action_id>` / `reports.resolved_at = now`
5. `outbox_events` に `photobook.hidden` 1 行 INSERT（pending、worker が後から no-op processed）

#### 制約

- `source_report_id` で指定する report は **status=submitted または under_review** でないと拒否（PR35b で `ErrSourceReportTerminal`）
- photobook は **status=published かつ hidden_by_operator=false** が前提（PR34b と同条件）
- 実行後は `report show` で `resolved_at` / `resolved_by_moderation_action_id` の双方が入ったことを確認（運用上の audit）
- `--source-report-id` を付けない通常の `hide` は引き続き動作（自主判断 hide 経路）

#### よくある失敗

- **report が既に terminal**（resolved_action_taken / resolved_no_action / dismissed）→ `ErrSourceReportTerminal`、対象外。別 reason / 自主判断 hide で対応する
- **photobook が既に hidden=true** → `ErrAlreadyHidden`、まず unhide → hide --source-report-id
- **photobook が draft / private / deleted / purged** → `ErrInvalidStatusForHide`、対象外

### 5.7 outbox-worker Job が `--all-pending` で過剰 consume

PR34b 運用は `--once --max-events 1` 固定（Job spec で固定済）。`--all-pending` は使わない。

---

## 6. 後回し事項（PR35b 範囲外）

| 項目 | 対応 |
|---|---|
| `soft_delete` / `restore` / `purge` UseCase | PR34 拡張または別 PR（CHECK 制約は既に 6 種受け入れ済、UseCase だけ追加すれば動く）|
| `reissue_manage_url`（管理URL 再発行 + Session revoke + ManageUrlDelivery） | Email Provider 確定後（PR32c 以降）|
| Report 状態遷移系（`mark-reviewed` / `dismiss` / `resolve-without-action`） | PR36 以降 |
| 90 日後の reports.detail / reporter_contact / source_ip_hash NULL 化 reconciler | PR36 以降 |
| Report 通知 / Email | Email Provider 確定後 |
| upload-verification への L1-L4 多層 Turnstile ガード横展開 + TurnstileWidget L0 安定 mount セルフレビュー | PR36 以降の最優先（roadmap §1.3） |
| Web admin UI / dashboard | MVP 範囲外（v4 §6.19）|
| 作成者通知メール | Email Provider 未確定 |
| OGP 自動再生成 / R2 stale cleanup | PR33e（任意）|
| 複数運営者対応（OperatorId 化）| 別 PR、必要になったタイミング |

---

## 7. 関連ドキュメント

- 計画書: [`docs/plan/m2-moderation-ops-plan.md`](../plan/m2-moderation-ops-plan.md)
- ドメイン設計: [`docs/design/aggregates/moderation/ドメイン設計.md`](../design/aggregates/moderation/ドメイン設計.md) / [`データモデル設計.md`](../design/aggregates/moderation/データモデル設計.md)
- ADR: [`docs/adr/0002-ops-execution-model.md`](../adr/0002-ops-execution-model.md)
- Backend deploy runbook: [`docs/runbook/backend-deploy.md`](./backend-deploy.md)
- 業務知識: [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §5.4 / §6.19 / §7.3 / §7.4
- 横断: [`docs/design/cross-cutting/outbox.md`](../design/cross-cutting/outbox.md)

---

## 8. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-29 | 初版（PR34b）。MVP（hide / unhide / show / list-hidden）の運用手順を集約 |
| 2026-04-29 | PR35b 反映: §5.7 Report 連携（`report list` / `report show` redact ルール / `hide --source-report-id` 同 TX 5 要素 / よくある失敗）追記。§6 後回し事項を PR35b 範囲外項目で更新 |
