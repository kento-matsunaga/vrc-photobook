# /manage MVP 安全性アクション計画書（M-1a / M-1b 二段構成）

> **状態**: STOP α 計画書（**実装未着手**）。本書は M-1a を先行実装、M-1b を別 STOP で実装する
> 二段構成を前提として固定する。本書 commit 後に M-1a 計画書 leg として user 承認を取り、
> 実装着手は別 STOP で別途承認する。

---

## 0. メタ情報

| 項目 | 値 |
|---|---|
| 起点 | `4f50746 feat(design): polish public static page readability`（PR37 final visual polish 完了状態） |
| 想定スコープ | `/manage` の MVP 安全性アクション拡張、二段構成（M-1a 安全 baseline / M-1b 破壊系） |
| 関連 spec | 業務知識 v4 §3.4 / §3.5 / §6.13 / §6.18 / §6.19 |
| 関連 ADR | ADR-0002（運営操作 cmd/ops 方式） / ADR-0003（token→session）/ ADR-0006（Email Provider 再選定中） |
| 関連 plan | [`m2-public-viewer-and-manage-plan.md`](./m2-public-viewer-and-manage-plan.md)（manage 初期実装、本書はその拡張） |
| 関連 runbook | [`docs/runbook/ops-moderation.md`](../runbook/ops-moderation.md)（hide / unhide の運営手順） |
| 出力 | docs only（本書 1 ファイル）、実装・migration・deploy なし |

### 0.1 本書 commit 時点の Open Questions 仮決定（user 確定済）

| # | 項目 | 仮決定 |
|---|---|---|
| Q1 | 公開停止と削除の関係 | **分ける**（unpublish と soft_delete を別 mutation として持つ） |
| Q2 | 作成者による soft_delete からの restore | **提供しない**（運営問い合わせ経由のみ。`cmd/ops photobook restore`（M-1b 範囲内 or 後続）で運営手動） |
| Q3 | manage session から draft session への昇格 | **許可する方向で設計**。M-1a では「編集を再開」導線の仕様（promote endpoint 仕様 + UI 文言）を固める。実装は M-1a と M-1b のいずれに含めるか §3.2.5 で判断 |
| Q4 | 破壊系 mutation の confirm 方式 | **Backend 発行 short-lived confirm token**（30 秒〜2 分有効、破壊系 mutation の input に必須）。**M-1b 範囲** |
| Q5 | unpublish の表現方法 | **`photobooks.hidden_by_owner: bool` 列追加**（status enum は既存 4 値 `draft / published / deleted / purged` を不変維持）。M-1b で migration |
| Q6 | `/manage` から visibility public への変更 | **不許可**（unlisted ⇄ private のみ）。public 化は publish 時のみ |
| Q7 | Slug 復元 rule の確定 | **M-1b plan で確定**（DB CHECK は既に `status IN ('published', 'deleted')` で UNIQUE 維持済、§6.18 のドメイン責務を M-1b の `SoftDeletePhotobook` usecase で具現化） |
| Q8 | 実装順序 | **M-1a → M-1b の順、別 STOP**。M-1a で deploy / smoke 完了後、M-1b の plan を別途切る |

---

## 1. 業務知識との整合

### 1.1 §3.4「フォトブック管理機能」との対応

§3.4 の「この機能が担うこと」リストに対する M-1a / M-1b の網羅状況:

| spec 項目 | 現状 | M-1a | M-1b | 残 |
|---|:-:|:-:|:-:|:-:|
| 公開 URL と管理 URL の表示 | ✓ | – | – | – |
| 管理 URL コピー | ✓（公開 URL のみ。管理 URL は再表示禁止 §3.4 ） | – | – | – |
| フォトブック内容編集の導線 | ✗ | ✓ §3.2.5 で設計確定 | – | – |
| 公開範囲の変更 | ✗ | ✓（unlisted ⇄ private） | – | public 化は範囲外（Q6） |
| センシティブ設定の変更 | ✗ | ✓ § §3.2.4 | – | – |
| フォトブックの削除 | ✗ | – | ✓ §4 | – |
| X 共有投稿文の再生成・再コピー | ✗ | – | – | M-2 / 後続 |
| 管理 URL 控えメール送信（ManageUrlDelivery） | ✗ MVP 範囲外 | – | – | M-3（ADR-0006 後続） |
| 「この端末から管理権限を削除」明示破棄 UI | ✗ | ✓ §3.2.3 | – | – |
| 「管理 URL は他人共有禁止」注意喚起 | ✗ | ✓ §3.2.1 | – | – |
| 破壊的操作の二重確認 + ワンタイム確認 token | n/a | – | ✓ §4.4 | – |

