# 失敗: PR37 public pages の視覚デザインが user 意図と大きく乖離

> 発生日: 2026-05-01（UTC 2026-04-30〜2026-05-01）
> 関連: PR37 STOP β 実装 (`5d85af5 feat(frontend): add LP, terms, privacy, about pages`) → STOP ε 実機確認で user から「design に盛大に違う」所見
> ルール化対象: 本 failure-log を起点に、デザインを伴う PR の STOP α に「見た目方針の承認」を含める運用に改める
> 対応 work-log: [`harness/work-logs/2026-05-01_pr37-public-pages-result.md`](../work-logs/2026-05-01_pr37-public-pages-result.md)

## 1. 発生状況

- PR37 は LP / Terms / Privacy / About の公開ページ整備
- STOP α で `design/README.md` / `design/mockups/README.md` / `design/design-system/{colors,typography,spacing,radius-shadow}.md` / `design/mockups/prototype/{styles.css,shared.jsx,pc-shared.jsx,screens-a.jsx,screens-b.jsx,pc-screens-a.jsx,pc-screens-b.jsx}` を読込必須として user 承認
- STOP β で実装、Tailwind token は `brand-teal` / `ink` / `surface` / `divider` / `status` のみ使用、`text-h1` / `text-h2` / `text-body` / `text-sm` / `text-xs` のみ、`rounded-lg` + `shadow-sm`、装飾的 gradient 不使用
- typecheck / vitest 139 / next build / cf:build / cf:check 全 PASS
- STOP δ で Workers redeploy（version `6f1e82d7-...`）、本番反映
- STOP ε で macOS Safari + iPhone Safari による実機確認
- → **user 所見: 「design に盛大に違う」「design ファイルを読んだのに視覚イメージが意図と大きく乖離」**

## 2. 失敗内容

機能・安全面（HTTP 200 / `X-Robots-Tag: noindex, nofollow` / `<meta name="robots">` / `Referrer-Policy` / metadata title 上書き / 既存経路 regression / Secret + raw 値漏えい 0 件）はすべて要件を満たしたが、**視覚的なデザイン品質が user 意図と大きく乖離**した。

具体的なギャップ箇所は本書では特定せず（user 所見が「盛大に違う」総評であり、後続 design rebuild PR で個別洗い出し）、**プロセス上の失敗**として扱う。

## 3. 根本原因（root cause）

複合要因:

1. **design ファイル読込を実装前承認に変換できていなかった**
   - design-system 4 ファイル（colors / typography / spacing / radius-shadow）は token 仕様の記載で、画面の構成・温度感・密度・要素配置を直接的には規定しない
   - prototype 画面群（`screens-a.jsx` / `pc-screens-a.jsx` 等）は LP 風 hero + features grid のモックを含むが、本 PR で採用する画面・どのレベルで踏襲するかが未確定だった
   - design ファイルから「視覚的な完成イメージ」を抽出するのは実装者の解釈に依存し、user 期待との照合が **実装後の Safari 実機確認で初めて行われた**
2. **画面ごとのワイヤーフレーム / 視覚方針を実装前にユーザー承認していなかった**
   - STOP α で「ページごとの内容案」（テキスト要素・セクション構成）は承認したが、**視覚的なレイアウト・密度・カードの大きさ感・hero の強さ・余白感** を明文化して承認していなかった
   - 結果として「内容は合っている / token は守っている / でも見た目は意図と違う」状態が発生
3. **prototype / concept image のどの画面を採用するかを明確化しなかった**
   - `design/mockups/concept-images/` 15 枚のコンセプト画像 / `prototype/screens-a.jsx` LP モックが存在するが、本 PR で参照すべき採用画面は STOP α で決定していなかった
   - 「LP は実際のサービス入口として使える画面にする」というディレクションは text で受領していたが、視覚的な temperature を user と擦り合わせていなかった

## 4. 影響範囲

- 本番 Workers `6f1e82d7-...` で公開済の 4 ページ + Help が、**機能としては動くが見た目としては user 意図と乖離**
- 4 ページ（LP / Terms / Privacy / About）と Viewer footer / Help footer の関連リンクが対象
- 既存ページ（`/p/<slug>` / `/manage/<id>` / `/p/<slug>/report` / Edit / Draft）には **影響なし**（ただし Viewer footer のリンク追加部分は影響範囲）
- Backend / Cloud Run / Cloud SQL / Secret / migration / Job / Workers binding には影響なし

