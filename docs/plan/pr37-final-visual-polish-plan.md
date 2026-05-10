# PR37 後続: public pages 最終 visual polish 計画書

> **状態**: STOP α 計画書（**実装未着手 / 画像候補は user 側で選定中**）。
> 本計画書は `harness/failure-log/2026-05-01_pr37-public-pages-design-mismatch.md` §5
> ルール（plan メモ + 画面別ワイヤーフレーム + 採用 prototype 画面 ID + STOP α 承認）の
> 適用継続案件。実装・commit・deploy はすべて user 承認後。

---

## 0. メタ情報

| 項目 | 値 |
|---|---|
| 起点 | `bf6fdd3 feat(design): restyle viewer report and error states` 以降の design rebuild 完了状態 |
| 前提 commit | `0a1e557 docs: align deployment log and publish rights knowledge`（HEAD） |
| 前提 Backend revision | `vrcpb-api-00030-2fp`（image `:4e935a9`、observability hotfix 反映済） |
| 前提 Workers version | `2143bd55-19b2-41bd-a3c7-73043bb0873a`（icon / themeColor 反映済） |
| Phase A 状態 | 既に本番反映済（Backend + Workers）。本計画は public pages のみで Phase A に触らない |
| 関連 plan | [`m2-design-refresh-stop-beta-2-plan.md`](./m2-design-refresh-stop-beta-2-plan.md)（β-2c で landing image pipeline 確立） |
| 関連 work-log | [`harness/work-logs/2026-05-01_pr37-design-rebuild-result.md`](../../harness/work-logs/2026-05-01_pr37-design-rebuild-result.md)（構造 rebuild 完了） |
| 関連 failure-log | [`harness/failure-log/2026-05-01_pr37-public-pages-design-mismatch.md`](../../harness/failure-log/2026-05-01_pr37-public-pages-design-mismatch.md)（§5 ルール起点） |

---

## 1. 現状（2026-05-10 確認）

### 1.1 構造 rebuild は完了

前回 PR37（`bf6fdd3`）で **構造・共通コンポーネント・機能・安全面**は完了している。
今回の polish では構造を変えない（変えるのは画像 / typography / 余白 / object-position 等）。

| Public page | 実装ファイル | 構造完成度 | 画像状態 |
|---|---|:-:|---|
| `/` (LP) | `frontend/app/page.tsx` | ✓ | 既存 7 slot 使用（hero / mock-cover / sample-01..05） |
| `/about` | `frontend/app/(public)/about/page.tsx` | ✓ | **画像なし**（design 正典どおり） |
| `/terms` | `frontend/app/(public)/terms/page.tsx` | ✓ | 画像なし（法務文言中心） |
| `/privacy` | `frontend/app/(public)/privacy/page.tsx` | ✓ | 画像なし（法務文言中心） |
| `/help/manage-url` | `frontend/app/(public)/help/manage-url/page.tsx` | ✓ | 画像なし（Q&A 中心） |

### 1.2 landing image pipeline は確立済（β-2c）

| 項目 | 値 |
|---|---|
| Build script | `frontend/scripts/build-landing-images.sh` |
| 入力 | `design/usephot/` の raw VRChat PNG（**gitignore 済 / user-local**） |
| 出力 | `frontend/public/img/landing/{slug}.{webp,jpg}` 14 file（**git に含む static asset**） |
| 7 stable slot | `hero` / `mock-cover` / `sample-01` ... `sample-05` |
| 寸法 | hero 1600px / mock-cover 720px / sample-* 640px |
| 品質 | WebP q70-72 / JPEG q78、`-strip -metadata none` で EXIF 除去 |
| 容量 guard | 合計 ≤ 4 MiB / 各 > 1 KB（`frontend/__tests__/landing-images.test.ts` で test 化済） |
| crop 制御 | `LandingPicture` の `objectPosition` prop で個別調整可能（ε-fix で追加済） |

raw filename → stable name の mapping は build script 内 `MAPPING` 配列に閉じ込め、
**React component / docs / commit message に raw filename を書かない**ルール継続。

### 1.3 共通コンポーネント

| Component | 役割 | 今回 polish 対象 |
|---|---|:-:|
| `PublicTopBar` | 全 public page 共通 header | 維持（変更なし想定） |
| `PublicPageFooter` | 全 public page 共通 footer | 維持（`showTrustStrip` は false 既定で継続） |
| `PolicyArticle` | terms / privacy 用 wf-box card | 視覚的余白 / 行間調整は対象 |
| `MockBook` / `MockThumb` | LP hero の視覚演出 | crop / 比率 / 配置の最終調整は対象 |
| `LandingPicture` | `<picture>` ラッパー | API 維持（objectPosition の利用拡張は OK） |
| `SectionEyebrow` | 小見出し補助 | 維持 |
| `TrustStrip` | （**LP / About では非表示の方針**） | **復活させない**（ε-fix の判断を継続） |

