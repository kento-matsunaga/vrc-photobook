# M1 スパイク検証計画

> 上流: 業務知識 v4 / ADR-0001〜0005 / 各集約・横断設計書（コミット `11f8f2b` 時点）
>
> 本書は M1 における**検証計画**であり、本実装の手順書ではない。M1 では本格コードを書かず、最小 PoC で技術前提の成否のみを判定する。検証結果は ADR / 設計書にフィードバックし、結果次第で M2 以降の実装方針を更新する。

---

## 1. M1 スパイクの目的

ADR-0001〜0005 と業務知識 v4、各集約・横断設計が**実環境で成立するか**を確認する。設計上の前提が崩れた場合、実装を始める前に方針を変えられるよう、リスクの早期検出に集中する。

### 達成したいこと

- Next.js 15 App Router + Cloudflare Pages の組み合わせが SSR / OGP / Cookie / ヘッダ制御の要件を満たすかを判定する
- token→session 交換方式（ADR-0003）が Safari / iPhone Safari を含む実機で成立するかを判定する
- Go Cloud Run + R2 presigned URL の最小フローが成立するかを判定する
- Turnstile セッション化（ADR-0005）と `upload_verification_sessions` のアトミック消費が成立するかを判定する
- Outbox + 自動 reconciler の起動基盤（U11）の選定方針を確定する
- ADR-0004 Proposed のメールプロバイダ選定要件を整理し、検証完了後に Accepted 化する

### 達成しないこと（M1 の対象外）

- 各集約の本格実装（M2 以降）
- 全 25 API の実装
- Frontend の全 16 画面の実装
- 本番環境のプロビジョニング（M1 はステージング相当の最小環境のみ）

---

## 2. 検証テーマ一覧（優先順位付き）

ユーザー指示の通り、最初の 2 つが崩れると Frontend 構成そのものを見直す必要があるため、ここを最優先する。

| 順位 | テーマ | 失敗時の影響範囲 | 想定工数 |
|:---:|--------|------------------|---------:|
| 1 | **Next.js 15 App Router + Cloudflare Pages**（SSR / OGP / Cookie / ヘッダ制御） | Frontend Deploy 先の見直し（Cloud Run / VPS / Fly.io への切替）。ADR-0001 改訂 | 2〜3 日 |
| 2 | **Cookie / Session 検証**（HttpOnly / SameSite=Strict / Safari ITP） | token→session 交換方式の見直し。ADR-0003 改訂、Session 機構設計の再構築 | 1〜2 日 |
| 3 | **Backend Cloud Run + Go chi + PostgreSQL + R2 接続** | バックエンド構成の見直し。ADR-0001 の Backend Deploy 候補切替 | 2〜3 日 |
| 4 | **R2 presigned URL** + complete 検証 | 画像アップロード方式の見直し。ADR-0005 改訂、Image 集約の upload-intent / complete UseCase 再設計 | 2 日 |
| 5 | **Turnstile + upload_verification_session** アトミック消費 | upload-verification 機構の方針変更。ADR-0005 補強 | 1 日 |
| 6 | **UUIDv7 / Web Crypto API / 乱数 / SHA-256** | 一括見直しは少ないが、ADR-0001 / ADR-0003 の細部補強 | 0.5 日 |
| 7 | **Outbox + 自動 reconciler の起動基盤（U11）** | reconcile-scripts.md §3.7.5 の確定、U11 解消 | 1〜2 日 |
| 8 | **Email Provider 選定（ADR-0004 Proposed → Accepted 化準備）** | ADR-0004 を Proposed のまま M2 に持ち越すか、M1 で先行確定するかの判断 | 1〜2 日（並行調査） |

**合計想定工数**: 約 10〜15 営業日（並行可能な部分は短縮可）。

### 優先順位の根拠

- **1〜2 が最優先**: Cloudflare Pages + Next.js + token→session 交換が前提とする要件（HttpOnly Cookie + SameSite=Strict + redirect で URL から token 除去）が実機で動かないと、Frontend 構成と Session 機構の両方を作り直すことになる。M2 以降の手戻りコストが最大になるため、ここを最初に潰す。
- **3〜4 は中位**: Cloud Run / R2 は採用例が多く、Go chi + sqlc + pgx も実績豊富。ただし HEIC 変換の libheif 同梱コンテナや Cloud Run と R2 のクロスクラウドレイテンシは未知数なので、本実装前に確認する。
- **5〜7 は後回し**: Turnstile / Outbox / 起動基盤は、上位が成立してから検証する方が手戻りが少ない。
- **8 は並行**: ADR-0004 は Proposed のため M1 で実装に乗せず、書類調査ベースで M1 期間中に並行評価する。

---

## 3. 各検証テーマの背景

### 3.1 Next.js 15 App Router + Cloudflare Pages

ADR-0001 で Frontend Deploy 第一候補として Cloudflare（エッジ配信・無料枠・R2 親和）を選定。Next.js 15 App Router の Cloudflare 対応は当初 OpenNext / `@cloudflare/next-on-pages` の 2 系統候補があったが、**M1 PoC 検証（2026-04-25）で `@cloudflare/next-on-pages` は deprecated 確認、OpenNext adapter（`@opennextjs/cloudflare`）でのターゲットは Cloudflare Workers + Static Assets binding に確定した**。本セクションでは OpenNext 経由の Cloudflare Workers 構成を前提とする。

業務知識 v4 §7.6 / §6.13 により、ページ種別ごとに以下を出し分ける必要がある。

- 通常ページ（閲覧・LP）: `Referrer-Policy: strict-origin-when-cross-origin` / `X-Robots-Tag: noindex, nofollow` / `<meta name="robots" content="noindex">`
- token 付き URL（`/draft/{token}`, `/manage/token/{token}`）: `Referrer-Policy: no-referrer` / 外部リソース読み込み禁止 / `noindex`

加えて OGP（`og:image` / `og:title` / `og:description` / `twitter:card`）を Photobook ごとに動的に出す必要があり、SSR が必須。

### 3.2 Cookie / Session 検証

ADR-0003 の token→session 交換は `HttpOnly` / `Secure` / `SameSite=Strict` / `Path=/` の Cookie に依存する。Safari ITP（Intelligent Tracking Prevention）と iOS Safari の Cookie 挙動は差異が大きいため、実機検証が必須。

特に懸念:

- Safari ITP がサードパーティ Cookie 扱いで session を 7 日未満で失効させないか
- iPhone Safari で `SameSite=Strict` + redirect の組み合わせが期待通り動くか
- Cloudflare Pages（Frontend）と Cloud Run（Backend API）が **異なるホスト** になる場合、Cookie の Domain 属性をどう設定するか（U2 関連）

### 3.3 Backend Cloud Run + Go chi

ADR-0001 で Cloud Run 第一候補。Go 1.24+ / chi / pgx / sqlc / goose の最小構成で `/health` を公開し、Cloud SQL（PostgreSQL）と Cloudflare R2 への接続を確認する。クロスクラウド構成（GCP ↔ Cloudflare）のレイテンシ計測も含む（ADR-0001 §クロスクラウド注意）。

### 3.4 Cloudflare R2 presigned URL

ADR-0005 で R2 presigned URL 方式を確定。Go SDK（aws-sdk-go-v2 の S3 互換）から R2 へ署名 URL を発行し、フロントから直接 PUT、その後 complete API で HeadObject 確認する流れを最小実装する。

特に懸念:

- R2 の S3 互換 API で `Content-Length` / `content-length-range` 制約がどこまで効くか（ADR-0005 §Content-Length 検証 / U7）
- presigned URL の有効期限 15 分が SDK で正しく設定できるか
- ログに presigned URL が漏れない構成（構造化ログのフィールド除外、Sentry スクラブ）

### 3.5 Turnstile + upload_verification_session

ADR-0005 で Turnstile セッション化（30 分 / 20 intent / Photobook 紐付け）を採用。Cloudflare Turnstile の Server-side 検証 API を Go から呼び、検証成功時に `upload_verification_sessions` テーブルに INSERT し、upload-intent ごとに `usedIntentCount` をアトミック UPDATE する流れを検証する。

特に懸念:

- `UPDATE ... SET used = used + 1 WHERE used < allowed` の条件付き UPDATE で 0 行 → 回数超過判定が並列実行時に正しく機能するか（U5）
- Turnstile の Server-side 検証レイテンシが Cloud Run から許容範囲か

