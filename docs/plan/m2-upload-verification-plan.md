# M2 Upload-Verification / Turnstile 実装計画（PR20 候補）

> 作成日: 2026-04-27
> 位置付け: PR19（Photobook ↔ Image 連携）完了後、PR21（R2 + presigned URL）の前提となる
> Upload-Verification / Turnstile session の設計フェーズ。実装コードはまだ書かない。
>
> 上流参照（必読）:
> - [`docs/adr/0005-image-upload-flow.md`](../adr/0005-image-upload-flow.md) §Turnstile 検証
> - [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md)
>   §2.4 / §2.7 / §6.13 / §6.18-6.20
> - [`docs/plan/m2-image-upload-plan.md`](./m2-image-upload-plan.md) §7
> - [`docs/plan/m2-photobook-image-connection-plan.md`](./m2-photobook-image-connection-plan.md)
> - [`docs/design/aggregates/image/ドメイン設計.md`](../design/aggregates/image/ドメイン設計.md)
> - [`docs/design/aggregates/image/データモデル設計.md`](../design/aggregates/image/データモデル設計.md)
> - [`docs/design/aggregates/photobook/ドメイン設計.md`](../design/aggregates/photobook/ドメイン設計.md)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
> - [`.agents/rules/testing.md`](../../.agents/rules/testing.md)
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)

---

## 0. 本計画書の使い方

- 設計の正典は `docs/adr/0005-image-upload-flow.md §Turnstile`。本書はそれを **PR20 として
  どこまで切り出すか** を整理する。設計と差異が出た場合は ADR が優先。
- **PR20 の実装範囲は migration + domain + sqlc + Repository + Turnstile client
  interface + UseCase + test** に限定する。
- public API endpoint / Frontend widget / Cloudflare Dashboard 操作 / Secret Manager
  本登録 / R2 / presigned URL は **実装しない**（§14）。
- §15 のユーザー判断事項に答えてもらってから PR20 実装に着手する。

---

## 1. 目的

- 画像アップロード前に Turnstile で人間確認 / Bot 抑制を行う。
- 1 回の Turnstile 検証で 30 分以内 / 20 回までの upload-intent を許可する
  （UX と防御の妥協点、ADR-0005）。
- 検証結果を `upload_verification_sessions` テーブルで短期保持する。
- PR21 の `POST /api/photobooks/{id}/images/upload-intent` 入口で atomic に consume
  できる Repository / UseCase を完成させる。
- Turnstile bypass / 二重 consume / Photobook 不一致 / 期限切れの全パスを 403 系
  エラーで弾く。
- 関連 spike / 既存実装は **存在しない**（`harness/spike/turnstile/` 不在を確認）。
  PR20 から実装を起こす。

---

## 2. PR20 の対象範囲

### 対象（PR20 で実装する）

- migration: `00011_create_upload_verification_sessions.sql`
- domain:
  - `UploadVerificationSession` entity
  - VO: `verification_session_id` / `verification_session_token` /
    `verification_session_token_hash` / `intent_count`
- sqlc: 既存 photobook set の schema に追加 + 新 query 群
- Repository: `UploadVerificationSessionRepository`（CreateSession / FindActive /
  ConsumeOne / Revoke）
- Turnstile client:
  - interface（`TurnstileVerifier`）
  - 本物実装（`infrastructure/turnstile/cloudflare_verifier.go`、Cloudflare
    siteverify endpoint）
  - fake / test double（`tests/fake_turnstile.go`）
- UseCase: `IssueUploadVerificationSession`（Turnstile 検証 → DB INSERT）/
  `ConsumeUploadVerificationSession`（atomic UPDATE）
- test: domain unit / repository test / UseCase test（実 DB） / Turnstile fake test
- Cloudflare Turnstile / GCP Secret Manager の **手順だけ計画書に記述**（実操作は
  PR21 の R2 設定とまとめて行うのが効率的、§9）

### 対象外（PR20 では実装しない）

