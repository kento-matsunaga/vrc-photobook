# Non-Intrusive Ads (AdSlot placeholder) 導入計画

> 位置付け: ローンチ前運用整備フェーズの後続タスク。`docs/plan/vrc-photobook-final-roadmap.md`
> §3 PR41+「ローンチ後改善」より前の追加 PR ラインとして扱う（番号は **PR42a / PR42b**
> の暫定）。Public repo 化（PR38）/ 本番運用整備（PR39）/ ローンチ前チェック（PR40）の
> 順序自体は変えない。

## 0. 背景と方針

### 0.1 何を解決するか

- 個人運営の運用費用回収のため、邪魔にならない範囲で広告枠を設けたい
- 配信元は **Amazon アソシエイト** を想定（固定 img + a タグ、script 不要）
- ただし実 ASP 登録 / 実商品リンク / Privacy 改訂は本 PR の責任範囲外

### 0.2 方針（ユーザ合意済）

1. **左右バナーは「Sidebar 末尾組込み」案 (X)**: PageNavSidebar 末尾 (PC 左) /
   RightPanel 末尾 (PC 右) / Mobile only block 末尾の 3 箇所を Viewer 内に配置
2. **LP 4 ページ末尾**: `/about` `/help/manage-url` `/terms` `/privacy` の footer 直前
3. **編集系には入れない**: `/create` `/edit` `/manage` `/prepare` `/draft` `/p/[slug]/report`
4. **本 PR は placeholder のみ**: env flag `NEXT_PUBLIC_ADS_ENABLED=true` のときだけ
   DOM に出力する gradient + 「広告」label の dummy。実 ASP は別 PR
5. **景表法ステマ規制 (2023-10〜) 準拠**: 「広告」label 必須（aria-label + 視覚表記）
6. **layout shift 抑止**: `min-height` で reserve、`position: sticky/fixed/overlay` 禁止
7. **本番 deploy 前に Privacy / Terms 改訂**: §第 4 条「第三者提供」§第 5 条「外部サービス」
   chip 追加 + Amazon アソシエイト参加表記（`/about` または `/help`）

### 0.3 PR 分割

| PR | スコープ | ブランチ |
|---|---|---|
| **PR42a**: AdSlot placeholder 導入 | コンポーネント新設 + 配置 + guard test + flag OFF deploy | `claude/add-non-intrusive-ads-g1ySN` |
| **PR42b**: 本番投入 (実 ASP / Privacy・Terms 改訂 / flag ON deploy) | Amazon アソシエイト連携 + 法務改訂 + Safari 実機 + post-deploy smoke | 別ブランチ（PR42a merge 後に切る） |

PR42b は **PR38（Public repo 化、履歴 secret scan）が完了してから着手する**。
理由: PR42b で Amazon アソシエイト tracking ID 等の準 secret 相当の値を本番 build に
入れるため、履歴 scan 後の方が安全。

---

## 1. 関連ファイル / 現状把握

### 1.1 既存コードの構造

| ファイル | 役割 | 改修要否 |
|---|---|---|
| `frontend/components/Viewer/ViewerLayout.tsx` | Viewer 全体 (3 col grid / Mobile stack) | **PR42a 改修**: Mobile only block 末尾に AdSlot |
| `frontend/components/Viewer/PageNavSidebar.tsx` | PC 左 sidebar (scroll spy nav) | **PR42a 改修**: 末尾に AdSlot |
| `frontend/components/Viewer/RightPanel.tsx` | PC 右 panel (About/Creator/Share/CTA/通報) | **PR42a 改修**: 末尾に AdSlot |
| `frontend/app/(public)/about/page.tsx` | About LP | **PR42a 改修**: footer 直前に AdSlot |
| `frontend/app/(public)/help/manage-url/page.tsx` | Help LP | **PR42a 改修**: 同上 |
| `frontend/app/(public)/terms/page.tsx` | 利用規約 | **PR42a 改修**: 同上 |
| `frontend/app/(public)/privacy/page.tsx` | プライバシー | **PR42a 改修**: 同上 + PR42b で §4 §5 改訂 |
| `frontend/middleware.ts` | header 制御 (X-Robots-Tag / Referrer-Policy) | 変更なし |
| `frontend/wrangler.jsonc` | Workers 設定 | 変更なし（NEXT_PUBLIC_* は build 時 inline） |
| `frontend/.env.production.example` | env サンプル | **PR42a 改修**: `NEXT_PUBLIC_ADS_ENABLED` の例を追記 |
| `frontend/__tests__/harness-class-guards.test.ts` | 横断 antipattern guard | **PR42a 改修**: 編集系から AdSlot import 禁止 guard 追加 |

