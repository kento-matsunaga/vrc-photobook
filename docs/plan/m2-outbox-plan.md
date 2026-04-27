# Outbox 実装計画（PR30 計画書）

> 作成日: 2026-04-28
> 位置付け: 新正典 [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md)
> §3 PR30 の本体。**実装は PR30 commit 以降**で行う（本書では計画のみ確定）。
>
> 上流参照（必読）:
> - [新正典ロードマップ](./vrc-photobook-final-roadmap.md)
> - **[`docs/design/cross-cutting/outbox.md`](../design/cross-cutting/outbox.md)**（v4 設計の正典）
> - [業務知識 v4](../spec/vrc_photobook_business_knowledge_v4.md) §2.8 / §6.11
> - [Photobook データモデル設計](../design/aggregates/photobook/データモデル設計.md)
> - [Image データモデル設計](../design/aggregates/image/データモデル設計.md)
> - [PR23 image-processor 結果](../../harness/work-logs/2026-04-27_image-processor-result.md)
> - [PR28 publish flow 結果](../../harness/work-logs/2026-04-27_publish-flow-result.md)
> - [PR29 deploy 自動化 結果](../../harness/work-logs/2026-04-28_backend-deploy-automation-result.md)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
> - [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md)

---

## 0. 既存正典との関係

`docs/design/cross-cutting/outbox.md` に Outbox の **業務的正典**が既にある
（カラム / 索引 / 全 event 種別 / 状態遷移 / retry ポリシー）。本書は**それを実装に落と
す MVP 範囲の絞り込み**を行う。**全イベント種別を一度に実装しない**ため、PR30 / PR31 /
PR32 / PR33 / PR34 / PR35 でどう段階的に投入するかを §4 で確定する。

> 既存正典との差異が出た場合は **本書 + 実装** を更新し、cross-cutting/outbox.md は
> v4 設計のまま維持する。実装側は cross-cutting/outbox.md の §3 / §5 をベースラインとする。

---

## 1. 目的

- 集約の状態変更と副作用通知を **同一 DB TX で保証**する Outbox 基盤を導入
- PublishFromDraft / ProcessImage / MarkImageFailed の同 TX に `outbox_events` INSERT を
  追加し、後続 worker（PR31）で SendGrid / OGP / Reconcile に繋げる土台を作る
- PR30 では **table + Repository + 同 TX INSERT** までで止める。worker / retry / dead letter /
  Cloud Run Jobs / Scheduler は PR31 で扱う

---

## 2. PR30 対象範囲

### 対象（本 PR で実装）

- `outbox_events` migration（cross-cutting/outbox.md §3 に従う）
- sqlc query: `CreateOutboxEvent`（INSERT のみ）
- Outbox domain（`internal/outbox/domain/`）+ VO（`event_type` / `aggregate_type` / `payload`）
- Repository（`internal/outbox/infrastructure/repository/rdb/`）の `Create` メソッドを TX-bound
  で公開
- 既存 UseCase の同 TX INSERT 統合:
  - `PublishFromDraft` → `PhotobookPublished`
  - `ProcessImage`（available 確定 TX）→ `ImageBecameAvailable`
  - `ProcessImage`（failed 確定 TX）→ `ImageFailed`
- 上記 3 イベントの payload schema 確定（§5）
- test:
  - migration up / down
  - VO validation
  - Repository.Create
  - 各 UseCase で同 TX INSERT が成功すること
  - rollback で outbox 行が残らないこと
  - payload に禁止値（raw token / Cookie / storage_key 完全値）が含まれないこと

### 対象外（PR31 以降）

- worker（`cmd/outbox-worker`）
- retry / dead letter / `failed` 遷移ロジック
- Cloud Run Jobs / Scheduler
- SendGrid 送信本実装
- OGP 自動生成本実装
- Reconcile（PR41+）
- `processing` / `processed` / `failed` 用の sqlc query（PR31 で追加）
- 他のイベント種別（`ManageUrlReissued` / `PhotobookHidden` / `PhotobookSoftDeleted` /
  `PhotobookPurged` / `ReportSubmitted` / `OGPRegenerationRequested` 等）
- Reconcile / `outbox_failed.sh`

---

