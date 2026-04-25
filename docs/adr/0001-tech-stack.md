# ADR-0001 技術スタック

## ステータス
Accepted

## 作成日
2026-04-25

## 最終更新
2026-04-25

## コンテキスト

VRC PhotoBook は、VRChat で撮影した写真をログイン不要で Web フォトブックとしてまとめ、URL で共有できるサービスである。MVP の実装を開始するにあたり、バックエンド・フロントエンド・ストレージ・デプロイまでを含む技術スタック全体を確定させる必要がある。

v3 は破棄済みであり、以後は `docs/spec/vrc_photobook_business_knowledge_v4.md`（作成予定 / v4相当の業務知識）および `docs/design/aggregates/` 配下の設計書を正の参照先として扱う。v4 では以下の前提が既に固まっており、技術選定の制約になっている。

- ログイン不要・管理URL方式（raw token の秘匿が必須）
- Photobook 中心の集約設計、Draft と Published は同一レコード・status 遷移のみ
- Outbox と Reconcile を MVP から採用
- PostgreSQL の部分INDEX / UNIQUE / CHECK 制約を前提としたデータモデル
- 画像は最大8192px・最大40MP・最大10MB、EXIF/XMP/IPTC 除去・HEIC変換・SVG/アニメーションWebP禁止
- MVP は全ページ noindex、Referrer-Policy をページ種別ごとに制御する必要がある
- Safari / iPhone Safari 対応

さらに、プロジェクトルール（`.agents/rules/domain-standard.md`、`.agents/rules/testing.md`）が Go のディレクトリ構造・コード例を前提に記述されており、バックエンドは Go を第一前提として読める。フロントエンドは `design/mockups/prototype/` に React + Tailwind 風のプロトタイプが既に存在する。

以上の制約を踏まえて、実装者がライブラリ選定で迷わない粒度まで確定させる。

## 決定

### 採用技術

| レイヤー | 採用技術 | 備考 |
|---------|---------|------|
| Backend 言語 | **Go 1.24+** | ルールと設計書が Go 前提 |
| Web Framework | **chi** | `net/http` に近く、ドメイン層への依存漏れが少ない |
| DB | **PostgreSQL 16** | 部分INDEX / CHECK / UNIQUE 制約を活用 |
| DB アクセス | **sqlc + pgx** | 生成コードを直接読める、型安全 |
| Migration | **goose** | Go バイナリで一貫、`up/down` が素直 |
| Frontend | **Next.js 15 (App Router)** | SSR 必須要件（OGP・noindex・Referrer-Policy） |
| Styling | **Tailwind CSS** | プロトタイプとの親和、デザイントークン化しやすい |
| Form | **react-hook-form** | 大規模フォーム（編集画面）向け |
| Validation | **zod** | サーバー/クライアント共通スキーマ化可能 |
| Icon | **lucide-react** | 軽量・SVG・Tree shake 効く |
| Client state | React state / URL state / Server state を基本、必要時のみ **Zustand** | エディタのページ並び替え等、局所的に必要な場合に限定 |
| Storage | **Cloudflare R2** | S3互換、エグレス無料、ADR-0005 の前提 |
| Email | **Resend or AWS SES**（ADR-0004 で確定） | 管理URL本文ログの扱いで最終判断 |
| Backend Deploy 第一候補 | **Cloud Run** | Go + コンテナとの相性、Cloud SQL / Secret Manager / Cloud Logging 接続が自然 |
| Backend Deploy 代替候補 | **VPS** / **Fly.io** | VPS はコスト読みやすさ、Fly.io は代替として保持 |
| Frontend Deploy 第一候補 | **Cloudflare Workers + Static Assets binding（OpenNext adapter）** | M1 検証結果（2026-04-25）で `@cloudflare/next-on-pages` が deprecated と確認、`@opennextjs/cloudflare` に切替済み。M1 スパイク次第でさらに変更余地を残す |
| Testing | Go 標準 `testing` + **testcontainers-postgres** + **table-driven test** | `.agents/rules/testing.md` 準拠 |
| ID | **UUIDv7** | DB 主キー・内部ID。公開URL slug は別途 `public_url_slug` を生成 |

### 重要な確定事項

#### Backend Deploy は Cloud Run 第一候補