### 3.6 UUIDv7 / Web Crypto API / 乱数

ADR-0001 で UUIDv7（DB 内部 ID）採用、ADR-0003 で session token 用の 256bit 暗号論的乱数採用。Backend（Go の `crypto/rand` + UUIDv7 ライブラリ）と Frontend（Cloudflare Workers の Web Crypto API）の両方で乱数生成・SHA-256 ハッシュ化が動くかを確認する。

### 3.7 Outbox + 自動 reconciler 起動基盤

ADR-0001 / 横断設計（reconcile-scripts.md §3.7.5）で MVP 基本案 Cloud Run Jobs + Cloud Scheduler。M1 では「自動 reconciler が cron で起動して `outbox_events` を読み、failed → pending 再投入する最小フロー」の動作確認を行う。

特に確認事項:

- Cloud Run Jobs の起動信頼性（Scheduler の遅延・失敗率）
- 多重起動防止（U11）が必要か（advisory lock で十分か、Job 側の排他制御で済むか）
- GitHub Actions cron / 専用 worker と比較する評価軸の整理

### 3.8 Email Provider 選定

ADR-0004 Proposed。M1 ではコード実装はせず、書類調査ベースで以下を整理する。

- Resend / AWS SES / SendGrid / Mailgun の本文ログ保持仕様
- 送信履歴 UI に本文が残るか
- API ログに本文が残るか
- 本文ログ保持期間を制御できるか
- バウンス / 苦情処理の Webhook 仕様
- 日本語メールの到達性（Gmail / Yahoo! JP / iCloud / Outlook）

実機テスト送信は M1 後半 or M2 早期に行う想定。

---

## 4. 検証手順

### 4.1 Next.js 15 App Router + Cloudflare Pages（最優先）

1. `harness/spike/frontend/` を新規作成、Next.js 15 App Router で初期化（本実装用 `frontend/` は M2 まで触らない）
2. **アダプタ確定: OpenNext (`@opennextjs/cloudflare`) 一本**（2026-04-25 検証完了）
   - 経緯: `@cloudflare/next-on-pages` は **deprecated**（Cloudflare 公式が OpenNext adapter 推奨へ切替済）
   - v1 next-on-pages 版 PoC（コミット `c7ba16b`）と v2 OpenNext adapter 版 PoC（コミット `6e2840a`）の両方で SSR / OGP / Cookie / redirect / ヘッダ制御がすべて成立することを CLI 検証で確認済み
   - **M2 本実装の Frontend は `@opennextjs/cloudflare` で構築**。`@cloudflare/next-on-pages` は採用しない
   - PoC 結果: `harness/spike/frontend/README.md` §検証履歴 / §検証結果（OpenNext adapter 版）
   - 詳細経緯: `harness/failure-log/2026-04-25_cloudflare-next-on-pages-deprecated.md`
   - **OpenNext で確定した実装ルール**:
     - `export const runtime = "edge"` を**指定しない**（指定するとビルドエラー）
     - レスポンスヘッダ制御は `middleware.ts` に一本化する（`next.config.mjs` の `headers()` と二重化すると値が重複する）
     - OGP の絶対 URL は `metadata.metadataBase = new URL(process.env.NEXT_PUBLIC_BASE_URL)` で解決する（PoC で localhost URL が焼き込まれる挙動を確認済み）
3. **検証用ルートを作成**:
   - `/p/sample-slug`: 公開ページ風、`generateMetadata` で OGP メタタグ動的出力、`X-Robots-Tag: noindex, nofollow` を返す
   - `/draft/sample-token`: token 受け取り → ダミー検証 → Cookie 発行 → `/edit/sample-photobook-id` redirect
   - `/manage/token/sample-token`: 同様、`/manage/sample-photobook-id` redirect
   - `/edit/sample-photobook-id`: Cookie 検証ページ
4. **ヘッダ制御の差別化**:
   - 通常ページ: `Referrer-Policy: strict-origin-when-cross-origin`
   - token 付き / `/edit/*` / `/manage/*`: `Referrer-Policy: no-referrer`
5. Cloudflare Pages にデプロイし、各ルートで実機検証
6. **計測**: TTFB、SSR レンダリング時間、OGP メタタグの実際の HTML 出力
7. **OGP の絶対 URL 解決方針**（2026-04-25 検証で発見）: PoC では `og:image` が dev サーバ URL（`http://localhost:3000/...`）として焼き込まれた。Next.js Metadata API はベース URL を環境変数等から解決する設計であり、本実装では `metadata.metadataBase = new URL(process.env.NEXT_PUBLIC_BASE_URL)` を必ず設定する。M2 本実装の必須要件として記録

### 4.2 Cookie / Session 検証（最優先）

1. 4.1 のルートに `Set-Cookie` 発行を組み込む（`HttpOnly` / `Secure` / `SameSite=Strict` / `Path=/`）
2. **redirect 後の Cookie 引き渡し検証**: `/draft/{token}` → 302 + Set-Cookie → `/edit/{photobook_id}` で Cookie 読取
3. **Safari / iPhone Safari 実機検証**:
   - macOS Safari（最新）
   - iOS Safari（iOS 17 / iOS 18 推奨）
   - 7 日間放置後も Cookie が残るか
   - ITP がサードパーティ Cookie として扱わないか
4. **Cookie Domain 属性の検証**:
   - Frontend = `*.pages.dev` / Backend = `*.run.app` の異なるホスト構成で、Cookie の Domain 未指定がどう動くか
   - 共通の親ドメイン（独自ドメイン）を切るべきかの判断
5. **CSRF 対策**: `SameSite=Strict` + Origin ヘッダ検証の組み合わせを確認

### 4.3 Backend Cloud Run + Go chi

> **2026-04-25 検証完了**（コミット `c2a5919`）。`harness/spike/backend/` でローカル CLI + Docker container の最小構成成立を確認。詳細は `harness/spike/backend/README.md` §検証結果。残タスクは実 Cloud Run デプロイ・コールドスタート計測・Cloud SQL 接続方式・Cloud Logging slog JSON パース・SIGTERM 実機確認・東京 ↔ R2 レイテンシ。R2 接続 PoC（§4.4）で一部を扱う。

1. `harness/spike/backend/` を新規作成、Go 1.24+ で初期化（PoC ローカルは Go 1.23.2 で成立確認、M2 で 1.24 へ移行）。本実装用 `backend/` は M2 まで触らない
2. `cmd/api/main.go` に最小 chi サーバを実装、`/healthz` エンドポイントを公開
3. `pgx` で PostgreSQL に接続（Cloud SQL Auth Proxy 経由 or 直結）
4. `sqlc` で 1 つの最小クエリを生成（例: `SELECT 1` をラップ）
5. `goose` で 1 つの migration を実行（例: `_test_alive` テーブル作成）
6. Dockerfile を書き、Cloud Run にデプロイ
7. **R2 接続検証**: aws-sdk-go-v2 で R2 の HeadBucket / ListObjects を実行
8. **計測**: Cloud Run コールドスタート時間、R2 までのレイテンシ（東京リージョン）

### 4.4 R2 presigned URL

1. 4.3 の Backend に `/api/sandbox/upload-intent`（POST）と `/api/sandbox/complete`（POST）を追加
2. upload-intent: 12B 乱数 + UUIDv7 で `storage_key` を生成、presigned URL を 15 分有効で発行
3. フロント（curl or 検証用 HTML）から R2 へ直接 PUT
4. complete: HeadObject で R2 オブジェクト存在確認 → ContentLength 検証 → `available` 状態に遷移（DB に最小レコード）
5. **境界テスト**:
   - 10MB ぴったりのファイル
   - 10MB + 1 バイトのファイル（拒否されるべき）
   - 期限切れ presigned URL（PUT が失敗するべき）
   - 異なる photobook_id への流用（complete が拒否するべき）
6. **ログ検証**: 構造化ログに presigned URL / storage_key / token が出ていないか grep で確認

### 4.5 Turnstile + upload_verification_session

1. Cloudflare Turnstile のテストキー（公開されているサンドボックスキー）でフロントウィジェットを表示
2. 4.3 の Backend に `/api/sandbox/upload-verification`（POST）を追加、Turnstile Server 検証 API を呼ぶ
3. 成功時、`upload_verification_sessions` テーブルに INSERT（最小スキーマ、auth/upload-verification データモデル §3 に準拠）
4. `validateAndConsume(rawToken, photobookId)`: アトミック UPDATE 実行、20 回まで成功、21 回目で 403
5. **並列テスト**: 100 並列 upload-intent を投げて、`used_intent_count` が壊れず厳密に 20 件成功・80 件失敗になるか（U5）