- public API endpoint の追加（PR21 で upload-intent と統合）
- Frontend Turnstile widget 表示 / フォーム連携
- Safari / iPhone Safari の widget 表示確認（PR22 で UI 実装と同時）
- Cloudflare Dashboard 上の Turnstile widget 作成（PR21 計画時に実操作）
- Secret Manager への TURNSTILE_SECRET_KEY 本登録
- R2 接続 / bucket 操作 / presigned URL / complete-upload
- image-processor / EXIF 除去 / variant 生成
- Outbox events / SendGrid
- public photobook image API
- Cloud SQL / spike / Cloudflare Dashboard 操作
- Cloud Run revision 更新

---

## 3. DB 設計案

### 3.1 追加 migration

| 番号 | ファイル | 内容 |
|---|---|---|
| 00011 | `00011_create_upload_verification_sessions.sql` | `upload_verification_sessions` テーブル |

### 3.2 `upload_verification_sessions`（ADR-0005 §upload_verification_session に整合）

```sql
id                    uuid        NOT NULL PRIMARY KEY  -- UUIDv7
photobook_id          uuid        NOT NULL              -- FK photobooks(id) ON DELETE CASCADE
session_token_hash    bytea       NOT NULL              -- SHA-256(token)、32 byte 固定
allowed_intent_count  int         NOT NULL DEFAULT 20
used_intent_count     int         NOT NULL DEFAULT 0
expires_at            timestamptz NOT NULL
created_at            timestamptz NOT NULL DEFAULT now()
revoked_at            timestamptz NULL
```

### 3.3 制約（CHECK / FK / INDEX）

```sql
-- length 32 (SHA-256)
CHECK (octet_length(session_token_hash) = 32)
-- 値域
CHECK (allowed_intent_count > 0)
CHECK (used_intent_count >= 0)
CHECK (used_intent_count <= allowed_intent_count)
CHECK (expires_at > created_at)

-- FK
FOREIGN KEY (photobook_id) REFERENCES photobooks(id) ON DELETE CASCADE

-- index
UNIQUE (session_token_hash)
INDEX  (photobook_id, expires_at)
INDEX  (expires_at) WHERE revoked_at IS NULL  -- cleanup batch 用
```

### 3.4 ADR との差分・採用案

ADR-0005 の `allowed_intent_count` / `used_intent_count` 方式（**増加カウンタ**）を採用する。
ユーザー指示書にあった `remaining_intents` / `consumed_count` 方式は意味的には等価だが、
ADR との一致を優先。

atomic consume は次の SQL（§7）:

```sql
UPDATE upload_verification_sessions
   SET used_intent_count = used_intent_count + 1
 WHERE id = $1
   AND session_token_hash = $2
   AND photobook_id = $3
   AND used_intent_count < allowed_intent_count
   AND expires_at > now()
   AND revoked_at IS NULL
RETURNING used_intent_count, allowed_intent_count;
```

0 行影響をエラーに分類（§7.2）。

### 3.5 別テーブルとして独立させる理由

ADR-0005 §upload_verification_session の保存先 通り、`sessions` テーブルへの統合は
MVP では行わない（用途 / 回数制限 / 期限ポリシーが異なる）。Phase 2 で QPS が上がった
時点で Redis 移行を検討する。

---

## 4. token 設計

### 4.1 raw token

- 長さ: **32 byte（256bit）の暗号論的乱数**を `crypto/rand` で生成
- 形式: **base64url（padding なし）で 43 文字**
- ADR-0003 の draft / manage session token と完全に同じ生成方式（既存 VO `draft_edit_token`
  / `manage_url_token` の実装パターンを流用）

### 4.2 DB 保存

- raw token は **保存しない**
- DB には `SHA-256(raw_token)` の 32 byte（`bytea`）のみを `session_token_hash` 列に保存
- VO 側の `Encode()` / `Reveal()` は ADR-0003 既存 token と同パターン
- `String()` / `GoStringer` は実装しない（不用意なログ出力を防ぐ）

### 4.3 Cookie / header 方針（推奨案）

選択肢:

| 案 | 配送方法 | 利点 | 欠点 |
|---|---|---|---|
| **案 A（推奨）** | Frontend が response body で受け取り → メモリ / sessionStorage で保持 → 各 upload-intent の `Authorization: Bearer <token>` または独自 header に乗せる | upload-intent 専用に scope を絞れる / Cookie 漏洩経路と独立 | リロードで失われる（再検証が走る、UX 上は許容） |
| 案 B | HttpOnly Cookie `vrcpb_uv_<photobook_id>` を 30 分発行 | リロードに耐える / 既存 draft / manage session と同パターン | 他 Cookie と混在、HTTP method 経路の取り違いが出やすい |