## 5. 対策（recurrence prevention）

### 5.1 デザインを伴う PR の STOP α に「見た目方針の承認」を含める

新規ルール（本 failure-log を起点に運用化）:

- design を伴う PR では、**STOP α で計画書の承認に加えて「画面別ワイヤーフレームまたはスクリーン構成案」を提示し、ユーザー承認を取る**
- 提示形式は以下のいずれかを最低限カバー:
  - 既存 prototype 画面（`design/mockups/prototype/screens-*.jsx` / `pc-screens-*.jsx`）のうち採用するものを **画面 ID 単位で明示**
  - 採用しない場合は、ページごとのセクション順 / 密度 / カード or 平文 / hero の有無 / アクセントカラーの使い方を **箇条書きで明文化**
  - concept image を採用する場合は `concept-NN.png` 番号を明示
- design-system 準拠だけでは「同じ token を使った別の見た目」が無数に作れるため、**token 準拠 + 採用画面の明示**を STOP α 必須項目とする

### 5.2 design rebuild は別 PR で実施

- 本 PR は機能・安全面で完了扱い（roll forward しない、roll back しない）
- 視覚デザインは roadmap §1.3 後続候補「PR37 public pages design rebuild」として別 PR で実施
- design rebuild PR は本 failure-log §5.1 のルールに従って STOP α で見た目方針承認を取る

### 5.3 design rebuild PR の STOP α 要件（参考、運用ルール化）

design rebuild PR を起こす場合、最低限以下を STOP α に含める:

- 採用する prototype 画面（`screens-a.jsx` の LP / `pc-screens-a.jsx` 等）の ID
- LP の hero（強い hero 画像 / mock book / コピーのみ）の方針
- 各ページの密度（`<p>` 中心 / カード中心 / グリッド有無）
- アクセント色（teal の使い方、violet 不使用 等）
- モバイル / PC のレイアウト切り替えポイント
- 採用しない要素（gradient orb / bokeh 等の禁止事項の再確認）
- 既存ページ（Viewer / Edit / Manage / Help）との温度感整合の方針

## 6. 対策種別

- [x] **ルール化**: デザインを伴う PR の STOP α に「見た目方針の承認」を必須化（本 failure-log §5.1 + §5.3）
- [x] **failure-log 起票**（本書）
- [x] **roadmap 後続候補追加**: PR37 public pages design rebuild
- [ ] スキル化（不要、運用ルールで足りる）
- [ ] テスト追加（不要、視覚品質はテストでは担保不能）
- [ ] フック追加（不要、人間判断項目）

## 7. 「絶対に出さないもの」遵守

本書には以下を含まない:
- raw slug / raw photobook_id / raw report_id / raw public URL
- token / Cookie / DATABASE_URL / Secret 値
- source_ip_hash 完全値 / scope_hash 完全値
- reporter_contact 実値 / detail 実値

仮置き値（運営者表示名 `ERENOA` / 連絡用 X `@Noa_Fortevita` / 準拠法）は user 承認済の **公開対象値**で機密ではないが、本 failure-log では参照のみで再掲不要のため省略。

## 8. 関連

- [`harness/work-logs/2026-05-01_pr37-public-pages-result.md`](../work-logs/2026-05-01_pr37-public-pages-result.md) §4.3 / §8
- [`docs/plan/vrc-photobook-final-roadmap.md`](../../docs/plan/vrc-photobook-final-roadmap.md) §1.3 後続候補（PR37 design rebuild）
- [`design/README.md`](../../design/README.md) / [`design/mockups/README.md`](../../design/mockups/README.md)
- [`design/design-system/`](../../design/design-system/) 4 ファイル
- [`design/mockups/prototype/`](../../design/mockups/prototype/) prototype 画面群
- [`.agents/rules/feedback-loop.md`](../../.agents/rules/feedback-loop.md)（失敗 → ルール化の運用原則）
- [`.agents/rules/pr-closeout.md`](../../.agents/rules/pr-closeout.md)