### 4.6 UUIDv7 / Web Crypto API / 乱数

1. Backend Go で `github.com/google/uuid`（v7 サポート）または独自実装で UUIDv7 生成
2. `crypto/rand` で 32 バイト乱数 → base64url 化 → SHA-256 → bytea 32B
3. Frontend Cloudflare Pages で `crypto.subtle.digest("SHA-256", ...)` が動くか確認
4. 単純なベンチマーク（1000 回生成の所要時間）

### 4.7 Outbox + 自動 reconciler 起動基盤

1. 4.3 の Backend に最小 `outbox_events` テーブルを migration、最小 INSERT エンドポイントを追加
2. `cmd/worker/outbox_dispatcher` を作成、`SELECT FOR UPDATE SKIP LOCKED` で pending を取得 → status='processed' 遷移
3. **Cloud Run Jobs + Cloud Scheduler 検証**:
   - Cloud Scheduler から 5 分ごとに Cloud Run Jobs を起動
   - 起動信頼性・遅延・失敗時の retry 動作を計測
4. **比較対象の最小調査**: GitHub Actions cron と専用 worker（VPS）の運用コスト比較
5. **多重起動防止検証**: 同時に 2 つの Job が走った場合、`SKIP LOCKED` で衝突しないかを実測

### 4.8 Email Provider 選定（並行調査）

1. 各プロバイダの公式ドキュメントから以下を抽出（書類調査）:
   - 送信履歴 UI の本文表示仕様
   - API ログの保持期間と本文出力可否
   - Webhook ペイロードの内容
   - バウンス / 苦情通知の仕組み
2. サポート問い合わせ（必要なら）
3. 各プロバイダで簡易テストアカウントを作成し、テスト送信 1 通だけ実行（本文に管理 URL ダミーを含めない、漏洩テスト対策）
4. ADR-0004 の比較表を埋めて、Accepted 化の条件を整理

---

## 5. 成功条件

各検証テーマの成功条件を明確にし、満たしたら次フェーズに進める。

### 5.1 Next.js + Cloudflare Pages

| # | 条件 |
|---|------|
| ✅ | `/p/{slug}` で SSR が動き、`generateMetadata` で OGP メタタグが動的に出力される |
| ✅ | `X-Robots-Tag: noindex, nofollow` と `<meta name="robots" content="noindex">` が両方出る |
| ✅ | ページ種別ごとに `Referrer-Policy` を `strict-origin-when-cross-origin` / `no-referrer` で出し分けられる |
| ✅ | `/draft/{token}` でサーバ側で token 受け取り → Set-Cookie → 302 redirect が成功 |
| ✅ | redirect 後の `/edit/{photobook_id}` で Cookie が読み取れる |
| ✅ | 上記すべてが macOS Safari / iOS Safari / Chrome で動く |

### 5.2 Cookie / Session

| # | 条件 |
|---|------|
| ✅ | `HttpOnly: true` / `Secure: true` / `SameSite: Strict` / `Path: /` の Cookie が iOS Safari でも 7 日間維持される |
| ✅ | redirect 後の Cookie 引き渡しが Safari で破綻しない |
| ✅ | Origin ヘッダ検証が API でできる |
| ✅ | Cookie Domain 未指定で、Cloudflare Pages と Cloud Run の異なるホスト構成が成立する（または独自ドメインが必要かを確定） |

### 5.3 Backend Cloud Run

| # | 条件 |
|---|------|
| ✅ | `/healthz` が Cloud Run でレスポンスを返す |
| ✅ | PostgreSQL 接続が成立する（接続プール、TLS 含む） |
| ✅ | sqlc 生成コードがビルドできる |
| ✅ | goose で migration の up/down が動く |
| ✅ | Cloud Run から R2 への HeadBucket / ListObjects が成立する |
| ✅ | Cloud Run 東京 ↔ R2 のレイテンシが p50 で 200ms 以下 |

### 5.4 R2 presigned URL

| # | 条件 | 2026-04-25 検証結果 |
|---|------|------|
| ✅ | Backend Go から presigned URL（PUT、15 分有効）を発行できる | **達成**。`/sandbox/r2-presign-put` で 519 bytes の URL を取得、`X-Amz-Algorithm` / `X-Amz-Credential` / `X-Amz-Signature` / `X-Amz-Expires` を含み、`expires_in_seconds=900`（15 分）|
| ✅ | フロントから R2 への直接 PUT が成功する | **達成**。1024 bytes 合致で `200 OK`。aws-sdk-go-v2 が `Content-Length` を SignedHeaders に含めるため、宣言サイズと実 PUT サイズの一致が必須（不一致時は R2 が `403 SignatureDoesNotMatch` を返す挙動を実機確認） |
| ✅ | complete 時の HeadObject 確認が成立する | **達成**。`/sandbox/r2-headobject` で `content_length=1024 / content_type=image/png / etag` を取得 |
| ✅ | 10MB 超過 PUT が拒否されるか、complete 時の検証で `failed(file_too_large)` 判定できる | **達成**。`byte_size=11000000` で 400 `file_too_large`（presign 段階で拒否） |
| ✅ | 期限切れ presigned URL での PUT が失敗する | （未確認） 15 分待機が必要なため M1 では未検証。本実装での E2E テストで補強する |
| ✅ | ログに presigned URL / storage_key / token が出ない | **達成**。`grep -E 'X-Amz-Signature\|X-Amz-Credential\|presigned\|access[_-]?key'` で 0 ヒット。`R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_ENDPOINT` / `storage_key` の各値で `grep -F` count もすべて 0 |
| ✅ | バリデーション全パターン | **達成**。SVG / GIF → `unsupported_format`、path traversal → `key_traversal_forbidden`、prefix invalid → `key_prefix_invalid`、byte_size=0 → `byte_size_invalid`、filename 空 → `filename_required`、存在しない key → 502 `r2_headobject_failed`（ADR-0005 の方針通り「分類キーのみ」） |

### 5.5 Turnstile + upload_verification_session

| # | 条件 | 2026-04-25 検証結果 |
|---|------|------|
| ✅ | Turnstile サーバ検証 API が Backend から呼べる | **達成**。Cloudflare 公式公開サンドボックス secret（必ず success: `1x0000...AA` / 必ず failure: `2x0000...AA`）で `siteverify` 実呼び出しを確認。always-pass は `200` + セッション発行、always-fail は `403 turnstile_rejected`。サーバ側 slog のみ `error_codes:["invalid-input-response"]` を残し、クライアントには返さない |
| ✅ | `upload_verification_sessions` への INSERT と SHA-256 hash 保存が動く | **達成**。32 バイト乱数を `crypto/rand` → `base64.RawURLEncoding`（43 文字）→ `sha256.Sum256` で 32 バイト bytea として永続化。raw token は DB に残さない。`encode(session_token_hash, 'hex')` で 64 桁の hex のみ確認、`DUMMY_*` / `MOCK_FAIL` / `tampered_*` 由来の文字列は 0 件 |
| ✅ | アトミック UPDATE で 20 回成功・21 回目失敗が厳密に成立する | **達成**。逐次 21 回 consume の結果は `[1〜20] 200 used_intent_count=N / [21] 403 consume_rejected`。SQL は単一 UPDATE（`session_token_hash` + `photobook_id` + `revoked_at IS NULL` + `expires_at > now()` + `used < allowed`）の 5 条件 AND を一発で評価して `used_intent_count` を +1 する設計。0 行返却 (`pgx.ErrNoRows`) を 403 に集約 |
| ✅ | 100 並列実行でも `used_intent_count` が壊れない | **達成**。`scripts/turnstile-consume-race.sh` で 100 並列 consume を発射、`HTTP 200=20 / HTTP 403=80 / その他=0` を確認。PostgreSQL Read Committed の単一行 UPDATE で原子性が保証される |
| ✅ | mock モード（`TURNSTILE_SECRET_KEY` 空）でローカル PoC が完結する | **達成**。起動時 WARN `running in MOCK mode (PoC only)` を表示、`MOCK_FAIL` 含み token のみ 403、それ以外は 200。Cloudflare アカウントが無くてもローカル検証可能 |
| ✅ | secret / Turnstile token / verification_session_token のログ漏洩なし | **達成**。`grep -E 'TURNSTILE_SECRET\|0000000AA\|verification_session_token\|DUMMY_OK\|MOCK_FAIL\|tampered'` で 0 ヒット。出力されるのは「設定状態（configured / not configured）」と「拒否理由カテゴリ（`reason=no_rows`）」のみ |
| ✅ | 拒否カテゴリのクライアント露出を抑える | **達成**。`hash 不一致 / photobook_id 不一致 / 期限切れ / revoked / 残数枯渇` の 5 通りすべて 403 `consume_rejected` の単一カテゴリで返却。攻撃者は脱落条件を判別できない |

