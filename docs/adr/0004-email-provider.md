# ADR-0004 メールプロバイダ選定

## ステータス

**Accepted（MVP プロバイダ選定として）**

実送信 PoC（実 API キー発行 + 1 通テスト送信 + bounce/complaint webhook 受信）は M2 早期に別タスクで実施する。本 ADR は「MVP の第一候補・第二候補・不採用」と「実装方針（EmailSender ポート抽象化、本文最小化、Outbox payload に管理 URL を入れない等）」を Accepted 化する。

## 作成日
2026-04-25

## 最終更新
2026-04-25（M1 priority 8、4 候補比較完了 → Accepted 化）

## コンテキスト

VRC PhotoBook では、フォトブック公開時の「管理 URL 控えメール」と、運営による「管理 URL 再発行時の通知メール」、および運営通知（必要に応じて）を送信する。これらのメールには **管理 URL が本文に含まれる** ため、メールプロバイダ側でのログ保持・UI 表示が漏洩点になる。

v4 で確定している関連要件は以下のとおり。

- 送信履歴 UI にメール本文が残らないプロバイダを選定する
- `recipient_email` は ManageUrlDelivery 集約で短期保持（24h で NULL 化）
- `manage_url_token_version_at_send` を送信時点の snapshot として保持
- ReissueManageUrl 時は運営が申請者の連絡先を確認したうえで実行し、recipient_email は運営が手動入力、過去の ManageUrlDelivery の recipient_email は再利用しない

プロバイダごとに以下の差異がある。

- 送信履歴 UI にメール本文（HTML / text）が保持される / されない
- API ログにリクエストボディの本文が保持される / されない
- ログ保持期間を制御できる / できない
- 日本語メールの到達性（迷惑メール判定率）

未検証のまま採用すると、事後にプロバイダ変更が発生した場合の移行コスト（DNS 変更、DKIM/DMARC 再設定、送信ドメイン評判再構築）が大きい。本 ADR は M1 priority 8 として 4 候補（Resend / AWS SES / SendGrid / Mailgun）+ 参考 2 候補（Postmark / Cloudflare Email Service）の公式ドキュメント調査結果（2026-04-25 時点）に基づき、第一・第二候補を確定する。実 API 接続検証は M2 早期に別タスクで実施。

## 決定

### 第一候補: AWS SES

採用理由:

- **本文を SES 側ストレージに保持しない**：SES は SMTP relay であり「送信した email body」を恒常的に保持するストレージ機能を持たない。Console にも「過去送信メールの本文」を閲覧する画面が無い
- **Event Destinations の SNS / Firehose / EventBridge payload に body が含まれない**：subject / headers / recipient は含まれるが、body 本文は含まれない。`subject` に管理 URL を入れない設計（後述）にすれば、event 経路の漏洩点は無い
- **Tokyo（ap-northeast-1）リージョン利用可能**：DKIM 設定・SMTP credential はリージョン単位なので、Cloud Run（東京）と同一リージョンで運用可能
- **MVP 規模（月 500〜2000 通）でコスト安**：$0.10 / 1000 通。EC2 同居時の無料枠は不適用だが、それでも月 $0.20 程度
- **Easy DKIM / SPF / DMARC が標準サポート**：Verified Identity に DKIM CNAME を追加するだけ
- **bounce / complaint は SNS topic 経由で確実に受信**：永続バウンスや苦情の自動抑制リスト管理が成熟

懸念点（実装方針で対応）:

- `subject` は SNS event payload に残る → **subject に管理 URL の token を含めない**（subject は固定文字列とし、本文に管理 URL を入れる）
- 初期は SES Sandbox 状態で本番ドメイン以外への送信が制限される → 「production access request」を申請する手順を運用ドキュメントに残す
- リージョン単位で Verified Identity / DKIM / suppression list が独立 → 検証用と本番で別アカウント or 同アカウント別リージョンを採用するか M2 早期に決定

### 第二候補: SendGrid (Twilio)

採用候補理由:

- **本文を「送信に必要な間だけ保持」と Twilio 公式が明記**：Twilio 公式 Data Retention and Deletion ドキュメントで「SendGrid only holds email message bodies for as long as it takes to send them」「SendGrid does not retain the contents of emails sent through its platform」と明示。recipient 等の個人データは最大 37 日（30 日 + 削除完了までの 7 日）
- **Email Activity Feed が body 非表示**：dashboard で event 履歴のみ表示、HTML / plain text 本文を表示する画面が無い
- **イベント retention は default 短期 + 30 日 add-on**：恒常的な dashboard 漏洩点が無い
- **無料枠 100 通/日（恒常）**：MVP の本番運用前検証に使える
- **大手・実績豊富、Go SDK / REST API 成熟**

懸念点:

- データセンターは米国のみ（リージョン選択不可、Twilio 公式記述）→ Cloud Run 東京 ↔ SendGrid 米国でのレイテンシ +100ms 程度
- Twilio の課金体系・契約が SES より複雑

### 不採用候補

#### Resend（不採用）

不採用理由:

- **Dashboard が本文を表示する仕様**：公式ドキュメント「Each email contains a Preview, Plain Text, and HTML version to visualize the content of your sent email in its various formats」と明示
- **sensitive data 非保存（"turn off message content storage"）は条件が厳しい**：Pro / Scale プラン + 1 ヶ月以上の subscriber + 3,000 通以上の送信実績 + bounce <5% + active website domain + **$50/月の add-on** + サポートに連絡して有効化。MVP 立ち上げ時は条件未達で adopt できない
- 実装は最も簡単・モダン API だが、本文漏洩リスクが第一候補基準を満たさない

将来的に MVP がスケールし sensitive data 非保存条件を満たす段階になれば再評価可。本 ADR では **MVP では不採用**。

#### Mailgun（不採用）

不採用理由:

- **本文を retention 期間中ストレージに保持**：Free / Foundation で 1 日、Scale で 7 日（公式 Help Center）。recovery / resending 機能の都合で本文 + headers が MIME ごと保持される
- 1 日が最短で 0 日にはできない
- Dashboard で本文を直接閲覧可能
- SES / SendGrid より本文漏洩リスクが大きい

#### Postmark（参考評価、不採用）

不採用理由:

- **45 日 default で本文も保存**：transactional email を売りにしているが、まさにその「45 日の Activity / Content Previews」が本ユースケースでは漏洩点
- Retention Add-on で 0-365 日に変更可能。理論的には 0 日設定で漏洩点を消せるが、Dashboard 表示そのものは残る
- 価格は MVP 規模ではやや高め

「retention 0 日」設定の柔軟性は評価できるが、**第一候補（SES）と比較して優位点が無い**。

### 比較参考: Cloudflare Email Service（送信機能、public beta）

VRC PhotoBook の Frontend は Cloudflare Workers（OpenNext）で動かす予定なので、同一プラットフォームでメール送信もできれば運用負荷を下げられる。ただし 2026-04-25 時点で:

- public beta（GA 前）
- daily sending limits は「Cloudflare のアカウント評価に基づき変動」と公式記載 — 安定運用の予測が立たない
- 新規アカウントは「verified email address」のみ送信可、paid プランで任意宛先
- transactional 専用

**MVP 採用は時期尚早**。Phase 2 で GA + 必要 throughput 達成・Frontend と同居メリットを再評価する候補として記録のみ留める。

### 比較表（2026-04-25 公式ドキュメント調査）

| 観点 | AWS SES（第一） | SendGrid（第二） | Resend（不採用） | Mailgun（不採用） | Postmark（不採用） | Cloudflare Email Service（参考） |
|------|:-------------:|:--------------:|:--------------:|:----------------:|:-----------------:|:-------------------------------:|
| **dashboard で本文表示** | ❌ なし | ❌ なし（events のみ） | ✅ あり（Preview/Plain/HTML） | ✅ あり | ✅ あり（45 日 default） | 不明（beta） |
| **本文の恒常保持** | なし | 送信中のみ | retention 期間中 | retention 期間中 | retention 期間中 | 不明（beta） |
| **本文 retention 0 日設定** | 該当不要 | 該当不要 | $50/月 add-on + 厳しい条件 | 不可（最短 1 日） | Add-on で 0 日可 | 不明 |
| **event payload に body** | ❌ なし（subject のみ） | ❌ なし | ❌ なし | ❌ なし | ❌ なし | 不明 |
| **リージョン選択** | ✅ Tokyo 含む複数 | 米国のみ | 米国 | EU / 米国 | 米国 | Cloudflare network |
| **無料枠 / MVP コスト** | $0.10/1000 通 | 100 通/日 恒常無料 | 月 100 通 / 日 100 通 無料 | 100 通/日 trial | 月 100 通 trial | 不明（変動） |
| **DKIM / SPF / DMARC** | Easy DKIM 標準 | 標準サポート | 標準サポート | 標準サポート | 標準サポート | Email Routing 経由 |
| **bounce / complaint** | SNS topic | Webhook | Webhook | Webhook | Webhook | Workers binding |
| **Go SDK / REST 成熟度** | aws-sdk-go-v2 v1 | 公式 SDK あり | 公式 SDK あり | 公式 SDK あり | 公式 SDK あり | Workers binding |
| **管理 URL 漏洩リスク評価** | 低 | 低 | 高 | 中 | 中（add-on で低化可） | 不明 |