MVP の運用負担を最小化するため、スケールゼロとマネージド運用を備えた Cloud Run を第一候補とする。Go の軽量コンテナはコールドスタートのペナルティが小さく、Cloud SQL・Secret Manager・Cloud Logging との統合が自然である。また、HTTP API と Outbox ワーカーを別サービスとして分離しやすい点も MVP 運用に合う。

VPS を代替として残す理由は、コスト読みやすさと既存の VPS 運用ノウハウが活きる点である。アクセス量が極小で安定する局面では Cloud Run より低コストになりうる。Fly.io は選択肢として保持するが、日本リージョンのプロダクション運用事例・GCP/AWS 比でのエコシステム規模を考慮して第一候補にはしない。

#### UUIDv7 採用

DB 内部 ID には UUIDv7 を採用する。UUIDv7 は時刻ベースで単調増加するため、B-tree インデックスでランダムページ書き込みを避けられ、INSERT 性能が UUIDv4 より良好である。PostgreSQL の `uuid` 型は 16バイト固定で、sqlc / pgx がネイティブサポートしているため型変換コストも低い。

ULID も同等の時系列性を持つが、`uuid` 型で直接扱えず `text(26)` または bytea に落とすことになり、DB レイヤでの互換性が劣る。公開URL 用の識別子は衝突可能性・短さ・視覚的扱いやすさが優先されるため、別途 `public_url_slug` として短い文字列を生成する（例：base32 crockford 10〜12文字）。

なお、UUIDv7 は内部 ID 用途に限定する。**Cookie session 値や Bearer 的に用いるトークンには UUIDv7 を使わず、256bit 以上の暗号論的乱数を採用する**（時刻情報を含まない、予測困難性を最大化するため）。詳細は ADR-0003 を参照。

#### chi 採用

Web フレームワークは chi を採用する。以前の検討では Echo / Gin が候補に上がったが、chi は `net/http.Handler` そのまま使える点、middleware が `func(http.Handler) http.Handler` の標準形である点、フレームワーク固有のコンテキスト型を持たない点で、DDD / Clean Architecture と相性が良い。Echo / Gin は独自の `Context` を持ち、これをユースケースや集約に渡すとドメイン層へフレームワーク依存が漏れる事故が起きやすい。chi なら `context.Context` と `*http.Request` で一貫させられる。

### Cloud Run + Cloudflare R2 のクロスクラウド構成

Backend を Cloud Run（GCP）、Storage を Cloudflare R2 に置くため、MVP はクロスクラウド構成になる。これは R2 のエグレス無料を享受するための意図的な選択だが、以下の運用上の注意点がある。

- R2 の API 資格情報（Access Key ID / Secret）は GCP Secret Manager で管理し、Cloud Run サービスの環境変数として注入する。
- Cloud Run のリージョンと R2 のレイテンシを M1 / M3 で計測する（presigned URL 発行のレイテンシ、image-processor から R2 への PUT / GET スループット）。
- 障害切り分けが 2 クラウドにまたがるため、ログ相関 ID（request_id / photobook_id / image_id）をアプリ層で一貫発行する。

### M1 で必要なスパイク

Next.js on Cloudflare Pages は、以下を M1 で必ずスパイク検証する。

- SSR が正しく動くか（OpenNext / @cloudflare/next-on-pages のどちらを使うか含む）
- `generateMetadata` で OGP メタタグ（og:image / og:title / og:description / twitter:card）を動的に出せるか
- public / manage / draft でページ種別ごとに異なるレスポンスヘッダ（X-Robots-Tag・Referrer-Policy・Cache-Control）を返せるか
- `X-Robots-Tag: noindex, nofollow` を全種別に付与できるか、同時に HTML `<meta name="robots" content="noindex">` を出せるか
- Referrer-Policy をページ種別ごとに分けられるか（通常ページ `strict-origin-when-cross-origin`、token 付き URL `no-referrer`）
- Safari / iPhone Safari で Cookie 属性（SameSite=Strict, Secure）・履歴戻り・ITP の挙動に問題がないか
- Route Handler / Middleware で HttpOnly Cookie を発行し、Server Component 側で再読取できるか（ADR-0003 のフロー）
- Cloudflare Workers 実行環境で **Web Crypto API（`crypto.subtle.digest` 等）と Cookie / headers 処理** が期待通り動作するか

検証で致命的な制約が見つかった場合、Frontend Deploy は Cloud Run / VPS / Fly.io に切り替える余地を残す。スタイリング・フレームワーク（Next.js / Tailwind）は維持する。