## 3. DB 設計

### 3.1 ベースライン

cross-cutting/outbox.md §3 のカラム / 索引を**そのまま採用**。MVP 段階の追加 / 変更は無し。

| カラム | 型 | NULL | 既定 | 備考 |
|---|---|---|---|---|
| `id` | uuid | NOT NULL | `gen_random_uuid()` | PK |
| `aggregate_type` | text | NOT NULL | - | CHECK（§3.3） |
| `aggregate_id` | uuid | NOT NULL | - | 対象集約 ID |
| `event_type` | text | NOT NULL | - | CHECK（§3.4 で MVP 値域に絞る） |
| `payload_json` | jsonb | NOT NULL | `'{}'` | §5 schema に従う |
| `status` | text | NOT NULL | `'pending'` | CHECK: `pending` / `processing` / `processed` / `failed` |
| `retry_count` | int | NOT NULL | `0` | PR31 の worker が更新 |
| `next_retry_at` | timestamptz | NULL | - | PR31 の worker が更新 |
| `created_at` | timestamptz | NOT NULL | `now()` | |
| `processing_started_at` | timestamptz | NULL | - | PR31 |
| `processed_at` | timestamptz | NULL | - | PR31 |
| `failed_at` | timestamptz | NULL | - | PR31 |
| `failure_reason` | text | NULL | - | PR31（最大 200 文字、§11） |

### 3.2 索引（MVP）

cross-cutting/outbox.md §3.2 通り。PR30 で全部作る:

```sql
-- worker pick 用（PR31 まで使われないが先に作る）
CREATE INDEX outbox_events_pickup_idx
  ON outbox_events (status, next_retry_at)
  WHERE status IN ('pending', 'failed');

-- 集約別履歴
CREATE INDEX outbox_events_aggregate_idx
  ON outbox_events (aggregate_type, aggregate_id, created_at DESC);

-- 種別集計
CREATE INDEX outbox_events_event_type_status_idx
  ON outbox_events (event_type, status);

-- failed 抽出
CREATE INDEX outbox_events_failed_idx
  ON outbox_events (status, failed_at DESC)
  WHERE status = 'failed';
```

### 3.3 `aggregate_type` の CHECK

```sql
CHECK (aggregate_type IN ('photobook', 'image', 'report', 'moderation', 'manage_url_delivery'))
```

PR30 で実体的に使うのは `photobook` / `image` のみ。CHECK 値域は将来 PR で `report` 等を
追加する際の拡張余地として広く取る。

### 3.4 `event_type` の CHECK（**MVP では値域を絞る**）

cross-cutting/outbox.md §4 の全 event を一気に CHECK に入れず、**PR30 で実体投入する 3 種だけ**を許可:

```sql
CHECK (event_type IN (
  'PhotobookPublished',
  'ImageBecameAvailable',
  'ImageFailed'
))
```

> **理由**: PR30 で UseCase に組み込む 3 種以外を CHECK で許可すると、誤って未対応 event
> が DB に書き込まれる事故を防げない。`failed` 状態の event を後から runtime で発見する
> より、CHECK で 0 行 INSERT させる方が安全。
>
> PR32 / PR33 / PR34 / PR35 で event を追加する都度、**migration で CHECK を緩める**。
> migration 番号は連続して増える。

### 3.5 payload_json の制約

DB レベルでの schema 強制はしない（jsonb のまま）。形式は §5 で application 側にて担保。
Repository に `payload` を渡す前に VO で構造検証する。

### 3.6 status 遷移

```
[INSERT 同 TX]
       ↓
   pending
       ↓ worker pick（PR31）
   processing
       ↓ 成功         ↓ 失敗
   processed       pending（retry）
                       ↓ retry 上限
                    failed → Reconcile（PR41+）
```

PR30 では **`pending` のみ**を application 層で扱う。worker / retry / failed は PR31。

---

## 4. Event type 段階投入

### 4.1 PR30 で入れる 3 種

| event_type | aggregate_type | 発火点（同 TX）|
|---|---|---|
| `PhotobookPublished` | `photobook` | `PublishFromDraft.Execute` の WithTx 内 |
| `ImageBecameAvailable` | `image` | `ProcessImage.Execute` の MarkAvailable + AttachVariant TX 内 |
| `ImageFailed` | `image` | `ProcessImage.failAndReturn` の MarkFailed 経路 |

