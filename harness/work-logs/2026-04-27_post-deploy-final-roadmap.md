# 2026-04-27 公開ローンチまでの最終ロードマップ + design 参照ルール

> 位置付け: PR12（Backend Domain Mapping、証明書発行待ち）以降、公開ローンチまでの **最終ロードマップ** を確定する。
> 既存ロードマップ [`2026-04-26_project-roadmap-overview.md`](./2026-04-26_project-roadmap-overview.md) の続編であり、
> ここから先は本書を「現在地マーカー」として参照する。
>
> **CLAUDE.md の「現在地」を本書に向け直す価値はあるが、それは PR12 完了後に別途実施する**。

## 0. 本書の使い方

- 各 PR には **完了条件**、**design 参照点**、**課金リスク**、**ユーザー操作 vs Claude Code 操作の分担**を明記
- 計画書はここから派生する形で作成（`docs/plan/m2-*.md` に積み上げる）
- 道を踏み外しそうになったら本書 §A〜§E のフェーズ表に戻る
- design 参照点は §F の design 資産マッピングで定義する

---

## A. M2 ドメイン疎通フェーズ（PR12〜PR17、目安 1〜2 週間）

### PR12（進行中）: Backend Domain Mapping

- 証明書発行完了後に以下を実施:
  - `curl https://api.vrc-photobook.com/health` 200 確認
  - `/readyz` 200 ready 確認
  - token exchange 400 / 401 確認（Cache-Control: no-store / Set-Cookie 無し）
  - `openssl s_client` で証明書 issuer = Google Trust Services 確認
  - Cloud Run logs に raw token / Cookie / DSN 漏れなし grep
- 作業ログ: `harness/work-logs/2026-04-27_backend-domain-mapping-result.md`（コミット）

### PR13: Frontend Workers deploy 計画書（**実施前必須**）

`docs/plan/m2-frontend-workers-deploy-plan.md` を作成。以下を含める:

- **`COOKIE_DOMAIN` の Workers 注入方式**:
  - Next.js OpenNext は build 時に `process.env.COOKIE_DOMAIN` を inline する
  - Server-only env のため `NEXT_PUBLIC_*` 接頭辞は付けない（Client Component から見えてはいけない）
  - 注入方法は 3 通り: (a) ローカル `.env.production` を build 時に読ませる / (b) `wrangler.jsonc` の `vars` セクション / (c) Workers Secrets API
  - 推奨: **(a) ローカル `.env.production` で build → bundle に inline**（Server-only env が Workers runtime に届かない問題を回避、PR5 段階の方針と整合）
- 切戻し手順、`wrangler deployments list` で旧 deployment への traffic 切戻し
- **PR16 前の実 token 取得手順テンプレ**を §X に併記（コードはコミットしない）

### PR14: Frontend Workers deploy 実施

- `npm --prefix frontend run cf:build`
- `wrangler deploy` （実 deploy、ユーザー操作）
- `https://vrcpb-frontend.<account>.workers.dev/` で動作確認
- middleware ヘッダ二重出力チェック
- **このとき `app.vrc-photobook.com` には繋がない**

### PR15: app.vrc-photobook.com Custom Domain 設定

- Cloudflare Dashboard → Workers & Pages → vrcpb-frontend → Settings → Triggers → Custom Domains（**ユーザー手動**）
- DNS / 証明書は Cloudflare 自動
- HTTPS 疎通: `curl https://app.vrc-photobook.com/`
- 不正 token redirect で reason redirect が機能 + Cookie 属性確認

### PR16: Frontend ↔ Backend 実 token 結合

- `~/scratch/` 経由で raw token を生成（**repo 外、コミットしない、PR13 §X のテンプレを参照**）
- ブラウザで `https://app.vrc-photobook.com/draft/<raw>` にアクセス
- DevTools で:
  - 302 redirect → `/edit/<id>`
  - URL から token 消失
  - `Set-Cookie: vrcpb_draft_<id>=...; Domain=.vrc-photobook.com; HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=...`
  - 再読込で session 維持