### 1.2 既存ルール / 制約

- `.agents/rules/safari-verification.md`: モバイル UI / レスポンスヘッダ変更時は Safari 実機必須
  → PR42a は **DOM 構造変化なし (flag OFF)**、PR42b で実機確認
- `.agents/rules/predeploy-verification-checklist.md`: deploy 完了基準は production bundle marker grep まで
  → PR42b で適用
- `.agents/rules/pr-closeout.md`: PR 完了前に stale コメント / 先送り記録 / Secret grep
- `.agents/rules/security-guard.md`: Cookie / token / Secret の DOM 出力禁止
- `.agents/rules/testing.md`: テーブル駆動 + Builder + description 必須
- `.agents/rules/coding-rules.md`: `any` / `interface{}` 禁止、明示的 > 暗黙的
- CLAUDE.md スマホファースト方針 → モバイル下バナーは必須

---

## 2. 設計詳細

### 2.1 AdSlot コンポーネント仕様

ファイル: `frontend/components/Ads/AdSlot.tsx`

```tsx
"use client" は不要 (Server Component で良い)。

Props:
  - placement: string  // "viewer-sidebar-left" / "viewer-sidebar-right" / "viewer-mobile"
                       // / "lp-about" / "lp-help" / "lp-terms" / "lp-privacy"
                       // data-testid と aria-label の suffix に使う
  - width: number      // CLS 対策の min-width
  - height: number     // CLS 対策の min-height
  - children?: ReactNode  // 実 ASP banner の差し込み口。未指定なら placeholder

挙動:
  - process.env.NEXT_PUBLIC_ADS_ENABLED !== "true" → return null (DOM ごと消す)
  - "true" のとき:
    - aria-label="広告" / role="complementary" の wrapper
    - 視覚表記: 上部に小さく「広告」label (text-[10px] text-ink-soft)
    - children が無ければ gradient placeholder (from-teal-50 to-surface-soft)
    - children が有れば children をそのまま埋め込む（実 ASP 投入後の差し込み）
    - data-testid=`ad-slot-${placement}`
    - style に minWidth / minHeight を inline で指定

禁止:
  - position: sticky / fixed / absolute
  - z-index 操作で本文を覆う
  - photobook_id / image_id / token を data-* / DOM に出す
```

### 2.2 配置仕様

| placement | ファイル | 位置 | サイズ | 表示条件 |
|---|---|---|---|---|
| `viewer-sidebar-left` | `PageNavSidebar.tsx` | nav 末尾 (pages.map の後) | 160×600 | PC (`hidden lg:block` 内) |
| `viewer-sidebar-right` | `RightPanel.tsx` | 通報リンクの後 | 300×250 | PC (`hidden lg:block` 内) |
| `viewer-mobile` | `ViewerLayout.tsx` | Mobile only block の末尾 (通報リンクの後) | 300×250 | Mobile (`lg:hidden` 内) |
| `lp-about` | `app/(public)/about/page.tsx` | `<PublicPageFooter />` 直前 | 300×250 | PC/Mobile 共通 |
| `lp-help` | `app/(public)/help/manage-url/page.tsx` | 同上 | 300×250 | 同上 |
| `lp-terms` | `app/(public)/terms/page.tsx` | 同上 | 300×250 | 同上 |
| `lp-privacy` | `app/(public)/privacy/page.tsx` | 同上 | 300×250 | 同上 |

