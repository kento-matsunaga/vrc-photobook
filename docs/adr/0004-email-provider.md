# ADR-0004 メールプロバイダ選定

## ステータス

**Superseded by [ADR-0006](./0006-email-provider-and-manage-url-delivery.md)（2026-04-28）**

> 2026-04-28 時点で **SendGrid Japan は個人 / 個人事業主 / 任意団体は契約不可**であることが
> 判明（SendGrid 公式 FAQ「個人でも契約可能ですか？」）。VRC PhotoBook の運用主体は個人の
> ため SendGrid は採用不可。同時に AWS SES の production access 申請も不通過のままであり、
> 本 ADR の第一候補・第二候補が共に MVP では運用不可となった。
>
> ADR-0006 で **MVP のメール送信機能を必須要件から外す**判断（Complete 画面で 1 度だけ
> 表示する方式を MVP 標準）を行ったため、本 ADR は Superseded。
>
> **本 ADR は過去の選定経緯としてのみ参照**してください。MVP の方針は ADR-0006、
> 後続のメール Provider 候補も ADR-0006 §4.4 を参照。

---

## 旧ステータス（2026-04-25 時点、参考）

**Accepted（MVP プロバイダ選定として、2026-04-25 再選定）**

MVP では **SendGrid を第一候補、Mailgun を第二候補**として採用する。

実送信 PoC（実 API キー発行 + 1 通テスト送信 + bounce/complaint webhook 受信）は M2 早期に別タスクで実施する。本 ADR は「MVP の第一候補・第二候補・不採用」と「実装方針（EmailSender ポート抽象化、本文最小化、Outbox payload に管理 URL を入れない等）」を Accepted 化する。

## 作成日
2026-04-25

## 最終更新

- 2026-04-25（M1 priority 8、4 候補比較完了 → Accepted 化、当初は AWS SES 第一候補）
- **2026-04-25（再選定）**: AWS SES が **Amazon 側の申請で不通過となり MVP では運用不可**となったため、ADR-0004 を再オープンし、第一候補を SendGrid に昇格、Mailgun を第二候補に再評価

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

### 当初の AWS SES 第一候補からの変更経緯（2026-04-25）

当初の Accepted 化（同日）では、本文を SES 側ストレージに保持しない構造的優位性 + Tokyo リージョン + MVP コスト最安の観点から **AWS SES を第一候補**としていた。しかし同日中に **Amazon 側の SES 利用申請が不通過**となり、MVP の実装スケジュールに乗せられなくなった。

これは「技術的な不採用」ではなく「アカウント／運用上の利用不可」による不採用である。将来的な再申請の可能性は残すが、MVP のクリティカルパスからは外す。

そのため本 ADR を再オープンし、第二候補だった SendGrid を第一候補に昇格、第三勢力だった Mailgun を第二候補に再評価する。

## 決定

### 第一候補: SendGrid (Twilio)

採用理由（公式記述に基づく）:

- **本文を保存しない方針が公式で明示**: SendGrid 公式 Support「How do I find the body/contents of my emails?」に
  - 「Twilio SendGrid does not offer inbox services, nor do we store email content of emails processed and sent through our servers」
  - 「To maintain the highest standards of security and data privacy, Twilio SendGrid does not retain the contents of emails sent through our platform」
  - と明記。本文は配送に必要な間だけしか保持されない
- **Email Activity Feed は events のみ表示**: dashboard で本文 (HTML / plain text) を表示する画面が無い
- **イベント retention の標準保持は短期 + 30 日 add-on**: 本文は最初から保持されないため、events だけ拡張しても本文漏洩点が増えない
- **無料枠 100 通/日（恒常）**: MVP 立ち上げから本番初期まで無料で運用可能、SDK / REST API 成熟
- **Go / Cloud Run から呼びやすい**: 公式 Go SDK、または素の REST API（API Key を Bearer Header）で組める
- **EmailSender ポート抽象化と相性が良い**: provider 中立な Send(toEmail, subject, body) インターフェイスにマッピングしやすい

懸念点（実装方針で対応）:

- **データセンターは米国のみ**（リージョン選択不可）→ Cloud Run 東京 ↔ SendGrid 米国でレイテンシ +100ms 程度。Outbox 経由の非同期送信なので体感影響は小さい
- recipient メールアドレス・送信元・件名・カスタム引数・イベント履歴などのメタデータは保持される → 件名 / カスタム引数 / categories / metadata に管理 URL や token を含めない設計で対応
- **Twilio の課金体系・契約**は MVP 規模では問題ないが、Premier 系の add-on や Volume Pricing は事前確認が必要

