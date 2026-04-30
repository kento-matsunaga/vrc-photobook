# PR37 LP / Terms / Privacy / About 実装結果（2026-05-01、完了）

> **状態**: STOP α / β / δ / ε / final closeout 全完了。**ただし design 品質は user 意図と大きく乖離しており、後続の design rebuild が必須**（再発防止のため failure-log 起票済）。
> 起点 commit: `5d85af5 feat(frontend): add LP, terms, privacy, about pages`
> closeout commit: 本書反映 commit

## 0. 本書のスコープ

PR37「LP / Terms / Privacy / About 公開ページ整備」の進行記録。STOP α 計画 → STOP β 実装 → STOP δ Workers redeploy → STOP ε Safari 実機確認 → final closeout までを集約する。

機能・安全面（HTTP 200 / noindex / Referrer-Policy / metadata 上書き / 既存経路 regression / Secret + raw 値漏えい 0 件）は要件を満たしたが、視覚デザイン品質が user 意図と大きく乖離したため、design rebuild を後続必須事項として明記する。

## 1. STOP α（計画、2026-05-01 ユーザー承認）

ユーザー判断 7 項目確定 + design ファイル群読込指示:
- 採用案: 案 B（個別ページ自前 footer + 共通 PublicPageFooter コンポーネント追加）
- 仮置き値（user 承認済）: 連絡先 `https://x.com/Noa_Fortevita` / 運営者表示名 `ERENOA` / 法的レビュー予定「ローンチ後」/ 作成導線 URL「未提供」/ 準拠法・管轄「日本法・東京地裁第一審専属管轄」
- design ファイル必読: `design/README.md` / `design/mockups/README.md` / `design/design-system/{colors,typography,spacing,radius-shadow}.md` / `design/mockups/prototype/{styles.css,shared.jsx,pc-shared.jsx,screens-a.jsx,screens-b.jsx,pc-screens-a.jsx,pc-screens-b.jsx}`

## 2. STOP β（実装、commit `5d85af5`）

### 2.1 変更ファイル

| ファイル | 種別 |
|---|---|
| `frontend/app/page.tsx` | M（rewrite 92%、LP 本実装に置換）|
| `frontend/app/(public)/terms/page.tsx` | A（第 1〜9 条）|
| `frontend/app/(public)/privacy/page.tsx` | A（第 1〜10 条）|
| `frontend/app/(public)/about/page.tsx` | A（できる 6 / できない 4 / ポリシー導線）|
| `frontend/app/(public)/help/manage-url/page.tsx` | M（関連リンクに About / Terms / Privacy 追加）|
| `frontend/components/Public/PublicPageFooter.tsx` | A（共通 footer）|
| `frontend/components/Viewer/ViewerLayout.tsx` | M（footer に nav リンク + 既存通報リンク）|
| `frontend/app/__tests__/public-pages.test.tsx` | A（renderToStaticMarkup 7 ケース）|

合計 8 files / +1000 / -24。

### 2.2 design-system 反映

- Tailwind token: `brand-teal` / `ink` / `surface` / `divider` / `status` のみ使用、`gray-*` 等の Tailwind 既定色は不使用
- Typography: `text-h1` / `text-h2` / `text-body` / `text-sm` / `text-xs` のみ
- Spacing: `max-w-screen-md` + `px-4 sm:px-6` + `mt-6`〜`mt-8` + `p-4`
- Radius / Shadow: `rounded-lg` + `shadow-sm`、装飾的 gradient 不使用

### 2.3 検証結果

| 項目 | 結果 |
|---|---|
| `typecheck` | OK |
| `vitest run` | 14 files / 139 tests 全 PASS（新規 7 ケース含む）|
| `next build` | OK（`/`、`/about`、`/privacy`、`/terms` は Static 生成）|
| `cf:build` | OK（OpenNext bundle 生成）|
| `cf:check` | OK（dry-run、bindings 維持）|
| commit | `5d85af5 feat(frontend): add LP, terms, privacy, about pages` |
| push | `73257c9..5d85af5  main -> main` SUCCESS |

## 3. STOP δ Workers redeploy（完了）

| 項目 | 値 |
|---|---|
| `cf:build` 再実行 | OK |
| `wrangler deploy` | SUCCESS（22.80 sec、Total Upload 4738.33 KiB / gzip 976.03 KiB、Worker Startup 41 ms）|
| 新 Worker version | **`6f1e82d7-cf57-41ab-99dd-0ede5266a3a5`**（100% active）|
| 直前 active（rollback candidate）| `ac2b884a-7c75-49d3-a21c-5c2a66c462ed`（PR36 STOP δ 由来）|
| bindings 維持 | `OGP_BUCKET (vrcpb-images)` / `ASSETS` |
| binding 変更 | なし |
| Backend / Cloud Run / Cloud SQL / Secret / migration / Job / Scheduler への操作 | **なし** |

## 4. STOP ε Safari 実機確認（完了、機能・安全面）

### 4.1 4 ページ + Help: HTTP / noindex / metadata

| path | HTTP | X-Robots-Tag | Referrer-Policy | meta robots | metadata title 上書き |
|---|---|---|---|---|---|
| `/` | 200 | noindex, nofollow | strict-origin-when-cross-origin | yes | 「VRC PhotoBook｜VRChat 写真をログイン不要で 1 冊に」|
| `/about` | 200 | noindex, nofollow | strict-origin-when-cross-origin | yes | 「VRC PhotoBook について｜About」|
| `/terms` | 200 | noindex, nofollow | strict-origin-when-cross-origin | yes | 「利用規約｜VRC PhotoBook」|
| `/privacy` | 200 | noindex, nofollow | strict-origin-when-cross-origin | yes | 「プライバシーポリシー｜VRC PhotoBook」|
| `/help/manage-url` | 200 | noindex, nofollow | strict-origin-when-cross-origin | yes | 既存（関連リンクに About / Terms / Privacy 追加）|

