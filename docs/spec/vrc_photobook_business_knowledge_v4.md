# VRC PhotoBook 業務知識定義書 v4

> 本ドキュメントは、VRC PhotoBook を構成する各機能の**責任範囲と業務としての振る舞い**を定義する。
> クラス設計・集約・識別子・データベース等の具体構造は、本書を前提として別途ドメインモデル設計書・データモデル設計書で扱う。技術的な実装判断は `docs/adr/` 配下の ADR 群で管理する。
>
> 本書の目的は、実装者・企画・ドメインエキスパートの間で「この機能は何をするものか」の理解を一致させることにある。v4 以降、本書はドメイン設計・データモデル設計・M1 スパイクの**正の参照元**として扱う。
>
> **v3 からの v4 改訂の主題**（付録B 参照）:
> 1. 画像の所有モデルを `owner_photobook_id` 方式に切り替え
> 2. `server draft + draft_edit_token` の採用
> 3. OGP 生成の独立管理（`photobook_ogp_images`）
> 4. Transactional Outbox の MVP 採用
> 5. 管理 URL 漏洩対策の強化
> 6. Reconcile による整合性保証
> 7. 未成年関連通報カテゴリの独立（`minor_safety_concern`）
> 8. **token→session 交換方式の採用**（ADR-0003）
> 9. **運営操作の `scripts/ops` + `cmd/ops` 方式**（ADR-0002）
> 10. **画像アップロードの R2 presigned URL 方式**（ADR-0005）
>
> **参照する ADR**:
>
> - ADR-0001: 技術スタック（Accepted）
> - ADR-0002: 運営操作方式（Accepted）
> - ADR-0003: フロントエンド認可フロー（Accepted）
> - ADR-0004: メールプロバイダ選定（Proposed）
> - ADR-0005: 画像アップロード方式（Accepted）

---

## 第1部 サービス全体の前提

### 1.1 サービスの定義

VRC PhotoBook は、VRChat で撮影した写真を「フォトブック」という一貫した形式にまとめ、Web で公開し X で共有するためのサービスである。

単なる画像保存サービスではなく、X で共有されたリンクを踏んだ第三者が「自分も作りたい」と感じる体験を提供することを目的とする。

### 1.2 根本ポリシー

本サービスは以下の根本ポリシーのもとに設計される。

- **ログイン不要で完結する**。作成・公開・閲覧・編集・削除はログインなしで行える
- **ログインは任意**。複数フォトブックの横断管理や有料機能を使うための手段である
- **編集・削除手段は作成者に帰属する**。ログインなしでも編集・削除できる仕組み（管理URL方式）を提供する
- **スマホファースト**。X から流入する閲覧者の多くはスマートフォンを利用する
- **写真が主役**。UI や演出は写真の魅力を妨げてはならない
- **SNS 化しない**。フォロー、いいね、コメント等の機能を安易に持ち込まない
- **最低限の安全性を持つ**。荒らし、無断転載、センシティブ投稿に対する最低限の防御と対応手段を持つ

### 1.3 想定される利用シーン

本サービスは VRChat コミュニティ全般を対象とするが、特に以下のシーンで使われることを想定する。

- イベント記録（VRC バー営業、コンカフェ営業、撮影会、コミュニティイベント）
- ワールド記録（訪れたワールドの紹介、世界観の共有）
- フレンドとの思い出の記録
- 日常ログ（おはツイ、日々の活動）
- 作品集（フォトグラファーによる作品発表）
- アバター紹介（アバター改変・衣装紹介）

### 1.4 登場するアクター

| アクター | 定義 |
|---|---|
| 作成者 | フォトブックを作成する人。ログイン有無を問わない |
| 閲覧者 | 公開されたフォトブックを閲覧する人。ログイン有無を問わない |
| 運営 | 通報対応や不適切コンテンツの削除、管理 URL の失効判断等を行う。`cmd/ops` CLI 経由で操作する（ADR-0002） |
| 任意ユーザー | 複数フォトブックの横断管理や有料機能を使うために任意で登録した作成者。MVP では扱わない（Phase 2 以降） |

---

## 第2部 ユビキタス言語（共通語彙）

本サービスに関わる全ての会話・ドキュメント・コードで揺らぎなく使用する用語を定義する。

### 2.1 フォトブックの構造に関する用語

| 用語 | 定義 |
|---|---|
| フォトブック | 複数のページで構成される、公開・共有可能な 1 つの単位。本サービスの中核概念。「アルバム」「まとめ」等の表現は使わない |
| ページ | フォトブック内の 1 つの表示単位。**MVP では 1 ページ = 1 写真を基本とする**（将来拡張で複数写真を許容） |
| 写真 | フォトブックを構成する画像ファイル |
| キャプション | ページまたは写真ごとに付く短い説明文 |
| メタ情報 | ページに任意で付く構造化情報。World ワールド名、Cast キャスト、Photographer 撮影者、Note 補足メモ の 4 種を MVP で扱う |
| 表紙 | フォトブックの先頭に置かれる、タイトルと代表画像からなる特別なページ。**MVP では `photobooks` テーブルにインライン化する**（v4 で確定） |
| 作成者情報 | フォトブックに紐付く作成者の表示情報。**MVP では表示名と任意の X ID のみ**（creator avatar 画像は Phase 2 以降） |

### 2.2 UI 上の表示要素に関する用語

ドメイン概念（§2.1）と UI 表示概念を分離する。UI 表示要素はドメイン語彙ではない。

| 用語 | 定義 |
|---|---|
| フォトカード | カード型レイアウトで表示される、1 枚の写真を囲む UI 表示要素。**ドメイン概念ではなく UI 上の表示単位**。ページの見せ方の一種として用いられる |
| バナー / 帯 | タイトルやイベント名を示す横長の装飾要素（UI 概念） |
| 作る CTA | 閲覧ページ末尾に配置する「自分もフォトブックを作る」への誘導ボタン（UI 概念） |

### 2.3 URL とトークン / セッションに関する用語

ログイン不要を実現する中核概念のため、揺らぎは特に禁止する。ここでの token と session は**明確に別概念**として扱う。

| 用語 | 定義 |
|---|---|
| 公開 URL | 閲覧者に共有される URL。X に貼るのはこの URL |
| `public_url_slug` | 公開 URL 中の識別子。publish 成功時に発行される。推測困難な短文字列 |
| 管理 URL | 作成者のみが保持する、publish 後の長期の再入場手段。token を含む |
| `manage_url_token` | 管理 URL 中の raw token。**DB には hash のみを保存する**（平文保持禁止） |
| `manage_url_token_version` | 管理 URL トークン世代番号。**初回 `manage_url_token` 発行時は 0**、運営による再発行のたびに +1 される（初回再発行後 1、2回目再発行後 2…）。draft 中は `manage_url_token` が未発行のため version も利用されない |
| draft 編集 URL | 作成途中（server draft）のフォトブックにアクセスするための URL。token を含む |
| `draft_edit_token` | draft 編集 URL 中の raw token。**DB には hash のみを保存する**（平文保持禁止）。publish 成功で失効 |
| `draft_expires_at` | draft の有効期限。初期値 now+7 日、編集系操作成功時に now+7 日へ延長 |
| 管理 URL 控え | 作成者が管理 URL を紛失しないための手段（コピー、メール送信、端末保存）の総称 |
| session | token を検証後に発行される、短命の操作セッション。HttpOnly Cookie で保持される（詳細は §2.4） |
| session token | Cookie に保持される raw 値。256bit 以上の暗号論的乱数を base64url 化したもの。DB には `session_token_hash` のみ保存 |
| `token_version_at_issue` | session 発行時点の `manage_url_token_version` の snapshot。再発行時の一括失効に用いる |
| ワンタイム確認トークン | 破壊的操作（削除・公開範囲変更・管理 URL 再発行・センシティブ変更等）に要求される単一用途の追加トークン。5〜10 分で失効 |

**URL 設計**（ADR-0003 で確定）:

| 役割 | URL |
|------|-----|
| Draft 入場（token 消費） | `/draft/{draft_edit_token}` |
| Draft 編集（session 認可） | `/edit/{photobook_id}` |
| Manage 入場（token 消費） | `/manage/token/{manage_url_token}` |
| Manage 管理（session 認可） | `/manage/{photobook_id}` |

**URL と session の役割分離**:

- raw token は初回 URL アクセス時のみ URL に含める
- token 検証後、短命 session を発行し HttpOnly Cookie に保存する
- redirect で URL から raw token を除去する
- API 呼び出しでは raw token を query string / path / body に含めない

### 2.4 セッションと入場フローに関する用語

| 用語 | 定義 |
|---|---|
| draft session | draft 編集用の短命 session。`/draft/{token}` アクセス時に発行され、`/edit/{photobook_id}` へ redirect。Cookie 名 `vrcpb_draft_{photobook_id}`。期限は `draft_expires_at` まで、最大 7 日 |
| manage session | 管理用の短命 session。`/manage/token/{token}` アクセス時に発行され、`/manage/{photobook_id}` へ redirect。Cookie 名 `vrcpb_manage_{photobook_id}`。期限 24 時間〜7 日程度（長期化しない） |
| upload verification session | 画像アップロードの Turnstile 検証結果を短期保持する session。Photobook に紐づく。期限 30 分、1 検証あたり最大 20 回の upload-intent を許可 |
| session revoke | session を明示的に失効させる操作。`revoked_at` を設定し Cookie を削除する。管理 URL 自体は失効させない |
| 明示破棄 UI | 編集画面・管理画面に置く「この端末の編集/管理権限を削除」ボタン。共有 PC 対策 |

**Cookie 属性**（共通、ADR-0003）:

- `HttpOnly: true`
- `Secure: true`
- `SameSite: Strict`
- `Path: /`

### 2.5 フォトブックタイプと表示に関する用語

