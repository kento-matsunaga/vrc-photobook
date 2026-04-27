# M2 Email Provider 再選定計画（PR32）

> **位置付け**: 本計画書は **計画書・調査メモ**であり、本書段階では **Provider 契約 /
> API key 発行 / Secret 登録 / Cloud Run 更新は行わない**。
>
> **正典関係**: 本計画書は [ADR-0006](../adr/0006-email-provider-and-manage-url-delivery.md)
> の §4.4「メール Provider 後続候補」を踏襲し、**個人 / 個人事業主 / 任意団体で契約
> 可能な provider のみ**に絞って公式情報ベースで再評価する。

---

## 0. 前提（再確認）

- ADR-0004（SendGrid 第一 / Mailgun 第二）は ADR-0006 で **Superseded**
- **SendGrid Japan は法人専用**で個人 / 個人事業主は契約不可（公式 FAQ）
- **AWS SES は production access 申請不通過**で MVP 採用不可
- ADR-0006 の決定により MVP は **メール送信なし**で進行中（Complete 画面で 1 度表示）
- PR28 で `UrlCopyPanel` / `ManageUrlWarning` が稼働中
- PR30 outbox table / PR31 outbox-worker は **メール非依存**で実装済
- Cloud Run Jobs / Scheduler は未作成（後続独立 PR）
- 業務知識 v4 §6 で「管理 URL は再表示しない」が原則
- 運営主体は **個人**

---

## 1. 目的

1. SendGrid / SES なしで **MVP 後続** に「管理 URL の救済 / 通知」をどう実現するかの方針を確定する
2. メール provider 候補を **公式情報ベースで再評価**し、PoC へ進めるショートリストを作る
3. **メール送信に依存しない代替導線**（QR / .txt / mailto / Save 確認 / FAQ 等）を整理し、
   PoC の前にどこまで救済できるかを判断する
4. PR32 の実装範囲（コードを書くか / 計画 + ADR 補追だけか）を決める

---

## 2. 範囲

### 対象（本計画書で確定）

- メール provider 候補の比較（**契約可否**を最優先）
- 推奨方針（A / B / C / D の選択）
- Manage URL Delivery の MVP / Provider 採用後の 2 段階設計
- Complete 画面 Provider 不要改善の候補出し（PR32b の想定範囲）
- Outbox / Worker への影響と「event 追加タイミング」の判断
- Security / Secret 管理方針の確認
- PR32 結論（実装するなら何をするか、計画書だけで止めるか）

### 対象外（本計画書で扱わない）

- **Provider 契約 / アカウント作成 / API key 発行 / Secret Manager 登録**
- **Cloud Run / Cloud Run Jobs の env / secretKeyRef 更新**
- **コードの追加実装**（payload struct / handler / EmailSender / Frontend UI）
- メール本文テンプレートの確定
- bounce / complaint webhook の設計詳細
- Phase 2（ローンチ後）の email 戦略

---

## 3. Provider 候補比較（公式情報、2026-04-28 調査）

> 出典は §13 の参照リンク。**日本在住の個人での契約可否は公式 ToS / 実体験報告を
> 横断確認する必要があり、本書では「公式記述で明示的に拒否されていない」レベルで
> 判定**する。**確定は別 ADR で「実アカウント作成 → 本人確認通過」を実証してから**。

### 3.1 ショートリスト