### 5.6 UUIDv7 / Web Crypto API / 乱数

| # | 条件 |
|---|------|
| ✅ | Backend Go で UUIDv7 が生成できる（時系列順序が保証される） |
| ✅ | Backend Go で 32B 乱数 → base64url → SHA-256 → bytea 32B が生成できる |
| ✅ | Cloudflare Pages（Workers ランタイム）で `crypto.subtle.digest` が動く |

### 5.7 Outbox + 自動 reconciler 起動基盤

| # | 条件 | 2026-04-25 検証結果 |
|---|------|------|
| ✅ | `outbox_events` の状態変更と同一 TX INSERT が成立する | **達成（PoC 範囲）**：`harness/spike/backend/migrations/00003_create_outbox_events.sql` で最小スキーマ + CHECK 制約 + 部分インデックスを定義し、`internal/db/queries/outbox.sql` の `CreateOutboxEvent` で INSERT が動作。本実装では集約の状態変更と同一 TX で呼ばれる前提（cross-cutting/outbox.md §2、PoC では sandbox API 経由で単独 INSERT を検証） |
| ✅ | `SELECT FOR UPDATE SKIP LOCKED` で並列ワーカーが衝突しない | **達成**：30 件 enqueue 後に 2 並列で `POST /sandbox/outbox/process-once?limit=30` を発射、event_ids の overlap=0、最終 by_status `processed=30`。CTE 内の `FOR UPDATE SKIP LOCKED` + 直後の UPDATE で `processing` への遷移が原子的に成立 |
| ✅ | Cloud Scheduler + Cloud Run Jobs で 5 分ごと cron 起動が動く | （未確認） M1 残作業。CLI 側は `cmd/outbox-worker --once` / `--retry-failed` で Cloud Run Jobs から呼べる前提の構造に整理済（`scripts/outbox-process-once.sh` ラッパー含む）。実 Cloud Run Jobs での起動は Cloud Run + Workers 実環境デプロイ工程で実施 |
| ✅ | 多重起動が発生しても DB 行ロックで処理が壊れない | **部分達成**：プロセス並列での重複 claim が発生しないことは確認済み。Cloud Run Jobs が同時刻に 2 つ走るケース（スケジューラ重複）における advisory lock の必要性は本実装で評価（reconcile-scripts.md §3.7.6）|
| ✅ | `ImageIngestionRequested` をイベント種別として扱える | **達成**：`event_type=ImageIngestionRequested` として enqueue → claim → MarkOutboxProcessed のフローを 5 件で確認（`attempts=1` で `processed_at` セット） |
| ✅ | failed イベントの retry 候補化 | **達成**：`event_type` に `ForceFail` を含む → MarkOutboxFailed → `RetryFailedOutboxEvents`（`outbox_failed_retry` 自動 reconciler 最小実装）で `failed → pending` に一括戻し → 再 claim でも `attempts +1` され失敗継続することを実機確認 |
| ✅ | payload / Secret / token のログ漏洩なし | **達成**：`payload` 全文は一覧 API もログにも出さない設計。`grep -E 'payload\|X-Amz-*\|presigned\|R2_SECRET\|TURNSTILE_SECRET'` 0 ヒット、Secret 値での `grep -F` count も 0 |

### 5.8 Email Provider

| # | 条件 | 2026-04-25 検証結果（再選定後） |
|---|------|------|
| ✅ | 4 プロバイダの本文ログ保持仕様が ADR-0004 比較表に埋まる | **達成**：Resend / AWS SES / SendGrid / Mailgun + 参考 2 候補（Postmark / Cloudflare Email Service）の公式ドキュメントから dashboard 本文表示・retention・event payload・地域・無料枠を比較表化（ADR-0004 §比較表） |
| - | テスト送信 1 通が成功し、送信履歴 UI / API ログの本文表示を実機確認 | （**未実施**） M2 早期に SendGrid アカウント作成 + 1 通テスト送信 + Email Activity Feed / API ログ / Webhook payload で本文が出ないことを実機確認、bounce/complaint webhook 受信を別タスクで実施 |
| ✅ | 第一候補が決まり、Accepted 化の条件が ADR-0004 に書ける状態になる | **達成（再選定）**：第一候補 SendGrid（Twilio 公式が「本文を保存しない」を明言）、第二候補 Mailgun（Domain 設定で retention 0 day 選択可）、運用不可 AWS SES（Amazon 側申請落ち、技術不採用ではなくアカウント／運用上の利用不可）、不採用 Resend / Postmark / Cloudflare Email Service（理由は ADR-0004 に詳述） |

---

## 6. 失敗時の分岐判断

検証で成功条件を満たさなかった場合の代替案。失敗を放置せず、ADR への反映までを M1 で完了させる。

### 6.1 Next.js + Cloudflare Pages 失敗時

**症状例**: Cloudflare Pages で SSR が動かない、ヘッダ制御が効かない、Safari で Cookie が消える。

**代替案**:
- **案A**: Cloudflare Pages 維持 + OpenNext / @cloudflare/next-on-pages を別系統に切替
- **案B**: **Frontend を Cloud Run に移す**。Next.js 標準の Node.js ランタイムで動かす（最も無難、コスト増）
- **案C**: Frontend を Vercel に移す（Cloudflare 統一原則を崩すが、Next.js 互換性は最強）
- **案D**: Frontend を VPS / Fly.io に移す

**ADR 改訂**: ADR-0001 §Frontend Deploy 第一候補を変更。

### 6.2 Cookie / Session 失敗時

**症状例**: Safari ITP で session が 24 時間で消える、redirect 後に Cookie が引き渡されない。

**代替案**:
- **案E**: Cookie 発行ホストを Backend と統一（独自ドメイン経由で Cloudflare Pages を噛まさない）
- **案F**: token を URL に残す方式に戻す（ADR-0003 全面見直し）
- **案G**: Cookie Domain を独自親ドメインで切る（`*.example.com` 共通）

**ADR 改訂**: ADR-0003 §Cookie 属性 / §URL 設計 / U2 を更新。

### 6.3 Backend Cloud Run 失敗時

**症状例**: Cloud Run コールドスタートが 5 秒超、R2 へのレイテンシが許容外、libheif 同梱コンテナが極端に大きい。

**代替案**:
- **案H**: Backend Deploy 第一候補を VPS or Fly.io に変更（ADR-0001 §Backend Deploy 改訂）
- **案I**: HEIC 変換だけ Cloud Functions / 別 Worker に分離（マイクロサービス化）

**ADR 改訂**: ADR-0001 §Backend Deploy 候補。

### 6.4 R2 presigned URL 失敗時

**症状例**: R2 SDK で `Content-Length` 制約が効かない、presigned URL が期待通り期限切れにならない。

**代替案**:
- **案J**: complete 時の検証を強化（サーバー側 HeadObject + バイトサイズ厳格確認）
- **案K**: Cloudflare Workers 経由で R2 に書き込む方式に変える（presigned URL を使わない）
- **案L**: Storage を S3（AWS）に変更（コスト増、エグレス課金）

**ADR 改訂**: ADR-0005 §Content-Length 検証 / §Storage 採用。

### 6.5 Turnstile アトミック消費 失敗時

**症状例**: 並列で `used_intent_count` が一時的に矛盾、20 回上限が破られる。

**代替案**:
- **案M**: PostgreSQL advisory lock で session 単位の排他を取る
- **案N**: Redis を導入し INCR で上限管理（U5 → Redis 化を MVP に前倒し）
- **案O**: Turnstile セッション化を諦め、毎回再検証（UX 劣化を許容）

**ADR 改訂**: ADR-0005 §Turnstile 検証 / U5。

### 6.6 UUIDv7 / Web Crypto API 失敗時

**症状例**: Cloudflare Workers で `crypto.subtle` が動かない、UUIDv7 のライブラリが Go 1.24 と互換でない。

