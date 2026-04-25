# ADR-0003 フロントエンド認可フロー

## ステータス
Accepted

## 作成日
2026-04-25

## 最終更新
2026-04-25

## コンテキスト

VRC PhotoBook はログイン不要で動作する。作成者の権限は以下の 2 種類の raw token のみで表現される。

- `draft_edit_token`: Draft 状態のフォトブックを編集する権限（最大 7 日）
- `manage_url_token`: 公開後のフォトブックを管理する権限（長期）

これらは URL に含めてユーザーに渡すことを前提とした設計（管理URL方式）だが、raw token を URL に常時残すと以下の漏洩経路が存在する。

- Referrer ヘッダ経由の外部サイトへの漏洩
- ブラウザ履歴・オートコンプリート・ブックマーク経由
- 画面共有・スクリーンショット
- 共有 PC で前のユーザーのセッションが残る
- サーバーログ・CDN ログ・エラートラッキング（Sentry 等）への混入

v4 では「draft_edit_token は管理 URL と同等のセキュリティ要件」「管理 URL ページは Referrer-Policy: no-referrer」「エラートラッキングへの URL 送信禁止」が確定している。

さらに、`coding-rules.md`・`security-guard.md` により、API の executor ID は context から取得すべきであり、リクエストパラメータ経由の raw token で認可を実装すると事故源になる。

以上を踏まえ、raw token を URL に残さず、初回アクセスで短命 session に交換する方式を採用する。

## 決定

### 全体方針

- `draft_edit_token` / `manage_url_token` は **初回 URL アクセス時のみ URL に含める**。
- アクセス後、サーバーが token を検証し、**256bit 以上の暗号論的乱数を session token として生成**し、base64url 化した値を **HttpOnly Cookie に保存**する。DB には **session_token の hash のみ** を保存し、raw session token は保存しない。
- その後、**redirect により URL から raw token を除去**する。
- API 呼び出しでは raw token を query string や path で渡さない。API 認可は Cookie session のみ。

### 対象 URL（ルーティング衝突回避）

入場 URL と、redirect 先の通常ルートを別パスで分ける。これにより Next.js App Router の動的セグメント解析で曖昧性が発生しない（`{token}` と `{photobook_id}` が同一階層に並ばない）。

| 役割 | URL |
|------|-----|
| Draft 入場（token 消費） | `/draft/{draft_edit_token}` |
| Draft 編集（session 認可） | `/edit/{photobook_id}` |
| Manage 入場（token 消費） | `/manage/token/{manage_url_token}` |
| Manage 管理（session 認可） | `/manage/{photobook_id}` |

**Draft フロー**:

1. ユーザーが `/draft/{draft_edit_token}` にアクセス
2. サーバーが `draft_edit_token` を hash 化し DB 照合して検証
3. 256bit の暗号論的乱数 `session_token` を生成、`session_token_hash` を DB 保存
4. `Set-Cookie: vrcpb_draft_{photobook_id}=<session_token_base64url>; HttpOnly; Secure; SameSite=Strict; Path=/`
5. `/edit/{photobook_id}` に redirect（raw token を URL から除去）
6. 以後、API は Cookie session のみで認可

**Manage フロー**:

1. ユーザーが `/manage/token/{manage_url_token}` にアクセス
2. サーバーが `manage_url_token` を hash 化し DB 照合して検証
3. 256bit の暗号論的乱数 `session_token` を生成、`session_token_hash` を DB 保存、`token_version_at_issue = Photobook.manage_url_token_version` を記録
4. `Set-Cookie: vrcpb_manage_{photobook_id}=<session_token_base64url>; HttpOnly; Secure; SameSite=Strict; Path=/`
5. `/manage/{photobook_id}` に redirect
6. 以後、API は Cookie session のみで認可

### Cookie 名と属性

| Cookie 名 | 用途 |
|-----------|------|
| `vrcpb_draft_{photobook_id}` | Draft 編集 session |
| `vrcpb_manage_{photobook_id}` | Manage 管理 session |

Cookie 名に `photobook_id` を含めることで、ユーザーが複数のフォトブックを並行編集しても Cookie が衝突しない。

**Cookie 属性**:
- `HttpOnly: true`（JavaScript から不可視）
- `Secure: true`（HTTPS のみ）
- `SameSite: Strict`（CSRF 一次対策）
- `Path: /`
- draft session の期限: `draft_expires_at` まで、最大 7 日
- manage session の期限: 24 時間〜7 日程度。長期化しない（raw token と区別する意図）

### session 値の設計（重要）

