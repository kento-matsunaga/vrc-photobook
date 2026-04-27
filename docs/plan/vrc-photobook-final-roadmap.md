# VRC PhotoBook 公開ローンチまでの最終ロードマップ（新正典）

> 作成日: 2026-04-27
> 位置付け: PR23（image-processor）完了時点での **現在地マーカー兼新正典ロードマップ**。
> 旧正典 [`harness/work-logs/2026-04-27_post-deploy-final-roadmap.md`](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)
> は M2 早期（PR12〜PR23）の進行に対する正典として役割を終え、**PR24 以降は本書を新正典とする**。
>
> CLAUDE.md からも本書を「最初に確認するロードマップ」として参照する。

---

## 0. 必ず最初に確認するルール

新しい PR / サイクルに着手する前に、以下を **必ずこの順で**確認する。

1. **本書 §1（現在地）と §3（新 PR 番号体系）を確認**し、自分が今どの PR にいるかを特定する
2. PR 番号と対象範囲がズレた場合、**実装前に本書を更新**する。実装後に書き換えない
3. 旧ロードマップ（`2026-04-27_post-deploy-final-roadmap.md`）と矛盾した場合、**本書を優先**する
4. `design/mockups/prototype/` は **参照元（Single Source of Truth の暫定）** であり、
   `frontend/` から **直接 import しない**。値の抽出のみ行う
5. 実装 PR の前に **計画書 PR を挟むかどうか**を必ず判断する（複雑度 / 影響範囲が大きい場合は挟む）
6. **実リソース操作（Cloud Run Jobs / Cloud Build API enable / Scheduler / DNS / Secret 登録 /
   Dashboard 操作）は停止ポイントを置き、実施前にユーザー判断を仰ぐ**
7. **raw token / Cookie / R2 credentials / DATABASE_URL / Secret 実値**は、
   チャット / work-log / commit / コードコメント に書かない（`.agents/rules/security-guard.md`）
8. commit author は **`kento-matsunaga` 単独**、`Co-Authored-By: Claude` は付けない
9. Cookie / redirect / OGP / レスポンスヘッダ / モバイル UI / token→session 交換 を
   変更したら **macOS Safari + iPhone Safari** で確認（`.agents/rules/safari-verification.md`）
10. 失敗を検知したら **`harness/failure-log/` に起票** して再発防止に繋げる
    （`.agents/rules/feedback-loop.md`）

---

## 1. 現在地（2026-04-27 PR23 締め時点）

### 1.1 commit / revision

- **最新 commit**: `1dcdf09 docs(work-log): record PR23 cloud run revision update result`
- **直前の機能 commit**: `609b1f2 feat(backend): add image processor for generated variants`
- **Cloud Run vrcpb-api revision**: `vrcpb-api-00007-8dv`（image: `vrcpb-api:609b1f2`、100% traffic）
- **Cloud Run vrcpb-api rollback**: `vrcpb-api-00006-wdg`（image: `vrcpb-api:8928be8`）
- **Cloud Workers Frontend deployment**: `6860b721-4ddb-456d-9f2d-be7f9d62bbe7`
- **Cloud SQL**: `vrcpb-api-verify`（asia-northeast1、検証用名のまま **本番相当に使用継続**）

### 1.2 実装済み（重要なものから）

| 領域 | 内容 | 関連 PR / 作業ログ |
|---|---|---|
| インフラ | Cloud Run `vrcpb-api`（asia-northeast1） | `2026-04-26_backend-cloud-run-deploy-result.md` |
| インフラ | Cloud SQL `vrcpb-api-verify`（PostgreSQL 16） | `2026-04-26_cloud-sql-short-verification-result.md` |
| インフラ | Custom Domain `api.vrc-photobook.com` (Cloud Run Domain Mapping) | `2026-04-27_backend-domain-mapping-result.md` |
| インフラ | Custom Domain `app.vrc-photobook.com` (Workers Custom Domain) | `2026-04-27_frontend-custom-domain-result.md` |
| インフラ | Workers Frontend deploy（OpenNext） | `2026-04-27_frontend-workers-deploy-result.md` |
| インフラ | Cloudflare R2 bucket `vrcpb-images`、API token 注入済 | `2026-04-27_r2-presigned-url-real-upload-result.md` |
| インフラ | Cloudflare Turnstile（Production widget + secret 注入済） | `2026-04-27_frontend-upload-ui-result.md` |
| 認可 | token URL → HttpOnly Cookie session 交換（draft / manage 両方） | `2026-04-27_frontend-backend-real-token-e2e-result.md` |
| 認可 | macOS Safari / iPhone Safari 実機 token 結合確認（PR17） | `2026-04-27_safari-real-token-e2e-result.md` |
| 認可 | session middleware（draft / manage 共通） | `m2-session-auth-implementation-plan.md` 完了範囲 |
| ドメイン | Image 集約（domain / VO / Repository / state machine） | `m2-image-upload-plan.md` 完了範囲 |
| ドメイン | Photobook 集約（VO / OCC / pages / photos / page_metas） | `m2-photobook-image-connection-plan.md` 完了範囲 |
| ドメイン | UploadVerificationSession 集約（atomic consume + Turnstile siteverify） | `m2-upload-verification-plan.md` 完了範囲 |
| 機能 | upload-verifications endpoint（Turnstile session 発行） | PR22 work-log |
| 機能 | upload-intent endpoint（presigned PUT URL 発行） | `m2-r2-presigned-url-plan.md` 完了範囲 |
| 機能 | complete-upload endpoint（HeadObject + processing 遷移） | 同上 |
| 機能 | image-processor（CLI）: original 取得 → JPEG 再エンコード → display/thumbnail PUT → MarkAvailable / MarkFailed | `m2-image-processor-plan.md` PR23 |
| 機能 | display / thumbnail variant 生成（plan §10 通り JPEG 統一） | 同上 |
| 機能 | image-processor binary を `vrcpb-api` image に同梱（PR24+ Cloud Run Jobs での流用準備） | PR23 work-log |
| Frontend | Next.js skeleton + middleware + Workers deploy | PR13/14/15 |
| Frontend | `/draft/[token]` / `/manage/token/[token]` Route Handler（token→Cookie 交換） | PR16 |
| Frontend | `/edit/[photobookId]` upload UI（最小骨格、Turnstile + presigned PUT + complete） | PR22 |
| Frontend | HEIC 拒否（PR22.5 で content_type / 拡張子で多層ガード） | PR22.5 commit history |