- manage 経路も同様
- Cloud Run / Workers logs に raw 漏れなし

### PR17: Safari / iPhone Safari 実機確認

- macOS Safari と iPhone Safari で PR16 と同じシナリオ
- `safari-verification.md` チェックリスト全項目
- 24 時間後 / 7 日後の Cookie 残存観察開始（起点を作業ログに記録）
- iPad Safari / Private Browsing も範囲に応じて確認

### **PR17 完了後の必須判断: Cloud SQL 残置 / 一時削除**

- **Image / R2 にすぐ着手する**（数日以内に PR18 に進む）→ Cloud SQL **残置**
- **数日以上空く**（他作業に時間が掛かる）→ Cloud SQL **一時削除**（`m2-cloud-sql-short-verification-plan.md` §11 の手順）
- 検証用 DB (`vrcpb-api-verify`) を本番相当に使い続けるのは避ける（ズルズル使用は運用上のリスク）
- 判断を先送りせず、PR17 完了直後に必ず実施する

---

## B. M2 Image / Upload フェーズ（PR18〜PR24、目安 2〜4 週間）

### PR18: Image aggregate domain + migration

- `backend/internal/image/`（Image / ImageVariant / StorageKey VO）
- migration `00005_create_images.sql` + `00006_create_image_variants.sql`
- presigned URL 発行 service の interface 定義（R2 実装は PR21）

### PR19: Photobook ↔ Image 連携（pages / photos）

- migration: `pages` / `photos` / `page_metas`
- Photobook 集約に `addPage` / `removePage` / `addPhoto` / `removePhoto`
- `cover_image_id` への FK を `photobooks` に追加（PR9a で意図的に保留）

### PR20: upload-verification（Turnstile セッション化）

- `internal/auth/upload-verification/`
- migration `00007_create_upload_verification_sessions.sql`
- Turnstile siteverify HTTP client（テスト siteKey で実装）
- `IssueUploadVerification` / `ValidateAndConsume` UseCase
- 「20 intent / 30 分 / atomic 消費」を実 DB test で確認

### PR21: R2 設定 + presigned URL 発行

- Cloudflare R2 バケット `vrcpb-images` 作成（**ユーザー手動**）
- API token / endpoint を Secret Manager に登録
- AWS SDK Go S3 API client で presigned URL 発行
- `upload-intent` UseCase で Turnstile session 検証 + presigned URL 発行
- `complete-upload` UseCase で R2 にオブジェクト存在確認 + Photobook 紐付け

### PR22: 編集 UI 最小骨格（**design 参照、§F-1**）

- `app/(draft)/edit/[photobookId]/page.tsx` 本実装
- Server Component で `ValidateSession` (draft) → Backend `/api/photobooks/{id}/edit-view`
- 画像アップロード UI（Turnstile widget + presigned URL）
- 並び替え / 削除（楽観ロック）
- このタイミングで Backend に **CORS middleware 追加**（Client Component の credentials: include 対応）
- **design 参照**: `design/mockups/prototype/screens-a.jsx` の `Edit` コンポーネント、`pc-screens-a.jsx` の `PCEdit`、共通 `shared.jsx` Icon set + Photo / Av、`styles.css` / `pc-styles.css` のトークン

### PR23: 公開ページ / 管理ページ最小骨格（**design 参照、§F-2**）

- `app/(public)/p/[slug]/page.tsx`（誰でも閲覧、SSR + OGP）
- `app/(manage)/manage/[photobookId]/page.tsx` 本実装（reissueManageUrl ボタン等）
- **design 参照**: prototype の `Viewer` / `PCViewer` / `Manage` / `PCManage` / `UrlRow`

### PR24: Backend deploy 自動化（Cloud Build）

- `cloudbuild.yaml` 整備（`docker build` → Artifact Registry → `gcloud run deploy`）
- GitHub Actions or 手動トリガー
- 本実装後の deploy がワンコマンド化
- ここで `cloudbuild.googleapis.com` を有効化（PR12 段階で skip した分）

