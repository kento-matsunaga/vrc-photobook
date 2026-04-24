# ADR-0005 画像アップロード方式

## ステータス
Accepted

## 作成日
2026-04-25

## 最終更新
2026-04-25

## コンテキスト

VRC PhotoBook では、作成者が 1 冊のフォトブックあたり数枚〜数十枚の画像をアップロードする。v4 で確定している画像関連の制約は厳しい。

- 画像 1 枚あたり最大 10MB
- 最大長辺 8192px、最大 40MP
- SVG 禁止
- アニメーション WebP / APNG 拒否
- EXIF / XMP / IPTC 全メタデータ原則除去（GPS・シリアル・PC ユーザー名・撮影日時）
- HEIC 入力は内部で JPG / WebP に変換
- Image 集約は owner_photobook_id を持ち、publish 時も変更しない
- 画像状態: uploading → processing → available / failed / deleted / purged
- failed 画像は公開利用禁止

API サーバーが multipart を直接受ける方式には以下の問題がある。

- Cloud Run のリクエストサイズ・時間制限（デフォルト 32MB / 60 秒）
- 数十枚の並列アップロードでメモリ / 帯域がピーク化し、API 本体のレスポンス劣化
- HEIC デコード / variant 生成を同期処理するとタイムアウトリスク大
- 画像バイナリがアプリログ・APM に意図せず混入するリスク

一方、R2 presigned URL 方式は次の利点を持つ。

- API サーバーが画像バイナリを抱えない
- フロントから R2 へ並列 PUT 可能
- 画像処理を非同期化しやすい（Outbox + image-processor）

また v4 で `ImageIngestionRequested` が Outbox 対象イベントとして確定しているため、非同期化は設計前提である。

## 決定

### 全体方針

**API サーバー multipart 直送ではなく、R2 presigned URL 方式を採用する**。2 段 API（upload-intent + complete）+ 非同期処理（image-processor）でフローを分離する。

### 基本フロー

1. フロントが `POST /api/photobooks/{id}/images/upload-intent` を呼ぶ
2. サーバーが **Turnstile を検証**する（セッション化済みならスキップ、後述）
3. サーバーが **RateLimit を確認**する
4. サーバーが **軽量バリデーション**を行う（申告 content-type、Content-Length、拡張子）
5. **Image レコードを作成**（status=uploading, owner_photobook_id 固定, usage_kind=photo）
6. **R2 presigned URL を発行**（PUT 用、15 分有効）
7. フロントが R2 へ直接 PUT
8. フロントが `POST /api/images/{image_id}/complete` を呼ぶ
9. サーバーが R2 上のオブジェクト存在を確認
10. Image が対象 Photobook に属しているか確認
11. Image 状態が uploading であることを確認
12. `outbox_events` に `ImageIngestionRequested` を INSERT（Image 状態更新と同一 TX）
13. image-processor が非同期で本検証・変換・EXIF 除去・variant 生成を行う

### Turnstile 検証

Turnstile 検証は **upload-intent 発行前** に行う。complete 時ではなく upload-intent 時に置くのは、署名 URL の大量発行を防ぐため。署名 URL そのものはサーバーリソースであり、無制限に発行させない設計にする。

ただし、画像 20 枚アップロード時に毎回 Turnstile を要求すると UX が著しく悪化する。そこで **Turnstile 検証結果をセッションスコープで短期保持**する。

**MVP 方針**:
- Turnstile 検証成功後、`upload_verification_session` を発行する
- 有効期限 30 分
- 1 検証あたり最大 20 回の upload-intent を許可
- 対象 Photobook ID に紐づける（他フォトブックに流用不可）
- 期限切れ、回数超過、Photobook 不一致の場合は再検証を求める

**失敗時**:
- 403 相当
- エラーコード: `turnstile_verification_failed`
- 再試行可能な文言を返す
- 過度な失敗は UsageLimit 対象にする（bot / abuse 検知）

#### upload_verification_session の保存先

**MVP では DB テーブル `upload_verification_sessions` で管理する**。Redis を増やすより DB に寄せることで、MVP のミドルウェア点数を抑える。Phase 2 以降で QPS が上がった時点で Redis 移行を検討する。

テーブル案:

```
upload_verification_sessions
├── id                     uuid         PK           -- UUIDv7
├── photobook_id           uuid         NOT NULL
├── session_token_hash     bytea        UNIQUE NOT NULL  -- Cookie/header に載せる session token の SHA-256
├── allowed_intent_count   int          NOT NULL DEFAULT 20
├── used_intent_count      int          NOT NULL DEFAULT 0
├── expires_at             timestamptz  NOT NULL
├── created_at             timestamptz  NOT NULL
└── revoked_at             timestamptz  NULL
```