middleware 由来の `X-Robots-Tag` ヘッダ + 各ページ `metadata.robots` 由来の `<meta>` の **両方** を確認。

### 4.2 既存経路 regression

| path | HTTP | 備考 |
|---|---|---|
| `/help/manage-url` | 200 | 関連リンク追加版が反映、既存表示維持 |
| `/p/<dummy>` | 404 | Next.js 標準（既存挙動）|
| `/p/<dummy>/report` | 404 | 同上 |
| `/manage/<dummy>` | 200（Cookie なし）| `Referrer-Policy: no-referrer` 維持（token URL 漏洩対策） |
| `/og/default.png` | 200 | image/png 維持 |
| `/ogp/<dummy>?v=1` | 302 | fallback 維持 |

regression なし。

### 4.3 design 品質乖離（最重要、後続必須）

**user 所見: 「design に盛大に違う」「design ファイル群を読んだのに視覚イメージが意図と大きく乖離」**

詳細は failure-log: [`harness/failure-log/2026-05-01_pr37-public-pages-design-mismatch.md`](../failure-log/2026-05-01_pr37-public-pages-design-mismatch.md)。

機能・安全面は要件を満たしたが、視覚デザイン品質は要件未達。design rebuild は必須として roadmap §1.3 後続候補に積む。

## 5. final closeout（完了、本 commit）

PR closeout チェックリスト（`.agents/rules/pr-closeout.md` §6 準拠）:

- [x] STOP β 実装 commit `5d85af5` push 確認
- [x] STOP δ deploy 結果 + 新 Worker version 記録
- [x] STOP ε 機能・安全面結果記録
- [x] **design 乖離を failure-log として起票**（再発防止のためルール化対象）
- [x] roadmap §1.1 Worker version 更新 + §1.3 PR37 完了マーカー + design rebuild 後続候補
- [x] runbook 追記要否判断（後述 §7）
- [x] stale-comments 4 区分分類
- [x] Secret / raw 値漏えい grep 0 件
- [x] generated files（`.open-next` / `.wrangler`）git 不該当
- [x] tmp / proxy / DSN cleanup（本 PR では DB アクセスなし、tmp 残存なし）

## 6. Secret / raw 値の取り扱い

- raw slug / raw photobook_id / raw report_id / raw URL / token / Cookie / DATABASE_URL / Secret 値 / source_ip_hash 完全値 / scope_hash 完全値 / reporter_contact 実値 / detail 実値: chat / commit / docs / work-log / failure-log には **未含有**（grep 0 件）
- 仮置き値（運営者表示名 `ERENOA` / 連絡用 X `@Noa_Fortevita` / 法的レビュー予定 / 準拠法）は user 承認済の **公開対象値**で、機密ではない
- Secret literal grep の hits は test 内の検出パターン正規表現定義（`/\bsk_live_[A-Za-z0-9]+/` 等）のみで実値ではない（PR36 closeout / SubmitReport 緩和 PR と同パターン）

## 7. runbook 追記判断

**追記なし** とする:
- 本 PR は Frontend 公開ページ追加のみ。Backend / Cloud SQL / Secret / migration / Cloud Run Job / cmd/ops いずれも不変
- `docs/runbook/backend-deploy.md` / `docs/runbook/usage-limit.md` / `docs/runbook/ops-moderation.md` への影響なし
- Frontend deploy 専用 runbook は現状未整備（Workers deploy 手順は `frontend/package.json` scripts + `wrangler.jsonc` で十分）。後続でデザインリビルド時に Workers deploy runbook 化を検討する余地あり（roadmap §1.3 候補に潜在）
- 本 PR は work-log + roadmap + failure-log に集約

## 8. 後続候補

最重要から順:

- **PR37 public pages design rebuild**（最優先、failure-log 起点）
  - design ファイル群を読んだだけでは user 期待に届かなかった
  - 次回は実装前にページ単位のワイヤーフレーム / 見た目方針をユーザー承認する
  - LP / About / Terms / Privacy を `design/mockups/prototype` と `design-system` に沿って再構築する
  - 採用する concept image / prototype 画面を STOP α の段階で明示する
- 法的レビュー後の Terms / Privacy 改訂（ローンチ後）
- Phase 2 検索エンジン許可判断（業務知識 v4 §7.6、`<meta robots>` を作成者の opt-in で `index, follow` に切り替えるかどうか）
- Frontend Workers deploy 専用 runbook 化（必要時）
- LP からの作成導線（draft 新規作成）が公開された段階で CTA 差し替え
- About の機能ステータス（できる / できない）を実装進行に合わせて更新

## 9. Secret / Privacy 取り扱い（PR 全期間）

- redact 対象値（raw slug / raw photobook_id / raw report_id / raw URL / token / Cookie / DATABASE_URL / Secret 値 / source_ip_hash 完全値 / scope_hash 完全値 / reporter_contact / detail）: chat / commit / docs / work-log / failure-log には **未含有**
- 公開承認済の仮置き値（連絡用 X / 運営者表示名 / 準拠法）は意図的に Terms / About に記載
- Cloud Workers version ID（`6f1e82d7-...`、`ac2b884a-...`）はインフラ識別子（rollback 参照用）として記録、PII ではない

## 10. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-01 | 初版（PR37 STOP α / β / δ / ε / final closeout 完了）。design 品質乖離を failure-log として起票し、roadmap §1.3 に design rebuild を後続候補として積んだ |