### 1.3 未実装（公開ローンチまでに必要）

#### Frontend
- 公開 Viewer (`/p/[slug]` 等) — display variant の本格表示
- 管理ページ (`/manage/[photobookId]`) — manage URL 再発行 / 公開設定
- 編集 UI 本格機能（photo grid / caption 編集 / page-photo 並び替え / cover 設定 / publish settings）
- Publish flow UI（completion screen / URL コピー / manage URL 控え）
- LP (`/`) / `/terms` / `/privacy` / `/about`
- design system 整備（`design/design-system/` への token 抽出、`tailwind.config.ts` への反映）

#### Backend
- Outbox table + 同一 TX INSERT（PhotobookPublished / ManageUrlReissued / ImageBecameAvailable / ImageFailed 等）
- `cmd/outbox-worker`（pending event poll → 実行 → mark done）
- Cloud Run Jobs（image-processor / outbox-worker のスケジューリング）
- **メール Provider 再選定 + ManageUrlDelivery 集約**
  - ADR-0006（2026-04-28）で **MVP のメール送信機能を必須要件から外す**ことを確定
    （SendGrid 個人不可 / SES 申請不通過のため）
  - MVP は Complete 画面で 1 度だけ表示する方式（PR28 で実装済）が標準
  - 後続 PR32 で個人契約可能な Provider を再選定し、採用確定後にメール送信を再開
- Moderation 集約 + `cmd/ops`（hide / unhide / softDelete / restore / purge / reissueManageUrl）
- Report 集約（通報受付 + 運営対応）
- UsageLimit 集約（公開数制限 / abuse 抑止）
- OGP 独立管理（`photobook_ogp_images` / 公開ページ OGP 自動生成）
- HEIC 本対応（libheif + cgo 切替、Dockerfile 改修）
- R2 orphan Reconcile（display/thumbnail PUT 成功 → DB 失敗、failed image の original 残存等を 7 日後に cleanup）

#### 運用 / インフラ
- Backend CI/CD: PR29 で `gcloud builds submit` 経由の **manual submit 方式**を導入済。
  以下は **PR29 で先送り**、PR40 / PR41+ で扱う:
  - Cloud Build trigger オブジェクト（GCP Console からワンクリック起動）→ PR40
  - GitHub App / Cloud Build GitHub connection（2nd gen）→ PR38 + PR40
  - tag trigger（`release-*` push で自動）→ PR40 / PR41+
  - main push 自動 deploy → PR41+
  - Artifact Registry retention policy → PR40
- **Frontend Workers deploy 自動化**（現状 `npm run cf:build` + `wrangler deploy` 手動）→ PR41+
- 本番 Cloud SQL への移行（または `vrcpb-api-verify` の rename / 本番化）
- spike 環境削除（spike Cloud Run / Workers / Artifact Registry / R2 bucket）
- Public repo 化判断 + 履歴 secret scan
- 本番監視 / Budget Alert 再設計 / Error Reporting

---

## 2. 旧ロードマップとのズレ

旧 [`harness/work-logs/2026-04-27_post-deploy-final-roadmap.md`](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)
の PR 番号体系と実際の進行に **以下のズレ**がある。

| 旧ロードマップ | 実際の進行 |
|---|---|
| PR22 = 編集 UI 最小骨格 | **PR22 = upload UI 最小骨格 + Turnstile + Safari 確認**（編集機能本体は未着手） |
| PR23 = 公開ページ / 管理ページ最小骨格 | **PR23 = image-processor + variant 生成 + Cloud Run image 更新**（公開ページ / 管理ページは未着手） |
| PR24 = Backend deploy 自動化 | **未着手** |
| PR25 = Outbox table | **未着手** |
| PR26 = outbox-worker | **未着手** |
| PR27 = SendGrid + ManageUrlDelivery | **未着手** |
| PR28 = Moderation 集約 | **未着手** |
| PR29 = OGP 独立管理 | **未着手** |
| PR30 = Report 集約 | **未着手** |
| PR31 = UsageLimit 集約 | **未着手** |
| PR32 = LP / terms / privacy / about | **未着手** |
| PR33 = ローンチ前チェック + spike 削除 | **未着手** |

旧ロードマップの PR23 以降は **進行とズレた状態で「過去の正典」**。今後は本書を参照する。

---

## 3. 新 PR 番号体系（PR24〜PR41+）

各 PR は §4 のテンプレに沿って必須 9 項目を満たすこと。

### PR24: 公開 Viewer / 管理ページ 計画書

- **目的**: 公開 Viewer (`/p/[slug]`) と管理ページ (`/manage/[photobookId]`) の最小骨格を実装するための計画書を作る。Viewer が機能するための最小 publish 経路（status='published' へ遷移できる UseCase + slug 生成）も計画範囲に含める
- **実装するもの**:
  - 計画書 `docs/plan/m2-public-viewer-and-manage-plan.md`
  - 認可（Viewer は誰でも / Manage は manage Cookie）
  - display variant の Viewer での提示方式（presigned GET URL? Public R2? Workers proxy?）の判断
  - 公開 slug の発行ルール（業務知識 v4 §6.x との整合）
  - Safari 確認チェックリスト
  - PR25（実装）への引き継ぎ事項