(出典: 各 provider 公式ドキュメントおよびサポート記事、2026-04-25 調査)

### ManageUrlDelivery との関係

ManageUrlDelivery 集約（`docs/design/aggregates/manage-url-delivery/`）で扱う以下の仕様と整合させる。

- 管理 URL 控えメール送信要求は ManageUrlDelivery 集約で扱う
- `recipient_email` は短期保持（24h 後に NULL 化）
- `recipient_email_hash` は必要に応じて重複検出に使う
- `manage_url_token_version_at_send` を送信時点の snapshot として保持（送信後の token 再発行があっても、どの世代の token を送ったかを追跡可能）
- メール本文やログに管理 URL を出す場合はマスクや保持期間に注意する
- メール送信は Outbox 経由（`ManageUrlDeliveryRequested` イベント）で非同期実行する
- `DeliveryAttempt.providerMessageId` には provider の追跡 ID のみを保持（管理 URL は含めない）
- `DeliveryAttempt.errorSummary` には簡潔な失敗キー（例: `permanent_bounce`）のみを保持し、stack trace や本文・管理 URL を含めない

### ReissueManageUrl 時の方針

- 運営が申請者の連絡先を確認したうえで `cmd/ops/photobook_reissue_manage_url` を実行する（ADR-0002）
- `recipient_email` は運営が CLI 引数で手動入力する
- **過去の ManageUrlDelivery の recipient_email は再利用しない**（古いアドレスが悪用された場合の再送防止）
- 新しい管理 URL 送信が必要な場合は ManageUrlDelivery を新規作成する
- Photobook.manage_url_token 再発行、ModerationAction 記録、ManageUrlDelivery 作成、outbox_events INSERT は **同一トランザクション** で行う
- manage session の一括 revoke も同一 TX で実行する（ADR-0003）

## 検討した代替案

- **独自 SMTP 運用**（自前メールサーバー）: 送信 IP 評判がゼロから、到達率が極端に低い、SPF/DKIM/DMARC/PTR 全て自前管理、運用負荷過大。MVP 不可
- **管理 URL をメールで送らない**: UX が致命的に悪化。公開完了画面を閉じると管理 URL を再取得する手段がなくなり、ユーザーがフォトブックを自分で削除できなくなる。MVP の中核機能を壊す
- **QR コード画像化して本文に入れる**: 本文ログ保持の問題は同じ（画像 URL を生成したり、本文にデータ URI で埋めても QR は解読可能）。スクリーンショット経由の漏洩も同じ。セキュリティ上の改善ほぼなし
- **「再発行リクエストリンク」を送り、クリック後に管理 URL を表示する**: 再発行リンクそのものが一段秘匿層になるが、結局プロバイダログに「再発行リンク」が残る点は同じ。UX が複雑化し、ユーザーが管理 URL を保存するまでに 2 ステップ要求される。MVP には過剰。ただし Phase 2 で検討の価値はある
- **メール送信せず、画面表示のみで管理 URL を完結**: ユーザーが「後で」に失敗する率が高く、事実上の管理不能フォトブックが大量発生する。運営への問合せが増える。採用不可
- **Cloudflare Email Service（同一プラットフォーム）**: public beta + daily limits 変動 + 新規アカウントは verified address 制限のため、MVP では不採用。Phase 2 で GA 後に再評価

## 結果

### メリット

