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

1. `harness/spike/backend/` を新規作成、Go 1.24+ で初期化（本実装用 `backend/` は M2 まで触らない）
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

| # | 条件 |
|---|------|
| ✅ | Backend Go から presigned URL（PUT、15 分有効）を発行できる |
| ✅ | フロントから R2 への直接 PUT が成功する |
| ✅ | complete 時の HeadObject 確認が成立する |
| ✅ | 10MB 超過 PUT が拒否されるか、complete 時の検証で `failed(file_too_large)` 判定できる |
| ✅ | 期限切れ presigned URL での PUT が失敗する |
| ✅ | ログに presigned URL / storage_key / token が出ない |

### 5.5 Turnstile + upload_verification_session

| # | 条件 |
|---|------|
| ✅ | Turnstile サーバ検証 API が Backend から呼べる |
| ✅ | `upload_verification_sessions` への INSERT と SHA-256 hash 保存が動く |
| ✅ | アトミック UPDATE で 20 回成功・21 回目失敗が厳密に成立する |
| ✅ | 100 並列実行でも `used_intent_count` が壊れない |

### 5.6 UUIDv7 / Web Crypto API / 乱数

| # | 条件 |
|---|------|
| ✅ | Backend Go で UUIDv7 が生成できる（時系列順序が保証される） |
| ✅ | Backend Go で 32B 乱数 → base64url → SHA-256 → bytea 32B が生成できる |
| ✅ | Cloudflare Pages（Workers ランタイム）で `crypto.subtle.digest` が動く |

### 5.7 Outbox + 自動 reconciler 起動基盤

| # | 条件 |
|---|------|
| ✅ | `outbox_events` の状態変更と同一 TX INSERT が成立する |
| ✅ | `SELECT FOR UPDATE SKIP LOCKED` で並列ワーカーが衝突しない |
| ✅ | Cloud Scheduler + Cloud Run Jobs で 5 分ごと cron 起動が動く |
| ✅ | 多重起動が発生しても DB 行ロックで処理が壊れない |

### 5.8 Email Provider

| # | 条件（M1 ではここまで、Accepted 化は M2 早期） |
|---|------|
| ✅ | 4 プロバイダの本文ログ保持仕様が ADR-0004 比較表に埋まる |
| ✅ | テスト送信 1 通が成功し、送信履歴 UI / API ログの本文表示を実機確認 |
| ✅ | 第一候補が決まり、Accepted 化の条件が ADR-0004 に書ける状態になる |

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

| フィードバック項目 | 想定更新箇所 |
|------------------|------------|
| 4 プロバイダ比較表 | §候補 / §評価軸 → 比較結果セクション新設 |
| 第一候補確定 → Proposed → Accepted | §ステータス + §結果 |
| 検証結果の追記 | §未解決事項 / 検証 TODO |

### 12.5 ADR-0005 画像アップロード方式

| フィードバック項目 | 想定更新箇所 |
|------------------|------------|
| R2 content-length-range 実機検証結果（U7 関連） | §Content-Length 検証 |
| HEIC 変換コンテナ構築結果 | §未解決事項 |
| Turnstile セッション永続化先確定（U2 別件） | §未解決事項 |
| 並列消費の競合解決結果（U5） | §未解決事項 |

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
| ADR-0004 | **Proposed** | **Accepted**（M1 中、もしくは M2 早期） |
| ADR-0005 | Accepted | Accepted（U7 / U5 の解消） |

---

## 13. M1 完了条件

以下を全て満たした時点で M1 完了とする。

### 13.1 必須

- [ ] §5 の各成功条件をすべて満たす（または満たさない場合に §6 の代替案を選択し ADR 改訂を完了）
- [x] **Frontend PoC（OpenNext adapter）の CLI 検証完了**（2026-04-25、コミット `6e2840a`）
- [ ] §11 の Safari / iPhone Safari 検証項目を実機で完了（**Frontend PoC の次工程**）
- [ ] **Cloudflare Workers 実環境（`*.workers.dev`）での Frontend PoC デプロイ動作確認**
- [ ] §12 のフィードバック項目を ADR / 設計書に反映
- [ ] U2（Cookie Domain）/ U7（storage_key 乱数）/ U5（アトミック消費）/ U11（起動基盤）の 4 つを最終確定
- [ ] ADR-0004 を Accepted 化（または M2 早期での Accepted 化条件を ADR-0004 に明記）
- [ ] PoC コード（`harness/spike/`）が動作する状態でリポジトリにコミット済み（本実装と分離）

### 13.0 次に進める順序（2026-04-25 時点）

Frontend PoC の CLI 検証完了を受け、以下の順で M1 残作業を進める:

1. **macOS Safari 実機検証**（PoC をローカルまたは実環境で動かして DevTools 確認）
2. **iPhone Safari 実機検証**（最重要、ITP 影響評価含む）
3. **Cloudflare Workers 実環境（`*.workers.dev`）への PoC デプロイ検証**
4. 上記の結果を ADR-0001 / ADR-0003 / 本書 §12 に反映
5. **Backend / R2 / Turnstile / Outbox / Email Provider の PoC**（優先順位 3〜8）に着手

### 13.2 推奨（達成できれば望ましい）

- [ ] Cloud Run コールドスタート計測結果のドキュメント化
- [ ] R2 レイテンシ計測結果のドキュメント化
- [ ] ブラウザマトリクス（§11.1）全 7 ブラウザでの検証
- [ ] `harness/QUALITY_SCORE.md` に M1 検証結果サマリ追記

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