- **実装しないもの**: 実装本体、edit UI 本格機能、Outbox、SendGrid
- **参照すべき design 資産**:
  - `design/mockups/prototype/screens-b.jsx` の `Viewer` / `Manage`
  - `design/mockups/prototype/pc-screens-b.jsx` の `PCViewer` / `PCManage`
  - `design/mockups/prototype/shared.jsx` の `UrlRow`（teal / violet 切替）
  - `design/mockups/prototype/styles.css` / `pc-styles.css`（token）
- **参照すべき docs**:
  - `docs/spec/vrc_photobook_business_knowledge_v4.md` §3〜§6（公開 / Manage / slug 仕様）
  - `docs/design/aggregates/photobook/ドメイン設計.md`
  - `docs/adr/0005-image-upload-flow.md` §display variant 配信
- **実リソース操作の有無**: なし（計画書のみ）
- **Secret が絡むか**: なし
- **Safari 確認が必要か**: 計画書段階では不要（実装は PR25）
- **完了条件**: 計画書 review 通過、PR25 のスコープが PR 単位に分割可能なところまで具体化
- **次 PR への引き継ぎ**: 確定した display variant 配信方式 / slug 生成ルール / 認可方針

### PR25: 公開 Viewer / 管理ページ 実装

- **目的**: PR24 計画書に従い Viewer / Manage を最小骨格で実装し、Safari で確認する
- **実装するもの**:
  - `app/(public)/p/[slug]/page.tsx`（SSR、display + thumbnail 表示）
  - `app/(manage)/manage/[photobookId]/page.tsx` 本実装（`UrlRow` で manage URL 表示、再発行ボタン placeholder）
  - 必要に応じて Backend に slug→photobook lookup endpoint / display variant URL 取得 endpoint を追加
  - 基本的な OGP メタタグ（og:title / og:image はプレースホルダ可、本実装は PR33）
  - design-system 整備の第一弾（`design/design-system/colors.md` / `typography.md` / `spacing.md` / `radius-shadow.md` / `tailwind.config.ts` 反映）
- **実装しないもの**: edit 本格化（PR26-27）、publish flow 完成（PR28）、Outbox、SendGrid、本格 OGP（PR33）
- **参照すべき design 資産**: 同 PR24
- **参照すべき docs**: 同 PR24 + PR24 計画書
- **実リソース操作の有無**: Workers redeploy（vrcpb-frontend）+ 必要なら Cloud Run revision 更新
- **Secret が絡むか**: 既存 secret の追加注入なし
- **Safari 確認が必要か**: **必須**（公開ページ / 管理ページの初回 SSR、Cookie 維持、redirect、OGP メタ）
- **完了条件**: Viewer URL で実画像表示 OK / Manage で manage URL 表示 OK / Safari OK / Cookie 漏れなし
- **次 PR への引き継ぎ**: viewer から見える未実装機能リスト（caption 編集導線等）

### PR26: 編集 UI 本格化 計画書

- **目的**: 編集ページ（既存 `/edit/[photobookId]`）に photo grid / caption / 並び替え / cover / publish settings を追加する計画書を作る
- **実装するもの**:
  - 計画書 `docs/plan/m2-frontend-edit-ui-fullspec-plan.md`
  - photo grid のレイアウト方針 / 仮想スクロール要否
  - caption 編集 UI / 文字数 / バリデーション
  - drag & drop reorder の選択肢（HTML5 DnD / dnd-kit / 手動up/down）
  - Optimistic UI と OCC（楽観ロック）の整合
  - cover 設定 UI
  - publish settings（公開設定 / type 選択 / 公開ボタン）
  - design 抽出と Tailwind token への反映方針
- **実装しないもの**: 実装本体、Viewer / Manage、Outbox
- **参照すべき design 資産**:
  - `design/mockups/prototype/screens-a.jsx` の `Edit`
  - `design/mockups/prototype/pc-screens-a.jsx` の `PCEdit`（3 列レイアウト）
  - `design/mockups/prototype/shared.jsx` の `Photo` / `Av` / `Steps`
  - `design/mockups/prototype/styles.css` / `pc-styles.css`
- **参照すべき docs**:
  - `docs/design/aggregates/photobook/ドメイン設計.md` §pages / photos / OCC
  - `.agents/rules/domain-standard.md` §「集約子テーブルと親 version OCC ルール」
- **実リソース操作の有無**: なし
- **Secret が絡むか**: なし
- **Safari 確認が必要か**: 計画書段階では不要
- **完了条件**: 計画書 review 通過、PR27 が単独 PR 1〜2 本に分解可能
- **次 PR への引き継ぎ**: photo grid / caption / reorder / cover / publish settings の API / domain 拡張

### PR27: 編集 UI 本格化 実装

- **目的**: PR26 計画書に従い、edit UI を本格化する
- **実装するもの**:
  - photo grid（display variant 表示）
  - caption 編集（保存 + バリデーション）
  - photo / page reorder（OCC 経由）
  - cover 設定
  - publish settings UI（公開ボタンは PR28 で完成）
  - 必要なら Backend に Photobook UseCase 拡張（`reorderPhoto` / `setCover` / `updateCaption` 等、既存 OCC ルール準拠）
- **実装しないもの**: publish flow 完成（PR28）、Outbox、SendGrid
- **参照すべき design 資産**: 同 PR26
- **参照すべき docs**: 同 PR26 + PR26 計画書 + `.agents/rules/domain-standard.md`
- **実リソース操作の有無**: Workers redeploy + Cloud Run revision 更新
- **Secret が絡むか**: なし
- **Safari 確認が必要か**: **必須**（reorder / drag drop / form 保存 / Cookie 維持）
- **完了条件**: 編集 → 表示 → Viewer 反映が e2e で成立
- **次 PR への引き継ぎ**: publish flow への入力（photobook 完成度の判定基準）