### 1.2 §3.5「管理 URL 控え機能」との対応

- `ManageUrlDelivery` 集約は ADR-0006 後続（**M-3 / M-1 範囲外**）
- M-1a / M-1b では `recipient_email` を扱わない

### 1.3 §6.18「Slug 復元ルール」との対応

DB の CHECK 制約は既に以下を担保:

```sql
CONSTRAINT photobooks_status_columns_consistency_check CHECK (
    CASE status
        WHEN 'deleted' THEN ... AND deleted_at IS NOT NULL  -- §6.18 deleted 整合
        ...
    END
);
CREATE UNIQUE INDEX photobooks_public_url_slug_uniq
    ON photobooks (public_url_slug)
    WHERE status IN ('published', 'deleted');  -- §6.18 deleted で slug 保持・unique
```

**M-1b の `SoftDeletePhotobook` usecase は既存制約を活用**:
- status 'published' → 'deleted' 遷移、`deleted_at = now` セット
- `public_url_slug` / `manage_url_token_hash` / `manage_url_token_version` / `published_at` は不変
- migration での schema 変更は **`hidden_by_owner` 列追加のみ**（unpublish 用、§4.2）
- `purge` / `restore`（status 'deleted' → 'published' 戻し）は M-1 範囲外、運営 cmd/ops で別 PR

### 1.4 §6.19「運営操作は HTTP API 化しない」との関係

- M-1a / M-1b は **作成者の管理 URL session で動く mutation のみ**（運営操作ではない）
- `/manage` UI は作成者が自分の photobook を操作する用途で、§6.19 の制約に抵触しない
- 運営による hide / unhide / purge / restore は引き続き `cmd/ops photobook *` のみ

### 1.5 認可境界（draft session vs manage session）

`backend/migrations/00002_create_sessions.sql` の制約:

```sql
CONSTRAINT sessions_session_type_check CHECK (session_type IN ('draft', 'manage'))
```

- draft session: `/api/photobooks/{id}/edit-view` 等の編集経路で必要（middleware `RequireDraftSession`）
- manage session: `/api/manage/photobooks/{id}` 等の管理経路で必要（middleware `RequireManageSession`）
- **session_type は固定列**で promote はできない。Q3 の「昇格」は **新規 draft session 発行**を意味する

---

## 2. 既存認可境界の整理（M-1a / M-1b 設計の前提）

### 2.1 manage session の発行経路

```
POST /api/auth/manage-session-exchange   (manage_url_token を一度だけ受け取って交換)
  ↓
session 発行 (session_type='manage', token_version_at_issue=manage_url_token_version)
  ↓
Set-Cookie: vrcpb_manage_<photobook_id>=<session_token>
```

### 2.2 draft session の発行経路（既存）

```
POST /api/auth/draft-session-exchange    (draft_edit_token を一度だけ受け取って交換)
  ↓
session 発行 (session_type='draft', token_version_at_issue=0)  ※ I-S5 / CHECK
  ↓
Set-Cookie: vrcpb_draft_<photobook_id>=<session_token>
```

### 2.3 manage→draft 昇格（Q3、新規設計）