### 4.2 PR30 で **入れない** event（後続 PR）

| event_type | 担当 PR |
|---|---|
| `ManageUrlReissued` | PR32（SendGrid 連携時） |
| `PhotobookHidden` / `PhotobookUnhidden` | PR34（Moderation） |
| `PhotobookSoftDeleted` / `PhotobookRestored` / `PhotobookPurged` | PR34 |
| `PhotobookUpdated` | PR41+（公開済 photobook の編集が始まったら） |
| `ReportSubmitted` | PR35 |
| `OGPRegenerationRequested` | PR33 |
| `UsageLimit*` | PR36 |
| `ManageUrlDeliveryRequested` | PR32 |

### 4.3 ProcessImage に Outbox INSERT を入れるか

**入れる**（推奨）。理由:

- PR23 計画書 §11 / image-processor の DB TX は既に **MarkAvailable + AttachVariant×2 を同一 TX**で
  実行中。同 TX に `CreateOutboxEvent` を 1 行追加するだけ
- 同 TX のため、event INSERT 失敗で MarkAvailable も rollback → R2 orphan は残るが PR25
  Reconcile（PR41+）で cleanup される設計と整合
- failed 経路（§3 / `failAndReturn`）も別 TX 化されているため、`MarkFailed` の TX に同様に追加

注意: `ProcessImage` 内で `failAndReturn` は単独の SQL UPDATE（`MarkFailed`）で動いているが、
**outbox INSERT を含める TX に書き換える**必要がある。実装 PR で `WithTx` ラップ + repo INSERT を追加する。

---

## 5. payload schema

### 5.1 共通フィールド

```jsonc
{
  "event_version": 1,        // schema 進化用
  "occurred_at": "2026-...Z" // RFC3339
}
```

### 5.2 PhotobookPublished

```jsonc
{
  "event_version": 1,
  "occurred_at": "2026-04-28T...Z",
  "photobook_id": "<uuid>",
  "slug": "ab12cd34ef56gh78",
  "visibility": "unlisted",
  "type": "memory",
  "cover_image_id": "<uuid>" // 任意
}
```

### 5.3 ImageBecameAvailable

```jsonc
{
  "event_version": 1,
  "occurred_at": "2026-...Z",
  "image_id": "<uuid>",
  "photobook_id": "<uuid>",
  "usage_kind": "photo",
  "normalized_format": "jpg",
  "variant_count": 2 // display + thumbnail
}
```

### 5.4 ImageFailed

```jsonc
{
  "event_version": 1,
  "occurred_at": "2026-...Z",
  "image_id": "<uuid>",
  "photobook_id": "<uuid>",
  "failure_reason": "object_not_found" // failure_reason VO の値域に限定
}
```

### 5.5 payload に **入れない**項目（厳守）

- `raw_token` / `manage_url_token` / `draft_edit_token` / `session_token` / Cookie 値
- `manage_url_token_hash` / `draft_edit_token_hash` 等の hash bytea
- presigned URL（GET / PUT 両方）
- `storage_key` 完全値
- R2 credentials（access key / secret）
- DATABASE_URL / 各種 secret 値
- email address（PR32 で SendGrid 連携時に最小限を許容、PR30 では入れない）
- 個人を特定する文字列（VRC user id 等は creator_x_id だけは公開済のため payload にも入れて可）

> payload の方針は「**worker が必要な情報を再取得できる最小値**」。詳細データは aggregate
> 側を find し直すのが原則。

---

## 6. 同一 TX 方針

### 6.1 PublishFromDraft

既存実装は `database.WithTx` 内で:

1. `repo.PublishFromDraft`（status='draft' AND version=$expected で UPDATE）
2. `revoker.RevokeAllDrafts`

ここに **(3) `outboxRepo.Create(ctx, event)`** を追加する。