### 第二候補: Mailgun

採用候補理由（SendGrid が運用上使えない場合のフォールバック）:

- **Domain ごとに retention 0 days を選択可能**: Mailgun Help Center「Adjusting a domain's message retention settings」に
  - 「In the General section and by the Message retention setting, you can click the Edit button and select the desired value, which includes disabling retention by setting the value to 0」
  - と明記。0 day を選択した場合、本文 (MIME) は保存されない設計に変更できる
- 設定変更は forward only に反映される（過去メッセージには影響しない）ので、初期設定時に 0 day を選んでおけば運用上問題なし
- 公式 Go SDK あり、REST API も成熟、SendGrid 同等の手軽さで導入可能
- DKIM / SPF / DMARC 標準サポート

懸念点:

- 0 day 設定の挙動（dashboard 表示・Webhook payload・Email Logs ファセット）は **導入前に実アカウントで実機確認必須**。プラン (Free / Foundation / Scale) ごとに retention の最大値が違う仕様があるため、0 day 設定が全プランで完全に有効かどうか公式記述だけでは断定しづらい
- SendGrid の「公式が本文非保持を明言」のレベルには達していない（自分で 0 day 設定する責任を負う形）
- 採用は SendGrid が運用 / 料金面で使えない場合のフォールバックとして位置付ける

### 不採用候補

#### AWS SES（運用不可、技術不採用ではない）

不採用理由:

- **Amazon 側の SES 利用申請が不通過となり、MVP では採用不可**（2026-04-25 確認）
- 当初は第一候補だった（本文を SES 側ストレージに保持しない / SNS event payload に body が含まれない / Tokyo リージョン利用可 / MVP コスト最安）が、申請落ちにより MVP スケジュールに乗らない
- **本判断は技術的な不採用ではなく、アカウント／運用上の利用不可による不採用**
- 将来的な再申請の可能性は残すが、MVP のクリティカルパスには含めない。Phase 2 以降で運用評価のうえ再申請を検討してもよい

#### Resend（不採用）

不採用理由:

- **Dashboard が本文を表示する仕様**：公式ドキュメント「Each email contains a Preview, Plain Text, and HTML version to visualize the content of your sent email in its various formats」と明示
- **sensitive data 非保存（"turn off message content storage"）は条件が厳しい**：Pro / Scale プラン + 1 ヶ月以上の subscriber + 3,000 通以上の送信実績 + bounce <5% + active website domain + **$50/月の add-on** + サポートに連絡して有効化。MVP 立ち上げ時は条件未達で採用不可
- 実装は最も簡単・モダン API だが、本文漏洩リスクが第一候補基準を満たさない

将来的に MVP がスケールし sensitive data 非保存条件を満たす段階になれば再評価可。本 ADR では **MVP では不採用**。

#### Postmark（不採用）

不採用理由:

- **45 日 default で本文も保存**：transactional email を売りにしているが、まさにその「45 日の Activity / Content Previews」が本ユースケースでは漏洩点
- Retention Add-on で 0-365 日に変更可能。理論的には 0 日設定で漏洩点を消せるが、それでも Mailgun と比較して優位性なし（料金体系がより複雑）

#### Cloudflare Email Service（参考のみ、不採用）

不採用理由:

- public beta（GA 前）
- daily sending limits は「Cloudflare のアカウント評価に基づき変動」と公式記載 — 安定運用の予測が立たない
- 新規アカウントは「verified email address」のみ送信可、paid プランで任意宛先
- transactional 専用

VRC PhotoBook の Frontend は Cloudflare Workers で動かす予定なので、同一プラットフォームのメール送信は将来的に魅力的だが、**MVP 採用は時期尚早**。Phase 2 で GA + 必要 throughput 達成・Frontend と同居メリットを再評価する候補として記録のみ留める。

### 比較表（2026-04-25 公式ドキュメント調査）

