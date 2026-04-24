# ADR-0004 メールプロバイダ選定

## ステータス
Proposed

## 作成日
2026-04-25

## 最終更新
2026-04-25

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

未検証のまま Accepted にすると、事後にプロバイダ変更が発生した場合の移行コスト（DNS 変更、dkim/dmarc 再設定、送信ドメイン評判再構築）が大きい。そのため本 ADR は **Proposed** とし、検証完了後に Accepted へ昇格する。

## 決定

### 全体方針

- **MVP 時点では暫定第一候補を置かない**。本文ログ保持・API ログ・送信履歴 UI の検証を完了したうえで、最適なプロバイダを 1 つに絞って Accepted 化する。
- 候補は Resend / AWS SES / SendGrid / Mailgun の 4 つとし、本 ADR 内で比較表を埋めて根拠を残す。
- 検証中の MVP 実装では、`EmailSender` ポートを定義して具象プロバイダ実装を差し替え可能にしておく（M4 でポート確定、M6 で最終プロバイダ接続）。

### 候補プロバイダ

| 候補 | 備考 |
|------|------|
| Resend | 実装が簡単、モダン API、日本語到達性要確認 |
| AWS SES | コスト低、到達性高、SES ログが CloudWatch に流れる点の確認必須 |
| SendGrid | 比較のため評価、本文ログ保持仕様を確認 |
| Mailgun | 比較のため評価、本文ログ保持仕様を確認 |

### 評価軸

各プロバイダについて以下を確認し、比較表を本 ADR に埋める。

- メール本文が管理画面（送信履歴 UI）に保存されるか
- メール本文に含まれる管理 URL がプロバイダログに残るか
- API リクエストログに本文が残るか
- ログ保持期間を制御できるか（最短化できるか）
- 日本語メールの到達性（Gmail / Yahoo! JP / iCloud / Outlook）
- 送信失敗時の扱い（バウンスコード、Webhook の粒度、リトライ判定）
- Webhook 対応（delivered / bounced / complained）
- 価格（MVP 規模想定 月 500〜2000 通）
- 実装の簡単さ（Go SDK の成熟度、REST で十分か）
- Cloud Run / Go との相性（IAM 連携、IAM Workload Identity か API Key か）
- バウンス / 苦情処理（自動抑制リスト管理）
- ドメイン認証の容易さ（SPF / DKIM / DMARC 設定）

### ManageUrlDelivery との関係

ManageUrlDelivery 集約（`docs/design/aggregates/manage-url-delivery/`）で扱う以下の仕様と整合させる。

- 管理 URL 控えメール送信要求は ManageUrlDelivery 集約で扱う
- `recipient_email` は短期保持（24h 後に NULL 化）
- `recipient_email_hash` は必要に応じて重複検出に使う
- `manage_url_token_version_at_send` を送信時点の snapshot として保持（送信後の token 再発行があっても、どの世代の token を送ったかを追跡可能）
- メール本文やログに管理 URL を出す場合はマスクや保持期間に注意する
- メール送信は Outbox 経由（`ManageUrlDeliveryRequested` イベント）で非同期実行する

### ReissueManageUrl 時の方針

- 運営が申請者の連絡先を確認したうえで `cmd/ops/photobook_reissue_manage_url` を実行する（ADR-0002）
- `recipient_email` は運営が CLI 引数で手動入力する
- **過去の ManageUrlDelivery の recipient_email は再利用しない**（古いアドレスが悪用された場合の再送防止）
- 新しい管理 URL 送信が必要な場合は ManageUrlDelivery を新規作成する
- Photobook.manage_url_token 再発行、ModerationAction 記録、ManageUrlDelivery 作成、outbox_events INSERT は **同一トランザクション** で行う
- manage session の一括 revoke も同一 TX で実行する（ADR-0003）

### Proposed → Accepted への移行条件

以下を検証し、ADR 本文「検証結果」セクション（新設）に結果を追記したうえで、ステータスを Accepted に変更する。