タイプ分類は**内部タイプ**と**表示タイプ名**の 2 層で扱う。内部タイプは実装・業務ルールで用い、表示タイプ名は作成者に提示する言葉として用いる。

| 内部タイプ | 表示タイプ名 | 説明 |
|---|---|---|
| event | イベント | VRC バー、撮影会、コミュニティイベント |
| daily | おはツイ | 日々の活動、おはツイ、気分の記録 |
| portfolio | 作品集 | フォトグラファーの作品発表 |
| avatar | アバター紹介 | アバター改変、衣装紹介 |
| world | ワールド | ワールド紹介、世界観の共有 |
| memory | 思い出 | フレンドとの思い出、特定の集まりの記録 |
| free | 自由 | 上記に当てはまらない自由な用途 |

| 用語 | 定義 |
|---|---|
| フォトブックタイプ | フォトブックの用途分類。内部タイプで識別される |
| テンプレート | タイプごとに用意される見た目と構造のプリセット |
| レイアウト | 閲覧時の写真の並べ方。「シンプル」「雑誌」「カード」「大判」の 4 種 |
| 開き方 | 閲覧開始時の表示スタイル。「軽め」または「表紙ファーストビュー」の 2 種 |

### 2.6 公開とモラルに関する用語

| 用語 | 定義 |
|---|---|
| 公開範囲 | フォトブックの可視性。「公開」「限定公開」「非公開」の 3 値 |
| 公開（全体に公開） | 公開 URL で誰でも閲覧可能 |
| 限定公開 | 公開 URL を知っている人のみ閲覧可能。一覧には載らない。**MVP の既定値** |
| 非公開 | 公開 URL による閲覧は無効となり、**管理 URL を保持している人のみ**が閲覧できる状態。「自分だけ」という表現は用いない |
| センシティブ設定 | フォトブックがセンシティブな表現を含むかを作成者が申告するフラグ。閲覧前にワンクッションを表示する |
| 権利・配慮確認 | 公開前に作成者が必須で同意するチェック。被写体・アバター・ワールド等への配慮を確認する |
| 一時非表示 | 通報等を受けて運営が一時的に公開を停止する状態。作成者の意思とは独立して行われる |

### 2.7 荒らし対策に関する用語

| 用語 | 定義 |
|---|---|
| 利用制限 | ログイン不要環境における荒らし抑止のための各種制限。画像枚数・サイズ・作成レート等 |
| 作成レート制限 | 一定時間内に同一の作成元から作成できるフォトブック数の上限 |
| bot 検証 | 公開操作前・画像アップロード前に実行する自動投稿防止のための検証。本サービスでは **Turnstile** を採用 |

### 2.8 画像アップロードに関する用語（v4 新規）

画像アップロードは API サーバー multipart 直送ではなく、R2 presigned URL 方式を採用する（ADR-0005）。

| 用語 | 定義 |
|---|---|
| upload-intent | 画像アップロード意図の表明 API。Turnstile / RateLimit / 軽量バリデーションを通過した後に Image レコード作成と R2 presigned URL 発行を行う |
| complete | アップロード完了通知 API。R2 オブジェクトの存在確認、Image/Photobook 整合性確認、状態確認を経て `outbox_events` に `ImageIngestionRequested` を INSERT する |
| presigned URL | R2 への直接 PUT 用の期限付き署名付き URL。MVP では有効期限 15 分 |
| storage_key | R2 上の画像オブジェクトパス。`photobooks/{photobook_id}/images/{image_id}/{variant}/{random}.{ext}` 形式 |
| image-processor | `ImageIngestionRequested` を購読する非同期ワーカー。マジックナンバー検証・実デコード・EXIF 除去・HEIC 変換・variant 生成を担当 |
| `usage_kind` | Image の用途。`photo` / `cover` / `ogp` のいずれか（`creator_avatar` は MVP 非対応） |
| `failure_reason` | 画像処理失敗時の分類キー。§3.10 に語彙を列挙 |

### 2.9 集約間イベント用語

集約間の副作用伝搬は Outbox パターンで行う（ADR-0001、§6.11）。

| 用語 | 定義 |
|---|---|
| Outbox | 集約の状態変更と副作用（OGP 生成、メール送信、CDN パージ、画像処理等）の実行をトランザクション的に保証するためのイベント記録テーブル |
| Reconcile | Outbox 失敗・状態不整合・孤児データを検出し、手動または自動で修復する運用プロセス |

**Outbox 対象イベント**（§6.11）:

- `PhotobookPublished`
- `PhotobookUpdated`
- `PhotobookHidden`
- `PhotobookUnhidden`
- `PhotobookSoftDeleted`
- `PhotobookRestored`
- `PhotobookPurged`
- `ManageUrlReissued`
- `ReportSubmitted`
- `ManageUrlDeliveryRequested`
- `ImageIngestionRequested`

### 2.10 運営操作用語（v4 新規、ADR-0002）

| 用語 | 定義 |
|---|---|
| `cmd/ops` | 運営操作用の Go CLI 単一バイナリ。サブコマンドで hide / unhide / restore / purge / reissue_manage_url / resolve_report / reconcile 系を実行する |
| `scripts/ops/*.sh` | `cmd/ops` への薄い Shell ラッパー。環境切替・Secret 注入・必須フラグ付与を担う |
| operator | 運営操作の実行者識別子。個人情報を含まない運営内識別子。`ModerationAction.actor_label` に記録される |
| `ModerationAction` | 運営操作の監査記録。immutable、追記のみ。`hide/unhide/soft_delete/restore/purge/reissue_manage_url` の 6 種 |
| `--dry-run` | 破壊的操作の既定モード。DB に書き込まない。参照系では受け付けない |
| `--execute` | 破壊的操作を実際に実行するための明示フラグ |

**operator 識別子の形式**:

- 許可例: `ops-kento`, `ops-001`, `legal-team`, `support-01`, `security.lead`
- 禁止: 個人メールアドレス、本名フルネーム、電話番号、外部 SNS ID そのもの
- 正規表現: `^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}[a-zA-Z0-9]$`（3〜64 文字、英数字 / ハイフン / アンダースコア / ドット、先頭末尾は英数字）

### 2.11 非採用用語

以下は過去に検討されたが採用しない用語。今後の議論で揺らがないよう明記する。

| 非採用用語 | 理由 |
|---|---|
| アルバム | 「フォトブック」に統一する |
| マイページ | MVP では管理 URL 方式のため「管理 URL ページ」を用いる |
| 作者（Author） | ログイン不要方針により「作成者（Creator）」に統一 |
| 見開きブック型レイアウト | スマホファースト方針により非採用 |
| メンバー / アカウント（基本機能における用途） | MVP では基本機能でログインを前提としないため、これらの語は基本機能の文脈で使わない |
| Comment（メタ情報フィールドとして） | SNS 機能の「コメント」と混同しやすいため、メタ情報フィールド名としては採用せず「Note」を用いる |
| `rejected`（ImageStatus として） | **MVP では使わない**。不正形式・サイズ超過・SVG・アニメーション画像・デコード失敗はすべて `failed` に集約し、`failure_reason` で理由を区別する |
| `reference_count`（画像参照モデル） | **MVP では採用しない**。`owner_photobook_id` 方式に統一 |

---

## 第3部 機能定義

MVP スコープで必要な機能を、責任範囲・振る舞い・守るべきことの観点から定義する。

### 3.1 フォトブック作成機能

#### 責任範囲

作成者が VRChat で撮影した写真をまとめて、公開可能な形に整える一連の体験を提供する。

#### この機能が担うこと

- フォトブックの新規作成を受け付ける
- フォトブックタイプの選択を受け付ける
- タイプに応じたテンプレートを提示し選択を受け付ける
- 写真のアップロードを受け付ける（upload-intent / complete の 2 段 API、§2.8）
- ページの追加・削除・並び替えを受け付ける
- ページごとのキャプションとメタ情報の入力を受け付ける
- タイトルと説明文の入力を受け付ける
- レイアウトと開き方の選択・変更を受け付ける（タイプごとの既定値を初期値として提示する）
- 公開操作前までログイン・アカウント作成を一切要求しない
- **初回画像アップロード時に server draft Photobook を作成し、`draft_edit_token` を発行する**
- **draft 編集 URL（`/draft/{draft_edit_token}`）を作成者に提示する**
- **draft アクセス時に token を session に交換し、以降は Cookie session で認可する**

#### この機能が守ること

- 作成者は一切のログインなしに、フォトブックの中身を作り上げることができる
- 作成者がタイプを選んだ時点で、そのタイプに最適な既定レイアウト・既定の開き方が自動で設定される
- 既定の選択肢だけで、それなりに見栄えのするフォトブックが成立する
- **draft 編集 URL の漏洩は管理 URL と同等のリスクを持つため、管理 URL と同じ漏洩対策を適用する**（§6.13）
- **ブラウザローカル保持に依存しない**。画像のアップロードで server draft 化され、端末依存性は軽減される
- 画像本体がブラウザ側で復元できない場合は、作成者に再アップロードを求める

#### draft ライフサイクル

```
作成開始（クライアント）
  ↓
初回画像 upload-intent
  ↓
server draft Photobook 作成（status='draft', public_url_slug/manage_url_token は未発行）
  ↓
draft_edit_token 発行、draft_expires_at = now + 7日
  ↓
draft 編集URL `/draft/{draft_edit_token}` を提示
  ↓
初回アクセスで token を draft session に交換、Cookie 発行、/edit/{photobook_id} へ redirect
  ↓
編集系API成功ごとに draft_expires_at = now + 7日に延長
  ↓
（公開前まで）
  ↓
publish 成功
  ↓
public_url_slug / manage_url_token 発行、draft_edit_token 失効、draft session 全 revoke
  ↓
以後は管理 URL のみ有効
```

#### draft 延長ルール