インデックス:
- `UNIQUE (session_token_hash)`
- `(photobook_id, expires_at)`

セッション値は ADR-0003 の session と同様、256bit の暗号論的乱数を base64url 化したものを使い、DB には hash のみを保存する。

上記テーブルを独立させず、ADR-0003 の `sessions` テーブルに `session_type = 'upload_verification'` として統合する案もあるが、用途・回数制限・期限ポリシーが異なるため MVP では分離テーブルとする。統合は Phase 2 で検討。

### upload-intent 時の検証

upload-intent では以下を検証する（軽量・早期拒否）。

- Turnstile（セッション有効なら簡易チェック、無効なら full 検証）
- RateLimit（upload_image の閾値）
- 画像枚数上限（Photobook あたりの上限）
- 1 画像 10MB 上限（申告）
- 拡張子の軽量チェック（jpg / jpeg / png / webp / heic / heif）
- content-type の仮チェック（申告値）
- SVG 拒否の軽量チェック（申告 content-type と拡張子で早期拒否）
- 申告された Content-Length が 10MB 以下であること

### Content-Length / サイズ検証

presigned URL 発行時に、可能な限り **Content-Length または content-length-range 制約**を設定する。R2 の S3 互換 API で制約が効くかは M3 / M6 で実地検証する（R2 は POST Policy の content-length-range は対応するが、PUT の Content-Length 制約は SDK 次第）。

R2 / S3 互換機能で制約できない場合でも、complete 時および image-processor 時に **必ず実サイズを確認**する。サーバー側で二重に検証することで、フロントからの改ざんや R2 側での仕様差異に耐える。

実際にアップロードされたサイズが申告と異なる場合:
- 10MB を超えていれば `ImageStatus = failed` とする
- `failure_reason` に `file_too_large` または `size_mismatch` を記録する
- 公開 Photobook には紐づけない（processor で failed 化）

### presigned URL

presigned URL の有効期限は **15 分** とする。

**理由**:
- 長すぎると漏洩時の悪用余地（意図しない第三者による書き込み）が増える
- 短すぎると大きめ画像・モバイル回線で失敗しやすい
- 15 分は MVP 初期値として妥当

必要に応じて、30 分への延長は将来検討とする（モバイル回線でのタイムアウト実測結果を見て判断）。

**presigned URL はログに出さない**。構造化ログの禁止フィールドに登録する（`url`, `presigned_url`, `storage_key` は慎重に扱う）。

### storage_key 命名規則

storage_key は以下のパターンとする。外部からの推測耐性とキャッシュ破棄を兼ね、各 variant に乱数サフィックスを付ける。

```
photobooks/{photobook_id}/images/{image_id}/original/{random}.{ext}
photobooks/{photobook_id}/images/{image_id}/display/{random}.webp
photobooks/{photobook_id}/images/{image_id}/thumbnail/{random}.webp
photobooks/{photobook_id}/ogp/{ogp_id}/{random}.png
```

- `{random}` は 8〜16 バイト（12 バイト推奨）の暗号論的乱数を base64url 化したもの
- `{ext}` は original の場合のみ元拡張子、display / thumbnail は常に webp
- R2 バケットの公開設定は使わず、CDN 配信時は署名付き URL または Workers 経由にする（M6 で詳細決定）

storage_key は DB の `image_variants.storage_key` に保存し、presigned URL 発行時はこの値を使う。complete 時に改ざんされていないかをサーバー側で照合する。

### complete 時の検証

complete は「フロントから PUT が成功したよ」という通知にすぎず、**通知だけを信用しない**。サーバー側で以下を全て確認する。

- R2 オブジェクトが存在する（HeadObject）
- Image が対象 Photobook に属している（owner_photobook_id 一致）
- `Image.status == uploading`
- Presigned URL 発行時の想定 storage_key と一致する
- アップロードサイズが上限（10MB）を超えていない
- すでに complete 済みではない（冪等性）
- 対象 Photobook が draft または編集可能状態である

これにより、フロント改ざん・リプレイ攻撃・他フォトブックの画像差し替えを防ぐ。

### image-processor 時の本検証

非同期ワーカー（`image-processor`）が `ImageIngestionRequested` を受け取り、以下を実行する。