**代替案**:
- **案P**: Frontend での暗号処理を諦め、Backend で全て生成して Cookie 発行
- **案Q**: UUIDv7 を諦めて UUIDv4 + 別カラムで created_at 管理（B-tree 性能低下を許容）
- **案R**: ULID に切替（ADR-0001 §UUIDv7 改訂）

**ADR 改訂**: ADR-0001 §UUIDv7 / ADR-0003 §Web Crypto API。

### 6.7 Outbox 起動基盤 失敗時

**症状例**: Cloud Scheduler の遅延が許容外、Cloud Run Jobs の起動失敗率が高い。

**代替案**:
- **案S**: 専用 worker（VPS 常駐）に切替
- **案T**: GitHub Actions cron に切替（信頼性は劣るが課金枠が緩い）
- **案U**: Backend Cloud Run サービスに常駐 worker を同居（小規模 MVP では可）

**ADR 改訂**: reconcile-scripts.md §3.7.5 / U11 解消。

### 6.8 Email Provider 失敗時

**症状例**: 4 プロバイダすべてで本文ログ保持を完全に止められない。

**代替案**:
- **案V**: 「管理 URL を本文に直接含めない」設計に変更（再発行リンクを送付、クリックで管理 URL 表示）
- **案W**: 自前 SMTP（高難度、MVP 不可）
- **案X**: 本文ログ保持を許容する代わりに、プロバイダのログ保持期間を 1 日に短縮できるプロバイダ縛り

**ADR 改訂**: ADR-0004 比較表 + 採用結果。

---

## 7. 作成する最小 PoC 一覧

検証中に作る最小実装の一覧。M1 完了後、これらは `harness/spike/` 配下に保管し、本実装には流用しない。

| PoC 名 | 目的 | 配置先 |
|--------|------|--------|
| `frontend-spike` | Next.js 15 App Router + Cloudflare Pages の SSR / OGP / Cookie / ヘッダ制御 | `harness/spike/frontend/` |
| `backend-spike` | Go chi + sqlc + pgx + goose の最小起動 + Cloud Run デプロイ | `harness/spike/backend/` |
| `r2-presigned-spike` | R2 presigned URL 発行 + 直接 PUT + complete 検証 | `backend-spike` 内に統合 |
| `turnstile-spike` | Turnstile Server 検証 + `upload_verification_sessions` アトミック消費 | `backend-spike` + `frontend-spike` 統合 |
| `outbox-worker-spike` | `cmd/worker/outbox_dispatcher` の最小実装 + Cloud Run Jobs 起動 | `harness/spike/worker/` |
| `email-provider-eval` | 4 プロバイダ調査結果の比較表 | `docs/adr/0004-email-provider.md` 内に追記 |

各 PoC は **本実装と独立**。M2 以降の本実装は `backend/`, `frontend/` を新規作成する想定で、PoC コードは流用しない（PoC は粗い実装が許される、本実装は ルール遵守が必須）。

---

## 8. 触る予定のファイル / ディレクトリ

### 新規作成

```
harness/spike/                            # 全 PoC の置き場
├── frontend/                             # Next.js 検証
│   ├── package.json
│   ├── next.config.mjs
│   ├── app/
│   │   ├── p/[slug]/page.tsx
│   │   ├── draft/[token]/route.ts
│   │   ├── manage/token/[token]/route.ts
│   │   ├── edit/[photobook_id]/page.tsx
│   │   └── manage/[photobook_id]/page.tsx
│   └── README.md                         # 検証手順と結果
├── backend/                              # Go 検証
│   ├── go.mod
│   ├── cmd/api/main.go
│   ├── internal/health/handler.go
│   ├── internal/sandbox/                 # 4.4〜4.5 用
│   ├── migrations/
│   ├── sqlc.yaml
│   ├── Dockerfile
│   └── README.md
└── worker/                               # 4.7 用
    ├── cmd/dispatcher/main.go
    └── README.md
```

### 既存ファイルへの追記（M1 完了後）

- `docs/adr/0001-tech-stack.md` §未解決事項 / §M1 で必要なスパイク → 検証結果を追記
- `docs/adr/0003-frontend-token-session-flow.md` §13 未解決事項（U2 等） → 検証結果を追記
- `docs/adr/0004-email-provider.md` 比較表 + Proposed → Accepted 化判断
- `docs/adr/0005-image-upload-flow.md` §未解決事項（U7 等） → 検証結果を追記
- `docs/design/cross-cutting/reconcile-scripts.md` §3.7.5 / §11 U11 → 起動基盤確定
- `harness/QUALITY_SCORE.md` → M1 検証結果サマリ

### 触らないもの

- `backend/`（本実装、まだ作らない）
- `frontend/`（本実装、まだ作らない）
- `docs/design/aggregates/` の各設計書（M1 で原則変更しない、結果が著しい影響を持つ場合のみ追記）

---

## 9. 検証に必要な環境変数一覧

**重要**: 実際の値は本書に書かない。Secret Manager / 各環境の `.env.local`（git 管理外）で管理する。

### 9.1 Backend スパイク（`harness/spike/backend/`）

| 環境変数 | 用途 | 取得方法 |
|---------|------|---------|
| `DATABASE_URL` | PostgreSQL 接続 | Cloud SQL Auth Proxy 経由 or 開発用ローカル DB |
| `R2_ACCOUNT_ID` | Cloudflare アカウント ID | Cloudflare ダッシュボード |
| `R2_ACCESS_KEY_ID` | R2 API キー ID | R2 トークン発行画面（M1 用専用キー、本実装と分離） |
| `R2_SECRET_ACCESS_KEY` | R2 API キー Secret | 同上 |
| `R2_BUCKET_NAME` | スパイク用バケット名 | M1 専用バケット（例: `vrcpb-spike`） |
| `R2_ENDPOINT` | R2 のエンドポイント URL | `https://{R2_ACCOUNT_ID}.r2.cloudflarestorage.com` |
| `TURNSTILE_SITE_KEY` | フロント用 Turnstile キー | Cloudflare Turnstile（テストキー or M1 専用） |
| `TURNSTILE_SECRET_KEY` | サーバ検証用 | 同上 |
| `IP_HASH_SALT_V1` | ソルト（v4 §3.7） | M1 はダミー固定値 |
| `IP_HASH_SALT_CURRENT_VERSION` | 現在のソルトバージョン | `1` |

### 9.2 Frontend スパイク（`harness/spike/frontend/`）

| 環境変数 | 用途 |
|---------|------|
| `NEXT_PUBLIC_API_BASE_URL` | Backend スパイクの URL |
| `NEXT_PUBLIC_TURNSTILE_SITE_KEY` | Turnstile ウィジェット用 |

### 9.3 Cloud Run / Cloud Run Jobs

| 環境変数 | 用途 |
|---------|------|
| `GOOGLE_APPLICATION_CREDENTIALS` | サービスアカウント鍵（ローカル検証時のみ、Cloud Run 上では不要） |

### 9.4 共通方針

- すべての Secret は **Secret Manager 経由**で Cloud Run に注入
- ローカルでは `.env.local`（git ignore 済）に配置
- **本書に実際の値は絶対に書かない**
- M1 用キーは本実装と完全分離（漏洩時の影響範囲を限定）

---

## 10. セキュリティ上の注意

M1 検証中も、以下のセキュリティ原則は本実装と同じ厳しさで守る。

### 10.1 ログ出力禁止フィールド

構造化ログ（zerolog / zap 想定）の禁止フィールド:

- `Authorization` ヘッダ全体
- `Cookie` / `Set-Cookie` ヘッダ全体
- `draft_edit_token` / `manage_url_token` / `session_token`（raw）
- `session_token_hash` の base64url 表現
- presigned URL（クエリ文字列の署名部分含む全体）
- `storage_key` （path 推測抑止のため）
- `recipient_email`（24h 後 NULL 化）
- 画像バイナリ（multipart body）
- スタックトレースに含まれる上記の値

実装方法: ログライブラリの middleware でフィールド名ホワイトリスト方式 or マスキング処理。

### 10.2 Sentry / エラートラッキング

- M1 でエラートラッキングを導入する場合（Sentry 等）、`beforeSend` フックで URL の token セグメントをスクラブ
- バウンスメール / Webhook ペイロードに管理 URL が含まれる場合、ログ保管前にマスク処理

### 10.3 Secret Manager