| 観点 | 設計判断 |
|---|---|
| なぜ必要か | 公開後の修正手段。作成者が draft URL を保持していない場合の救済 |
| spec 整合 | §3.4「フォトブック内容編集の導線を提供する」を満たす |
| 認可 | manage Cookie session で認可（既存 `RequireManageSession`）→ 検証成功なら draft session を発行 |
| 制約 | published / deleted のフォトブックは原則 draft 化しない（業務知識 v4 で publish 後 draft 戻しは未定義）。<br>**設計案 A**: published photobook の編集には新たな draft session を「published 同 photobook 用」として発行（draft_edit_token は再発行しない）。<br>**設計案 B**: 編集機能を `/manage` 内に直接実装し draft session を介さない。 |
| 推奨 | **設計案 A**（既存 /edit UI を流用、editor 側の処理は version OCC と既存 status check で守る） |
| token_version_at_issue | session_type='draft' のときは 0 固定（既存 CHECK）→ 新発行 draft session は draft_edit_token に紐付かない「manage 由来 draft session」として発行（**新仕様**） |
| 抜け道 | manage_url 漏洩 → draft 編集まで全部できる（既存も同等、§6.13 で「draft / manage 漏洩リスクは同等」と明文化済） |
| 実装 endpoint 仕様 | `POST /api/auth/issue-draft-from-manage`（manage Cookie 必須、photobook_id を URL or body で渡す、photobook の status が published/deleted でも 200 で draft session 発行）|

### 2.4 認可境界の M-1a / M-1b 影響

| 操作 | 必要 session | M-1 内訳 |
|---|---|---|
| 公開停止 / 削除 | manage | M-1b |
| visibility / sensitive 変更 | manage | **M-1a**（既存 manage session で完結） |
| この端末から権限削除 | manage（自分自身を revoke） | **M-1a** |
| 編集を再開（draft 発行） | manage（issue-draft-from-manage） | M-1a で endpoint + UI を実装、または M-1a で UI ガイドだけ・実装は M-1b 以降に分離（§3.2.5 で確定） |

---

## 3. M-1a: Manage safety baseline（先行 STOP）

### 3.1 scope

**含む**:
- 常設注意喚起（「管理 URL は他人共有禁止」「漏洩したら運営問い合わせ」）
- 「編集を再開」導線（§3.2.5 で詳細仕様確定、実装方針 A: endpoint 含む / B: UI ガイドのみ から user 承認時に確定）
- session revoke（この端末から管理権限を削除）
- visibility 変更（unlisted ⇄ private のみ、public 化禁止）
- sensitive 切替（ON / OFF）

**含まない（M-1b 以降）**:
- 公開停止（hidden_by_owner）
- soft_delete
- short-lived confirm token（M-1b で破壊系操作とセット）
- ManageUrlDelivery 系 / 再発行
- OGP 再生成
- moderation 履歴の作成者向け表示
- public 化

### 3.2 Backend endpoint / usecase 仕様

#### 3.2.1 既存（不変）

- `GET /api/manage/photobooks/{id}` — read-only

#### 3.2.2 新 endpoint: visibility 変更

```
PATCH /api/manage/photobooks/{id}/visibility
Cookie: <manage session>
Body: {"visibility": "unlisted" | "private", "expected_version": <int>}

成功 200: {"version": <new>}
失敗 409: {"status":"version_conflict"}
失敗 409: {"status":"manage_precondition_failed","reason":"public_change_not_allowed"}  ← public 指定
失敗 409: {"status":"manage_precondition_failed","reason":"not_published"}              ← published でない（draft / deleted 等）
失敗 401 / 403: 既存 middleware
```

- UseCase: 新規 `UpdatePhotobookVisibilityFromManage`
- 同 TX で `photobooks.visibility = $new`、version+1、`updated_at = $now`
- 既存 OCC 規約（§domain-standard 集約子テーブル更新ルール）に準拠
- public 指定は handler で 409 + reason="public_change_not_allowed" にして UseCase に到達させない

#### 3.2.3 新 endpoint: sensitive 切替

```
PATCH /api/manage/photobooks/{id}/sensitive
Cookie: <manage session>
Body: {"sensitive": <bool>, "expected_version": <int>}

成功 200: {"version": <new>}
失敗 409: {"status":"version_conflict"}
失敗 409: {"status":"manage_precondition_failed","reason":"not_published"}
```

- UseCase: 新規 `UpdatePhotobookSensitiveFromManage`
- 同 TX で `photobooks.sensitive = $new`、version+1

#### 3.2.4 新 endpoint: session revoke (this device)

```
POST /api/manage/photobooks/{id}/session-revoke
Cookie: <manage session>

成功 200: {"status":"ok"} + Set-Cookie でこの session Cookie を削除
失敗 401: 既存 middleware（呼ぶ前に session が無ければ unauthorized）
```