- マジックナンバー検証（先頭数バイトで実形式判定、申告 content-type に依存しない）
- 実デコード（デコード成功まで本物とみなさない。デコンプレッションボム対策含む）
- 最大長辺 8192px
- 最大 40MP
- アニメーション WebP / APNG 拒否
- SVG 拒否（マジックナンバーで再確認）
- EXIF / XMP / IPTC 除去
- HEIC 変換（libheif 経由で JPG / WebP 化、normalized_format に残すのは JPG / WebP のみ）
- display / thumbnail variant 生成
- usage_kind ごとの処理（photo / cover / ogp）
- 失敗時は `ImageStatus = failed`、`failure_reason` を保存
- 成功時は `ImageStatus = available`

### failure_reason の語彙

MVP では `rejected` のような独立した状態は設けず、失敗は全て `ImageStatus = failed` に集約する。理由は `failure_reason` で区別する。初期語彙は以下とする（v4 業務知識で追加定義があれば合わせて更新）。

- `file_too_large` — 申告または実サイズが 10MB 超
- `size_mismatch` — 申告 Content-Length と実サイズが不一致
- `unsupported_format` — 受け入れ対象外の形式（GIF、BMP、TIFF 等）
- `svg_not_allowed` — SVG は拒否
- `animated_image_not_allowed` — アニメーション WebP / APNG
- `dimensions_too_large` — 長辺 8192px 超 または 40MP 超
- `decode_failed` — 実デコードに失敗（破損ファイル、デコンプレッションボム疑い含む）
- `exif_strip_failed` — メタデータ除去に失敗
- `variant_generation_failed` — display / thumbnail 生成に失敗
- `storage_missing` — complete 時に R2 オブジェクトが存在しない
- `internal_error` — 上記に分類できない内部エラー

`failure_reason` は enum（text + CHECK 制約）で管理し、未知の値が入らないようにする。

### Image 状態遷移

```
uploading → processing → available
         ↘             ↘
           failed       deleted → purged
```

- `uploading`: upload-intent で作成。presigned URL 発行済み、R2 に PUT 待ち
- `processing`: complete 受信後、image-processor 実行中
- `available`: 正規化 / variant 生成完了、利用可能
- `failed`: 検証失敗・変換失敗。`failure_reason` を保存、公開利用禁止
- `deleted`: soft delete。一定期間後に purge
- `purged`: 物理削除（R2 オブジェクト削除含む）

### upload-intent で Image 作成時

- `status = uploading`
- `owner_photobook_id = 対象 Photobook ID`（publish 時も変更しない。v4 所有モデル）
- `usage_kind = photo`
- **draft 画像も owner_photobook_id を持つ**（Draft と Published は同一 Photobook レコード）

### Draft との関係

- 初回画像アップロード時に server draft Photobook がなければ作成する（`CreateDraftPhotobook` UseCase を呼ぶ）
- draft と published は **同じ Photobook ID**
- publish 時に `Image.owner_photobook_id` は変更しない（status 遷移のみ）

### GC 方針

以下を自動 reconciler と `scripts/ops/reconcile` の両方で扱う。

- **upload-intent 後、complete されない Image**: 一定時間（例: 1 時間）後に削除対象。presigned URL 期限 15 分 × 安全マージンで決定。
- **`draft_expires_at` を過ぎた draft に紐づく Image**: 削除対象（draft_expired reconciler で連鎖削除）
- **failed 状態で一定期間経過した Image**: 7 日で deleted、30 日で purged
- **deleted 状態で一定期間経過した Image**: 30 日で purged（R2 オブジェクト削除）

手動 reconcile:
- `scripts/ops/reconcile/image_references.sh`（孤立画像検出）
- `scripts/ops/reconcile/photobook_image_integrity.sh`（Photobook と Image の参照整合性検査）

自動 reconciler:
- `draft_expired` 連鎖で画像削除をキューイング
- `outbox_failed_retry` で `ImageIngestionRequested` のリトライ

### 認可

upload-intent / complete ともに **Cookie session による認可**を使う（ADR-0003）。draft_edit_token / manage_url_token を直接 API に渡さない。`owner_photobook_id` と session の photobook_id が一致することをサーバーが検証する。

## 検討した代替案