| Provider | 無料枠 | 最低有料 | 個人契約（公式記述） | 本文保持 | API / SMTP | 想定リスク |
|---|---|---|---|---|---|---|
| **Resend** | 3,000/月（100/日） | Pro $20/月 (50K) | 公式に法人限定の記述なし、個人開発者向けが UI 全体の主張 | dashboard で本文表示。content storage off は Pro 以上 + 厳しい条件 + $50/月 add-on | 両方 | 本文非保持の MVP 採用は条件未達 |
| **Mailgun** | Free 100/日（Basic 機能） | Basic $15/月、Foundation $35/月（50K） | 公式に法人限定の記述なし | **domain 単位で retention 0 day を選択可**（Help Center） | 両方 | 0 day 設定の挙動は実機検証必要 |
| **Postmark** | 100 通 free（恒常） | Basic $15/月 (10K) | 公式に法人限定の記述なし | **45 日 default**、Retention Add-on で 0-365 日 | 両方 | 本文 default 保持は管理 URL 用途で不利 |
| **Brevo（旧 Sendinblue）** | 300/日（無期限）| Starter $9/月〜（5K） | 公式に法人限定の記述なし、UI が個人〜SMB 向け | unlimited log retention（本文相当が見える前提）| 両方 | 本文長期保持で管理 URL 用途は不利 |
| **ZeptoMail（Zoho）** | 10,000 通の free credit（初回） | PAYG $2.50/10,000 通（6 ヶ月有効） | 公式に法人限定の記述なし、Zoho アカウントベース | 公式に「本文非保持」明言なし、ログ詳細は要実機確認 | 両方 | 低価格 / 個人向け契約しやすい候補。実機で本文表示の有無確認必要 |
| **MailChannels Email API** | Free dev 100/日 | 詳細は要確認（要 sales、blog で告知） | 公式に法人限定の記述なし | 公式 retention 仕様の明示が薄い | API 中心 | Cloudflare Workers 連携の旧無料枠は 2024-08-31 終了。新 Email API への移行が必要 |
| **Mailtrap Email Sending** | Free 4,000/月（150/日、3 日 retention） | $10/月 (10K) | 公式に法人限定の記述なし | log retention は 3〜30 日（プラン別） | 両方 | testing 由来で transactional 用途は新興、評判形成は途上 |

### 3.2 Provider 不要 / 限定的な選択肢（参考）

| 候補 | 評価 |
|---|---|
| **Cloudflare Email Routing**（送信） | **受信が中心**。送信 API は限定的で MVP に不適 |
| **Cloudflare 旧 MailChannels 無料連携** | **2024-08-31 EOL**。後継は MailChannels stand-alone |
| **Gmail SMTP（個人 Gmail 経由）** | Google ToS で transactional 自動送信は実質禁止。SMTP relay は Workspace 必須 |
| **Google Workspace SMTP relay** | **Workspace 契約が前提**（最低 6 USD/user/月）。送信ドメイン認証可。だが個人 MVP のためにアカウント取得が overhead |
| **独自 SMTP（VPS / 自前 MTA）** | **送信 IP 評判ゼロから**で到達性が極端に低い。SPF / DKIM / DMARC / PTR 全て自前管理。MVP 不可（ADR-0004 で既に評価済） |

### 3.3 配送ポリシー観点（管理 URL を本文に含む前提）

管理 URL は **token 相当**のため、本文 / 件名 / metadata 漏洩点を最小化するのが必須。

| Provider | dashboard 本文表示 | event payload に本文 | 「本文非保持」公式明言 | 0 retention 設定 |
|---|---|---|---|---|
| Resend | あり | なし | なし（add-on で限定的） | 厳しい条件付き |
| Mailgun | あり（保持期間中） | なし | なし | **domain 単位で 0 day 可** |
| Postmark | あり（45 日 default） | なし | なし | Add-on で 0 日可 |
| Brevo | あり（長期 retention） | なし | なし | unlimited が default で不利 |
| ZeptoMail | 公式記述要再確認 | 公式記述要再確認 | 要再確認 | 要再確認 |
| MailChannels | 公式記述要再確認 | 公式記述要再確認 | 要再確認 | 要再確認 |
| Mailtrap | あり（3〜30 日） | なし | なし | 短期 retention |

> **Resend は ADR-0004 でも「本文非保持の add-on は Pro+ かつ厳しい条件 + $50/月」で
> MVP 採用基準を満たさないと判定済**。本書でも同じ判定を維持する。

---

## 4. 評価軸（点数化）