### M1 検証結果（2026-04-25 時点）

`harness/spike/frontend/` での最小 PoC（`@cloudflare/next-on-pages` 版、Next.js 標準 dev + `wrangler pages dev`）で以下を **CLI 検証成功**として確認済み:

- SSR 動作（`x-edge-runtime: 1`）
- `generateMetadata` による OGP / Twitter card メタタグの動的出力
- HTML 内 `<meta name="robots" content="noindex, nofollow">`
- HTTP ヘッダ `X-Robots-Tag: noindex, nofollow`
- ページ種別ごとの `Referrer-Policy` 出し分け（通常ページ `strict-origin-when-cross-origin` / token 付き URL `no-referrer`）
- Route Handler から `Set-Cookie` (HttpOnly / Secure / SameSite=Strict / Path=/) + 302 redirect
- redirect 後の Server Component で `cookies()` 読取が動作
- Cloudflare Pages 互換ローカル環境（`wrangler pages dev`）でも同等に動作

### M1 検証結果（2026-04-25 時点、OpenNext adapter 版で再確認済み）

#### v2 OpenNext adapter 版（コミット `6e2840a` 時点、本実装第一候補）

`harness/spike/frontend/` を `@opennextjs/cloudflare` + `wrangler 4` に切り替えて再検証。`opennextjs-cloudflare build` → `opennextjs-cloudflare preview` 経由で **Cloudflare Workers + Static Assets binding** ローカル環境（`http://localhost:8787`）にて以下を CLI 検証成功:

- SSR 動作（OpenNext 識別ヘッダ `x-opennext: 1`）
- `generateMetadata` による OGP / Twitter card メタタグの動的出力
- HTML 内 `<meta name="robots" content="noindex, nofollow">`
- HTTP ヘッダ `X-Robots-Tag: noindex, nofollow`
- ページ種別ごとの `Referrer-Policy` 出し分け（通常ページ `strict-origin-when-cross-origin` / token 付き URL `no-referrer`）
- Route Handler から `Set-Cookie` (HttpOnly / Secure / SameSite=Strict / Path=/) + 302 redirect
- redirect 後の Server Component で `cookies()` 読取が動作

#### v1 next-on-pages 版（コミット `c7ba16b` 時点、ベースライン参照のみ）

`@cloudflare/next-on-pages` + `wrangler pages dev` でも同等の検証が成立していたが、`npm install` 時に Cloudflare 公式から **deprecated 警告**が出たため、M2 本実装は OpenNext adapter 一本に絞る。`@cloudflare/next-on-pages` の検証ログは Git 履歴で参照可能。

#### Safari 実機検証結果（2026-04-25）

- **macOS Safari 実機検証 ✅ 成立**: `/draft/{token}` / `/manage/token/{token}` → redirect → session found 表示、再読込後も維持、Web Inspector で Cookie 属性目視確認
- **iPhone Safari 実機検証 ✅ 成立**: 同上の経路で問題なし
- 詳細: ADR-0003 §M1 検証結果 / `harness/spike/frontend/README.md` §検証チェックリスト

#### 未確認（実機 / 実環境が必要、M1 残作業）

- 24 時間 / 7 日後の Cookie 残存（**ITP 長期影響評価**、継続観察）
- **Cloudflare Workers 実環境（`*.workers.dev` ドメイン）でのデプロイ動作**
- Backend（Cloud Run）と異なるホスト構成下での Cookie Domain 動作（U2、Backend PoC と統合）
- iOS Safari 1 世代前 / iPad Safari / プライベートブラウジング

#### M1 検証で確定した方針変更（M2 本実装の必須要件）

1. **Frontend Deploy 第一候補を OpenNext adapter での Cloudflare Workers + Static Assets binding に確定**
   - 旧: Cloudflare Pages（`@cloudflare/next-on-pages`）
   - 新: Cloudflare Workers + `assets` バインディング（`@opennextjs/cloudflare`）
   - URL: `*.pages.dev` → `*.workers.dev`（カスタムドメインは独立に設定）
   - デプロイコマンド: `wrangler pages deploy` → `wrangler deploy`
   - 詳細経緯: `harness/failure-log/2026-04-25_cloudflare-next-on-pages-deprecated.md`