| 観点 | SendGrid（第一） | Mailgun（第二） | AWS SES（運用不可） | Resend（不採用） | Postmark（不採用） | Cloudflare Email Service（参考） |
|------|:--------------:|:--------------:|:-----------------:|:--------------:|:-----------------:|:-------------------------------:|
| **dashboard で本文表示** | ❌ なし（events のみ） | ❌ なし（0 day 設定時） | ❌ なし（参考） | ✅ あり（Preview/Plain/HTML） | ✅ あり（45 日 default） | 不明（beta） |
| **本文の恒常保持** | しない（公式明言） | 0 day 設定で保持しない | しない（参考） | retention 期間中 | retention 期間中 | 不明（beta） |
| **本文 retention 0 設定** | 該当不要（最初から保存しない） | ✅ Domain 設定で 0 day 選択可 | 該当不要 | $50/月 add-on + 厳しい条件 | Add-on で 0 日可 | 不明 |
| **公式の「本文非保持」明言** | ✅ あり（Twilio 公式） | △ 0 day 設定で実効的に非保持 | ✅ 構造的に保持なし | ❌ | ❌ | 不明 |
| **event payload に body** | ❌ なし | ❌ なし | ❌ なし | ❌ なし | ❌ なし | 不明 |
| **リージョン選択** | 米国のみ | EU / 米国 | （Tokyo 含む複数） | 米国 | 米国 | Cloudflare network |
| **無料枠 / MVP コスト** | 100 通/日 恒常無料 | 100 通/日 trial（プランで変動） | $0.10/1000 通（運用不可） | 月 100 通 / 日 100 通 無料 | 月 100 通 trial | 不明（変動） |
| **DKIM / SPF / DMARC** | 標準サポート | 標準サポート | Easy DKIM 標準（参考） | 標準サポート | 標準サポート | Email Routing 経由 |
| **bounce / complaint** | Webhook | Webhook | SNS topic（参考） | Webhook | Webhook | Workers binding |
| **Go SDK / REST 成熟度** | 公式 SDK あり | 公式 SDK あり | aws-sdk-go-v2 v1（参考） | 公式 SDK あり | 公式 SDK あり | Workers binding |
| **アカウント運用上の利用可否** | ✅ 可能 | ✅ 可能 | ❌ 申請不通過 | ✅ 可能 | ✅ 可能 | beta |
| **管理 URL 漏洩リスク評価** | 低 | 低（0 day 設定時） | 低（参考） | 高 | 中（add-on で低化可） | 不明 |

(出典: 各 provider 公式ドキュメントおよびサポート記事、2026-04-25 調査)

### ManageUrlDelivery との関係

ManageUrlDelivery 集約（`docs/design/aggregates/manage-url-delivery/`）で扱う以下の仕様と整合させる。

- 管理 URL 控えメール送信要求は ManageUrlDelivery 集約で扱う
- `recipient_email` は短期保持（24h 後に NULL 化）
- `recipient_email_hash` は必要に応じて重複検出に使う
- `manage_url_token_version_at_send` を送信時点の snapshot として保持（送信後の token 再発行があっても、どの世代の token を送ったかを追跡可能）
- メール本文やログに管理 URL を出す場合はマスクや保持期間に注意する
- メール送信は Outbox 経由（`ManageUrlDeliveryRequested` イベント）で非同期実行する
- `DeliveryAttempt.providerMessageId` には provider の追跡 ID（SendGrid 採用時は `X-Message-Id` ヘッダ値）のみを保持
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
- **AWS SES（当初第一候補）**: 公式仕様上は最も望ましいが、Amazon 側の利用申請で不通過。MVP では運用不可

## 結果

### メリット

- 「本文を保持しない」公式明言（SendGrid）と「0 day retention 設定可能」（Mailgun）の二段構えで、管理 URL の本文漏洩リスクを構造的に低減
- 申請落ちなど運用上のリスクに対するフォールバック（第一→第二の切替手順）を ADR で明記
- ManageUrlDelivery 集約の snapshot 設計と組み合わせ、送信時 token 世代を追跡可能
- `EmailSender` ポート抽象化により、SendGrid → Mailgun 切替が必要になっても M6 ワーカー実装の影響範囲に閉じる

### デメリット

- SendGrid のデータセンターが米国のみ → Cloud Run 東京 ↔ SendGrid 米国でのレイテンシ +100ms 程度
- 実送信 PoC（1 通テスト + bounce 受信）が未実施。本 ADR の Accepted は「provider 選定として」であり、実送信検証は M2 早期に別タスク
- AWS SES が将来再申請可能となっても、本実装の DKIM / DMARC 設定を SendGrid 用に行った後では、再切替コスト（DNS 変更 + 送信ドメイン評判再構築）が発生

### 後続作業への影響

- M2 早期: SendGrid アカウント作成 + 独自ドメインの DKIM/SPF/DMARC 設定 + API Key 発行（Secret Manager 注入）+ テスト送信 PoC + bounce/complaint webhook 受信エンドポイント
- M4: PublishPhotobook UseCase でメール送信トリガ（Outbox 経由）を組む。`EmailSender` ポートは provider 中立で確定可能
- M6: `manage-url-mailer` ワーカーで SendGrid SDK or REST API を呼ぶ
- 運用: SendGrid suppression list の手動メンテナンス手順、Safari / iPhone Safari でのメール受信→管理 URL アクセスの実機検証