- UseCase: 既存 `auth/session/RevokeSession` を流用
- middleware が session_token_hash を context に入れているなら、その session のみ revoke
- `Set-Cookie: vrcpb_manage_<id>=; Max-Age=0; ...` で Cookie 自体を消す
- **「全端末から revoke」ではなく自分の Cookie の session のみ revoke**（共有 PC 用途）

#### 3.2.5 「編集を再開」導線（Q3、要 user 判断）

実装方針を 2 案で提示し、本書 commit 後の M-1a 着手前 STOP α で確定:

| 案 | 実装内容 | 利点 | 欠点 |
|---|---|---|---|
| **A. endpoint 込み（M-1a 範囲）** | `POST /api/auth/issue-draft-from-manage` を新設、manage Cookie で認証して draft session を発行、`/edit/{id}` に redirect | 1 click で編集に戻れる、UX ◎ | 認可境界が広がる（manage→draft 昇格を許可、§2.3）。abuse risk 議論必要 |
| **B. UI ガイドのみ（M-1a は UI、endpoint は別 STOP）** | manage panel に「編集を再開するには？」FAQ 風セクション、Help link で /help/manage-url Q7（新設）に誘導 | 認可境界を変えない、安全 | UX ✗（user は draft URL を別途保持必須、現実的に困難） |

**推奨**: **A** をセットで実装する。理由:
- spec §3.4 「フォトブック内容編集の導線を提供する」を本質的に満たすには endpoint 必要
- §6.13 で「manage URL 漏洩リスク = draft URL 漏洩リスク」と明文化済 → 認可境界拡張は許容
- abuse 抑止は既存の rate limit + Turnstile で十分（§6 abuse risk）

ただし user が認可境界拡張に懸念を示す場合は B → 後続 STOP で A に格上げする。

#### 3.2.6 router 追加（`backend/internal/http/router.go`）

```go
// 既存 manage route block に追記
sub.Patch("/visibility",       cfg.PhotobookManageHandlers.UpdateVisibility)
sub.Patch("/sensitive",        cfg.PhotobookManageHandlers.UpdateSensitive)
sub.Post("/session-revoke",    cfg.PhotobookManageHandlers.RevokeSession)
// 案 A 採用時のみ:
// /api/auth route block に追記
// auth.Post("/issue-draft-from-manage", cfg.AuthHandlers.IssueDraftFromManage)
```

#### 3.2.7 CORS

- 全 mutation（PATCH / POST）は `cors.go` の `AllowedMethods` に既に含まれる（PR12 / PR27 系で対応済）→ **CORS 変更不要**
- 新 endpoint は既存 origin / credentials の組合せで動作（変更なし）

### 3.3 Frontend UI 仕様

#### 3.3.1 ManagePanel への追加要素

- **常設注意喚起バナー**（panel 上部、`HiddenByOperatorBanner` の info トーン版）
  - 文言: 「管理 URL は他人と共有しないでください。漏洩・紛失時は X で運営にお問い合わせください。」
  - 「Help を見る」link（→ `/help/manage-url`）
- **編集を再開セクション**（案 A 採用時）:
  - 「編集を再開」ボタン → `POST /api/auth/issue-draft-from-manage`（fetch + Cookie include）→ 成功時 `/edit/{id}` に遷移
  - 失敗時の文言（401 / 409 / network）
- **公開設定セクション**:
  - visibility radio: `unlisted` / `private`（`public` は disabled + 「公開時のみ設定可能」注記）
  - sensitive toggle
  - 「公開設定を保存」ボタン → 確認 modal なし（破壊操作ではないので OCC で十分）
- **session revoke セクション**:
  - 「この端末から管理権限を削除」ボタン
  - 確認 modal（「本当に？別端末からは引き続きアクセスできます」）
  - 成功時 `/` または `/manage/{id}?reason=session_revoked` に redirect
- **管理リンクの再発行**:
  - 既存の disabled placeholder は **そのまま維持**（M-3 ADR-0006 後続で活性化）

#### 3.3.2 component 構成