`draft_expires_at` は**編集系 API 成功時のみ** `now + 7日` に延長する。GET やプレビュー閲覧では延長しない。放置された draft は自動的に期限切れとなり Reconcile で GC される。

**編集系操作の例**: 内容更新 / 写真追加 / 写真削除 / ページ追加 / ページ削除 / 並び替え / メタ情報更新 / 公開設定保存。

#### 画像アップロードフロー

画像アップロードは API サーバー multipart 直送を使わず、R2 presigned URL 方式で行う（ADR-0005）。

1. フロントが `upload-intent` を呼ぶ（Photobook ID + 画像メタ申告）
2. サーバーが Turnstile / RateLimit / 軽量バリデーションを通過させる
3. Image レコード作成（`status=uploading`, `owner_photobook_id` 固定, `usage_kind=photo`）
4. R2 presigned URL（PUT 用、15 分有効）を返す
5. フロントが R2 に直接 PUT
6. フロントが `complete` を呼ぶ
7. サーバーが R2 オブジェクト存在確認・整合性検証・`ImageIngestionRequested` INSERT（Image 更新と同一 TX）
8. image-processor が非同期で本検証・変換・EXIF 除去・variant 生成を実行

**Turnstile はセッション化して UX を確保する**。1 検証で 30 分以内 / 20 回までの upload-intent を許可（§2.8、ADR-0005）。

#### この機能が担わないこと

- フォトブックの公開や永続化（§3.2 の責任）
- 作成者のアカウント管理
- AI による画像加工や自動生成
- 画像ファイルそのものの保存形式選択や形式変換（§3.10 の責任）

#### 付随する業務ルール

- 作成途中の状態では `public_url_slug` / `manage_url_token` は発行されない
- draft 状態の Photobook に紐づく Image は `owner_photobook_id` でこの draft を指す
- `draft_expires_at` を過ぎた draft は削除対象となる（自動 reconciler の `draft_expired`）
- タイプを変更した場合、既定レイアウトと既定の開き方が新しいタイプのものに切り替わる
- **`rights_agreed` は本機能（draft 作成・編集）では取得しない**。権利・配慮確認の同意は §3.2 公開機能の責任範囲であり、**publish 操作と同一トランザクション内で取得・保存**する（`status='draft'→'published'` 遷移と同 TX で `rights_agreed=true` を確定。途中保存は不可）。
  - 同 TX 化により「同意したが publish 失敗 → 次回 publish で再同意せず通過」「未同意で publish 成立」のいずれもが防止される
  - 実装は `9c4fb7d` で確定（Backend handler / Frontend `PublishSettingsPanel` / OCC 規約）

---

### 3.2 フォトブック公開機能

#### 責任範囲

draft Photobook を実際に Web 上で閲覧可能な状態にする最終工程を担う。作成者の意思確認と、最低限の安全性確保がここに集約される。

#### この機能が担うこと

- 作成者が入力した公開範囲（公開・限定公開・非公開）を受け付ける
- センシティブ設定の有無を受け付ける
- 権利・配慮確認のチェックを必須で受け付ける
- 作成者情報（表示名・任意で X ID）の入力を受け付ける
- bot 検証（Turnstile）を実行する
- 作成レート制限を確認する
- フォトブックの内容を永続化する（`status='draft'` → `status='published'` に遷移）
- **Photobook ID は変わらない**（Draft と Published は同一レコード）
- **`Image.owner_photobook_id` は publish 時も変更しない**（所有モデル、§6.14）
- `public_url_slug` を発行する
- `manage_url_token` を発行する（`manage_url_token_version = 0` で開始。draft 中は `manage_url_token` が存在しないため version も利用されない。初回再発行で 1、2回目再発行で 2 …とインクリメントされる）
- `draft_edit_token` を失効させる
- 対象 Photobook の draft session をすべて revoke する（ADR-0003）
- `outbox_events` に `PhotobookPublished` を同一トランザクションで INSERT する
- 閲覧時に表示される OGP 画像を非同期で生成する（§6.12、独立管理）
- X で共有するための投稿文テンプレートを提示する

#### この機能が守ること

- 権利・配慮確認にチェックが入っていないフォトブックは公開されない
- 作成者名が未入力のフォトブックは公開されない
- 公開操作が成功した場合、必ず公開 URL と管理 URL の両方が作成者に提示される
- 管理 URL は推測困難な長さとランダム性を持ち、公開 URL や他の公開情報からは推測できない
- bot 検証・作成レート制限を通過しなかった公開操作は拒否される
- 公開範囲として「限定公開」が既定で選ばれており、作成者が明示的に変更しない限り全世界公開にはならない
- **OGP 画像生成に失敗しても、フォトブックの公開自体は成功させる**。`photobook_ogp_images.status = failed` または `fallback` となり、既定の OGP 画像を用いる

#### この機能が担わないこと

- 公開後の再編集や設定変更（§3.4 の責任）
- 閲覧時の可視性判定そのもの（§3.3 の責任）
- 画像本体のアップロード・保存・形式変換（§3.10 の責任）

#### 付随する業務ルール

- 権利・配慮確認の同意日時は、後から確認できる形で記録される
- 公開範囲の既定値は「限定公開」
- センシティブ設定の既定値は「OFF」
- 管理 URL は原則として固定であり、**作成者自身による再発行機能は MVP では提供しない**（§3.4 参照。漏洩疑義時は運営経由で `cmd/ops/photobook_reissue_manage_url` 実行）

---

### 3.3 フォトブック閲覧機能

#### 責任範囲

公開 URL を知っている閲覧者がフォトブックを快適に見て、共有したり自分も作りたくなる体験を提供する。

#### この機能が担うこと

- 公開 URL へのアクセスを受け付ける
- 公開範囲に応じた可視性を判定する
- 一時非表示状態のフォトブックへのアクセスを適切に扱う
- センシティブ設定が ON の場合、閲覧前にワンクッションを表示する
- 表紙・タイトル・説明・ページ群・作成者情報を閲覧者に提示する
- タイプとレイアウトに応じた見せ方で写真を表示する
- X で共有するボタン、URL をコピーするボタンを提示する
- 閲覧者に対し「自分も作る」への導線（作る CTA）を提示する
- 問題を報告する導線を提示する

#### この機能が守ること

- 「非公開」のフォトブックは公開 URL からは閲覧できない。管理 URL を保持する人のみが閲覧できる
- 「限定公開」のフォトブックは公開 URL を知っている閲覧者のみが見られる。一覧や検索には出さない
- 運営により「一時非表示」になっているフォトブックは公開 URL 経由では閲覧できない。その旨を閲覧者に穏当に伝える
- センシティブ設定が ON のフォトブックは閲覧者が同意するまで本文を表示しない
- 閲覧者はログインなしで全ての機能（閲覧、共有、通報、自分も作る）を使える
- 閲覧ページに管理 URL が一切露出しない（OGP 画像、HTML、ログ、ブラウザ履歴等のあらゆる経路で）
- **MVP では全フォトブックページに `noindex` を付与する**（§7.6）
- **閲覧ページの Referrer-Policy は `strict-origin-when-cross-origin`**（§7.6）

#### この機能が担わないこと

- 閲覧履歴の記録や分析（MVP では対象外）
- 閲覧者による「いいね」「コメント」等の反応
- 作成者ごとのページ表示（作者ページは Phase 2）

#### 付随する業務ルール

- 閲覧時の初期表示は 3 秒以内を目標とする（スマホから流入するため）
- 作る CTA はページ末尾の、閲覧体験を阻害しない位置に配置される
- 作成者情報は `by 表示名 / @X ID` 程度の軽い表示に留める

---

### 3.4 フォトブック管理機能（管理 URL ページ）

#### 責任範囲

作成者が管理 URL を用いて、自分のフォトブックの編集・設定変更・削除を行えるようにする。ログイン不要を実現する要となる機能。

#### 管理 URL アクセスフロー（ADR-0003）

1. 作成者が `/manage/token/{manage_url_token}` にアクセス
2. サーバーが `manage_url_token` を hash 化し DB 照合して検証
3. 256bit の暗号論的乱数 `session_token` を生成、`session_token_hash` を DB 保存、`token_version_at_issue = Photobook.manage_url_token_version` を記録
4. `Set-Cookie: vrcpb_manage_{photobook_id}=<session_token>; HttpOnly; Secure; SameSite=Strict; Path=/`
5. `/manage/{photobook_id}` へ redirect（raw token を URL から除去）
6. 以後の API は Cookie session のみで認可

#### この機能が担うこと

- 管理 URL へのアクセスを受け付ける（上記フロー）
- 管理 URL が正当なものかをサーバー側で厳密に検証する
- 対象フォトブックの公開 URL と管理 URL の両方を表示する
- 管理 URL をコピーする操作を提供する
- フォトブックの内容編集（編集画面への導線）を提供する
- 公開範囲の変更を受け付ける
- センシティブ設定の変更を受け付ける
- フォトブックの削除を受け付ける
- X で共有するための投稿文を再生成・再コピーする操作を提供する
- 管理 URL 控えメール送信（ManageUrlDelivery）を受け付ける
- 「この端末から管理権限を削除」する明示破棄 UI を提供する
- 「管理 URL は他人に共有してはならない」旨の注意喚起を表示する

#### この機能が守ること

- 管理 URL を持たない者は、この画面にアクセスしても何もできない
- 管理 URL の検証はサーバー側で厳密に行う
- この画面が「管理モード」であることを、閲覧体験と明確に区別して視覚的に示す
- 削除操作は取り消せない、または取り消し期間が有限である旨を明示する
- 管理 URL 自体は、作成者以外に露出しない
- **raw token を Cookie に入れない**。Cookie には乱数 session token を入れ、DB には hash のみを保存
- 管理 URL ページは以下を守る:
  - 外部画像・外部フォント・外部スクリプトを読まない（§6.13）
  - アクセス解析タグを入れない
  - エラートラッキング（Sentry 等）に URL 全文を送らない
  - X 共有ボタンには公開 URL のみを渡す（管理 URL 混入禁止）
  - `Referrer-Policy: no-referrer` を必須化
  - 全ページ `noindex`（§7.6）