- すべての API キー / Secret は Cloud Run の環境変数経由で **Secret Manager から注入**
- ローカル `.env.local` は `.gitignore` 対象、コミット対象外
- M1 用キーと本実装用キーを分離（M1 漏洩時の影響範囲を限定）

### 10.4 R2 バケット権限

- M1 スパイク用バケット（例: `vrcpb-spike`）と本実装用バケット（M2 以降）を分離
- 公開設定は使わず、署名付き URL or Workers 経由でのみアクセス

### 10.5 Turnstile

- M1 はテストキー（公開された Cloudflare のサンドボックスキー）で検証可能
  - 必ず success を返す secret: `1x0000000000000000000000000000000AA`
  - 必ず failure を返す secret: `2x0000000000000000000000000000000AA`
  - **2026-04-25 PoC で動作確認済み**（コミット `53fa568`）
- **本番 Turnstile widget の発行は M1 PoC 完了時点では不要**。Workers 実環境デプロイ後（hostname 確定後）に Cloudflare Dashboard → Turnstile → Add widget で発行する（widget 名: `vrcpb-spike` 等、hostname に Workers の URL を登録）
- 本番 secret は **チャット・ログに貼らない**。`.env.local`（`.gitignore` 対象）または Secret Manager で扱う
- 本実装用キーへの切替は M2 で実施

### 10.6 Cookie 漏洩対策

- M1 検証中も `HttpOnly` / `Secure` を必須
- 検証用ページに XSS が混入しないよう、外部スクリプトを読み込まない
- 検証用 token は本書 / git に書かない（環境変数 or 一時的な定数で扱う）

---

## 11. Safari / iPhone Safari 検証項目

iOS Safari は ITP（Intelligent Tracking Prevention）と Cookie 挙動の差異が大きいため、専用テスト項目を立てる。

### 11.1 ブラウザマトリクス

| ブラウザ | 優先度 | 検証内容 |
|---------|:-----:|---------|
| iOS Safari（最新） | 必須 | 全項目 |
| iOS Safari（1 世代前） | 必須 | 全項目 |
| iPadOS Safari | 推奨 | Cookie 7 日間維持 |
| macOS Safari（最新） | 必須 | 全項目 |
| Chrome（最新） | 必須 | ベースライン比較 |
| Edge（最新） | 推奨 | Chromium 系で違いがないか |
| Firefox（最新） | 推奨 | SameSite 解釈差 |

### 11.2 Cookie 挙動

| 検証項目 | 期待結果 |
|---------|---------|
| `HttpOnly` Cookie が JS から読めない | `document.cookie` で見えない |
| `Secure` Cookie が HTTP では送られない | HTTPS のみ |
| `SameSite=Strict` で別オリジンからのリンク遷移時に Cookie が送られない | 別タブの直接リンクで Cookie が無くなる |
| 7 日間放置後も session Cookie が残る | iOS Safari ITP で消えない（最大 7 日要件） |
| ITP がサードパーティ Cookie として扱わない | Cookie Domain 設定で First-party になる |
| Cloudflare Pages（`*.pages.dev`）と Cloud Run（`*.run.app`）の異なるホスト構成での Cookie 動作 | Domain 未指定で発行ホストのみ有効、API は同オリジンに見える独自ドメイン経由で送信 |

### 11.3 Redirect 挙動

| 検証項目 | 期待結果 |
|---------|---------|
| `/draft/{token}` → 302 + `Set-Cookie` → `/edit/{photobook_id}` でリダイレクト先で Cookie 読取 | iOS Safari でも成功 |
| 302 ではなく `meta refresh` を使った場合の Cookie 引き渡し | 比較として確認（ただし 302 が正規ルート） |
| 同一ドメイン内のサブパス redirect で Cookie が引き継がれる | `Path: /` で全パス共有 |

### 11.4 ヘッダ制御

| 検証項目 | 期待結果 |
|---------|---------|
| `Referrer-Policy: no-referrer` を iOS Safari が遵守 | 外部リンクへの遷移時に Referer が空 |
| `X-Robots-Tag` ヘッダが Safari の View Source / Web Inspector で確認できる | 全種別で出力 |
| `<meta name="robots" content="noindex">` が DOM に出る | `generateMetadata` 経由で SSR HTML に含まれる |
| iOS Safari の Reader Mode 対応 | OGP メタタグが正しく解釈される（テスト項目だが MVP 必須でない） |

### 11.5 Web Crypto API

| 検証項目 | 期待結果 |
|---------|---------|
| `crypto.subtle.digest("SHA-256", ...)` が iOS Safari で動く | フロントでハッシュ計算が必要な場合に成立 |
| `crypto.getRandomValues(...)` が iOS Safari で動く | session token 生成（フロント側で必要な場合） |

### 11.6 ITP 影響テスト

- 7 日間連続でアクセスせず、8 日目にアクセスした場合に session Cookie が残っているか
- Safari の「履歴と Web サイトデータを消去」を実行した場合の挙動
- プライベートブラウジングでの動作（参考）

---

## 12. ADR へのフィードバック項目

検証結果を反映する ADR / 設計書のセクション一覧。M1 完了時に必ず更新する。

### 12.1 ADR-0001 技術スタック

| フィードバック項目 | 想定更新箇所 |
|------------------|------------|
| Cloudflare Pages SSR の OpenNext / next-on-pages の選択結果 | §M1 で必要なスパイク → 検証結果セクション新設 |
| Cloud Run コールドスタート実測値 | §結果デメリット |
| Cloud Run + R2 クロスクラウドレイテンシ実測値 | §クロスクラウド構成の注意 |
| HEIC デコードコンテナ構築結果 | §未解決事項 → HEIC デコード戦略 |
| UUIDv7 ライブラリ選定（`google/uuid` vs 別実装） | §UUIDv7 採用 |

### 12.2 ADR-0002 運営操作方式

| フィードバック項目 | 想定更新箇所 |
|------------------|------------|
| Cloud Run Jobs での cmd/ops 実行可否 | §未解決事項 → CLI 実行環境 |
| サブコマンドライブラリ選定（Cobra 確定） | §未解決事項 |

### 12.3 ADR-0003 フロントエンド認可フロー

| フィードバック項目 | 想定更新箇所 |
|------------------|------------|
| Cookie Domain 属性の最終決定（U2） | §13 未解決事項 |
| Middleware vs Server Component 検証結果 | §13 未解決事項 |
| Safari ITP 影響評価 | §13 未解決事項 |
| session_token の長さ確定（base64url 43 文字） | §13 未解決事項 |

### 12.4 ADR-0004 メールプロバイダ選定

| フィードバック項目 | 想定更新箇所 | 状況 |
|------------------|------------|------|
| 4 プロバイダ比較表 + 参考 2 候補 | §比較表 | **2026-04-25 完了**：dashboard 本文表示・retention・event payload・地域・無料枠を 6 候補で比較表化 |
| 第一候補確定 → Proposed → Accepted | §ステータス + §決定 | **2026-04-25 Accepted（再選定後）**：第一候補 SendGrid、第二候補 Mailgun、運用不可 AWS SES、不採用 Resend / Postmark / Cloudflare Email Service。当初は AWS SES 第一候補で Accepted 化したが、Amazon 側申請不通過のため同日中に再オープン |
| AWS SES の扱い | §決定 / §不採用候補 | **運用不可（技術不採用ではない）**：当初は本文非保持・Tokyo リージョン・MVP 最安で第一候補だったが、Amazon 側 SES 利用申請が不通過。MVP では運用不可、Phase 2 で再申請検討 |
| 実装方針の整理 | §実装方針 | **2026-04-25 完了**：EmailSender ポート抽象化、本文最小化、Outbox payload に管理 URL を入れない、件名／カスタム引数／categories／metadata に token を含めない、recipient_email 24h NULL 化、application ログには message_id のみ |
| 切替手順テンプレート（SendGrid → Mailgun） | §M2 以降の TODO | M2 早期にフォールバック手順を準備（DKIM/DMARC 再設定 TODO リストを含む） |
| 実送信 PoC（1 通テスト + bounce 受信） | §M2 以降の TODO | **未実施**。M2 早期に SendGrid アカウント作成 + テスト送信 + bounce/complaint 受信を別タスクで実施 |

### 12.5 ADR-0005 画像アップロード方式

