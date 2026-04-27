# ADR-0006 メールプロバイダ再選定 + Manage URL Delivery 再設計

## ステータス

**Accepted（2026-04-28）**

ADR-0004（SendGrid 第一候補 / Mailgun 第二候補）は **本 ADR により Superseded**。
本 ADR では **MVP のメール送信機能を必須要件から外す**。Manage URL の配送は
**Complete 画面で 1 度だけ表示し、ユーザー自身に控えてもらう方式を MVP 標準**とする。

メール送信の本実装は、後続 PR で **個人事業主・任意団体でも契約可能なメールプロバイダが
確定した時点で再開**する。

## 作成日
2026-04-28

## コンテキスト

ADR-0004（2026-04-25 再選定）で、MVP のメール送信は以下を採用していた。

- 第一候補: **SendGrid (Twilio)**（AWS SES 申請不通過後の昇格採用）
- 第二候補: **Mailgun**

しかし 2026-04-28 時点で、以下の事実が判明 / 再確認された。

### 1. SendGrid Japan は法人向けで、個人 / 個人事業主 / 任意団体は契約不可

SendGrid Japan の公式サポート FAQ「個人でも契約可能ですか？」に
「**SendGrid Japan のサービスは、企業様（法人）を対象としております。個人事業主や任意団体の方はお申し込みをお受けすることができません**」
と明記されている。

VRC PhotoBook の運用主体は **個人**のため、SendGrid Japan の契約は不可能。
US 本体（Twilio SendGrid）への直接契約は支払い手段・本人確認・サポート等で運用負荷が
過大であり、MVP 段階では現実的でない。

### 2. AWS SES は production access 申請で**既に不通過**

ADR-0004 で記録のとおり、Amazon 側で SES 利用申請が不通過。再申請の見通しが立たない。
本 ADR でも **SES は採用しない**判断を維持し、第一候補から正式に降格する。

### 3. 既存実装の状況

- **PR28（2026-04-27）**で Complete 画面に管理 URL を 1 度表示する導線（`UrlCopyPanel` +
  `ManageUrlWarning`）が稼働中
- 業務知識 v4 §6 で「管理 URL は再表示しない」と元々規定されていたため、Complete 画面の
  1 度表示はメールに依存しない MVP 経路として機能している
- メール送信に依存していたのは「**紛失時の救済**」のみ（管理 URL 再発行 → メール送付）

## 決定

### 4.1 MVP は **メール送信なし**で進める

- ManageUrlDelivery 集約 / SendGrid 連携 / メール送信本実装は **MVP 必須要件から外す**
- Complete 画面での **1 度だけの表示 + コピー**を MVP 標準とする
- 管理 URL 紛失時の救済は **MVP 対象外**（後続 Provider 確定後に再着手）
- ADR-0004 は本 ADR により Superseded（SendGrid 第一候補 / Mailgun 第二候補の前提を破棄）

### 4.2 PR32 を「Email Provider 再選定 + Manage URL Delivery 再設計」に変更

新正典 §3 PR32 を本書の決定に合わせて更新する。

- 旧 PR32: SendGrid + ManageUrlDelivery 集約（Outbox handler でメール送信）
- 新 PR32: **Email Provider 再選定タスク**（候補調査 → PoC → ADR 化 → 採用確定後に
  ManageUrlDelivery 再設計）

### 4.3 PR30 / PR31 への影響

- PR30 Outbox table + 同 TX INSERT は **メール非依存のまま進める**
  （`PhotobookPublished` / `ImageBecameAvailable` / `ImageFailed` のみ、PR30 計画書 §4 通り）
- PR31 outbox-worker は handler を **no-op + log のみ**として実装し、メール送信に依存しない
- `ManageUrlReissued` / `ManageUrlDelivery*` event は **後続 PR（メール Provider 確定後）まで
  追加しない**

### 4.4 メール Provider 後続候補（**本 ADR では採用確定しない**）

調査対象として候補を列挙する。**契約可否（個人事業主 / 任意団体 / 個人）の実確認**を
行ってから、別 ADR で採用判断する。

| Provider | 期待 | 検証必要事項 |
|---|---|---|
| **Resend** | 開発者向け、個人プランあり | 本人確認 / 個人契約可否 / 日本語メール到達性 / 本文保存ポリシー |
| **Mailgun** | ADR-0004 で第二候補だった | 個人・個人事業主の契約条件再確認 |
| **Brevo（旧 Sendinblue）** | EU 系、無料枠大きい | 個人契約可否 / 本文保存ポリシー |
| **Postmark** | 本文保持有り（避けたい）/ 個人契約可 | 本文保持の retention 詳細、減免可否 |
| **ZeptoMail（Zoho）** | transactional 専用、低価格 | 個人契約可否 / 日本語到達性 / 本文保存 |
| **Zoho SMTP / Zoho Mail** | Zoho アカウント経由 | SMTP relay 用途で運用可能か |
| **Cloudflare Email Routing / MailChannels** | 受信向け中心、送信は限定的 | 送信 API としての可否（MailChannels API 経由） |
| **独自 SMTP（VPS / Cloud Run + 認証 SMTP relay）** | フォールバック | DKIM / SPF / DMARC 設定 / 到達性 / 運用負荷 |