## リスク

| リスク | 影響度 | 緩和策 |
|------|:----:|------|
| **件名 / カスタム引数 / categories / metadata に管理 URL を入れてしまう** | 高 | 件名は固定文字列（例: 「VRC PhotoBook 管理 URL のご案内」）とし、URL は本文にのみ含める。`X-SMTPAPI` / `personalizations` / `categories` / `custom_args` / `unique_args` も使わない。実装レビューで送信ペイロードを必ず確認 |
| **本文に含まれる管理 URL の漏洩**（provider 内部の運用者・パートナーアクセス） | 中 | SendGrid は本文を恒常保持しない方針を公式明言。本文テンプレートは管理 URL を 1 箇所のみ含み、フッタなど不要な情報を最小化 |
| **bounce / complaint 受信漏れによる送信ドメイン評判悪化** | 高 | M6 で webhook 受信エンドポイントの冗長化、suppression list 自動更新を実装 |
| **誤送信時の取り消し不可**（メール送信完了後は撤回手段なし） | 中 | Outbox 投入前に `recipient_email` の RFC 5322 簡易 validation、ManageUrlDelivery で送信履歴を残し問題発覚時の連絡導線を確保 |
| **管理 URL の有効期限と再発行**（旧 URL のメール内残留） | 低〜中 | manage_url_token_version で世代管理、ReissueManageUrl で旧 token を即無効化（ADR-0003） |
| **provider 側のログ保持仕様変更**（公式 ToS / Privacy Policy 改訂） | 低〜中 | M2 早期に provider 公式 status 確認手順をドキュメント化、定期レビュー（年 1 回） |
| **SendGrid アカウント審査の遅延 / 申請落ち**（AWS SES と同様の事象が SendGrid でも起きうる） | 中〜高 | M2 早期にアカウント審査・送信ドメイン認証を進めて運用可否を早期確認。落ちた場合は第二候補 Mailgun に切替（DKIM/DMARC 再設定が発生する） |
| **Mailgun 0 day retention 設定の挙動** | 中 | 第二候補に切替する場合、導入時に実アカウントで 0 day 設定後の dashboard 表示・Webhook payload・Logs を実機確認 |

## 実装方針

### 1. EmailSender ポート抽象化（必須）

```
backend/manage-url-delivery/
├── domain/
│   └── service/
│       └── email_sender.go      # interface EmailSender { Send(...) (MessageID, error) }
└── infrastructure/
    └── email/
        ├── sendgrid_sender.go   # SendGrid 実装（M6、第一候補）
        └── mailgun_sender.go    # Mailgun 実装（フォールバック発動時、または将来差し替え）
```

- domain / application 層では `EmailSender` interface のみを使う
- provider 実装は infra 層に閉じ、ApplicationService からは provider に依存しない
- テストでは Fake / Mock 実装を差し込む
- 切替が必要になった場合（SendGrid 申請落ち等）は `manage-url-mailer` ワーカーで使う具象を `mailgun_sender.go` に差し替えるだけで済む構造にする

### 2. 本文・件名・メタデータの取り扱い（SendGrid 採用時の安全ルール）

- 本文テンプレートは管理 URL を **最小限 1 箇所**だけ含める（フッタや署名に重複させない）
- **件名（subject）に管理 URL の token を含めない**。件名は固定文字列のみ
- **SendGrid のカスタム引数 / categories / metadata（`custom_args` / `unique_args` / `categories` / `personalizations[].custom_args`）に管理 URL や token を入れない**
- 本文以外（subject / preheader / from name）はすべて固定文字列
- アプリログには `provider_message_id`（SendGrid の `X-Message-Id` ヘッダ）/ `delivery_id` / `photobook_id` のみ残す。`recipient_email` / 管理 URL / token は出さない
- provider のレスポンスやエラー詳細をそのままログに出さない（必要時は分類キーに集約）

### 3. Outbox payload に管理 URL を入れない

- `ManageUrlDeliveryRequested` の outbox payload には `delivery_id` / `photobook_id` / `manage_url_token_version_at_send` のみを記録
- 実送信時にハンドラが DB から最新の `Photobook.manage_url_token` を引いて URL を組み立てる
- ハンドラのメモリ上には URL が一時的に存在するが、送信完了後は破棄
- これにより `outbox_events` テーブルの row dump に管理 URL が出ることを防ぐ