| フィードバック項目 | 想定更新箇所 | 状況 |
|------------------|------------|------|
| R2 content-length-range 実機検証結果（U7 関連） | §Content-Length 検証 | **2026-04-25 部分解消**：aws-sdk-go-v2 の presign は `Content-Length` を SignedHeaders に含めるため、宣言サイズと実 PUT サイズの一致が R2 側で厳格に要求される（不一致時 `403 SignatureDoesNotMatch`）ことを実機で確認。本実装の client は宣言サイズと一致した body を送る前提で設計する |
| HEIC 変換コンテナ構築結果 | §未解決事項 | M3〜M6 で扱う（M1 対象外） |
| Turnstile セッション永続化先確定 | §未解決事項 | **2026-04-25 解消**：MVP は DB テーブル `upload_verification_sessions` で確定（コミット `53fa568`）。Phase 2 で Redis 移行検討余地は残す |
| 並列消費の競合解決結果（U5） | §未解決事項 | **2026-04-25 解消**：単一 SQL UPDATE による 5 条件 AND での原子消費（PostgreSQL Read Committed）が 100 並列で `success=20 / forbidden=80` に収束することを確認（コミット `53fa568`） |
| Wrangler CLI 経由 R2 疎通検証 | §M1 検証結果 | **2026-04-25 中断記録**：Wrangler 4.82.2 / 4.85.0 の OAuth スコープに `r2:*` が無く、本来の検証目的は Go backend 経由で代替達成。Wrangler 採用時の制約として ADR に記録 |

### 12.6 横断設計

| ファイル | フィードバック項目 |
|---------|-------------------|
| `cross-cutting/reconcile-scripts.md` §3.7.5 / §11 U11 | 自動 reconciler 起動基盤の最終確定 |
| `cross-cutting/outbox.md` §6.1 ピックの排他制御 | `SKIP LOCKED` の並列性能実測 |

### 12.7 ステータス遷移

| ADR | 現状 | M1 後の目標 |
|-----|------|-----------|
| ADR-0001 | Accepted | Accepted（検証結果追記） |
| ADR-0002 | Accepted | Accepted（U の解消） |
| ADR-0003 | Accepted | Accepted（U2 / U3 の解消） |
| ADR-0004 | **Accepted（MVP プロバイダ選定として、2026-04-25 再選定）**：第一 SendGrid / 第二 Mailgun / 運用不可 AWS SES（申請不通過） | M2 早期に SendGrid 実送信 PoC 完了で「Accepted」に注記を外す |
| ADR-0005 | Accepted | Accepted（U7 / U5 の解消） |

---

## 13. M1 完了条件

以下を全て満たした時点で M1 完了とする。

### 13.1 必須

- [ ] §5 の各成功条件をすべて満たす（または満たさない場合に §6 の代替案を選択し ADR 改訂を完了）
- [x] **Frontend PoC（OpenNext adapter）の CLI 検証完了**（2026-04-25、コミット `6e2840a`）
- [x] **macOS Safari 実機検証完了**（2026-04-25、大きな問題なし）
- [x] **iPhone Safari 実機検証完了**（2026-04-25、大きな問題なし。24h / 7 日後の ITP 影響評価は §13.2 継続観察）
- [x] **Backend PoC（Cloud Run + Go chi + pgx + sqlc + goose）の CLI + Docker 検証完了**（2026-04-25、コミット `c2a5919`）
- [x] **R2 接続 PoC（aws-sdk-go-v2 / presigned PUT / HeadObject）のコード実装完了**（2026-04-25、コミット `a33be4c`）
- [x] **R2 S3 互換 API 実接続検証完了**（2026-04-25、`vrcpb-spike` バケット + 短期 API Token + `.env.local`）。HeadBucket / List / presigned PUT / R2 PUT（1024 bytes 合致）/ HeadObject / 失敗系 8 ケース / ログ漏洩 grep 0 ヒットすべて成立
- [-] **R2 Wrangler CLI 疎通検証**: Wrangler 4.82.2 / 4.85.0 の OAuth スコープに `r2:*` が無いため中断。Dashboard 上で R2 有効化済みは目視確認、本来の検証目的（Go backend → R2）は項目 4 で達成済み
- [x] **Outbox + 自動 reconciler 起動基盤 PoC 完了**（2026-04-25、優先順位 7）。`outbox_events` migration + sqlc + sandbox API + `cmd/outbox-worker` CLI + shell ラッパー。enqueue → claim → processed / failed → retry-failed → 再 claim、2 並列 process-once での `FOR UPDATE SKIP LOCKED` 二重処理防止、`ImageIngestionRequested` を扱えること、payload / Secret 漏洩なしすべて成立
- [x] **Frontend / Backend 結合 PoC（CORS / Origin / Cookie 引き渡し）完了**（2026-04-25、コミット `7f971fc`）
- [x] **Turnstile + upload_verification_session PoC 完了**（2026-04-25、コミット `53fa568`、§5.5 検証マトリクス全項目達成）
- [ ] **Cloudflare Workers 実環境（`*.workers.dev`）での Frontend PoC デプロイ動作確認**
- [ ] **Cloud Run 実環境（`*.run.app`）での Backend PoC デプロイ動作確認**（コールドスタート / Cloud SQL / Cloud Logging / SIGTERM）
- [ ] **R2 Wrangler 実操作疎通検証**（次工程）
- [ ] §12 のフィードバック項目を ADR / 設計書に反映
- [x] U5（アトミック消費）解消（2026-04-25 Turnstile PoC で確定）
- [ ] U2（Cookie Domain）/ U7（storage_key 乱数）/ U11（起動基盤）の 3 つを最終確定
- [x] **ADR-0004 を Accepted 化**（2026-04-25、4 候補比較完了 + 第一/第二候補確定 + 実装方針整理。実送信 PoC は M2 早期に別タスクで実施する旨を明記）。**同日中に再選定**：当初は AWS SES 第一候補だったが Amazon 側申請不通過のため、第一候補を SendGrid に昇格・第二候補を Mailgun に再評価（AWS SES は運用不可として記録）
- [ ] PoC コード（`harness/spike/`）が動作する状態でリポジトリにコミット済み（本実装と分離）
- [ ] **M1 実環境デプロイ前に GCP Budget Alert を設定**（`docs/plan/m1-live-deploy-verification-plan.md` §4.5 推奨閾値: 500 円 / 1,000 円 / 3,000 円）
- [ ] **Cloud SQL は M1 実環境デプロイで「最初から常時起動」を避け、必要時のみ短時間起動する方針を採用**（同 §4.4 段階化 Step A → B → C）
- [ ] **M1 実環境デプロイ完了時の後片付け**: Cloud SQL 停止 / Cloud Scheduler 削除 / Cloud Run Jobs 削除 / Artifact Registry 検証 image 削除 / Cloud Run `min-instances=0` 確認 / R2 API Token Revoke / Secret Manager 検証 Secret 削除 / Billing 画面で検証用リソース消失を目視確認（同 §14 後片付け手順）

### 13.0 次に進める順序（2026-04-25 更新版）

Frontend PoC + Safari 実機検証 + Backend PoC（CLI / Docker）+ R2 コード実装 + Frontend/Backend 結合 PoC + Turnstile PoC が完了したため、進行順序を以下に更新:

1. ~~**macOS Safari 実機検証**~~ ✅ 2026-04-25 完了
2. ~~**iPhone Safari 実機検証**~~ ✅ 2026-04-25 完了
3. ~~**Backend PoC（Cloud Run + Go chi + pgx + sqlc + goose）**~~ ✅ 2026-04-25 完了（コミット `c2a5919`）
4. ~~**R2 接続 PoC コード実装**~~ ✅ 2026-04-25 完了（コミット `a33be4c`、aws-sdk-go-v2 / sandbox エンドポイント）
5. ~~**Frontend / Backend 結合 PoC（CORS / Origin / Cookie）**~~ ✅ 2026-04-25 完了（コミット `7f971fc`）
6. ~~**Turnstile + upload_verification_session PoC**~~ ✅ 2026-04-25 完了（コミット `53fa568`）
7. ~~**R2 Wrangler 実操作疎通検証**~~ ⚠️ **中断**（2026-04-25）
   - Wrangler 4.82.2 / 4.85.0 の `wrangler login --scopes-list` に R2 系スコープが無く、OAuth 経由では R2 操作トークンが取得できないことが判明
   - エラーは `code: 10042 Please enable R2 through the Cloudflare Dashboard.` だが、これは「アカウントで R2 が無効」ではなく「OAuth トークンに R2 スコープが無い」状態でも返される。Dashboard で R2 有効化・既存バケット 2 個稼働を目視確認済み
   - 切り替え判断: 本来の検証目的（Go backend → R2 経由の presigned URL 発行・PUT・Head）は項目 8 で直接達成できるため、Wrangler 経由は M1 では不要と判断