評価軸（既存 ADR-0004 の延長）:

- 個人 / 個人事業主 / 任意団体での契約可否
- 本文を保存しないポリシー（送信履歴 UI に本文が残らない）
- API 接続性（Cloud Run Jobs から呼び出し可能）
- 日本語メールの到達性（GMail / iCloud / docomo / softbank / au）
- DKIM / SPF / DMARC / BIMI 設定容易さ
- MVP 段階の課金（無料枠の月送信数 / 想定 10〜100 通の範囲で収まるか）
- 法令対応（特定電子メール法 / GDPR、個人事業主名義での DPA 締結可否）

## 不採用とする選択肢

- **SendGrid (Twilio)**: 個人 / 個人事業主は SendGrid Japan で契約不可。US 本体直契約は MVP 範囲外
- **AWS SES**: production access 申請が不通過、再申請見通し立たず

## 影響範囲

### 影響を受けるドキュメント

- `docs/adr/0004-email-provider.md` → 本 ADR で Superseded（ステータス更新）
- `docs/plan/vrc-photobook-final-roadmap.md` § 1.3 / §3 PR32 → 名称・内容変更
- `docs/plan/m2-outbox-plan.md` → ManageUrlReissued / ManageUrlDelivery* event を
  「メール Provider 確定まで保留」と明示
- `docs/spec/vrc_photobook_business_knowledge_v4.md` → §6 メール送信前提があれば note 追加
  （業務知識 v4 自体の改訂は M2 後半で別途）

### 影響を受ける既存実装

- **無し**（PR28 まで実装済の Complete 画面 / `UrlCopyPanel` / `ManageUrlWarning` が
  既に MVP 経路を満たしている）
- 編集後の管理 URL 再発行ボタンは PR27 から disabled placeholder のまま維持

### 影響を受ける PR 計画

| PR | 影響 | 対応 |
|---|---|---|
| PR30 Outbox | 影響なし（メール非依存の 3 event のみ） | 計画書 §4.2 で `ManageUrlReissued` を明示的に保留と再記載 |
| PR31 outbox-worker | handler は no-op + log（メール送信なし） | PR31 計画書段階で本 ADR を参照 |
| PR32 | 「SendGrid + ManageUrlDelivery」→ **「Email Provider 再選定 + Manage URL Delivery 再設計」** | 名称 / 範囲を新正典で書き換え |
| PR33 OGP | 影響なし | - |
| PR34 Moderation / PR35 Report | 通知メール部分は保留 | 操作ログ + 画面表示で代替 |
| PR40 ローンチ前 | メール救済が無い旨を runbook に明記 | チェックリスト追加 |

## Manage URL 紛失時の MVP の扱い

- **MVP では復旧不可**（業務知識 v4 §6.x の「管理 URL は再表示しない」を厳守）
- Complete 画面で「再表示できません」警告を強める（PR28 で既に実装、今後さらに強化候補）
- 後続 PR でメール Provider 確定後に「メール再送 / 再発行」を提供

### Complete 画面の追加改善候補（**本 ADR では決定せず、後続候補として記録**）

| 項目 | 後続 PR 候補 |
|---|---|
| コピー導線をより目立たせる | 編集 UI 改善 |
| 「再表示不可」警告の文言を強化 | 編集 UI 改善 |
| QR コード表示 | 編集 UI 改善 |
| ローカル保存用テキスト / .vrcpb ファイル ダウンロード | 編集 UI 改善 |
| `mailto:` で自分宛にメール起動 | 編集 UI 改善（メール Provider 不要、ユーザー側で送信） |
| 「自分で控えた」確認チェックボックス | 編集 UI 改善 |
| 紛失時の案内ページ（FAQ） | LP / About と統合 |

これらは編集 UI 改善 PR（PR41+ または専用 PR）で評価する。本 ADR では実装しない。

## 関連リンク

- ADR-0004（Superseded by 本書）: `docs/adr/0004-email-provider.md`
- 業務知識 v4 §6 manage URL: `docs/spec/vrc_photobook_business_knowledge_v4.md`
- PR28 Complete 画面実装: `harness/work-logs/2026-04-27_publish-flow-result.md`

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（Accepted）。SendGrid 個人不可 + SES 不通過を受け、MVP メール送信なしを決定 |
| 2026-04-28 | PR32a で `docs/plan/m2-email-provider-reselection-plan.md` を作成。§4.4 の候補をショートリスト化（**Mailgun + ZeptoMail を PoC 候補**、Resend は本文非保持基準未達で MVP 不採用、Postmark は Add-on 課金で個人 MVP には重く第三候補）。本 ADR の決定（MVP メール送信なし）は変更せず、**C + D（メール送信なし継続 + Complete 画面の Provider 不要改善）**を PR32b で実装する方針を採用。Provider PoC は PR32c 以降の独立 PR で扱う |