```go
// 擬似コード（PR30 実装で追加）
err := database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
    pbRepo := u.photobookRepoFactory(tx)
    revoker := u.revokerFactory(tx)
    outboxRepo := outboxrdb.NewOutboxRepository(tx)

    if err := pbRepo.PublishFromDraft(...); err != nil { return err }
    if err := revoker.RevokeAllDrafts(...); err != nil { return err }
    if err := outboxRepo.Create(ctx, photobookPublishedEvent(...)); err != nil { return err }
    return nil
})
```

### 6.2 ProcessImage（MarkAvailable）

既存 `process_image.go` の `database.WithTx` 内:
1. `txRepo.MarkAvailable`
2. `txRepo.AttachVariant(displayVariant)`
3. `txRepo.AttachVariant(thumbnailVariant)`

→ **(4) `outboxRepo.Create(ImageBecameAvailable)`** を追加。

### 6.3 ProcessImage.failAndReturn（MarkFailed）

現在は `repo.MarkFailed` を pool で直接呼んでいる（TX なし）。実装 PR で `WithTx` ラップに
書き換え、TX 内に `MarkFailed` + `outboxRepo.Create(ImageFailed)` を入れる。

### 6.4 失敗時の挙動

- pbRepo / imgRepo の UPDATE が失敗 → outbox INSERT も rollback（同 TX）
- outbox INSERT が失敗 → 状態更新も rollback（同 TX）
- どちらも `database.WithTx` の defer Rollback により担保

### 6.5 outbox INSERT 自体の実行コスト

- 1 行 INSERT、jsonb payload 数百 byte 程度
- index 4 個あるが INSERT 時の overhead は ms 単位
- TX 全体の時間に対し誤差レベル

---

## 7. Repository / sqlc 方針

### 7.1 PR30 で追加する sqlc query

**MVP では INSERT のみ。** worker 系の query は PR31 で。

```sql
-- name: CreateOutboxEvent :exec
INSERT INTO outbox_events (
    id, aggregate_type, aggregate_id, event_type, payload_json,
    status, retry_count, created_at
) VALUES (
    $1, $2, $3, $4, $5, 'pending', 0, $6
);
```

### 7.2 PR31 で追加する query（**PR30 では作らない**）

- `ListPendingOutboxEventsForUpdate`（FOR UPDATE SKIP LOCKED）
- `MarkOutboxProcessing`
- `MarkOutboxProcessed`
- `MarkOutboxFailed`
- `BumpOutboxRetry`
- `ReleaseStaleLocks`（worker crash 時の reset 用）

### 7.3 sqlc.yaml

新規 sqlc set を追加:

```yaml
- engine: postgresql
  schema:
    - migrations/00001_create_health_check.sql
    - migrations/000XX_create_outbox_events.sql  # PR30 追加分
  queries: internal/outbox/infrastructure/repository/rdb/queries
  gen:
    go:
      package: sqlcgen
      out: internal/outbox/infrastructure/repository/rdb/sqlcgen
      sql_package: pgx/v5
      emit_pointers_for_null_types: true
      emit_json_tags: false
```

---

## 8. Outbox package 構成

```
backend/internal/outbox/
├── domain/
│   ├── event.go                    # Event entity（PR30 で実装）
│   ├── vo/
│   │   ├── event_type/             # PR30 で 3 種のみ
│   │   ├── aggregate_type/         # PR30 で 5 種許可
│   │   └── payload/                # JSON serializable struct（型ごと）
│   └── tests/
│       └── event_builder.go
└── infrastructure/
    └── repository/
        └── rdb/
            ├── outbox_repository.go # Create のみ
            ├── marshaller/
            ├── queries/
            │   └── outbox.sql
            └── sqlcgen/             # sqlc 生成物
```

PR31 で `internal/outbox/internal/usecase/` と `cmd/outbox-worker/` を追加。
PR30 では UseCase 層は作らない（既存 photobook / imageprocessor UseCase が outbox を
呼ぶため）。

---

## 9. Migration 方針

### 9.1 番号

既存最終: `00011_create_upload_verification_sessions.sql`。
PR30 で **`00012_create_outbox_events.sql`** を追加。

### 9.2 goose up / down

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE outbox_events ( ... );
CREATE INDEX outbox_events_pickup_idx ON ...;
CREATE INDEX outbox_events_aggregate_idx ON ...;
CREATE INDEX outbox_events_event_type_status_idx ON ...;
CREATE INDEX outbox_events_failed_idx ON ...;
-- +goose StatementEnd