| 新規 / 既存 | component | 役割 |
|---|---|---|
| 新規 | `ManageUrlNoticeBanner` | 「他人共有禁止」常設バナー（info トーン） |
| 新規 | `ManageEditResumeButton` | 「編集を再開」ボタン + endpoint 呼出（案 A） |
| 新規 | `ManageVisibilityForm` | radio + sensitive toggle + 保存ボタン |
| 新規 | `ManageSessionRevokeButton` | session revoke ボタン + 確認 modal |
| 既存 | `HiddenByOperatorBanner` | 維持 |
| 既存 | `ManagePanel` | 上記 4 component を統合 |
| 既存 | `UrlRow` | 維持 |

#### 3.3.3 lib 追加（`frontend/lib/managePhotobook.ts` 拡張）

```ts
export async function updatePhotobookVisibilityFromManage(
  photobookId: string,
  visibility: "unlisted" | "private",
  expectedVersion: number,
  signal?: AbortSignal,
): Promise<{ version: number }>;

export async function updatePhotobookSensitiveFromManage(
  photobookId: string,
  sensitive: boolean,
  expectedVersion: number,
  signal?: AbortSignal,
): Promise<{ version: number }>;

export async function revokeManageSession(
  photobookId: string,
  signal?: AbortSignal,
): Promise<void>;

// 案 A 採用時:
export async function issueDraftSessionFromManage(
  photobookId: string,
  signal?: AbortSignal,
): Promise<void>;
```

すべて Client Component 経由（`credentials: "include"`）。`client-vs-ssr-fetch.md` rule 準拠。

### 3.4 test 方針（M-1a）

#### 3.4.1 Backend 単体

| 観点 | test |
|---|---|
| visibility 正常 unlisted ↔ private | `UpdatePhotobookVisibilityFromManage` UseCase test、handler test |
| visibility public 拒否 | handler test で 409 + reason=public_change_not_allowed |
| visibility published 以外で拒否 | handler test で 409 + reason=not_published |
| OCC violation | 409 + reason=version_conflict |
| sensitive 同等 | 同パターン |
| session revoke 自 session のみ | usecase test、Cookie 削除 header 確認 |
| 認可なしで 401 | middleware test |

#### 3.4.2 Frontend 単体

| 観点 | test |
|---|---|
| ManageUrlNoticeBanner 表示 | render test |
| ManageVisibilityForm public 不可 | render test で public radio が disabled |
| ManageVisibilityForm OCC エラー時の文言 | render test |
| ManageSessionRevokeButton 確認 modal | render + click test |
| `updatePhotobookVisibilityFromManage` lib | fetch mock test |
| ManageEditResumeButton（案 A） | fetch mock test |

#### 3.4.3 統合（既存の public-pages.test.tsx は対象外、Manage は SSR + render test のみ）

### 3.5 影響まとめ（M-1a）

| 範囲 | 変更 |
|---|---|
| DB migration | **なし**（既存 column / VO で完結） |
| Backend domain | 新規 method（`Photobook.UpdateVisibilityFromManage` / `UpdateSensitiveFromManage`）追加、status 遷移なし |
| Backend usecase | 3〜4 新規（visibility / sensitive / session revoke / 案 A: issue-draft-from-manage） |
| Backend handler | 同上 + router |
| Backend test | 各 usecase / handler の単体（既存パターン踏襲） |
| Frontend lib | `managePhotobook.ts` に 3〜4 関数追加 |
| Frontend component | 4 新規 + ManagePanel 改修 |
| Frontend test | 各 component / lib の単体 |
| design token | 不変 |
| CORS / Secret / env / binding | 不変 |
| deploy | Backend + Workers 両方（mutation 追加のため） |

---

## 4. M-1b: Destructive actions（後続 STOP）

### 4.1 scope

**含む**:
- 公開停止（owner-initiated hide）
- soft_delete
- short-lived confirm token
- migration（`hidden_by_owner` 列追加）
- domain rule 拡張（hidden_by_owner / status 'deleted' 遷移）
- rollback / smoke 強化

**含まない（M-1 範囲外、後続）**:
- restore（運営 cmd/ops 経由のみ）
- purge（保持期間後の運営 cmd/ops 経由のみ）
- ManageUrlDelivery（M-3）
- OGP 再生成

### 4.2 DB migration

新規 migration ファイル: `backend/migrations/00019_add_photobooks_hidden_by_owner.sql`

```sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE photobooks
    ADD COLUMN hidden_by_owner boolean NOT NULL DEFAULT false;

-- 公開ページ可視性の判定 index（既存に追加）
CREATE INDEX photobooks_published_visible_idx
    ON photobooks (status, hidden_by_operator, hidden_by_owner)
    WHERE status = 'published';
-- +goose StatementEnd

-- +goose Down
DROP INDEX photobooks_published_visible_idx;
ALTER TABLE photobooks DROP COLUMN hidden_by_owner;
```

**注意**:
- 既存 row はすべて `hidden_by_owner=false` で初期化（`DEFAULT false NOT NULL`）→ row level 影響なし
- 公開可視性判定（Public Viewer の `is_visible` 計算）に `hidden_by_owner` を OR 条件で追加する変更が必要 → **既存 `get_public_photobook.go` の query 拡張も M-1b に含む**
- migration 適用順は forward-only、rollback は `00019` の Down で `hidden_by_owner` 列削除

### 4.3 domain rule 拡張

| domain | 追加 |
|---|---|
| `Photobook` entity | `hidden_by_owner: bool` 属性追加、`HideByOwner()` / `UnhideByOwner()` method、`IsPubliclyVisible()` を `!hidden_by_operator && !hidden_by_owner` に拡張 |
| `Photobook` entity | `SoftDelete(now)` method（status='deleted' 遷移、`deleted_at = now`、§6.18 整合） |
| `PhotobookStatus` VO | `IsDeleted()` method を新規追加（既存 status enum は不変） |
| Slug 復元 invariant | 既存 DB UNIQUE INDEX で担保（`status IN ('published', 'deleted')` で unique） |

### 4.4 short-lived confirm token

破壊系（unpublish / soft_delete）の前段に Backend 発行 short-lived confirm token を必須化。

#### 4.4.1 endpoint

```
POST /api/manage/photobooks/{id}/confirm-token
Cookie: <manage session>
Body: {"action": "unpublish" | "soft_delete"}

成功 200: {"confirm_token": "<random base64>", "expires_at": "<ISO8601>"}
```

- 有効期限 60 秒（M-1b plan で確定）
- `confirm_tokens` テーブル新設 or in-memory 保持（Cloud Run 多 instance だと in-memory は壊れる → DB 保持必須、§4.4.3 で詳細）
- token は使い捨て（消費後 DB から削除）

#### 4.4.2 破壊系 mutation 側で必須化

```
POST /api/manage/photobooks/{id}/unpublish
DELETE /api/manage/photobooks/{id}                    (soft_delete)

Body: {"confirm_token": "<...>", "expected_version": <int>}
```

- 不一致 / 期限切れ / 既消費 → 400 + reason="invalid_confirm_token"
- 不一致は **token 値や期限の詳細を返さない**（敵対者観測抑止）

#### 4.4.3 confirm_tokens テーブル

新規 migration `00020_create_confirm_tokens.sql`:

```sql
CREATE TABLE confirm_tokens (
    token_hash       bytea       NOT NULL,    -- SHA-256(32 bytes)、raw token は保持しない
    photobook_id     uuid        NOT NULL,
    session_id       uuid        NOT NULL,    -- 発行したい manage session に紐付け
    action           text        NOT NULL,    -- "unpublish" | "soft_delete"
    expires_at       timestamptz NOT NULL,
    consumed_at      timestamptz NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT confirm_tokens_pk PRIMARY KEY (token_hash),
    CONSTRAINT confirm_tokens_action_check CHECK (action IN ('unpublish', 'soft_delete')),
    CONSTRAINT confirm_tokens_expires_check CHECK (expires_at > created_at)
);
CREATE INDEX confirm_tokens_expires_idx ON confirm_tokens (expires_at);
```

**Reconcile**: expired tokens の cleanup（既存 reconcile 系に追加 or pgcron 等は M-1b 範囲外、別後続）

### 4.5 Backend endpoint / usecase（M-1b）

```
POST   /api/manage/photobooks/{id}/confirm-token        — confirm token 発行
POST   /api/manage/photobooks/{id}/unpublish            — owner hide（hidden_by_owner=true）
POST   /api/manage/photobooks/{id}/republish            — owner unhide（hidden_by_owner=false）
DELETE /api/manage/photobooks/{id}                      — soft_delete（status='deleted', deleted_at）
```