---

## C. M2 Outbox / 通知 / 運営（PR25〜PR29、目安 2〜3 週間）

### PR25: Outbox table + 同一 TX INSERT

- migration `outbox_events`
- Photobook UseCase（Publish / Reissue / SoftDelete 等）に Outbox INSERT を同一 TX で追加
- I-O1 不変条件の test

### PR26: cmd/outbox-worker（Cloud Run Jobs）

- `backend/cmd/outbox-worker/main.go`
- pending event poll → 処理 → mark done
- Cloud Run Jobs deploy + Cloud Scheduler

### PR27: SendGrid 設定 + ManageUrlDelivery

- SendGrid API key 取得（**ユーザー操作**）
- 送信ドメイン認証（DKIM/SPF レコードを Cloudflare DNS に追加）
- `ManageUrlDelivery` 集約 + `internal/notification/`
- Outbox handler で `ManageUrlReissued` / `PhotobookPublished` 等を契機にメール送信

### PR28: Moderation 集約

- `internal/moderation/` + `cmd/ops/`（CLI、ADR-0002 準拠）
- hide / unhide / softDelete / restore / purge / reissueManageUrl の運営 entry
- `ModerationAction` 履歴

### PR29: OGP 独立管理

- `photobook_ogp_images` table
- 公開ページの OGP 自動生成（後追い non-blocking、Cloud Run Jobs）

---

## D. M3 公開ローンチ準備（PR30〜PR33、目安 1〜2 週間）

### PR30: Report 集約

- 通報受付 + 運営対応フロー
- **design 参照**: prototype の `Report` / `PCReport` 画面

### PR31: UsageLimit 集約

- 公開数制限・抑止機構（業務知識 v4 §6.x）

### PR32: 利用規約 / プライバシーポリシー / About / LP（**design 参照、§F-3**）

- `/`、`/terms`、`/privacy`、`/about`
- 非公式表記（`m2-domain-purchase-checklist.md` §2.2 文言案）
- **design 参照**: prototype の `LP` / `PCLP` / `PCTrust`、コンセプト画像 (`design/mockups/concept-images/`)

### PR33: ローンチ前チェック + spike 削除

- 既存 spike Cloud Run / Workers / Artifact Registry 削除
- 検証用 Cloud SQL `vrcpb-api-verify` を **本番相当の名前 (例: `vrcpb-api-prod`) へ migrate** する判断（or rename）
- Budget Alert 再設計
- 監視 / アラート（Cloud Monitoring）
- ローンチ告知の OGP 確認

---

## E. ローンチ後（PR34+）

- iPad Safari / Edge / Firefox 動作再確認
- 24h / 7 日後 / 30 日後 ITP 観察結果の運用反映
- パフォーマンス計測（Lighthouse / Cloud Trace）
- Cloud SQL の本番 instance 名へ rename / HA / バックアップ ON
- Frontend / Backend の CI/CD 充実
- 利用ログから機能改善

---

## F. design 資産参照ルール（**フロント実装の道標**）

### F-0. 全体方針

- `design/mockups/prototype/` を **本実装の暫定 Single Source of Truth** として扱う
- ただし「探索段階の成果物」（`design/mockups/README.md`）であり、**そのままコピペで実装しない**
- 本実装は `frontend/` の Tailwind v3 + React 19 + Server Component で組む
- prototype は React + plain CSS（Babel standalone）なので、**設計値の抽出元** として参照
- `design/design-system/` は現在空。**PR22 着手時に同時に整備**（§F-4）

### F-1. PR22 編集 UI で参照すべき資産