### PR28: Publish flow 完成

- **目的**: 編集中の photobook を「公開」へ遷移させ、Viewer / Manage / 完了画面を統合する
- **実装するもの**:
  - publish / unpublish / hidden の境界（業務知識 v4 §6.x）
  - public slug 生成（PR24 計画書で決定したルール）
  - 完了画面（`screens-a.jsx` の `Complete`、`pc-screens-a.jsx` の `PCComplete`）
  - 公開 URL 表示 + コピー
  - manage URL 控え誘導
  - Backend Photobook UseCase の publish UseCase 完成
- **実装しないもの**: Outbox（PR30）、SendGrid（PR32）、本格 OGP（PR33）
- **参照すべき design 資産**:
  - `design/mockups/prototype/screens-a.jsx` の `Complete`
  - `design/mockups/prototype/pc-screens-a.jsx` の `PCComplete`
  - `design/mockups/prototype/shared.jsx` の `UrlRow`
- **参照すべき docs**: PR24 計画書 + 業務知識 v4
- **実リソース操作の有無**: Workers redeploy + Cloud Run revision 更新
- **Secret が絡むか**: なし
- **Safari 確認が必要か**: **必須**（公開操作 / URL コピー / Cookie 維持）
- **完了条件**: edit → publish → viewer → manage が e2e 成立 / Safari OK
- **次 PR への引き継ぎ**: 公開イベントを Outbox に流す要件（PR30 へ）

### PR29: Backend deploy 自動化（Cloud Build）

- **目的**: `git push` → `docker build` → Artifact Registry → `gcloud run deploy` をワンコマンド化
- **実装するもの**:
  - `cloudbuild.yaml`（既存 Dockerfile を流用）
  - Cloud Build trigger（main ブランチ push）
  - rollback 手順（`gcloud run services update-traffic`）
  - 既存 GitHub Actions（`.github/workflows/backend-ci.yml`）との関係整理
  - 失敗時の通知 / ログ
- **実装しないもの**: Cloud Run Jobs / Scheduler（PR31 に統合）
- **参照すべき design 資産**: なし
- **参照すべき docs**:
  - 旧 `m2-backend-cloud-run-deploy-plan.md`
  - `.agents/rules/security-guard.md`（Secret 注入は cloudbuild.yaml に書かない、Secret Manager 経由のみ）
- **実リソース操作の有無**: **`cloudbuild.googleapis.com` を有効化（課金開始決定ポイント）** / Cloud Build trigger 作成 / IAM service account 権限付与
- **Secret が絡むか**: cloudbuild.yaml が secret を直接読まない設計を維持。既存 Cloud Run env の secretKeyRef はそのまま
- **Safari 確認が必要か**: なし（CI/CD のみ）
- **完了条件**: main ブランチ push → 自動 deploy → revision 切替 / rollback 確認
- **次 PR への引き継ぎ**: PR31 で Cloud Run Jobs を作る際の build / deploy パターン共有
- **補追（2026-04-28、PR30 完了後の独立タスク）**: ロールバックドリル後の traffic
  pin 状態で `cloudbuild.yaml` の `gcloud run services update --image=` だけでは
  新 revision に traffic が流れない事象が PR30 deploy で顕在化。`cloudbuild.yaml`
  に `traffic-to-latest` step を追加して恒久対処。詳細は
  `harness/failure-log/2026-04-28_cloudbuild-traffic-pin-not-switched.md` /
  `docs/runbook/backend-deploy.md` §1.4 / §5.7。

### PR30: Outbox table + 同一 TX INSERT

- **目的**: 集約状態変更と同一 TX で Outbox に event を INSERT する基盤を作る
- **実装するもの**:
  - migration `outbox_events` table
  - event 種別: `PhotobookPublished` / `ManageUrlReissued` / `ImageBecameAvailable` / `ImageFailed` / `PhotobookHidden` 等
  - Photobook UseCase / Image UseCase に Outbox INSERT を同 TX 内で追加
  - I-O1 不変条件（同 TX 保証）の test
- **実装しないもの**: outbox-worker（PR31）、SendGrid 連携（PR32）
- **参照すべき design 資産**: なし
- **参照すべき docs**:
  - `docs/design/cross-cutting/`（Outbox 設計）
  - `docs/spec/vrc_photobook_business_knowledge_v4.md`
- **実リソース操作の有無**: migration 適用（cloud-sql-proxy + goose）
- **Secret が絡むか**: なし
- **Safari 確認が必要か**: なし
- **完了条件**: 主要 UseCase が Outbox INSERT を同 TX で行う / DB rollback で event も rollback
- **次 PR への引き継ぎ**: pending event を消費する worker（PR31）

### PR31: outbox-worker + Cloud Run Jobs / Scheduler 化

- **目的**: outbox-worker と image-processor を Cloud Run Jobs として運用する。
  メール送信は ADR-0006 で MVP 必須要件から外したため、worker handler は当面 **no-op + log のみ**
- **実装するもの**:
  - `cmd/outbox-worker`（pending event poll → 種別ごと handler 実行 → mark done / failed）
  - Cloud Run Jobs spec（image-processor / outbox-worker）
  - Cloud Scheduler（定期 trigger、業務知識 v4 §運用 SLO に沿う頻度）
  - retry / dead letter（同 row の retry 上限と stuck 検出）
  - 各 event handler は **no-op + structured log**（ADR-0006、メール送信なし MVP）
- **実装しないもの**: メール送信本実装（**ADR-0006 後続**、Provider 確定後）/ OGP 自動生成本実装（PR33）
- **参照すべき design 資産**: なし
- **参照すべき docs**:
  - **ADR-0006**（メール送信を MVP 必須から外す根拠）
  - `docs/plan/m2-outbox-plan.md` §12 引き継ぎ
  - `docs/plan/m2-image-processor-plan.md` §10〜§17