→ **案 A 推奨**（PR20 では「sessionStorage / メモリで Frontend が一時保持」を前提に、
header 経由 consume を想定）。draft / manage session（HttpOnly Cookie）と「役割」を分離
することで、漏洩面と取り扱い面が独立する。

ただし、Safari の Private Browsing / sessionStorage の挙動（リロード / タブ閉じで消える）
を PR22 で実機確認する。再検証 UX が破綻するなら案 B（HttpOnly Cookie）に切り替える
判断ポイントを §15 Q2 に置く。

### 4.4 既存 draft / manage session との関係

- upload verification session は **photobook_id に紐付く**
- draft / manage session は同じく **photobook_id に紐付く** が、**役割が異なる**
- consume 時に「draft session で認可された Frontend からのみ発行可能」とする（§8）
- DB レベルでは upload_verification_sessions と sessions テーブルは相互参照しない
  （独立、photobook 経由で関連）

---

## 5. Turnstile 検証方式

### 5.1 Cloudflare siteverify endpoint

- URL: `https://challenges.cloudflare.com/turnstile/v0/siteverify`
- request: `secret`（server secret）/ `response`（widget が返した token）/
  optional `remoteip` / optional `idempotency_key`
- response: `success` (bool) / `error-codes` (string[]) / `challenge_ts` /
  `hostname` / `action` / `cdata`

### 5.2 検証する項目（PR20 で実装）

- `success == true`
- `hostname == "app.vrc-photobook.com"`（環境別に許容を切り替え、test 用は別 hostname）
- `action == "upload"` 等の固定値（widget 側で設定して照合）
- `challenge_ts` から数分以内（時刻ずれ許容、5 分程度）
- `error-codes` の標準語彙を logs に出さない（`security-guard.md`）

### 5.3 remoteip の扱い

- `request.RemoteAddr` は Cloud Run / Workers 経由で実 IP ではないため、
  `X-Forwarded-For` の **末尾**（または Cloudflare の `Cf-Connecting-IP`）を取る
- 送るかどうか: **送る**（PR20 推奨案）。Cloudflare 側の不正検知精度が上がるため
- DB に保存するかは §3.2 / §15 Q3 で別判断（推奨は ハッシュも含め保存しない）

### 5.4 timeout / retry / Cloudflare 障害

- timeout: 3 秒（Cloud Run リクエストタイムアウト 60 秒 / upload-intent 全体 を考慮）
- retry: しない（一過性失敗は Frontend で再試行）
- Cloudflare siteverify 障害時:
  - 503 + `turnstile_unavailable` を返し、Frontend に再試行案内
  - **fail-open は禁止**（Bot 検証のため、failure 時は必ず拒否）

### 5.5 test sitekey / test secret の扱い

- 開発 / staging / 本番でそれぞれ別の Turnstile widget を Cloudflare Dashboard で作る
- 設計上の使い分け:
  - local dev: Cloudflare 公開の `test sitekey` を使う（常に success / failure を返す）
  - staging: app.staging.vrc-photobook.com 用に専用 widget
  - 本番: `app.vrc-photobook.com` 用 widget
- secret は環境ごとに Secret Manager で分離

### 5.6 interface 設計（PR20 で実装）

```go
// TurnstileVerifier は Cloudflare Turnstile siteverify を抽象化する。
// 本番は Cloudflare 実装、テストは fake を使う。
type TurnstileVerifier interface {
    Verify(ctx context.Context, in VerifyInput) (VerifyOutput, error)
}

type VerifyInput struct {
    Token    string  // widget からの response token
    RemoteIP string  // 任意
    Action   string  // 期待値 (e.g., "upload")
    Hostname string  // 期待値 (e.g., "app.vrc-photobook.com")
}

type VerifyOutput struct {
    Success      bool
    ChallengeTs  time.Time
    Hostname     string
    Action       string
    ErrorCodes   []string  // logs には出さない
}
```

実装は `infrastructure/turnstile/cloudflare_verifier.go`、HTTP POST + form-urlencoded
+ 3 秒 timeout。Fake は `tests/fake_turnstile.go` で `Verify` を struct field で
差し替え可能にする。