| 軸 | 重み | Resend | Mailgun | Postmark | Brevo | ZeptoMail | MailChannels | Mailtrap |
|---|---|---|---|---|---|---|---|---|
| 契約可否（個人 / 日本） | 5 | 4 | 4 | 4 | 4 | 4 | 4 | 4 |
| 本文非保持 / retention 制御 | 5 | 1 | 4 | 3（add-on で 4） | 1 | 2（要検証） | 2（要検証） | 2 |
| 実装容易性（API / SDK） | 3 | 5 | 4 | 4 | 4 | 4 | 3 | 4 |
| 初期費用 / MVP の月額 | 3 | 4（Free 3K）| 4（Free 100/日）| 3（100/月 Free）| 5（300/日 Free）| 5（PAYG $2.50/10K）| 4（dev 100/日）| 5（4K Free）|
| 審査リスク | 3 | 3 | 3 | 3 | 3 | 3 | 3 | 3 |
| deliverability（評判） | 4 | 4 | 4 | 5 | 3 | 3（要検証） | 3 | 3 |
| API 品質 / docs | 2 | 5 | 4 | 4 | 3 | 4 | 3 | 3 |
| 運用負荷 | 2 | 4 | 4 | 4 | 3 | 4 | 3 | 4 |
| 将来性 / 安定性 | 2 | 4 | 5 | 5 | 4 | 4 | 3 | 3 |
| 日本からの使いやすさ | 2 | 4 | 4 | 4 | 3 | 4 | 3 | 3 |
| **合計（重み付き）** | - | **108** | **121** | **115** | **103** | **111** | **96** | **104** |

> 点数は 1〜5 の主観評価。**契約可否はすべて公式記述上 4（拒否されていない）**で
> 揃え、実体検証で 5 に上げるか 1 に落とすかは別 ADR で確定する。
>
> **本文 retention の重み（5）と deliverability の重み（4）が支配的**で、Mailgun /
> Postmark が上位、Resend / Brevo が下位という配置になる。

### 4.1 ショートリスト（PoC 候補）

1. **Mailgun**: 0 day retention 設定可能 + 個人契約可。ADR-0004 で第二候補だった経緯あり
2. **Postmark**: Retention Add-on で 0 日可能 + deliverability 評価が高い
3. **ZeptoMail**: PAYG が個人 MVP に最適 + 低価格。本文非保持の実機検証が必要
4. **Resend**: 本文非保持条件は厳しいが、API モダンで PoC コストが低い（参考枠）

---

## 5. 推奨方針

### 採用: **C + D の併用**

> **C: MVP ではメール送信なし継続**（Complete 画面 1 度表示を MVP 標準として維持）
> **D: Provider 不要の代替導線を強化**（QR / .txt / mailto / Save 確認 / FAQ）

理由:

- ADR-0006 で既に「MVP メール送信なし」が決定済。本書はその方針を維持
- 過去 SES / SendGrid で **アカウント審査** に時間を取られた経緯がある。同じ詰まりが
  Mailgun / Postmark / ZeptoMail でも起き得る
- **Provider 確定までの待ち時間中、ユーザー救済をどう強化するか**が現実的なボトルネック
- Complete 画面の代替導線（D）はメール provider なしで MVP 完成度を上げられる
- Provider PoC は **C/D 完了後、別 PR** で実施する（時間ボックスを明確に分割）

### 補完: **Provider PoC は B のサブセットで段階実施**

> **B: Provider を 2 つに絞って PoC**

PoC 対象 2 候補（実装承認 + 課金承認のうえ別 PR で着手）:

1. **Mailgun**（第一候補、0 day retention の実機確認が必要）
2. **ZeptoMail**（第二候補、PAYG で個人 MVP に最適。本文非保持を実機確認）

> Resend は本文非保持基準を満たさないため MVP の第一候補から外し、Phase 2 でスケール
> 後に再評価する。Postmark は Add-on の追加課金が個人運用には重い（Basic $15/月 + Add-on）
> ため、第三候補として保留する。

### 不採用判断

- **A（1 つに絞って PoC）**: 過去の SES / SendGrid と同じ詰まりリスク。2 候補で
  リスクヘッジする
- **「コード実装で確定」する判断**: 契約 / 審査が通る前に EmailSender 実装を進めると
  廃棄コストが高い。PoC 通過後に着手

---

