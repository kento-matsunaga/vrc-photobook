# PR37 design rebuild 実装結果（2026-05-01、構造 rebuild 完了 / 最終 visual polish は後続）

> **状態**: STOP α / β / δ / ε / final closeout 全完了。**prototype-aligned structure / 共通コンポーネント / public pages の機能・安全面 / failure-log §5 ルール適用** までを本 PR で完了扱いとし、**最終 visual polish は機能完成後の最終デザイン PR へ延期**する。
> 起点 commit: `7f459f5`（PR37 機能編 final closeout）→ 設計判断 commit `bf6fdd3`（design rebuild 実装）→ 本 closeout commit
> failure-log §5 ルール適用初事例: [`harness/failure-log/2026-05-01_pr37-public-pages-design-mismatch.md`](../failure-log/2026-05-01_pr37-public-pages-design-mismatch.md)

## 0. 本書のスコープ

PR37 design rebuild の進行記録。STOP α 計画（plan メモ固定）→ STOP β 実装 → STOP δ Workers redeploy → STOP ε Safari 実機確認 → final closeout までを集約する。

機能・安全面（HTTP 200 / noindex / Referrer-Policy / metadata 上書き / 既存経路 regression / Secret + raw 値漏えい 0 件）は要件達成。**構造 rebuild と共通コンポーネント整備は plan メモ通り完了**。視覚的な最終仕上げは、機能全体の完成度に合わせて **後続の最終デザイン PR** に延期する（user 明示）。

## 1. STOP α 確定事項（2026-05-01 ユーザー承認）

failure-log §5 ルールに従い、実装前に画面別ワイヤーフレーム + 採用 prototype 画面 ID + 既存ページ温度感整合を提示してユーザー承認を取得した。

- 採用 prototype 画面 ID:
  - LP mobile: `design/mockups/prototype/screens-a.jsx` `LP({ go })`
  - LP PC: `design/mockups/prototype/pc-screens-a.jsx` `PCLP({ go })`（`pc-header` nav は不採用）
  - About: LP feature-cell パターン + `screens-b.jsx` Viewer "Memories card" の dashed 区切りを 1 箇所のみ
  - Terms / Privacy: 既存 `/help/manage-url` の温度感引用 + design-system token 整理
  - 共通 footer: `screens-a.jsx` `trust-strip` を closing element として採用
- ページ別ワイヤーフレーム: `/` `/about` `/terms` `/privacy` footer の 5 種を承認
- 使う / 使わない装飾: gradient placeholder photos は MockBook / MockThumb 内の局所のみ、ページ背景は禁止、48px 超の巨大見出し禁止（mobile h1 26px / PC h1 40px は prototype forward port として承認）
- Safari 確認観点: plan メモ §7 で 13 観点を事前明示
- 新規コンポーネント: `MockBook` / `TrustStrip` / `PolicyArticle` / `SectionEyebrow` + `PublicPageFooter` 拡張

plan メモは `harness/work-logs/2026-05-01_pr37-design-rebuild-plan.md` に固定し、STOP β 実装は本メモを唯一の正典として参照した。

## 2. STOP β 実装（commit `bf6fdd3`）

### 2.1 変更ファイル

| ファイル | 種別 |
|---|---|
| `frontend/app/page.tsx` | M（rewrite）|
| `frontend/app/(public)/about/page.tsx` | M |
| `frontend/app/(public)/terms/page.tsx` | M（rewrite 82%）|
| `frontend/app/(public)/privacy/page.tsx` | M（rewrite 84%）|
| `frontend/app/(public)/help/manage-url/page.tsx` | M（footer 統一）|
| `frontend/components/Public/PublicPageFooter.tsx` | M（`showTrustStrip` / `extraSlot` prop 追加）|
| `frontend/components/Public/SectionEyebrow.tsx` | A |
| `frontend/components/Public/TrustStrip.tsx` | A |
| `frontend/components/Public/MockBook.tsx` | A（MockBook + MockThumb 5 variants）|
| `frontend/components/Public/PolicyArticle.tsx` | A（PolicyArticle + PolicyToc + PolicyNotice）|
| `frontend/components/Viewer/ViewerLayout.tsx` | M（footer 統一 + `extraSlot` で通報リンク維持）|
| `frontend/app/__tests__/public-pages.test.tsx` | M（新構造に合わせて 5 ケース）|
| `frontend/components/Public/__tests__/{MockBook,TrustStrip,PolicyArticle,PublicPageFooter}.test.tsx` | A（新規 5 + 2 + 3 + 4 = 14 ケース）|
| `harness/work-logs/2026-05-01_pr37-design-rebuild-plan.md` | A（plan メモ正典）|

合計 17 files / +1462 / -662。

### 2.2 prototype-aligned structure（核心成果）