- **実リソース操作の有無**: **Cloud Run Jobs + Cloud Scheduler 作成（課金影響あり）** / IAM service account 権限
- **Secret が絡むか**: 既存 R2 / DATABASE_URL を Jobs にも secretKeyRef で注入。メール
  Provider 関連 Secret は**当面追加しない**（ADR-0006）
- **Safari 確認が必要か**: なし
- **完了条件**: image-processor / outbox-worker が Jobs 経由で動く / dead letter 経路あり /
  各 handler が no-op log を出すこと
- **次 PR への引き継ぎ**: メール Provider 再選定（PR32 / ADR-0006）/ OGP 自動生成（PR33）

### PR32: Email Provider 再選定 + Manage URL Delivery 再設計

> **2026-04-28 ADR-0006 で本 PR の範囲を変更**。
> SendGrid Japan は個人 / 個人事業主 / 任意団体は契約不可、AWS SES の production
> access も不通過のため、ADR-0004 は Superseded。MVP のメール送信機能は必須要件から
> 外し、Complete 画面で 1 度だけ表示する方式（PR28 で実装済）が MVP 標準。
>
> 本 PR はメール送信実装ではなく、**Provider 再選定タスク + ManageUrlDelivery 集約の
> 再設計**を担う。実 API 接続 / 実装 / Outbox handler 実装は後続 PR で扱う。

- **目的**: 個人 / 個人事業主でも契約可能なメール Provider を再選定し、確定後の
  ManageUrlDelivery 設計を更新する。MVP リリース直前までに採用 Provider を確定できれば
  「manage URL 再発行 → メール送信」を後続 PR で乗せる
- **実装するもの**:
  - 候補 Provider の **個人契約可否の実確認**（ADR-0006 §4.4 候補表）
    - Resend / Mailgun / Brevo / Postmark / ZeptoMail / Zoho / Cloudflare 系 / 独自 SMTP
  - 採用候補 1〜2 個に絞った PoC（API key 発行 + 1 通テスト送信）
  - 新 ADR で採用確定（or「メール送信は ローンチ後対応」を確定）
  - 採用確定時のみ `ManageUrlDelivery` 集約 / `internal/notification/` の設計を確定
    （実装は採用後の別 PR）
- **実装しないもの**: メール送信本実装 / Outbox handler の実装 / Cloud Run Jobs 統合
  （いずれも採用 Provider 確定後の **PR32 後続 PR**で扱う）
- **参照すべき design 資産**: なし
- **参照すべき docs**:
  - **ADR-0006**（本 PR の根拠）
  - ADR-0004（Superseded、過去経緯参照のみ）
  - 業務知識 v4 §6 manage URL / §通知
- **実リソース操作の有無**: 候補 Provider のアカウント開設は **ユーザー判断**
  （採用前 PoC 段階では trial / sandbox を使用）
- **Secret が絡むか**: 採用確定時のみ `<PROVIDER>_API_KEY` を Secret Manager に登録
- **Safari 確認が必要か**: なし
- **完了条件**:
  - 採用 Provider の確定 ADR が Accepted、または
  - 「ローンチ後対応」を ADR で明記し、MVP は Complete 画面 1 度表示で受け入れる方針を確定
- **次 PR への引き継ぎ**:
  - 採用 Provider 確定: 後続 PR で ManageUrlDelivery 集約実装 + Outbox handler 実装
  - ローンチ後対応: PR41+ にメール対応をルーチン化

### PR33: OGP 独立管理

- **目的**: 公開ページの OGP 画像を後追いで自動生成し、独立 table で管理
- **実装するもの**:
  - migration `photobook_ogp_images`
  - OGP 生成 UseCase（cover / type / title から合成）
  - Cloud Run Jobs での非同期生成
  - public viewer の `<meta og:image>` を生成済み URL に切替
- **実装しないもの**: PR25 で入れた placeholder OGP の置き換えのみ
- **参照すべき design 資産**:
  - `design/mockups/concept-images/`（type 別キービジュアル）
- **参照すべき docs**: `docs/design/cross-cutting/` の OGP 設計
- **実リソース操作の有無**: Cloud Run Jobs 追加 / R2 prefix `photobooks/{pid}/ogp/`
- **Secret が絡むか**: なし（既存 R2 credentials を流用）
- **Safari 確認が必要か**: **必須**（OGP の Twitter / X / Slack プレビュー含む）
- **完了条件**: viewer の og:image が生成 URL を返す / Twitter card validator OK
- **次 PR への引き継ぎ**: 運用整備（Moderation / Report）

### PR34: Moderation / Ops

- **目的**: 運営の手動操作（hide / unhide / softDelete / restore / purge / reissueManageUrl）の経路と履歴を整える
- **実装するもの**:
  - `internal/moderation/`（集約）
  - `cmd/ops`（CLI、ADR-0002 準拠）
  - `ModerationAction` 履歴
  - 必要なら Backend HTTP endpoint（運営限定、IP allowlist 等）
- **実装しないもの**: Report 集約（PR35）
- **参照すべき design 資産**: なし（運営者のみ）
- **参照すべき docs**: `docs/design/aggregates/moderation/` / 業務知識 v4 §運営
- **実リソース操作の有無**: 運用ロール作成 / IAM
- **Secret が絡むか**: 運営用認可は別系（Cookie ではなく ops CLI の short-lived credential）
- **Safari 確認が必要か**: なし
- **完了条件**: hide / unhide / softDelete / restore / purge / reissueManageUrl が CLI で動く
- **次 PR への引き継ぎ**: Report 受付の運営対応経路

### PR35: Report 集約

- **目的**: 通報受付（公開 viewer 上）+ 運営対応フロー
- **実装するもの**:
  - 通報受付 endpoint（公開）
  - Report 集約（`internal/report/`）
  - 運営側 cmd/ops 連携
  - Frontend 通報 UI（viewer から）