2. **`export const runtime = "edge"` を指定しない**
   - OpenNext for Cloudflare は Workers 上の **Node.js 互換ランタイム**で動作する
   - App Router のページ・Route Handler・Server Component で `runtime = "edge"` を指定するとビルドエラー（`OpenNext requires edge runtime function to be defined in a separate function.`）
   - M2 本実装では「`runtime = "edge"` を指定しない」を必須ルールとする

3. **レスポンスヘッダ制御を middleware に一本化する**
   - 検証で `next.config.mjs` の `headers()` と `middleware.ts` の両方で `X-Robots-Tag` を付与すると、OpenNext 上で `noindex, nofollow, noindex, nofollow` のように **値が重複**することを確認
   - M2 本実装では `next.config.mjs` の `headers()` でのヘッダ付与を行わず、**`middleware.ts` でページ種別ごとに集中管理**する
   - 業務知識 v4 §7.6 / §6.13 のヘッダ要件はそのまま維持し、実装手段だけを middleware 一本化に確定

4. **OGP の絶対 URL 解決を必ず `metadataBase` で行う**
   - PoC 検証中、`og:image` が `http://localhost:3000/og-sample.png` のように dev サーバ URL のまま焼き込まれる挙動を確認
   - 原因は Next.js Metadata API が相対 URL を絶対 URL に展開するときに `metadataBase` を参照する仕様
   - M2 本実装では `app/layout.tsx`（または該当ページ）の `Metadata` に **`metadataBase: new URL(process.env.NEXT_PUBLIC_BASE_URL)` を必ず設定**する。`NEXT_PUBLIC_BASE_URL` は環境ごとに切替（本番 / プレビュー / dev）

5. **`@cloudflare/next-on-pages` を採用しない**
   - 上記 1 と同根。M2 以降のフロント実装ガイド・Dockerfile / GitHub Actions / Wrangler 設定すべてに反映する

### M1 検証結果（2026-04-25 時点、Backend / Cloud Run + Go chi + pgx + sqlc + goose）

`harness/spike/backend/` で Cloud Run 向けの最小 Go API を構築し（コミット `c2a5919`）、ローカル CLI + Docker container 経由で以下を確認した。

#### 採用ライブラリの最小構成成立を確認

| 種別 | 採用 | バージョン |
|------|------|-----------|
| HTTP ルーター | `github.com/go-chi/chi/v5` | v5.1.0 |
| chi middleware | RequestID / RealIP / Recoverer / Timeout（chi 標準） | — |
| DB ドライバ・プール | `github.com/jackc/pgx/v5/pgxpool` | v5.7.1 |
| Migration | `github.com/pressly/goose/v3`（`go run` 経由で実行） | v3.22.0 |
| Code generation (SQL → Go) | `sqlc` CLI | v1.30.0（`pgx/v5` 出力） |
| ロガー | 標準 `log/slog`（JSON ハンドラ） | Go 1.21+ 標準 |

ADR-0001 §採用技術 表で確定済みの構成のまま、M1 PoC で動作成立。**変更なし**。

#### 検証成立した項目

- `go mod tidy` / `go vet` / `go test` / `go build` すべて成功
- `docker compose` でローカル PostgreSQL 16-alpine を起動、`pg_isready` healthcheck 通過
- `goose ... up` で migration 適用（5.9ms）
- `sqlc generate` で `pgx/v5` 出力 3 ファイル生成
- ローカル直起動で `/healthz` / `/readyz` / `/sandbox/db-ping` 全 200 応答
- Graceful shutdown（SIGINT → `srv.Shutdown(ctx)` で 10 秒以内に終了）
- `docker build` で multi-stage / distroless static-debian12:nonroot の **12.4MB イメージ**生成
- container 経由でも同じエンドポイントが正常動作（compose ネットワーク経由で DB 接続）
- slog JSON 出力に DSN・パスワード・token・cookie 値が含まれないことを目視確認

#### Go バージョンの扱い

- PoC ローカル環境: **Go 1.23.2**
- ADR-0001 §採用技術 表の「Go 1.24+」方針は**変更なし**（M2 本実装着手時に 1.24 へ移行）
- 1.24 必須機能を使っていないため、PoC は 1.23 で動作確認したことが「1.24 必須要件の否定」にはならない。chi / pgx / sqlc / goose の最小構成成立確認として扱う

#### M1 検証で確定したサーバ構成のテンプレート