- Resend の送信履歴 UI に本文が表示されるか（設定で非表示・短期化できるか）
- Resend の API ログに本文が残るか、どれくらいの期間か
- AWS SES の送信ログに本文が残るか（SES Event Destinations の設定で本文除外できるか）
- CloudWatch 等に本文が出ない構成にできるか
- SendGrid / Mailgun の本文ログ保持仕様
- バウンス / 苦情処理をどう扱うか（SES は SNS トピック、Resend は Webhook、抑制リスト自動更新の可否）
- 本文ログ保持を最短化できる設定の存在
- 管理 URL を本文に含める運用が許容可能か（プロバイダ側 ToS / Privacy 含む）

## 検討した代替案

- **独自 SMTP 運用**（自前メールサーバー）: 送信 IP 評判がゼロから、到達率が極端に低い、SPF/DKIM/DMARC/PTR 全て自前管理、運用負荷過大。MVP 不可。
- **管理 URL をメールで送らない**: UX が致命的に悪化。公開完了画面を閉じると管理 URL を再取得する手段がなくなり、ユーザーがフォトブックを自分で削除できなくなる。MVP の中核機能を壊す。
- **QR コード画像化して本文に入れる**: 本文ログ保持の問題は同じ（画像 URL を生成したり、本文にデータ URI で埋めても QR は解読可能）。スクリーンショット経由の漏洩も同じ。セキュリティ上の改善ほぼなし。
- **「再発行リクエストリンク」を送り、クリック後に管理 URL を表示する**: 再発行リンクそのものが一段秘匿層になるが、結局プロバイダログに「再発行リンク」が残る点は同じ。UX が複雑化し、ユーザーが管理 URL を保存するまでに 2 ステップ要求される。MVP には過剰。ただし Phase 2 で検討の価値はある。
- **メール送信せず、画面表示のみで管理 URL を完結**: ユーザーが「後で」に失敗する率が高く、事実上の管理不能フォトブックが大量発生する。運営への問合せが増える。採用不可。

## 結果

### メリット

- 決め打ちせず、検証に基づく選定ができる
- 管理 URL という最もセンシティブな情報の漏洩点を事前に潰せる
- ManageUrlDelivery 集約の snapshot 設計と組み合わせて、送信時の token 世代を追跡できる
- `EmailSender` ポートで抽象化することで、プロバイダ変更時の影響範囲を M6 ワーカーに限定できる

### デメリット

- M1 の段階ではメール送信機能が未確定で、M4 の PublishPhotobook ユースケースでメール送信のスタブ実装が必要になる
- 検証期間中に Accepted 化できないと、M6（非同期基盤）の実装がブロックされる可能性がある
- プロバイダ変更時は DNS / DKIM / DMARC 再設定・送信ドメイン評判の再構築が発生するため、初期選定での妥協を避ける必要がある

### 後続作業への影響

- M4: PublishPhotobook UseCase でメール送信トリガ（Outbox 経由）を組む。プロバイダ未確定でもインターフェイス（`EmailSender` ポート）は確定できる。
- M6: `manage-url-mailer` ワーカーでプロバイダ SDK を呼ぶ。Accepted 化後に確定実装。
- 運用: SPF / DKIM / DMARC 設定、バウンス処理、苦情処理 Webhook 受信エンドポイントの整備。

## 未解決事項 / 検証TODO（Proposed → Accepted の条件）

- Resend のログ仕様を公式ドキュメント + サポート問い合わせで確認
- AWS SES Event Destinations の設定で本文除外できるかを実機検証
- SendGrid / Mailgun の本文ログ保持を公式ドキュメントで確認
- 日本語メールの到達性テスト（Gmail / Yahoo! JP / iCloud / Outlook の各アカウントへテスト送信、迷惑メール判定を計測）
- 比較表を本 ADR に追記し、最終候補を 1 つに絞る
- 選定後、ステータスを Accepted に更新、日付を「最終更新」に反映

## 関連ドキュメント

- `docs/spec/vrc_photobook_business_knowledge_v4.md`（作成予定 / v4相当の業務知識）
- `docs/design/aggregates/manage-url-delivery/ドメイン設計.md`
- `docs/design/aggregates/manage-url-delivery/データモデル設計.md`
- `ADR-0001 技術スタック`
- `ADR-0002 運営操作方式`（ReissueManageUrl は cmd/ops で実行）
- `ADR-0003 フロントエンド認可フロー`（ReissueManageUrl 時の manage session 一括 revoke と同一 TX）