- **実装しないもの**: UsageLimit（PR36）
- **参照すべき design 資産**:
  - `design/mockups/prototype/screens-b.jsx` の `Report`
  - `design/mockups/prototype/pc-screens-b.jsx` の `PCReport`
- **参照すべき docs**: 業務知識 v4 §Report / §通報
- **実リソース操作の有無**: なし（既存 Cloud Run 上で動く）
- **Secret が絡むか**: なし
- **Safari 確認が必要か**: **必須**（通報フォーム送信 / 確認画面）
- **完了条件**: 通報 → 運営 cmd/ops で対応 → 状態変更 → 通報者向け表示
- **次 PR への引き継ぎ**: abuse 抑止（UsageLimit）

### PR36: UsageLimit 集約

- **目的**: 公開数 / upload 数の上限と abuse 抑止
- **実装するもの**:
  - UsageLimit 集約（`internal/usagelimit/`）
  - upload-intent / publish 経路に上限チェック追加
  - abuse 抑止 cleanup
- **実装しないもの**: LP / 利用規約（PR37）
- **参照すべき design 資産**: なし
- **参照すべき docs**: 業務知識 v4 §UsageLimit / §abuse
- **実リソース操作の有無**: migration 追加
- **Secret が絡むか**: なし
- **Safari 確認が必要か**: 上限到達時の UI 確認
- **完了条件**: 制限到達 → 適切な表示 / 制限解除（時間経過 or 運営）
- **次 PR への引き継ぎ**: ローンチ前の整備

### PR37: LP / terms / privacy / about

- **目的**: 公開トップ + 利用規約 + プライバシー + サービス紹介を実装
- **実装するもの**:
  - `app/(public)/page.tsx`（LP）
  - `app/(public)/terms/page.tsx`
  - `app/(public)/privacy/page.tsx`
  - `app/(public)/about/page.tsx`
  - 非公式表記（`m2-domain-purchase-checklist.md` §2.2 文言案）
  - SEO / OGP / sitemap.xml / robots.txt
- **実装しないもの**: 公開リポ化（PR38）
- **参照すべき design 資産**:
  - `design/mockups/prototype/screens-a.jsx` の `LP`
  - `design/mockups/prototype/pc-screens-a.jsx` の `PCLP`
  - `design/mockups/prototype/pc-shared.jsx` の `PCTrust` / `PCHeader`
  - `design/mockups/concept-images/`
- **参照すべき docs**: 業務知識 v4 §非公式表記 / 利用規約ドラフト
- **実リソース操作の有無**: Workers redeploy
- **Secret が絡むか**: なし
- **Safari 確認が必要か**: **必須**（LP 全画面 / モバイル / iPad / OGP）
- **完了条件**: 全 4 ページが Safari で破綻なく表示
- **次 PR への引き継ぎ**: 公開リポ化判断

### PR38: Public repo 化 / Security final audit

- **目的**: 公開可否を最終判断し、必要な掃除を行う
- **実装するもの**:
  - git 履歴 secret scan（trufflehog 等）
  - work-logs / failure-log の公開可否レビュー（個人情報 / 顧客 / Secret の grep）
  - README 公開向け仕上げ
  - GitHub branch protection / required reviewers 整備
  - 公開可否最終判断（業務知識 v4 §公開方針）
- **実装しないもの**: 本番運用整備（PR39）
- **参照すべき design 資産**: なし
- **参照すべき docs**: `.agents/rules/security-guard.md`、`docs/security/public-repo-checklist.md`（存在しない場合は本 PR で作成）
- **実リソース操作の有無**: GitHub repo visibility 切替（**ユーザー判断**）
- **Secret が絡むか**: 履歴 scan 結果は機密扱い
- **Safari 確認が必要か**: なし
- **完了条件**: scan clear / 公開判断確定（公開する / しない）
- **次 PR への引き継ぎ**: 本番運用整備

### PR39: 本番運用整備 / Cloud SQL 本番化

- **目的**: 検証 DB を本番化、監視 / アラート / バックアップ整備
- **実装するもの**:
  - `vrcpb-api-verify` を本番相当の名前に rename or 新規 instance に migrate
  - HA / バックアップ / 自動メンテナンス設定
  - Budget Alert 再設計
  - Cloud Monitoring / Error Reporting / Uptime Check
  - 障害対応 runbook
- **実装しないもの**: ローンチ前最終チェック（PR40）
- **参照すべき design 資産**: なし
- **参照すべき docs**: 旧 `m2-cloud-sql-short-verification-plan.md` §11 / 業務知識 v4 §運用
- **実リソース操作の有無**: **Cloud SQL instance 操作 / DNS 切替 / Secret 更新**
- **Secret が絡むか**: 新 DATABASE_URL を Secret Manager に追加（旧 secret 切替）
- **Safari 確認が必要か**: 切替後の Safari smoke 確認
- **完了条件**: 新 DB に traffic 移行 / 旧 DB 廃止 / Budget Alert 動作
- **次 PR への引き継ぎ**: ローンチ前最終チェック

### PR40: ローンチ前チェック + spike 削除 + Cloud Build trigger 化

- **目的**: ローンチ前の総点検と環境クリーンアップ + PR29 で先送りした deploy 自動化の整備
- **実装するもの**:
  - spike Cloud Run / Workers / Artifact Registry / R2 bucket 削除判断
  - 旧 secret 整理
  - 全 URL 確認 / 全エンドポイント smoke
  - macOS Safari / iPhone Safari / Chrome / iPad / Edge / Firefox 確認
  - OGP（X / Slack / Discord）プレビュー確認
  - ローンチ告知準備（X 投稿テキスト等）
  - **PR29 先送り項目の整備**:
    - Cloud Build trigger オブジェクト作成（GCP Console からワンクリック起動）
    - GitHub App / Cloud Build GitHub connection（PR38 Public repo 化と統合可）
    - tag trigger（`release-*` push で発動）の評価
    - Artifact Registry retention policy（過去 image 自動 cleanup）