- **API サーバー multipart 直送**: Cloud Run のリクエストサイズ・時間制限、メモリピーク、API 本体との帯域競合。MB 級の並列 PUT で落ちる。
- **Next.js Route Handler で multipart を受ける**: Cloudflare Pages / Workers での bodySizeLimit（デフォルト小）、CPU 時間制限（Workers では 50ms〜5分）、ネイティブライブラリ不可で HEIC デコードできない。
- **Cloudflare Workers で画像を受ける**: 上記と同じ。HEIC / EXIF 除去に libheif / libvips が必要で Workers では動かない。
- **Base64 で API 送信**: バイナリサイズが 1.33 倍、JSON パースでメモリ二重消費、ログに混入リスク増。最悪の選択肢。
- **直接 R2 公開 URL にアップロード**: 認可ゼロ、無制限書き込みで bot / abuse の温床。
- **upload-intent ごとに毎回 Turnstile を要求**: 20 枚アップロードで 20 回チャレンジが出て UX が破綻。完了率が激落ちする。
- **`rejected` を独立した ImageStatus として定義**: 状態遷移が複雑になり、集約側の判定ロジックが分岐する。`failed` に集約し `failure_reason` で区別する方が扱いやすい。

## 結果

### メリット

- API サーバーが画像バイナリを抱えない → メモリ・帯域・レイテンシが安定
- フロントから R2 へ並列 PUT 可能 → 大量画像アップロードが高速
- 画像処理を非同期化（Outbox + image-processor）で、リトライ・タイムアウト・失敗扱いが素直
- Turnstile を upload-intent 前に置くことで署名 URL 大量発行を防げる
- Turnstile セッション化（DB 管理）で UX を維持
- owner_photobook_id で画像の帰属が常に明確（v4 所有モデル）
- Draft / Published を同一 Photobook ID で扱えるため、publish 時に画像再アサインが不要
- `failure_reason` 語彙で失敗原因が一貫して追跡できる

### デメリット

- 2 段 API（upload-intent + complete）でフロント実装が複雑化
- complete 通知漏れ（ユーザー離脱）でゴミ Image が発生 → reconciler で GC
- presigned URL の漏洩リスク（15 分短命 + ログ非出力で緩和）
- R2 / S3 互換の content-length-range 制約の挙動差を M3 / M6 で実地検証する必要がある
- HEIC デコード用の libheif をコンテナイメージに含める必要がある（純 Go では不可、cgo + libheif）
- `upload_verification_sessions` テーブルが 1 つ増える（MVP は DB 管理、Phase 2 で Redis 移行検討）

### 後続作業への影響

- M3: R2 Adapter、sqlc で `images` / `image_variants` / `upload_verification_sessions` のクエリ生成。storage_key 生成ヘルパ（photobook_id / image_id / 乱数）実装。
- M4: `CreateImageUploadIntent`、`CompleteImageUpload` の 2 UseCase、upload_verification_session の発行・消費・失効処理。
- M5: `POST /api/photobooks/{id}/images/upload-intent`、`POST /api/images/{id}/complete` の handler、Cookie session 認可 + Turnstile middleware。
- M6: `image-processor` ワーカー（cgo + libheif）コンテナイメージ構築、variant 生成ロジック、failed 時の `failure_reason` 記録。
- M7: フロントからの並列 PUT、進捗表示、失敗時のリトライ UI、Turnstile ウィジェット統合。

## 未解決事項 / 検証TODO

- **R2 の content-length-range / Content-Length 制約の実地検証**（M3 冒頭）
- **HEIC デコードの image-processor コンテナ**（libheif + libde265 同梱、Cloud Run Jobs / Worker で動くか）
- **GC タイミング**（uncomplete 画像の保持時間 1h / 3h / 6h のどれが妥当か、フロント離脱率の実測後に調整）
- **complete の冪等性**（フロントのリトライでも二重 Outbox 発行しない設計、`IF NOT EXISTS` or outbox 側 dedup key）
- **並列 PUT 時のエラーハンドリング**（部分失敗時の UX、リトライ時に presigned URL を再発行するか）
- **`upload_verification_sessions` と ADR-0003 の `sessions` テーブル統合**（Phase 2 で検討）
- **`failure_reason` 語彙の確定**（v4 業務知識側の定義と整合する最終リストを v4 確定時に合わせて更新）

## 関連ドキュメント

- `docs/spec/vrc_photobook_business_knowledge_v4.md`（作成予定 / v4相当の業務知識）
- `docs/design/aggregates/image/ドメイン設計.md`
- `docs/design/aggregates/image/データモデル設計.md`
- `docs/design/aggregates/photobook/ドメイン設計.md`（Draft / Published、owner_photobook_id）
- `docs/design/aggregates/usage-limit/ドメイン設計.md`（RateLimit / ActorIdentifier）
- `.agents/rules/security-guard.md`
- `ADR-0001 技術スタック`（R2 採用）
- `ADR-0002 運営操作方式`（reconcile 系 CLI）
- `ADR-0003 フロントエンド認可フロー`（Cookie session 認可、session token 設計）