**Cookie に入れる値は UUIDv7 ではなく、256bit 以上の暗号論的乱数を base64url 化したもの** とする。

理由：

- UUIDv7 は時刻情報を含むため、Cookie 値として使うと生成時刻が推測可能になる。
- Cookie に入る session 値は実質的な Bearer token であり、予測困難性を最大化するべき。
- DB 内部 ID としての UUIDv7 採用（ADR-0001）と、Cookie に入る session token の設計は独立。

**DB には session_token の hash のみ** を保存する。Cookie に入っている raw session token は DB に保存しない。これにより DB ダンプ漏洩時に raw session token が直接漏れない。hash アルゴリズムは SHA-256 を採用する（ストレッチングは不要、エントロピーが十分大きいため）。

### Session テーブル

```
sessions
├── id                    uuid         PK    -- DB 内部 ID（UUIDv7）
├── session_token_hash    bytea        UNIQUE NOT NULL  -- Cookie の raw token の SHA-256
├── session_type          text         NOT NULL    -- 'draft' | 'manage'
├── photobook_id          uuid         NOT NULL
├── token_version_at_issue int         NOT NULL DEFAULT 0  -- 発行時の Photobook.manage_url_token_version
├── expires_at            timestamptz  NOT NULL
├── created_at            timestamptz  NOT NULL
├── last_used_at          timestamptz  NULL
└── revoked_at            timestamptz  NULL
```

**インデックス**:
- `UNIQUE (session_token_hash)`
- `(photobook_id, session_type, expires_at)` — session 検証時
- `(photobook_id, session_type, token_version_at_issue)` — 一括 revoke 時
- `(revoked_at) WHERE revoked_at IS NOT NULL` — 監査用

### 明示的な session 破棄（共有 PC 対策）

編集画面・管理画面に以下の UI を用意する。

- 「この端末の編集権限を削除」（draft）
- 「この端末から管理権限を削除」（manage）

実行時の処理:
- 対応する session を `revoked_at = now()` で revoke
- Cookie を削除（`Set-Cookie: <name>=; Max-Age=0`）
- **元の管理 URL 自体は失効させない**（別端末からの再入場を妨げない）

session 期限切れ時:
- 次回アクセス時に期限切れ判定
- Cookie を削除
- 「管理 URL または Draft URL から再入場してください」と案内画面を表示

別端末から同じ管理 URL にアクセスした場合:
- 別 session として扱う
- 既存 session は自動失効させない
- 管理 URL を再発行した場合のみ、旧 token 由来の session を一括 revoke する（下記参照）

### token 再発行 / publish 時の session 失効ルール

**manage_url_token 再発行時**（`cmd/ops/photobook_reissue_manage_url`）:

- `Photobook.manage_url_token_version` をインクリメントする
- 対象 Photobook で `session_type = 'manage'` かつ `token_version_at_issue` が旧 version の session をすべて `revoked_at = now()` で revoke する
- draft session には影響しない（draft は publish 済みなら既に失効しているはず、publish 前の draft なら無関係）
- この処理は Photobook 更新、ModerationAction 記録、ManageUrlDelivery 新規作成、outbox_events INSERT と **同一トランザクション** で行う

**publish 成功時**（`PublishPhotobook` UseCase）:

- `draft_edit_token` を失効（`Photobook.draft_edit_token_hash = NULL`）
- 対象 Photobook で `session_type = 'draft'` の session をすべて `revoked_at = now()` で revoke する
- `manage_url_token` を発行、`manage_url_token_version = 1` で開始
- Outbox に `PhotobookPublished` を同一 TX で INSERT

### API 設計ルール

- API では `draft_edit_token` / `manage_url_token` を **query string で受け取らない**
- API では raw token を **URL path に含めない**
- API ログに token を出さない（構造化ログの禁止フィールドに `authorization`, `cookie`, `draft_edit_token`, `manage_url_token`, `session_token` を登録）
- Sentry 等のエラーログにも token を出さない（URL スクラブ設定、`beforeSend` フックで query と path の該当セグメントを剥ぐ）
- token 付き URL を受けるページ（`/draft/*`, `/manage/token/*`）では `Referrer-Policy: no-referrer`
- Draft / Manage ページでは外部画像、外部スクリプト、外部フォントを読まない（Referer 経由の URL 漏洩を構造的に防ぐ）
- X 共有時には必ず公開 URL のみを使う（UI 実装レベルで制約）

### SSR 時の Cookie 検証

Next.js App Router では以下の方針で実装する。