### 1.4 既存 test 群

- `frontend/__tests__/landing-images.test.ts` — landing image asset の existence + 容量 guard
- `frontend/app/__tests__/public-pages.test.tsx` — 5 public page の SSR 構造 / metadata / external services chips 等
- `frontend/components/Public/__tests__/{PublicTopBar,MockBook,TrustStrip,PolicyArticle,PublicPageFooter}.test.tsx` — 各コンポーネント単体

これらの test は polish で **API / 構造を変えない限り壊れない**。test を意図的に修正する変更は plan §6 の境界違反になる可能性が高いので、原則 test の調整なしで通る範囲に polish を限定する。

---

## 2. スコープと境界

### 2.1 対象（5 ページ）

- `/`
- `/about`
- `/terms`
- `/privacy`
- `/help/manage-url`

### 2.2 対象外（明示）

- 公開 Viewer `/p/[slug]` および `/p/[slug]/report`（既に viewer v2 redesign 済）
- `/create` / `/prepare/[id]` / `/edit/[id]` / `/manage/[id]` / `/draft/[token]` / `/manage/token/[token]`
- Backend / DB / migration / Workers binding / env / Secret / Cloud Run Jobs
- design token (`design/source/project/wireframe-styles.css`) の値変更
- prototype (`design/mockups/prototype/`) からの直接 import

### 2.3 触る OK

- `frontend/app/page.tsx` / `frontend/app/(public)/{about,terms,privacy,help/manage-url}/page.tsx`
- `frontend/components/Public/*.tsx`（API 互換維持）
- `frontend/scripts/build-landing-images.sh` の MAPPING 配列（raw → stable name の差し替え）
- `frontend/public/img/landing/*.{webp,jpg}`（再生成後の差し替え）
- 上記に紐づく `__tests__/`（structure の意図的変更を伴わない範囲で必要なら）

---

## 3. 実装 Unit 一覧（推奨 3 本）

細かく STOP を切らず、user 承認単位として 3 Unit に集約する。各 Unit は 1〜2 commit。

### Unit 1: LP polish（最優先）

**意図**: LP の第一印象（hero）と作例ストリップを実画像で確定し、typography rhythm / spacing
を最終形に整える。

**作業範囲**:
- `frontend/app/page.tsx` の hero / sample / use-case section の余白・行間・viewport 切替の最終調整
- `LandingPicture` への `objectPosition` 値を画像個別に最適化
- `MockBook` の cover crop 確認（必要なら `objectPosition` 追加）
- `frontend/scripts/build-landing-images.sh` の MAPPING を user 選定の **採用画像 ID**（§4 参照）に差し替え + `webp/jpg` 再生成
- `frontend/__tests__/landing-images.test.ts` の容量 / 存在 guard が通ることを確認

**意図的に「やらない」こと**:
- TrustStrip 復活
- 新セクション追加
- design token 値変更
- LP コピー（h1 / eyebrow / sub / 「おはツイまとめ！」CTA 文）の本文変更（**作例文脈を維持**）

**完了条件**:
- LP が承認画像で本番品質に達している（user 視覚承認）
- `landing-images.test.ts` PASS（合計 ≤ 4 MiB 維持）
- `public-pages.test.tsx` PASS（structure 不変）
- 新 chunk size の悪化が 10% 以内

### Unit 2: About + 法務 / Help 静的ページの読みやすさ polish

**意図**: 信頼性を損なわない範囲で、静的ページの readability / 余白 / TOC / section rhythm を
整える。**写真装飾は原則追加しない**。

**作業範囲**:
- `/about`: 視覚的階層（h1 → eyebrow → wf-box → canDo / cannotDo grid）の余白・行間最終調整。
  画像追加は **任意 1 枚まで**（後述 §5.2）、入れる場合は控えめなアクセント
- `/terms`: PolicyToc の anchor jump 動作 / wf-box カードの行間 / `wf-note` の主張感（過剰でない）
- `/privacy`: 同上 + external services chips の整列
- `/help/manage-url`: Q&A wf-box の質問見出しと回答本文の rhythm（読み手の自然さ優先）
- `PolicyArticle.tsx` の余白 token 確認（必要なら `padding` / `gap` 微調整、token 値は変えない）