### 2.3 env flag 仕様

```
NEXT_PUBLIC_ADS_ENABLED
  - 値: "true" | "false" | undefined
  - "true" 以外はすべて OFF (DOM 出力なし)
  - .env.production.example にコメント付きで「未設定 = OFF」を明記
  - 本番 Workers 投入は PR42b で .env.production に "true" を書いた状態で
    `npm run cf:build && wrangler deploy`
  - bundle に inline されるため、wrangler runtime env では切り替え不可
    （README.md §181-182 に既出の方針に従う）
```

### 2.4 テスト仕様

新規:
- `frontend/components/Ads/__tests__/AdSlot.test.tsx`
  - 「flag OFF (`undefined` / `"false"` / `"True"` (大文字)) で null」(3 ケース)
  - 「flag ON で `data-testid=ad-slot-${placement}` が出る」
  - 「flag ON で「広告」label と `aria-label` が出る」
  - 「flag ON で children を埋め込める」
  - 「flag ON で `position: sticky/fixed/absolute` の class が無い」(antipattern guard)
  - 「width / height が min-width / min-height として反映される」

変更:
- `frontend/components/Viewer/__tests__/ViewerLayout.test.tsx`
  - 「正常_Mobile only block 末尾に `ad-slot-viewer-mobile` が来る」(env mock で flag ON)
  - 「ガード_PageHero 間に `ad-slot-*` が無い」(structure guard)
- `frontend/__tests__/harness-class-guards.test.ts`
  - 「ガード_`app/(draft)/` `app/(manage)/` `app/(public)/p/[slug]/report` から
    `AdSlot` が import されていない」(source scan)
- `frontend/app/__tests__/public-pages.test.tsx`
  - 「ガード_about / help / terms / privacy の footer 直前に `ad-slot-lp-*` が来る」

### 2.5 ロードマップ追記内容

`docs/plan/vrc-photobook-final-roadmap.md` §1.3「未実装」配下に追加:

```markdown
#### 広告枠 (運用費用回収目的)
- **PR42a 広告枠 placeholder 導入**: AdSlot コンポーネント + Viewer / LP 7 箇所配置
  + flag OFF deploy。実 ASP / Privacy・Terms 改訂は PR42b で対応
- **PR42b 広告本番投入**: Amazon アソシエイト連携 + Privacy §4 §5 / Terms / About 改訂
  + Safari 実機 + post-deploy smoke。PR38（Public repo 化）完了後に着手
```

---

## 3. タスク分解

### Phase 1: PR42a (placeholder 導入)

ブランチ: `claude/add-non-intrusive-ads-g1ySN`

#### Backlog

- [ ] **P1-T1**: ロードマップ追記 + 本計画書 commit
  - 完了条件:
    - `docs/plan/m2-non-intrusive-ads-plan.md`（本書）が commit 済
    - `docs/plan/vrc-photobook-final-roadmap.md` §1.3 に PR42a/PR42b の状態ベース記述が追加
    - ユーザ承認 → P1-T2 へ進む
  - 依存: なし

- [ ] **P1-T2**: `AdSlot.tsx` + 単体テスト (TDD)
  - 完了条件:
    - `frontend/components/Ads/AdSlot.tsx` 作成
    - `frontend/components/Ads/__tests__/AdSlot.test.tsx` 作成
    - `cd frontend && npm run test -- AdSlot` でテーブル駆動 6 ケース PASS
    - `npm run lint` / `npm run typecheck` PASS
    - DOM に raw photobook_id / token が出ないことを test で assert
  - 依存: P1-T1
  - 注意:
    - テスト先行 (赤 → 緑 → リファクタ)
    - `vi.stubEnv("NEXT_PUBLIC_ADS_ENABLED", "true")` パターンを使う
    - Builder パターンは props 少数のため使わず直接構築