- **Dockerfile**: multi-stage（`golang:1.23-alpine` ビルド → `gcr.io/distroless/static-debian12:nonroot` ランタイム）で 12.4MB に収まる。M2 本実装の Dockerfile はこのテンプレートを `domain-standard.md` 構造に合わせて整備
- **Graceful shutdown**: `signal.NotifyContext(ctx, SIGINT, SIGTERM)` + `srv.Shutdown(shutdownCtx)` パターンを採用
- **`/health` / `/readyz` 分離**: `/health` は process liveness のみ、`/readyz` は pgxpool.Ping ベース。Cloud Run の startup / liveness probe / 本番監視には **`/health`** を使う（Cloud Run / Google Frontend が小文字 `/healthz` を intercept する事象を 2026-04-26 の M1 実環境デプロイで確認、`harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md`）。ローカル PoC / 既存 README 互換のため `/healthz` も並存登録するが、**Cloud Run 上では `/healthz` は到達しない前提**で運用する。外部 LB やヘルスチェックには `/readyz` を使う
- **DB エラー詳細をクライアントに返さない**: 漏洩抑止。サーバ側 slog で追跡

#### Cloud Run へ向けた未確認事項（M1 残作業 / Cloud Run/R2 PoC で扱う）

- 実 Cloud Run へのデプロイ動作（`gcloud run deploy`）
- Cloud Run コールドスタート時間計測
- Cloud SQL 接続方式（Cloud SQL Auth Proxy 経由 vs 直接 DSN）とレイテンシ
- Cloud Logging で slog JSON が正しくパースされるか（severity マッピング）
- Cloud Run の SIGTERM → graceful shutdown が 10 秒内に完了するか実機確認
- Cloud Run 東京リージョン ↔ R2 のレイテンシ計測（クロスクラウド）

これらは次の **R2 接続 PoC** および後続の Cloud Run 実環境デプロイ検証で解消する。

## 検討した代替案

- **Echo**: 高速かつ軽量だが、独自の `echo.Context` を使う前提で middleware / handler が書かれており、ドメイン層に漏れやすい。バインディングの暗黙挙動も多く、`coding-rules.md` の「明示的 > 暗黙的」と相性が悪い。
- **Gin**: 同様に独自 `gin.Context` を持ち、Go 標準 `context.Context` との二重管理が発生する。エコシステムは大きいが MVP 規模では利点が活きない。
- **GORM**: 抽象化が強すぎ、N+1 や暗黙的な WHERE 条件補完など事故源が多い。`any` / `interface{}` を多用しやすく、`.agents/rules/coding-rules.md` の禁止事項と衝突する。
- **raw SQL 手書き**: 型安全性が弱く、プレースホルダ順序ミスやカラム順のズレが起きやすい。sqlc なら DDL から Go の型付きコードを生成でき、コンパイル時に検知できる。
- **Prisma 等を使う案**: Node.js 依存で Go バックエンドとの二重技術になる。schema.prisma と goose migration の二重管理が発生。
- **ULID**: UUIDv7 と同等の時系列性を持つが、PostgreSQL の `uuid` 型で直接扱えず `text(26)` もしくは bytea に落ちる。pgx / sqlc の生成コードが冗長になる。
- **UUIDv4**: 時系列性なし。INSERT のたびに B-tree のランダムページを書き換え、ディスクキャッシュ効率が悪化する。大規模化時に顕在化する。
- **Backend を VPS 第一候補にする案**: オートスケールとマネージドランタイムの利点を失う。セキュリティパッチ・証明書・Linux 運用が属人化する。
- **Fly.io 第一候補**: 日本リージョンあり（tokyo）だが、Cloud Run に比べマネージド Postgres・Secret Manager・監視が未成熟で、MVP の運用負担増。代替として保持は妥当。
- **Cloudflare Pages のみでバックエンドも完結させる案**: Cloudflare Workers の CPU 時間制限（有料プランでも 50ms〜5分）、ネイティブライブラリ（libheif 等の HEIC デコード）を動かせない、Node.js 互換が限定的、長時間の画像処理ワーカーが動かない、という理由で採用不可。
- **Tailwind 以外の CSS Modules / vanilla-extract**: プロトタイプが Tailwind 風に書かれている（`design/mockups/prototype/styles.css` にユーティリティ的クラスが混在）点、デザインシステムのトークン化がユーティリティクラスと素直に対応する点から、Tailwind の方が実装コストが低い。CSS Modules は純度は高いが、小さなデザイン差分をユーティリティで済ませる MVP 速度に不利。

## 結果

### メリット