- LP は hero（mock-book + 26px/40px h1）+ thumb strip（mobile 4 / PC 5）+ features（mobile 2 / PC 4 col）+ policy + cta-block + trust-strip + footer の 6 セクション構成
- About は 位置づけ card（dashed メタ 1 箇所のみ）+ できる 6（teal-soft ico）+ できない 4（surface-soft + ink-soft ico）+ ポリシー + trust-strip
- Terms / Privacy は notice box + TOC + PolicyArticle で第 1〜9 / 第 1〜10 条を統一、Privacy 第 5 条で外部サービス 6 chip を `bg-brand-teal-soft` で表示
- Help / Viewer の footer を `PublicPageFooter` に統一、Viewer は `extraSlot` で通報リンクを個別保持

### 2.3 検証結果

| 項目 | 結果 |
|---|---|
| `typecheck` | OK |
| `vitest run` | 18 files / 151 tests 全 PASS（新規 19 ケース含む）|
| `next build` | OK（`/` / `/about` / `/terms` / `/privacy` は ○ Static、`/help/manage-url` も Static 維持）|
| `cf:build` | OK |
| `cf:check` | OK（60 assets / Total Upload 4762.63 KiB / bindings 維持）|
| commit | `bf6fdd3 feat(frontend): rebuild LP, terms, privacy, about with prototype-aligned design` |
| push | `7f459f5..bf6fdd3 main -> main` SUCCESS |

## 3. STOP δ Workers redeploy（完了）

| 項目 | 値 |
|---|---|
| `cf:build` 再実行 | OK |
| `wrangler deploy` | SUCCESS（8.78 sec、Total Upload 4762.63 KiB / gzip 981.51 KiB、Worker Startup 41 ms）|
| 新 Worker version | **`c2d35a6c-9d14-4626-886c-47362b78b8e2`**（100% active）|
| 直前 active（rollback candidate）| `6f1e82d7-cf57-41ab-99dd-0ede5266a3a5`（PR37 機能編 STOP δ 由来）|
| bindings 維持 | `OGP_BUCKET (vrcpb-images)` / `ASSETS` |
| binding 変更 | なし |
| Backend / Cloud Run / Cloud SQL / Secret / migration / Job / Scheduler 操作 | **なし** |

## 4. STOP ε Safari 実機確認（完了、機能・安全面）

### 4.1 5 ページ + Help: HTTP / noindex / metadata

| path | HTTP | X-Robots-Tag | Referrer-Policy | meta robots | metadata title 上書き |
|---|---|---|---|---|---|
| `/` | 200 | noindex, nofollow | strict-origin-when-cross-origin | yes | 「VRC PhotoBook｜VRChat 写真をログイン不要で 1 冊に」|
| `/about` | 200 | noindex, nofollow | strict-origin-when-cross-origin | yes | 「VRC PhotoBook について｜About」|
| `/terms` | 200 | noindex, nofollow | strict-origin-when-cross-origin | yes | 「利用規約｜VRC PhotoBook」|
| `/privacy` | 200 | noindex, nofollow | strict-origin-when-cross-origin | yes | 「プライバシーポリシー｜VRC PhotoBook」|
| `/help/manage-url` | 200 | noindex, nofollow | strict-origin-when-cross-origin | yes | 既存（PublicPageFooter 統一反映済）|

production response に主要 `data-testid`（mock-book / trust-strip / public-page-footer / policy-toc / policy-notice / policy-article-terms-{1〜9} / policy-article-privacy-{1〜10} / privacy-external-services）と公開対象固定文字列（`@Noa_Fortevita` / `ERENOA` / `Cloudflare Workers` / `Google Cloud Run`）が含まれていることを curl で確認済。

### 4.2 Viewer footer + 既存経路 regression

| path | HTTP | 備考 |
|---|---|---|
| `/p/<dummy>` | 404 | Next.js 標準（既存挙動）|
| `/p/<dummy>/report` | 404 | 同上 |
| `/manage/<dummy>`（Cookie なし）| 200 | `Referrer-Policy: no-referrer` 維持（token URL 漏洩対策）|
| `/og/default.png` | 200 | image/png 維持 |
| `/ogp/<dummy>?v=1` | 302 | fallback 維持 |
| Viewer footer（known safe route 経由）| — | PublicPageFooter の 5 nav リンク + 通報リンク `extraSlot` + 「VRC PhotoBook（非公式ファンメイドサービス）」が反映、通報リンクは個別保持 |

regression なし。

### 4.3 Safari 実機確認サマリ

| 観点 | 結果 |
|---|---|
| 5 ページ表示・内部リンク遷移 | OK（macOS / iPhone Safari）|
| `X-Robots-Tag` / `<meta robots>` / `Referrer-Policy` 維持 | OK |
| token / Cookie / 任意 ID / Secret / raw URL 非露出 | OK |
| **構造 rebuild（prototype-aligned）が plan メモ通りに反映** | OK |
| **最終 visual polish** | **後続 PR へ延期**（user 明示）|

### 4.4 構造 rebuild 完了 / 最終 visual polish 後続化（user 明示）