---

## 6. API 方針

### 6.1 推奨: PR20 では public endpoint なし

PR20 は **domain + DB + Repository + Turnstile client + UseCase + test まで**。
public endpoint は追加しない。

理由:
- PR21 で `POST /api/photobooks/{id}/images/upload-intent` を起こす際、その内部で
  「Turnstile session を発行 or consume」を統合するため、PR20 で先行 endpoint を
  作ると interface が二度手間になる
- Frontend がまだ無いため、endpoint があっても叩く相手がいない（ローカル
  E2E 検証の値は薄い）

### 6.2 PR21 で起こす予定の API（参考）

```
POST /api/photobooks/{id}/upload-verifications
  Body: { turnstile_token: string }
  Response: { upload_verification_token: string, expires_at, allowed_intent_count }

POST /api/photobooks/{id}/images/upload-intent
  Header: Authorization: Bearer <upload_verification_token>
  Body: { content_type, declared_byte_size, source_format }
  Response: { image_id, presigned_put_url, storage_key, expires_at }
```

PR20 完了時点では UseCase まで揃っていて、PR21 で handler / router 統合だけ追加する形。

---

## 7. atomic consume 設計

### 7.1 SQL

```sql
-- name: ConsumeUploadVerificationSession :one
UPDATE upload_verification_sessions
   SET used_intent_count = used_intent_count + 1
 WHERE id                  = $1
   AND session_token_hash  = $2
   AND photobook_id        = $3
   AND used_intent_count   < allowed_intent_count
   AND expires_at          > now()
   AND revoked_at          IS NULL
RETURNING used_intent_count, allowed_intent_count;
```

### 7.2 0 行影響時のエラー分類

`UPDATE ... RETURNING` で 0 行になる原因は複数:

- token_hash 不一致（or session 自体存在しない）
- photobook_id 不一致
- used_intent_count >= allowed_intent_count（20 回使い切り）
- expires_at <= now()（30 分経過）
- revoked_at IS NOT NULL（明示 revoke 済）

これらを **PR20 では区別しない**（外部に詳細を返さない）。`ErrUploadVerificationFailed`
で一括し、Frontend には「再検証してください」相当のメッセージを返す。理由:
- 失敗理由が漏れると bot 側が条件を学習できる
- 内部 logs では別途分類（FOR UPDATE で SELECT してから判定するか）を後続 PR で検討

### 7.3 同時リクエストでの二重 consume 防止

`UPDATE ... SET used_intent_count = used_intent_count + 1` は単一行 UPDATE なので、
PostgreSQL の row-level lock で自動的に直列化される。`FOR UPDATE` は不要。

20 リクエスト同時に来た場合:
- 各リクエストが UPDATE を取り、`used_intent_count` が 0..19 に分散される
- 21 番目以降は `used_intent_count < allowed_intent_count` が偽になり 0 行
  → `ErrUploadVerificationFailed`

### 7.4 `revoke` 操作

明示 revoke は PR20 では UseCase / Repository に **メソッドだけ用意**（admin 操作用）。
PR20 段階では UI / endpoint からは呼ばない。`UPDATE ... SET revoked_at = now()
WHERE id = $1`。

---

## 8. Photobook / draft session との関係

### 8.1 発行条件

upload verification session を発行できるのは:

- 対象 Photobook が **status='draft'**（published / deleted は不可）
- 呼び出し元が **draft session Cookie** を持っている（既存 photobook session middleware で
  認可済の context）
- Turnstile 検証が success

manage session での発行可否（§15 Q5）:
- 業務知識 v4 §3.1 では published 後の編集は不可
- manage session は published photobook の管理用
- → **PR20 推奨案: manage session からの発行は許可しない**（draft session のみ）
- 将来 manage 経由で限定的編集を許す場合は別途追加

### 8.2 発行 API（PR21）の擬似フロー

```
1. middleware: vrcpb_draft_<photobook_id> Cookie を検証
2. UseCase: IssueUploadVerificationSession
3.   Turnstile siteverify
4.   token 生成 (crypto/rand 32B)
5.   token_hash = SHA-256(token)
6.   INSERT upload_verification_sessions(photobook_id, token_hash, allowed=20, expires=now+30m)
7.   raw token を response body で返す（DB / log には残さない）
```