-- +goose Down
DROP TABLE outbox_events;
```

### 9.3 適用タイミング（停止ポイント）

- PR30 実装 PR の test 時: ローカル docker postgres に goose up
- **本番 Cloud SQL `vrcpb-api-verify` への適用**: 実装 PR 内で **停止ポイント**を置き、
  ユーザー承認後に Cloud SQL Auth Proxy + goose up
- rollback は goose down で `DROP TABLE outbox_events`（参照する FK 等は無いため安全）

### 9.4 影響範囲

- 既存 UseCase はそのままだと `CreateOutboxEvent` を呼ばないため、migration だけ適用しても
  既存機能は影響しない
- UseCase に outbox INSERT を追加した版を deploy すると、旧 image にロールバックしても
  outbox にデータが**残るだけ**で副作用は無い（worker 未稼働のため）
- migration → deploy の順序は **migration 先**に確定。本番への migration 適用 → deploy →
  smoke のシーケンス

---

## 10. Test 方針

### 10.1 必須 test

| 観点 | テスト |
|---|---|
| migration | goose up / down が clean |
| event_type VO | 3 種を許可 / 未知値で error |
| aggregate_type VO | 5 種を許可 / 未知値で error |
| payload struct → jsonb 化 | `json.Marshal` + DB に書き込み + 取得で round-trip |
| Repository.Create（実 DB） | 1 行 INSERT 成功 / status='pending' / created_at 設定 |
| PublishFromDraft 同 TX | publish 成功時に outbox 1 行追加 / publish 失敗時に outbox 行も rollback |
| ProcessImage MarkAvailable 同 TX | available 成功時に outbox 1 行追加 |
| ProcessImage MarkFailed 同 TX | failed 確定時に outbox 1 行追加 / 失敗 reason が payload に乗る |
| payload 禁止値 grep test | payload JSON に `manage_url_token` / `draft_edit_token` / `presigned` / `R2_SECRET` / `Cookie` の string が含まれないことを test 内 grep |
| index 検証（軽め） | EXPLAIN で `outbox_events_pickup_idx` が使われることを 1 件確認 |

### 10.2 削除と rollback の test

- `WithTx` 内で intentional error を inject → outbox 行が DB に残らないことを確認
- `MarkFailed` 経路で intentional error → MarkFailed 行も outbox 行も rollback

---

## 11. Security

### 11.1 守るべき不変条件

- payload に raw token / Cookie / hash bytea / presigned URL / storage_key 完全値 / R2 credentials /
  DATABASE_URL / Secret 値 / email address を入れない（§5.5）
- log に payload 全文を出さない（必要に応じて event_id / event_type のみ）
- worker（PR31）の `failure_reason` 列にも Secret を含めない（§11.2）
- event_id を公開 viewer / response body に出さない
- `aggregate_type` / `event_type` は CHECK で値域固定、未知値の混入を防ぐ

### 11.2 `failure_reason` sanitize（PR31 で worker が書く）

PR30 では未使用だが、PR31 で worker が以下を守る前提で migration / Repository を設計:

- 200 文字に切り詰め
- `DATABASE_URL=` / `R2_*=` / token 形式 (`tk_` / `Bearer ` 等) のパターンを redact
- log にも sanitize 後の値だけを出す

PR30 の Repository / migration は文字列長制約（200 char）を CHECK 制約として張る:

```sql
CHECK (failure_reason IS NULL OR char_length(failure_reason) <= 200)
```

### 11.3 grep 監査（PR30 実装 PR commit 前）

```
grep -RInE "DATABASE_URL=|PASSWORD=|SECRET=|SECRET_KEY|API_KEY|sk_live|sk_test|R2_SECRET_ACCESS_KEY=|TURNSTILE_SECRET_KEY=" \
  backend/internal/outbox \
  backend/internal/photobook \
  backend/internal/imageprocessor \
  backend/migrations/00012_*.sql
