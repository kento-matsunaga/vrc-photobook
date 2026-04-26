# M2 Session auth 実装計画

> 作成日: 2026-04-26
> 位置付け: backend M2 本実装の **PR7 候補** 着手前の計画書。実装コードはまだ書かない。
>
> 上流参照（必読）:
> - [`docs/adr/0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md) — token → session 交換の中核 ADR、M1 PoC 検証結果含む
> - [`docs/design/auth/README.md`](../design/auth/README.md) / [`docs/design/auth/session/`](../design/auth/session/) — Session の集約相当ドメイン設計・データモデル設計
> - [`docs/design/auth/upload-verification/`](../design/auth/upload-verification/) — Turnstile セッション化（**本書の対象外、参照のみ**）
> - [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §2.4 / §6.15 / §7.6
> - [`docs/plan/m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md) — U2 Cookie Domain 解消方針
> - [`docs/plan/m2-implementation-bootstrap-plan.md`](./m2-implementation-bootstrap-plan.md) §4 step 7 / §5.1 / §10
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
> - [`.agents/rules/testing.md`](../../.agents/rules/testing.md)
> - [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md)

## 0. 本計画書の使い方

- 本計画書は **計画書のみ**。実装コード / migration / sqlc / frontend route はまだ書かない。
- 確定事項は ADR-0003 / 設計書（`docs/design/auth/session/`）で既に決まっており、本書では **どう PR に切り出すか** と **未決のユーザー判断事項** を整理する。
- 確定事項を再記述する際は、必ず引用元へのリンクを添える（情報源の単一化）。
- §14 のユーザー判断事項に答えてもらってから PR7 着手に進む。

---

## 1. 目的

- VRC PhotoBook の核である「ログイン不要 + token URL + HttpOnly Cookie session」を **本実装側に確立する**。
- token URL から raw token を消し、URL 共有による権限漏れを防ぐ（ADR-0003 §決定）。
- draft / manage の操作を **session（Cookie）で認可** し、各集約 UseCase（Photobook / Image など）から認可結果を前提にできる土台を作る。
- Safari / iPhone Safari 実機で **session が成立し、24h / 7 日後も維持される** Cookie 設計にする（業務知識 v4 §1.2 / §6.2 のスマホファースト前提）。
- harness/spike のコードを直接コピペで持ち込まず、設計書ベースで再実装する（`docs/plan/m2-implementation-bootstrap-plan.md` §11）。

---

## 2. 前提

