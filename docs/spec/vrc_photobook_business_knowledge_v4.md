# VRC PhotoBook 業務知識定義書 v4

> 本ドキュメントは v3 の後継。実装前に確定すべき論点をレビューで整理し、MVP 実装着手可能な水準まで補強したものである。
>
> v3 からの変更サマリは末尾の「付録B: バージョン間の主な改訂点」参照。
>
> **v4 改訂の主題**: (1) 画像の所有モデルを `owner_photobook_id` 方式に切り替え、(2) `server draft + draft_edit_token` の採用、(3) OGP生成の独立管理、(4) Transactional Outbox の MVP 採用、(5) 管理URL漏洩対策の強化、(6) Reconcile による整合性保証、(7) 未成年関連通報カテゴリの独立。

---

## 第1部 サービス全体の前提

### 1.1 サービスの定義

VRC PhotoBookは、VRChatで撮影した写真を「フォトブック」という一貫した形式にまとめ、Webで公開しXで共有するためのサービスである。

単なる画像保存サービスではなく、Xで共有されたリンクを踏んだ第三者が「自分も作りたい」と感じる体験を提供することを目的とする。

### 1.2 根本ポリシー

本サービスは以下の根本ポリシーのもとに設計される。

- **ログイン不要で完結する**。作成・公開・閲覧・編集・削除はログインなしで行える
- **ログインは任意**。複数フォトブックの横断管理や有料機能を使うための手段である
- **編集・削除手段は作成者に帰属する**。ログインなしでも編集・削除できる仕組み（管理URL方式）を提供する
- **スマホファースト**。Xから流入する閲覧者の多くはスマートフォンを利用する
- **写真が主役**。UIや演出は写真の魅力を妨げてはならない
- **SNS化しない**。フォロー、いいね、コメント等の機能を安易に持ち込まない
- **最低限の安全性を持つ**。荒らし、無断転載、センシティブ投稿に対する最低限の防御と対応手段を持つ

### 1.3 想定される利用シーン

v3 §1.3 と同じ。イベント記録／ワールド記録／フレンドとの思い出／日常ログ／作品集／アバター紹介。

### 1.4 登場するアクター

v3 §1.4 と同じ。作成者／閲覧者／運営／任意ユーザー。

---

## 第2部 ユビキタス言語（共通語彙）

### 2.1 フォトブックの構造に関する用語

| 用語 | 定義 |
|---|---|
| フォトブック | 複数のページで構成される、公開・共有可能な1つの単位 |
| ページ | フォトブック内の1つの表示単位。**MVPでは 1ページ = 1写真を基本とする**（将来拡張で複数写真を許容） |
| 写真 | フォトブックを構成する画像ファイル |
| キャプション | ページまたは写真ごとに付く短い説明文 |
| メタ情報 | ページに任意で付く構造化情報。World ワールド名、Cast キャスト、Photographer 撮影者、Note 補足メモ の4種 |
| 表紙 | フォトブックの先頭に置かれる、タイトルと代表画像からなる特別なページ。**MVPでは `photobooks` テーブルにインライン化する**（v4 新規） |
| 作成者情報 | フォトブックに紐付く作成者の表示情報。**MVPでは表示名と X ID のみ**（v4 で creator avatar 画像は Phase 2 に延期） |

### 2.2 UI上の表示要素に関する用語

v3 §2.2 と同じ。

### 2.3 URLと編集トークンに関する用語（v4 更新）

ログイン不要を実現する中核概念のため、揺らぎは特に禁止する。

| 用語 | 定義 |
|---|---|
| 公開URL | 閲覧者に共有されるURL。Xに貼るのはこのURL |
| 管理URL | 作成者のみが保持する、編集・削除・設定変更のためのURL。公開後に発行される |
| 管理URL控え | 作成者が管理URLを紛失しないための手段（コピー、メール送信、端末保存）の総称 |
| **draft編集URL** | **作成途中（server draft）のフォトブックにアクセスするためのURL。`draft_edit_token` を含む。公開成功で失効する**（v4 新規） |
| **draft_edit_token** | **draft状態のフォトブック編集用トークン。初回画像アップロード時にサーバー側で発行**（v4 新規） |