### 8.3 owner / role 概念がない中での防御

VRC PhotoBook は「ログイン不要」のため owner / role は無い。代替:

- draft session Cookie が紐付く photobook_id とのみ発行可（middleware で context に
  入っている photobook_id を使う）
- consume 時にも photobook_id を再確認（§7.1 SQL）
- Cookie 漏洩時の影響は draft_edit_token / manage_url_token と同じレベル

---

## 9. Cloudflare Dashboard / Secret Manager 手順（PR20 では実行しない）

### 9.1 Cloudflare Turnstile widget 作成（PR21 で実操作、ユーザー手動）

1. Cloudflare Dashboard → Turnstile → Add site
2. Sitename: `vrc-photobook-prod-upload`
3. Hostname Management:
   - `app.vrc-photobook.com`
4. Widget Mode: Managed（Invisible / Non-Interactive は MVP で選ばない）
5. Pre-clearance: Off（cookie clearance はまだ不要）
6. 取得する値:
   - **sitekey**（公開、Frontend NEXT_PUBLIC_TURNSTILE_SITE_KEY 用）
   - **secret**（Secret Manager で保管）

### 9.2 GCP Secret Manager 登録（PR21 で実操作）

```bash
# secret 値はチャット・コミットメッセージ・作業ログに貼らない
gcloud secrets create TURNSTILE_SECRET_KEY \
  --replication-policy=automatic \
  --project=project-1c310480-335c-4365-8a8

# 値の登録は対話シェルで（Claude Code Bash では sudo 同様の制約）
# echo -n "$VALUE" | gcloud secrets versions add TURNSTILE_SECRET_KEY --data-file=-
```

Cloud Run revision 更新（PR21）:

```bash
gcloud run services update vrcpb-api \
  --region=asia-northeast1 \
  --update-secrets=TURNSTILE_SECRET_KEY=TURNSTILE_SECRET_KEY:latest
```

### 9.3 sitekey の扱い

- **sitekey は公開値**（Frontend のクライアント JS に embed されて全ユーザーに見える）
- Secret Manager ではなく `frontend/.env.production` に `NEXT_PUBLIC_TURNSTILE_SITE_KEY`
  として書く（PR14 の COOKIE_DOMAIN と同じパターン）
- ただし、開発 / staging / 本番でそれぞれ別 sitekey を割り当てる

### 9.4 ローカル開発用テスト sitekey

Cloudflare 公式の test sitekey:
- 常に成功: `1x00000000000000000000AA` / secret `1x0000000000000000000000000000000AA`
- 常に失敗: `2x00000000000000000000AB` / secret `2x0000000000000000000000000000000AA`
- 強制 challenge: `3x00000000000000000000FF` / secret `3x0000000000000000000000000000000AA`

local dev の `frontend/.env.local` にこれらを設定し、本番値混入を防ぐ。

### 9.5 ADR-0005 / 業務知識との整合

`harness/spike/turnstile/` は存在しない（grep で確認済）。spike 段階での実証実験は
**未実施**。PR20 / PR21 / PR22 が初の Turnstile 実装。よって PR21 で本番 widget 作成
時は実機（Safari / iPhone Safari）での widget 表示確認が必須（`safari-verification.md`）。

---

## 10. Frontend 連携方針（PR22 で実装）

PR20 では Frontend に変更を加えない。PR22 着手時に整理:

- Turnstile widget の表示位置: 編集画面の「アップロード開始」ボタン付近
- 検証タイミング:
  - 初回 upload 直前
  - 20 intent / 30 分超過時
- iPhone Safari widget 表示:
  - JavaScript 必須
  - Private Browsing で widget が表示されるか実機確認
- failure UI:
  - 「Bot 検証に失敗しました。もう一度お試しください」
  - error-codes は表示しない
- 再検証タイミング:
  - 20 intent 使い切り
  - 30 分経過
  - 別 photobook への切り替え（実質発生しない、編集 URL ごとに別 session）

---

## 11. テスト方針

### 11.1 PR20 で書くテスト

**domain unit test**:
- token VO（length 43, IsZero, Encode, hash 32B）
- intent_count VO（0/20 境界、negative reject）