- [ ] **P1-T3**: Viewer 3 箇所への組込み
  - 完了条件:
    - `PageNavSidebar.tsx` 末尾に `<AdSlot placement="viewer-sidebar-left" width={160} height={600} />`
    - `RightPanel.tsx` 末尾に `<AdSlot placement="viewer-sidebar-right" width={300} height={250} />`
    - `ViewerLayout.tsx` Mobile only block 末尾に `<AdSlot placement="viewer-mobile" width={300} height={250} />`
    - `ViewerLayout.test.tsx` の既存 7 テスト + 新規 2 ケース (Mobile slot / PageHero 間に無い) が PASS
  - 依存: P1-T2
  - 注意: PR42a 段階では flag OFF なので、test は env mock で flag ON にして DOM 構造を確認

- [ ] **P1-T4**: LP 4 ページへの組込み
  - 完了条件:
    - `app/(public)/about/page.tsx` `<PublicPageFooter />` 直前に AdSlot
    - `app/(public)/help/manage-url/page.tsx` 同上
    - `app/(public)/terms/page.tsx` 同上
    - `app/(public)/privacy/page.tsx` 同上
    - `public-pages.test.tsx` に 4 ページ分の structure 確認テストを追加 PASS
  - 依存: P1-T2 (P1-T3 とは並行可能)

- [ ] **P1-T5**: 横断 guard test 追加
  - 完了条件:
    - `harness-class-guards.test.ts` に新 describe block:
      「`app/(draft)/` `app/(manage)/` および report 配下から `AdSlot` を import していない」
      (source scan で 0 件 assert)
    - 既存 3 つの guard block (client-vs-ssr / publish ux / cors) は変更しない
    - `npm run test -- harness-class-guards` PASS
  - 依存: P1-T3, P1-T4

- [ ] **P1-T6**: env サンプル + README 更新
  - 完了条件:
    - `frontend/.env.production.example` に `NEXT_PUBLIC_ADS_ENABLED` のコメント付き例追記
    - `frontend/.env.local.example` も同様に追記
    - `frontend/README.md` env 章に短く 1 段落 (flag OFF default / build 時 inline / 本番投入は PR42b)
  - 依存: P1-T2

- [ ] **P1-T7**: self-verification + PR closeout
  - 完了条件:
    - `.claude/skills/self-verification/SKILL.md` の checklist 全 PASS
    - `bash scripts/check-stale-comments.sh` 実行、ヒットを §3 分類で評価
    - `pr-closeout.md` §6 チェックリストを完了報告に含める
    - `npm run test` `npm run lint` `npm run typecheck` 全 PASS
    - Secret grep: `grep -rn 'TURNSTILE_SECRET\|DATABASE_URL\|sk_live\|amazon.*tag=' frontend/` 0 件
    - 「広告」label が DOM に出るのは flag ON のみ、flag OFF で `ad-slot-*` testid が無いことを再確認
  - 依存: P1-T2 〜 P1-T6 全完了

- [ ] **P1-T8**: commit + push + PR 作成
  - 完了条件:
    - 論理単位での commit（本計画書 / AdSlot 本体 / 配置 / guard test / env サンプル）
    - `git push -u origin claude/add-non-intrusive-ads-g1ySN`
    - PR 作成は **ユーザの明示的依頼があった場合のみ**（CLAUDE.md / pr-closeout 方針）
    - PR body は `pull-request-creation` skill のテンプレに沿う
  - 依存: P1-T7

#### Phase 1 の所要時間目安

| Task | 見積 |
|---|---|
| P1-T1 | 20 分 |
| P1-T2 | 60 分 |
| P1-T3 | 40 分 |
| P1-T4 | 30 分 |
| P1-T5 | 30 分 |
| P1-T6 | 15 分 |
| P1-T7 | 30 分 |
| P1-T8 | 15 分 |
| **合計** | 約 4 時間（Claude Code 単独作業時間） |

並列可能ペア: P1-T3 と P1-T4 (構造変更で疎)

---

### Phase 2: PR42b (本番投入)