### 2.4 フォトブックタイプと表示に関する用語

v3 §2.4 と同じ。内部タイプ7種と表示タイプ名の2層で扱う。

### 2.5 公開とモラルに関する用語

v3 §2.5 と同じ。「公開」「限定公開」「非公開」の3値。非公開は「管理URL保持者のみ閲覧可能」。

### 2.6 荒らし対策に関する用語

v3 §2.6 と同じ。

### 2.7 非採用用語

v3 §2.7 の非採用用語はすべて維持。v4 で追加する非採用語はなし。

### 2.8 集約間イベント用語（v4 新規）

| 用語 | 定義 |
|---|---|
| **Outbox** | **集約の状態変更と副作用（OGP生成、メール送信、CDNパージ等）の実行をトランザクション的に保証するためのイベント記録テーブル** |
| **PhotobookPublished / PhotobookUpdated / PhotobookHidden / PhotobookUnhidden / PhotobookSoftDeleted / PhotobookRestored / PhotobookPurged / ManageUrlReissued / ReportSubmitted / ManageUrlDeliveryRequested** | 集約間の業務イベント（Outboxに記録される） |
| **Reconcile** | **Outbox失敗・状態不整合・孤児データを検出し、手動または自動で修復する運用プロセス** |

---

## 第3部 機能定義

### 3.1 フォトブック作成機能（v4 更新）

#### 責任範囲

作成者がVRChatで撮影した写真をまとめて、公開可能な形に整える一連の体験を提供する。

#### この機能が担うこと

v3 §3.1 の担うことに加え、v4 では以下を明示する。

- **初回画像アップロード時に server draft Photobook を作成し、`draft_edit_token` を発行する**
- **draft 編集URL を作成者に提示する（公開URLや管理URLとは別物）**
- **draft の有効期限（`draft_expires_at`）をアクセスごとに延長する**（アクセスから7日）

#### draft ライフサイクル（v4 新規）

```
作成開始
  ↓
初回画像アップロード
  ↓
server draft Photobook 作成（status='draft'、public_url_slug/manage_url_tokenは未発行）
  ↓
draft_edit_token 発行、draft_expires_at = now + 7日
  ↓
編集アクセスごとに draft_expires_at を now + 7日に延長
  ↓
（公開前まで）
  ↓
publish成功
  ↓
public_url_slug / manage_url_token 発行、draft_edit_token 失効
  ↓
以後は管理URLのみ有効
```

#### この機能が守ること

- 作成者は一切のログインなしに、フォトブックの中身を作り上げることができる
- 作成者がタイプを選んだ時点で、そのタイプに最適な既定レイアウト・既定の開き方が自動で設定される
- 既定の選択肢だけで、それなりに見栄えのするフォトブックが成立する
- **draft 編集URLの漏洩は管理URLと同等のリスクを持つため、管理URLと同じ漏洩対策を適用する**（v4 新規）
- **ブラウザローカル保持に依存しない**。画像のアップロード → server draft 化で端末依存性を軽減する（v4 新規）
- 画像本体がブラウザ側で復元できない場合は、作成者に再アップロードを求める

#### 付随する業務ルール

- **draft状態では `public_url_slug` と `manage_url_token` は発行されない**
- **draft状態の Photobook に紐づく Image は `owner_photobook_id` でこの draft を指す**
- **`draft_expires_at` を過ぎた draft は削除対象となる**（`draft_expired.sh` による Reconcile）
- タイプを変更した場合、既定レイアウトと既定の開き方が新しいタイプのものに切り替わる

---

### 3.2 フォトブック公開機能（v4 更新）