- Go + chi + sqlc + pgx は DDD / Clean Architecture と整合し、`domain-standard.md` の集約構造にそのまま適合する。
- Next.js 15 + Tailwind は、既存の React プロトタイプからの移植コストが最も低く、SSR 要件（OGP・noindex・Referrer-Policy）を満たしやすい。
- UUIDv7 により、主キー INSERT 性能と時系列デバッグ容易性を両立できる。
- Cloud Run + R2 + Cloudflare Pages の組み合わせはベンダーロックインを避けつつマネージドサービスの利点を享受できる。
- テストは `testcontainers-postgres` + テーブル駆動で一貫し、`testing.md` 準拠を強制しやすい。

### デメリット

- Cloud Run のコールドスタート影響は Go で軽微だが、Outbox ワーカーの常駐コストは恒常的に発生する。
- Cloudflare Pages 上の Next.js 15 App Router は実戦事例がまだ少なく、M1 スパイクでつまづくリスクがある。致命的な場合は Cloud Run への切替が必要になり、設定とデプロイパイプラインを再構築する手戻りコストが発生する。
- sqlc は DDL 変更時に再生成が必要で、CI への組み込みコストがある。
- Go + libheif の HEIC デコードは純 Go ではなく cgo 前提になり、コンテナイメージに libheif / libde265 を含める必要がある（ADR-0005 参照）。
- Cloud Run（GCP）と R2（Cloudflare）のクロスクラウド構成のため、資格情報管理・レイテンシ・障害切り分けが 2 クラウドにまたがる運用を許容する必要がある。

### 後続作業への影響

- M1: `backend/go.mod`、`frontend/package.json`、Dockerfile（backend / image-processor）、goose migration 構造、sqlc 設定、GitHub Actions CI の整備。
- 全マイルストーン: ディレクトリ構造は `domain-standard.md` に従う（`backend/internal/{module}/{domain,infrastructure,internal}/...`）。
- 運営操作（ADR-0002）は `cmd/ops` に単一バイナリで集約する。
- フロントエンドの認可フロー（ADR-0003）は Next.js App Router の Route Handler / Server Component / Middleware の組み合わせで構築する。

## 未解決事項 / 検証TODO

- **M1 Cloudflare スパイク（OpenNext adapter 版）**: 8 項目のうち CLI 検証で完結する項目は 2026-04-25 に検証完了、上記「M1 検証結果」§v2 に記録。Safari / iPhone Safari 実機検証も 2026-04-25 に成立確認済み。残タスクは 24h / 7 日後 Cookie 残存（ITP 長期影響）、Cloudflare Workers 実環境（`*.workers.dev`）デプロイ。致命的問題があれば Frontend Deploy を Cloud Run 等に切り替え、本 ADR を更新する。
- **M1 Backend PoC**: ローカル CLI + Docker container 検証は 2026-04-25 に成立（コミット `c2a5919`）。残タスクは Cloud Run 実環境デプロイ、コールドスタート計測、Cloud SQL 接続方式、Cloud Logging 連携、SIGTERM 実機確認、Cloud Run ↔ R2 レイテンシ。R2 接続 PoC（次工程）で一部を扱う。
- **HEIC デコード戦略**: libheif を含むコンテナイメージ構築方針を ADR-0005 実装時に確定する。Cloud Run サイドカー分離 or 単一イメージで libheif 同梱、のいずれか。
- **Email プロバイダ確定**: ADR-0004 の Proposed → Accepted への移行結果を本 ADR に反映する。
- **R2 レイテンシ実測**: Cloud Run 東京リージョンから R2 への presigned URL 発行 / 画像 GET のレイテンシを M1 / M3 で計測する。
- **Zustand 導入判断**: M7 のエディタ実装時、React state のみで回るか判断し、必要時のみ導入する。ルール違反を避けるため、グローバル状態は最小化する。

## 関連ドキュメント

- `docs/spec/vrc_photobook_business_knowledge_v4.md`（作成予定 / v4相当の業務知識）
- `docs/design/aggregates/README.md`（集約全体）
- `docs/design/aggregates/photobook/ドメイン設計.md`
- `ADR-0002 運営操作方式`
- `ADR-0003 フロントエンド認可フロー`
- `ADR-0004 メールプロバイダ選定`（Proposed）
- `ADR-0005 画像アップロード方式`
- `design/mockups/prototype/` （UI プロトタイプ、Tailwind 風の既存資産）