## 6. Manage URL Delivery 再設計

### 6.1 MVP（Provider 採用前、現状）

```
┌─────────────────────────┐
│ PublishFromDraft        │
│  └─ 同 TX で:           │
│      - photobooks UPDATE│
│      - sessions revoke  │
│      - outbox INSERT    │
│        (photobook.published) │
└──────────┬──────────────┘
           │
           ▼
   Complete 画面（PR28 実装済）
   - UrlCopyPanel: コピーボタン
   - ManageUrlWarning: 「再表示しません」警告
   - **メール送信なし**
   - 紛失時の救済なし（FAQ 案内のみ）
```

- ManageUrlDelivery 集約 / `manage_url_deliveries` table は **作らない**
- `ManageUrlReissued` / `ManageUrlDelivery*` event は outbox に投入しない
- ReissueManageUrl UseCase は domain ロジックとしては既存（PR9b）だが、
  ops CLI / 通知経路は未実装のまま

### 6.2 Provider 採用後（後続 PR）

採用が確定したら段階的に追加:

| ステップ | 内容 | 担当 PR（暫定） |
|---|---|---|
| 1 | EmailSender ポート抽象化 + Provider 実装（Mailgun / ZeptoMail のいずれか） | Provider PoC PR |
| 2 | ManageUrlDelivery 集約復活（recipient_email 短期保持 24h NULL 化、token_version snapshot） | 後続 PR |
| 3 | `ManageUrlReissueRequested` / `ManageUrlDeliveryRequested` event を outbox CHECK 緩和 + handler 実装 | 後続 PR |
| 4 | bounce / complaint webhook 受信 + suppression list | 後続 PR |
| 5 | resend / reissue flow（運営 ops CLI 経由） | 後続 PR |
| 6 | rate limit / abuse 対策（IP 単位 / photobook 単位） | 後続 PR |

実装方針は ADR-0004（Superseded）の §「実装方針」を **provider 中立**として継承する:

- 件名 / metadata に管理 URL を含めない
- outbox payload に管理 URL を入れない（worker が DB から組み立てる）
- recipient_email は 24h で NULL 化
- DeliveryAttempt.providerMessageId のみ保持
- DeliveryAttempt.errorSummary は失敗キーのみ（stack trace / 本文は含めない）

---

## 7. Complete 画面 Provider 不要改善（PR32b 候補）

### 7.1 改善案の比較

| 案 | 実装容易性 | Safari OK | iPhone Safari OK | リスク | 採用候補 |
|---|---|---|---|---|---|
| コピー導線強化（既存 UrlCopyPanel の文言 / ボタン強化） | 高 | ✓ | ✓ | なし | **採用候補 A** |
| QR コード表示（pure JS / canvas） | 中 | ✓ | ✓ | スクショ漏洩は既存と同じ | 採用候補 |
| .txt ダウンロード（Blob + a download attribute） | 高 | ✓ | iPhone Safari は Files に保存 | なし | **採用候補 A** |
| `.vrcpb` 拡張子 | 中 | ✓ | カスタム拡張子は OS 標準で開けない | アプリ無し | 不採用 |
| `mailto:` でユーザー自身に送信 | 高 | ✓ | ✓ | OS の Mail 設定依存 | **採用候補 A** |
| 「保存しました」確認チェックボックス | 高 | ✓ | ✓ | UX 摩擦の追加 | **採用候補 A** |
| FAQ / 紛失時案内ページ | 高 | ✓ | ✓ | なし | **採用候補 A** |
| ブラウザ localStorage / IndexedDB 保存 | 中 | ✓ | ✓ | XSS / 共有端末漏洩 | 不採用（セキュリティ的に推奨せず） |
| PWA / Web Share API | 中 | △（Web Share Level 2 制限） | iPhone Safari は Web Share API ベースのみ | 共有先のログに残る | 候補（Phase 2） |

### 7.2 PR32b 採用提案セット

採用候補 A をまとめて 1 PR で実装する想定:

1. **コピー導線強化**: UrlCopyPanel の文言 / 視覚強調
2. **.txt ダウンロード**: Blob 経由で `vrc-photobook-manage-url-{slug}.txt` を生成。
   内容は管理 URL のみ（フッタ / 説明文を入れない）
3. **mailto: 起動ボタン**: subject = 「VRC PhotoBook 管理 URL（自分用）」、
   body = 管理 URL のみ。ユーザーの Mail App で送信、Provider 不要
4. **「保存しました」確認チェックボックス**: チェックしないと Complete 画面の閉じる
   ボタンを enable しない（誤誘導防止）
5. **FAQ / 紛失時案内**: 「紛失時は再発行できません」を明記したページを `/about` 配下に追加

すべて Frontend のみで実装可能。Backend / DB / migration / Cloud Run 変更なし。

### 7.3 セキュリティ確認（PR32b 想定）

- 管理 URL を `localStorage` / `IndexedDB` に書かない（XSS / 共有端末漏洩）
- mailto: の URL encode で改行 / 制御文字を escape
- .txt download は管理 URL のみ。photobook ID / token version 等の付加情報を入れない
- PWA / Service Worker キャッシュに管理 URL を残さない
- Safari / iPhone Safari 実機確認（ITP / mailto / Files 保存）

---

## 8. PR32 結論（実装範囲）

### 推奨: **PR32a で本計画書 + ADR 補追まで、PR32b は別 PR**

| 段階 | 内容 | 想定 PR |
|---|---|---|
| **PR32a** | 本計画書 + ADR-0006 補追 + 新正典 §3 PR32 説明更新 | **本 PR**（コード変更なし、実装無し） |
| PR32b | Complete 画面 Provider 不要改善（コピー導線 / .txt / mailto / 確認チェック / FAQ）| 後続独立 PR（Frontend のみ）|
| PR32c | Provider PoC（Mailgun + ZeptoMail のうち 1 つで本人確認 → 1 通テスト送信 → ADR 化）| 後続独立 PR（課金 / 契約承認の停止ポイントあり） |
| PR32d 以降 | EmailSender 実装 + ManageUrlDelivery 集約復活 + handler 接続 + bounce webhook | Provider 確定後の連続 PR |

### 順序判断

- **OGP 自動生成（PR33）は Email Provider と独立**で、本書の影響を受けない
- PR32 結論によりロードマップ §3 の流れは:
  - PR32a（本書）→ PR32b（Frontend 改善）→ PR33 OGP → PR32c（Provider PoC）→ ...
  - **PR33 OGP を Provider PoC より先に進める**（Provider 確定の待ち時間を OGP 実装で埋める）

---

## 9. Outbox / Worker への影響

### 9.1 PR31 worker は **no-op + log のまま維持**

- Cloud Run Jobs / Scheduler 未作成。pending event は consume されない
- 副作用 handler が実装されるまで、event を `processed` に進めるリスクを避ける

### 9.2 ManageUrlDelivery event は **PR32d 以降まで追加しない**

- `ManageUrlReissueRequested` / `ManageUrlDeliveryRequested` を outbox に入れる時期は
  **EmailSender 実装と同 PR**
- migration で event_type CHECK を緩めるのも同 PR
- それまでは outbox は 3 種のまま

### 9.3 Provider 未確定のままでも PR33 OGP の event は **追加可能**

- OGP 系 event（例: `ogp_image.requested`）は email provider と無関係
- PR33 で migration + handler 実装 + Cloud Run Jobs / Scheduler の判断を行う
  （本書の範囲外）

### 9.4 pending event を processed にしてよいタイミング

- handler の副作用（OGP 再生成 / 通知 / cleanup / メール送信）が実装されたあと
- それ以前に Cloud Run Jobs を稼働させると「処理済みだが副作用未実行」になる
  （pr-closeout.md / PR31 work-log に記録済の判断）

---

## 10. Security 方針

### 10.1 管理 URL は token 相当

- 本文 / 件名 / metadata に含む経路を最小化
- メール送信しない MVP は **構造的に漏洩点が無い**（最大の利点）

### 10.2 Provider 採用後の漏洩点