#### 責任範囲

draft Photobook を実際にWeb上で閲覧可能な状態にする最終工程を担う。

#### この機能が担うこと

v3 §3.2 の担うことに加え、v4 では以下を明示する。

- **公開成功時、`public_url_slug` と `manage_url_token` を同一トランザクション内で発行する**
- **公開成功時、`draft_edit_token` を失効させる**
- **公開成功時、`outbox_events` に `PhotobookPublished` をINSERTする**（同一トランザクション）
- **OGP画像生成は非同期（`photobook_ogp_images` を独立管理）で、公開成功 → 後続で生成される**

#### この機能が守ること

v3 §3.2 と同じ。以下を追記。

- **OGP画像生成に失敗しても、フォトブックの公開自体は成功させる**。`photobook_ogp_images.status = failed or fallback` となり、既定のOGP画像を用いる（v3 既定方針を v4 で独立テーブル化して実装）

#### 付随する業務ルール

- 権利・配慮確認の同意日時は、後から確認できる形で記録される
- 公開範囲の既定値は「限定公開」
- センシティブ設定の既定値は「OFF」
- 管理URLは原則として固定であり、作成者自身による再発行機能はMVPでは提供しない（v3 §3.4 に準拠）

---

### 3.3 フォトブック閲覧機能

v3 §3.3 と同じ。以下のみ v4 で追加。

- **MVP では全フォトブックページに `noindex` を付与する**（公開・限定公開・非公開問わず、v4 §7.6 参照）
- **閲覧ページの Referrer-Policy は `strict-origin-when-cross-origin`**。**管理URLページは `no-referrer` を必須化**

---

### 3.4 フォトブック管理機能（管理URLページ）（v4 更新）

#### この機能が守ること（v4 追記）

- 管理URLページは以下を守る：
  - **外部画像を読まない**
  - **外部フォントを読まない（またはセルフホスト）**
  - **外部スクリプトを読まない**
  - **アクセス解析タグを入れない**
  - **エラートラッキング（Sentry 等）に URL 全文を送らない**
  - **X共有ボタンには公開URLのみを渡す**（管理URL混入禁止）
  - **`Referrer-Policy: no-referrer` を必須化**
- **破壊的操作には二重確認を入れる**：
  - 削除
  - 公開範囲変更（公開 → 限定公開 → 非公開 の変更）
  - 管理URL再発行（運営ルート経由）
  - センシティブ設定変更
- **GETで状態変更しない**。POST/PUT/PATCH/DELETEのみ
- **CORSは自サイトに限定**

#### 付随する業務ルール（v3 §3.4 維持、v4 で Slug 復元ルールを明記）

v3 §3.4 の削除・管理URL再発行ルールを維持。v4 で以下を明文化：

**Slug 復元ルール（v4 新規、第12節）**:

- `published`: `public_url_slug` は有効
- `deleted`: `public_url_slug` は保持され、他のPhotobookに再利用されない。保持期間内は運営判断で `restore` 可能（同じslugで復元）
- `purged`: 物理削除済み。`public_url_slug` は解放され再利用可能。**restore 不可**
- `restore` / `soft_delete` / `purge` はすべて `ModerationAction` として追記記録される
- 保持期間内であれば複数回 `restore` 可能

---

### 3.5 管理URL控え機能

v3 §3.5 維持。v4 で以下を追加。

- **メール送信プロバイダ選定時に、送信履歴UIや API ログに管理URL本文が残るかを確認する**（v4 §9.3 参照）
- **ManageUrlDelivery の `manage_url_token_version` は `manage_url_token_version_at_send` にリネームする**（v4 P0-6）

---

### 3.6 通報機能（v4 更新）

#### この機能が担うこと

v3 §3.6 に加え、v4 では以下を追加。