### 4. ManageUrlDelivery の永続化方針

- 送信成功 / 失敗確定 / expire 到達で `recipient_email` を NULL 化（24h 後）
- `DeliveryAttempt.providerMessageId` は SendGrid の `X-Message-Id` のみ
- `DeliveryAttempt.errorSummary` は簡潔な失敗キー（`permanent_bounce` / `expired_during_retry` 等）のみ。stack trace や本文を含めない
- 監査用に `manage_url_token_version_at_send` を保持（送信後の token 再発行があっても、どの世代の token を送ったかを追跡可能）

### 5. provider 切替の影響範囲

- `EmailSender` interface に閉じれば、provider 切替は infra 実装の差し替えのみ
- DKIM / SPF / DMARC は DNS の問題で provider 切替時に再設定が必要 → 切替時の TODO リストを M6 運用ドキュメントに明記
- SendGrid → Mailgun への切替手順テンプレートを M2 で準備しておく（運用上のリスクヘッジ）

## M2 以降の TODO

| タスク | フェーズ | 内容 |
|------|------|------|
| **SendGrid アカウント作成 / 審査通過** | M2 早期 | アカウント作成、ドメイン認証、審査が通ることを早期確認（運用上の利用不可リスク回避） |
| 独自ドメインの DKIM / SPF / DMARC 設定 | M2 早期 | DNS 設定、SendGrid の sender domain authentication |
| API Key 発行 + Secret Manager 注入 | M2 早期 | scoped API Key（Mail Send 専用、IP allowlist 検討）、Cloud Run 環境変数で Secret Manager から注入 |
| テスト送信 PoC | M2 早期 | 実 API キー（短期）+ 1 通の自宅アドレス送信、Email Activity Feed / dashboard / API ログに本文や管理 URL が出ないことを実機確認 |
| Webhook payload に本文が含まれないか確認 | M2 早期 | bounce / complaint event の payload を実機受信で確認 |
| bounce / complaint webhook | M3 / M6 | 受信エンドポイント、suppression list 自動更新 |
| **SendGrid → Mailgun 切替判断 / 手順テンプレート** | M2 早期 | SendGrid が運用 / 料金面で使えなかった場合の切替手順、DKIM/DMARC 再設定の TODO リスト |
| Mailgun を選んだ場合の retention 0 day 確認 | M2（条件付き） | Mailgun 採用時のみ、Domain settings で retention 0 day 設定後の dashboard / Webhook / Logs に本文が出ないことを実機確認 |
| Cloud Run からの送信テスト | M5〜M6 | 実環境からの送信、レイテンシ計測、bounce 受信動作確認 |
| **Safari / iPhone Safari での受信 → 管理 URL アクセス確認** | M5〜M6 | メール本文の管理 URL を iPhone Safari で開き、token → session 交換が成立することを確認（`.agents/rules/safari-verification.md` 準拠） |
| AWS SES 再申請の検討 | Phase 2 | MVP 運用が安定してから、コスト最適化と Tokyo region 同居の観点で再申請を検討 |

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

- **SendGrid (Twilio)**:
  - `How do I find the body/contents of my emails?`（Twilio SendGrid 公式 Support）— "Twilio SendGrid does not offer inbox services, nor do we store email content of emails processed and sent through our servers" / "To maintain the highest standards of security and data privacy, Twilio SendGrid does not retain the contents of emails sent through our platform"
  - `Email Activity Feed`（events のみ表示、body 非表示）
  - `Data Retention and Deletion in Twilio Products`（Twilio 公式 Support）
- **Mailgun**:
  - `Adjusting a domain's message retention settings`（Mailgun Help Center）— "you can click the Edit button and select the desired value, which includes disabling retention by setting the value to 0"
  - `Logs`（Mailgun Help Center、retention の最大値はプラン依存）
- **Resend**:
  - `Managing Emails`（Preview/Plain/HTML 表示）
  - `How do I ensure sensitive data isn't stored on Resend?`（Pro/Scale + 厳しい条件 + $50/月 add-on）
- **AWS SES**（参考、運用不可）:
  - `Monitor email sending using Amazon SES event publishing`
  - `Examples of event data that Amazon SES publishes to Amazon SNS`
  - `Regions endpoints quotas`
- **Postmark**:
  - `45 Days of Email Activity and Content Previews`
  - `Retention Add-on`
- **Cloudflare Email Service**:
  - `Send emails from Workers`
  - `Limits` / public beta 告知