```

実値ヒット 0 件。用語のみ可。

---

## 12. PR31 への引き継ぎ

PR31 で実装する:

- `cmd/outbox-worker/main.go`
- worker UseCase（`ListPendingOutboxEventsForUpdate` + handler dispatch）
- handler:
  - `PhotobookPublished` → 当面 no-op + log（OGP は PR33、SendGrid は PR32）
  - `ImageBecameAvailable` → no-op + log（OGP は PR33）
  - `ImageFailed` → no-op + log（cleanup は PR41+ Reconcile）
- retry / dead letter 遷移
- Cloud Run Jobs / Scheduler 定義（image-processor と同 image を流用、PR23 で同梱済）
- worker 用 sqlc query 群（§7.2）

PR30 でやらない:

- worker 本体
- Cloud Run Jobs
- Scheduler
- retry 実行
- dead letter 処理
- 各 event の副作用本実装（OGP / SendGrid / Reconcile）

---

## 13. 実リソース操作

### 13.1 PR30 計画書（本書）

実リソース操作なし（docs のみ）。

### 13.2 PR30 実装 PR

- migration 追加（`00012_create_outbox_events.sql`）
- ローカル docker postgres に goose up（test 用）
- **停止ポイント**: 本番 Cloud SQL `vrcpb-api-verify` への migration 適用前にユーザー承認
- Backend deploy は **PR29 の Cloud Build manual submit 経由**
  （`gcloud builds submit --config=cloudbuild.yaml --service-account=vrcpb-cloud-build@... <repo-root>`）
- Secret 追加なし
- Dashboard 操作なし

---

## 14. PR28 visual Safari 残課題

PR30 とは独立。引き続き **manual 残課題**として新正典 §1.3 に維持。
PR30 進行中も並行実施可能（手順は
[`harness/work-logs/2026-04-27_publish-flow-result.md`](../../harness/work-logs/2026-04-27_publish-flow-result.md)
§推奨次手順）。

PR30 作業中にユーザーが manual 実施した場合は work-log に追記。

---

## 15. ユーザー判断事項（実装 PR 着手前に確定）

| 判断項目 | 推奨 | 代替 |
|---|---|---|
| PR30 で入れる event 種別 | **3 種（PhotobookPublished / ImageBecameAvailable / ImageFailed）** | 5 種以上（PR31 担当を前倒し）|
| outbox status 設計 | **pending / processing / processed / failed の 4 値（cross-cutting/outbox.md §5）**| 簡略化（pending / processed のみ） |
| worker 用 query を PR30 で作るか | **作らない**（PR31 で追加）| PR30 で先に作る |
| payload に slug を入れるか | **入れる**（PhotobookPublished、worker 側で OGP 生成 URL 組立に使う） | 後で aggregate 再 fetch |
| ProcessImage に outbox insert を入れるか | **入れる**（available / failed 両方） | available のみ |
| 本番 Cloud SQL に migration 適用するタイミング | **PR30 実装 PR の停止ポイントで承認後** | PR31 着手と同時 |
| event_type CHECK 値域 | **3 種に絞る**（誤投入防止） | 全種を入れる |
| Cloud SQL `vrcpb-api-verify` 残置 | PR39 まで継続 | 早期 rename |
| Public repo 化 | PR38 まで保留 | 早期公開 |
| PR28 visual Safari 残課題 | manual 残課題として継続 | PR30 作業中に並行実施 |

---

## 16. 完了条件

- 本計画書 review 通過
- §15 ユーザー判断事項が確定
- §2.1 / §10 / §11 が PR30 実装 PR で checklist としてそのまま使える状態
- §13 実リソース操作の停止ポイントが明示されている

---

## 17. 次 PR への引き継ぎ事項

PR30 実装 PR 着手時に必ず参照:

- §3 DB 設計（cross-cutting/outbox.md と整合）
- §4 PR30 で入れる 3 イベント
- §5 payload schema（禁止値含む）
- §6 各 UseCase の同 TX INSERT 手順
- §7 sqlc query は Create のみ
- §8 package 構成
- §9 migration `00012_*` + 本番適用の停止ポイント
- §10 test 必須項目
- §11 Security 不変条件

PR31 への引き継ぎ事項は §12。

---

## 18. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版作成。cross-cutting/outbox.md を上流正典として参照、PR30 MVP 範囲を 3 イベントに絞る |