- **通報理由カテゴリに `minor_safety_concern` を追加**（未成年を連想させる不適切表現、年齢・センシティブに関する問題の優先対応）
- **通報レコードには、通報対象Photobookの `public_url_slug` / `title` / `creator_display_name` のスナップショットを保持する**（Photobook物理削除後も通報証跡を残すため）

#### 通報理由カテゴリ（v4）

| 値 | 表示名 | 想定ケース |
|----|-------|----------|
| `subject_removal_request` | 被写体として削除希望 | 写っている本人からの削除依頼 |
| `unauthorized_repost` | 無断転載の可能性 | 他人の写真が無断で掲載されている |
| `sensitive_flag_missing` | センシティブ設定の不足 | センシティブ申告なしで不適切表現 |
| `harassment_or_doxxing` | 嫌がらせ・晒し | 個人攻撃、晒し |
| **`minor_safety_concern`** | **年齢・センシティブに関する問題** | **未成年を連想させる性的表現、その他未成年保護関連**（v4 新規） |
| `other` | その他 | 上記以外 |

#### 付随する業務ルール

v3 §3.6 付随ルール維持。v4 で以下を明文化：

- **`minor_safety_concern` は `other` よりも通知レベルを上げ、優先対応する**
- **`reports.target_photobook_id` に DB 外部キー制約は付けない**（Photobook 物理削除後も監査証跡として残すため）
- **通報対象の `public_url_slug` / `title` / `creator_display_name` をスナップショットとして保持**

---

### 3.7 荒らし対策機能（利用制限）

v3 §3.7 維持。v4 では以下を明文化。

- **画像アップロード時の安全検証**：
  - 拡張子ではなく MIME / マジックナンバー / 実デコードで形式判定
  - SVG は MVP で禁止
  - アニメーション WebP / APNG は MVP で拒否
  - 最大長辺 8192px、最大ピクセル数 40MP 程度
  - デコンプレッションボム対策
- **UsageLimit と Report の IP ハッシュは同じソルトポリシーを共有する**（同一作成元からの大量投稿・大量通報の相関検出のため）
- **ハッシュソルトはバージョン番号を持ち、ローテーション時は長期追跡性が失われることを許容する**

---

### 3.8 X共有支援機能

v3 §3.8 維持。

---

### 3.9 ランディング機能

v3 §3.9 維持。

---

### 3.10 画像管理機能（v4 更新）

#### この機能が担うこと

v3 §3.10 に加え、v4 では以下を明示する。

- **画像は `owner_photobook_id` により特定の Photobook に所有される**（`reference_count` 方式は廃止）
- **画像の用途（`usage_kind`）を明示する**：`photo` / `cover` / `ogp`
  （Phase 2 以降で `creator_avatar` を追加する可能性あり）
- **画像処理失敗時に `failed` 状態に遷移する**（`failed_at`, `failure_reason` を記録）

#### この機能が守ること（v4 追記）

- **EXIF / XMP / IPTC 等の埋め込みメタデータは、公開配信用画像から原則全除去する**
  - GPS位置情報、デバイスシリアル、PCユーザー名、ローカルファイルパス、撮影日時などの個人特定情報
  - Author / Artist / Photographer / Copyright 等のクレジット情報も**画像メタデータとしては残さない**
- **クレジット表示が必要な場合は、Photobook のメタ情報として明示入力させる**（誤って PC ユーザー名や編集ソフト由来の文字列が公開されるリスクを防ぐため）
- **正規化形式は JPG または WebP**。PNG は入力として受け付け、透過がある場合 WebP、透過がない場合 JPG または WebP に変換する
- **SVG は禁止**、**アニメーションWebP / APNG は拒否**

#### 画像の所有と削除連鎖

```
Photobook (draft または published)
  │
  │  owner_photobook_id
  ▼
Image (usage_kind = photo / cover / ogp)

Photobook 物理削除（purge）
  ↓
owner_photobook_id が一致する Image を削除対象に
  ↓
Image 物理削除 + CDNキャッシュパージ
```