- token 付き初回 URL（`/draft/{token}` / `/manage/token/{token}`）は **Route Handler または Server Component** で検証する
- 検証成功時は session Cookie を発行して redirect する（`Response` に `Set-Cookie` を付与して 302、または `redirect()`）
- `/edit/{photobook_id}` や `/manage/{photobook_id}` では **Server Component 側で Cookie session を検証**する
- 無効 session の場合は、再入場 URL を求める画面へ誘導する
- Middleware で全リクエスト検証するかは M1 スパイクで確認する（Edge Runtime + DB アクセスの可否、レイテンシ影響を評価）
- API Route Handler（または Go バックエンドへのプロキシ）でも必ず session 検証を行う
- **UI 側の表示制御だけに頼らない**。権限は常にサーバー側で再検証する

### CSRF 対策

Cookie 認可を採用するため、CSRF 対策を明示的に行う。

**基本対策**:
- `SameSite=Strict`（主要ブラウザは遵守）
- `HttpOnly`
- `Secure`
- Origin ヘッダ検証（自オリジンのみ受理）
- Referer ヘッダ検証可能な場合は確認
- CORS は自サイトのみに限定

**状態変更 API の追加対策（MVP 方針）**:
- 通常の編集 API は `SameSite=Strict` + Origin チェックを必須とする
- **破壊的操作にはワンタイム確認トークン**（サーバー発行、単一操作用、5〜10 分で失効）を要求する

**破壊的操作の例**:
- フォトブック削除
- 公開範囲変更（public ↔ unlisted ↔ private）
- 管理 URL 再発行（運営側）
- センシティブフラグ変更

削除時はさらに UI 側で「削除」と入力させる確認を検討する。サーバー側の二重確認とは独立した UX 上の最終ガード。

### draft 延長ルール

- `draft_expires_at` の初期値: `now() + 7日`
- **編集系 API 成功時のみ** `draft_expires_at = now() + 7日` に延長
- GET やプレビュー閲覧では延長しない
- publish 成功時に `draft_edit_token` は失効し、draft session も全 revoke される（上記参照）
- publish 後は `manage_url_token` のみ有効

**編集系 API の例**:
- 内容更新
- 写真追加・削除
- ページ追加・削除
- 並び替え
- メタ情報更新
- 公開設定の保存（publish 直前）

この「編集したら延長、見ているだけでは延長しない」ルールにより、放置された draft は自動的に期限切れとなり、reconciler で GC される（ADR-0005 GC 方針とも連携）。

## 検討した代替案

- **URL path に token を残し続ける**: Referrer リーク、履歴漏洩、ブックマーク漏洩。最悪の選択肢。
- **token を localStorage に保存する**: XSS で即漏洩、HttpOnly にできない、sub-domain 分離もできない。
- **token を Cookie に raw 値で保存する**: Cookie 漏洩即侵害。短命化もできない。session 層を挟む意味を失う。
- **Cookie に session_id（UUIDv7）をそのまま入れる**: UUIDv7 は時刻情報を含むため予測可能性が上がる。また DB 内部 ID と Cookie 値を同一にすると、DB 漏洩時に Cookie 偽造に直結する。採用せず、Cookie には乱数 session token を入れて DB には hash のみを保存する。
- **query string で API に token を渡す**: 各 API アクセスログに token が残る。reverse proxy・CDN・メトリクス・APM のすべてにログが広がる。
- **ログイン必須にする**: MVP 方針「ログイン不要」と真っ向対立。却下。
- **`SameSite=Strict` のみで CSRF 対策を完結させる**: 主要ブラウザは遵守するが、古いブラウザや一部の navigation パターンでの挙動差がある。破壊的操作では追加のワンタイムトークンで二重防衛する方が MVP のリスク許容度に合う。
- **入場 URL と編集 URL を同一階層（`/edit/{何か}`）で分岐させる**: `{token}` と `{photobook_id}` が動的ルートで衝突し、Next.js App Router の解釈が紛らわしくなる。入場 URL を `/draft/*` / `/manage/token/*` として階層を分けるほうが実装事故を防げる。

## 結果

### メリット

- raw token が URL に残らず、Referrer・履歴・画面共有経由の漏洩を構造的に抑制
- 管理 URL（長期の再入場手段）と操作 session（短期）の責務分離で、端末側の漏洩窓を狭められる
- Cookie HttpOnly + SameSite=Strict + Origin 検証 + 破壊的操作のワンタイムトークンで多層防御
- session revoke で共有 PC の利用にも対応
- `token_version_at_issue` により、管理 URL 再発行時に旧 token 由来の session を一括 revoke できる
- publish 時に draft session を全 revoke できるため、publish 後に draft 画面が残る事故を防げる
- 入場 URL と編集 URL の階層分離で Next.js App Router のルーティング衝突を回避