- **破壊的操作には二重確認 + ワンタイム確認トークンを要求する**:
  - 削除
  - 公開範囲変更
  - 管理 URL 再発行（運営ルート経由）
  - センシティブ設定変更
- **GET で状態変更しない**。POST/PUT/PATCH/DELETE のみ
- **CORS は自サイトに限定**、Origin ヘッダ検証必須

#### この機能が担わないこと

- 管理 URL を紛失した作成者への救済（MVP では運営問い合わせ対応のみ。運営は `cmd/ops/photobook_reissue_manage_url` で対応）
- 複数フォトブックの横断管理（Phase 2 の任意ユーザー機能）

#### 付随する業務ルール（管理 URL の失効と再発行）

- **管理 URL は原則として公開時に 1 度だけ発行され、以後固定される**
- **作成者自身による管理 URL の再発行機能は MVP では提供しない**
- **漏洩・不正利用が疑われる場合、運営は `cmd/ops/photobook_reissue_manage_url` により**:
  - `Photobook.manage_url_token_version` をインクリメント
  - 旧 version 由来の manage session を一括 revoke
  - 新しい `manage_url_token` を発行
  - ManageUrlDelivery を新規作成（過去の `recipient_email` は再利用しない）
  - `ModerationAction` を記録
  - `outbox_events` に `ManageUrlReissued` を INSERT
  - **上記をすべて同一トランザクション**で実行
- 上記処理は申請者の連絡先確認後に運営が手動実行する

#### 付随する業務ルール（削除の取り扱い、Slug 復元）

- 公開済みのフォトブックを編集して保存した場合、公開中の内容が即時更新される
- 削除は論理削除（soft_delete）を基本とする
- **論理削除後のフォトブックは、作成者・閲覧者のいずれからも閲覧不可とする**
- **論理削除後の一定の保持期間内は、運営の判断で `restore` 可能とする**（誤削除・誤通報への対応のため）
- **保持期間を過ぎたデータは物理削除（purge）対象とする**。関連する画像ファイルも削除対象に含まれる
- 保持期間の具体的日数・復元条件はプライバシーポリシー・利用規約に明記する
- Slug 復元ルールは §6.18 参照

#### 付随する業務ルール（session と共有 PC 対策）

- 明示破棄 UI で session を revoke しても、管理 URL 自体は失効させない（別端末からの再入場を妨げない）
- session 期限切れ時は「管理 URL または Draft URL から再入場してください」と案内する
- 別端末から同じ管理 URL にアクセスした場合は別 session として扱い、既存 session は自動失効させない（管理 URL 再発行時のみ一括 revoke）

---

### 3.5 管理 URL 控え機能

#### 責任範囲

ログイン不要サービスの要である管理 URL を、作成者が確実に保持できるよう支援する。

#### この機能が担うこと

- 公開完了時に管理 URL を強調表示する
- 管理 URL をコピーする操作を提供する
- 管理 URL を作成者が指定したメールアドレスに送信する操作を提供する（ManageUrlDelivery 集約）
- 管理 URL の重要性（これがないと編集・削除不能になる旨）を明示的に伝える

#### この機能が守ること

- メール送信はあくまで「控えの送付」であり、ログイン・アカウント作成ではない
- メール送信時に受け取ったメールアドレスは、控え送付以外の用途に用いない
- 管理 URL の重要性は、見逃せない位置・デザインで作成者に伝える
- 控えを保存せずに公開完了ページを離脱することへの警告を表示する
- **メール本文が送信履歴 UI や API ログに残らないプロバイダを選定する**（ADR-0004、現在 Proposed）

#### この機能が担わないこと

- メールアドレスの永続保存やログイン情報化（Phase 2）
- 管理 URL の再発行（§3.4 参照）

#### 付随する業務ルール（ManageUrlDelivery）

- 管理 URL 控えメール送信要求は ManageUrlDelivery 集約で扱う
- `recipient_email` は短期保持し、**24 時間後に NULL 化**する
- 重複検出等が必要な場合は `recipient_email_hash` を使う
- `manage_url_token_version_at_send` を送信時点の snapshot として保持する（後の token 再発行でどの世代の URL を送ったかを追跡可能）
- メールプロバイダは ADR-0004 が Proposed のため未確定。候補は Resend / AWS SES / SendGrid / Mailgun。本文ログ保持の扱いが最重要選定基準
- ReissueManageUrl 時は運営が連絡先を確認したうえで `recipient_email` を手動入力する。過去の `recipient_email` は再利用しない

---

### 3.6 通報機能

#### 責任範囲

閲覧者が、不適切なフォトブックや自分が無断で掲載されているフォトブックなどを運営に報告するための手段を提供する。

#### この機能が担うこと

- フォトブック閲覧ページから通報画面への遷移を受け付ける
- 通報対象のフォトブックを特定する
- 通報理由のカテゴリ選択を受け付ける
- 自由記述の詳細を任意で受け付ける（最大 2000 文字）
- 連絡先を任意で受け付ける（最大 200 文字、短期保持）
- 運営への通知を行う
- 通報送信完了を閲覧者に伝える

#### 通報理由カテゴリ

| 値 | 表示名 | 想定ケース |
|----|-------|----------|
| `subject_removal_request` | 被写体として削除希望 | 写っている本人からの削除依頼 |
| `unauthorized_repost` | 無断転載の可能性 | 他人の写真が無断で掲載されている |
| `sensitive_flag_missing` | センシティブ設定の不足 | センシティブ申告なしで不適切表現 |
| `harassment_or_doxxing` | 嫌がらせ・晒し | 個人攻撃、晒し |
| `minor_safety_concern` | 年齢・センシティブに関する問題 | 未成年を連想させる性的表現、その他未成年保護関連（v4 新規、優先対応） |
| `other` | その他 | 上記以外 |

#### この機能が守ること

- 閲覧者は自分のログインや登録なしに通報を行える
- 通報の理由カテゴリは VRC コミュニティの実情に即している
- 通報を受け付けたことが運営側で確実に記録される
- 通報時に受け取った連絡先は通報対応以外の用途に用いない
- **`minor_safety_concern` は `other` よりも通知レベルを上げ、優先対応する**
- **`reports.target_photobook_id` に DB 外部キー制約は付けない**（Photobook 物理削除後も監査証跡として残すため）
- **通報対象の `public_url_slug` / `title` / `creator_display_name` をスナップショットとして保持**
- **通報受付対象の可視性条件**: `status=published` かつ `hidden_by_operator=false` かつ `visibility != private` の photobook を通報対象とする。すなわち `public` / `unlisted` は通報可能、`private` / `hidden` / `draft` / `deleted` / `purged` は通報対象外（公開 Viewer の到達条件と同じ）

#### この機能が担わないこと

- 通報に対する自動的な処置（全ての対応は運営が手動で行う）
- 運営側の通報対応管理 UI（MVP では `cmd/ops/list_reports` / `cmd/ops/report_resolve` で対応）

#### 付随する業務ルール（通報後の取り扱い）

- **通報直後は原則として公開状態を維持する**
- **明らかに重大な権利侵害、嫌がらせ、センシティブ違反、未成年を連想させる性的表現、個人攻撃・晒しが疑われる場合、運営判断で一時非表示にできる**
- 一時非表示は運営のみが実行でき、作成者自身は実行できない
- 一時非表示中のフォトブックは公開 URL 経由で閲覧できない
- 運営判断の結果、問題ないと判断されれば一時非表示は解除される（`cmd/ops/photobook_unhide`）
- 削除処分となる場合は §3.4 の削除ルールに従う
- 通報者への結果通知は、連絡先が提供された場合に限り運営判断で行う
- Report 解決時、sourceReportId 付きの hide / soft_delete / purge では `Report.status = resolved_action_taken`
- unhide / restore では Report 状態を自動変更しない
- `Report.contact` と `Report.source_ip_hash` は一定期間後に NULL 化する

---

### 3.7 荒らし対策機能（利用制限）

#### 責任範囲

ログイン不要サービスにおいて、スパム・大量投稿・悪意ある利用を抑止するための最低限の防御を提供する。

#### この機能が担うこと

- 公開操作前に bot 検証（Turnstile）を実行する
- 画像アップロード前に Turnstile を実行する（upload-intent より前、ADR-0005）
- 同一の作成元（IP アドレス等）からの作成レートを計測する
- 1 フォトブックあたりの画像枚数上限を適用する
- 1 画像あたりのファイルサイズ上限を適用する
- 画像形式の軽量バリデーションを行う（upload-intent）
- 画像の本検証を image-processor で行う（§3.10）
- 上限超過時に公開・アップロードを拒否する

#### この機能が守ること

- bot 検証を通過しない公開操作・アップロード操作は行われない
- 上限を超える画像枚数・ファイルサイズを持つフォトブックは公開されない
- レート制限の閾値は、通常利用者の体験を損なわない水準に設定する
- **画像アップロード時の安全検証**:
  - 拡張子ではなく MIME / マジックナンバー / 実デコードで形式判定
  - SVG は MVP で禁止
  - アニメーション WebP / APNG は MVP で拒否
  - 最大長辺 8192px、最大ピクセル数 40MP
  - デコンプレッションボム対策
- **UsageLimit と Report の IP ハッシュは同じソルトポリシーを共有する**（同一作成元からの大量投稿・大量通報の相関検出のため）
- **ハッシュソルトはバージョン番号を持ち、ローテーション時は長期追跡性が失われることを許容する**

#### Turnstile セッション化（ADR-0005）