#### 付随する業務ルール（v4 新規含む）

- **1画像が複数Photobookから共有されることはMVPでは起きない想定**
- **draft状態のPhotobookに所属するImageは、draft削除時に連鎖削除される**
- **Imageのステータスが `available` 以外のとき、公開中のPhotobookに紐づけてはならない**

---

## 第4部 フォトブックタイプ別の業務ルール

v3 第4部と同じ。内部タイプ7種（event/daily/portfolio/avatar/world/memory/free）、既定値と選択肢、タイプ別の作り込み深度。

**v4 で追加**: 表紙（cover）は `photobooks` にインライン化されるが、タイプ別の表紙表示有無は `OpeningStyle` で引き続き制御される。

---

## 第5部 業務上の境界（MVPスコープ）

### 5.1 MVPで提供する機能

v3 §5.1 の10機能。v4 では内部構造として以下を追加：

- **Outbox（集約間イベントのトランザクション保証）**
- **OGP生成（独立管理）**
- **Reconcile スクリプト群（運営向け）**

### 5.2 MVPで提供しない機能（v4 更新）

v3 §5.2 に加え、v4 で以下を MVP スコープ外に明示：

- **Creator Avatar（作成者アイコン）画像アップロード** — Phase 2 以降
- **表紙の複数画像レイアウト・専用テンプレート** — Phase 2 以降
- **検索エンジンへのインデックス（noindex 方針）** — Phase 2 以降で作成者明示ONなら許可検討

### 5.3 MVP境界の判断理由

v3 §5.3 維持。

### 5.4 運営対応の最低限手段（v4 更新）

v3 §5.4 維持。v4 で以下を追加：

- **運営操作は `ModerationActionExecutor` 経由で、Photobook状態・ModerationAction記録・Report状態・Outbox を同一トランザクションで更新する**
- **DB直接操作ではなく、運営スクリプト（`scripts/ops/`）経由を推奨**
- **Reconcile スクリプト（`scripts/ops/reconcile/`）は MVP から用意する**

### 5.5 将来の拡張方針

v3 §5.5 維持。

---

## 第6部 横断的な業務ルール

v3 第6部の §6.1〜§6.10 を維持。v4 で以下を追加。

### 6.11 集約間イベントは Outbox で伝搬する（v4 新規）

- 集約の状態変更と副作用（OGP生成、メール送信、CDNパージ、画像物理削除等）は**同一トランザクション内で `outbox_events` にINSERT**し、非同期ワーカーで実行する
- 失敗時は retry し、上限超過で `failed` となり Reconcile 対象になる

### 6.12 整合性は Reconcile で保証する（v4 新規）

- 画像参照、OGP状態、Outbox失敗、Draft期限切れ、CDNキャッシュ等の不整合は、定期的な Reconcile スクリプトで検出・修復する
- すべての Reconcile スクリプトは `--dry-run` を必須とする

### 6.13 管理URLと draft 編集URL の漏洩対策は同等（v4 新規）

- `draft_edit_token` は公開前のフォトブックの編集権を握るため、`manage_url_token` と同等の漏洩対策を適用する
- `Referrer-Policy: no-referrer`、外部リソース読み込み禁止、エラートラッキングへのURL送信禁止

### 6.14 画像の所有と削除連鎖（v4 新規）

- 画像は `owner_photobook_id` により特定 Photobook に所有される（`reference_count` 方式は MVP では採用しない）
- Photobook 物理削除時、所有 Image は連鎖削除される
- draft 期限切れ時、draft 所属 Image も連鎖削除される

### 6.15 楽観ロックによる同時編集事故防止（v4 新規）

- Photobook に `version`（または `lock_version`）を持ち、更新時にインクリメント
- 管理URL が複数人に共有された場合や、複数タブでの編集時、後勝ち更新による事故を防ぐ
- MVP では version 不一致でエラーを返す（マージ処理はしない）