**意図的に「やらない」こと**:
- 法務文言の削減 / 言い換え（**業務知識 v4 §7 を Single Source of Truth として削らない**）
- 新規装飾画像の terms / privacy 追加（読みやすさ阻害リスク）
- TOC アンカー ID の変更（既存 anchor `#article-N` を test と Frontend が依存）

**完了条件**:
- 4 静的ページが本番品質（user 視覚承認）
- 法務文言の文字列差分が 0 件（diff で確認、見た目調整のみ）
- `public-pages.test.tsx` PASS
- About に画像を入れた場合は landing image 8 枚目 slot を追加した上で test guard 更新

### Unit 3: verification + deploy readiness

**意図**: U1 / U2 を本番反映する直前の最終確認と smoke 計画固定。

**作業範囲**:
- 全 verification 実行:
  - `npm --prefix frontend run typecheck`
  - `npm --prefix frontend run test`
  - `npm --prefix frontend run build`
  - `npm --prefix frontend run cf:build`
  - `npm --prefix frontend run cf:check`
  - `git diff --check`
  - raw value grep（`docs cleanup` と同 11 パターン）
  - landing image 容量 / 個数の最終確認
  - bundle size の悪化チェック（`build` 出力の First Load JS / chunk 一覧を pre/post で比較）
- smoke チェックリスト固定:
  - 既存 routes regression（`/health` 系は Backend のため対象外、Workers 側 5 page の HTTP 200）
  - icon / themeColor 維持確認（Workers redeploy 後も 200 / `<meta theme-color>` 維持）
  - Phase A bundle marker 維持確認（`page-action-bar` / `page-caption-editor` / `page-move-picker` / `draft-preview` 等）
  - Safari smoke 観点を plan §7 にリストアップ
- deploy 用 work-log の雛形作成（`harness/work-logs/2026-05-XX_public-polish-deploy-plan.md`）

**意図的に「やらない」こと**:
- 実 deploy（`wrangler deploy`）— deploy は Unit 3 完了後に user 承認で別 STOP

**完了条件**:
- すべての verification が PASS
- smoke チェックリストが work-log に固定済み
- user 承認 → deploy 別 STOP に進む

---

## 4. 画像候補整理（user 側で選定する作業）

### 4.1 raw 素材の取り扱い（厳守）

- raw VRChat PNG は `design/usephot/` 配下に置く（既存 path、`.gitignore` 済）
- **raw filename / 実 path を plan / commit message / report に書かない**
- raw PNG は **commit しない**（build script で生成された `webp/jpg` のみ commit）
- raw → stable name の mapping は `frontend/scripts/build-landing-images.sh` の `MAPPING` 配列に閉じ込める
- React component / docs では **stable name と匿名 ID 経由でのみ参照**

### 4.2 plan 内での匿名 ID 命名

候補画像をやり取りするときは以下の prefix を使う（実 file 名と無関係）:

| 匿名 ID | 想定スロット | 用途 |
|---|---|---|
| `public-polish-photo-01` | `hero` | LP 上部メイン視覚（横長 16:9） |
| `public-polish-photo-02` | `mock-cover` | MockBook 左側 cover（縦長 9:16） |
| `public-polish-photo-03` | `sample-01` | sample strip 1 枚目（mobile 1:1 / PC 4:3） |
| `public-polish-photo-04` | `sample-02` | sample strip 2 枚目 |
| `public-polish-photo-05` | `sample-03` | sample strip 3 枚目 |
| `public-polish-photo-06` | `sample-04` | sample strip 4 枚目 |
| `public-polish-photo-07` | `sample-05` | sample strip 5 枚目 |
| `public-polish-photo-08`（任意） | About hero | About に控えめアクセント画像を入れる場合のみ |

> 既存 7 slot を使い回す場合は新規生成不要。既存画像で OK なら slot ごとに「採用継続」と明記すれば足りる。
> 8 枚目を追加する場合は `frontend/scripts/build-landing-images.sh` に新 stable name と寸法を追加し、`landing-images.test.ts` の `STABLE_NAMES` / `MAX_TOTAL_BYTES` も連動更新（4 MiB → 4.5 MiB 等の見直しは Unit 2 で実施）。

### 4.3 採用判断軸（user が候補を絞るときのチェックリスト）

各 photo を以下の 6 軸で○/△/×評価し、すべて ○ または △ までを採用候補とする:

1. **VRChat 文脈が伝わる**（avatar / world / 雰囲気が明確）
2. **顔や主役が crop で切れない**（`object-cover` 中央クロップに耐える、または `objectPosition` で調整可能）
3. **他者の写り込み・権利リスクが低い**（公開不適切な avatar / 文字 / 場所が映り込んでいない）
4. **LP hero / sample / about の用途で不自然でない**（暗すぎない、UI と喧嘩しない色味）
5. **mobile 1:1 / PC 4:3 / 16:9 のいずれかに耐える**（用途別の crop 耐性）
6. **暗すぎない / 情報量が多すぎない**（thumbnail で潰れない）

### 4.4 user に決めてもらうこと（Unit 1 着手前の必須インプット）

| 項目 | 例 |
|---|---|
| **既存 7 slot を継続するか / 差し替えるか** | 「hero と sample-03 だけ差し替え、他は継続」のような部分差し替え可 |
| **差し替える場合の slot ↔ 匿名 ID 対応** | `hero ← public-polish-photo-01` のように 1 行ずつ |
| **About に 8 枚目を入れるか** | 入れる / 入れない / 既存 slot を流用（hero を再利用、等） |
| **objectPosition の希望値があるか** | 顔の高さ / 主役の位置で「center 30%」等の指示があれば |

決定が揃った時点で、Unit 1 の MAPPING 差し替え + Unit 2 の About 画像有無が確定し実装着手可能。

---

## 5. 各画面の polish 方針

### 5.1 LP `/`

- **第一印象最優先**: hero 画像と「無料でフォトブックを作る」CTA の視覚的引力を強化
- **「おはツイまとめ！」の作例文脈を維持**（h1 / eyebrow / sub 文言は変えない）
- **TrustStrip は復活させない**（ε-fix で「LP の集中導線を弱める」と判断済）
- **`object-position` / crop を明示的に管理**: 各画像の objectPosition を明示指定し、`object-cover` のデフォルト中央クロップに任せない
- **typography rhythm 微調整**: 既存 design token を使った class 組み換えのみ（token 値は変えない）
- **section spacing**: hero → sample strip → 4-feature → use-case → CTA band の section gap を visual rhythm として整える

### 5.2 About `/about`

- **画像を入れるなら控えめに 1 枚まで**（hero 直下の wf-box 装飾、または canDo grid 上部のアクセント）
- **画像追加が信頼性を損なわないこと**を優先（過剰に華美にしない）
- **dl meta（運営 / 運営者表示名 ERENOA / @Noa_Fortevita）は production truth として維持**
- **canDo (6) / cannotDo (4) 本文は削らない**
- 視覚的階層調整は token 内で完結（h1 → eyebrow → wf-box → grid の余白 / 行間）

### 5.3 Terms `/terms` / Privacy `/privacy`

- **法務文言は削らない**（業務知識 v4 §7.1〜§7.5 を Single Source of Truth として維持）
- **写真装飾は原則追加しない**（読みやすさ阻害 + noindex 対象なので装飾の意義薄）
- **readability / spacing / TOC / section rhythm 中心**:
  - PolicyToc の anchor jump スムーズスクロール挙動の確認
  - PolicyArticle wf-box card の行間 / padding の最終調整
  - `wf-note`（PolicyNotice）の visual 強度を「過剰主張しない」レベルに整える
  - external services chips（privacy のみ）の整列を整える
- **anchor ID（`#article-1` 〜）は変えない**（test と内部リンクが依存）

### 5.4 Help `/help/manage-url`

- **Q&A の読みやすさ中心**（写真より操作理解優先）
- **Q1〜Q6 wf-box card の構造維持**
- 質問見出し（疑問形）と回答本文の rhythm を整える
- **anchor ID（`help-q1` 〜 `help-q6`）は変えない**
- 操作画面のスクリーンショット等の追加は **やらない**（生成 / 維持コストに対し効果が薄い）

---

## 6. 触らない境界（recap）

- design token (`design/source/project/wireframe-styles.css`) の値は変えない
- prototype (`design/mockups/prototype/`) を直接 import しない
- Backend / DB / migration / Workers binding / env / Secret / Cloud Run Jobs / Cloud Scheduler を触らない
- 法務文言 / Q&A 本文 / canDo・cannotDo 本文を削らない
- raw token / Cookie / Secret / storage_key / presigned URL を docs / commit / log に書かない
- raw image filename / 実 path を docs / commit / log に書かない（匿名 ID + stable name のみ）
- TrustStrip を LP / About に復活させない（コンポーネント自体は単体テスト用に残置）
- Phase A 機能 / Backend observability hotfix を触らない
- deploy（`wrangler deploy` / `gcloud builds submit` / Cloud Build trigger）は Unit 3 完了後の別 STOP