ブランチ: 別名 (`feat/ads-amazon-affiliate-launch` 等。PR42a merge 後に切る)
前提条件: **PR38 (Public repo 化 / 履歴 secret scan) 完了後**に着手

#### Backlog

- [ ] **P2-T1 (ユーザ作業)**: Amazon アソシエイト・プログラム申請 + 承認待ち
  - 完了条件:
    - Amazon.co.jp アソシエイト ID 取得（`xxxxxx-22` 形式）
    - 承認メール受領
    - サイト URL `https://app.vrc-photobook.com` を登録
  - 依存: PR42a merge 済 (placeholder で実 URL レビュー可能になっている)
  - 注意: 申請から承認まで通常 1-3 営業日 + 初売上発生までの仮承認

- [ ] **P2-T2**: 掲載商品の選定 + 画像 URL / 商品 URL の決定
  - 完了条件:
    - 7 配置箇所それぞれに表示する商品 (or バナー) を選定（VR HMD / VR 関連書籍 等）
    - Amazon SiteStripe または商品ページから固定 img URL / 商品 URL を取得
    - 取得した URL に tracking ID `?tag=xxxxxx-22` が含まれることを確認
  - 依存: P2-T1
  - 注意: Amazon API は使わず固定 img 方式 (script 不要 / Cookie 不要)

- [ ] **P2-T3**: `AmazonAffiliateBanner.tsx` 専用 wrapper 実装
  - 完了条件:
    - `frontend/components/Ads/AmazonAffiliateBanner.tsx` 作成
    - props: `productUrl` / `imageUrl` / `imageAlt` / `width` / `height`
    - `<a href={productUrl} target="_blank" rel="noopener noreferrer sponsored">` + `<img>`
    - `rel="sponsored"` 必須 (検索エンジンへのシグナル)
    - 単体テスト追加 (URL に `tag=` が含まれること / rel 属性 / target / alt 必須)
  - 依存: P2-T1, P2-T2

- [ ] **P2-T4**: 7 配置箇所の AdSlot children に AmazonAffiliateBanner を差し込み
  - 完了条件:
    - PR42a で導入した AdSlot の children に AmazonAffiliateBanner を渡す
    - PageNavSidebar / RightPanel / ViewerLayout / 4 LP ページの該当箇所を編集
    - 既存テストが PASS、新規 placement-specific test も PASS
  - 依存: P2-T3

- [ ] **P2-T5**: Privacy ポリシー改訂
  - 完了条件:
    - `app/(public)/privacy/page.tsx` 改訂:
      - §第 4 条「第三者提供」: 広告配信用に閲覧情報を提供する旨の例外を 1 項追加
        （Amazon の場合は商品リンク click 時のみ Amazon が遷移先で情報取得する旨）
      - §第 5 条「利用する外部サービス」: chip に「Amazon アソシエイト」を追加
        (slug=`amazon-associates`, purpose=「広告（書籍・VR ガジェット等）」)
    - `app/(public)/privacy/page.tsx` の最終更新日を当日に更新
    - test 追加 (chip 6 個 / 新 chip slug が出る)
  - 依存: P2-T1

- [ ] **P2-T6**: Terms 改訂 + Amazon アソシエイト参加表記
  - 完了条件:
    - `app/(public)/terms/page.tsx` に広告掲載 1 段落を追加
      (運営費用に充当する目的で広告掲載することがある旨)
    - Amazon アソシエイト規約準拠の参加表記を `app/(public)/help/manage-url/page.tsx`
      の末尾 (Footer 直前) または `/about` 末尾に追加:
      「VRC PhotoBook は Amazon.co.jp アソシエイト・プログラムに参加しています。
       Amazon、Amazon.co.jp、およびそれらのロゴは Amazon.com, Inc. またはその関連会社の商標です。」
    - 最終更新日を当日に更新
  - 依存: P2-T1