### デメリット

- 実装複雑度が上がる（token 検証ルート、session テーブル、Middleware/Server Component 検証の二重実装）
- session テーブルが 1 つ増える
- Next.js App Router + HttpOnly Cookie + redirect の組み合わせに Safari 特有の挙動（ITP、SameSite の扱い差異）があるため、M1 スパイクで確認が必要
- 破壊的操作のワンタイムトークン発行 API が追加で必要
- 別端末での同時編集時の UX（楽観ロックエラーのハンドリング）を UI で丁寧に扱う必要がある

### 後続作業への影響

- M1: Next.js Cloudflare Pages スパイクで HttpOnly Cookie と redirect の挙動を検証する（ADR-0001 参照）。
- M3: `sessions` テーブル migration と Repository / Query 実装。session token 生成ヘルパ（`crypto/rand` から 32 バイト生成 → base64url）の実装。
- M4: session 発行 UseCase（`ExchangeTokenForSession`）、破壊的操作のワンタイムトークン発行・消費 UseCase、publish 時の draft session 全 revoke、reissue_manage_url 時の manage session 一括 revoke。
- M5: Cookie 検証 middleware、Origin 検証 middleware、session 無効時のリダイレクト。
- M7: Frontend 側で session 切れ時の案内画面、session 明示破棄 UI、X 共有時の公開 URL 限定。
- ログ: 構造化ログのフィールドに `authorization`・`cookie`・`draft_edit_token`・`manage_url_token`・`session_token` を禁止フィールドとして登録し、マスク処理を中央化する。

## M1 検証結果（2026-04-25）

`harness/spike/frontend/`（OpenNext adapter 版、コミット `6e2840a`）でフロント PoC を構築し、**token → HttpOnly Cookie session → redirect** 方式の中核動作を以下の経路で検証した。

### 検証成立を確認した項目

| 検証経路 | 結果 |
|---------|:---:|
| CLI 検証（curl）: Next.js 標準 dev サーバ | ✅ |
| CLI 検証（curl）: OpenNext + wrangler dev（Workers 互換ローカル） | ✅ |
| **macOS Safari 実機検証**（2026-04-25） | ✅ 大きな問題なし |
| **iPhone Safari 実機検証**（2026-04-25） | ✅ 大きな問題なし |

#### Safari 実機検証で確認できたこと

- `/draft/{token}` → 302 + `Set-Cookie` → `/edit/{photobook_id}` へ redirect、URL から token 消去
- `/manage/token/{token}` → 同様に `/manage/{photobook_id}` へ redirect
- redirect 後、Server Component で Cookie 読取 → `draft session found` / `manage session found` 表示
- Cookie 属性（`HttpOnly` / `Secure` / `SameSite=Strict` / `Path=/`）を Web Inspector で目視確認
- ページ再読込後も session 維持

→ **token → session 交換方式（本 ADR の中核）は、macOS Safari / iPhone Safari の実機検証でも成立**することを確認した。本 ADR の方針は M2 本実装で採用する。

### 継続観察項目（M1 では時間制約上未確認）

下記は時間経過が必要な検証のため、M1 残作業 + Cloudflare Workers 実環境デプロイ後に再確認する:

- 24 時間後 / 7 日後の Cookie 残存（**ITP 影響評価**）
- iOS Safari 1 世代前での再確認
- iPad Safari
- プライベートブラウジング動作
- 数時間〜24 時間スパンで再アクセス時の session 維持

これらが「Cookie が想定外に消える」結果になった場合、§案E（Cookie 発行ホストの統一）/ §案F（token を URL に残す方式へ後退）/ §案G（独自親ドメイン共有）への切替を検討する。

### M1 実環境デプロイ後の追加検証結果（2026-04-26）

Workers 実環境（`https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev`）と Cloud Run 実環境（`https://vrcpb-spike-api-7eosr3jcfa-an.a.run.app`）の両方をデプロイし、以下を確認した（詳細ログ: `harness/work-logs/2026-04-26_m1-live-deploy-verification.md`）。

#### 確認できた事実