---

## 7. 実装時の検証方針

### 7.1 各 Unit 共通の必須チェック

```bash
# Frontend
npm --prefix frontend run typecheck
npm --prefix frontend run test
npm --prefix frontend run build
npm --prefix frontend run cf:build
npm --prefix frontend run cf:check

# Static
git diff --check
bash scripts/check-stale-comments.sh

# raw value grep（pre-commit）
git diff --staged --name-only | xargs -I{} grep -lE \
  'manage_url_token=[a-z0-9]|draft_edit_token=[a-z0-9]|Cookie: [a-zA-Z0-9]+=|Set-Cookie: [a-zA-Z0-9]+=|Bearer [a-zA-Z0-9]{20}|storage_key=[a-z0-9]|presigned_url=[a-z0-9]|sk_live_|sk_test_|DATABASE_URL=postgres|TURNSTILE_SECRET_KEY=[a-z0-9]|R2_SECRET_ACCESS_KEY=[a-z0-9]' {} 2>/dev/null \
  || echo "(漏洩 0 件)"
```

### 7.2 landing image 専用チェック

- `frontend/__tests__/landing-images.test.ts` PASS
- 合計サイズ ≤ 4 MiB（slot 数を増やす場合は plan で更新後に test 上限も連動更新）
- 各 file > 1 KB
- raw `.png` が `frontend/public/img/landing/` に混入していない（test 内 guard）
- raw filename が React component / 直 commit に出ていない（grep）

### 7.3 bundle size の悪化チェック（Unit 1 / 2 の post）

```bash
# pre/post で First Load JS と Route 別サイズを比較
npm --prefix frontend run build 2>&1 | grep -E 'First Load JS|^├|^└' | tee /tmp/build-size.txt
```

悪化基準（参考）: First Load JS が 10% 以上増加したら原因確認。画像差し替えのみで JS bundle は通常変動しない。

### 7.4 mobile Safari / desktop Chrome smoke（Unit 3）

実機 smoke は user 担当。チェック観点を Unit 3 完了報告に列挙する:

| 観点 | 確認項目 |
|---|---|
| LP 第一印象 | hero 画像 + h1 + CTA が above-the-fold で破綻なく組まれている |
| LP image crop | object-position の意図通りに主役が見える、顔が切れていない |
| Mobile レイアウト | iPhone Safari で section 間の余白が窮屈 / 過剰でない |
| PC レイアウト | desktop Chrome で wf-grid 系が prototype と整合 |
| About | サービス紹介の信頼性 + canDo / cannotDo の対比 |
| Terms / Privacy | 法務文言の読みやすさ、anchor jump の自然さ |
| Help | Q&A の質問 → 回答の rhythm |
| theme-color / icon | Phase A icon / themeColor が引き続き反映 |
| 既存 Phase A | `/edit` 等の機能ページが本変更で壊れていない |

### 7.5 deploy 不要部分の確認（Unit 3）

- DB migration: 不要
- Secret / env / binding: 不要
- Cloud Run service: 不要（Backend 不変）
- Cloud Run Jobs image tag 同期: 不要（Backend 不変、Jobs drift 許容方針継続）
- 対象は **Workers のみ**（`wrangler deploy`）

---

## 8. open questions（user 判断待ち）

実装着手前に user に確定してもらう項目:

1. **LP の既存 7 slot を継続するか / 差し替えるか**
   - 全継続 / 部分差し替え（slot 単位） / 全差し替え
2. **About に画像を入れるか**
   - 入れない / 1 枚（slot 8 を追加 or 既存 hero を流用）
3. **採用候補画像の匿名 ID ↔ slot 対応**
   - 例: `hero ← public-polish-photo-01`
4. **objectPosition の希望値**
   - 顔・主役の位置で個別指示があれば（なければ user 視覚承認サイクルで微調整）
5. **Unit 1 / Unit 2 / Unit 3 を 1 commit にまとめるか / Unit 単位で commit を分けるか**
   - 推奨: **Unit 単位で 3 commit**（レビューしやすく rollback もしやすい）
6. **bundle size 悪化の許容基準**
   - 推奨: First Load JS 増 ≤ 10%、image 合計 ≤ 4 MiB（slot 8 追加時は ≤ 4.5 MiB に上限緩和）

---

## 9. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-10 | 初版作成（plan-only commit、実装未着手）。STOP α として本書を user 承認に提出。Unit 1 / Unit 2 / Unit 3 の 3 本に集約 |