- 公式ドキュメント根拠に基づく選定で、最もセンシティブな「管理 URL の本文漏洩」リスクを構造的に低減
- AWS SES の Tokyo リージョン利用で、Cloud Run（東京）と同一リージョン構成
- ManageUrlDelivery 集約の snapshot 設計と組み合わせ、送信時 token 世代を追跡可能
- `EmailSender` ポート抽象化により、将来 SES → SendGrid 切替が必要になっても M6 ワーカー実装の影響範囲に閉じる

### デメリット

- AWS SES の Sandbox 解除には production access request が必要（数営業日）。M2 早期に申請しておく必要がある
- bounce / complaint の SNS topic 受信エンドポイントを Cloud Run 側に立てる必要がある（または Pub/Sub 経由）
- 実送信 PoC（1 通テスト + bounce 受信）が未実施。本 ADR の Accepted は「provider 選定として」であり、実送信検証は M2 早期に別タスク

### 後続作業への影響

- M2 早期: AWS SES の sandbox 解除申請、独自ドメインの DKIM/SPF/DMARC 設定、production access request、テスト送信 PoC、bounce/complaint 受信エンドポイント
- M4: PublishPhotobook UseCase でメール送信トリガ（Outbox 経由）を組む。`EmailSender` ポートは provider 中立で確定可能
- M6: `manage-url-mailer` ワーカーで AWS SES SDK（aws-sdk-go-v2）を呼ぶ。SES クライアントは R2 接続クライアントと同様の構造で実装
- 運用: SES suppression list の手動メンテナンス手順、Secret Manager 経由の SMTP credential / API Key 注入、Safari / iPhone Safari でのメール受信→管理 URL アクセスの実機検証

## リスク

| リスク | 影響度 | 緩和策 |
|------|:----:|------|
| **SNS event payload の subject に管理 URL を入れてしまう** | 高 | subject は固定文字列（例: 「VRC PhotoBook 管理 URL のご案内」）とし、URL は本文にのみ含める。実装レビューで subject テンプレートを必ず確認 |
| **本文に含まれる管理 URL の漏洩**（provider 内部の運用者・パートナーアクセス） | 中 | SES は本文を恒常保持しないため構造的に低減。本文テンプレートは管理 URL を 1 箇所のみ含み、フッタなど不要な情報を最小化 |
| **bounce / complaint 受信漏れによる送信ドメイン評判悪化** | 高 | M6 で SNS topic → Cloud Run / Pub/Sub の冗長化、suppression list 自動更新を実装 |
| **誤送信時の取り消し不可**（メール送信完了後は撤回手段なし） | 中 | Outbox 投入前に `recipient_email` の RFC 5322 簡易 validation、ManageUrlDelivery で送信履歴を残し問題発覚時の連絡導線を確保 |
| **管理 URL の有効期限と再発行**（旧 URL のメール内残留） | 低〜中 | manage_url_token_version で世代管理、ReissueManageUrl で旧 token を即無効化（ADR-0003） |
| **provider 側のログ保持仕様変更**（公式 ToS / Privacy Policy 改訂） | 低〜中 | M2 早期に provider 公式 status 確認手順をドキュメント化、定期レビュー（年 1 回） |
| **AWS SES Sandbox 状態のまま本番運用** | 高 | M2 早期に production access request を申請、申請完了まで MVP リリース不可とする運用判断 |

## 実装方針

### 1. EmailSender ポート抽象化（必須）

```
backend/manage-url-delivery/
├── domain/
│   └── service/
│       └── email_sender.go      # interface EmailSender { Send(...) (MessageID, error) }
└── infrastructure/
    └── email/
        └── ses_sender.go        # AWS SES 実装（M6）
```

- domain / application 層では `EmailSender` interface のみを使う
- provider 実装は infra 層に閉じ、ApplicationService からは provider に依存しない
- テストでは Fake / Mock 実装を差し込む

### 2. 本文・管理 URL の取り扱い

- 本文テンプレートは管理 URL を **最小限 1 箇所**だけ含める（フッタや署名に重複させない）
- subject に管理 URL の token を含めない（SNS event payload に残るため）
- 本文以外（subject / preheader / from name）は固定文字列
- アプリログには `provider_message_id` / `delivery_id` / `photobook_id` のみ残す。`recipient_email` / 管理 URL / token は出さない

### 3. Outbox payload に管理 URL を入れない