画像 20 枚アップロード時に毎回 Turnstile を要求すると UX が破綻するため、Turnstile 検証結果を `upload_verification_sessions` テーブルで短期保持する。

- 有効期限 30 分
- 1 検証あたり最大 20 回の upload-intent を許可
- 対象 Photobook ID に紐づく（他フォトブックに流用不可）
- 期限切れ・回数超過・Photobook 不一致の場合は再検証を求める

#### この機能が担わないこと

- 通報されたフォトブックの自動削除（運営判断）
- センシティブ画像の自動検出（MVP では対象外、申告ベース）

#### 付随する業務ルール（画像枚数上限）

上限値は **MVP 無料枠の初期値**として以下を設定する。将来、任意ユーザー機能や有料プランが導入された際に段階的に緩和する。

- **MVP 無料枠**: 1 フォトブックあたり画像 **20 枚** まで、1 画像 **10MB** まで、同一作成元で 1 時間に **5 冊** まで
- **将来の任意ユーザー枠**: 画像枚数上限を 50 枚 程度に引き上げる想定
- **将来の有料プラン枠**: 100 枚以上を想定

---

### 3.8 X 共有支援機能

#### 責任範囲

作成者が完成したフォトブックを X で共有する行為を、自然で手軽にする。

#### この機能が担うこと

- X で共有するための投稿文テンプレートを生成する
- フォトブックのタイトル・タイプ・ハッシュタグを含む投稿文を提示する
- X の投稿画面への遷移を提供する
- 公開 URL をコピーする操作を提供する
- X でリンクが展開された際の見え方（OGP 画像）を作成者がプレビューできる

#### この機能が守ること

- **共有される URL は公開 URL であり、管理 URL が混入しない**
- OGP 画像はスマホ上の X タイムラインで視認性が十分な内容である
- 投稿文にはサービス認知を促す最低限の要素（ハッシュタグ等）が含まれる

#### この機能が担わないこと

- X への自動投稿（作成者が手動で投稿する）
- X 連携ログイン（Phase 2）

#### 付随する業務ルール

- 投稿文は作成者が編集可能（自動生成は初期値の提示に留める）
- OGP 画像はタイプごとに適切なデザインで生成される
- OGP 状態は `photobook_ogp_images` で独立管理される（§6.17）

---

### 3.9 ランディング機能（サービス入口）

#### 責任範囲

サービスに初めて訪れた人に、何ができるサービスかを伝え、フォトブックの作成開始へ導く。

#### この機能が担うこと

- サービスの概要を伝える
- ログイン不要で使えることを強く伝える
- 作成開始への主要な導線（CTA）を提供する
- 作成例（サンプルフォトブック）を提示する
- 特徴と使い方の簡潔な説明を提供する

#### この機能が守ること

- 「登録不要・すぐ始められる」というメッセージが最初に伝わる
- スマホファーストで、スクロールすれば主要情報に触れられる
- 主 CTA は画面上の目立つ位置に配置される

#### 付随する業務ルール

- 主 CTA 文言の方向性は「ログイン不要で作る」「今すぐフォトブックを作る」等、ログイン不要の価値を明示する
- サンプルフォトブックは実際のサービス上のフォトブックを用いる
- LP ページも含めて MVP では `noindex` を付与する（§7.6）

---

### 3.10 画像管理機能

#### 責任範囲

作成者がアップロードした画像ファイルを、フォトブックに適した形式で保存・配信・削除する。本サービスが扱う「画像」というデータ資産のライフサイクルを担う。

#### この機能が担うこと

- upload-intent / complete の 2 段 API で画像ファイルを受け付ける（§3.1、ADR-0005）
- R2 への画像永続化（presigned URL 経由で直接 PUT）
- image-processor による非同期変換（HEIC→JPG/WebP、variant 生成）
- 公開 URL からの画像配信
- フォトブックの削除・ページ削除・写真削除に伴う、関連画像の削除
- 画像ファイルに付随するメタデータ（EXIF / XMP / IPTC 等）の除去

#### 画像の状態遷移

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

**`rejected` は MVP では使わない**。すべて `failed` に集約し、理由は `failure_reason` で区別する（§2.11 非採用用語）。

#### `failure_reason` 語彙

`failure_reason` は enum（text + CHECK 制約）で管理し、未知の値が入らないようにする。MVP 初期語彙:

- `file_too_large` — 申告または実サイズが 10MB 超
- `size_mismatch` — 申告 Content-Length と実サイズが不一致
- `unsupported_format` — 受け入れ対象外の形式（GIF、BMP、TIFF 等）
- `svg_not_allowed` — SVG は拒否
- `animated_image_not_allowed` — アニメーション WebP / APNG
- `dimensions_too_large` — 長辺 8192px 超 または 40MP 超
- `decode_failed` — 実デコードに失敗（破損ファイル、デコンプレッションボム疑い含む）
- `exif_strip_failed` — メタデータ除去に失敗
- `heic_conversion_failed` — HEIC からの変換に失敗
- `variant_generation_failed` — display / thumbnail 生成に失敗
- `object_not_found` — complete 時に R2 オブジェクトが存在しない
- `unknown` — 上記に分類できない内部エラー（運用で追補する）

#### storage_key 命名規則（ADR-0005）

```
photobooks/{photobook_id}/images/{image_id}/original/{random}.{ext}
photobooks/{photobook_id}/images/{image_id}/display/{random}.webp
photobooks/{photobook_id}/images/{image_id}/thumbnail/{random}.webp
photobooks/{photobook_id}/ogp/{ogp_id}/{random}.png
```

`{random}` は 12 バイト程度の暗号論的乱数を base64url 化したもの。外部推測耐性とキャッシュ破棄性を兼ねる。

#### この機能が守ること

- MVP では **JPG / PNG / WebP** を正式対応形式とする
- **HEIC** は Safari/iPhone 利用者がアップロードする可能性が高いため、サーバー側で libheif により JPG / WebP へ変換する
- 公開時には表示用のリサイズ画像とサムネイルを生成する
- 正規化形式は **JPG または WebP**。PNG は入力として受け付け、透過がある場合 WebP、透過がない場合 JPG または WebP に変換する
- **SVG は禁止**、**アニメーション WebP / APNG は拒否**
- **EXIF / XMP / IPTC 等の埋め込みメタデータは、公開配信用画像から原則全除去する**（§3.10「EXIF / XMP / IPTC とクレジット」参照）
- **画像は `owner_photobook_id` により特定の Photobook に所有される**（`reference_count` 方式は採用しない）
- **画像の用途（`usage_kind`）を明示する**: `photo` / `cover` / `ogp`（`creator_avatar` は MVP 非対応）
- **Image のステータスが `available` 以外のとき、公開中の Photobook に紐づけてはならない**（`failed` 画像の公開禁止）

#### EXIF / XMP / IPTC とクレジット

- GPS 位置情報、デバイスシリアル、PC ユーザー名、ローカルファイルパス、撮影日時などの個人特定情報は公開配信用画像から除去する
- Author / Artist / Photographer / Copyright 等のクレジット情報も**画像メタデータとしては残さない**
- **クレジット表示が必要な場合は、Photobook のメタ情報として作成者が明示入力する**（誤って PC ユーザー名や編集ソフト由来の文字列が公開されるリスクを防ぐため）
- VRC 文化のクレジット需要（ワールド名、撮影者名、アバター改変クレジット等）は、ページメタ情報で満たす

#### 画像の所有と削除連鎖

```
Photobook (draft または published)
  │
  │  owner_photobook_id
  ▼
Image (usage_kind = photo / cover / ogp)

Photobook 物理削除（purge）
  ↓
owner_photobook_id が一致する Image を削除対象に（Outbox 経由で非同期実行）
  ↓
Image 物理削除 + R2 オブジェクト削除 + CDN キャッシュパージ
```

#### GC 方針

以下を自動 reconciler と `scripts/ops/reconcile` の両方で扱う（§6.16）。

- upload-intent 後に complete されない Image は一定時間後に削除対象（MVP は 1 時間目安、フロント離脱率で調整）
- `draft_expires_at` を過ぎた draft に紐づく Image は削除対象（draft_expired reconciler で連鎖削除）
- `failed` 状態で一定期間経過した Image は deleted（7 日）→ purged（30 日）
- `deleted` 状態で一定期間経過した Image は purged（30 日、R2 オブジェクト削除）

#### この機能が担わないこと

- 画像の加工・編集・切り抜き（AI 画像加工、MVP 外）
- 画像のアップロード時の UI や入力バリデーション（§3.1 の責任）
- 画像に関するストレージサービスの選定（ADR-0001 で R2 採用）

#### 付随する業務ルール

- 1 画像が複数 Photobook から共有されることは MVP では起きない想定
- draft 状態の Photobook に所属する Image は、draft 削除時に連鎖削除される
- draft 画像も `owner_photobook_id` を持つ
- **publish 時に `Image.owner_photobook_id` は変更しない**
- OGP 画像も Image 集約で実ファイルを管理する（`usage_kind = ogp`）。OGP 状態（pending/generated/failed/fallback/stale）は `photobook_ogp_images` で独立管理（§6.17）

---

## 第4部 フォトブックタイプ別の業務ルール

MVP で提供するフォトブックタイプについて、それぞれの業務的な意図・既定値・選択可能な選択肢を定義する。

### 4.1 タイプ別既定値の一覧