---

## 第7部 規約・プライバシー・SEO に関する業務前提

v3 第7部の §7.1〜§7.5 を維持。v4 で §7.6 を追加。

### 7.6 SEO / robots 方針（v4 新規）

MVPでは、検索エンジンへの露出リスクを最小化する。

- **全フォトブックページに `noindex` を付与**（public / unlisted / private すべて）
- **draft 編集URL ページと管理URLページにも `noindex`**
- **sitemap は生成しない**
- **robots.txt で `/manage/` と `/draft/` を Disallow**（ただし robots.txt だけに頼らず、ページ側にも `noindex` を入れる）

**Phase 2 以降**: 作成者が「検索エンジンに表示する」を明示的にONにした場合のみ index 許可を検討する。

**理由**: VRC 写真は、被写体・アバター・イベント参加者・ワールド情報が絡みやすく、最初から検索エンジンに露出させるリスクが高い。

---

## 付録A: 業務知識とドメインモデル設計の関係

v3 付録A 維持。

## 付録B: バージョン間の主な改訂点

### v3 → v4

| 項目 | v3 | v4 |
|------|----|----|
| 画像参照モデル | `reference_count`（参照カウント） | `owner_photobook_id` + `usage_kind`（所有モデル） |
| Image 状態 | uploading / processing / available / deleted / purged | 左記 + `failed`（処理失敗状態）を追加 |
| 画像安全検証 | 記載なし | MIME/マジックナンバー/実デコード、SVG禁止、アニメーション拒否、最大長辺/ピクセル数、ボム対策を明記 |
| EXIF / XMP / IPTC | EXIFのみ「機密性の高いものは除去」 | **全メタデータを原則除去**。Author/Artist もクレジットは Photobook メタ情報として明示入力 |
| 正規化形式 | 記載なし | JPG または WebP。PNG の扱いを明記 |
| 作成途中の保持 | ブラウザローカル + 必要時再アップロード | **server draft + `draft_edit_token`**（初回画像アップロードでdraft作成、7日延長式） |
| OGP生成管理 | Photobook上の暗黙状態 | **`photobook_ogp_images` による独立管理**（pending/generated/failed/fallback/stale） |
| Cover | `photobook_covers` 別テーブル | **MVPは `photobooks` にインライン化** |
| Creator Avatar | MVPに含む | **MVP除外、Phase 2 へ** |
| 楽観ロック | 記載なし | `version` / `lock_version` を Photobook に追加 |
| 通報理由カテゴリ | 5種（subject_removal, unauthorized_repost, sensitive_flag_missing, harassment, other） | 6種（上記 + **`minor_safety_concern`**） |
| 通報のPhotobook参照 | FK 方針は要検討 | **FKなし + snapshot（slug/title/display_name）保持** |
| 運営対応トランザクション | Photobook と ModerationAction の原子性 | **Photobook + ModerationAction + Report + Outbox を同一トランザクション** |
| 集約間イベント | 明示的な仕組みなし | **Transactional Outbox を MVP から採用** |
| 整合性保証 | 記載なし | **Reconcile スクリプト群をMVPから用意** |
| ManageUrlDelivery のtoken version | `manage_url_token_version` | **`manage_url_token_version_at_send`**（意味明示化） |
| 管理URL 漏洩対策 | 秘匿方針のみ | **`Referrer-Policy: no-referrer` 必須、外部リソース禁止、破壊的操作の二重確認** |
| Slug 復元ルール | 不明瞭 | **deleted内は同slug維持・再利用不可、purge後は解放・restore不可** |
| SEO / robots | 記載なし | **MVPは全noindex、/manage/と/draft/をDisallow** |
| IPハッシュソルト | UsageLimit内でのみ管理 | **UsageLimit と Report で共有、version管理** |

### v2 → v3

v3 付録B 参照。

### v1 → v2

v3 付録B 参照。