**Turnstile fake test**:
- success / failure / hostname mismatch / action mismatch / 期限切れ challenge_ts

**Repository test（実 DB）**:
- CreateSession で INSERT
- ConsumeOne 成功（used_intent_count が 1, 2, ..., 20 と増える）
- ConsumeOne 失敗: token_hash 不一致 / photobook_id 不一致 / 20 回使い切り /
  30 分経過 / revoked_at セット済
- 並行 consume が race-free（goroutine × 30 で 20 個成功 + 10 個失敗）

**UseCase test（実 DB + Turnstile fake）**:
- IssueUploadVerificationSession 成功（Turnstile success → token 発行 → DB INSERT）
- IssueUploadVerificationSession 失敗（Turnstile failure → ErrUploadVerificationFailed
  / DB に行を作らない）
- ConsumeUploadVerificationSession 成功 / 失敗パス

**migration**:
- `goose up` / `down` ラウンドトリップ
- CHECK 制約検証（負の used_intent_count、used > allowed が拒否される）

### 11.2 PR20 で書かないテスト

- Cloudflare 実 API への接続テスト（PR21 で staging 環境に対して実施）
- HTTP handler integration test（PR21）
- Frontend widget E2E（PR22）
- Safari / iPhone Safari widget 表示（PR22）

---

## 12. Security / abuse 対策

### 12.1 Turnstile bypass 禁止

- fail-open は禁止（Cloudflare 障害時は 503 + 再試行案内、§5.4）
- test sitekey / test secret の本番混入防止: Secret Manager の environment 別管理
  + Cloud Run env vars の検証（CI で値の prefix チェックを後続追加）

### 12.2 ログに出さない

- raw upload verification token
- session_token_hash の中身（debug 困難になっても優先）
- Turnstile response body（特に error-codes / hostname / action）
- Cloudflare に送る remoteip
- Cookie 値全般

### 12.3 CSRF / SameSite / CORS

- 案 A（header 配送）: CSRF risk は低い（Authorization header は cross-origin で
  自動送信されない）
- 案 B（HttpOnly Cookie）: SameSite=Strict + Origin / Referer 検証
- CORS: PR21 で `app.vrc-photobook.com` のみを `Access-Control-Allow-Origin` に
  許可（PR21 で R2 endpoint も別途許可）

### 12.4 Bot 連打 / rate limit

- Turnstile 自体が一次防御
- upload_verification_sessions の発行頻度に rate limit を別途 PR23 / PR24 で
  追加検討（IP ベース or 時間ベース）
- 過度な失敗は UsageLimit 集約に記録（ADR-0005 §Turnstile 検証 失敗時 通り、
  PR24 以降で UsageLimit 接続）

### 12.5 expired / revoked cleanup

- 30 分超過した session は誰も consume できないため放置でも実害なし
- DB 容量対策として **月次 cleanup batch** を PR23 / PR24 で追加検討
  （`DELETE WHERE expires_at < now() - 7 days`）

### 12.6 IP / UA hash 保存（§15 Q3）

選択肢:

| 案 | DB に IP/UA hash 保存 | 利点 | 欠点 |
|---|---|---|---|
| **案 A（推奨）** | 保存しない | プライバシー側に倒せる / DB 列増えない | 同一 IP からの大量発行を後追いできない |
| 案 B | 保存する（SHA-256 hash） | 後追い分析可 | プライバシー懸念 + GDPR 対応コスト |

→ **案 A 推奨**。MVP では UsageLimit / Turnstile 自体で十分とみなす。

---

## 13. Cloud SQL 残置/一時削除判断

### 13.1 PR20 計画書完了時点での判断材料

- **PR20 実装にすぐ進むなら残置**（migration / Repository / UseCase test を
  連続実行するため、DB が生きている方が手戻り少）
- **数日空くなら一時削除**
- 費用目安: 残置 ¥55/日。30 日放置で ¥1,650
- 再作成手順: 約 10 分（PR18 / PR19 と同じ）

### 13.2 推奨

**残置継続**（PR19 完了直後判断と整合、PR20 実装に連続着手予定）。
次回判断タイミング: 「PR20 実装 PR の完了時 or 2 日後」の早い方。

---

## 14. PR20 実装範囲（明確化）

### PR20 で実装する