- 本 PR は **prototype-aligned structure / 共通コンポーネント / public pages の機能・安全面 / failure-log §5 ルール適用** までを完了扱いとする
- 機能全体の完成度に合わせて、最終 visual polish は **後続の最終デザイン PR** に延期する
- user から **VRChat 写真フォルダ**（個人ローカル resource、本書では `<user-local VRChat photo folder>` と redact 表記）を後続デザイン PR の実画像素材として使ってよい旨の許可を受領
- 実 Windows ローカルパスは commit / docs / failure-log には書かない方針を維持
- 後続デザイン PR は failure-log §5 ルール（plan メモ + 画面別ワイヤーフレーム + STOP α 承認）を継続適用する

## 5. final closeout（完了、本 commit）

PR closeout チェックリスト（`.agents/rules/pr-closeout.md` §6 準拠、本 commit 完了時点）:

- [x] STOP β 実装 commit `bf6fdd3` push 確認
- [x] STOP δ deploy 結果 + 新 Worker version `c2d35a6c-...` 記録
- [x] STOP ε 機能・safety 面結果 + 構造 rebuild 完了 / 最終 visual polish 後続化を記録
- [x] failure-log 「適用結果」追記（再発防止策が機能した事例として）
- [x] roadmap §1.1 Worker version 更新 + §1.3 PR37 design rebuild 構造完了マーカー + 最終 visual polish 後続候補追加
- [x] runbook 追記要否判断（追記なし、後述 §7）
- [x] stale-comments 4 区分分類
- [x] Secret / raw 値漏えい grep 0 件
- [x] generated files（`.open-next` / `.wrangler`）git 不該当
- [x] tmp / proxy / DSN cleanup（本 PR では DB アクセスなし、tmp 残存なし）

## 6. Secret / raw 値の取り扱い

- raw slug / raw photobook_id / raw report_id / raw URL / token / Cookie / DATABASE_URL / Secret 値 / source_ip_hash 完全値 / scope_hash 完全値 / reporter_contact 実値 / detail 実値: chat / commit / docs / work-log / failure-log には **未含有**（grep 0 件）
- 仮置き値（運営者表示名 `ERENOA` / 連絡用 X `@Noa_Fortevita` / 準拠法）は user 承認済の **公開対象値**で、機密ではない
- VRChat 写真フォルダの実 Windows ローカルパスは **本書および全 commit / docs に未記録**、redact 表記 `<user-local VRChat photo folder>` のみで参照
- Cloud Workers version ID（`c2d35a6c-...`、`6f1e82d7-...`）はインフラ識別子（rollback 参照用）として記録、PII 非該当

## 7. runbook 追記判断

**追記なし** とする:
- 本 PR は Frontend 公開ページ design rebuild のみ。Backend / Cloud SQL / Secret / migration / Cloud Run Job / cmd/ops いずれも不変
- `docs/runbook/backend-deploy.md` / `docs/runbook/usage-limit.md` / `docs/runbook/ops-moderation.md` への影響なし
- Frontend deploy 専用 runbook は現状未整備。本 PR は work-log + roadmap + failure-log（既存に追記）に集約
- 後続の最終デザイン PR で必要なら Frontend deploy 専用 runbook を整備する余地

## 8. 後続候補

最重要から順:

- **機能完成後の最終 visual polish / final design pass**（最重要、user 明示）
  - 本 PR で構築した prototype-aligned structure / 共通コンポーネントを土台に、機能全体の完成度に合わせて視覚的な最終仕上げを実施
  - 採用画像素材: `<user-local VRChat photo folder>`（user 許可済、本 PR では redact 表記のみ、後続 PR の STOP α で具体採用画像を承認）
  - failure-log §5 ルール（plan メモ + 画面別ワイヤーフレーム + STOP α 承認）を継続適用
- 法的レビュー後の Terms / Privacy 改訂（ローンチ後）
- Phase 2 検索エンジン許可判断（業務知識 v4 §7.6、`<meta robots>` opt-in 化）
- Frontend Workers deploy 専用 runbook 化（最終デザイン PR 時に検討）
- LP からの作成導線（draft 新規作成）が公開された段階で CTA 差し替え
- About の機能ステータスを実装進行に合わせて更新

## 9. Secret / Privacy 取り扱い（PR 全期間）

- redact 対象値（raw slug / raw photobook_id / raw report_id / raw URL / token / Cookie / DATABASE_URL / Secret 値 / source_ip_hash 完全値 / scope_hash 完全値 / reporter_contact / detail）: chat / commit / docs / work-log / failure-log には **未含有**
- 公開承認済の仮置き値（連絡用 X / 運営者表示名 / 準拠法）は意図的に Terms / About に記載
- VRChat 写真フォルダの実 Windows ローカルパスは **書かない**（redact 表記 `<user-local VRChat photo folder>` のみ）
- Cloud Workers version ID（`c2d35a6c-...`、`6f1e82d7-...`）はインフラ識別子として記録

## 10. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-01 | 初版（PR37 design rebuild STOP α / β / δ / ε / final closeout 完了）。prototype-aligned structure / 共通コンポーネント / failure-log §5 ルール適用初事例として完了。最終 visual polish は機能完成後の最終デザイン PR へ延期 |