**新 UseCase**:
- `IssueConfirmToken`（manage session validate + token 発行）
- `ConsumeConfirmToken`（破壊系 mutation の input chain で呼ぶ）
- `UnpublishPhotobookFromManage`（hidden_by_owner=true、version+1、Outbox `photobook.unpublished_by_owner`）
- `RepublishPhotobookFromManage`（hidden_by_owner=false、version+1、Outbox `photobook.republished_by_owner`）
- `SoftDeletePhotobookFromManage`（status='deleted'、deleted_at=now、version+1、Outbox `photobook.soft_deleted_by_owner`）

**Outbox event**: 公開ページ / OGP の不可視化を outbox handler で連動（既存 OGP outbox handler を拡張）

### 4.6 Frontend UI（M-1b）

| 新規 component | 役割 |
|---|---|
| `ManagePublishToggleSection` | 公開停止 / 再公開ボタン + 確認 modal + confirm token フェッチ |
| `ManageDeleteSection` | 削除ボタン + 二重確認 modal（タイピング確認 + confirm token） + 削除後の `/manage/{id}?reason=deleted` 表示 |
| `ConfirmActionModal`（共通） | 二重確認 dialog、タイピング確認、confirm token フェッチ + mutation 呼出 |

UI 文言は M-1b plan で確定（spec §3.4「削除操作は取り消せない、または取り消し期間が有限である旨を明示する」）。

### 4.7 test 方針（M-1b）

| 観点 | test |
|---|---|
| confirm token 発行 / 消費 / 期限切れ / 既消費 | usecase test、handler test |
| unpublish 正常 / 反復（republish） | usecase test、handler test |
| soft_delete 正常（status / deleted_at / slug 維持） | usecase test、handler test |
| Slug 復元 invariant（DB UNIQUE 維持） | infrastructure test（既存 `photobook_repository_test.go` 拡張） |
| Outbox INSERT 同 TX | usecase test |
| Migration up / down | spike or local DB で確認 |
| Frontend modal（タイピング確認） | render test |
| Frontend confirm token 取得 → mutation | fetch mock test |
| 公開可視性（hidden_by_owner=true で /p/{slug} 404） | public_handler test |
| Safari 実機 smoke（破壊系のため必須） | runbook 化 |

---

## 5. deploy / rollback / smoke

### 5.1 M-1a deploy 順序

| ステップ | 内容 |
|---|---|
| 1 | Backend Cloud Build manual trigger（`docs/runbook/backend-deploy.md` §1） |
| 2 | Cloud Run Jobs image tag 同期（`vrcpb-image-processor` / `vrcpb-outbox-worker`、handler 経路に到達せずとも運用ルール） |
| 3 | Backend smoke（`/health` / `/readyz` / 既存 routes regression / **新 PATCH/POST endpoint preflight + 401 / 409 確認**） |
| 4 | Workers `cf:build` + `wrangler deploy` |
| 5 | Workers smoke（`/manage/<dummy>` 表示、Cookie なしで error UI、Cookie ありで visibility 変更動作）|
| 6 | Safari 実機 smoke（visibility 切替が iOS Safari ITP の Cookie で動作するか）|

### 5.2 M-1a rollback

- Backend: `gcloud run services update-traffic vrcpb-api --to-revisions=<PREV>=100`
- Workers: `npx wrangler --cwd frontend rollback --name vrcpb-frontend <PREV_VERSION>`
- migration 不要のため DB rollback は不問

### 5.3 M-1b deploy 順序

migration を含むため M-1a より厳重:

| ステップ | 内容 |
|---|---|
| 1 | DB migration apply（`00019_add_photobooks_hidden_by_owner.sql` + `00020_create_confirm_tokens.sql`） — `goose up` を Backend deploy **前**に手動実行 |
| 2 | Backend Cloud Build manual trigger |
| 3 | Cloud Run Jobs image tag 同期 |
| 4 | routing 安定化 wait 5〜10 分 |
| 5 | Backend smoke（既存 + confirm token 発行 + unpublish / soft_delete dummy uuid で 401 → confirm token なしで 400 → 認証通っても 409 等の経路） |
| 6 | Workers deploy |
| 7 | Workers smoke（manage panel に新 section 表示、削除モーダル表示、test photobook で実際に soft_delete → /p/{slug} 404 確認） |
| 8 | Safari 実機 smoke（破壊系のため、user 自身の test photobook で削除挙動を必ず確認） |