| タイプ | 業務的意図 | 既定の開き方 | 既定レイアウト | 選択可能レイアウト |
|---|---|---|---|---|
| event（イベント） | 複数の人が関わる集まりの記録。VRC バー・コンカフェ・撮影会・コミュニティイベント | 軽め（日付とイベント名の帯） | カード | カード / シンプル / 雑誌 |
| daily（おはツイ） | おはツイ、日々の活動を軽く記録して頻繁に共有 | 軽め（表紙なし） | シンプル | シンプル / 雑誌 |
| portfolio（作品集） | フォトグラファーによる作品発表 | 表紙ファーストビュー | 大判 | 大判 / カード / 雑誌 |
| avatar（アバター紹介） | アバター改変・衣装紹介 | 表紙ファーストビュー | カード | カード / 雑誌 / 大判 |
| world（ワールド） | 訪れた VRChat ワールドの魅力・世界観 | 表紙ファーストビュー | 大判 | 大判 / 雑誌 / カード |
| memory（思い出） | フレンド・グループとの私的な記録 | 軽め | シンプル | シンプル / カード / 雑誌 |
| free（自由） | 上記に明確に当てはまらない用途 | 軽め | シンプル | シンプル / カード / 雑誌 / 大判 |

### 4.2 タイプ別の提供範囲と実装優先度

MVP では全 7 タイプを選択肢として提示するが、実装の作り込みの深さには差を設ける。

| タイプ | MVP 提供 | 専用機能の作り込み |
|---|---|---|
| event | ○ | 完全対応（日付帯、キャスト情報等） |
| daily | ○ | 完全対応（軽量テンプレ） |
| portfolio | ○ | 簡易対応（大判レイアウトのみ、ポートフォリオ専用機能は後回し） |
| avatar | ○ | 簡易対応（カードレイアウトのみ、BOOTH 連携等は後回し） |
| world | ○ | 簡易対応 |
| memory | ○ | 簡易対応 |
| free | ○ | 既定レイアウトのみ |

「簡易対応」とは、タイプ選択肢として提示し既定レイアウトを用意するが、専用のメタ情報フィールドや連携機能は実装しないことを意味する。

表紙（cover）は `photobooks` テーブルにインライン化されるが、タイプ別の表紙表示有無は `OpeningStyle` で引き続き制御される。

---

## 第5部 業務上の境界（MVP スコープ）

### 5.1 MVP で提供する機能

以下を MVP で提供する。

- フォトブック作成機能（§3.1）
- フォトブック公開機能（§3.2）
- フォトブック閲覧機能（§3.3）
- フォトブック管理機能（管理 URL ページ）（§3.4）
- 管理 URL 控え機能（§3.5）
- 通報機能（§3.6）
- 荒らし対策機能（利用制限）（§3.7）
- X 共有支援機能（§3.8）
- ランディング機能（§3.9）
- 画像管理機能（§3.10）

**内部構造として MVP から提供するもの**:

- Transactional Outbox（集約間イベントのトランザクション保証、§6.11）
- OGP 生成（独立管理、§6.17）
- Reconcile スクリプト群（運営向け、§6.16）
- token→session 交換方式（§6.15、ADR-0003）
- R2 presigned URL 方式の画像アップロード（§3.10、ADR-0005）
- 運営操作 CLI（`cmd/ops` + `scripts/ops`、§6.19、ADR-0002）

### 5.2 MVP で提供しないこと（意図的にスコープ外）

以下は MVP では**作らない**。

- **ログイン必須化**（ログインは任意、MVP では扱わない）
- **作者ページ**
- **マイページ**（複数フォトブック横断管理）
- **いいね**
- **コメント**
- **フォロー**
- **ランキング**
- **X 自動取得 / X 連携ログイン**
- **高度な AI 画像加工**（背景除去、切り抜き、合成）
- **creator_avatar 画像アップロード**（Phase 2 以降）
- **運営 HTTP API**（`/internal/ops/*` は作らない、ADR-0002）
- **SNS 化全般**（フォロー・いいね・コメント等）
- **画像の `reference_count` 共有管理**（`owner_photobook_id` 方式に統一）
- **検索エンジン index 許可**（全ページ `noindex`、§7.6）
- **物理フォトブック注文**（印刷発注等）
- **表紙の複数画像レイアウト・専用テンプレート**
- **動画対応**
- **共同編集**
- **決済・有料プラン**
- **タイプ別の深い作り込み**（BOOTH 連携、ポートフォリオ専用機能等）

### 5.3 MVP 境界の判断理由

| 除外した機能 | 理由 |
|---|---|
| 作者ページ・マイページ | ログイン不要方針を取るため、恒久的な作成者アカウントを前提とする機能は Phase 2 以降に回す |
| 本格的なログイン | 初回体験の摩擦を最小化するため、MVP では管理 URL 方式で代替する |
| AI 画像加工 | VRC 写真は髪・羽・尻尾・発光・半透明パーツで既存の切り抜き AI が破綻しやすく、品質期待値を下げるリスクが高い |
| 見開きブック型 | スマホでの閲覧体験・レスポンシブ対応の困難さから、縦スクロール単一形式に絞る |
| SNS 機能 | 本サービスはフォトブックの共有が目的であり、SNS 化しない方針 |
| タイプ別の作り込み | タイプの多様性は確保しつつ、まずは共通機能を完成させる |
| 運営 HTTP API | 外部攻撃面を最小化し、MVP 規模の運営操作は CLI で十分（ADR-0002） |
| creator_avatar | 運営負担・モデレーション範囲を MVP ではフォトブック本体に絞る |

### 5.4 運営対応の最低限手段

MVP では運営が使う専用の管理 UI は提供しない。通報対応・一時非表示・削除・管理 URL 失効等を実行する手段は `cmd/ops` + `scripts/ops` で提供する（ADR-0002）。

- **運営操作は `scripts/ops/*.sh` + `cmd/ops` Go CLI で行う**
- `cmd/ops` は単一バイナリ + サブコマンド方式
- shell は薄いラッパー
- **運営 UseCase は Application 層に実装**。Phase 2 で運営 UI を作る場合も同じ UseCase を流用する
- **破壊的操作には `--dry-run` を用意し既定とする**。実行には `--execute` を明示
- **実行時には `--operator` を必須にする**
- 参照系（list_reports / list_moderation_actions）は `--execute` 不要。個人情報・token・管理 URL を出力しない
- **ModerationAction を必ず記録する**
- **Photobook 状態変更 / ModerationAction INSERT / Report 状態更新 / outbox_events INSERT は同一トランザクションで行う**

運営操作の対象（§2.10、ADR-0002）:

- hide / unhide
- restore / purge
- reissue_manage_url
- resolve_report
- list_reports / list_moderation_actions
- reconcile 系

operator 識別子の形式は §2.10 参照。

### 5.5 将来の拡張方針

MVP 以降の拡張は以下の順で想定する。各フェーズの詳細は別途計画する。

- Phase 2: 任意ユーザー機能の本格化（ログイン、マイページ、作者ページ、デコレーション強化、運営 UI）
- Phase 3: AI 生成補助（タイトル・説明・X 投稿文の自動生成）
- Phase 4: AI 画像加工（背景除去等）
- Phase 5 以降: 新レイアウト、タイプ別の作り込み深化（BOOTH 連携、ポートフォリオ機能）、SNS 機能の段階的導入、有料プラン、物理印刷発注

---

## 第6部 横断的な業務ルール

各機能に限定されず、サービス全体で守るべきルールを定義する。

### 6.1 写真が主役であること

- UI・演出・装飾は写真の魅力を阻害してはならない
- 作成者・閲覧者いずれの画面でも、写真は十分な面積と画質で表示される

### 6.2 スマホファースト

- 全ての公開側画面（閲覧・LP・通報フォーム等）はスマートフォン表示を第一に設計する
- スクロール体験が最重要で、凝ったインタラクションよりも軽快さを優先する
- **Safari / iPhone Safari 対応を考慮する**（Cookie 属性、ITP、履歴挙動）

### 6.3 作成者の表現の尊重

- 作成者が入力した内容（タイトル・キャプション・作成者名等）は、運営が勝手に改変しない
- タイプ既定値はあくまでガイドレールであり、作成者の選択は可能な限り尊重される

### 6.4 閲覧者の体験の尊重

- 閲覧者にログインを強要しない
- 閲覧者に SNS アカウントの連携を強要しない
- センシティブな内容は閲覧前にワンクッションを挟む

### 6.5 権利とモラルの確認

- 公開するフォトブックには、必ず作成者による権利・配慮確認の同意を得る
- 同意の内容は「写っている人やアバター、ワールド等に配慮し、公開して問題ない内容であることを確認しました」という、VRC 文化に馴染む表現を用いる
- 問題があった場合の通報導線は常に提供される

### 6.6 ログイン不要の徹底

- 作成・公開・閲覧・編集・削除・通報のいずれも、ログインなしで成立する
- ログインは複数管理や有料機能のための任意手段であり、基本機能の条件にしない

### 6.7 管理 URL / draft_edit_token の秘匿

- 管理 URL と draft 編集 URL は作成者以外の誰にも知られてはならない
- これらは公開情報・分析データ・ログ・OGP・メタタグ等のいずれにも露出しない
- DB には token の hash のみを保存し、raw token を保持しない
- 管理 URL の管理責任は作成者に帰属する旨を明示する

### 6.8 作成途中データの保持

- 作成途中のタイトル・説明・並び順・入力情報は、可能な限りブラウザに保持する
- ただし**初回画像アップロードで server draft 化**されるため、端末依存性は軽減される（§3.1）
- 画像本体のブラウザ側保持は、ブラウザ・端末の制約に依存する
- Safari を含むブラウザの容量制約により、画像本体が復元できない場合は作成者に再アップロードを求める

### 6.9 画像ファイルのライフサイクル

- 画像は `owner_photobook_id` により特定 Photobook に所有される（`reference_count` 方式は採用しない）
- フォトブックの削除・ページ削除・写真削除に伴い、関連する画像ファイルも削除対象となる
- 論理削除の保持期間を過ぎたフォトブックは、画像ファイルを含めて物理削除される
- 削除された画像ファイルは、CDN キャッシュが残存する期間を経てアクセス不能となる
- draft 期限切れ時、draft 所属 Image も連鎖削除される

### 6.10 公開範囲に関する UI 文言の方針