- `ManageUrlDeliveryRequested` の outbox payload には `delivery_id` / `photobook_id` / `manage_url_token_version_at_send` のみを記録
- 実送信時にハンドラが DB から最新の `Photobook.manage_url_token` を引いて URL を組み立てる
- ハンドラのメモリ上には URL が一時的に存在するが、送信完了後は破棄
- これにより outbox_events テーブルの row dump に管理 URL が出ることを防ぐ

### 4. ManageUrlDelivery の永続化方針

- 送信成功 / 失敗確定 / expire 到達で `recipient_email` を NULL 化（24h 後）
- `DeliveryAttempt.providerMessageId` は provider の追跡 ID のみ
- `DeliveryAttempt.errorSummary` は簡潔な失敗キー（`permanent_bounce` / `expired_during_retry` 等）のみ。stack trace や本文を含めない
- 監査用に `manage_url_token_version_at_send` を保持（送信後の token 再発行があっても、どの世代の token を送ったかを追跡可能）

### 5. provider 切替の影響範囲

- `EmailSender` interface に閉じれば、provider 切替は infra 実装の差し替えのみ
- DKIM / SPF / DMARC は DNS の問題で provider 切替時に再設定が必要 → 切替時の TODO リストを M6 運用ドキュメントに明記

## M2 以降の TODO

| タスク | フェーズ | 内容 |
|------|------|------|
| AWS SES sandbox 解除申請 | M2 早期 | production access request、申請内容のテンプレート化 |
| 独自ドメインの DKIM / SPF / DMARC 設定 | M2 早期 | DNS 設定 + Easy DKIM verify |
| テスト送信 PoC | M2 早期 | 実 API キー（短期）+ 1 通の自宅アドレス送信、SNS event 受信テスト |
| bounce / complaint webhook | M3 / M6 | SNS topic 受信エンドポイント（Cloud Run）、suppression list 自動更新 |
| provider のログ保持の最終確認 | M2 早期 | サポート問い合わせで「dashboard / API / event payload に body が残らないこと」の書面確認を取る |
| Secret Manager 注入 | M2〜M6 | AWS access key / SMTP credential を Secret Manager に格納、Cloud Run 環境変数で注入 |
| Cloud Run からの送信テスト | M5〜M6 | 実環境からの送信、レイテンシ計測、bounce 受信動作確認 |
| Safari / iPhone Safari での受信 → 管理 URL アクセス確認 | M5〜M6 | メール本文の管理 URL を iPhone Safari で開き、token → session 交換が成立することを確認 |

## 関連ドキュメント

- `docs/spec/vrc_photobook_business_knowledge_v4.md` §3.5, §6.7, §7.5
- `docs/design/aggregates/manage-url-delivery/ドメイン設計.md`
- `docs/design/aggregates/manage-url-delivery/データモデル設計.md`
- `docs/design/cross-cutting/outbox.md`（`ManageUrlDeliveryRequested` イベント）
- `ADR-0001 技術スタック`
- `ADR-0002 運営操作方式`（ReissueManageUrl は cmd/ops で実行）
- `ADR-0003 フロントエンド認可フロー`（ReissueManageUrl 時の manage session 一括 revoke と同一 TX）
- `.agents/rules/safari-verification.md`（メール受信後の管理 URL アクセスを Safari でも確認）

## 参照した公式ドキュメント（2026-04-25）

- AWS SES: `Monitor email sending using Amazon SES event publishing` / `Examples of event data that Amazon SES publishes to Amazon SNS` / `Regional availability` （SNS event payload に body が含まれず subject のみ含まれることを確認）
- SendGrid (Twilio): `Email Activity Feed` / `Data Retention and Deletion in Twilio Products`（"only holds email message bodies for as long as it takes to send them" を Twilio 公式で確認）
- Resend: `Managing Emails`（Preview/Plain/HTML 表示）/ `How do I ensure sensitive data isn't stored on Resend?`（Pro/Scale + 厳しい条件 + $50/月 add-on）
- Mailgun: `Adjusting a domain's message retention settings` / `Logs`（Free/Foundation 1 日 / Scale 7 日 retention）
- Postmark: `45 Days of Email Activity and Content Previews` / `Retention Add-on`（45 日 default、0-365 日可変）
- Cloudflare Email Service: `Send emails from Workers` / `Limits` / public beta 告知（daily limits 変動、新規 verified address のみ）
