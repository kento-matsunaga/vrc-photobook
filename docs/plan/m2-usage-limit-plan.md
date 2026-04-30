# PR36 UsageLimit 集約 / RateLimit 設計計画書

## 0. このドキュメントの位置付け

PR35b（Report 受付 + 監査チェーン）と PR36-0（upload-verification への Turnstile 多層
ガード横展開）が完了したことを受け、**MVP 運用に必要な UsageLimit / RateLimit / abuse
防止の基盤**を設計する計画書。

本書は **PR36 の計画書のみ**を扱う。実装は行わない。本計画書 review 通過後、PR36 commit 1
（migration + domain 雛形）から実装に入る。

新正典ロードマップ参照: [`./vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md) §1 / §1.3 / §3 PR36

## 1. 現状整理（PR36 着手時点）

### 1.1 既に整っている前提

- **Bot 検証**: ADR-0005 に従い Turnstile が公開操作前 / upload-intent 前に必須。L0-L4
  多層ガード（`.agents/rules/turnstile-defensive-guard.md`）が Report / Upload 双方に適用
  済（PR35b / PR36-0）
- **Turnstile セッション化**: `upload_verification_sessions` で 30 分 / 20 回（業務知識 v4
  §3.7）。upload-intent はこの session を 1 回 consume する
- **Report 受付**: PR35b で `reports` テーブル + `source_ip_hash` 保存（salt+sha256、32 byte）。
  Salt は `REPORT_IP_HASH_SALT_V1` で管理（業務知識 v4 §3.7「IP ハッシュソルトは UsageLimit
  と Report で共有、version 管理」）
- **個人情報**: IP 生値は **保存しない**（hash のみ）。reporter_contact / detail / source_ip_hash
  完全値は logs / work-log / chat に出さない運用
- **Outbox / Worker**: 手動 Job 運用（Cloud Scheduler 未作成、PR33e で要否再判断）
- **Email Provider**: ADR-0006 で MVP 必須から外し済、未確定
- **monit / cmd/ops**: PR34b moderation + PR35b report で読み取り経路が整っている

### 1.2 未整備事項（PR36 で扱う候補）

- 同一作成元からの **作成レート上限**（業務知識 v4 §2.7 / §3.7「同一作成元で 1 時間 5 冊
  まで」）が **未実装**
- **通報 submit の過剰送信抑止**（PR35b 計画 §8）が **未実装**
- **upload verification / upload intent の過剰発行抑止**（Turnstile session の 20 回上限は
  ある、ただしそれを超える複数 session 取得への抑止が未実装）が **未実装**
- **publish 連打 / 無効操作の抑止**が **未実装**
- 運営が UsageLimit 状態を確認する経路（`cmd/ops`）が **未実装**
- IP hash の cleanup / TTL ポリシーが **未確定**

### 1.3 業務知識 v4 が確定している MVP 数値

業務知識 v4 §3.7 で以下が確定:

- **1 フォトブックあたり画像 20 枚まで**（既存実装あり、photobook_photos の論理制約）
- **1 画像 10MB まで**（既存実装あり、`MAX_UPLOAD_BYTE_SIZE` / Backend declared_byte_size
  検証）
- **同一作成元で 1 時間に 5 冊まで**（**未実装、本 PR の主目的**）

PR36 ではこの **5 冊 / 1 時間 / 同一作成元** を中核に、各 endpoint の rate-limit を
体系化する。具体的な閾値（report submit / upload verification / publish 等）は
本計画書の §18 ユーザー判断事項として設定確定する。

### 1.4 Cloud SQL / インフラの現状

- Cloud SQL は `vrcpb-api-verify`（asia-northeast1、検証用名のまま本番相当に使用継続）
- migration v17（00014 moderation_actions + 00015 outbox CHECK + 00016 reports + 00017
  outbox CHECK 拡張、PR34b/PR35b で適用）
- Cloud Run service `vrcpb-api` revision `vrcpb-api-00021-vl9` / image `vrcpb-api:540cd1f`
- Cloud Run Job `vrcpb-outbox-worker` image `vrcpb-api:540cd1f`、Cloud Scheduler 0 件
- Workers Frontend version `ce64f95a-d4ce-405b-821a-f71c22a992db`
- Secret Manager: 8 件（`REPORT_IP_HASH_SALT_V1` 含む）

---

## 2. PR36 のゴール（MVP）

### G1. 同一作成元の作成レート上限を実装

業務知識 v4 §3.7 確定の **「同一作成元で 1 時間に 5 冊まで」** を `publishFromDraft` 経路に
適用する。同一作成元の判定は **draft session + source_ip_hash の複合キー**（仮、§4 で確定）。

### G2. 通報 submit の過剰送信抑止

同一 source_ip_hash + target_photobook_id（あるいは action 単位）で短時間の連投を抑止する。
具体閾値は §18 で確定（候補: 同一 photobook 宛 5 分に 3 件 / 同一 source_ip_hash 全体で 1 時間に
20 件）。

### G3. upload-verification の過剰発行抑止

同一 draft session で短時間に複数の verification session を取得することを抑止する
（既存の Turnstile session 30 分 / 20 回 は **個別 session 内** の制約のため、複数 session
発行による迂回を抑止）。具体閾値は §18 で確定。

### G4. **MVP は軽量 RDB 実装**で完結させる

Redis / 専用 RateLimit サーバーは導入しない。Cloud SQL の単一テーブルで RateLimit
バケット相当を管理し、PostgreSQL の `INSERT ... ON CONFLICT DO UPDATE` または
serializable TX で concurrent increment を解決する。

### G5. **false positive を最小化**

通常利用者が誤って block されないこと。とくに同一 NAT / モバイル回線下のユーザー間で
hash が衝突することは稀だが、閾値は通常利用を阻害しない値に設定。閾値超過時の UI 文言
で「時間をおいて再度」と案内し、リカバリ可能にする。

### G6. 運営が `cmd/ops` で usage 状態を確認できる（list / show のみ）

MVP では **読み取りのみ**（list / show）。reset / cleanup は手動 SQL or 後続 PR で。

### G7. **個人情報を増やさない**

IP 生値は保存しない（既存方針継続）。新規追加するのは「UsageLimit が使う scope hash」
のみ。**hash salt は既存 `REPORT_IP_HASH_SALT_V1` を流用**するか、別 salt を新規導入
するかは §6 で検討する。

### G8. **失敗時も安全に deny できる**

DB 書き込み失敗 / Cloud SQL 障害時は、**deny（fail-closed）** を選択することで abuse 抑止
を維持。ただし正常利用者への影響を避けるため、特定の致命エラーのみ fail-open に倒す
余地は §17 リスクで触れる。

### G9. Frontend / Backend 双方で **429 Too Many Requests** を統一表示

L4 ガードの 400 / 403 とは別に、UsageLimit 起因の拒否は **HTTP 429** で返す。Backend
handler / UseCase / Frontend client の各層で扱いを揃える。

---

## 3. PR36 で扱うこと / 扱わないこと

### 3.1 扱うこと（MVP）

| 項目 | 内容 |
|---|---|
| `usage_counters` テーブル新設 | scope_hash / action / window 単位のカウンター |
| `internal/usagelimit/` 集約 | domain entity + VO + UseCase + Repository |
| `CheckAndConsumeUsage` UseCase | 既存 UseCase の前段で呼び、429 で deny |
| publish / report submit / upload-verifications endpoint への組み込み | §3 の対象 endpoint に適用 |
| 429 response（Retry-After header / JSON body）| Backend |
| Frontend error mapping + UI 文言 | ReportForm / Upload UI / publish flow |
| `cmd/ops usage list / show`（読み取り）| MVP |
| 期限切れ counter の手動 cleanup SQL（runbook 記載） | MVP |
| migration（`usage_counters`）| 1 本（00018 想定） |
| Backend test（unit + handler 統合）+ Frontend test（vitest 既存パターン）| 各層 |
| runbook 追記（`docs/runbook/usage-limit.md` 新設候補 or `ops-moderation.md` §拡張）| MVP |
| failure-log 起票準備（実装中に問題発生時）| MVP |

### 3.2 扱わないこと（PR36 範囲外、対応 PR を明示）

| 項目 | 対応 |
|---|---|
| Redis / 専用 RateLimit サーバー導入 | 不要（Cloud SQL で完結）/ 必要になったら別 PR |
| Cloud Armor / 本格 WAF | PR40 ローンチ前安全性強化 / 別 PR |
| `cmd/ops usage reset` / `cleanup --execute` | PR36 拡張 or 後続 PR（MVP は list / show のみ）|
| Cloud Scheduler 化（自動 cleanup）| PR33e / PR41+ |
| Web admin dashboard | MVP 範囲外（v4 §6.19）|
| 有料プラン / Stripe 連動 | Phase 2 |
| AI moderation / spam 自動検知 | Phase 2 |
| Email / Slack 通知 | Email Provider 確定後 |
| Public repo 化 | PR38 |
| spike 削除 | PR40 |
| OGP endpoint への rate-limit | 基本対象外（MVP）。高負荷時は §17 で再検討 |
| `/p/[slug]/report` の GET（フォーム表示）への rate-limit | 基本対象外（POST submit のみ抑止）|

---

## 4. 制限対象 endpoint / action

### 4.1 一覧（候補）

| 対象 | endpoint | 操作主体 | scope 候補 |
|---|---|---|---|
| **Public Report submit** | POST `/api/public/photobooks/{slug}/reports` | anonymous | source_ip_hash + target_photobook_id |
| **Upload verification 発行** | POST `/api/photobooks/{id}/upload-verifications` | draft session | session_id + photobook_id |
| **Upload intent 発行** | POST `/api/photobooks/{id}/images/upload-intent` | draft session | session_id + photobook_id |
| **Image complete 通知** | POST `/api/photobooks/{id}/images/{imageId}/complete` | draft session | session_id + photobook_id（補助） |
| **Publish from draft** | POST `/api/photobooks/{id}/publish` | draft session | source_ip_hash + 1 時間（業務知識 5 冊上限）|
| **Edit mutation 系**（caption / reorder / cover / settings）| PATCH 系 | draft session | session_id + photobook_id（緩めの quota）|
| **Manage view 取得** | GET `/api/manage/photobooks/{id}` | manage session | session_id + photobook_id（緩めの quota、または対象外）|
| **Manage URL 再発行**（未実装） | - | - | 将来対象（Email Provider 確定後）|
| **OGP endpoint** | GET `/api/public/photobooks/{photobookId}/ogp` | anonymous | **基本対象外**（高頻度アクセスでも安全、§17 で再検討） |
| **`/p/[slug]/report` GET**（フォーム表示）| Frontend SSR | anonymous | **対象外**（フォーム表示自体は副作用なし）|
| **`/p/[slug]` GET**（公開 Viewer）| Frontend SSR + Backend lookup | anonymous | **対象外**（基本的に高頻度を想定すべき経路）|
| **`cmd/ops`** | CLI | operator | **対象外**（社内ツール）|

### 4.2 推奨（MVP 初期対象）

**G1〜G3 を満たす最小セット**として、以下に絞る:

1. **POST /api/public/photobooks/{slug}/reports**（Report submit、最重要）
2. **POST /api/photobooks/{id}/upload-verifications**（upload verification 発行、抑止対象）
3. **POST /api/photobooks/{id}/publish**（publish、業務知識 5 冊上限）

**初期非対象**（後続 PR / 運用フェーズで拡張可能とする）:

- upload-intent / image complete: Turnstile session 30 分 / 20 回上限で既に抑止されている
  ため、本 PR では追加しない（多重抑止の効果対コストで MVP では非搭載）
- Edit mutation 系: draft session 内の操作で abuse 動機が低い、本 PR では非搭載
- Manage view 取得: 既存 `RequireManageSession` middleware で制限済、追加 quota は不要

---

## 5. RateLimit / UsageLimit の粒度

### 5.1 案比較

| 案 | 粒度 | メリット | デメリット |
|---|---|---|---|
| **A: source_ip_hash + action + time window** | IP hash 単位 | 同一作成元の総量を抑制可能 / 業務知識 v4 §3.7 と整合 | NAT 配下で衝突する可能性、false positive リスク |
| **B: session id + action + time window** | session 単位（draft / manage） | session 取得済みのため衝突なし、UX に優しい | session を新規取得すれば迂回可能（draft session は token 入力が必要なので攻撃者にはハードル）|
| **C: photobook id + action + time window** | photobook 単位 | 同一 photobook への spam 抑止に強い | 別 photobook には効かない |
| **D: 複合キー（action ごとに分ける）** | action ごとに最適粒度 | false positive 最小化 + 用途特化 | 設計複雑化 |

### 5.2 推奨（案 D 派生：action ごとの複合キー）

| action | scope 構成 | 主目的 |
|---|---|---|
| `report.submit` | `source_ip_hash` + `target_photobook_id` AND `source_ip_hash`（全体） | 同一 IP からの同一対象連投 + 全体ボム抑止 |
| `upload_verification.issue` | `draft_session_id` + `photobook_id` | session 内の連投抑止（30 分 / 20 回上限とは別軸の上限）|

> **2026-04-30 commit 3.5 補足（実装上の scope_type 表現）**:
> 2 軸の複合 scope は **scope_type='source_ip_hash'**（report.submit）/ **scope_type='draft_session_id'**（upload_verification.issue）に統一し、scope_hash に `sha256(主軸 || ":" || 副軸)` の合成 hex を入れる方式を採用した（migration v18 の CHECK 制約 4 種を維持しつつ複合表現を実現）。
> 例:
>   - `report.submit` の 5 分 3 件 → `scope_type='source_ip_hash'` / `scope_hash=sha256(ip_hash || ":" || photobook_id)`（主軸: ip）
>   - `upload_verification.issue` の 1 時間 20 件 → `scope_type='draft_session_id'` / `scope_hash=sha256(session_id || ":" || photobook_id)`（主軸: session）
> scope_type は「主観点の集計軸」として一貫した意味（同 IP 軸 / 同 session 軸）を持ち、scope_hash の hex 表現は単軸 / 複合の双方を含む。**「scope_type='photobook_id' を photobook 単体軸として誤読しない」**運用ガイドを runbook に明記する。
| `publish.from_draft` | `source_ip_hash`（全体）| 業務知識 v4 §3.7 「同一作成元 1 時間に 5 冊」 |

**理由**:

- Report submit は anonymous → IP hash が唯一の同一元判定軸
- Upload verification は draft session 経由 → session_id で十分
- Publish は draft session でも可だが、業務知識が「同一作成元」基準のため IP hash 採用
- 複合キーで false positive を最小化し、迂回コストも上がる

### 5.3 hash 衝突 / NAT 問題への対策

- 閾値は通常利用を阻害しない値に設定（§18 で確定）
- 429 deny 時の UI に「時間をおいて再度」を明記し、ユーザー側でリカバリ可能にする
- `cmd/ops usage show --scope <hash-prefix>` で運営が個別調査できるようにする
- raw IP / hash 完全値を画面に出さない（runbook §security で固定）

---

## 6. DB 設計

### 6.1 採用方針

**単一テーブル `usage_counters`** で固定窓 (fixed window) RateLimit バケットを表現する。
Redis 等の専用ストアは導入しない。

- スキーマは PostgreSQL（既存 `vrcpb-api-verify`）
- 既存集約と同 schema（`public`）
- migration v18（既存 v17 → v18、`backend/internal/database/migrations/00018_usage_counters.sql`
  想定）

### 6.2 migration 案（`usage_counters`）

```sql
-- name: usage_counters
-- 用途: 同一 scope（IP hash / session / photobook）+ action + 固定窓の利用回数を集計し、
-- 閾値超過時に 429 で拒否するための RateLimit バケット。
--
-- 設計:
--   - scope_type / scope_hash で集計対象を一意化（hex 文字列、salt+sha256 由来 32 byte）
--   - window_start は fixed window 開始時刻（時間単位の切り捨てを推奨、UseCase 側で算出）
--   - window_seconds は窓の長さ（300, 3600, ... 等の秒数）
--   - count はそのバケット内の出現回数（atomic UPDATE で increment）
--   - limit_at_creation は INSERT 時点の閾値スナップショット（後の閾値変更で過去履歴を
--     誤読しないよう保持。UseCase は実行時の current limit を別途参照して deny 判定）
--   - expires_at = window_start + window_seconds + retention（cleanup 用、retention は §11）
--   - PRIMARY KEY (scope_type, scope_hash, action, window_start) で同窓内の単一行
--   - INSERT ... ON CONFLICT DO UPDATE で increment（race-free）
CREATE TABLE usage_counters (
    scope_type        TEXT        NOT NULL,
    scope_hash        TEXT        NOT NULL, -- hex 64 文字（sha256）想定
    action            TEXT        NOT NULL,
    window_start      TIMESTAMPTZ NOT NULL,
    window_seconds    INT         NOT NULL CHECK (window_seconds > 0),
    count             INT         NOT NULL DEFAULT 0 CHECK (count >= 0),
    limit_at_creation INT         NOT NULL CHECK (limit_at_creation > 0),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at        TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (scope_type, scope_hash, action, window_start),
    CONSTRAINT usage_counters_scope_type_check
      CHECK (scope_type IN ('source_ip_hash', 'draft_session_id', 'manage_session_id', 'photobook_id')),
    CONSTRAINT usage_counters_action_check
      CHECK (action IN ('report.submit', 'upload_verification.issue', 'publish.from_draft'))
);

CREATE INDEX usage_counters_expires_at_idx ON usage_counters (expires_at);
CREATE INDEX usage_counters_action_window_start_idx ON usage_counters (action, window_start DESC);
```

### 6.3 INSERT ... ON CONFLICT DO UPDATE（atomic increment）

```sql
INSERT INTO usage_counters
  (scope_type, scope_hash, action, window_start, window_seconds, count, limit_at_creation, expires_at)
VALUES
  ($1, $2, $3, $4, $5, 1, $6, $4 + ($5 || ' seconds')::interval + ($7 || ' seconds')::interval)
ON CONFLICT (scope_type, scope_hash, action, window_start)
DO UPDATE
   SET count = usage_counters.count + 1,
       updated_at = now()
RETURNING count, limit_at_creation;
```

判定:

- `RETURNING count, limit_at_creation` で increment 後の値を取得
- UseCase 側で `count > limit_at_creation` なら deny（429）
- ON CONFLICT DO UPDATE で race-free に increment（PostgreSQL の serializable 不要）

### 6.4 scope_hash の作り方

- `source_ip_hash`: 既存 `internal/report/internal/usecase.HashSourceIP` と **同 salt** で算出
  （業務知識 v4 §3.7「UsageLimit と Report で共有」）。32 byte（hex 64 文字）
- `draft_session_id`: 既存 `sessions.id` を hex 化（UUID v7 → hex）。salt を**かけない**
  （session_id 自体が機密でない、salt 不要）
- `manage_session_id`: 同上
- `photobook_id`: UUID v7 を hex 化、salt 不要

### 6.5 unique 制約 / index

- PRIMARY KEY: `(scope_type, scope_hash, action, window_start)` → 同窓内 1 行を保証
- INDEX: `expires_at`（cleanup 用）/ `action, window_start DESC`（運営調査用）
- 別 INDEX を増やさない（write-heavy になる前に必要性を見て後続で追加）

### 6.6 retention / TTL

- `expires_at = window_start + window_seconds + RETENTION_GRACE`
- RETENTION_GRACE は **24 時間**（運営調査用に少し残す、cleanup ジョブで delete）
- DB は自動で削除しない。**手動 cmd/ops cleanup or 直接 SQL delete** で運用（§11）

### 6.7 推奨

**§6.2 の `usage_counters` スキーマ** を採用。

- PostgreSQL 単機で完結
- INSERT ... ON CONFLICT DO UPDATE で race-free
- expires_at で cleanup 容易
- scope_type / action は CHECK 制約で限定（誤代入防止）

---

## 7. Salt / Secret 方針

### 7.1 選択肢比較

| 案 | salt | メリット | デメリット |
|---|---|---|---|
| **A: `REPORT_IP_HASH_SALT_V1` を流用** | 既存 1 本 | 業務知識 v4「UsageLimit と Report で共有」と整合、Secret 追加不要 | Report と UsageLimit の hash 出力が同一 → cross-reference で可能性が広がる（業務知識上はこれが意図、相関検出のため）|
| **B: `USAGE_LIMIT_HASH_SALT_V1` を新規作成** | UsageLimit 用 1 本 | Report と分離、独立ローテーション可能 | 業務知識 v4「UsageLimit と Report で共有」と矛盾、新 Secret 追加 STOP 必要 |
| **C: IP hash は Report のみ、UsageLimit は session/photobook のみ** | salt 不要 | Secret 追加不要 / 個人情報増えない | publish の「同一作成元 1 時間 5 冊」が実装できない（IP 軸が必要）|

### 7.2 推奨

**案 A（`REPORT_IP_HASH_SALT_V1` を流用）** を強く推奨。

- 業務知識 v4 §3.7 の確定方針「UsageLimit と Report で共有」と整合
- 同一作成元の **大量投稿 + 大量通報の相関検出** が可能（業務知識のルール）
- Secret 追加不要、deploy STOP も不要
- ローテーション時は Report と UsageLimit が **同タイミングで長期追跡性を失う** ことを許容
  （業務知識 v4 §3.7「ローテーション時は長期追跡性が失われることを許容する」と整合）

**ローテーション時のポリシー**: 将来 V2 salt が必要になった場合は `_V2` を新設し、Report
側も同 V2 を使う運用にする。本 PR36 では V1 を共有。

### 7.3 注意事項

- Cloud Build SA に secretAccessor を **付けない**（runtime SA のみ、PR35b STOP β で確立済方針継続）
- Secret 値は logs / chat / work-log に**出さない**（PR35b 同方針継続）

---

## 8. Domain / UseCase 設計

### 8.1 Domain（`backend/internal/usagelimit/`）

- `domain/entity/usage_counter.go`: UsageCounter エンティティ（scope_type / scope_hash /
  action / window_start / window_seconds / count / limit_at_creation / expires_at）
- `domain/vo/scope_type/`: VO「source_ip_hash / draft_session_id / manage_session_id /
  photobook_id」（v4 業務知識整合の enum）
- `domain/vo/action/`: VO「report.submit / upload_verification.issue / publish.from_draft」
- `domain/vo/scope_hash/`: VO（hex 64 文字 / 不変）
- `domain/vo/window/`: VO（fixed window の seconds + start 算出ロジック）

### 8.2 UseCase（一覧、PR36 MVP）

| UseCase | 用途 | 入出力 |
|---|---|---|
| `CheckAndConsumeUsage` | 既存 UseCase の前段で呼ぶ。INSERT ON CONFLICT で increment し、count > limit なら ErrRateLimited を返す | Input: scope_type / scope_hash / action / now / config（limit / window / retention）, Output: 残り回数 / Retry-After |
| `GetUsageForOps` | cmd/ops usage show 用、scope + action で現窓の count + limit + reset_at を返す | Input: scope_type / scope_hash_prefix / action（任意）, Output: 一覧 |
| `ListUsageForOps` | cmd/ops usage list 用、action / status（threshold 超過のみ等）でフィルタ | Input: filter, Output: redacted view |

### 8.3 UseCase 配置（前段 vs middleware）

**前段呼び出し方式を推奨**（middleware ではなく）:

- 既存 UseCase（SubmitReport / IssueUploadVerificationSession / PublishFromDraft）の Execute
  冒頭で `CheckAndConsumeUsage.Execute` を呼ぶ
- 理由:
  - middleware 化すると domain 文脈が薄くなり、ScopeHash 計算（IP 取得 / session 取得）が
    middleware 共通層と重複しがち
  - UseCase 単位で「どの action にどの scope を適用するか」を明示できる
  - test 容易性（既存 UseCase の test に `CheckAndConsumeUsage` の fake を差し込める）

### 8.4 エラー設計

```go
// internal/usagelimit/internal/usecase/errors.go
var (
    // ErrRateLimited は閾値超過。HTTP 429 にマップ。
    ErrRateLimited = errors.New("usagelimit: rate limited")

    // ErrUsageRepositoryFailed は DB 障害等。fail-closed なら 429 にマップ、
    // fail-open なら呼び出し側が握りつぶす（運用判断、§17 リスク）。
    ErrUsageRepositoryFailed = errors.New("usagelimit: repository failed")
)
```

各 endpoint UseCase 側で `CheckAndConsumeUsage` 戻り値を errors.Is チェックして 429 deny に
変換。**fail-closed** を MVP の既定とする。

### 8.5 SubmitReport との連動例

```go
// internal/report/internal/usecase/submit_report.go (concept)
func (u *SubmitReport) Execute(ctx, in) (...) {
    // L4 trim ガード（PR35b 既存）
    if strings.TrimSpace(in.TurnstileToken) == "" { ... }

    // PR36 新規: UsageLimit
    if err := u.usageLimit.CheckAndConsume(ctx, usagelimit.CheckInput{
        ScopeType:    "source_ip_hash",
        ScopeHash:    HashSourceIP(SaltVersionV1, u.ipHashSalt, in.RemoteIP),
        Action:       "report.submit",
        Now:          in.Now,
        WindowSeconds: 3600,
        Limit:         20, // §18 で確定
    }); err != nil {
        if errors.Is(err, usagelimit.ErrRateLimited) {
            return SubmitReportOutput{}, ErrRateLimited // → 429
        }
        return SubmitReportOutput{}, err
    }

    // 既存: Turnstile siteverify → reports INSERT + outbox INSERT
    ...
}
```

---

## 9. API / Error 設計

### 9.1 HTTP status / body

- **HTTP 429 Too Many Requests** を採用
- `Retry-After: <seconds>` header（window 残り時間）
- body: `{"status":"rate_limited","retry_after_seconds":<int>}`
- header: `X-Robots-Tag: noindex, nofollow` / `Cache-Control: private, no-store`
- **失敗詳細を漏らさない**: body に scope_hash / count / limit を出さない（敵対者に閾値を
  推測させない、運営対応は cmd/ops 経由）

### 9.2 endpoint 別

| endpoint | 既存 status | 追加 status | 備考 |
|---|---|---|---|
| POST `/api/public/photobooks/{slug}/reports` | 201 / 400 / 403 / 404 / 500 | **429 rate_limited** | Frontend は kind=`rate_limited` に mapping |
| POST `/api/photobooks/{id}/upload-verifications` | 200 / 400 / 401 / 403 / 503 | **429 rate_limited** | Frontend は kind=`rate_limited` に mapping |
| POST `/api/photobooks/{id}/publish` | 200 / 400 / 401 / 409 | **429 rate_limited** | Frontend は kind=`rate_limited` に mapping |

### 9.3 Frontend error mapping（`frontend/lib/*.ts`）

```ts
// lib/report.ts (concept)
export type SubmitReportError =
  | { kind: "invalid_payload" }
  | { kind: "turnstile_failed" }
  | { kind: "not_found" }
  | { kind: "rate_limited"; retryAfterSeconds: number } // ★ 新規
  | { kind: "server_error" }
  | { kind: "network" };
```

429 を kind=`rate_limited` に map、Retry-After header を取り出して UI 文言に渡す。

---

## 10. Frontend UI 設計

### 10.1 文言例

| 経路 | 文言 |
|---|---|
| ReportForm 429 | 「短時間に通報を送信しすぎました。{retryAfterMin} 分ほど時間をおいて再度お試しください。」|
| Upload UI 429 | 「短時間にアップロード操作を繰り返しています。{retryAfterMin} 分ほど時間をおいて再度お試しください。」|
| Publish 429 | 「公開操作の上限に達しました。1 時間あたりの公開数には上限があります。時間をおいて再度お試しください。」|

### 10.2 UX

- Turnstile 未完了による 403 とは **別 UI 状態**として表示する
- Retry-After が取得できれば残り時間（分単位）を表示、取得できなければ「時間をおいて」のみ
- iPhone Safari でも文言折り返しが破綻しないよう短文化
- ボタンは disable せず、再 submit 可能にする（ユーザーが時間をおいて再試行できるよう）

### 10.3 Safari 確認（§14 で詳細）

- ReportForm の rate_limited 表示
- Upload UI の rate_limited 表示
- iPhone Safari でレイアウト破綻なし
- Turnstile 状態と混同しない

---

## 11. Cleanup / Retention

### 11.1 retention ポリシー

- `expires_at = window_start + window_seconds + RETENTION_GRACE`
- RETENTION_GRACE = **24 時間**（運営調査用に少し残す）
- expires_at 経過行は **手動 cmd/ops cleanup or 直接 SQL delete** で削除

### 11.2 MVP 運用

- **Cloud Run Job / Cloud Scheduler は作らない**（PR36 範囲外）
- runbook に手動 cleanup SQL 例を載せる:
  ```sql
  DELETE FROM usage_counters WHERE expires_at < now() - interval '7 days';
  ```
- cmd/ops `usage list` で残件を表示し、運営が判断して手動 SQL を打つ

### 11.3 後続 PR で扱う

- `cmd/ops usage cleanup --dry-run / --execute`（後続 PR）
- Cloud Run Job 化（PR33e 系で検討、Cloud Scheduler 可否含む）
- 90 日 NULL 化 reconciler（reports.detail / reporter_contact / source_ip_hash 用、別タスク）
  との関係: UsageLimit は 24h grace で削除、reports は 90 日 NULL 化 → **別経路で別 retention**

---

## 12. Outbox

### 12.1 必要性検討

**結論: PR36 MVP では Outbox event は不要**。

理由:

- UsageLimit deny は **同期 response（429）** で完結、外部副作用なし
- 「abuse 検知 → Email 通知」「abuse 検知 → 別系統 alert」は MVP 範囲外
- 将来「rate_limited が一定回数超えた scope を運営に通知」等が必要になれば、その時に
  `usage.abuse_detected` のような event を追加する
- 本 PR で Outbox 配線を増やすと、無使用の event handler を抱え続けることになる

### 12.2 後続候補

- `usage.rate_limited`（個別 deny を logging する用、log で十分なので Outbox 不要かも）
- `usage.abuse_detected`（threshold 超過の集計、Email / Slack 通知用、Phase 2）

---

## 13. Security / Privacy

### 13.1 必須事項

- **IP 生値は保存しない**（PR35b 既存方針継続）
- **scope_hash 完全値を logs / chat / work-log に出さない**（先頭 8 文字 prefix のみ表示）
- **hash salt は Secret Manager 管理、runtime SA のみ secretAccessor**（PR35b 既存方針）
- `cmd/ops` 出力は `<redacted-uuid>` / `<hash-prefix-8>` 形式
- `usage_counters` row には raw IP / raw session_token / raw photobook_id（UUID 自体は機密性低いが、scope_hash は hex 化して保存）を含めない

### 13.2 hash の個人情報性

- hash は **個人関連情報になり得る**（裁判例 / GDPR / 改正個人情報保護法の論点）
- 本サービスは Cloud SQL（asia-northeast1）保存、retention は 24h grace
- runbook で「hash 値の取り扱い」を明記し、運営が外部に出さないルールを固定

### 13.3 token / Cookie / manage URL / storage_key を保存しない

- UsageLimit が扱うのは scope_hash / action のみ
- 既存方針継続（誤って raw token を usage_counters に書き込まない設計）

### 13.4 DoS 時の DB 負荷

- 大量 increment が Cloud SQL に集中する可能性
- 対策: PRIMARY KEY での single-row UPSERT + INDEX 最小化で write-amplification を抑制
- 万一 Cloud SQL が degraded した場合の **fail-closed**（429 deny）を既定とする（§G8 / §17）

### 13.5 bot 対策と rate-limit の違い

- **bot 対策（Turnstile）**: 「人間かどうか」を判定（L0-L4 多層ガード）
- **rate-limit（UsageLimit）**: 「人間でも回数が多すぎる」を抑制
- 両者は**直列に配置**し、Turnstile 失敗（403）/ rate-limit 失敗（429）/ 通常成功 を区別する

### 13.6 CSRF

- 既存方針（POST のみ受理 / Origin 検証 / Cookie SameSite=Strict）を継続
- UsageLimit は CSRF 対策の代替ではない（攻撃者が同一 IP / session で連投する場合のみ抑止）

---

## 14. Safari 確認

### 14.1 必須項目

- ReportForm の **rate_limited 表示**（429 を受けた時の UI）
- Upload UI の **rate_limited 表示**（同上）
- Publish 操作 429 → CompleteView 系の error state で表示（既存 publish flow に組み込み）
- **iPhone Safari** でレイアウト破綻なし、Retry-After 表示が読める
- **Turnstile 状態（L0-L4）と混同しない** UI 文言

### 14.2 macOS Safari

- 同様の確認、ただし iPhone Safari が主要動線（業務知識 v4 §1.2 / §6.2）

### 14.3 切り分け順（NG 時）

- iPhone Safari Wi-Fi → モバイル回線 → プライベートブラウズ OFF → コンテンツブロッカー OFF
  → macOS Safari（Cloudflare 公式トラブルシュート準拠）

---

## 15. Tests

### 15.1 Backend

- `usage_counters` Repository test（実 DB、`go test -count=1`）:
  - 初回 INSERT で count=1
  - 同 scope+action+window で再 INSERT → count=2（ON CONFLICT DO UPDATE）
  - 別 scope / 別 window では別行
  - expires_at が `window_start + window_seconds + grace` で計算される
- `CheckAndConsumeUsage` UseCase test（unit + 実 DB）:
  - threshold 以内で `count <= limit` → 成功
  - threshold 超過で `ErrRateLimited`
  - window reset で count が 1 から再開
  - Repository 失敗時の挙動（fail-closed → 429）
- handler 統合 test:
  - SubmitReport 429 → `Retry-After` header / body `rate_limited`
  - IssueUploadVerification 429 → 同上
  - PublishFromDraft 429 → 同上
- concurrency test（serializable 不要、ON CONFLICT で race-free）:
  - goroutine 並列 increment で最終 count が一致

### 15.2 Frontend

- `lib/report.ts` / `lib/upload.ts` で 429 → kind=`rate_limited` mapping
- `lib/publishPhotobook.ts` で同上
- ReportForm / Upload UI / Publish UI で rate_limited 文言レンダリング（renderToStaticMarkup 経由）

### 15.3 cmd/ops

- `usage list --action=report.submit --threshold-only` で deny 履歴が一覧表示
- `usage show --scope-type=source_ip_hash --scope-prefix=<8文字> --action=report.submit` で
  個別 scope の現窓 count / limit / reset_at を redact 表示

---

## 16. 実リソース操作

### 16.1 計画書フェーズ（本書）

実リソース操作なし。

### 16.2 実装 PR フェーズの STOP 設計

| STOP | 内容 |
|---|---|
| **STOP α** | Cloud SQL migration v18（`usage_counters`）適用前停止 → 適用後検証 |
| **STOP β** | （case A 採用なら不要）`USAGE_LIMIT_HASH_SALT_V1` 新規 Secret 作成・注入。**case A 採用なら STOP β は省略** |
| **STOP γ** | Backend Cloud Build deploy 前停止 → deploy（runbook §1.4.1 準拠で 5〜10 分待機 + handler JSON smoke）|
| **STOP δ** | Workers redeploy 前停止 → deploy（rate_limited UI 反映） |
| **STOP ε** | Safari 実機 smoke 前停止 → 429 表示 / レイアウト確認（実 abuse 状況の作成は dummy で OK）|
| **STOP closeout** | work-log / failure-log / roadmap / runbook 反映 + Secret grep + commit + push |

### 16.3 cleanup Job 化はしない

- `cmd/ops usage cleanup --execute` 等の Cloud Run Job 化は本 PR 範囲外
- Cloud Scheduler も作らない（PR33e / PR41+ の判断）

---

## 17. 後回し事項・懸念

### 17.1 後回し事項

- **本格 WAF / Cloud Armor**: PR40 ローンチ前安全性強化 / 別 PR
- **Distributed rate-limit（Redis）**: 未使用、必要になったら別 PR
- **Cleanup 自動化**（Cloud Run Job + Scheduler）: 後続 PR / 運用フェーズ
- **`cmd/ops usage reset / cleanup --execute`**: 後続 PR
- **abuse analytics**: Phase 2
- **Web admin dashboard**: MVP 範囲外（v4 §6.19）
- **Paid plan / billing**: Phase 2
- **Email 通知（abuse 検知 → 運営）**: Email Provider 確定後
- **Legal / privacy policy 反映**: PR37（LP / terms / privacy / about）

### 17.2 懸念 / リスク

| 懸念 | 対応 |
|---|---|
| **NAT / IPv6 prefix 問題による false positive** | 閾値を緩めに設定 / 429 UI で時間をおけば再試行可能、cmd/ops で個別調査可能 |
| **Cloud SQL への write 増加** | INDEX 最小化 + ON CONFLICT DO UPDATE で write-amplification 抑制 / 過剰になれば後続 PR で Redis or partitioning 検討 |
| **fail-closed が正常利用者を巻き込む** | DB 障害時のみの稀な事象、選択肢として fail-open に倒す flag を実装可能（§17.3） |
| **hash salt rotation で長期追跡性を失う** | 業務知識 v4 §3.7 で許容済 |
| **IP hash の個人情報性** | runbook で取り扱いルール固定、外部提供しない |
| **Cloud SQL 容量増加（cleanup 未自動化）** | retention 24h grace + 手動 cleanup SQL を runbook に明記 |
| **cmd/ops の race**（運営が cleanup 中に増分が走る）| OK（読み取り専用なので race しても整合性に影響なし） |
| **window 境界での「リセット直後」連投** | fixed window の本質的弱点。MVP は許容、必要なら後続で sliding window 検討 |
| **report.submit の片方 consume 副作用** | 2 本の連続 CheckAndConsumeUsage（1: 5 分 3 件 / 2: 1 時間 20 件）を順に呼ぶため、1 を consume 成功した後 2 で deny されると「1 のカウントだけ進む」。MVP は許容（PR36 計画書）。後続候補: CheckOnly + Consume 分離 / TX 内 reservation 方式 / 1 SQL 内で複数 bucket atomic update（PostgreSQL stored procedure 化） |

### 17.3 fail-open / fail-closed の選択肢

- **MVP 既定: fail-closed**（DB 失敗時 429 deny）
- 将来 flag で切替可能にする選択肢を残す（`USAGE_LIMIT_FAIL_OPEN_ON_DB_ERROR=false` 等）
- 切替が必要になったら別 PR

---

## 18. ユーザー判断事項（**2026-04-30 ユーザー承認で全 12 項目確定済**）

> 本セクションの推奨案 A〜L はすべてユーザー承認済（2026-04-30、PR36 実装着手時）。
> 以下表は確定値として参照する。実装中に方針変更の必要が生じた場合は再ユーザー判断。
> 

| 項目 | 選択肢 | 推奨 | 備考 |
|---|---|---|---|
| **A. Salt 方針** | 案 A: `REPORT_IP_HASH_SALT_V1` 流用 / 案 B: 新規 / 案 C: IP hash 不使用 | **案 A** | 業務知識 v4 §3.7 整合 / Secret 追加不要 |
| **B. 初期対象 endpoint** | report.submit / upload_verification.issue / publish.from_draft / 上記以外 | **3 つすべて**（§4.2 推奨） | edit / OGP / public viewer は対象外 |
| **C. RateLimit ストア** | DB / in-memory / none | **DB（usage_counters）** | Redis 不採用 / Cloud SQL 単機 |
| **D. report.submit 閾値** | 同 photobook 5 分 N 件 / 同 IP 1 時間 M 件 | **同 IP 1 時間 20 件 + 同 IP × photobook 5 分 3 件** | §18 で確定 |
| **E. upload_verification.issue 閾値** | session × photobook で 1 時間 N 件 | **session × photobook で 1 時間 20 件**（Turnstile session 上限と同等以上）| 多重抑止 |
| **F. publish.from_draft 閾値** | 同一作成元 1 時間 N 件 | **同一 IP 1 時間 5 件**（業務知識 v4 §3.7 確定値）| 確定 |
| **G. cmd/ops list / show を本 PR に入れるか** | 入れる / 後続 | **入れる**（list / show のみ、reset / cleanup は後続）| 運営調査の最低限 |
| **H. cleanup を本 PR に含めるか** | runbook 手動 SQL のみ / cmd/ops cleanup --execute / Job 化 | **runbook 手動 SQL のみ** | 自動化は後続 |
| **I. Scheduler 作らない方針継続か** | 継続 / 作る | **継続**（PR33e / PR41+ で再判断）| outbox-worker と同じ運用 |
| **J. Safari 確認範囲** | 表示確認のみ / 429 文言 + Turnstile 状態区別 | **後者**（§14 必須項目）| iPhone Safari 必須 |
| **K. Cloud SQL は `vrcpb-api-verify` 継続か** | 継続 / 本番 rename | **継続** | PR39 で本番化判断 |
| **L. fail-closed / fail-open** | fail-closed / fail-open / flag | **fail-closed**（MVP 既定）| §17.3 |

> **本計画書を承認時に上記 12 項目を確定**してから実装 PR に入る。判断が割れる項目は
> ユーザーに選択肢を再提示して停止する。

---

## 19. 完了条件

- 計画書 review 通過
- §18 ユーザー判断事項 12 項目すべて確定
- PR36 実装 PR への引き継ぎ事項が明確
- roadmap §1.3 / §3 PR36 章を本計画書整合に更新

## 20. 関連

- 業務知識 v4 §2.7（用語）/ §3.7（UsageLimit 機能）/ §6（横断ルール）
- 計画書: [`m2-report-plan.md`](./m2-report-plan.md) §8 / §G5 / §11.5
- ルール: [`.agents/rules/turnstile-defensive-guard.md`](../../.agents/rules/turnstile-defensive-guard.md) / [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md) / [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
- failure-log: [`harness/failure-log/2026-04-29_report-form-turnstile-bypass.md`](../../harness/failure-log/2026-04-29_report-form-turnstile-bypass.md) / [`harness/failure-log/2026-04-29_turnstile-widget-remount-loop.md`](../../harness/failure-log/2026-04-29_turnstile-widget-remount-loop.md)
- runbook: [`docs/runbook/backend-deploy.md`](../runbook/backend-deploy.md) §1.4.1 / §1.4.2 / [`docs/runbook/ops-moderation.md`](../runbook/ops-moderation.md) §5.7
- 横断: [`docs/design/cross-cutting/outbox.md`](../design/cross-cutting/outbox.md)（本 PR では使わないが将来 abuse_detected 等の検討時に参照）

## 21. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-29 | 初版（PR36 計画書）。MVP scope（report.submit / upload_verification.issue / publish.from_draft の 3 endpoint × DB 単機 RateLimit）と §18 ユーザー判断 12 項目を整理 |
| 2026-04-30 | commit 3.5 反映: §5.2 に scope_type 統一表現の補足を追加（report.submit は scope_type='source_ip_hash' で複合 hash を使う、photobook_id 単体軸とは誤読しない）/ §17.2 に「片方 consume 副作用」をリスク表に追記 |
