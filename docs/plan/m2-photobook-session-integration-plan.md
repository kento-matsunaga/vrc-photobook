# M2 Photobook aggregate + Session 接続 実装計画（PR9 候補）

> 作成日: 2026-04-26
> 位置付け: backend M2 本実装の **PR9（または PR9a/9b/9c 分割）** 着手前の計画書。実装コードはまだ書かない。
>
> 上流参照（必読）:
> - [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §2.3 / §2.6 / §3.1 / §3.2 / §3.4 / §3.5 / §5 / §6.18-6.20
> - [`docs/design/aggregates/README.md`](../design/aggregates/README.md)
> - [`docs/design/aggregates/photobook/ドメイン設計.md`](../design/aggregates/photobook/ドメイン設計.md)
> - [`docs/design/aggregates/photobook/データモデル設計.md`](../design/aggregates/photobook/データモデル設計.md)
> - [`docs/design/auth/session/ドメイン設計.md`](../design/auth/session/ドメイン設計.md)（I-S1〜I-S14）
> - [`docs/adr/0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md)
> - [`docs/plan/m2-session-auth-implementation-plan.md`](./m2-session-auth-implementation-plan.md) §13
> - [`docs/plan/m2-implementation-bootstrap-plan.md`](./m2-implementation-bootstrap-plan.md)
> - [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md)
> - [`.agents/rules/testing.md`](../../.agents/rules/testing.md)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)

---

## 0. 本計画書の使い方

- 本計画書は **計画書のみ**。実装 / migration / sqlc / handler はまだ書かない。
- 設計書（`docs/design/aggregates/photobook/`）が確定事項を持つ。本書ではそれを **どう PR に切り出すか / どこまでを PR9 に入れるか** を整理する。
- 範囲が広いため、**PR9 を 3 分割（PR9a / PR9b / PR9c）** することを推奨する（§13）。
- §14 のユーザー判断事項に答えてもらってから PR9a 着手に進む。

---

## 1. 目的

- M2 本実装に Photobook 集約を追加し、Session auth 機構と接続する。
- 「ログイン不要 + token URL + HttpOnly Cookie session」の核となるフロー（ADR-0003）を **本物の token 検証経路で完成させる**。
- PR8 で枠だけだった Session middleware を、Photobook の draft_edit_token / manage_url_token を起点とした入場フローに接続する。
- publish / reissueManageUrl 時の **同一 TX 内 Session 一括 revoke**（I-D7 / I-S9 / I-S10）を実装する。
- Frontend route handler / 実環境デプロイ / 独自ドメインは PR9 のスコープ外（PR10 で実機検証込みの追加）。

---

## 2. 前提

- PR7 で Session domain / repository / Cookie policy / migration（`sessions` テーブル）が完成済み。
- PR8 で Session UseCase（Issue / Validate / Touch / Revoke / RevokeAll*）と middleware 枠が完成済み。**dummy token 経由の公開 endpoint は本番 router へ未接続**。
- PR9 で初めて **本物の draft_edit_token / manage_url_token 検証経由の token 交換 endpoint** を扱う。
- Cloud SQL は使わない、Cloud Run / Workers 実 deploy しない、ローカル PostgreSQL（`docker-compose.yaml`）でのみ動かす。
- Frontend route handler（`/draft/[token]/route.ts` 等）は PR10 で実装。
- 独自ドメイン購入も PR9 完了後、PR10 着手前に再判断（`m2-session-auth-implementation-plan.md` §14.8）。
- Safari / iPhone Safari 実機確認は PR10（Cookie / redirect / OGP / レスポンスヘッダが入る段階）で実施。
- harness/spike のコードは直接コピペで持ち込まない。

---

## 3. PR9 の対象範囲

### 対象（本計画 / PR9a〜9c で扱う）

- `backend/internal/photobook/`（集約一式：domain / VO / infrastructure / usecase）
- migration: `00003_create_photobooks.sql`、`00004_add_photobooks_fk_to_sessions.sql`（または同一 migration 内で連続実行）
- token: `DraftEditToken` / `DraftEditTokenHash` / `ManageUrlToken` / `ManageUrlTokenHash` / `ManageUrlTokenVersion` 値オブジェクト
- Photobook 集約の状態遷移（draft / published）と最小ドメイン操作
- `publishFromDraft` の同一 TX 内 `RevokeAllDrafts` 呼び出し
- `reissueManageUrl` の同一 TX 内 `RevokeAllManageByTokenVersion` 呼び出し
- token 交換 UseCase（`ExchangeDraftTokenForSession` / `ExchangeManageTokenForSession`）
- HTTP endpoint（`POST /api/auth/draft-session-exchange` / `POST /api/auth/manage-session-exchange`）— **本物の token 検証経由**
- Session middleware の本番 router 接続（保護ルートのプレースホルダ）
- ローカル DB での実 TX 動作確認

### 対象外（後続 PR に持ち越し）

- Frontend route handler（PR10）
- Image / Page / Photo 集約（Photobook と密結合だが、本 PR は Photobook 単体まで）
- Image upload（presigned URL / R2）
- OGP / Twitter card / SSR メタタグ
- Outbox の **本実装**（Outbox table への INSERT は本 PR で **しない**、§7.1 で議論）
- Reconcile / GC（draft 期限切れ削除のバッチ）
- Report / Moderation 集約（本 PR は Moderation 経由でない `reissueManageUrl` の最小経路のみ）
- ManageUrlDelivery / SendGrid / メール送信
- UsageLimit
- 公開ページ `/p/{slug}` / 編集 UI / 管理 UI
- 楽観ロックの **複数 UI 経由テスト**（OptimisticLockConflict の発火検証は単体テストレベル）
- Cloud SQL / Cloud Run / Workers 実 deploy / ドメイン購入

---

## 4. Photobook aggregate データモデル案

### 4.1 確定事項（設計書 [`docs/design/aggregates/photobook/データモデル設計.md`](../design/aggregates/photobook/データモデル設計.md) より）

カラム一覧（全カラム、PR9 でどこまで入れるかは §4.2）:

| カラム | 型 | NULL | 既定 |
|---|---|---|---|
| `id` | uuid | NOT NULL | UUIDv7 |
| `type` | text | NOT NULL | - |
| `title` | text | NOT NULL | - |
| `description` | text | NULL | - |
| `layout` | text | NOT NULL | - |
| `opening_style` | text | NOT NULL | - |
| `visibility` | text | NOT NULL | `unlisted` |
| `sensitive` | boolean | NOT NULL | `false` |
| `rights_agreed` | boolean | NOT NULL | `false` |
| `rights_agreed_at` | timestamptz | NULL | - |
| `creator_display_name` | text | NOT NULL | - |
| `creator_x_id` | text | NULL | - |
| `cover_title` | text | NULL | - |
| `cover_image_id` | uuid | NULL | - |
| `public_url_slug` | text | NULL | - |
| `manage_url_token_hash` | bytea | NULL | - |
| `manage_url_token_version` | int | NOT NULL | `0` |
| `draft_edit_token_hash` | bytea | NULL | - |
| `draft_expires_at` | timestamptz | NULL | - |
| `status` | text | NOT NULL | `draft` |
| `hidden_by_operator` | boolean | NOT NULL | `false` |
| `version` | int | NOT NULL | `0` |
| `published_at` | timestamptz | NULL | - |
| `created_at` | timestamptz | NOT NULL | now() |
| `updated_at` | timestamptz | NOT NULL | now() |
| `deleted_at` | timestamptz | NULL | - |

### 4.2 PR9 で入れるか / 入れないかの切り分け（推奨）

| カラム | PR9 で入れる | 理由 |
|---|---|---|
| `id` | ✅ 入れる | PK 必須 |
| `type` / `layout` / `opening_style` | ✅ 入れる（CHECK 制約付き、固定値のみ） | 状態遷移条件の検証で使う、既定値を VO で固定 |
| `title` | ✅ 入れる | createDraft の必須引数 |
| `description` | ⏸ NULL 許容のまま入れるが、編集 UseCase は PR9 では作らない | 列だけあれば良い |
| `visibility` / `sensitive` | ✅ 入れる（既定値のみ、編集 UseCase なし） | publish 時の検証で参照 |
| `rights_agreed` / `rights_agreed_at` | ✅ 入れる（既定値 false） | publish 条件 I7 で使う |
| `creator_display_name` | ✅ 入れる | publish 条件 I7 |
| `creator_x_id` | ⏸ NULL 許容、入れる | スキーマ整合のみ |
| `cover_title` / `cover_image_id` | ⏸ NULL 許容、入れる（FK は張らない） | Image 集約が PR11 以降のため FK 後付け |
| `public_url_slug` | ✅ 入れる（部分 UNIQUE） | publish で発行 |
| `manage_url_token_hash` / `manage_url_token_version` | ✅ 入れる（核） | 本 PR の主要対象 |
| `draft_edit_token_hash` / `draft_expires_at` | ✅ 入れる（核） | 本 PR の主要対象 |
| `status` / `hidden_by_operator` | ✅ 入れる（CHECK 制約） | 状態遷移の核 |
| `version` | ✅ 入れる | 楽観ロック必須 |
| `published_at` / `created_at` / `updated_at` / `deleted_at` | ✅ 入れる | 状態遷移の検証で使う |

**まだ入れない（後続 PR / 別テーブル）**:

- `pages` / `photos` / `page_metas` テーブル（Photobook 集約内の子テーブル群、Image 集約と一緒に PR11 以降）
- `outbox_events`（Outbox 本実装は別 PR、§7.1）
- `images` / `image_variants`（Image 集約、PR11 以降）
- `manage_url_deliveries`（PR12 以降）

### 4.3 状態遷移（PR9 で扱うのは draft → published と reissue のみ）

```
[create] →  draft  →[publish]→ published
                                  ↓ [reissueManageUrl]（同状態に留まる、token version +1）
                                  ↓ [softDelete]
                                deleted  →[restore]→ published
                                  ↓ [purge]
                                purged
```

**PR9 で実装する状態遷移**:

- `createDraft`（draft 作成）
- `touchDraft`（draft 延長、編集系 API 成功時に `draft_expires_at = now+7d`）
- `publishFromDraft`（draft → published）
- `reissueManageUrl`（published 内で token version +1）

**PR9 で実装しない状態遷移**（後続 PR）:

- `softDelete` / `restore` / `purge`（Moderation 経路のため、Moderation 集約と一緒に後続 PR）
- `hide` / `unhide`（同上）
- `updateContent` / `setTitle` 等の編集系操作（編集 UI が無いため意味が薄い、後続 PR）

### 4.4 unique / CHECK / index 概要（詳細は §7 migration 計画）

UNIQUE:
- `id`（PK）
- `public_url_slug` WHERE `status IN ('published','deleted')`（部分 UNIQUE、Slug 復元ルール）
- `manage_url_token_hash` WHERE `manage_url_token_hash IS NOT NULL`
- `draft_edit_token_hash` WHERE `draft_edit_token_hash IS NOT NULL`

CHECK:
- `status IN ('draft','published','deleted','purged')`
- `visibility IN ('public','unlisted','private')`
- `type IN (...)` / `layout IN (...)` / `opening_style IN (...)`
- draft / published 整合性（I-D1 / I-D2 / I-D6）— 大きな CASE WHEN を 1 つ
- token hash 長 = 32B
- `version >= 0`

INDEX:
- `(status, deleted_at)`
- `(status, draft_expires_at) WHERE status='draft'`

---

## 5. Token 設計

### 5.1 確定事項（既存設計書）

| 種別 | raw 形式 | DB 保存 | 発行タイミング | 失効タイミング |
|---|---|---|---|---|
| `draft_edit_token` | base64url（**32B 乱数 / 43 文字**、§5.2 で根拠） | SHA-256 32B | createDraft | publish 成功で hash を NULL |
| `manage_url_token` | 同上 | 同上 | publishFromDraft / reissueManageUrl | reissueManageUrl で置換 |
| `session_token`（PR7 既存） | 同上 | 同上 | token 交換成功時 | publish の RevokeAllDrafts / reissue の RevokeAllManageByTokenVersion / 期限切れ / 明示破棄 |

### 5.2 token 長について（業務知識 v4 §2.3 と PR7 整合）

業務知識 v4 では「最低 128bit 相当のエントロピー」と書かれているが、PR7 の `SessionToken` は **256bit (32 バイト)** を採用済（ADR-0003 §5.2 と整合）。`draft_edit_token` / `manage_url_token` も同じ実装ポリシーで **256bit** に揃える（業務知識の「最低 128bit」を満たしつつ、Session と同じ生成器で安全側に倒す）。

→ §14 でユーザー最終確認。

### 5.3 生成器の配置

- `backend/internal/photobook/domain/vo/draft_edit_token/draft_edit_token.go`
- `backend/internal/photobook/domain/vo/draft_edit_token_hash/draft_edit_token_hash.go`
- `backend/internal/photobook/domain/vo/manage_url_token/manage_url_token.go`
- `backend/internal/photobook/domain/vo/manage_url_token_hash/manage_url_token_hash.go`
- `backend/internal/photobook/domain/vo/manage_url_token_version/manage_url_token_version.go`

**実装方針**: PR7 の `session_token` / `session_token_hash` と **同じパターン**で書く（コピペではなく、単独の VO として書き直す。コア生成器は `crypto/rand.Read(buf[:32])` + `base64.RawURLEncoding`、hash は `sha256.Sum256`）。共通化（generic / 親パッケージ）はしない（VO の独立性を優先、`domain-standard.md` §VO 自己完結）。

### 5.4 raw token を返すタイミング

| 種別 | クライアントへ raw を返す唯一のタイミング |
|---|---|
| `draft_edit_token` | createDraft の戻り値（CreatorReceipt の一部、ADR-0003）。発行直後の HTTP response でのみ raw を出す |
| `manage_url_token` | publishFromDraft / reissueManageUrl の戻り値（同上） |
| `session_token` | token 交換 endpoint の戻り値（response body）— Frontend Route Handler が受け取って Cookie に書く |

**いずれもログ・diff・テストログ・OGP・公開 URL に raw を出さない**（I13 / I14 / I15）。

### 5.5 token 失効ルール

- draft_edit_token: publishFromDraft 成功で `draft_edit_token_hash = NULL`、`draft_expires_at = NULL`（同一 TX、I-D6）
- manage_url_token: reissueManageUrl で **置換のみ**（旧 hash は捨てる、不可逆）
- session_token: §6 を参照

### 5.6 manage_url_token_version の運用

- 初期値 `0`（publish 時セット、それ以前は意味を持たない）
- `reissueManageUrl` 成功で `+1`
- manage session 発行時に snapshot（`token_version_at_issue = Photobook.manage_url_token_version`）
- `reissueManageUrl` 同一 TX で `RevokeAllManageByTokenVersion(photobook_id, oldVersion)` を呼ぶ（I-S10）

### 5.7 ログ・露出禁止チェック（再掲）

`shared/logging.go` 禁止リスト + 本 PR で追記:

- raw `draft_edit_token` / `manage_url_token`（hex / base64 表現含む）
- `draft_edit_token_hash` / `manage_url_token_hash`（バイナリ / hex / base64 表現）
- これらを含む URL（`/draft/{token}` / `/manage/token/{token}`）の **完全 URL**

---

## 6. Session 連携

### 6.1 連携ポイント一覧

| Photobook 操作 | Session 機構への呼び出し | 同一 TX か |
|---|---|---|
| `createDraft` | （まだ session を作らない、draft_edit_token を返すだけ）| - |
| `ExchangeDraftTokenForSession` UseCase | `IssueDraftSession`（PR8 既存） | ✅ TX 内 |
| `ExchangeManageTokenForSession` UseCase | `IssueManageSession`（PR8 既存） | ✅ TX 内 |
| `publishFromDraft` | `RevokeAllDrafts(photobook_id)`（PR8 既存）| ✅ **必須**（I-D7 / I-S9） |
| `reissueManageUrl` | `RevokeAllManageByTokenVersion(photobook_id, oldVersion)`（PR8 既存） | ✅ **必須**（I-S10） |
| `softDelete` 等 | （PR9 では実装しない、後続 PR）| - |

### 6.2 同一 TX の組み立て方（pgx の Tx を共有する）

PR8 の Session UseCase は `SessionRepository` interface に依存しているが、現在の実装は `*pgxpool.Pool` を受け取る `SessionRepository`（PR7 で作成）のみ。同一 TX で動かすには、**pgx の `Tx` を `sqlcgen.DBTX` として渡せる**ことを利用して、Repository を `Tx` 起点で生成する経路を提供する。

実装方針:

1. `Session` 側 / `Photobook` 側の Repository を **`sqlcgen.DBTX` で構成可能**にする（既存 PR7 の `NewSessionRepository(db sqlcgen.DBTX)` はすでにそう）。
2. UseCase 層に **TxRunner** ヘルパを 1 つだけ追加する（`backend/internal/database/tx.go`）:
   ```go
   func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
       tx, err := pool.Begin(ctx)
       if err != nil { return err }
       defer tx.Rollback(ctx)
       if err := fn(tx); err != nil { return err }
       return tx.Commit(ctx)
   }
   ```
3. Photobook の `PublishFromDraft` UseCase は `WithTx` 内で:
   - `photobookRepo := rdb.NewPhotobookRepository(tx)`
   - `sessionRepo := sessionrdb.NewSessionRepository(tx)`
   - `photobookRepo.PublishFromDraft(...)` → 成功
   - `sessionRepo.RevokeAllDrafts(...)` → 成功
   - 全成功で `Commit`
4. これで **2 集約の repository が同じ pgx.Tx を共有**して動く。

### 6.3 Circular dependency 回避

- Photobook UseCase は **Session の UseCase ではなく Repository interface を経由して呼ぶ**（usecase が依存する interface は usecase 自身が宣言する形にする）。
- Session 側の Repository（PR7）は Photobook を知らない（photobook_id を引数 UUID として受け取るだけ）。
- 逆方向（Session → Photobook）の依存は本 PR では作らない。

### 6.4 Session interface を Photobook 側で再宣言する案

Photobook UseCase が Session UseCase の interface を import すると、import の方向が `photobook → auth/session/internal/usecase` になり、Go の `internal/` 規則で **import 不可**（auth/session 配下からしか import できない）。

そのため、Photobook UseCase は **Session 側の Repository interface を直接呼ばず**、PR8 の usecase をラップする「session ports interface」を Photobook 側に宣言する:

```go
// backend/internal/photobook/internal/usecase/session_ports.go
type DraftSessionRevoker interface {
    RevokeAllDrafts(ctx context.Context, pid PhotobookID) (int64, error)
}
type ManageSessionRevoker interface {
    RevokeAllManageByTokenVersion(ctx context.Context, pid PhotobookID, oldVersion int) (int64, error)
}
```

実体（adapter）を `backend/internal/photobook/infrastructure/session_adapter/` に置き、PR8 の `*usecase.RevokeAllDrafts` / `*usecase.RevokeAllManageByTokenVersion` を呼ぶ薄い wrapper を 1 ファイルで実装する。

これにより:
- Photobook UseCase は Session 側の internal package を直接 import しない
- Tx 共有のために、adapter は Tx を受け取り、`session/usecase.NewRevokeAllDrafts(sessionRepoFromTx)` で組み立てる
- Photobook と Session の依存方向は **photobook → adapter → session** に一方向化

---

## 7. Migration 計画

### 7.1 Outbox は本 PR では作らない

- 設計書 I-O1 では Photobook の状態変更時に `outbox_events` への INSERT を同一 TX で必須としているが、Outbox table 自体の DDL / Outbox Worker / Reconcile は別 PR で扱う方針（`m2-implementation-bootstrap-plan.md` step 7+）。
- PR9 では **Outbox INSERT を呼び出さない**（コメントで「PR12 以降で追加」と明記）。
- Outbox 抜きでは「副作用が即時実行されない」状態だが、本 PR の主目的は「Session revoke の同一 TX 動作」を確立することであり、Outbox は後付けで TX に追加できる（Photobook 集約の状態遷移 SQL 自体は変えずに済む）。
- → §14 でユーザー判断。「PR9 範囲では Outbox を含めない」を確定する。

### 7.2 PR9 内の migration ファイル

- `backend/migrations/00003_create_photobooks.sql`
  - photobooks table（§4.2 のカラム）
  - CHECK / UNIQUE / INDEX
  - `cover_image_id` への FK は **張らない**（Image table が無い、PR11 で `ALTER TABLE`）
- `backend/migrations/00004_add_photobooks_fk_to_sessions.sql`
  - `ALTER TABLE sessions ADD CONSTRAINT sessions_photobook_id_fkey FOREIGN KEY (photobook_id) REFERENCES photobooks(id) ON DELETE CASCADE;`
  - PR7 で意図的に未設定だった FK を、photobooks table が存在するこの段階で追加する（`m2-session-auth-implementation-plan.md` §8.3 / §13 の方針通り）

### 7.3 Down migration

- `00004` → FK 削除のみ
- `00003` → DROP TABLE photobooks（FK が先に外れているので影響なし）

### 7.4 既存 sessions 行の整合

- PR7-8 のテストで作った session 行はテスト後に TRUNCATE される前提
- 開発者がローカル環境に session を残したまま `goose up` した場合、photobook_id が photobooks に存在せず FK 違反が発生しうる
- 対策: README の手順を「FK 追加前に sessions を TRUNCATE する手順」を追加するか、本番で意味のあるデータ移行は無いので「ローカルで `down -v` して再起動」を案内する

### 7.5 seed なし

- `migrations/` 配下に seed は置かない（`testing.md` §禁止事項）。
- テストでの photobook 作成はすべて Builder 経由。

---

## 8. sqlc / repository 計画

### 8.1 sqlc.yaml への追加

PR3 / PR7 と同じ集約別パターンで、photobook 用 schema + queries を 3 つ目の sql エントリとして追加:

```yaml
- engine: "postgresql"
  schema:
    - "migrations/00001_create_health_check.sql"
    - "migrations/00002_create_sessions.sql"
    - "migrations/00003_create_photobooks.sql"
    # 00004 (sessions FK 追加) も schema 解析対象に含めると、sqlc が
    # 既存 sessions 型を再生成するか挙動確認が必要。
    - "migrations/00004_add_photobooks_fk_to_sessions.sql"
  queries: "internal/photobook/infrastructure/repository/rdb/queries"
  gen:
    go:
      package: "sqlcgen"
      out: "internal/photobook/infrastructure/repository/rdb/sqlcgen"
      sql_package: "pgx/v5"
```

PR3（database）/ PR7（auth/session）側の schema 指定にも `00003` / `00004` を追加するかは要検討:
- 追加する → `database/sqlcgen` にも `Photobook` 型が混入するリスク
- 追加しない → PR3 側はそのまま、PR7（sessions）側は FK 追加 migration が読めても問題なさそう

→ **PR9 で再度確認**（PR7 の経験則として「schema は当該 query が触るテーブルだけに絞る」が安全）。

### 8.2 sqlc query 一覧（最小）

`backend/internal/photobook/infrastructure/repository/rdb/queries/photobook.sql`:

| query | 内容 |
|---|---|
| `CreateDraftPhotobook` | INSERT、status='draft'、token hash 等 |
| `FindByID` | SELECT、id 一致 |
| `FindByDraftEditTokenHash` | SELECT、`draft_edit_token_hash = $1 AND status = 'draft' AND draft_expires_at > now()` |
| `FindByManageUrlTokenHash` | SELECT、`manage_url_token_hash = $1 AND status IN ('published','deleted') AND manage_url_token_hash IS NOT NULL` |
| `FindBySlug` | SELECT、`public_url_slug = $1 AND status = 'published'`（公開ページ用、PR9 では UseCase から呼ばないが query だけ用意） |
| `TouchDraft` | UPDATE、`draft_expires_at = $2` を更新（`status='draft' AND id=$1 AND version=$3` 楽観ロック） |
| `PublishFromDraft` | UPDATE、`status='published'` / `public_url_slug=$N` / `manage_url_token_hash=$N` / `manage_url_token_version=0` / `draft_edit_token_hash=NULL` / `draft_expires_at=NULL` / `published_at=now()` / `version=version+1` WHERE `id AND version=$expectedVersion AND status='draft'` |
| `ReissueManageUrl` | UPDATE、`manage_url_token_hash=$N` / `manage_url_token_version=manage_url_token_version+1` / `version=version+1` WHERE `id AND version=$expectedVersion AND status IN ('published','deleted')` |

すべての UPDATE 系は **`version = $expectedVersion` を WHERE に含める**（楽観ロック）。0 行の UPDATE は app 層で `OptimisticLockConflict` として返す。

### 8.3 Repository

`backend/internal/photobook/infrastructure/repository/rdb/photobook_repository.go`:

```go
type PhotobookRepository struct{ q *sqlcgen.Queries }
func NewPhotobookRepository(db sqlcgen.DBTX) *PhotobookRepository { ... }
```

メソッド:

- `CreateDraft(ctx, photobook) error`
- `FindByID(ctx, id) (Photobook, error)`
- `FindByDraftEditTokenHash(ctx, hash) (Photobook, error)`
- `FindByManageUrlTokenHash(ctx, hash) (Photobook, error)`
- `TouchDraft(ctx, id, newExpiresAt, expectedVersion) error`（0 行 UPDATE で `ErrOptimisticLockConflict`）
- `PublishFromDraft(ctx, id, slug, manageHash, expectedVersion) error`
- `ReissueManageUrl(ctx, id, newManageHash, expectedVersion) error`

### 8.4 marshaller

- `internal/photobook/infrastructure/repository/rdb/marshaller/photobook_marshaller.go`
- ドメインの `Photobook` ↔ sqlcgen `Photobook` row の変換
- VO ↔ プリミティブ変換は本パッケージに閉じる

### 8.5 Repository テスト方針

- 実 DB 必須（PR7 と同じ docker-compose の postgres を使う）
- テーブル駆動 + description + Builder
- 各テストで `TRUNCATE photobooks, sessions` してから開始
- query が `version = expectedVersion` を WHERE に含めることを 0 行 UPDATE で確認

---

## 9. UseCase 計画

### 9.1 PR9 で実装する UseCase（最小）

| UseCase | 入力 | 出力 | 副作用 |
|---|---|---|---|
| `CreateDraftPhotobook` | type / title / creator_display_name / now / draft_ttl | `(Photobook, raw DraftEditToken)` | photobooks INSERT |
| `TouchDraft` | photobook_id / now / expectedVersion | - | `draft_expires_at = now+7d` UPDATE |
| `ExchangeDraftTokenForSession` | raw DraftEditToken / now | `(SessionToken raw, photobook_id, draft_expires_at)` | photobooks SELECT + sessions INSERT |
| `ExchangeManageTokenForSession` | raw ManageUrlToken / now / manage_session_ttl | `(SessionToken raw, photobook_id, manage_url_token_version)` | photobooks SELECT + sessions INSERT |
| `PublishFromDraft` | photobook_id / now / expectedVersion / publishContext | `(Photobook, raw ManageUrlToken)` | photobooks UPDATE + RevokeAllDrafts、**同一 TX** |
| `ReissueManageUrl` | photobook_id / now / expectedVersion | `(Photobook, raw ManageUrlToken)` | photobooks UPDATE + RevokeAllManageByTokenVersion、**同一 TX** |

### 9.2 PR9 で実装しない UseCase（後続 PR）

- `GetPhotobookForEdit` / `GetPhotobookForManage` の閲覧系 — ページ / 写真 / メタが PR11+ で揃ってから
- `UpdateContent` / `SetTitle` / `SetVisibility` 等の編集操作
- `softDelete` / `restore` / `purge` / `hide` / `unhide`
- `agreeRights`（publish の前提だが、PR9 では `rights_agreed=true` を CreateDraft の引数で立てるか、固定値で publish 可とする簡略化を検討、§14）

### 9.3 PublishFromDraft の処理シーケンス（PR9 版、最小）

```
WithTx(pool):
  1. photobookRepo := NewPhotobookRepository(tx)
  2. sessionRepo := NewSessionRepository(tx)
  3. photobook := photobookRepo.FindByID(id)
  4. 状態検証（status==draft / rights_agreed==true / creator_display_name 非空）
  5. publicSlug := slugGen.Generate()
  6. manageRaw := manage_url_token.Generate()
  7. manageHash := manage_url_token_hash.Of(manageRaw)
  8. photobookRepo.PublishFromDraft(id, publicSlug, manageHash, expectedVersion)
     → 0 行 → OptimisticLockConflict
  9. revokeAllDrafts(sessionRepo, photobook_id)（PR8 既存 UseCase を Tx 起点で呼ぶ）
  10. （Outbox INSERT は PR12 以降）
  11. Commit → return (photobook, manageRaw)
```

### 9.4 ReissueManageUrl の処理シーケンス（同上）

```
WithTx(pool):
  1. photobookRepo := NewPhotobookRepository(tx)
  2. sessionRepo := NewSessionRepository(tx)
  3. photobook := photobookRepo.FindByID(id)
  4. 状態検証（status IN {published, deleted} / version==expected）
  5. oldVersion := photobook.manage_url_token_version
  6. newRaw := manage_url_token.Generate()
  7. newHash := manage_url_token_hash.Of(newRaw)
  8. photobookRepo.ReissueManageUrl(id, newHash, expectedVersion)
     → version +1, manage_url_token_version +1
  9. revokeAllManageByTokenVersion(sessionRepo, photobook_id, oldVersion)
  10. （ModerationAction は別 PR、ManageUrlDelivery も別 PR）
  11. Commit → return (photobook, newRaw)
```

### 9.5 ExchangeDraftTokenForSession の処理シーケンス

```
1. hash := draft_edit_token_hash.Of(rawDraftToken)
2. photobook := photobookRepo.FindByDraftEditTokenHash(hash)
   → 不一致 / 期限切れ / status!=draft → ErrInvalidDraftToken（401 相当）
3. now := time.Now()
4. expiresAt := photobook.draft_expires_at（draft session expires は draft_expires_at と同一、I-S7）
5. issueOut := IssueDraftSession.Execute(IssueDraftSessionInput{
       PhotobookID: photobook.id,
       Now: now,
       ExpiresAt: expiresAt,
   })
6. return (issueOut.RawToken, photobook.id, expiresAt)
```

draft 入場で `touchDraft` を **同時に行うか** はデザイン判断。設計書では「編集系 API 成功時に延長」なので、入場（GET 相当）では延長しない方針が安全。§14 で確認。

### 9.6 ExchangeManageTokenForSession の処理シーケンス

```
1. hash := manage_url_token_hash.Of(rawManageToken)
2. photobook := photobookRepo.FindByManageUrlTokenHash(hash)
   → 不一致 → ErrInvalidManageToken
3. version := photobook.manage_url_token_version
4. now := time.Now()
5. expiresAt := now + manage_session_ttl（24h〜7日、§14 で確定）
6. issueOut := IssueManageSession.Execute(IssueManageSessionInput{
       PhotobookID: photobook.id,
       TokenVersionAtIssue: token_version_at_issue.New(version),
       Now: now,
       ExpiresAt: expiresAt,
   })
7. return (issueOut.RawToken, photobook.id, version)
```

---

## 10. HTTP endpoint 計画

### 10.1 PR9 で本接続するエンドポイント（最小）

| エンドポイント | 目的 | 認証 | dummy 要素 |
|---|---|---|---|
| `POST /api/auth/draft-session-exchange` | raw `draft_edit_token` を渡して draft session 発行 | なし（token 自体が認証） | **なし** |
| `POST /api/auth/manage-session-exchange` | raw `manage_url_token` を渡して manage session 発行 | なし（token 自体が認証） | **なし** |

token は **必ず本物**（DB の hash と一致するもの）でなければ 401。dummy / 固定値の動作経路は本番 router に作らない。

### 10.2 リクエスト / レスポンス（提案）

```http
POST /api/auth/draft-session-exchange
Content-Type: application/json

{ "draft_edit_token": "<43 chars base64url>" }
```

成功時:
```http
HTTP/1.1 200 OK
Content-Type: application/json
Cache-Control: no-store

{
  "session_token": "<43 chars base64url>",
  "photobook_id": "<uuid>",
  "expires_at": "2026-05-03T00:00:00Z"
}
```

失敗時:
```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json
Cache-Control: no-store

{ "status": "unauthorized" }
```

### 10.3 raw session_token を body で返すことの整合

- `m2-session-auth-implementation-plan.md` §12.2 / §14.5 で「Set-Cookie 発行は Frontend Route Handler 側」を採用済
- そのため Backend は **raw session_token を body で返す**
- HTTPS（本番）/ localhost（ローカル）以外では呼べない（CORS + Secure 属性 + 同オリジンの組合せで担保、PR10 で実機検証）
- Backend からは `Set-Cookie` を出さない（出すと Frontend Route Handler の Cookie Domain 制御と二重になる、M1 学習）

### 10.4 PR9 で **作らない** エンドポイント

- `POST /api/photobooks` (createDraft の HTTP 化) — 編集 UI が無いため UseCase の単体テストでカバー、HTTP 化は PR11 以降
- `POST /api/photobooks/{id}/publish` — 同上
- `POST /api/photobooks/{id}/reissue-manage-url` — Moderation 集約が無いため、PR9 では **CLI / cmd/ops** からの呼び出しは未実装、UseCase の単体テストでカバー
- `POST /api/auth/sessions/revoke`（明示破棄） — PR10 以降
- `GET /api/photobooks/{id}` 等の閲覧系 — Page / Photo が PR11 で入ってから

### 10.5 本番 router 接続範囲（最小）

`internal/http/router.go` に以下を追加:

```go
r.Post("/api/auth/draft-session-exchange", draftExchangeHandler)
r.Post("/api/auth/manage-session-exchange", manageExchangeHandler)
```

protected route のプレースホルダを **作るか作らないか** は §14 で確認。作る場合の例:

```go
r.With(sessionMiddleware.RequireDraftSession(validator, extractor)).
  Get("/api/photobooks/{id}/_session-check", sessionCheckHandler)
```

`/_session-check` は **デバッグ用 / 内部確認用**。PR10 の Safari 実機検証で session が成立しているか確認するために有用。ただし「dummy endpoint を作らない」原則と緊張するので、入れる場合は:

- response body は `{"draft_session_active": true}` のような最小情報のみ
- 本番デプロイ時は環境変数で除外可能にする
- もしくは PR10 で frontend route と一緒に追加する

→ §14 で確認、推奨: **PR9 では入れず、PR10 で入れる**。

---

## 11. テスト方針

### 11.1 全体方針（`testing.md` 準拠）

- テーブル駆動 + `description` + Builder
- レイヤー別:

| レイヤー | テスト | DB |
|---|---|---|
| VO（draft_edit_token / manage_url_token / hash 等） | 生成 / Parse / 衝突 / hash 一致 | 不要 |
| Photobook entity | コンストラクタ不変条件 / 状態遷移メソッド | 不要 |
| UseCase | フロー検証、Repository mock / fake | 不要 |
| Repository | sqlc query / CHECK / 楽観ロック 0 行 UPDATE | 必要 |
| TX 統合（publishFromDraft + RevokeAllDrafts） | 同一 TX での原子性 / ロールバック | 必要 |
| HTTP handler | リクエスト / レスポンス（httptest）、validator は実物 | UseCase が実物の場合は必要 |

### 11.2 重点テストケース

**Photobook domain**:
- createDraft の不変条件（status=draft / draft_edit_token_hash 必須 / draft_expires_at 必須 / manage_url_token_hash NULL / public_url_slug NULL）
- publish 状態遷移（status=draft → status=published で各カラムが期待通り）
- reissueManageUrl で `manage_url_token_version +=1`
- 楽観ロック: 古い version で UPDATE → 0 行 → エラー

**Token / Hash**:
- draft_edit_token: 32B 乱数 / 43 文字 / 衝突なし 1000 回
- manage_url_token: 同上
- hash: idempotent / SHA-256 一致

**Repository**:
- CreateDraft / FindByDraftEditTokenHash / FindByManageUrlTokenHash の正常系
- FindByDraftEditTokenHash で `draft_expires_at < now()` の draft はヒットしない
- PublishFromDraft で 0 行（version 不一致） → ErrOptimisticLockConflict
- ReissueManageUrl で version +1
- CHECK 制約: draft で manage_url_token_hash != NULL は INSERT 拒否

**UseCase**:
- ExchangeDraftTokenForSession: 不一致トークン → ErrInvalidDraftToken
- ExchangeDraftTokenForSession: 期限切れ draft → ErrInvalidDraftToken
- ExchangeManageTokenForSession: 不一致 → エラー
- ExchangeManageTokenForSession: deleted 状態の photobook も manage は使える
- PublishFromDraft: status≠draft の photobook → エラー
- PublishFromDraft: 成功時に **draft session が revoke される**（fake または実 DB）
- ReissueManageUrl: 成功時に **旧 version の manage session が revoke される、新 version は影響なし**

**TX 統合**:
- publishFromDraft の途中で session repo がエラーを返した場合、photobook 側の UPDATE もロールバックされる
- reissueManageUrl の途中で session repo がエラー → photobook も巻き戻る
- どちらも実 DB で確認

**HTTP handler**:
- 200 成功（form: 正しい token）
- 401 不正 token
- 401 期限切れ
- response body に raw session_token が乗る（Cache-Control: no-store ヘッダ確認）
- Set-Cookie ヘッダが **付かない**ことを確認（Frontend が出す方針との整合）
- リクエスト body の draft_edit_token がログに出ない（middleware の slog handler は raw を出さない経路で動かす）

### 11.3 テスト用 Builder

- `internal/photobook/domain/tests/photobook_builder.go`
- 既定値: status=draft / type=memory / layout=simple / opening_style=light / visibility=unlisted / rights_agreed=true（テスト便宜）/ creator_display_name="Tester"

### 11.4 raw token / Cookie 値をテストログに出さない

- t.Log / t.Errorf に raw を入れない
- middleware test の grep（PR8 で実装済の方針）を Photobook handler test にも適用

---

## 12. セキュリティ確認

### 12.1 ログ・露出禁止チェックリスト

- [ ] raw `draft_edit_token` をログ・diff・コミットメッセージ・スクリーンショット・テストログに出さない
- [ ] raw `manage_url_token` 同上
- [ ] raw `session_token` 同上（PR7-8 で確立、PR9 で再確認）
- [ ] `draft_edit_token_hash` / `manage_url_token_hash` をログに出さない
- [ ] `Cookie:` / `Set-Cookie:` ヘッダ全体をログに出さない
- [ ] `Authorization` ヘッダ同上
- [ ] エラーメッセージにも上記を含めない（panic / err.Error() / wrap 文字列）
- [ ] HTTP handler のリクエスト body （raw token を含む JSON）をログに出さない
- [ ] response body に出すのは raw session_token のみ。draft / manage token を間違って混入させない

### 12.2 認可ガード

- [ ] DB にハッシュのみ保存、raw 保存 0
- [ ] 期限切れ draft は Find* でヒットしない（query 条件で `draft_expires_at > now()`）
- [ ] revoked manage session は Session middleware で 401（PR8 既存）
- [ ] draft / manage の取り違え検証（hash query が status / 列を強制するため、DB レベルで取り違え不可）
- [ ] deleted / purged photobook へのアクセスは Find* で除外

### 12.3 dummy / バイパス防止

- [ ] dummy token で動く endpoint を作らない
- [ ] 認証バイパス flag を本番コードに作らない
- [ ] テスト用の固定 token は **テストファイル内のみ**、本番コードからは見えない（`tests/` ディレクトリ内 + `_test.go` のみ）
- [ ] `_session-check` 等の確認 endpoint は **PR9 では作らない**（PR10 で frontend route と一緒に追加検討）

### 12.4 Frontend との接続前提（PR10 で再検証）

- [ ] Backend は `Set-Cookie` を出さない（response body で raw session_token を返すのみ）
- [ ] Frontend Route Handler が Cookie Domain `.<domain>` で Cookie を発行する（PR10、ドメイン取得後）
- [ ] localhost では Cookie Domain 未設定（host-only）

---

## 13. PR 分割案

### 13.1 推奨: 3 分割

| PR | 内容 | 完了条件 |
|---|---|---|
| **PR9a** | Photobook **domain + token VO + migration + repository + 単体テスト** | photobooks table + sessions FK 追加で goose up/down 動作 / VO + repository テスト合格（実 DB） / 楽観ロック 0 行 UPDATE 検証 / **HTTP / UseCase はまだ未着手** |
| **PR9b** | UseCase + **同一 TX での Session revoke 接続** + 統合テスト | CreateDraftPhotobook / TouchDraft / PublishFromDraft / ReissueManageUrl / ExchangeDraft* / ExchangeManage* の UseCase 完成、TX 統合テスト（実 DB）で「publish で draft session が revoke される」「reissue で旧 manage session が revoke される」が確認できる |
| **PR9c** | HTTP endpoint 接続 + 本番 router 接続 + handler test | `POST /api/auth/draft-session-exchange` / `POST /api/auth/manage-session-exchange` を本番 router に接続、httptest で 200 / 401 を網羅、Cookie 値・raw token がログ / response に漏れない |

### 13.2 1 PR でやる場合のリスク

- diff が大きく（おそらく 2,000〜3,500 行）レビューが追えない
- TX 境界 / 楽観ロック / Session 連携 / HTTP それぞれの問題が混ざり、原因切り分けが困難
- セキュリティ確認チェックリストが膨大化し、漏れリスクが上がる

### 13.3 推奨理由

- PR9a 完了時点で「Photobook の状態モデル」がレビュー可能（最も設計判断の多い箇所）
- PR9b で TX 統合という単一テーマに集中できる（PR9a が終わっていれば、TX を組むだけ）
- PR9c は HTTP 周りに絞れる（PR9b までで UseCase 動作が確認済なので、HTTP の薄い接続のみ）

### 13.4 各 PR の所要規模見積（参考）

- PR9a: 〜1,200 行（migration + VO 5 種 + Photobook entity + repository + sqlc + tests）
- PR9b: 〜800 行（UseCase 6 種 + Session adapter + TX runner + 統合テスト）
- PR9c: 〜400 行（handler + router 接続 + httptest）

---

## 14. ユーザー判断事項

PR9a 着手前に以下を確認してください。**※ 各項目に推奨案を併記** しています。

### 14.1 PR9 を分割するか

- [ ] **3 分割 (PR9a / 9b / 9c)**（推奨、§13.1）
- [ ] 1 PR で全部
- [ ] 2 分割（PR9a 統合 + PR9b 統合 etc）

### 14.2 token 長を 256bit (32B) に統一するか

- [ ] **統一する**（推奨、Session と同実装パターン、§5.2）
- [ ] 業務知識通り 128bit (16B) にする

### 14.3 manage session TTL の既定値

- [ ] **7 日**（推奨、設計書 I-S8 上限、`m2-session-auth-implementation-plan.md` §6.2）
- [ ] 24 時間
- [ ] 環境変数で切替可能にする

### 14.4 ExchangeDraftTokenForSession で touchDraft を呼ぶか

- [ ] **呼ばない**（推奨、入場は GET 相当、設計書 §6.4 / I-D4「編集系 API 成功時のみ延長」、§9.5）
- [ ] 呼ぶ（入場も延長対象にする）

### 14.5 Outbox INSERT を本 PR に含めるか

- [ ] **含めない**（推奨、Outbox table 自体が後続 PR、§7.1）
- [ ] 含める（PR9 で `outbox_events` table も同時に作る）

### 14.6 publish 条件 `rights_agreed` の扱い

- [ ] **CreateDraft の引数で受け取り、テストでは true 固定**（推奨、編集 UI が無いため簡略化、§9.2）
- [ ] PR9 で agreeRights UseCase を別途実装する

### 14.7 `_session-check` 等のデバッグ endpoint を入れるか

- [ ] **入れない、PR10 で frontend route と一緒に追加**（推奨、§10.5 / §12.3）
- [ ] PR9c で `/api/photobooks/{id}/_session-check` を入れる（環境変数で切替可能）

### 14.8 reissueManageUrl の HTTP 化を本 PR に含めるか

- [ ] **HTTP 化しない、UseCase の単体テストのみ**（推奨、Moderation 集約が無いため、`cmd/ops` も後続 PR、§10.4）
- [ ] HTTP endpoint を作って Postman / curl で動かせるようにする

### 14.9 sqlc.yaml の schema 指定方針

- [ ] **集約別に schema を絞る**（推奨、PR7 と同パターン、§8.1）
- [ ] 全 schema を全エントリで読む（型混入を許容）

### 14.10 ドメイン購入タイミングの再確認

- [ ] PR9a/9b/9c 全完了後、PR10 着手前に購入（推奨、`m2-session-auth-implementation-plan.md` §14.8）
- [ ] PR9a 完了時点で購入
- [ ] さらに後で

---

## 15. 実施しないこと（再掲）

本計画書 + PR9a/9b/9c では **以下を実施しない**:

- Photobook 実装コード作成（PR9a で実施）
- migration 作成（PR9a で実施）
- sqlc query 作成（PR9a で実施）
- HTTP endpoint 作成（PR9c で実施）
- frontend route handler 作成（PR10 で実施）
- Image / Page / Photo aggregate / R2 / Turnstile / SendGrid / Outbox / ManageUrlDelivery / Moderation / Report 実装（後続 PR）
- Cloud SQL 作成 / Cloud Run deploy / Cloud Run Jobs / Cloud Scheduler / Workers deploy
- 独自ドメイン購入 / Cloudflare DNS 変更 / Workers Custom Domain / Cloud Run Domain Mapping
- 既存リソース削除

---

## 16. 関連ドキュメント

- [Photobook ドメイン設計](../design/aggregates/photobook/ドメイン設計.md)
- [Photobook データモデル設計](../design/aggregates/photobook/データモデル設計.md)
- [Session ドメイン設計](../design/auth/session/ドメイン設計.md) / [Session データモデル設計](../design/auth/session/データモデル設計.md)
- [ADR-0003 frontend token-session flow](../adr/0003-frontend-token-session-flow.md)
- [M2 Session auth 実装計画](./m2-session-auth-implementation-plan.md)
- [M2 早期ドメイン + Cookie 計画](./m2-early-domain-and-cookie-plan.md)
- [M2 実装ブートストラップ計画](./m2-implementation-bootstrap-plan.md)
- [プロジェクト全体ロードマップ](../../harness/work-logs/2026-04-26_project-roadmap-overview.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`safari-verification.md`](../../.agents/rules/safari-verification.md) / [`testing.md`](../../.agents/rules/testing.md) / [`domain-standard.md`](../../.agents/rules/domain-standard.md)