- **実装しないもの**: main push 自動 deploy（PR41+）/ Frontend Workers deploy 自動化（PR41+）/ ローンチ後改善
- **参照すべき design 資産**: なし
- **参照すべき docs**: 業務知識 v4 §ローンチ / `docs/plan/m2-backend-deploy-automation-plan.md` §6 / `docs/runbook/backend-deploy.md` §6
- **実リソース操作の有無**: spike 環境削除 / Cloud Build trigger 作成 / GitHub App 接続（**ユーザー判断**）
- **Secret が絡むか**: 旧 secret の削除（誤って現役 secret を消さない）
- **Safari 確認が必要か**: **全画面**
- **完了条件**: チェックリスト 100% / spike 削除 / コスト降下確認 / Cloud Build trigger 動作確認
- **次 PR への引き継ぎ**: ローンチ実行

### PR41+: ローンチ後改善

- HEIC 本対応（libheif + cgo + Dockerfile 改修、PR23 計画書 §4 H2 / H3）
- WebP / AVIF 配信
- drag & drop アップロード / progress bar
- design system 正式化
- パフォーマンス計測 / 改善（Lighthouse / Cloud Trace）
- iPad / Firefox / Edge 動作再確認
- 24h / 7 日 / 30 日 ITP 観察結果の運用反映
- R2 orphan Reconcile（display/thumbnail PUT 成功 → DB 失敗 / failed image の original / DELETE 失敗の orphan を 7 日後 cleanup）
- multi-worker 化と claim 用 column 追加（PR23 で見つかった dry-run の挙動を re-design）
- **PR29 先送り項目（PR40 で扱わない分）**:
  - main push 自動 deploy（e2e test 充実後に評価）
  - Frontend Workers deploy 自動化（OpenNext build + wrangler deploy の自動化）
  - Cloud Build machineType 昇格（速度改善必要時）

---

## 4. 各 PR のテンプレ

各 PR の説明には以下 9 項目を必ず書く。

| # | 項目 | 例 |
|---|---|---|
| 1 | 目的 | 一文で何のために作るか |
| 2 | 実装するもの | 具体ファイル / 機能 |
| 3 | 実装しないもの | 隣接 PR と境界を切る |
| 4 | 参照すべき design 資産 | mockups / concept-images |
| 5 | 参照すべき docs | plan / ADR / 業務知識 / rules |
| 6 | 実リソース操作の有無 | DNS / Secret / Dashboard / 課金 |
| 7 | Secret が絡むか | 注入 / 値の持ち回り |
| 8 | Safari 確認が必要か | safari-verification.md 適用範囲 |
| 9 | 完了条件 + 次 PR への引き継ぎ | 受け渡しを明確に |

---

## 5. フロント実装の整理

### 5.1 実装済み

| 項目 | 場所 |
|---|---|
| Next.js 15 skeleton + middleware | `frontend/middleware.ts` |
| Workers deploy（OpenNext） | `frontend/wrangler.jsonc` |
| Custom Domain `app.vrc-photobook.com` | Cloudflare Workers Custom Domain |
| `/draft/[token]` Route Handler | `frontend/app/(draft)/draft/[token]/route.ts` |
| `/manage/token/[token]` Route Handler | `frontend/app/(manage)/manage/token/[token]/route.ts` |
| `/edit/[photobookId]` upload UI（最小骨格） | `frontend/app/(draft)/edit/[photobookId]/UploadClient.tsx` |
| TurnstileWidget | `frontend/components/TurnstileWidget.tsx` |
| upload API client（issueUploadVerification / issueUploadIntent / putToR2 / completeUpload） | `frontend/lib/upload.ts` |
| HEIC 拒否（content_type / 拡張子の多層ガード） | `frontend/lib/upload.ts` + `UploadClient.tsx` |
| processing 状態の UI 表示 | `UploadClient.tsx` |

### 5.2 これから作る（PR と対応）

| 項目 | 担当 PR |
|---|---|
| 公開 Viewer (`/p/[slug]` 等) | PR24 計画 / PR25 実装 |
| 管理ページ (`/manage/[photobookId]`) 本格化 | PR24 計画 / PR25 実装 |
| design-system 整備（colors / typography / spacing / radius-shadow / tailwind 反映） | PR25（第一弾）+ PR41+（正式化） |
| photo grid（display 表示） | PR26 計画 / PR27 実装 |
| caption 編集 | PR27 |
| reorder（page / photo / display_order） | PR27 |
| cover 設定 | PR27 |
| publish settings | PR27 |
| publish flow 完成（complete 画面 / URL コピー / manage URL 控え） | PR28 |
| 通報 UI | PR35 |
| 上限到達時の UI | PR36 |
| LP / terms / privacy / about | PR37 |
| OGP（本格） | PR33 |
| drag & drop / progress bar | PR41+ |

---

## 6. バックエンド実装の整理

### 6.1 実装済み（パッケージ）

- `backend/internal/auth/session/` — session 認可機構（Cookie + middleware）
- `backend/internal/photobook/` — Photobook 集約（pages / photos / page_metas / OCC）
- `backend/internal/image/` — Image 集約（status machine / variants / FOR UPDATE SKIP LOCKED）
- `backend/internal/imageupload/` — upload-intent / complete-upload + R2 client
- `backend/internal/uploadverification/` — Turnstile session 集約
- `backend/internal/imageprocessor/` — image-processor（imaging / process_image / process_pending / wireup）
- `backend/cmd/api/` — HTTP server
- `backend/cmd/image-processor/` — CLI（PR23 で追加、image 同梱済）

### 6.2 これから作る（PR と対応）