| 用途 | 参照先 | 抽出する内容 |
|---|---|---|
| カラートークン | `design/mockups/prototype/styles.css` (`--teal` 等) | Tailwind config の `theme.extend.colors` に転記 |
| タイポ | 同上 (`.t-h1`, `.t-h2`, `.t-body`, `.t-sm`, `.t-xs`) | Tailwind の `fontSize` / `fontWeight` |
| 余白・角丸・shadow | 同上 (`--radius` 系, `--shadow*`) | Tailwind の `borderRadius` / `boxShadow` |
| Icon | `design/mockups/prototype/shared.jsx` の `Icon` 40 種 | 必要な分だけ SVG コンポーネントとして frontend/lib/icons/ に移植 |
| 編集画面レイアウト | `screens-a.jsx` の `Edit`、`pc-screens-a.jsx` の `PCEdit`（3 列） | 構造 / 情報階層 / フォームパターン |
| Photo placeholder | `shared.jsx` の `Photo` (`v-a`〜`v-f`) | 6 種グラデーションを Tailwind `bg-gradient-to-*` で再現、本番では実画像に差替え |
| Avatar | `shared.jsx` の `Av` | イニシャル + 5 色グラデ、CSS 数値を Tailwind 化 |
| Steps | `shared.jsx` の `Steps`、`pc-shared.jsx` の `PCSteps` | 3 段階プログレスバー |

**注意**:
- iOS frame / browser-window は **プレビュー専用**、本実装に流用しない
- prototype の React コンポーネントを直接 import しない（依存方向が逆になる）
- prototype の `inline style` ではなく、Tailwind class + design-system tokens で書き直す

### F-2. PR23 公開・管理ページで参照すべき資産

| 用途 | 参照先 |
|---|---|
| 公開ページ レイアウト | `screens-b.jsx` の `Viewer`、`pc-screens-b.jsx` の `PCViewer`（2 列） |
| 管理ページ レイアウト | `screens-b.jsx` の `Manage`、`pc-screens-b.jsx` の `PCManage` |
| URL 表示 | `shared.jsx` の `UrlRow`（teal / violet 切替） |
| 公開完了画面 | `screens-a.jsx` の `Complete`、`pc-screens-a.jsx` の `PCComplete` |
| 「URL を失うと編集できない」警告 | `Complete` 画面のテキスト |

### F-3. PR32 LP / 利用規約で参照すべき資産

| 用途 | 参照先 |
|---|---|
| LP / Hero | `screens-a.jsx` の `LP`、`pc-screens-a.jsx` の `PCLP` |
| Trust strip | `pc-shared.jsx` の `PCTrust`（4 アイテム） |
| Photobook type の視覚化 | `design/mockups/concept-images/` 15 枚（VRC type ごとのコンセプトビジュアル） |
| Header / CTA | `pc-shared.jsx` の `PCHeader`（「無料で作る」CTA） |

### F-4. design-system/ 整備計画（PR22 と同時並行）

prototype から抽出して `design/design-system/` に正典として配置:

- `colors.md`（teal / neutral / status の token 表）
- `typography.md`（font-family / size / weight / line-height）
- `spacing.md`（gap / padding / margin の段階値）
- `radius-shadow.md`
- `components.md`（Photo / Av / TopBar / Steps / UrlRow 等の仕様）

frontend 側は同じ値を `tailwind.config.ts` の theme.extend に反映。**design-system/ と tailwind.config.ts の二重管理を避けるため、token は JSON で持って両方から読む** 形を PR22 で検討。

### F-5. Photobook type の扱い

prototype 内の type 配列:
```javascript
// CreateStart / PCCreateStart の types
[
  { k:'event',     t:'イベント',     v:'a' },
  { k:'morning',   t:'おはツイ',     v:'d' },
  { k:'portfolio', t:'作品集',       v:'c' },
  { k:'avatar',    t:'アバター紹介', v:'e' },
  { k:'free',      t:'自由作成',     v:'f' },
];
```

業務知識 v4 で確定の type は `event / daily / portfolio / avatar / world / memory / free`（7 種）で、prototype の `morning` (= daily 相当) は **命名差**。本実装では業務知識を正典として `daily` 採用。

**type ごとのアイコン / カラー**は本実装で定義（prototype には placeholder の photo variant のみ）。`design/mockups/concept-images/` の 15 枚から各 type のキービジュアルを 1 枚ずつ選定するのは PR32 の判断。