- M1 で **token → session 交換 + Cookie 受領 + redirect 後の Server Component で session 検証** が macOS Safari / iPhone Safari の実機 PoC で成立済み（ADR-0003 §M1 検証結果）。
- M1 実環境デプロイ（Cloud Run + Workers、別オリジン）で **Cookie が Backend に渡らない（U2 確定）**。M2 早期に独自ドメイン取得 + 案 A（`app.<domain>` / `api.<domain>` / Cookie Domain `.<domain>`）で解消（[`m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md) §7）。
- ドメインは `vrc-photobook.com` で確定・購入済（2026-04-26 後段、`m2-domain-candidate-research.md` §9.5）。本書執筆時点では未購入だったが、PR9c / PR10 / PR10.5 完了後にハイフン入りで購入確定。本 Session auth PR の最初のステップ（PR7）はドメインに依存させない設計のままで、Cookie Domain は `.vrc-photobook.com` を本番設定値とする。
- backend の足場（PR1〜PR3 / PR6）は完了済み。`/health`、`/readyz`、pgx pool、goose migration 1 本（`_health_check`）、sqlc base が動作する状態。
- frontend の足場（PR4 / PR5）は完了済み。`middleware.ts` で `X-Robots-Tag` / `Referrer-Policy` 一本化、OpenNext / wrangler 設定済み。
- Cloud SQL は使わず、ローカル PostgreSQL（`docker-compose.yaml`）で進める。Cloud Run 実 deploy / Workers 実 deploy は本 Session auth 範囲では行わない。
- Cookie / redirect / レスポンスヘッダ変更を伴うため、**`safari-verification.md` ルールが本実装のすべての PR にかかる**。

---

## 3. 対象範囲

### 対象（本計画 / 後続 PR で扱う）

- `backend/internal/auth/session/` ディレクトリ全体
  - `domain/`: `Session` 概念、`SessionId` / `SessionToken` / `SessionTokenHash` / `SessionType` / `TokenVersionAtIssue` 値オブジェクト
  - `infrastructure/repository/rdb/`: `sessions` テーブルへの永続化、marshaller、repository、tests Builder
  - `internal/usecase/`: `IssueDraftSession` / `IssueManageSession` / `ValidateSession` / `TouchSession` / `RevokeSession` / `RevokeAllDrafts` / `RevokeAllManageByTokenVersion`
- session 関連 migration 1 本（`00002_create_sessions.sql`、`_health_check` の次、Photobook 集約 DDL より前）
- session token 生成 / SHA-256 hash 化 / base64url エンコード方針の確定
- Cookie 生成ポリシー（属性 / 名前 / Domain 切替）の **値定義モジュール**
- draft / manage の区別（汎用 1 本テーブル + `session_type` で分岐、§4 で議論）
- 既存 `/readyz` / pgx pool / chi router / sqlc base との整合
- frontend `app/(draft)/draft/[token]/route.ts` / `app/(manage)/manage/token/[token]/route.ts` への **接続方針整理**（実装は別 PR）

### 対象外（本計画 / 後続 PR では扱わない）

- Photobook aggregate 本実装（draft_edit_token / manage_url_token を **発行する側** の集約）
- Image aggregate / R2 連携 / presigned URL
- `upload_verification_sessions` テーブル / Turnstile siteverify（別の集約相当 [`docs/design/auth/upload-verification/`](../design/auth/upload-verification/)）
- SendGrid / メール送信 / ManageUrlDelivery
- Outbox / Reconcile
- Frontend の token 受け Route Handler 実装本体（**接続方針のみ整理、実コードは PR9 以降**）
- 独自ドメイン購入 / Cloudflare DNS / Workers Custom Domain / Cloud Run Domain Mapping
- Cloud SQL / Cloud Run deploy / Cloud Run Jobs / Cloud Scheduler

---

## 4. データモデル案

### 4.1 既存設計（確定事項）

[`docs/design/auth/session/データモデル設計.md`](../design/auth/session/データモデル設計.md) §3 で **汎用 1 本テーブル `sessions` + `session_type` 分岐** に決定済。upload-verification は別テーブル `upload_verification_sessions`（[`../upload-verification/データモデル設計.md`](../design/auth/upload-verification/データモデル設計.md)）。本書ではこの確定設計をそのまま採用する前提で、PR への落とし方を整理する。

### 4.2 sessions テーブル（確定）

| カラム | 型 | NULL | 既定 | 備考 |
|---|---|---|---|---|
| `id` | `uuid` | NOT NULL | - | PK、UUIDv7 で生成（ADR-0001）。**Cookie には載せない**（DB 内部 ID） |
| `session_token_hash` | `bytea` | NOT NULL | - | SHA-256(SessionToken)、32 バイト固定。raw token は保存しない |
| `session_type` | `text` | NOT NULL | - | CHECK: `'draft' / 'manage'` |
| `photobook_id` | `uuid` | NOT NULL | - | FK → `photobooks.id` ON DELETE CASCADE |
| `token_version_at_issue` | `int` | NOT NULL | `0` | 発行時の `Photobook.manage_url_token_version` の snapshot。draft では常に 0（CHECK 強制） |
| `expires_at` | `timestamptz` | NOT NULL | - | draft: `Photobook.draft_expires_at` まで、manage: 24h〜7 日 |
| `created_at` | `timestamptz` | NOT NULL | `now()` | 発行時刻 |
| `last_used_at` | `timestamptz` | NULL | - | 編集系 API のみ更新 |
| `revoked_at` | `timestamptz` | NULL | - | NULL なら有効 |

CHECK 制約（同設計書 §3.1 より引用、PR7 migration で全件入れる）:

```sql
session_type IN ('draft', 'manage')
expires_at > created_at
last_used_at IS NULL OR (last_used_at >= created_at AND last_used_at <= expires_at)
revoked_at IS NULL OR revoked_at >= created_at
token_version_at_issue >= 0
session_type != 'draft' OR token_version_at_issue = 0
```

### 4.3 汎用 1 本 vs draft/manage 分離テーブル — 比較

| 観点 | 汎用 1 本（`sessions` + `session_type`、確定案） | 分離（`draft_sessions` / `manage_sessions`）|
|---|---|---|
| ライフサイクル | draft も manage も「発行 → 検証 → revoke → GC」で共通 | 共通 |
| TTL の違い | アプリ層で `expires_at` を計算（draft=draft_expires_at、manage=24h〜7 日） | 同様 |
| Cookie 名の違い | クライアント側に分離（`vrcpb_draft_*` / `vrcpb_manage_*`）、テーブルは共通 | クライアント側に分離（同じ） |
| 認可対象の違い | `session_type` で分岐 | テーブルで分岐 |
| reissue 時の一括 revoke | `WHERE photobook_id=$1 AND session_type='manage' AND token_version_at_issue<=$2` の 1 SQL | manage_sessions だけ touch、draft_sessions は別 SQL |
| 拡張性（third type 追加） | `CHECK` 緩めて済む。Photobook publish 履歴 session 等も追加容易 | 新テーブル追加が必要 |
| sqlc 実装容易性 | 1 query セット、CRUD が単純 | 2 query セット、似たコードが二重化 |
| Repository 実装容易性 | 1 repository、CRUD が単純 | 2 repository、抽象化必要 |
| migration 量 | 1 テーブル + index 5 種程度 | 2 テーブル + index ×2 |
| index | `(photobook_id, session_type, revoked_at)` 等で `session_type` を必ず WHERE に入れる必要あり（性能影響は MVP 規模で無視可） | `session_type` 不要、index 設計が単純 |

**推奨: 汎用 1 本 `sessions` テーブルを採用**（既存設計書と一致）。理由:

- スキーマがほぼ同一で、分離する利益より sqlc / repository の二重化コストが上回る
- `revokeAllManageByTokenVersion` のような複合条件 SQL が **1 文で書ける**
- 将来 `system / admin` 等の third type を追加するときに migration 不要

### 4.4 user_agent_hash / ip_hash の扱い

- **MVP では入れない**（[`docs/design/auth/session/ドメイン設計.md`](../design/auth/session/ドメイン設計.md) と一致、`m2-implementation-bootstrap-plan.md` §10.1 のログマスキング方針とも整合）。
- 理由:
  - session 有効性判定に必須ではない（hash 衝突 / NAT 配下 / モバイルキャリア IP 変動でむしろ誤判定リスクがある）
  - 監査・紛争対応は **構造化ログ（access log）に委ねる**（Outbox イベントとは別経路）
  - 将来 abuse 検出を入れるとき、別テーブル `session_audit_log` を追加する方が責務分離が明確
- §14 でユーザー最終確認を取る項目として残す。

### 4.5 token_version_at_issue を初期から入れるか

- **入れる**。理由:
  - manage_url_token 再発行時の一括 revoke（I-S10）は **VRC PhotoBook の運用上の核**（盗まれた管理 URL の即時無効化）
  - スキーマ追加の後回しは migration コスト + 既存 session の値補完が面倒
  - draft session では CHECK で `0` を強制すれば、誤代入は起きない
  - sqlc query の引数が 1 つ増えるだけで、実装コストは無視できる
- §14 でユーザー最終確認を取る項目として残す。

---

## 5. token / hash 方針

### 5.1 raw token と DB 保存値の対応（確定）

| 種別 | raw 値（Cookie / URL に載る） | DB 保存値 | 出所 |
|---|---|---|---|
| `draft_edit_token` | `Photobook.draft_edit_token`（base64url、URL 経由） | `Photobook.draft_edit_token_hash`（SHA-256、32B） | Photobook 集約（後続 PR）|
| `manage_url_token` | `Photobook.manage_url_token`（base64url、URL 経由） | `Photobook.manage_url_token_hash`（SHA-256、32B） | Photobook 集約（後続 PR）|
| `session_token` | Cookie 値（base64url） | `sessions.session_token_hash`（SHA-256、32B） | **本 PR の対象** |

**DB には raw を保存しない**（ADR-0003 §決定 / 設計書 §3.3）。

### 5.2 SessionToken の生成方針

- **256bit 以上の暗号論的乱数**（推奨 32 バイト）。
- Go 標準: `crypto/rand.Read(buf[:32])` を用いる（math/rand 禁止）。
- エンコード: **base64url（パディングなし）**、長さは固定 43 文字（`base64.RawURLEncoding`）。
- 出力幅は VO `SessionToken` で固定し、生成器も VO 内に閉じる（`domain-standard.md` §VO 自己完結）。
- 256bit エントロピーで衝突確率 ≪ 2^-128 のため、**ストレッチング・ソルト不要**（ADR-0003 / 設計書）。
- **再利用禁止**: 一度発行した SessionToken は revoke 後も再発行に使わない。

### 5.3 SessionTokenHash の方針

- `sha256.Sum256(rawToken []byte)` で生成、`bytea` 32 バイト固定で DB に渡す。
- DB ルックアップは `WHERE session_token_hash = $1` のみ（`session_type` / `photobook_id` は別途条件追加）。
- `UNIQUE` 制約により同一 hash の重複は構造的に発生しない。

### 5.4 ログ禁止 / マスキング方針

以下は **絶対にログ・diff・コミットメッセージ・スクリーンショット・エラーメッセージに出さない**（`security-guard.md` / `m2-implementation-bootstrap-plan.md` §10.1）:

- raw `session_token`
- raw `draft_edit_token` / raw `manage_url_token`
- `session_token_hash` のバイナリ
- Cookie 値（`Cookie:` / `Set-Cookie:` ヘッダ全体）
- `Authorization` ヘッダ

実装は **構造化ログ中央マスキング**（PR2 で枠を `internal/shared/logging.go` に作成済、本 PR で禁止フィールドリストを拡張する）。

### 5.5 テスト時の固定 token 取り扱い

- **テスト用に固定 token を `tests/builder.go` に持たせる**（`testing.md` §Builder パターン）。
- 固定値は **テスト専用**であることをコメントで明記し、本番コードに混入しないことを golangci-lint / grep で防ぐ。
- 固定 token のフォーマットは「明らかにダミー」と分かる形（例: `test_session_token_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`）。
- raw token の hex / 値そのものをログに出すテストは禁止（`t.Log` も）。

---

## 6. Cookie 方針

### 6.1 Cookie 名（確定）

- draft session: `vrcpb_draft_{photobook_id}`
- manage session: `vrcpb_manage_{photobook_id}`

`{photobook_id}` は UUIDv7（hex 36 文字）。photobook 単位で分離するため、複数 Photobook を編集する操作者でも独立した session が成立する。

### 6.2 Cookie 属性（確定 / ADR-0003）

| 属性 | 値 | 備考 |
|---|---|---|
| `HttpOnly` | true | 必須。JS 側からアクセス不可 |
| `Secure` | true（本番） / 環境分岐（local） | §6.3 参照 |
| `SameSite` | `Strict` | CSRF 一次対策（M1 PoC 検証済） |
| `Path` | `/` | 全パス共通 |
| `Domain` | M2 早期 §F-1 後に `.<domain>` / それ以前は **未設定** | §6.4 参照 |
| `Max-Age` | draft: `int(time.Until(expires_at).Seconds())` / manage: 同（24h〜7 日） | `Expires` は `Max-Age` で代替（モダンブラウザ前提） |
| `Expires` | 設定しない | `Max-Age` のみで管理（M1 検証済） |

### 6.3 localhost で `Secure` 属性をどう扱うか

`Secure` 属性付きの Cookie は **HTTPS でないと送信されない**。`http://localhost:8080` のローカル開発時の扱いを決める必要がある。

候補:

| 案 | 内容 | 開発体験 | 本番との乖離 |
|---|---|---|---|
| 案 A | `Secure` を **常に true**、ローカルでは `localhost` の HTTPS exemption を使う（ブラウザは `Secure` でも `localhost` には送る） | 良 | 小 |
| 案 B | 環境変数 `APP_ENV=local` のときだけ `Secure: false` | 良 | 中（属性が分岐する点が運用ミスを呼ぶリスク）|
| 案 C | ローカルでも mkcert / Cloudflare Tunnel で HTTPS にし、本番と同条件 | 中（セットアップ手間）| ゼロ |

**推奨: 案 A**。理由:

- 主要ブラウザ（Chrome / Safari / Firefox）は **`localhost` を Secure context として扱う**（`http` でも `Secure` Cookie を送る）ため、`Secure: true` のままで動く
- 案 B は本番フラグの設定漏れで `Secure: false` が本番に出る事故リスクがある
- 案 C は M2 段階で要求するには重い

→ §14 ユーザー最終確認項目。

### 6.4 Cookie Domain の段階的切替

| フェーズ | 状態 | Cookie Domain | 備考 |
|---|---|---|---|
| 現在（PR7 着手時） | ドメイン未取得、ローカル開発のみ | **未設定**（host-only Cookie）| `localhost` で動作確認 |
| ドメイン取得後（2026-04-26 後段、`m2-domain-candidate-research.md` §9.5） | `vrc-photobook.com`、`app.vrc-photobook.com` / `api.vrc-photobook.com` 構成 | `.vrc-photobook.com` | [`m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md) §8 切替手順に従う |

切替を **環境変数 `COOKIE_DOMAIN`**（空なら未設定、値があればその値）で吸収する。コード本体は分岐しない。

### 6.5 明示破棄（共有 PC 対策）

- Backend `POST /api/auth/sessions/revoke`（後続 PR）で `revoke(sessionId)` 実行 + `Set-Cookie: <name>=; Max-Age=0; Path=/` を返す
- 元の `draft_edit_token` / `manage_url_token` 自体は失効させない（別端末からの再入場を妨げない、設計書 §3.3）

### 6.6 Safari 確認項目（safari-verification.md と紐付け）

PR7〜PR10 のいずれの PR でも、Cookie / redirect / レスポンスヘッダに触る場合は **macOS Safari + iPhone Safari** で以下を確認する:

- Cookie 発行: `Set-Cookie` の HttpOnly / Secure / SameSite=Strict / Path=/ が DevTools / Web Inspector で見える
- redirect 後 Cookie 引継ぎ: `/draft/{token}` → 302 → `/edit/{photobook_id}` で Cookie が読める
- URL から token 消去: redirect 後の URL に raw token が残っていない
- 24h / 7 日後の Cookie 残存（運用開始後の継続観察、`safari-verification.md` §継続観察）

---

## 7. backend API / UseCase 案

### 7.1 UseCase 一覧（確定 / 設計書 §ドメイン操作仕様）

| UseCase | 引数 | 返却 | 呼び出し元 |
|---|---|---|---|
| `IssueDraftSession` | `photobookId`, `expiresAt`（=Photobook.draft_expires_at） | `(SessionToken raw, Session)` | Photobook 集約の `enterDraft` 経路、または draft 入場 Route |
| `IssueManageSession` | `photobookId`, `tokenVersionAtIssue`, `expiresAt`（=now + 24h〜7d） | `(SessionToken raw, Session)` | manage 入場 Route |
| `ValidateSession` | `rawToken`, `photobookId`, `sessionType` | `Session` or 401 | 各 API の認可 middleware（後続 PR）|
| `TouchSession` | `sessionId` | - | 編集系 API 成功時のみ |
| `RevokeSession` | `sessionId` | - | 明示破棄 |
| `RevokeAllDrafts` | `photobookId` | - | Photobook `publishFromDraft` の同一 TX |
| `RevokeAllManageByTokenVersion` | `photobookId`, `oldVersion` | - | Photobook `reissueManageUrl` の同一 TX |

`TouchSession` の `last_used_at` 更新ポリシー:

- **編集系 API 成功時のみ更新**（GET / プレビューでは更新しない）
- 設計書 §ドメイン操作仕様と一致
- 高頻度呼び出しは想定しないため、UPDATE で十分（counter 列・debounce は不要）

### 7.2 PR をどう分けるか

Photobook 集約がまだないため、Session 単独で動くものを先に PR7 で出す。Photobook 集約の token 発行（draft_edit_token / manage_url_token）と接続するのは PR8 以降。

```
PR7  Session 認可機構の単体（domain + token + hash + Cookie policy + repository + sqlc + migration + unit tests）
     - Photobook が無くても完結できる: photobook_id を引数として受け取るだけで、FK 検証は migration では入れない（後続）
     - もしくは、_health_check の代わりに最小限の photobooks スタブテーブルを先に入れる（§14 で確認）
PR8  Session UseCase + handler 枠（IssueDraftSession / IssueManageSession を呼ぶ HTTP endpoint だが、token 検証は dummy）
     - Photobook 集約がまだなので、token は固定値で受ける（テスト専用）
     - middleware で ValidateSession を呼ぶ枠を提供
PR9  Photobook 集約（draft_edit_token / manage_url_token / publish / reissueManageUrl）
     - Session 機構と接続: publishFromDraft で revokeAllDrafts、reissueManageUrl で revokeAllManageByTokenVersion を同一 TX 呼び出し
PR10 Frontend `/draft/[token]/route.ts` / `/manage/token/[token]/route.ts` 実装、Backend へ token 交換委譲、Set-Cookie + redirect
     - Safari / iPhone Safari 実機確認をこの PR で必ず実施（safari-verification.md）
```

### 7.3 PR7〜PR10 の完了条件

- **PR7**:
  - `sessions` テーブル migration 適用 / rollback 動作
  - `Session` ドメイン + 値オブジェクト（`session_id` / `session_token` / `session_token_hash` / `session_type` / `token_version_at_issue`）の単体テスト合格（テーブル駆動 + Builder + description）
  - sqlc 生成成功 / repository CRUD テスト合格（実 DB、`docker-compose` 経由）
  - `crypto/rand` 使用、math/rand 不在
  - raw token がログに出ないこと（マスキング動作確認）
- **PR8**:
  - `IssueDraftSession` / `IssueManageSession` / `ValidateSession` / `TouchSession` / `RevokeSession` の usecase 単体テスト合格
  - HTTP endpoint 枠（dummy token 受け）
  - middleware が `ValidateSession` を呼ぶ枠
- **PR9**:
  - Photobook 集約の draft_edit_token / manage_url_token 発行・検証・再発行が動作
  - `publishFromDraft` で `revokeAllDrafts` が同一 TX 内で呼ばれる
  - `reissueManageUrl` で `revokeAllManageByTokenVersion` が同一 TX 内で呼ばれる
- **PR10**:
  - Frontend `/draft/[token]/route.ts` / `/manage/token/[token]/route.ts` 実装
  - Backend へ token 交換委譲、Set-Cookie + 302 redirect
  - Safari / iPhone Safari 実機で session 成立確認（screenshot 不要、`safari-verification.md` チェックリストの項目を埋める）

---

## 8. Migration 計画

### 8.1 ファイル

`backend/migrations/00002_create_sessions.sql`（goose 形式、`-- +goose Up` / `-- +goose Down`）

### 8.2 内容（DDL 概要、実 SQL は PR7 で書く）

```sql
-- +goose Up
CREATE TABLE sessions (
    id                       uuid        PRIMARY KEY,
    session_token_hash       bytea       NOT NULL,
    session_type             text        NOT NULL,
    photobook_id             uuid        NOT NULL,
    token_version_at_issue   int         NOT NULL DEFAULT 0,
    expires_at               timestamptz NOT NULL,
    created_at               timestamptz NOT NULL DEFAULT now(),
    last_used_at             timestamptz,
    revoked_at               timestamptz,

    CONSTRAINT sessions_session_type_check CHECK (session_type IN ('draft', 'manage')),
    CONSTRAINT sessions_expires_after_created CHECK (expires_at > created_at),
    CONSTRAINT sessions_last_used_in_range CHECK (
        last_used_at IS NULL
        OR (last_used_at >= created_at AND last_used_at <= expires_at)
    ),
    CONSTRAINT sessions_revoked_after_created CHECK (
        revoked_at IS NULL OR revoked_at >= created_at
    ),
    CONSTRAINT sessions_token_version_nonneg CHECK (token_version_at_issue >= 0),
    CONSTRAINT sessions_draft_token_version_zero CHECK (
        session_type != 'draft' OR token_version_at_issue = 0
    )
);

CREATE UNIQUE INDEX sessions_session_token_hash_uniq
    ON sessions (session_token_hash);

CREATE INDEX sessions_photobook_type_revoked_idx
    ON sessions (photobook_id, session_type, revoked_at);

CREATE INDEX sessions_photobook_type_version_active_idx
    ON sessions (photobook_id, session_type, token_version_at_issue)
    WHERE revoked_at IS NULL;

CREATE INDEX sessions_expires_active_idx
    ON sessions (expires_at)
    WHERE revoked_at IS NULL;

CREATE INDEX sessions_revoked_idx
    ON sessions (revoked_at)
    WHERE revoked_at IS NOT NULL;

-- +goose Down
DROP TABLE sessions;
```

### 8.3 photobook_id への FK は PR7 では入れない

- Photobook 集約テーブルが PR9 まで存在しないため、`REFERENCES photobooks(id) ON DELETE CASCADE` は **PR9 で追加**する（migration `00003` 等で `ALTER TABLE` する想定）。
- PR7 段階では `photobook_id uuid NOT NULL` だけにし、テストでは適当な UUID を入れる。
- §14 でユーザー判断: 「PR7 で先に photobooks スタブテーブル（id だけ）を入れて FK を最初から張る」案もある。

### 8.4 Down migration

- `DROP TABLE sessions;` のみ（CASCADE はつけない、index は自動 drop）。
- 開発時の `goose down` で動作確認する。

### 8.5 テスト用 seed は作らない

- `migrations/` 配下に seed SQL は置かない（`testing.md` §禁止事項：fixture.go と同種の問題を引き起こす）。
- テストでの session 生成は **すべて Builder 経由**（`tests/builder.go`）。

---

## 9. Repository / sqlc 計画

### 9.1 sqlc query（`backend/internal/auth/session/infrastructure/repository/rdb/queries/session.sql`）

| query 名 | 内容 |
|---|---|
| `CreateSession` | INSERT、新規 session 1 件 |
| `FindSessionByHash` | SELECT、`session_token_hash = $1 AND session_type = $2 AND photobook_id = $3 AND revoked_at IS NULL AND expires_at > now()` |
| `TouchSession` | UPDATE `last_used_at = now() WHERE id = $1 AND revoked_at IS NULL` |
| `RevokeSessionByID` | UPDATE `revoked_at = now() WHERE id = $1 AND revoked_at IS NULL` |
| `RevokeAllDraftsByPhotobook` | UPDATE `revoked_at = now() WHERE photobook_id = $1 AND session_type = 'draft' AND revoked_at IS NULL` |
| `RevokeAllManageByTokenVersion` | UPDATE `revoked_at = now() WHERE photobook_id = $1 AND session_type = 'manage' AND token_version_at_issue <= $2 AND revoked_at IS NULL` |
| `DeleteExpiredSessions` | DELETE `WHERE (revoked_at IS NOT NULL AND revoked_at < now() - interval '30 days') OR (expires_at < now() - interval '7 days')` （後続 PR の GC バッチ用、PR7 では query のみ用意でも可） |

### 9.2 sqlc 生成先

- 集約別 sqlc 分離方針（[`m2-implementation-bootstrap-plan.md`](./m2-implementation-bootstrap-plan.md) §3）に沿い、`internal/auth/session/infrastructure/repository/rdb/sqlcgen/` 配下に生成。
- `backend/sqlc.yaml` に session 用の `queries` / `out` パスを追加（PR3 の `internal/database/sqlcgen/` とは別ディレクトリ）。

### 9.3 Repository インターフェース

`backend/internal/auth/session/infrastructure/repository/rdb/session_repository.go`:

```go
type SessionRepository interface {
    Create(ctx, session Session) error
    FindByHash(ctx, hash SessionTokenHash, sessionType SessionType, photobookID PhotobookID) (Session, error)
    Touch(ctx, id SessionID) error
    Revoke(ctx, id SessionID) error
    RevokeAllDrafts(ctx, photobookID PhotobookID) error
    RevokeAllManageByTokenVersion(ctx, photobookID PhotobookID, oldVersion int) error
}
```

`PhotobookID` 型は本 PR では仮置き（`type PhotobookID uuid.UUID` を auth/session 内で定義）し、PR9 で正式な Photobook 集約 VO に置換する。

### 9.4 Repository テスト方針

- **実 DB 必須**（`testing.md` §テスト階層、`docker-compose.yaml` の postgres を利用）。
- テーブル駆動 + description + Builder。
- テストごとに `sessions` テーブルを TRUNCATE（または BEGIN/ROLLBACK でスコープ）。
- raw SQL を直接書いた検証は禁止、sqlc query 経由のみ。

### 9.5 marshaller

- `marshaller/session_marshaller.go`: ドメインの `Session` ↔ sqlc 生成物の `Session` row 構造の変換。
- VO ↔ プリミティブ変換は marshaller に閉じる（`domain-standard.md` §インフラ）。

---

## 10. テスト方針

### 10.1 全体方針（`testing.md` 準拠）

- **テーブル駆動 + `description` 必須 + Builder（メソッドテスト）/ コンストラクタテストは直接構築**
- フラットな `t.Run` 列挙、ヘルパー関数、fixture.go、Builder の `t` 保持は禁止
- レイヤーごとのテスト深度:

| レイヤー | テスト | DB |
|---|---|---|
| VO（`session_token` / `session_token_hash` / `session_type` / `token_version_at_issue` / `session_id`） | コンストラクタ、等価性、不変性、境界値 | 不要 |
| Domain（`Session` エンティティ） | コンストラクタ（不変条件）、`isExpired` / `isRevoked` / `isDraft` / `isManage` | 不要 |
| UseCase（`IssueDraftSession` 等） | ビジネスフロー、Repository mock 可 | 不要 |
| Repository | sqlc query 動作、CHECK 制約発火、INDEX 効果 | **必要** |
| Cookie policy | `MakeSetCookieHeader(domain, name, raw, expiresAt)` の出力検証（`HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`, `Domain=...` の有無） | 不要 |

### 10.2 token 生成テスト

- 32 バイトの乱数が出ること（base64url で 43 文字）
- 1000 回生成して衝突しないこと（弱い保証だが回帰検出）
- `crypto/rand` を呼んでいること（`math/rand` 経由でないこと）の検証は **golangci-lint の `gosec`** で代替（テストコードでは難しい）

### 10.3 hash テスト

- 既知のテストベクトル（RFC / NIST のサンプル）で SHA-256 が正しいこと
- `len(hash) == 32` であること
- 同一 raw token から同一 hash が出ること（idempotent）
- 異なる raw token から異なる hash が出ること

### 10.4 Cookie policy テスト

- 環境変数 `COOKIE_DOMAIN` の有無で `Domain=` 有無が分岐すること
- `Set-Cookie` ヘッダ文字列に `HttpOnly; Secure; SameSite=Strict; Path=/` が **必ず含まれる**こと
- `Max-Age` が `expires_at - now()` の整数秒であること
- 名前が `vrcpb_draft_<id>` / `vrcpb_manage_<id>` のフォーマットに一致すること

### 10.5 Repository test

- `CreateSession`: 正常系 / unique 違反（同一 hash 二度 INSERT）
- `FindSessionByHash`: hash 一致 / type 不一致 / photobook_id 不一致 / revoked / expired
- `RevokeSessionByID`: 正常系 / 既に revoked / 存在しない id
- `RevokeAllDraftsByPhotobook`: draft のみ / manage は影響なし / 既 revoked は変更なし
- `RevokeAllManageByTokenVersion`: oldVersion 以下が revoke される / それより新しい version は残る
- CHECK 制約: draft で `token_version_at_issue != 0` が REJECT される / `expires_at <= created_at` が REJECT される

### 10.6 handler test は PR8 以降

- PR7 は handler を持たない（usecase + repository まで）
- PR8 以降で chi router 経由の統合テストを追加

### 10.7 Builder の使用範囲

- `tests/session_builder.go`: `Session` の組み立てを Builder に集約
- VO ごとの Builder は最小限（VO はコンストラクタテストで直接構築）
- Builder は `t` を保持しない、`Build(t)` で受け取る（`testing.md` §禁止事項）

---

## 11. セキュリティ確認

### 11.1 ログ・露出禁止チェックリスト

- [ ] raw `session_token` をログに出さない（中央マスキング）
- [ ] raw `draft_edit_token` / raw `manage_url_token` をログに出さない（PR9 で Photobook 側に同様マスキング）
- [ ] `session_token_hash`（バイナリ）をログに出さない
- [ ] `Cookie:` / `Set-Cookie:` ヘッダ全体をログに出さない（要素単位で必要なら属性のみ）
- [ ] `Authorization` ヘッダをログに出さない
- [ ] エラーメッセージにも上記を含めない（panic / err.Error() / wrap 文字列）
- [ ] テスト出力（`t.Log` / `t.Errorf`）にも上記を含めない
- [ ] PR diff / コミットメッセージに上記が混入していない

### 11.2 認可チェック

- [ ] DB にはハッシュのみ保存、raw は保存しない（migration / repository / marshaller で確認）
- [ ] 期限切れ session は拒否（`expires_at > now()` を SQL に常に含める）
- [ ] revoked session は拒否（`revoked_at IS NULL` を SQL に常に含める）
- [ ] photobook_id 不一致は拒否（`ValidateSession` の引数で必ず検証）
- [ ] purpose（session_type）不一致は拒否（draft / manage を取り違えない）
- [ ] テナントスコープ違反: VRC PhotoBook はマルチテナントではない（業務知識 v4 §1.1 / §6.1）が、photobook_id を間違えないことが等価のガード

### 11.3 Safari / iPhone Safari 確認（PR10 で実施）

- [ ] macOS Safari で `Set-Cookie` 属性が DevTools で見える（HttpOnly / Secure / SameSite=Strict / Path=/）
- [ ] iPhone Safari で同様（Web Inspector）
- [ ] redirect 後 Cookie が引き継がれ、URL から token が消える
- [ ] 24h 後 / 7 日後の Cookie 残存（運用開始後の継続観察、`safari-verification.md`）
- [ ] プライベートブラウジングでも一時的に成立する（永続不要）

### 11.4 テストでの認証バイパス禁止

- 認可ミドルウェアを skip する flag や test only の bypass を **本番コードに混ぜない**（`security-guard.md` §禁止事項）
- テスト用に session を作成するときは Builder + repository.Create 経由で、本物の session を作る

---

## 12. Frontend との接続方針（実装は PR10）

### 12.1 Route Handler の置き場所（確定 / `m2-implementation-bootstrap-plan.md` §5.1）

- `app/(draft)/draft/[token]/route.ts`
- `app/(manage)/manage/token/[token]/route.ts`

これらは Frontend 側の **唯一の Route Handler 例外**（他は Backend が API を持つ）。

### 12.2 Set-Cookie の発行元

候補:

| 案 | 発行元 | 良さ | 課題 |
|---|---|---|---|
| 案 A | Frontend Route Handler が Backend を呼び、**Frontend 側で `Set-Cookie`** | 独自ドメイン下での Cookie Domain 設定が単純（`app.<domain>` から `.<domain>` を発行） | Backend の検証結果を Frontend が信頼する形（hash を返してもらう or session_token を返してもらう） |
| 案 B | Backend が `Set-Cookie` を直接付ける | Backend だけで完結、信頼境界が単純 | Frontend Route Handler が Backend のレスポンスヘッダを **そのまま中継**する必要があり、CDN / OpenNext の挙動依存 |

**`m2-implementation-bootstrap-plan.md` §5.1 で案 A 採用を確定**:
> token を Backend に渡してから Set-Cookie するより、Frontend の Route Handler 内で Backend を呼んで Cookie を発行する方が、独自ドメイン下での Cookie Domain 設定が単純

ただし、案 A だと **Backend が raw `session_token` を Frontend Route Handler のレスポンス body で返す**形になり、Worker → ブラウザ間の HTTP 通信に raw が乗る瞬間がある（HTTPS で守られる）。レビュー時に再確認。§14 で最終判断。

### 12.3 接続シーケンス（案 A）

```
ブラウザ
  ↓ GET /draft/{rawToken}
Frontend Route Handler (app/(draft)/draft/[token]/route.ts)
  ↓ POST /api/auth/draft-session-exchange { rawToken }
Backend
  ↓ Photobook.findByDraftEditTokenHash(hash(rawToken))
  ↓ IssueDraftSession(photobookId, draft_expires_at)
  ↓ return { sessionToken: <raw>, photobookId, expiresAt }
Frontend Route Handler
  ↓ Set-Cookie: vrcpb_draft_{photobookId}=<sessionToken>; HttpOnly; Secure; SameSite=Strict; Path=/; Domain=.<domain>; Max-Age=...
  ↓ 302 Location: /edit/{photobookId}
ブラウザ
  ↓ GET /edit/{photobookId} (Cookie 自動送信)
Frontend Server Component
  ↓ Backend GET /api/photobooks/{id}（Cookie 中継）
Backend middleware
  ↓ ValidateSession(rawToken from cookie, photobookId, 'draft')
  ↓ TouchSession(sessionId)
  ↓ return Photobook detail
```

manage は同様で `vrcpb_manage_*` Cookie を発行、`(manage)/manage/[photobookId]` へ redirect。

### 12.4 app.<domain> / api.<domain> 構成との整合

- Frontend Route Handler は `app.<domain>` 上で動く（Workers）
- Backend API は `api.<domain>` 上で動く（Cloud Run）
- Cookie Domain `.<domain>` で `app.*` ↔ `api.*` 間の Cookie 共有が成立（M1 で別オリジン NG を確認済、M2 早期で解消）
- 独自ドメイン取得前は **localhost** のみで動作確認（PR7〜PR9）。PR10 のドメイン確定 PR で実機接続。

### 12.5 localhost 開発時

- Frontend: `http://localhost:3000`（Next.js dev）
- Backend: `http://localhost:8080`（docker-compose api）
- Cookie Domain: 未設定（host-only Cookie）
- Frontend → Backend は **同一オリジンではない**ため、CORS + `credentials: 'include'` が必要
- `ALLOWED_ORIGINS=http://localhost:3000` を Backend env に渡す（PR8 以降の middleware で対応）

---

## 13. PR 分割案

### 13.1 PR7（Session 認可機構の単体）

スコープ:

- `backend/internal/auth/session/domain/` 全体（VO + Session エンティティ + ドメイン操作仕様）
- `backend/internal/auth/session/infrastructure/repository/rdb/` 全体（marshaller + repository + sqlc）
- `backend/migrations/00002_create_sessions.sql` + Down
- Cookie policy 値定義モジュール（属性生成器、`Set-Cookie` ヘッダ文字列ビルダ）
- 単体テスト（VO / Domain / Repository、テーブル駆動 + Builder + description）
- `backend/sqlc.yaml` に session 用設定追加
- README 更新（session 機構の追記）

完了条件:

- `go vet / build / test` すべて OK
- `goose up / down` 動作確認
- repository test が docker-compose の postgres で OK
- raw token / hash / Cookie のログ非露出確認（grep + 中央マスキング設定）
- secret 混入なし

### 13.2 PR8（Session UseCase + middleware 枠）

スコープ:

- `backend/internal/auth/session/internal/usecase/`: `IssueDraftSession` / `IssueManageSession` / `ValidateSession` / `TouchSession` / `RevokeSession`
- `backend/internal/http/middleware/session_auth.go`: chi middleware、Cookie から rawToken を取って `ValidateSession` を呼ぶ
- HTTP endpoint の **枠**:
  - `POST /api/auth/draft-session-exchange`（dummy: token 検証は固定値で OK）
  - `POST /api/auth/manage-session-exchange`（dummy）
  - `POST /api/auth/sessions/revoke`
- usecase 単体テスト（Repository mock）

完了条件:

- usecase test 合格
- middleware が dummy session で動く
- Photobook が無くても endpoint が応答する（PR9 で本物の token に置換）

### 13.3 PR9（Photobook 集約 + Session 接続）

スコープ:

- `backend/internal/photobook/`（draft_edit_token / manage_url_token / publish / reissueManageUrl）
- `00003_create_photobooks.sql` + `sessions.photobook_id` への FK 追加（`ALTER TABLE sessions ADD CONSTRAINT ... REFERENCES photobooks(id) ON DELETE CASCADE`）
- `publishFromDraft` の同一 TX 内で `RevokeAllDrafts` 呼び出し
- `reissueManageUrl` の同一 TX 内で `RevokeAllManageByTokenVersion` 呼び出し
- HTTP endpoint で dummy token を **本物の token 検証**に置換

完了条件:

- Photobook の publish フローが session revoke と一緒に動く
- manage URL 再発行で旧 version session が一括 revoke される
- `Outbox` への INSERT は **本 PR では行わない**（Outbox 実装は別 PR、`Photobook.version += 1` のみ更新）

### 13.4 PR10（Frontend route + Safari 検証）

スコープ:

- `frontend/app/(draft)/draft/[token]/route.ts` 実装
- `frontend/app/(manage)/manage/token/[token]/route.ts` 実装
- Backend へ token 交換委譲、`Set-Cookie` + 302 redirect
- Frontend Server Component で session 検証（Backend `/api/photobooks/{id}` 経由）
- Cookie util（`frontend/src/lib/cookies.ts`）
- **Safari / iPhone Safari 実機確認**（macOS Safari + iPhone Safari、`safari-verification.md` チェックリスト全項目）

完了条件:

- ローカル（localhost）で `/draft/{token}` 入場 → Cookie 受領 → `/edit/{id}` redirect が成立
- Safari / iPhone Safari で同様に成立（screenshot 不要、各項目を check）
- URL から raw token が消える
- DevTools で Cookie 属性確認（HttpOnly / Secure / SameSite=Strict / Path=/）

### 13.5 PR11 以降（範囲外、参考）

- Image aggregate + R2 + presigned URL
- upload-verification（Turnstile siteverify + `upload_verification_sessions` テーブル）
- Outbox / Reconcile
- ManageUrlDelivery + SendGrid

---

## 14. ユーザー判断事項

PR7 着手前に以下を確認してください。**※ 各項目に推奨案を併記** しています。

### 14.1 sessions テーブルの構成

- [ ] **汎用 1 本テーブル `sessions` + `session_type` で分岐**（推奨、設計書と一致、§4.3）
- [ ] draft / manage を別テーブルに分離

### 14.2 ローカルでの `Secure` Cookie 属性

- [ ] **常に `Secure: true`**（推奨、`localhost` はブラウザが Secure context として扱う、§6.3 案 A）
- [ ] `APP_ENV=local` のとき `Secure: false` に分岐
- [ ] mkcert / Tunnel でローカルも HTTPS 化

### 14.3 `token_version_at_issue` を初期から入れるか

- [ ] **入れる**（推奨、I-S10 一括 revoke の核、§4.5）
- [ ] 後で migration で追加

### 14.4 `user_agent_hash` / `ip_hash` を入れるか

- [ ] **入れない**（推奨、MVP 方針、設計書と一致、§4.4）
- [ ] 入れる（NAT・モバイル IP の誤判定リスクを許容）

### 14.5 Set-Cookie の発行元

- [ ] **Frontend Route Handler 側で発行**（推奨、`m2-implementation-bootstrap-plan.md` §5.1 で確定済、§12.2 案 A）
- [ ] Backend 側で発行し Frontend Route Handler で中継

### 14.6 PR7 で photobooks スタブテーブルを先に入れるか

- [ ] **入れない**（推奨、`photobook_id` は FK 無しで保持、PR9 で `ALTER TABLE` で FK 追加、§8.3）
- [ ] 入れる（PR7 で `photobooks(id uuid primary key)` だけ作って FK を最初から張る）

### 14.7 実装開始 PR

- [ ] **PR7 から開始**（推奨、§13.1）
- [ ] 先に未決事項（独自ドメイン購入 / Photobook 設計確認）を進めてから

### 14.8 ドメイン購入タイミング

- [ ] **PR7〜PR9 完了後、PR10 着手前**（推奨、`m2-implementation-bootstrap-plan.md` §10 ユーザー判断事項 #6 と一致）
- [ ] PR7 着手前に購入してしまう（Cookie Domain を最初から `.<domain>` で書ける）

---

## 15. 実施しないこと（PR7 計画範囲外、再掲）

本計画書および PR7〜PR10 では **以下を実施しない**:

- session 実装コード作成（PR7 で実施）
- Photobook aggregate 本実装（PR9 で実施）
- Image aggregate / R2 / presigned URL（PR11 以降）
- upload-verification / Turnstile（PR12 以降）
- SendGrid / メール送信 / ManageUrlDelivery（後続）
- Outbox / Reconcile（後続）
- 独自ドメイン購入（M2 早期 §F-1、PR9 完了後）
- Cloudflare DNS 変更 / Workers Custom Domain / Cloud Run Domain Mapping
- Cloud SQL 作成
- Cloud Run deploy / Cloud Run Jobs / Cloud Scheduler
- 既存リソース削除

---

## 16. 関連ドキュメント

- [ADR-0003 frontend token-session flow](../adr/0003-frontend-token-session-flow.md)
- [Session ドメイン設計](../design/auth/session/ドメイン設計.md)
- [Session データモデル設計](../design/auth/session/データモデル設計.md)
- [auth README](../design/auth/README.md)
- [M2 早期ドメイン + Cookie 計画](./m2-early-domain-and-cookie-plan.md)
- [M2 実装ブートストラップ計画](./m2-implementation-bootstrap-plan.md)
- [プロジェクト全体ロードマップ](../../harness/work-logs/2026-04-26_project-roadmap-overview.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`safari-verification.md`](../../.agents/rules/safari-verification.md) / [`testing.md`](../../.agents/rules/testing.md) / [`domain-standard.md`](../../.agents/rules/domain-standard.md)