| 項目 | 担当 PR |
|---|---|
| Photobook UseCase 拡張（reorder / setCover / updateCaption / publish 完成） | PR27 / PR28 |
| slug 発行 / public lookup endpoint | PR25 |
| Outbox table + 同 TX INSERT | PR30 |
| `cmd/outbox-worker` | PR31 |
| Cloud Run Jobs / Scheduler（image-processor / outbox-worker） | PR31 |
| SendGrid 連携 + ManageUrlDelivery | PR32 |
| OGP 独立 table + 生成 Job | PR33 |
| Moderation 集約 + `cmd/ops` | PR34 |
| Report 集約 | PR35 |
| UsageLimit 集約 | PR36 |
| HEIC 本対応（libheif + cgo） | PR41+ |
| R2 orphan Reconcile | PR41+ |

---

## 7. 実リソース操作が必要な PR（要 ユーザー判断）

| PR | 操作 | 課金 / リスク |
|---|---|---|
| PR25 | Workers redeploy / Cloud Run revision 更新 | 既存課金内 |
| PR27 | Workers redeploy / Cloud Run revision 更新 | 既存課金内 |
| PR28 | 同上 | 同上 |
| PR29 | **Cloud Build API 有効化（課金開始）** / Cloud Build trigger / IAM | Cloud Build 課金開始（小額） |
| PR30 | migration 適用（cloud-sql-proxy + goose） | DB write |
| PR31 | **Cloud Run Jobs 作成 / Cloud Scheduler 作成** | Jobs / Scheduler 課金開始（小額） |
| PR32 | **SendGrid アカウント / API Key / DKIM / SPF DNS 追加** | SendGrid 無料 100通/日 |
| PR33 | Cloud Run Jobs 追加 / R2 prefix 増 | R2 PUT/GET 増（小額） |
| PR34 | 運用 IAM ロール作成 | なし |
| PR38 | **GitHub repo visibility 切替（公開化判断）** | リスク（履歴公開） |
| PR39 | **Cloud SQL instance 操作 / DNS 切替 / Secret 更新** | Cloud SQL 本番課金 |
| PR40 | **spike 環境削除** | 課金降下 |

---

## 8. Safari 確認が必要な PR

`.agents/rules/safari-verification.md` の発火条件（Cookie / redirect / OGP / レスポンスヘッダ /
モバイル UI / token→session 交換）に該当する PR:

- PR25（Viewer / Manage の SSR / Cookie / OGP）
- PR27（編集 UI / form / drag drop / Cookie）
- PR28（publish flow / completion 画面）
- PR33（OGP / Twitter card）
- PR35（通報 UI）
- PR36（上限 UI）
- PR37（LP / terms / privacy / about、全画面）
- PR40（最終確認、全画面）

---

## 9. 課金 / 運用判断ポイント

| 節目 | 判断内容 | 判断時期 |
|---|---|---|
| **Cloud SQL `vrcpb-api-verify`** | 検証 DB を本番相当に使い続けるか / rename / migration | PR39 |
| **R2 test object cleanup** | PR21〜PR23 で生成したテスト object の整理 | PR40 直前（spike 削除と同時） |
| **Cloud Build API 有効化（課金）** | 自動 deploy 導入の費用対効果 | PR29 着手前 |
| **Cloud Run Jobs / Scheduler 課金** | image-processor / outbox-worker の頻度設定 | PR31 着手前 |
| **SendGrid 課金** | 無料枠（100通/日）で M2 充分。突破時の課金プラン判断 | PR32 着手前 |
| **spike 環境削除** | 削除タイミング（PR40 が標準、必要なら PR39 で前倒し） | PR40 |
| **Budget Alert 見直し** | M2 中盤で月予算再設定 | PR39 |
| **Public repo 化判断** | git 履歴 scan / work-log 公開可否 / Branch protection | PR38 |

---

## 10. 旧ロードマップとの関係

- **旧正典**: `harness/work-logs/2026-04-27_post-deploy-final-roadmap.md`（PR12〜PR23 の進行を支えた）
- **新正典**: 本書（`docs/plan/vrc-photobook-final-roadmap.md`、PR24 以降を支える）
- 旧ロードマップの **PR23 以降は実進行とズレている**ため、今後は **参照専用**（過去の意図確認用途）
- 旧ロードマップの §A〜§D は事実として有効（過去の進行記録）。§E（ローンチ後）以降の方針は本書 PR41+ に統合
- `CLAUDE.md` の現在地マーカーは本書を指すよう PR23 締めで更新する

---

## 11. 関連ドキュメント

- 業務知識: [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md)
- ADR: [`docs/adr/`](../adr/)
- 集約設計: [`docs/design/aggregates/`](../design/aggregates/)
- 横断設計: [`docs/design/cross-cutting/`](../design/cross-cutting/)
- 認可: [`docs/design/auth/`](../design/auth/)
- M2 計画書群: [`docs/plan/m2-*.md`](.)
- design 資産: [`design/mockups/prototype/`](../../design/mockups/prototype/) / [`design/mockups/concept-images/`](../../design/mockups/concept-images/) / [`design/README.md`](../../design/README.md)
- 直近の作業ログ:
  - [`harness/work-logs/2026-04-27_image-processor-result.md`](../../harness/work-logs/2026-04-27_image-processor-result.md)（PR23）
  - [`harness/work-logs/2026-04-27_frontend-upload-ui-result.md`](../../harness/work-logs/2026-04-27_frontend-upload-ui-result.md)（PR22）
  - [`harness/work-logs/2026-04-27_safari-real-token-e2e-result.md`](../../harness/work-logs/2026-04-27_safari-real-token-e2e-result.md)（PR17）
  - [`harness/work-logs/2026-04-27_post-deploy-final-roadmap.md`](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)（旧ロードマップ）

---

## 12. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-27 | 初版作成。PR23（image-processor）完了時点での新正典。PR24〜PR41+ を再定義 |