公開範囲を選択・表示する画面では、ログイン不要サービスであることに配慮した正確な文言を用いる。

- 「非公開」の説明で「自分だけが見られる」「あなたしか見られない」等の表現は**使わない**。ログイン不要のため「自分」を識別できず、管理 URL を持つ人なら誰でも閲覧できるため、認識のズレを生む
- 代わりに「管理 URL を持っている人だけが確認できます」「公開 URL では表示されません」等、仕組みに即した文言を用いる
- 「限定公開」についても同様に、「URL を知っている人のみ閲覧できます」という事実ベースの表現を用いる

### 6.11 集約間イベントは Outbox で伝搬する

- 集約の状態変更と副作用（OGP 生成、メール送信、CDN パージ、画像物理削除等）は**同一トランザクション内で `outbox_events` に INSERT** し、非同期ワーカーで実行する
- 失敗時は retry し、上限超過で `failed` となり Reconcile 対象になる
- 対象イベントは §2.9 参照

### 6.12 整合性は Reconcile で保証する

画像参照、OGP 状態、Outbox 失敗、Draft 期限切れ、CDN キャッシュ等の不整合は、Reconcile で検出・修復する。自動 reconciler と手動 `scripts/ops/reconcile` を分ける（§6.16）。

### 6.13 管理 URL と draft 編集 URL の漏洩対策は同等

- `draft_edit_token` は公開前のフォトブックの編集権を握るため、`manage_url_token` と同等の漏洩対策を適用する
- `Referrer-Policy: no-referrer`、外部リソース読み込み禁止、エラートラッキングへの URL 送信禁止
- X 共有時には公開 URL のみを使う（UI 実装レベルで制約）

### 6.14 画像の所有と削除連鎖

- 画像は `owner_photobook_id` により特定 Photobook に所有される（`reference_count` 方式は MVP では採用しない）
- Photobook 物理削除時、所有 Image は Outbox 経由で連鎖削除される
- draft 期限切れ時、draft 所属 Image も連鎖削除される
- **publish 時に `Image.owner_photobook_id` は変更しない**（Draft と Published は同一 Photobook ID）

### 6.15 token→session 交換方式（ADR-0003）

`draft_edit_token` / `manage_url_token` は初回 URL アクセス時のみ URL に含める。サーバー検証後、短命 session を発行し HttpOnly Cookie に保存、URL から raw token を除去する（redirect）。

- Cookie に入れる値は **256bit 以上の暗号論的乱数を base64url 化**したもの
- DB には `session_token_hash` のみを保存
- session は `token_version_at_issue` を持ち、manage_url_token 再発行時に旧 version 由来の manage session を一括 revoke する
- publish 成功時には対象 Photobook の draft session を全 revoke
- Cookie 属性: HttpOnly / Secure / SameSite=Strict / Path=/
- 明示破棄 UI で session を revoke しても、管理 URL 自体は失効させない
- 詳細は ADR-0003

### 6.16 Reconcile の分類

**自動 reconciler**（cron 起動）:

- `draft_expired` — 期限切れ draft の自動削除
- `delivery_expired_to_permanent` — ManageUrlDelivery の failed_retryable を expireAt で permanent 確定
- `outbox_failed_retry` — Outbox 失敗イベントの再試行
- `stale_ogp_enqueue` — stale OGP の再生成キューイング

**手動 `scripts/ops/reconcile/`**（運営判断で実行）:

- `image_references.sh` — 画像参照不整合の検出
- `photobook_image_integrity.sh` — Photobook と Image の整合性検査
- `cdn_cache_force_purge.sh` — CDN キャッシュの強制パージ
- `outbox_failed.sh` — Outbox 失敗の手動対応
- `ogp_stale.sh` — stale OGP の手動再生成
- `draft_expired.sh` — 期限切れ draft の手動削除

**全手動 reconcile スクリプトは `--dry-run` を持つ**（ADR-0002）。

### 6.17 OGP の独立管理

- OGP 状態は `photobook_ogp_images` 等で独立管理する
- OGP 実ファイルは Image 集約で管理する（`images.usage_kind = ogp`）
- 生成前/失敗時は `image_id = NULL` を許容する
- OGP 状態: `pending` / `generated` / `failed` / `fallback` / `stale`
- OGP 生成失敗時も Photobook 公開自体は成功扱い（fallback OGP を利用）
- Photobook 更新時は OGP を `stale` にし、再生成対象にする
- **OGP 配信可否（2026-05-11 更新）**: `status='published'` かつ `hidden_by_operator=false` かつ `visibility ∈ {public, unlisted}` の photobook のみ generated OGP を配信する。`private` / `draft` / `deleted` / `purged` / `hidden_by_operator=true` は default OGP（teal placeholder）にfallback。検索拒否（`noindex` 全ページ付与、§7.6）は HTML 側で別途維持しており、本 OGP 配信判定とは独立

### 6.18 Slug 復元ルール

- `published`: `public_url_slug` は有効
- `deleted`: `public_url_slug` は保持され、**他の Photobook に再利用されない**。保持期間内は運営判断で `restore` 可能（同じ slug で復元）
- `purged`: 物理削除済み。`public_url_slug` は解放され再利用可能。**restore 不可**
- `restore` / `soft_delete` / `purge` はすべて `ModerationAction` として追記記録される
- 保持期間内であれば複数回 `restore` 可能

### 6.19 運営操作は HTTP API 化しない（ADR-0002）

- MVP では運営 HTTP API を作らない
- 運営操作は `scripts/ops/*.sh` + `cmd/ops` Go CLI で行う
- 運営 UseCase は Application 層に実装し、CLI はその薄いラッパーに徹する
- Phase 2 で運営 UI を作る場合も、同じ UseCase を HTTP から呼ぶ
- operator 識別子は個人情報を含まない運営内識別子（§2.10）
- 破壊的操作は `--dry-run` 既定、`--execute` 明示必須
- ModerationAction は必ず記録し、4 要素（Photobook / ModerationAction / Report / Outbox）を同一トランザクションで更新

### 6.20 楽観ロックによる同時編集事故防止

- Photobook に `version`（または `lock_version`）を持ち、更新時にインクリメント
- 管理 URL が複数人に共有された場合や、複数タブでの編集時、後勝ち更新による事故を防ぐ
- MVP では version 不一致でエラーを返す（マージ処理はしない）

---

## 第7部 規約・プライバシー・SEO に関する業務前提

本書は法律文書ではないが、本サービスが扱うデータと行為の性質上、利用規約・プライバシーポリシーで明記する必要がある業務前提を整理する。詳細な法務文書は別途作成する。

### 7.1 利用規約で明記すべき業務前提

- **投稿される画像に関する権利と責任**: 作成者は、投稿する画像について必要な権利（著作権、被写体の許諾等）を有することを表明する
- **権利・配慮確認の同意**: 公開操作時の権利・配慮確認は、作成者の自己責任による宣言として記録される
- **禁止事項**: 他者のプライバシー侵害、無断転載、誹謗中傷、性的表現、未成年を連想させる性的表現、暴力表現等の禁止
- **運営の権限**: 通報等を受けた場合、運営は一時非表示・削除・アカウント停止等の措置を講じることができる
- **管理 URL の取り扱い**: 管理 URL の管理責任は作成者に帰属する。紛失や漏洩は作成者の責任となる
- **サービスの変更・停止**: 予告なくサービス内容を変更・停止する可能性がある

### 7.2 プライバシーポリシーで明記すべき業務前提

- **取得する情報**:
  - 作成者が入力した作成者情報（表示名、任意で X ID）
  - 作成者が入力した管理 URL 控え送信先のメールアドレス（短期保持、24h 後 NULL 化）
  - 通報者が任意で入力した連絡先（短期保持、一定期間後 NULL 化）
  - アクセスログ、IP ハッシュ、Cookie 等
  - 画像ファイルに付随するメタデータ（EXIF / XMP / IPTC の位置情報等は公開時に除去する）
- **利用目的**:
  - 管理 URL 控えの送信（メールアドレス）
  - 通報対応と連絡（通報者連絡先）
  - 荒らし対策、レート制限（IP ハッシュ）
  - サービス品質改善
- **第三者提供**: 法令に基づく場合等を除き、第三者に提供しない
- **保持期間**:
  - 論理削除されたフォトブックおよび画像は、一定の保持期間を経て物理削除される
  - メールアドレス・通報連絡先は用途完了後、適切な期間内に削除する
  - IP ハッシュのソルトはバージョン管理され、ローテーション時は長期追跡性を失う
- **削除請求**: 作成者は自身のフォトブックを削除でき、被写体等の第三者は通報機能を通じて削除を申請できる

### 7.3 権利侵害申立てへの対応

- 被写体本人や権利者からの削除申立ては、通報機能を正式な窓口とする
- 申立てを受けた場合、運営は速やかに内容を確認し、必要に応じて一時非表示・削除の措置を講じる
- 明らかな権利侵害と判断された場合、作成者への事前通知なく措置を講じる場合がある

### 7.4 未成年保護に関する前提

- サービスは未成年の利用を制限しないが、未成年を被写体とするセンシティブな表現、あるいはアバターを通じて未成年を連想させる性的表現は禁止する
- 関連する通報（`minor_safety_concern`）は優先的に対応し、必要に応じて即時一時非表示とする

### 7.5 メールアドレスの取り扱い

- 管理 URL 控え送信機能で受け取ったメールアドレスは、控え送信以外の用途（マーケティング、ログイン情報化等）に用いない
- 送信後のメールアドレス保持期間は、運用と技術設計で最小限に留める（24h 後 NULL 化）
- メールプロバイダ選定では、本文ログ保持の扱いを選定基準とする（ADR-0004 Proposed）

### 7.6 SEO / robots 方針

MVP では、検索エンジンへの露出リスクを最小化する。