8. ~~**R2 S3 API + presigned URL の実接続検証**~~ ✅ **完了**（2026-04-25）
   - ユーザーが Cloudflare Dashboard で `vrcpb-spike` バケットを作成、R2 → Manage R2 API Tokens で Object Read & Write の短期 API Token を発行、`harness/spike/backend/.env.local` に手入力
   - Claude Code は `.env.local` の値を一切表示せず（中身を `cat`しない / `printenv` しない / grep の引数に literal で渡さない）、Backend を起動して `/sandbox/r2-headbucket` / `/sandbox/r2-list` / `/sandbox/r2-presign-put` / R2 へ実 PUT / `/sandbox/r2-headobject` / 失敗系 8 ケース / ログ漏洩 grep を実施
   - すべて成功。`R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_ENDPOINT` / `storage_key` / `X-Amz-*` / `presigned` / `access_key` のいずれもサーバ slog に 0 ヒット
   - 余談: 最初の PUT は宣言サイズ（byte_size=1024）と実 PUT サイズ（34 bytes）の不一致で R2 が `SignatureDoesNotMatch` を返す挙動を実機で観測（aws-sdk-go-v2 が Content-Length を SignedHeaders に含めるため、本実装側では client が宣言サイズと一致した body を送る必要がある旨を確認）
   - 後片付け: ユーザー側で R2 → `vrcpb-spike` のテストオブジェクト削除 + 検証用 API Token の Revoke を実施
9. ~~**Outbox / 自動 reconciler 起動基盤 PoC**~~ ✅ **完了**（2026-04-25、優先順位 7）
   - `harness/spike/backend/` に `outbox_events` migration + sqlc + sandbox API + `cmd/outbox-worker` CLI + shell ラッパーを追加
   - enqueue / claim（`FOR UPDATE SKIP LOCKED`）/ MarkProcessed / MarkFailed / `outbox_failed_retry` 最小実装 / 二重処理防止 / `ImageIngestionRequested` 種別を実機確認
   - 残: Cloud Run Jobs + Cloud Scheduler 実環境からの起動 / 同時刻スケジューラ重複時の advisory lock 評価 / 指数バックオフ / 保持期間（processed=30 日）クリーンアップ
10. ~~**Email Provider 選定検証**~~ ✅ **完了（書類調査ベース、ADR-0004 Accepted、再選定済み）**（2026-04-25、優先順位 8）
    - 4 候補 + 参考 2 候補（Postmark / Cloudflare Email Service）の公式ドキュメントから dashboard 本文表示・retention・event payload・地域・無料枠を比較表化
    - **当初の Accepted（同日）**: 第一候補 AWS SES（Tokyo region・本文非保持・event payload に body なし・MVP 安価）、第二候補 SendGrid
    - **再選定（同日）**: AWS SES が **Amazon 側申請不通過**で MVP 運用不可となったため、第一候補を SendGrid に昇格、第二候補を Mailgun に再評価
    - **第一候補 SendGrid**: Twilio 公式が「本文を保存しない」を明言（"does not store email content" / "does not retain the contents of emails"）、Email Activity Feed は events のみ、無料 100 通/日
    - **第二候補 Mailgun**: Domain settings で retention 0 day 選択可（"includes disabling retention by setting the value to 0"）、SendGrid が運用面で使えない場合のフォールバック
    - **運用不可 AWS SES**: 技術不採用ではなくアカウント／運用上の利用不可。Phase 2 で再申請検討
    - **不採用**: Resend（dashboard 本文表示 + sensitive 非保存は厳条件 add-on） / Postmark（45 日 default 本文保存） / Cloudflare Email Service（public beta、limit 変動）
    - 残: M2 早期に SendGrid アカウント作成 + 審査通過 + DKIM/SPF/DMARC + 1 通テスト送信 + Email Activity Feed / Webhook payload に本文が出ないことを実機確認 + bounce/complaint webhook + Cloud Run からの実送信レイテンシ計測
11. **Cloudflare Workers + Cloud Run 実環境デプロイ検証** ← 次工程候補（M1 で残る大きな未確認項目）
    - Frontend → `*.workers.dev`、Backend → `*.run.app`
    - Safari 実機での再確認、24h / 7 日後の ITP 影響観察開始
    - Cloud Run 東京 ↔ R2 のレイテンシ計測（M1 残作業）
    - Cloud Run Jobs + Cloud Scheduler から `outbox-worker --once` / `--retry-failed` を起動（U11 確定）
    - Cookie Domain（U2、ADR-0003）の最終確定
12. **Turnstile 本番 widget 発行**
    - Workers 実環境の hostname 確定後、Cloudflare Dashboard → Turnstile → Add widget で発行
    - widget 名 `vrcpb-spike`、hostname に Workers の URL を登録、Managed / Non-interactive を比較候補として記録
    - 本番 secret は `.env.local` または Secret Manager 経由のみで扱う（チャット・ログに貼らない）

### 13.2 推奨（達成できれば望ましい） / 継続観察

- [ ] Cloud Run コールドスタート計測結果のドキュメント化
- [ ] R2 レイテンシ計測結果のドキュメント化
- [ ] ブラウザマトリクス（§11.1）全 7 ブラウザでの検証
- [ ] `harness/QUALITY_SCORE.md` に M1 検証結果サマリ追記

#### 継続観察項目（M1 完了後も追跡、運用開始後に判定）

- [ ] iPhone Safari で **24 時間後 / 7 日後の Cookie 残存**（ITP 長期影響評価）
- [ ] iOS Safari 1 世代前 / iPad Safari / プライベートブラウジング動作
- [ ] Cloudflare Workers 実環境デプロイ後の Safari 再確認

### 13.3 失敗扱いの基準

以下のいずれかが発生した場合、M1 を「失敗」として扱い、追加検証期間を設ける:

- **Frontend 構成の根本変更が必要**（Next.js + Cloudflare Pages を諦める）
- **Backend 構成の根本変更が必要**（Cloud Run を諦める）
- **token→session 交換方式が成立しない**（ADR-0003 全面見直し）
- **R2 presigned URL 方式が成立しない**（ADR-0005 全面見直し）

これらは M1 中に必ず判定し、判定後は次のスプリント計画で対応方針を決める。

---

## 付録A: M1 中に変更しないもの

- 業務知識 v4 § / 付録C P0-1〜P0-31（実装着手前の確定事項）
- 各集約のドメインモデル / データモデル設計（M1 検証結果が著しい影響を持つ場合のみ追記）
- ADR の根本方針（Cloud Run / Cloudflare Pages / UUIDv7 / chi 等）。検証結果次第で代替案に切替する場合のみ ADR 改訂

## 付録B: M2 以降への引き継ぎ

M1 完了後、M2 では以下に進む:

- backend/ 本実装開始（`domain-standard.md` ディレクトリ構造、`testing.md` テーブル駆動テスト準拠）
- frontend/ 本実装開始（Next.js 15 App Router、Tailwind、design/mockups/prototype を参考）
- M1 PoC コードは流用しない（粗いコードを本実装に持ち込まない原則）
- failure-log の運用開始（最初の失敗をルール化）

## 付録C: 関連リンク

- 業務知識 v4: `docs/spec/vrc_photobook_business_knowledge_v4.md`
- ADR-0001 技術スタック: `docs/adr/0001-tech-stack.md`
- ADR-0002 運営操作方式: `docs/adr/0002-ops-execution-model.md`
- ADR-0003 フロントエンド認可フロー: `docs/adr/0003-frontend-token-session-flow.md`
- ADR-0004 メールプロバイダ選定（Proposed）: `docs/adr/0004-email-provider.md`
- ADR-0005 画像アップロード方式: `docs/adr/0005-image-upload-flow.md`
- Outbox 横断設計: `docs/design/cross-cutting/outbox.md`
- Reconcile スクリプト設計: `docs/design/cross-cutting/reconcile-scripts.md`
- OGP 生成設計: `docs/design/cross-cutting/ogp-generation.md`
- Session 機構: `docs/design/auth/session/`
- Upload Verification 機構: `docs/design/auth/upload-verification/`
- 各集約設計: `docs/design/aggregates/{photobook,image,report,moderation,manage-url-delivery}/`