- [ ] **P2-T7**: `.env.production` 更新 + Cloudflare Workers deploy
  - 完了条件:
    - `frontend/.env.production` (git ignore 済) に `NEXT_PUBLIC_ADS_ENABLED=true` を追記
      (ユーザ手作業、コミット対象外)
    - `cd frontend && npm run cf:build` で bundle に flag inline されることを確認
      (build 後の `.open-next/worker.js` に `"NEXT_PUBLIC_ADS_ENABLED":"true"` が含まれる）
    - `wrangler deploy` で deploy
    - 旧 Version ID を rollback target として記録
  - 依存: P2-T4, P2-T5, P2-T6
  - 注意: `predeploy-verification-checklist.md` §1〜§5 を全項目踏襲

- [ ] **P2-T8**: production bundle marker grep
  - 完了条件:
    - production の `_next/static/chunks/*.js` を curl で取得し marker grep:
      - 新 marker: `ad-slot-viewer-sidebar-left` / `ad-slot-viewer-sidebar-right`
        / `ad-slot-viewer-mobile` / `ad-slot-lp-*` が含まれる (各 placement で 1 件以上)
      - 旧 antipattern marker: なし
      - Amazon tag ID が含まれる (tracking ID `xxxxxx-22` を grep)
    - Cloud Run / Workers logs Secret grep 0 件
      （security-guard.md 禁止リストに `amazon.*tag=` を追加検査）
  - 依存: P2-T7

- [ ] **P2-T9**: Safari macOS / iPhone 実機確認
  - 完了条件:
    - macOS Safari (最新) で 7 配置箇所すべて表示確認
      - 「広告」label が見える
      - layout shift (CLS) が体感 / DevTools で発生していない
      - 広告 click → 別タブで Amazon 商品ページが開く
      - referrer-policy: strict-origin-when-cross-origin (Viewer / LP) で Amazon 側に正常に渡る
    - iPhone Safari (最新) で同上 + Mobile only ad-slot-viewer-mobile が見える
    - PageHero 間に広告が挟まらないことを目視確認
    - 既存 Cookie session (draft / manage) は影響なし (編集系には配置なし)
  - 依存: P2-T7

- [ ] **P2-T10**: post-deploy smoke (既存 routes regression + 新機能 direct verification)
  - 完了条件:
    - `predeploy-verification-checklist.md` §3 の既存 routes regression 全 PASS
    - `/p/<existing slug>` で 200 + AdSlot 3 箇所出力 (curl + HTML grep)
    - `/about` `/help/manage-url` `/terms` `/privacy` で 200 + AdSlot 1 箇所出力
    - 編集系 `/create` `/edit/<dummy>` `/manage/<dummy>` `/prepare/<dummy>` で
      AdSlot が出ない (source grep で `ad-slot` キーワード 0 件)
  - 依存: P2-T7

- [ ] **P2-T11**: closeout + failure-log / work-log
  - 完了条件:
    - PR42b の commit + push + PR 作成 (ユーザ依頼時)
    - `harness/work-logs/YYYY-MM-DD_pr42b-ads-launch-result.md` 作成
    - 観測した問題があれば `harness/failure-log/` 起票
    - `vrc-photobook-final-roadmap.md` の PR42b 行を「完了」に更新
    - `pr-closeout.md` §6 チェックリスト完了
  - 依存: P2-T8, P2-T9, P2-T10 全 PASS

#### Phase 2 の所要時間目安

| Task | 見積 |
|---|---|
| P2-T1 (ユーザ申請) | 1-3 営業日 |
| P2-T2 (ユーザ選定) | 1-2 時間 |
| P2-T3 | 45 分 |
| P2-T4 | 30 分 |
| P2-T5 | 60 分 |
| P2-T6 | 45 分 |
| P2-T7 (deploy) | 30 分 + routing 安定化 wait 10 分 |
| P2-T8 (bundle grep) | 30 分 |
| P2-T9 (Safari 実機) | 60 分 |
| P2-T10 (smoke) | 30 分 |
| P2-T11 (closeout) | 30 分 |
| **合計 (Claude Code 作業のみ)** | 約 6 時間 + ユーザ作業 |

並列可能: P2-T5 と P2-T6 (法務文面で独立)、P2-T3 と P2-T2 後半 (URL 取得とコンポーネント実装)

---

## 4. 依存関係マップ

```
[PR42a]
P1-T1 (計画書 commit)
  └─> P1-T2 (AdSlot 本体)
        ├─> P1-T3 (Viewer 配置)
        │     └─> P1-T5 (横断 guard)
        ├─> P1-T4 (LP 配置)
        │     └─> P1-T5
        └─> P1-T6 (env サンプル)
              └─> P1-T7 (self-verification)
                    └─> P1-T8 (commit/push/PR)

[PR42b] — 前提: PR42a merge + PR38 完了
P2-T1 (Amazon 申請)
  └─> P2-T2 (商品選定)
        └─> P2-T3 (Banner wrapper)
              └─> P2-T4 (差し込み)
                    └─> P2-T7 (deploy)
                          ├─> P2-T8 (bundle grep)
                          ├─> P2-T9 (Safari 実機)
                          └─> P2-T10 (smoke)
                                └─> P2-T11 (closeout)
P2-T5 (Privacy) ─┐
P2-T6 (Terms)  ─┴─> P2-T7
```

---

## 5. リスク / 注意点

### 5.1 構造リスク

| リスク | 対策 |
|---|---|
| Viewer の 3 col grid が破綻 (160px sidebar が窮屈) | PageNavSidebar 200px 列に 160 幅は収まる確認済。AdSlot は `width={160}` 固定 |
| Mobile で 300×250 が画面幅を超える | 320px 幅でも 300×250 は収まる。`max-width: 100%` 保険を AdSlot に入れる |
| CLS 悪化 (Lighthouse score 低下) | `min-height` 必須、test で sticky/fixed 禁止 |

### 5.2 ポリシー / 法務リスク

| リスク | 対策 |
|---|---|
| 景表法ステマ規制違反 | 「広告」label 必須 (aria-label + 視覚表記)、PR42a 段階で実装済 |
| Amazon アソシエイト規約違反 (参加表記なし) | PR42b で `/help` or `/about` に必須表記 |
| Privacy §4「第三者提供しない」と矛盾 | PR42b で §4 §5 改訂、改訂前に flag ON しない |
| Public repo 化前に Amazon tracking ID が public history に残る | PR38 完了後に PR42b 着手、または `.env.production` を git ignore 維持 |

### 5.3 運用リスク

| リスク | 対策 |
|---|---|
| flag ON のまま PR42a を本番投入してしまう | `wrangler.jsonc` に flag を書かない / `.env.production.example` で「未設定 = OFF」を強調 |
| 編集系画面に誤って AdSlot が混入 | `harness-class-guards.test.ts` で source scan 自動検出 |
| Mobile で Cookie session に影響 | AdSlot は `"use client"` 不要 / fetch も発火しない / Cookie 関与なし |

---

## 6. 関連ドキュメント

- `docs/plan/vrc-photobook-final-roadmap.md` (PR 番号体系の正典)
- `docs/spec/vrc_photobook_business_knowledge_v4.md` (スマホファースト / 個人運営)
- `.agents/rules/safari-verification.md` (Safari 必須確認)
- `.agents/rules/predeploy-verification-checklist.md` (deploy 完了基準)
- `.agents/rules/pr-closeout.md` (PR 完了処理)
- `.agents/rules/security-guard.md` (Secret / Cookie / 認可)
- `.agents/rules/testing.md` (テーブル駆動 + Builder)
- `.agents/rules/coding-rules.md` (明示的 > 暗黙的)
- 景表法 ステマ規制 (2023-10-01 施行) / JIAA インターネット広告倫理綱領
- Amazon.co.jp アソシエイト・プログラム運営規約 (参加表記必須)

---

## 7. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-11 | 初版作成。PR42a (placeholder) / PR42b (本番投入) の 2 段分割で立案 |