| 漏洩経路 | 緩和策 |
|---|---|
| Provider dashboard で本文表示 | Mailgun: 0 day retention / Postmark: Add-on 0 日 / Resend: 採用基準未達 |
| Provider event log payload | 各 provider とも本文は payload に入らない（公式記述） |
| webhook 受信側の log | suppression / bounce 受信 endpoint で本文 / 管理 URL を log に出さない |
| Provider 内部運用者アクセス | retention 0 day で物理的に消失 |
| Provider 退会 / アカウント停止 | 移行時の DKIM / DMARC / 過去送信ドメイン評判の引き継ぎを runbook 化 |

### 10.3 Secret 管理（Provider 採用後）

- API key / SMTP password は **Secret Manager** に格納
- Cloud Build SA には Secret Manager 権限を **付けない**（PR29 の方針継承）
- Runtime SA のみ `secretmanager.secretAccessor`
- Cloud Run env / secretKeyRef で注入（既存パターン踏襲）
- cloudbuild.yaml に provider key を書かない（既存ルール）

### 10.4 PR32a（本書）における security

- **コードは追加しない**ため、現時点で Secret 漏洩点は増えない
- 計画書 / ADR 補追には provider 実値（API key / 契約情報）を書かない
- Provider 名 / 公式 URL / 価格情報のみ記載

---

## 11. PR closeout 適用（pr-closeout.md §6）

PR32a 完了報告に以下のチェックリストを含める。

- [ ] コメント整合チェック実施: `bash scripts/check-stale-comments.sh --extra "SendGrid|SES|Email Provider|ManageUrlDelivery|SMTP|Resend|Postmark|Mailgun|Brevo|ZeptoMail|Zoho|Mailtrap"`
- [ ] 古いコメントを修正したか（または「該当なし」と明記）
- [ ] 残した TODO とその理由（4 区分）
- [ ] 先送り事項がロードマップに記録済み（Provider PoC / EmailSender 実装 / ManageUrlDelivery 復活）
- [ ] generated file 未反映コメント: 該当なし（コード変更なし）
- [ ] Secret 漏洩 grep（本書 / ADR-0006 / 新正典の禁止リスト記述のみで実値 0 件）

---

## 12. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版作成（PR32a）。SendGrid / SES 不可後の provider 候補を公式情報ベースで再評価。MVP は ADR-0006 の方針（メール送信なし）を維持し、Complete 画面の Provider 不要改善（PR32b）を優先する判断を採用。Provider PoC は PR32c 以降の独立 PR で扱う |

---

## 13. 参照（公式情報、2026-04-28）

> 本書の比較は以下の公式 / 公式に近い情報源を横断確認した結果。**実アカウント
> 作成 / 本人確認 / 契約完了は本書段階では未実施**。確定は PoC PR で実証する。

### Provider 公式

- Resend Pricing: <https://resend.com/pricing>
- Resend account quotas and limits: <https://resend.com/docs/knowledge-base/account-quotas-and-limits>
- Postmark Pricing: <https://postmarkapp.com/pricing>
- Mailgun Pricing: <https://www.mailgun.com/pricing/>
- Mailgun Adjusting a domain's message retention settings: Mailgun Help Center
- Brevo Pricing: <https://www.brevo.com/pricing/>
- Brevo Transactional Email: <https://www.brevo.com/products/transactional-email/>
- ZeptoMail Pricing: <https://www.zoho.com/zeptomail/pricing.html>
- ZeptoMail product page: <https://www.zoho.com/zeptomail/>
- MailChannels Email API End of Life Notice (Cloudflare Workers): MailChannels Help Center
- Mailtrap Pricing: <https://mailtrap.io/pricing/>

### 既存 ADR

- ADR-0004（Superseded）: `docs/adr/0004-email-provider.md`
- ADR-0006（Accepted）: `docs/adr/0006-email-provider-and-manage-url-delivery.md`

### 既存ルール

- `.agents/rules/security-guard.md`（Secret / Cookie / 認可）
- `.agents/rules/safari-verification.md`（Safari 必須確認）
- `.agents/rules/pr-closeout.md`（PR 終了処理）