### 5.4 M-1b rollback

- migration: `goose down` で `00020` → `00019` を順に reverse 適用（forward-only でない、要 dry-run 確認）
  - **代替**: forward-only 方針継続なら、`hidden_by_owner` 列は残置してコード側で参照しない hotfix（rollback コスト低）
- Backend / Workers: M-1a と同じ手順
- 既に `hidden_by_owner=true` にした photobook がある場合の rollback は要 hotfix（migration down + データ復旧手順）

### 5.5 smoke 必須項目（共通）

- raw manage_url_token / draft_edit_token / Cookie / Secret を logs / response に出さない（`security-guard.md` 既存）
- confirm_token は body にだけ出る、logs に出さない（`security-guard.md` 拡張、M-1b plan 内に明記）
- `/manage/<id>` の SSR HTML に raw token が混入しない

---

## 6. abuse risk / rate limit

| 操作 | risk | 対策 |
|---|---|---|
| visibility / sensitive 変更 | 低（OCC で重複防止、attacker が manage Cookie を取得していれば既に影響大） | 既存 manage session で十分 |
| session revoke | 低（自分の Cookie のみ） | 既存 |
| issue-draft-from-manage | 中（manage 漏洩で /edit まで広がる、ただし §6.13 で許容） | UsageLimit `manage.issue_draft` を新設（5 分 10 件 / 同 photobook、M-1a で実装） |
| confirm-token 発行 | 中（攻撃者が token 連続生成して DB を埋める） | UsageLimit `manage.confirm_token` 新設（1 分 10 件 / 同 session、M-1b で実装） |
| unpublish / soft_delete | 高（破壊的） | confirm token 必須 + UsageLimit + Turnstile 再認証は MVP 外 |

---

## 7. Open Questions（残）

本書 commit 時点で確定していない項目（M-1a 着手前の STOP α または M-1b plan 作成時に確定）:

1. **Q3 「編集を再開」導線の方針**: 案 A（endpoint 込み、推奨）/ 案 B（UI のみ）の最終確定
2. **session revoke の「ボタン」表現**: 「この端末から管理権限を削除」/ 「ログアウト」/ 「セッションを終了」のいずれが UX 的に分かりやすいか
3. **issue-draft-from-manage の rate limit 値**: 5 分 10 件 / 同 photobook で十分か
4. **削除 modal のタイピング確認文言**: 「削除する」/ 「DELETE」/ photobook の slug を打つ / 等
5. **confirm token 有効期限**: 60 秒で十分か（M-1b plan で確定）
6. **soft_delete 後の `/manage/{id}` 表示**: 「削除済み」表示のみ / アクセス自体を 404 にする
7. **republish（hidden_by_owner=true → false 戻し）の UX**: M-1b に含めるか、後続 PR に分けるか
8. **migration 適用タイミング**: M-1b deploy の **直前**で goose up（Cloud SQL Auth Proxy 経由）か、`Migrator` Job 化か

---

## 8. 推奨実装順 / commit 分割

| 順 | task | scope | 想定 commit 数 |
|---|---|---|---:|
| 1 | **本書 commit** | docs only | 1 |
| 2 | M-1a 着手前 STOP α（user 承認） | Q3 確定 | 0 |
| 3 | **M-1a Backend** | endpoint / usecase / domain method / test | 1〜2 |
| 4 | **M-1a Frontend** | component / lib / test | 1〜2 |
| 5 | M-1a verification + deploy + smoke | Backend → Workers | （deploy commit なし） |
| 6 | M-1a deploy 完了報告 + work-log | docs | 1 |
| 7 | **M-1b plan 作成** | docs / 残 Open Questions 確定 | 1 |
| 8 | M-1b 実装 | migration / domain / endpoint / UI / test | 3〜4 |
| 9 | M-1b verification + deploy + smoke | DB migration → Backend → Workers | （deploy commit なし） |
| 10 | M-1b deploy 完了報告 | docs | 1 |

---

## 9. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-11 | 初版作成。M-1a / M-1b の二段構成で固定、Open Questions Q1〜Q8 仮決定を §0.1 に記録 |