- migration: `00011_create_upload_verification_sessions.sql`
- domain:
  - `UploadVerificationSession` entity
  - VO 群（4 個）
- sqlc: query 5〜6 本 + generate
- Repository: `UploadVerificationSessionRepository`（Create / FindActive /
  Consume / Revoke）
- Turnstile client interface + Cloudflare 実装 + fake
- UseCase: `IssueUploadVerificationSession` / `ConsumeUploadVerificationSession`
- tests: domain unit / repository / UseCase / Turnstile fake / migration
- README に PR20 完了メモ（必要なら）

### PR20 で実装しない

- public API endpoint
- Frontend Turnstile widget
- Cloudflare Dashboard 上の widget 作成
- Secret Manager への TURNSTILE_SECRET_KEY 本登録
- Cloud Run revision 更新
- R2 接続
- presigned URL / complete-upload
- image-processor
- Outbox events / SendGrid
- Safari 実機検証（widget 表示なし）
- Cloud SQL / spike / 既存 Cloud Run 削除

---

## 15. ユーザー判断事項（PR20 着手前に確認）

| # | 判断対象 | 推奨案 | 代替案 | 影響 |
|---|---|---|---|---|
| Q1 | upload verification token の配送方法 | **案 A: response body 返却 + Frontend が `Authorization: Bearer` で送る** | 案 B: HttpOnly Cookie | Frontend 実装と Safari 確認の方針が変わる |
| Q2 | Safari Private Browsing で sessionStorage が消える前提を許容するか | **許容**（再検証で再発行、UX 上は妥協） | 許容しない（→ 案 B Cookie に切替） | UX 軽微悪化 vs 設計シンプル |
| Q3 | IP / UA hash を DB に保存するか | **保存しない**（プライバシー優先） | 保存する（後追い分析） | GDPR 対応 / 防御強度 |
| Q4 | manage session からの upload verification 発行 | **不可**（draft session のみ） | 限定的に許可 | published 後の編集は MVP 不可（業務知識 v4） |
| Q5 | 20 intent / 30 分の固定値 | **ADR-0005 通り 20 / 30** | 別の値 | UX / 防御のバランス |
| Q6 | Turnstile siteverify timeout | **3 秒 + retry なし** | 5 秒 / retry あり | Cloud Run 60 秒 timeout 内 |
| Q7 | Cloudflare 障害時の挙動 | **fail-closed**（503 + 再試行案内） | fail-open（許可） | Bot 検証の意味 |
| Q8 | siteverify への remoteip 送信 | **送る**（X-Forwarded-For 末尾 / Cf-Connecting-IP） | 送らない | Cloudflare 不正検知精度 |
| Q9 | hostname 検証 | **`app.vrc-photobook.com` 厳格一致** | 緩い | spoofing 防止 |
| Q10 | action 値 | **`upload` 固定**（widget 側で設定） | 別の値 | 用途分離 |
| Q11 | PR20 で public endpoint 追加 | **しない**（PR21 で upload-intent と統合） | する | API 二度手間回避 |
| Q12 | PR20 で Cloudflare Dashboard widget 作成 | **しない**（PR21 で実操作） | する | 操作タイミング集約 |
| Q13 | sqlc set | **既存 photobook set に統合** or **新 upload-verification set** | 別 set | 設計関心分離 vs 統合運用 |
| Q14 | Cloud SQL 残置 | **残置継続**（PR20 連続着手） | 一時削除 | 手戻り回避 |
| Q15 | PR20 着手タイミング | **計画書承認後すぐ** | 数日後 | Cloud SQL 残置期間 |

Q1〜Q15 への回答後、PR20 実装に進む。

---

## 16. 関連

- [ADR-0005 画像アップロード方式](../adr/0005-image-upload-flow.md)
- [Image 集約 計画](./m2-image-upload-plan.md)
- [Photobook ↔ Image 連携 計画](./m2-photobook-image-connection-plan.md)
- [Image ドメイン設計](../design/aggregates/image/ドメイン設計.md)
- [Image データモデル設計](../design/aggregates/image/データモデル設計.md)
- [Photobook + Session 接続実装計画](./m2-photobook-session-integration-plan.md)
- [Post-deploy Final Roadmap](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
- [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
- [`.agents/rules/testing.md`](../../.agents/rules/testing.md)