- **全フォトブックページに `noindex` を付与**（public / unlisted / private すべて）
- **draft 編集 URL ページ（`/draft/*`）と管理 URL ページ（`/manage/*`）にも `noindex`**
- **sitemap は生成しない**
- **`robots.txt` で `/manage/` と `/draft/` を `Disallow`**（ただし `robots.txt` だけに頼らず、ページ側にも `noindex` を入れる）

**Referrer-Policy**:

- 通常ページ（閲覧・LP）: `strict-origin-when-cross-origin`
- 管理 URL / draft URL / token 付き URL: `no-referrer`

**Phase 2 以降**: 作成者が「検索エンジンに表示する」を明示的に ON にした場合のみ index 許可を検討する。

**理由**: VRC 写真は、被写体・アバター・イベント参加者・ワールド情報が絡みやすく、最初から検索エンジンに露出させるリスクが高い。

**実装上の補足（業務ルールとは別、技術メモ）**: フロントエンドのレスポンスヘッダ（`X-Robots-Tag` / `Referrer-Policy` / `Cache-Control` 等）は実装側で **middleware に集中管理**する方針。M1 PoC で複数箇所からの重複付与による値の二重化を確認したため。実装詳細は ADR-0001 / `docs/plan/m1-spike-plan.md` 側で扱う。

---

## 付録A: 業務知識とドメインモデル設計の関係

本書は**業務知識の定義**であり、クラス設計・集約設計・データベーススキーマ・API 仕様等は扱わない。これらは本書を前提として、ドメインモデル設計書・データモデル設計書・ADR 群で別途扱う。

本書はドメインモデル設計の上流として、以下の役割を持つ。

- 開発者とドメインエキスパートが「何をするサービスか」を共通理解するための辞書
- 各機能の責任範囲を明確化し、責務分散の指針となる参照元
- 将来的な機能追加や仕様変更の判断基準
- ユビキタス言語の唯一の正典

**関連ドキュメント**:

- 集約設計: `docs/design/aggregates/{photobook,image,report,moderation,usage-limit,manage-url-delivery}/`
- 技術決定: `docs/adr/0001-*.md` 〜 `0005-*.md`
- UI プロトタイプ: `design/mockups/prototype/`

---

## 付録B: バージョン間の主な改訂点

### v3 → v4

| 項目 | v3 | v4 |
|------|----|----|
| 画像参照モデル | `reference_count`（参照カウント） | `owner_photobook_id` + `usage_kind`（所有モデル） |
| Image 状態 | uploading / processing / available / deleted / purged | 左記 + `failed`（処理失敗状態）を追加。`rejected` は採用しない |
| 画像安全検証 | 記載なし | MIME/マジックナンバー/実デコード、SVG 禁止、アニメーション拒否、最大長辺/ピクセル数、ボム対策を明記 |
| 画像アップロード方式 | API 直送想定 | **R2 presigned URL 方式（upload-intent / complete 2 段 API）**（ADR-0005） |
| `failure_reason` 語彙 | 記載なし | **12 種の初期語彙を確定**（§3.10） |
| `storage_key` | 記載なし | **`photobooks/{photobook_id}/images/{image_id}/{variant}/{random}.{ext}` を採用** |
| EXIF / XMP / IPTC | EXIF のみ「機密性の高いものは除去」 | **全メタデータを原則除去**。Author/Artist もクレジットは Photobook メタ情報として明示入力 |
| 正規化形式 | 記載なし | JPG または WebP。PNG の扱いを明記 |
| 作成途中の保持 | ブラウザローカル + 必要時再アップロード | **server draft + `draft_edit_token`**（初回画像アップロードで draft 作成、7 日延長式） |
| token→session 交換 | 記載なし | **URL の raw token を初回のみ消費し、HttpOnly Cookie session に交換**（ADR-0003） |
| Cookie 値 | 記載なし | **256bit 暗号論的乱数を base64url 化、DB には hash のみ保持** |
| `token_version_at_issue` | 記載なし | **manage_url_token 再発行時に旧 version 由来 session を一括 revoke** |
| URL 設計 | 暗黙 | **`/draft/{token}` → `/edit/{id}`, `/manage/token/{token}` → `/manage/{id}`**（ADR-0003） |
| OGP 生成管理 | Photobook 上の暗黙状態 | **`photobook_ogp_images` による独立管理**（pending/generated/failed/fallback/stale） |
| Cover | `photobook_covers` 別テーブル | **MVP は `photobooks` にインライン化** |
| Creator Avatar | MVP に含む | **MVP 除外、Phase 2 へ** |
| 楽観ロック | 記載なし | `version` / `lock_version` を Photobook に追加 |
| 通報理由カテゴリ | 5 種 | 6 種（+ `minor_safety_concern`） |
| 通報の Photobook 参照 | FK 方針は要検討 | **FK なし + snapshot（slug/title/display_name）保持** |
| 運営対応の実行手段 | DB 直接操作・内部スクリプト | **`cmd/ops` Go CLI + `scripts/ops/*.sh` ラッパー**（ADR-0002） |
| 運営 HTTP API | 暗黙 | **MVP では作らない**（`/internal/ops/*` なし） |
| operator 識別子 | 記載なし | **個人情報を含まない運営内識別子、正規表現で検証** |
| 運営対応トランザクション | Photobook と ModerationAction の原子性 | **Photobook + ModerationAction + Report + Outbox を同一トランザクション** |
| 集約間イベント | 明示的な仕組みなし | **Transactional Outbox を MVP から採用、`ImageIngestionRequested` 含む 11 種** |
| 整合性保証 | 記載なし | **自動 reconciler + 手動 `scripts/ops/reconcile/` に分けて MVP から用意** |
| ManageUrlDelivery の token version | `manage_url_token_version` | **`manage_url_token_version_at_send`**（意味明示化） |
| メールプロバイダ | 未記載 | **Resend / AWS SES / SendGrid / Mailgun を候補に、本文ログ保持を検証して確定**（ADR-0004 Proposed） |
| 管理 URL 漏洩対策 | 秘匿方針のみ | **`Referrer-Policy: no-referrer` 必須、外部リソース禁止、破壊的操作の二重確認 + ワンタイムトークン** |
| Slug 復元ルール | 不明瞭 | **deleted 内は同 slug 維持・再利用不可、purge 後は解放・restore 不可** |
| SEO / robots | 記載なし | **MVP は全 noindex、/manage/ と /draft/ を Disallow** |
| IP ハッシュソルト | UsageLimit 内でのみ管理 | **UsageLimit と Report で共有、version 管理** |
| Turnstile セッション化 | 記載なし | **`upload_verification_sessions` で 30 分・20 回まで保持**（ADR-0005） |

### v2 → v3

（v3 と同内容のため省略。v3 付録 B を参照）

### v1 → v2

（v3 付録 B を参照）

---

## 付録C: 次にドメイン/データモデル設計へ P0 反映すべき項目

本書 v4 と ADR-0001〜0005 を踏まえ、`docs/design/aggregates/` 配下のドメイン設計・データモデル設計に最優先（P0）で反映すべき項目を集約する。番号は実装計画上の追跡 ID として用いる。

### Photobook 集約

1. `manage_url_token_version` を持つ。
2. `draft_edit_token_hash` のみ保持し、raw token は保存しない。
3. publish 時は同一 Photobook レコードの `status` を `draft` から `published` に変更する。
4. publish 時に `draft_edit_token` を失効する。
5. publish 時に対象 Photobook の draft session をすべて revoke する。
6. publish 時に `public_url_slug` と `manage_url_token` を発行する。
7. `version` または `lock_version` による楽観ロックを持つ。

### Image 集約

8. `reference_count` は使わず、`owner_photobook_id` 方式にする。
9. `usage_kind` を持つ。MVP では `photo` / `cover` / `ogp` を想定する。
10. `rejected` は使わず、`failed` + `failure_reason` に統一する。
11. `failure_reason` の初期語彙を CHECK 制約または enum 相当で反映する。
12. `storage_key` 命名規則を variant 設計に反映する。
13. OGP 画像も Image 集約で実ファイルを管理する。

### Session 関連

14. `sessions` テーブルを設計する。
15. `sessions` には `session_token_hash` / `session_type` / `photobook_id` / `token_version_at_issue` / `expires_at` / `revoked_at` を持たせる。
16. Cookie には raw token ではなく、256bit 以上の乱数 session token を入れ、DB には hash のみ保存する。
17. `upload_verification_sessions` テーブルを設計する。
18. `upload_verification_sessions` には `photobook_id` / `session_token_hash` / `allowed_intent_count` / `used_intent_count` / `expires_at` を持たせる。

### Moderation / Report

19. ModerationAction 実行時は、Photobook 状態変更、ModerationAction 記録、Report 状態更新、`outbox_events` INSERT を同一トランザクションで行う。
20. `sourceReportId` 付きの hide / soft_delete / purge では `Report.status` を `resolved_action_taken` にする。
21. unhide / restore では `Report.status` を自動変更しない。
22. Report は DB FK なし + snapshot 保持とする。
23. Report には `target_public_url_snapshot` / `target_title_snapshot` / `target_creator_display_name_snapshot` を持たせる。

### ManageUrlDelivery

24. カラム名は `manage_url_token_version_at_send` に統一する。
25. `recipient_email` は 24h 後に NULL 化する。
26. ReissueManageUrl 時、過去の `recipient_email` は再利用しない。

### Outbox / Reconcile

27. `outbox_events` に `ImageIngestionRequested` を含める。
28. Outbox は状態変更と同一トランザクションで INSERT する。
29. failed Outbox は reconcile 対象にする。
30. `draft_expired` / `outbox_failed_retry` / `stale_ogp_enqueue` は自動 reconciler 対象にする。
31. `image_references` / `photobook_image_integrity` / `cdn_cache_force_purge` は手動 `scripts/ops/reconcile` 対象にする。