- macOS Safari / iPhone Safari の両方で **Workers 実環境**でも token → session 交換 + redirect + Cookie 引き継ぎが成立
- Cookie 属性（HttpOnly / Secure / SameSite=Strict / Path=/、draft 7 日 / manage 24 時間）はブラウザ DevTools で目視確認済
- Workers `*.workers.dev` 上で発行された Cookie は Workers ホスト上で正しく保持・再読込後も維持

#### U2 Cookie Domain の確定材料（重要）

`harness/spike/frontend/integration/backend-check` 経由で実機検証したところ、`GET /sandbox/session-check (credentials: include)` が **`{"draft_cookie_present":false,"manage_cookie_present":false}`** を返した。

これは **設計失敗ではなく**、ブラウザ仕様 + 構成上の想定通りの挙動である:

- Frontend `*.workers.dev` で発行された Cookie は、Cookie の Domain が **発行ホスト**に閉じる
- Backend `*.run.app` への `credentials: include` fetch では、Backend ホストに該当 Cookie が **付かない**
- CORS / preflight / Origin 反射 / `Access-Control-Allow-Credentials: true` は別途すべて成立しているため、ブラウザ仕様としての別オリジン Cookie 不通だけがブロッカー

これにより `docs/plan/m1-live-deploy-verification-plan.md` §7 Cookie Domain U2 検証案の評価が確定:

| 案 | M1 確認結果 | M2 推奨度 |
|---|---|---|
| **案A（共通親ドメイン + Cookie Domain `.example.com`）** | M1 では未実施（独自ドメイン未取得）| **第一候補**：M2 早期に独自ドメイン取得 → 案 A を採用 |
| 案B（Workers `/api/*` プロキシで同一オリジン化）| M1 では未実施 | 第二候補（独自ドメイン取得が遅延した場合）|
| 案C（Cookie 共有しない、Backend 認可方式を再設計）| 不採用 | 採用しない（本 ADR の根本変更を避ける）|

**M2 早期の必須タスクとして「独自ドメイン取得 → 案 A 採用」を本 ADR §未解決事項 U2 で確定**する。これにより M1 計画書 §13 失敗時の判断 §「Cookie Domain が期待通り動かない」は **想定通り発生 → 案 A 一次方針で吸収**として整理済となる。

### 今後の運用ルール（再確認）

### 今後の運用ルール（再々確認）

- **Cookie 発行 / redirect / OGP / レスポンスヘッダ / モバイル UI を変更した場合、macOS Safari と iPhone Safari の確認を必須にする**
- ルール詳細: `.agents/rules/safari-verification.md`

## 未解決事項 / 検証TODO

- **Middleware 全リクエスト検証 vs Server Component 単位検証**: M1 スパイクで、Edge Runtime で DB アクセスが可能か、レイテンシが許容範囲かを計測して決定する。
- **session token の長さ**: 32 バイト（256bit）を base64url 化すると 43 文字。Cookie サイズ・可読性ともに問題ない想定だが、M3 で最終決定。
- **別端末同時編集時の UX**: 楽観ロック（`photobook.version`）衝突時のモーダル文言と復旧フロー。
- **破壊的操作のワンタイムトークン仕様**: スコープ（1操作1トークン）、有効期限（5〜10分）、保存先（`sessions` テーブル拡張 or 別テーブル）を M4 で確定。
- **Safari ITP の長期影響評価（24h / 7 日後）**: 上記「継続観察項目」のとおり。実環境デプロイ後に時間経過観察を行う。本 ADR の方針自体には現時点で問題なし。**Workers 実環境デプロイ完了済（2026-04-26、起点）**、24h / 7 日後の再アクセス確認はユーザー側で継続観察。
- **U2 Cookie Domain（Frontend と Backend が異なるホスト）**: 2026-04-26 の Workers + Cloud Run 実環境で「別オリジン下では Cookie が Backend に渡らない」挙動を確認済。**M2 早期に独自ドメインを取得し、案 A（共通親ドメイン + Cookie Domain `.example.com`）を採用する**を一次方針として確定（上記 §M1 実環境デプロイ後の追加検証結果）。

## 関連ドキュメント

- `docs/spec/vrc_photobook_business_knowledge_v4.md`（作成予定 / v4相当の業務知識）
- `docs/design/aggregates/photobook/ドメイン設計.md`（ManageUrlToken / DraftEditToken / manage_url_token_version）
- `docs/design/aggregates/photobook/データモデル設計.md`
- `.agents/rules/security-guard.md`
- `ADR-0001 技術スタック`
- `ADR-0002 運営操作方式`（reissue_manage_url から session 一括 revoke を呼ぶ）
- `ADR-0005 画像アップロード方式`（upload_verification_session の扱いと整合）
