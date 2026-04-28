# M2 Report 集約 / 通報受付（PR35）計画書

> 上流: [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md) PR35 章
> 関連: [`docs/design/aggregates/report/`](../design/aggregates/report/)（v4 完全設計） /
> [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §3.6 / §3.7 / §5.4 / §6.11 / §7.2 / §7.3 / §7.4 /
> [`docs/design/cross-cutting/outbox.md`](../design/cross-cutting/outbox.md) §4.2 /
> [`docs/plan/m2-moderation-ops-plan.md`](./m2-moderation-ops-plan.md)（PR34a 計画書） /
> [`harness/work-logs/2026-04-28_moderation-ops-result.md`](../../harness/work-logs/2026-04-28_moderation-ops-result.md)（PR34b 結果）

---

## 0. このドキュメントの位置付け

- 本書は **PR35（実装PR）の計画書**であり、計画書段階ではコード変更 / migration 適用 / 実リソース操作は **行わない**
- v4 完全設計（Report 集約 6 ReportReason + 5 ReportStatus + Moderation/Outbox 連携）は
  [`docs/design/aggregates/report/`](../design/aggregates/report/) に既出
- 本計画は **PR35 MVP スコープ**に絞り、Email 通知 / 自動 moderation / Web admin UI / dashboard /
  個人情報 90 日 NULL 化 reconciler / UsageLimit 等は別 PR に持ち越す
- 本書の判断対象は §15 にまとめる

---

## 1. 現状整理（PR34b 完了時点）

### 1.1 既に整っている前提

| 観点 | 状態 |
|---|---|
| `moderation_actions` table | 存在（PR34b で適用済、`source_report_id uuid NULL` カラムを最初から持つ）|
| Moderation UseCase（hide / unhide）| 同 TX 4 要素 + Outbox event INSERT 完成。**`sourceReportId` の同 TX 連動は実装未完**（PR34b は常に nil で渡す）|
| `cmd/ops photobook hide / unhide / show / list-hidden` | 完成（`--source-report-id` フラグは未実装）|
| Outbox 機構 | `aggregate_type=report` は CHECK で受け入れ済（PR30 migration `00012_create_outbox_events.sql`）。`event_type` CHECK は **`report.submitted` 未追加** で、本 PR で migration が必要 |
| Outbox handler 配線 | photobook.published（OGP 生成）/ photobook.hidden / photobook.unhidden / image.* の 5 種登録済。`report.submitted` handler は未実装 |
| Turnstile 機構 | upload-verification 集約で導入済（`backend/internal/uploadverification/`、Turnstile siteverify + atomic consume）。site-key / secret-key は本番運用中 |
| Frontend `/p/[slug]` | 公開 Viewer 実装済（PR25a/PR33c）|
| Photobook public viewer 認可 | published + visibility=public + hidden=false のみ 200。それ以外は 410 / 404 / OGP fallback |
| Email Provider | **未確定**（ADR-0006、SendGrid 個人不可 / SES 申請不通過）。MVP は通知メール送信なし |
| 業務知識 v4 §3.6 / §3.7 / §7 | Report 上流設計は確定（snapshot 保持 / FK なし / `minor_safety_concern` 独立 / IP hash UsageLimit 共有 / 90 日 NULL 化）|

### 1.2 未整備事項（PR35 で扱う候補）

- `reports` table（v4 設計書 §3 通り、append でなく state machine、未作成）
- `internal/report/` パッケージ（domain / VO / Repository / UseCase）
- 公開 endpoint `POST /api/public/photobooks/{slug}/reports`
- Frontend 通報フォーム
- Outbox `report.submitted` event_type CHECK 拡張 + handler
- `cmd/ops` の `report list / show` サブコマンド
- `cmd/ops photobook hide --source-report-id <RID>` 拡張（同 TX で Report.status を `resolved_action_taken` 遷移）

### 1.3 Email Provider との切り分け

ADR-0006 で MVP メール送信は必須から外している。Report submission の運営通知も **本 PR35 ではメール送信を実装しない**。Outbox event は INSERT するが worker handler は **no-op + structured log + 本番 Cloud Run logs に通知レベル印字** とする（minor_safety_concern など優先カテゴリは log severity を上げる）。Email Provider 確定後（PR32c 以降）に handler を実装。

---

## 2. PR35 のゴール（MVP）

PR35 で達成する最低限のゴール:

### G1. 公開 Viewer から通報フォームに導線が作れる

- `/p/[slug]` の Viewer ページに「通報」リンクを表示
- リンク先（モーダル or 別ページ、§7）で reason 選択 + detail 任意入力 + reporter_contact 任意入力 + Turnstile widget

### G2. 通報を **DB に保存**できる

- `POST /api/public/photobooks/{slug}/reports` で受け付け
- Report 集約 1 行 + Outbox `report.submitted` event を **同一 TX で INSERT**
- v4 P0-13 の不変条件を守る

### G3. snapshot 保持で Photobook purge 後も証跡が残る（v4 P0-11）

- 通報受付時に Photobook の現在の slug / title / creator_display_name を snapshot として保存
- `target_photobook_id` に FK は **付けない**

### G4. 通報対象を **published + visible + not hidden の photobook に限定**

- private / draft / deleted / purged / hidden_by_operator は受付拒否
- 拒否時の HTTP は **404 / 410 を呼び分けず 1 種類**で返す（敵対者に状態を漏らさない、`get_public_photobook.go` の既存ポリシーと整合）

### G5. **Bot / abuse 対策**として Turnstile を **必須**にする

- 既存 upload-verification の Turnstile infrastructure を **流用するか / 別発行にするか**は §8 ユーザー判断 #1
- Turnstile token は server side で siteverify 実行、失敗で 403 拒否
- rate limit は MVP では実装せず、`source_ip_hash` 保存だけ行う（後続 UsageLimit 集約で連動）

### G6. 運営が **`cmd/ops report list / show`** で通報一覧と詳細を確認できる

- list は status / reason / submitted_at で絞り込み
- show は単一 Report の詳細（reporter_contact / detail / source_ip_hash 等を含む。CLI 標準出力なので運営者のみが見える前提）
- `minor_safety_concern` は最優先表示（v4 §3.6 / §7.4）

### G7. **`cmd/ops photobook hide --source-report-id <RID>`** で Moderation × Report を同 TX 連動

- PR34b の hide UseCase を拡張し、source_report_id を受け取れる
- 同 TX で `moderation_actions.source_report_id = $1` を INSERT + `reports SET status='resolved_action_taken', resolved_by_moderation_action_id = $action_id, resolved_at = now()` UPDATE
- v4 P0-5 / P0-19 の不変条件を守る
- unhide では Report 状態を自動更新しない（v4 P0-21）

### G8. Frontend 通報フォームが Safari / iPhone Safari で動く

- form / select / textarea / Turnstile widget が iPhone Safari 実機でレイアウト崩れせず submit できる
- submit 後の thanks view 表示
- submit 中の重複送信防止

### G9. Secret / Privacy を最小化

- raw token / Cookie は通報フローで一切使わない（公開 endpoint、creator session 不要）
- reporter_contact は任意（送らない選択肢を UI で明示）
- detail に個人情報を書かない注意文を UI に表示
- IP は **生値を保存せず source_ip_hash（ソルト + SHA-256）のみ保存**
- 通報者の通知メール送信は **PR35 範囲外**（Email Provider 未確定、MVP は受付確認画面で完結）

---

## 3. PR35 で扱うこと / 扱わないこと

### 3.1 扱うこと（MVP）

- migration `00016_create_reports.sql`（v4 設計書 §3 通り、append でなく state machine、9 カラム + 6 INDEX）
- migration `00017_relax_outbox_event_type_check.sql`（`report.submitted` 追加）
- `internal/report/` パッケージ（domain entity Report / VO 6 種 / Repository / UseCase 2-3 種）
- 公開 endpoint `POST /api/public/photobooks/{slug}/reports`（Turnstile 必須、Cookie 不要、rate-limit MVP 簡易）
- Frontend 通報フォーム（`/p/[slug]/report` page or modal、§7 ユーザー判断 #5）
- Outbox `report.submitted` event_type CHECK 拡張 + no-op handler（structured log、minor_safety_concern は severity 上げる）
- `cmd/ops report list / show`
- `cmd/ops photobook hide --source-report-id <RID>` 拡張（PR34b の HideInput に SourceReportID オプションを追加、同 TX で reports.status 更新）
- tests（domain / usecase / handler / cmd / frontend）
- `docs/runbook/ops-moderation.md` § Report 連携の追記（または `ops-report.md` 新規作成、§14 ユーザー判断 #6）

### 3.2 扱わないこと（PR35 範囲外、対応 PR を明示）

| 項目 | 対応 PR / 状態 |
|---|---|
| Email 通知（運営 / 通報者）| Email Provider 確定後（PR32c 以降）|
| Web admin dashboard / UI | MVP 範囲外（v4 §6.19）|
| 自動 moderation / 自動 hide | v4 §3.6 で禁止（運営が必ず手動判断）|
| reporter_contact / source_ip_hash の 90 日後 NULL 化 reconciler | PR33e 系の Reconcile 拡張、または別 PR |
| UsageLimit 集約（rate-limit、abuse 抑止）| **PR36** で扱う。本 PR35 では `source_ip_hash` 保存だけ、rate-limit は実装しない |
| Report の `under_review` / `resolved_no_action` / `dismissed` 遷移用 cmd/ops | PR35 拡張または別 PR（state machine の他の遷移）|
| 通報悪用（spam / 同一 IP 大量通報）の自動検知 | UsageLimit 連動、PR36 |
| 通報対象が `deleted` / `purged` の Photobook を許容するか（v4 ドメイン §6.1 では「削除済みも対応のため受付」と書いてあるが、MVP では拒否を推奨）| 計画書 §15 ユーザー判断 #2 |
| Photobook public viewer 上の「通報」UI 配置 | PR35 内、design 抽出は §7 |
| analytics / 通報数集計 dashboard | 後続任意 |

---

## 4. DB 設計

### 4.1 採用方針

v4 設計書 [`docs/design/aggregates/report/データモデル設計.md`](../design/aggregates/report/データモデル設計.md) §3 をそのまま採用。MVP で削るカラムは無い。理由:

- snapshot 3 カラム（v4 P0-11）は **Photobook purge 後の監査証跡保持**に必須
- `resolved_by_moderation_action_id` は PR34b と PR35 を繋ぐキー（同 TX 連動で必須）
- `source_ip_hash` は UsageLimit と同期させるためソルトポリシー共有が必要、PR36 でも使う
- 6 INDEX も v4 §4 通り（minor_safety_concern 部分 INDEX が運営優先キューに必須）

### 4.2 migration 案

```text
backend/migrations/00016_create_reports.sql
  - reports table（id / target_photobook_id / 3 snapshot / reason / detail /
    reporter_contact / status / submitted_at / reviewed_at / resolved_at /
    resolution_note / resolved_by_moderation_action_id / source_ip_hash）
  - 6 INDEX（v4 設計書 §4 通り、minor_safety_concern 部分 INDEX 含む）
  - CHECK 6 reason（subject_removal_request / unauthorized_repost /
    sensitive_flag_missing / harassment_or_doxxing / minor_safety_concern / other）
  - CHECK 5 status（submitted / under_review / resolved_action_taken /
    resolved_no_action / dismissed）
  - CHECK detail ≤ 2000 char / reporter_contact ≤ 200 char / snapshot 長さ制約
  - FK は付けない（target_photobook_id / resolved_by_moderation_action_id 共に）

backend/migrations/00017_relax_outbox_event_type_check.sql
  - outbox_events.event_type CHECK に 'report.submitted' を追加（5 種 → 6 種）
```

### 4.3 既存テーブル変更

`reports` 新設のため photobooks / moderation_actions / outbox_events に **追加カラムなし**。`outbox_events.event_type` CHECK のみ拡張。

### 4.4 IP ハッシュソルト管理

- 環境変数 `REPORT_IP_HASH_SALT_V1` を Secret Manager に登録 → Cloud Run / Cloud Run Job 経由で注入
- Application 層で `sha256(salt_version + ":" + salt + ":" + ip)` の形で hash 化
- 生 IP 値は一切保存しない / log に出さない
- ソルト version は将来ローテーション可能（PR36 / PR39）
- PR35 では **新規 Secret 1 個追加**（`REPORT_IP_HASH_SALT_V1`、§15 ユーザー判断 #3）

---

## 5. Domain / UseCase 設計

### 5.1 一覧（PR35 MVP）

| UseCase | 操作種別 | 同一 TX に含まれるもの |
|---|---|---|
| `SubmitReport` | 公開 endpoint | photobook 存在 / 公開状態確認 → reports INSERT + outbox_events INSERT |
| `GetReportForOps` | 参照（cmd/ops） | reports SELECT + photobooks 現状 lookup（任意）|
| `ListReportsForOps` | 参照（cmd/ops） | reports SELECT（status / reason filter）|
| `LinkReportToHide`（拡張） | PR34b HideInput を拡張、`SourceReportID` オプション付与 | hide の既存 4 要素 + reports.status='resolved_action_taken' UPDATE |

PR35 では **`startReview` / `resolveWithoutAction` / `dismiss` UseCase は実装しない**（state machine の他遷移は別 PR）。`resolveWithAction` は cmd/ops hide の拡張で間接実装する。

### 5.2 SubmitReport の詳細

#### 入力

```text
slug: 公開 URL slug（rough validation、Backend が photobook 解決）
reason: ReportReason VO（6 種）
detail: ReportDetail VO（任意、≤ 2000 rune）
reporterContact: ReporterContact VO（任意、≤ 200 char）
turnstileToken: Cloudflare Turnstile token（必須、§8）
remoteIP: HTTP request 経由（XFF / X-Real-IP / Workers の cf.connectingIP 等から取得）
now: 時刻
```

#### 振る舞い

```text
1. slug VO 化 + Turnstile siteverify
2. Photobook を slug で permissive lookup（FindAnyBySlug）
3. 公開対象判定:
   - status='published' AND visibility='public' AND hidden_by_operator=false
   - 上記以外は ErrTargetNotEligibleForReport（404 として返す）
4. Photobook 現在値（slug / title / creator_display_name）を snapshot として確保
5. source_ip_hash 算出（環境変数の salt_version + salt + remoteIP の sha256）
6. WithTx:
   - reports INSERT（status='submitted'、6 値を埋める）
   - outbox_events INSERT（event_type='report.submitted'、payload は §6）
7. report_id 返却
```

#### 同一 TX 4 要素適合（v4 P0-19、Report 1 + Outbox 1）

Report は単独集約のため、4 要素のうち「Photobook 状態変更」「Report 状態更新」は不要。**reports INSERT + outbox_events INSERT の 2 要素同 TX**で v4 P0-13 を満たす。

### 5.3 LinkReportToHide（cmd/ops hide 拡張）

PR34b の `HidePhotobookByOperator` UseCase に `SourceReportID *report_id.ReportID` オプションを追加:

```text
HideInput:
  PhotobookID
  ActorLabel
  Reason
  Detail
  SourceReportID *report_id.ReportID  # 新規、任意
  Now
```

実装で:
- moderation_actions INSERT 時 `source_report_id = $1::uuid` を埋める（既に nullable）
- SourceReportID が指定されたら同 TX で `UPDATE reports SET status='resolved_action_taken', resolved_by_moderation_action_id=$action_id, resolved_at=$now WHERE id=$report_id AND status IN ('submitted','under_review')` を実行
- 0 行 UPDATE（既に終端 / 不在）はエラー（敵対者検知 / 報告ミス検知）

cmd/ops 側:
```bash
ops photobook hide --id <PID> --reason report_based_minor_related --actor ops-1 \
  --source-report-id <RID> --execute
```

### 5.4 エラー設計

| Sentinel | 戻し条件 | 公開 endpoint の HTTP |
|---|---|---|
| `ErrInvalidSlug` | slug 形式不正 | 400 |
| `ErrTurnstileFailed` | Turnstile siteverify 失敗 | 403 |
| `ErrTargetNotEligibleForReport` | photobook 不存在 / draft / deleted / purged / private / hidden | **404**（区別を漏らさない）|
| `ErrInvalidPayload` | reason / detail / contact のバリデーション失敗 | 400 |
| `ErrInternal` | DB エラー / Turnstile 接続失敗等 | 500 |

---

## 6. API 設計

### 6.1 公開 endpoint

```text
POST /api/public/photobooks/{slug}/reports
```

#### 入力（JSON body）

```json
{
  "reason": "minor_safety_concern",
  "detail": "（任意、≤ 2000 char）",
  "reporter_contact": "（任意、≤ 200 char）",
  "turnstile_token": "0.xxxxxxxx..."
}
```

#### 出力（成功）

```text
HTTP 201
{
  "report_id": "019dd...",
  "status": "submitted"
}
```

#### 出力（拒否）

| HTTP | body | 用途 |
|---|---|---|
| 400 | `{"status":"invalid_payload"}` | reason 不正 / detail 過長 / contact 過長 / Turnstile token 欠如 |
| 403 | `{"status":"turnstile_failed"}` | siteverify 失敗 |
| 404 | `{"status":"not_found"}` | photobook 不在 / 公開対象でない（区別を漏らさない、敵対者対策）|
| 500 | `{"status":"internal_error"}` | DB / Turnstile 接続失敗 |

#### レスポンスヘッダ

- `Cache-Control: private, no-store, must-revalidate`
- `X-Robots-Tag: noindex, nofollow`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Set-Cookie` なし（Cookie 不要 endpoint、認証なし）

#### CSRF 対策

- 認証なし endpoint で Cookie session を使わないため CSRF の意味は限定的
- それでも **Origin ヘッダ検証**（`https://app.vrc-photobook.com` のみ許可）と Turnstile siteverify で実質的な多層防御
- HTTP method は **POST 限定**（GET / OPTIONS で受け付けない）

### 6.2 認可

- **Cookie 不要**（公開機能、v4 §3.6 で「閲覧者は自分のログインや登録なしに通報を行える」）
- session middleware を通さない（draft / manage cookie の有無に関係なく動作）
- Turnstile が唯一の bot 防御層

### 6.3 IP 取得方針

Cloudflare Workers / Cloud Run の reverse proxy 構成で:

| 取得先 | 信頼性 |
|---|---|
| `Cf-Connecting-Ip` ヘッダ | Cloudflare 経由のみ。app.vrc-photobook.com Workers が backend に転送するときに付与する想定 |
| `X-Forwarded-For` の最終 hop | Cloud Run が自動付与 |
| `RemoteAddr` | Cloud Run の前段 proxy IP（信頼できない）|

→ **Cloud Run service の `X-Forwarded-For` 末尾を採用**が最も実装簡単。Cloudflare 経由の `Cf-Connecting-Ip` が利用できれば優先（PR35 実装時に確認、§15 ユーザー判断 #4）。

---

## 7. Frontend UI 設計

### 7.1 配置と画面遷移

3 案を提示し、§15 ユーザー判断 #5 で確定:

| 案 | 内容 | リスク / メリット |
|---|---|---|
| **A. `/p/[slug]/report` 別ページ**（推奨）| Viewer から「通報」リンク → 別ページに遷移 → 送信 → thanks 表示 | メリット: 実装シンプル、SSR / Next.js Route Handler で動く、Safari 互換性高い。リスク: 1 クリック余計 |
| B. modal | Viewer 内でモーダル表示 | メリット: UX 向上。リスク: SSR が複雑、focus trap / aria-modal の Safari 実装が要注意 |
| C. 専用 dialog（HTML5 `<dialog>`）| native dialog 利用 | リスク: 古い Safari 互換性 / focus 制御 / a11y 設計の検証コスト |

**推奨: A**。

### 7.2 form 要素

| 要素 | 仕様 |
|---|---|
| **reason** | `<select>` 6 オプション（v4 reason）、required、初期値 `harassment_or_doxxing` |
| **detail** | `<textarea>`、任意、maxlength=2000、改行可、注意文「個人情報や URL を書かないでください」併記 |
| **reporter_contact** | `<input type="text">`、任意、maxlength=200、注意文「通報対応の連絡用にのみ使用します」併記 |
| **Turnstile** | Cloudflare Turnstile widget（既存 site-key 流用、§8 ユーザー判断 #1）|
| **送信** | `<button type="submit">`、disabled until Turnstile passes、submit 中は disabled |
| **thanks** | submit 成功後、報告 ID を表示しない or 表示（§15 ユーザー判断 #7）|

### 7.3 design 抽出

`design/mockups/prototype/screens-b.jsx` の `Report` および `pc-screens-b.jsx` の `PCReport` を参照。design system の既存 token を使用。新規 token は最小化。

### 7.4 Safari 対応

- macOS Safari + iPhone Safari で form / textarea / select / Turnstile widget の表示・タップ・キーボード表示を実機確認
- input zoom 防止: viewport meta tag は既存設定維持
- `<select>` の iOS Safari ネイティブ picker をデフォルトで使う（カスタム dropdown を作らない）
- Turnstile widget は responsive 対応済（Cloudflare 公式）

`.agents/rules/safari-verification.md` §「確認すべき最低限の項目」に従い、PR35 実装時に **必須 Safari 実機確認**を行う。

---

## 8. Bot / abuse 対策

### 8.1 Turnstile 戦略（§15 ユーザー判断 #1）

| 案 | 内容 | リスク / メリット |
|---|---|---|
| **A. PR35 で Turnstile 必須**（推奨）| Frontend で widget 表示 + Backend で siteverify | メリット: bot を確実に弾く、既存 infrastructure 流用可。リスク: UX 1 ステップ |
| B. PR35 では Turnstile なし、後日追加 | rate-limit のみで対応 | リスク: 通報悪用 / spam を弾けない、本番リスク高 |
| C. 簡易 hidden honeypot のみ | 隠し input + tarpit | リスク: 効果限定的、通報悪用に脆弱 |

**推奨: A**（Turnstile 必須）。理由:
- 通報フォームは公開 endpoint で認証なし、bot 標的になりやすい
- 既存 upload-verification の Turnstile site-key / secret-key を **流用可能**（同じ Cloudflare app）
- Backend 側 siteverify ロジックは `internal/uploadverification/infrastructure/turnstile/` で実装済、共通化または再利用

### 8.2 既存 Turnstile infrastructure 流用方法

| 観点 | 方針 |
|---|---|
| site-key（Frontend）| 既存の Turnstile site-key を流用 |
| secret-key（Backend）| **共通の Cloudflare Turnstile secret を別 endpoint でも使えるか確認**（PR35 実装時、Cloudflare dashboard で確認）|
| siteverify URL | 既存 `internal/uploadverification/infrastructure/turnstile/` のクライアント実装を `internal/turnstile/`（共通 package）にリファクタするか、Report 側で複製するか（§15 ユーザー判断 #8）|
| action 値 | upload-verification では action=`upload-intent`。Report 側は別 action（例 `report-submit`）にする |
| hostname | 同じ（app.vrc-photobook.com）|

### 8.3 rate-limit

PR35 では **MVP 簡易**: `source_ip_hash` を保存し、UsageLimit 集約（PR36）で連動。PR35 単独では rate-limit を実装しない。理由:
- UsageLimit 集約は PR36 で実装予定、ここで rate-limit を入れると 2 重実装になる
- Turnstile があれば bot 大量送信は概ね弾ける
- 実運用で spam が確認されたら PR36 を急ぐ

### 8.4 spam 通報対策

- detail / reporter_contact に URL や個人情報が含まれた場合のサーバ側サニタイズは **行わない**（運営が DB / cmd/ops で確認、本人視点で文字列を検証）
- React は escape するため XSS 安全
- 重大スパム（同一 photobook に大量通報）→ UsageLimit / 運営目視で対応

---

## 9. Ops 連携（cmd/ops）

### 9.1 サブコマンド一覧（PR35 MVP）

| 命令 | 安全性 |
|---|---|
| `ops report list [--status submitted] [--reason minor_safety_concern] [--limit 20] [--offset 0]` | 参照のみ |
| `ops report show --id <RID>` | 参照のみ（reporter_contact / detail / source_ip_hash 等を含む全情報、運営限定の CLI 出力）|
| `ops photobook hide --id <PID> --source-report-id <RID> ...` | PR34b の hide 拡張 |

PR35 では **`report mark-reviewed` / `report dismiss` / `report resolve-without-action` は実装しない**（state machine の他遷移は別 PR）。

### 9.2 出力ホワイトリスト

`ops report show` の出力に含めてよい:
- report_id
- target_photobook_id / target_*_snapshot
- reason / detail（**運営が見るのは可、ただし第三者には見せない、CLI 出力のため運営者のみ閲覧前提**）
- status / submitted_at / reviewed_at / resolved_at
- resolved_by_moderation_action_id
- reporter_contact（**運営が見るのは可、用途外利用禁止**、§10）
- source_ip_hash の **先頭数バイト or octet_length のみ**（生 hash 全値は出さない、§15 ユーザー判断 #9）

含めない:
- raw token / Cookie / DATABASE_URL / R2 credentials / storage_key 完全値

### 9.3 minor_safety_concern 優先表示

- `ops report list` のデフォルト sort を `(reason='minor_safety_concern' DESC, status='submitted' DESC, submitted_at DESC)` で並べる
- v4 §3.6 / §7.4 の最優先対応原則に従う

### 9.4 runbook

- PR34b の `docs/runbook/ops-moderation.md` に **§ Report 連携** セクションを追記する案
  - report list / show 手順
  - hide --source-report-id 連携手順
  - reporter_contact / detail を扱う際の個人情報保護注意
- または `docs/runbook/ops-report.md` を新規作成する案
- §15 ユーザー判断 #6

**推奨: ops-moderation.md に追記**（運営の「通報受付 → 通報確認 → hide で対応」がワンフロー、別 runbook に分けると参照漏れ）。

---

## 10. Moderation 連携

### 10.1 同 TX 連動の設計（v4 P0-5 / P0-19 / P0-20 / P0-21）

PR34b で moderation_actions に `source_report_id uuid NULL` カラムは既に存在。PR35 で次を追加実装:

```text
HidePhotobookByOperator(SourceReportID 付き):
  WithTx:
    1. SELECT photobooks（現状確認、既存）
    2. UPDATE photobooks SET hidden_by_operator=true（既存）
    3. INSERT moderation_actions（kind='hide', source_report_id=$rid, ...）
       ★ 既存の Insert を SourceReportID 引数を受け取る形に拡張
    4. INSERT outbox_events（event_type='photobook.hidden'、payload に source_report_id 追加）
       ★ payload 拡張のみ、event_type は既存
    5. **NEW**: SourceReportID が指定されている場合のみ:
       UPDATE reports SET status='resolved_action_taken',
                          resolved_by_moderation_action_id=$action_id,
                          resolved_at=$now
        WHERE id=$rid AND status IN ('submitted','under_review')
       0 行 UPDATE は ErrReportAlreadyTerminal（既に終端）として返す
```

5 要素同 TX で v4 P0-5 / P0-19 完全準拠。

### 10.2 unhide / restore は Report 自動更新しない（v4 P0-21）

- unhide UseCase は SourceReportID を受け取らない / 受け取っても reports は更新しない
- 運営が「unhide → report 状態を resolved_no_action に変更」したい場合は別途 cmd/ops（PR35 では未実装、別 PR で `report resolve-without-action` を追加）

---

## 11. Security / Privacy

### 11.1 Secret / token の取り扱い

| 対象 | 方針 |
|---|---|
| `DATABASE_URL` 実値 | env 経由のみ、CLI 引数・stdout・log に出さない |
| Turnstile secret-key | env 経由（既存 `TURNSTILE_SECRET_KEY` を流用または新規追加）|
| `REPORT_IP_HASH_SALT_V1` | 新規 Secret Manager に登録、Cloud Run env 経由 |
| reporter_contact / detail（運営入力ではなく通報者入力）| DB に保存。chat / log / commit / Outbox payload に値を出さない（§11.2）|
| source_ip_hash | DB に保存。生 IP は **絶対に保存しない / log に出さない** |
| storage_key / manage_url_token / draft_edit_token | Report フローでは扱わない |

### 11.2 Outbox payload の Privacy

`report.submitted` event の payload に **以下を含めない**（v4 §7 / 設計書 §7）:
- reporter_contact 本文
- detail 本文
- source_ip_hash の生値

**含めてよい**:
- event_version
- occurred_at
- report_id
- target_photobook_id
- reason
- has_contact: bool（reporter_contact が空でないことを示すフラグのみ）

理由: Outbox event は worker 経由で外部 (将来は email handler) に流れるため、漏洩リスクを最小化。

### 11.3 個人情報保持期間

v4 §7.2 / 設計書 §5 に従う:

| 対象 | 保持期間 |
|---|---|
| reports row | 対応完了から 90 日（PR35 では reconciler 未実装、後続 PR で）|
| reporter_contact | 対応完了から 30 日後 NULL 化 |
| source_ip_hash | 90 日後 NULL 化 |

PR35 では **reconciler を実装せず**、データだけ保存する。後続 PR（PR33e / PR39 系）で reconciler 実装。

### 11.4 XSS / CSRF / spam

- React の自動 escape で XSS 防御
- Cookie 不要 endpoint なので CSRF token 不要、代わりに Origin ヘッダ検証 + Turnstile
- spam: Turnstile + UsageLimit（PR36）

### 11.5 Actor 識別 / 通報者の匿名性

- PR35 は完全匿名通報を許容（reporter_contact 任意）
- 通報者の HTTP セッション情報は保存しない
- 通報者と作成者を結びつけない（同一作成元判定は source_ip_hash 経由のみ、UsageLimit でのみ使用）

---

## 12. Outbox 方針

### 12.1 報告

`SubmitReport` UseCase で **同 TX 2 要素**（reports INSERT + outbox_events INSERT）。worker handler は **no-op + structured log**。

### 12.2 minor_safety_concern の通知レベル

- handler 内で payload.reason を確認し、`minor_safety_concern` なら `slog.LevelWarn` 以上で log
- 他は `slog.LevelInfo`
- log message に `priority="urgent"` 等を埋めて Cloud Run logs alert で拾えるようにする
- **メール送信は実装しない**（Email Provider 未確定）。Cloud Run logs を運営が定期的に確認するか、Cloud Logging alert ポリシーを後続で設定

### 12.3 後続 PR で副作用 handler を入れる時の backfill 不要性

PR35 で event INSERT を始めておくことで、後日 Email Provider 確定 → 副作用 handler 追加時に backfill が要らない（PR33d で同じ思想）。

---

## 13. 実リソース操作

### 13.1 計画書 PR（本書）

- 実リソース操作: なし
- 影響範囲: docs のみ
- Secret 注入なし

### 13.2 実装 PR（PR35）

| 操作 | タイミング | STOP / 手順 |
|---|---|---|
| migration goose up（00016 reports + 00017 outbox CHECK 拡張）| 実装完了 + ローカル goose up + Cloud SQL 適用 | **Cloud SQL 適用 STOP** |
| `REPORT_IP_HASH_SALT_V1` を Secret Manager に登録 | migration 後 / Backend deploy 前 | **Secret 登録 STOP**（手動 GCP コンソール / `gcloud secrets create`）|
| Backend Cloud Build deploy | Secret 登録後 | runbook `docs/runbook/backend-deploy.md` 通り、repo root submit、`SHORT_SHA=...`|
| Cloud Run env / secretKeyRef に `REPORT_IP_HASH_SALT_V1` 追加 | deploy と同タイミング | service spec 更新（一時的に env 配置）|
| Cloud Run Job vrcpb-outbox-worker image 更新 | Backend deploy 後 | Job も新 image に更新（`report.submitted` handler 配線）|
| Workers redeploy（通報フォーム追加）| Frontend 完了後 | `npm run cf:build` + `wrangler deploy`、**STOP** |
| Cloudflare Turnstile（new action / 共有判断）| 実装中 | dashboard 操作要否は実装時に判断（既存 site-key 流用なら不要）|
| Cloud Run Jobs / Scheduler | 不要 | cmd/ops はローカル運用、Scheduler は PR33e で要否判断 |
| Cloud SQL 削除 / spike 削除 / Public repo 化 | 不要 | 別 PR ライン |

---

## 14. Tests 方針

### 14.1 単体（DB 不要 or DB あり）

| 範囲 | 内容 |
|---|---|
| `internal/report/domain/entity/report_test.go` | Report エンティティのコンストラクタ / I1〜I8 不変条件 |
| VO 6 種 test | ReportID / ReportReason / ReportDetail / ReporterContact / ReportStatus / TargetSnapshot |
| `internal/report/internal/usecase/submit_report_test.go` | 正常 / 不存在 photobook / draft / private / hidden / Turnstile 失敗 / detail 過長 / contact 過長 / 同 TX rollback |
| Repository test（DB あり）| INSERT / SELECT / status 更新 / 6 INDEX 経由クエリ |
| `internal/photobook/internal/usecase/...`（既存）| hide UseCase が SourceReportID 連動できることを test 拡張 |

### 14.2 統合 / 公開 endpoint

| 範囲 | 内容 |
|---|---|
| Backend `interface/http/report_handler_test.go` | 201 / 400 / 403 / 404 / 500 各シナリオ + Cookie 不要確認 + レスポンスヘッダ検証 |
| Turnstile siteverify mock | 成功 / 失敗パターン |
| source_ip_hash mock | env salt 渡しで決定論的 hash 確認 |

### 14.3 cmd/ops

| 範囲 | 内容 |
|---|---|
| `report list` / `report show` | 出力ホワイトリスト確認 / Secret grep / minor_safety_concern 優先表示 |
| `photobook hide --source-report-id` | 同 TX で reports.status 更新 / 失敗時 rollback |

### 14.4 Frontend

| 範囲 | 内容 |
|---|---|
| `app/(public)/p/[slug]/report/page.tsx` または同 component | render テスト（renderToStaticMarkup）/ form validation |
| Submit success → thanks view |
| Turnstile failure → error display |
| `npm run typecheck / test / build / cf:build` 全 OK |

### 14.5 Safari 実機

PR35 実装時に **必須**。`/p/<slug>/report` の form / textarea / select / Turnstile widget が macOS Safari + iPhone Safari で正しく表示・送信できることを確認。

---

## 15. 後回し事項 / 懸念

### 15.1 後回し（運用 / 別 PR）

| 項目 | 再開・解消条件 |
|---|---|
| Email 通知（運営 / 通報者）| Email Provider 確定後（PR32c 以降）|
| Web admin dashboard | MVP 範囲外（v4 §6.19）|
| `report mark-reviewed` / `report dismiss` / `report resolve-without-action` | PR35 拡張または別 PR（state machine 完全網羅）|
| reporter_contact / source_ip_hash の 90 日後 NULL 化 reconciler | PR33e / PR39 系の Reconcile |
| UsageLimit（rate-limit、abuse 抑止）| **PR36** |
| 通報悪用 / spam の自動検知 | UsageLimit 連動、PR36 |
| 通報数の analytics / dashboard | 後続任意 |
| reporter への結果通知 | Email Provider 確定後、運営判断 |

### 15.2 懸念

| 懸念 | 対応 |
|---|---|
| 公開 endpoint が DDoS 標的になる | Cloudflare Workers 経由 + Turnstile + UsageLimit（PR36）の多層防御。MVP は Turnstile 必須で抑制 |
| reporter_contact に個人情報（メアドや実名）が入る可能性 | UI 側に注意文表示、運営は個人情報を取り扱う旨を `.agents/rules/security-guard.md` に追記検討 |
| detail に第三者の個人情報が混入する可能性 | UI 注意文 + 運営目視。MVP では受け取り後に手動判断 |
| source_ip_hash の salt 漏洩で IP が逆算される可能性 | Secret Manager で隔離、salt version 管理、ローテーション可能設計 |
| 同一 IP からの大量通報 | UsageLimit（PR36）で対応、PR35 では source_ip_hash を保存するだけ |
| 通報直後の Outbox event が Cloud Run logs に流れすぎる | minor_safety_concern のみ Warn 以上、他は Info、log volume 観察で必要なら抑制 |

### 15.3 未検証

- Turnstile が `app.vrc-photobook.com/p/[slug]/report` の Worker 経由で正しく動作するか（既存 upload-verification は upload UI のみで検証済、別 path での挙動）
- Cloudflare Workers 経由の `Cf-Connecting-Ip` ヘッダが Backend に届くか
- macOS Safari + iPhone Safari の Turnstile widget レンダリングと a11y

---

## 16. ユーザー判断事項

| # | 判断項目 | 候補 / 推奨 | 影響 |
|---|---|---|---|
| 1 | **Turnstile 必須にするか** | **推奨: 必須**（案 A）。最小は rate-limit のみ（リスク高）| §8 / §11 |
| 2 | hide / deleted / purged photobook を通報対象に含めるか | **推奨: published + visible + not hidden のみ受付**（v4 ドメイン §6.1 は「削除済みも対応のため受付」だが MVP 簡素化）。最小は public のみ | §2 G4 / §5.2 |
| 3 | `REPORT_IP_HASH_SALT_V1` を新規 Secret として導入するか | **推奨: 新規 Secret 導入**。既存 salt 共有も可能だが PR36 連動を考えると独立した方が clean | §4.4 / §13 |
| 4 | IP 取得経路 | **推奨: `Cf-Connecting-Ip` 優先 + `X-Forwarded-For` フォールバック**。最小は `RemoteAddr`（信頼性低）| §6.3 |
| 5 | Frontend 通報フォーム配置 | **推奨: 案 A（`/p/[slug]/report` 別ページ）**。modal / dialog は SSR / a11y / Safari 互換性のリスク | §7.1 |
| 6 | runbook 配置 | **推奨: `docs/runbook/ops-moderation.md` に § Report 連携を追記**。新規 ops-report.md は分散リスク | §9.4 |
| 7 | thanks view で report_id を表示するか | **推奨: 表示しない**（reporter は ID を再利用しない、表示すると個人特定リスク微増）| §7.2 |
| 8 | Turnstile siteverify ロジックの共通化 | **推奨: PR35 で `internal/turnstile/` 共通 package に切り出し**。最小は Report 側で複製（後で技術的負債化）| §8.2 |
| 9 | `ops report show` での source_ip_hash 表示 | **推奨: 先頭 4 バイト hex のみ**（同一 IP 判定の手がかり程度に絞る）。最小は完全非表示、最大は完全表示（運営限定だが log 残存リスク）| §9.2 |
| 10 | Outbox `report.submitted` handler の MVP 挙動 | **推奨: no-op + structured log（minor_safety_concern は Warn 以上）**。最小は完全 no-op、最大は MVP で Slack webhook も実装（Email 不要だが実装コスト+） | §12.2 |
| 11 | Cloud SQL は引き続き `vrcpb-api-verify` 名のままで PR35 を進めるか | **推奨: そのまま**（本番化 / rename は PR39）| §13 |

---

## 17. 完了条件

PR35a 計画書（本書）の完了条件:

- [ ] §1 現状整理（PR34b との接続点）が事実と一致
- [ ] §2 ゴール / §3 スコープがユーザー承認可能
- [ ] §4 DB 設計が v4 設計書と整合
- [ ] §5〜§12 が v4 設計書（既存）+ PR34b 実装と整合
- [ ] §13 実リソース操作が実装 PR まで発生しないことが明示
- [ ] §14 Tests 方針が PR35 実装 PR で実行可能
- [ ] §15 後回し事項が roadmap / 別 PR に紐付け済
- [ ] §16 ユーザー判断事項 11 件が漏れなく列挙
- [ ] check-stale-comments + Secret grep をクリア（commit 時に確認）

PR35 実装 PR（PR35b）の完了条件:

- migration 00016 / 00017 が Cloud SQL に適用済
- `REPORT_IP_HASH_SALT_V1` Secret 登録済 / Cloud Run env に注入
- `internal/report/` パッケージが domain / VO / Repository / UseCase / test 揃っている
- 公開 endpoint `POST /api/public/photobooks/{slug}/reports` が Turnstile 必須で動作
- Outbox `report.submitted` event が同 TX で INSERT、worker handler は no-op + log
- `cmd/ops report list / show` が動作
- `cmd/ops photobook hide --source-report-id` が同 TX で reports.status 更新
- Frontend `/p/[slug]/report` ページが Safari 実機で送信成功
- Backend deploy 完了 / Workers redeploy 完了
- runbook（`docs/runbook/ops-moderation.md` § Report 連携 追記）整備
- pr-closeout（コメント整合 / Secret grep / 後回し記録）通過

---

## 18. 関連ドキュメント

- 上流設計: [`docs/design/aggregates/report/ドメイン設計.md`](../design/aggregates/report/ドメイン設計.md) / [`データモデル設計.md`](../design/aggregates/report/データモデル設計.md)
- 業務知識: [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §3.6 / §3.7 / §5.4 / §6.11 / §7.2 / §7.3 / §7.4
- 横断: [`docs/design/cross-cutting/outbox.md`](../design/cross-cutting/outbox.md) §4.2
- ADR: [`docs/adr/0002-ops-execution-model.md`](../adr/0002-ops-execution-model.md) / [`0006-email-provider-and-manage-url-delivery.md`](../adr/0006-email-provider-and-manage-url-delivery.md)
- PR34b 計画書: [`docs/plan/m2-moderation-ops-plan.md`](./m2-moderation-ops-plan.md)
- PR34b 結果: [`harness/work-logs/2026-04-28_moderation-ops-result.md`](../../harness/work-logs/2026-04-28_moderation-ops-result.md)
- runbook: [`docs/runbook/ops-moderation.md`](../runbook/ops-moderation.md) / [`backend-deploy.md`](../runbook/backend-deploy.md)
- ロードマップ: [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md) PR35 章

---

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-29 | 初版作成。PR35 MVP（SubmitReport + cmd/ops report list/show + photobook hide --source-report-id 連動）スコープと v4 完全設計（5 ReportStatus + reconciler + email 通知）の差分整理。Turnstile 必須 / IP hash 保存 / 同 TX 5 要素 連動 / Frontend 別ページ + ユーザー判断事項 11 件 |