---

## G. 修正点まとめ（直前のロードマップ提案からの変更）

| 元案 | 修正後 | 理由 |
|---|---|---|
| PR13 を計画書 1 本でサクッと | **PR13 計画書に「COOKIE_DOMAIN の Workers 注入方式」と「PR16 前の token 取得テンプレ」を必ず含める** | NEXT_PUBLIC_ 系と Server-only env の事故、token 生成手順の毎回ぶれを防ぐ |
| PR16 で `~/scratch` 任意 | **PR13 計画書 §X に repo 外スクリプトのテンプレ手順を文書化**（コードはコミットしない） | 毎回の試行錯誤を排除、`security-guard.md` 違反リスクを事前に塞ぐ |
| PR17 完了後の Cloud SQL 判断は推奨レベル | **必須判断とする**（残置 or 一時削除を作業ログに明記） | 検証 DB をなし崩しに本番相当扱いするリスクを排除 |
| design/ への参照は暗黙 | **§F として PR22 / PR23 / PR32 の各段階で参照すべき design 資産を明示** | フロント実装が prototype を見ずに進む / コピペで進む両方を防ぐ |
| design-system/ 空のまま | **PR22 と同時に design-system/ を整備**（colors.md / typography.md / spacing.md / components.md） | 本実装と prototype の二重管理を避け、long-term の正典を作る |

---

## H. 課金 / リスクの節目（再掲、本書時点で更新）

| 節目 | 注意点 |
|---|---|
| **PR12 完了時点** | Cloud SQL 累計 ~1 日（¥55）。証明書発行待ちで進まない場合、4 時間以上掛かるなら Cloud SQL を一時削除して PR13 に進むことを検討 |
| PR17 完了時点 | 累計 ~1 週間（¥390）。**§A の必須判断**を実施 |
| PR21 完了 | R2 課金開始（最小） |
| PR27 完了 | SendGrid 課金（無料 100 通/日 で M2 充分） |
| PR33 完了 | spike 削除で課金が下がる |

---

## I. ユーザー操作と Claude Code 操作の最終分担

### ユーザー手動が必須

- Cloudflare Dashboard 操作（DNS レコード追加・Custom Domain クリック）
- GCP の API 有効化承認（Budget / Billing 関連）
- Workers / Cloud Run の手動 deploy 承認
- Secret 値の登録（実値はユーザーが stdin で渡す or Secret Manager UI）
- ブラウザ / Safari 実機確認
- 失敗時の切戻し承認

### Claude Code が実施

- gcloud / wrangler / dig / curl / openssl コマンド実行
- 設定値の提案 / 雛形作成
- README / 計画書 / 作業ログ更新
- ログ・diff のセキュリティ grep
- 失敗時の failure-log 起票
- design 資産の参照点抽出 / Tailwind config への転記提案

---

## J. 関連ドキュメント

- [プロジェクト全体ロードマップ（M1 完了時点）](./2026-04-26_project-roadmap-overview.md)
- [M2 ドメイン購入チェックリスト + 購入記録](../../docs/plan/m2-domain-purchase-checklist.md)
- [M2 Domain Mapping 実施計画](../../docs/plan/m2-domain-mapping-execution-plan.md)
- [M2 Backend Cloud Run Deploy 計画](../../docs/plan/m2-backend-cloud-run-deploy-plan.md)
- [M2 Cloud SQL 短時間検証 計画](../../docs/plan/m2-cloud-sql-short-verification-plan.md)
- [Cloud SQL 短時間検証 実施結果（残す判断含む）](./2026-04-26_cloud-sql-short-verification-result.md)
- [Backend Cloud Run deploy 実施結果](./2026-04-26_backend-cloud-run-deploy-result.md)
- [`design/README.md`](../../design/README.md) / [`design/mockups/README.md`](../../design/mockups/README.md)
- [業務知識 v4](../../docs/spec/vrc_photobook_business_knowledge_v4.md)
