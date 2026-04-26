# M2 早期 ドメイン候補調査結果

> M2 早期 §F-1（独自ドメイン + U2 Cookie Domain 解消）における**ドメイン候補の事前調査記録**。本書は購入判断の根拠であり、購入手順ではない。
>
> **作成日**: 2026-04-26
>
> **位置付け**: [`docs/plan/m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md) §3〜§4 / §13 #1 の判断材料として、調査結果を独立記録する。
>
> **上流**:
> - [`docs/plan/m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md) §3 候補 / §4 取得元
> - [`harness/work-logs/2026-04-26_project-roadmap-overview.md`](../../harness/work-logs/2026-04-26_project-roadmap-overview.md) §F-1
>
> **重要前提**:
> - **本書の段階ではドメインを購入しない**（DNS 一次照会 + 公開情報による調査のみ）
> - 取得可否は購入時に **Cloudflare Dashboard `domains.cloudflare.com` で必ず最終確認**
> - VRChat 関連の名称使用は**「公式と誤認させない運用」が必須**

---

## 1. 調査方法と限界

### 1.1 実施した調査
- DNS 一次照会（`dig NS / A`）で各候補の `NS` / `A` レコード有無を確認
- WebSearch で各候補の既存サービス重複 / VRChat 商標ガイドライン
- WebFetch で Cloudflare Registrar 2026 価格

### 1.2 実施できなかった / 限界
- **正式な WHOIS / RDAP 照会**: WSL 環境に `whois` 不在、`rdap.org` は 403 でブロック → DNS 一次照会と Web 検索で代替
- **取得可否の最終確定**: 上記により、購入時に Cloudflare Dashboard で実検索する必要がある（DNS なし=未登録の可能性高、ただし「登録済だが未公開」のケースもある）
- **過去所有歴**: Wayback Machine / WHOIS 履歴は購入時にユーザー側で実施

---

## 2. 候補別一覧

> **取得可否**: DNS 上 `NS` / `A` レコード無し → 未登録の可能性が高い、ただし**購入時に Cloudflare Dashboard で最終確認必須**。
> **価格**: Cloudflare Registrar 2026 at-cost（first-year / renewal 同額）

| 候補 | 取得見込み | 年額 | 既存重複 | 商標/名称リスク | ブランド性 | 技術適性 | 総合 |
|---|---|---|---|---|---|---|---|
| **vrcphotobook.com** | 未登録の可能性高 | **$10.46** | 検索ヒットなし | 低（VRChat 公式と誤認させない運用なら可）| 用途明確、`.com` 信頼性高 | 全要件適合 | **★★★★★** |
| **vrcphotobook.app** | 同上 | $14.20 | 検索ヒットなし | 低 | 用途明確 + HSTS preload で HTTPS 強制（Secure Cookie と相性◎）| 全要件適合 + HSTS 強制 | **★★★★** |
| **vrcphotobook.net** | 同上 | $11.86 | 検索ヒットなし | 低 | `.com` より信頼度低い印象 | 全要件適合 | ★★★ |
| **vrcpb.app** | 同上 | $14.20 | VRCX / VRC Pic 等の既存サードパーティと並ぶ感 | 中（略称で機能伝わりにくい）| 短い・打ちやすい / 意味伝わらない | 適合 | ★★ |
| **vrcpb.net** | 同上 | $11.86 | 同上 | 中 | 略称 + `.net`、優位性少 | 適合 | ★ |
| **vrc-photobook.com** | 同上 | $10.46 | 検索ヒットなし | 低 | ハイフン入りは口頭で伝えづらい / SEO 不利 | 適合 | ★ |
| **vrcphotobook.io** | 同上 | $30〜$50 高め | なし | 中（`.io` は Chagos 諸島移管問題で TLD 自体の将来不透明） | 開発者寄り | 適合 | ★ 避ける |

### 2.1 既存 VRChat 界隈サービスの重複確認

| サービス | URL | 用途 | 本サービスとの関係 |
|---|---|---|---|
| VRC Pic | `vrcpic.com` | Social VR Photo Sharing Platform（リアルタイム共有 / SNS 系）| 用途近接、ジャンル混同リスクあり（ブランド名は別） |
| VRCX | `vrcx.org` | VRChat 用デスクトップ companion アプリ（友達管理）| 用途違い、影響少 |
| VRC List | `vrclist.com` | VRChat ワールド検索 | 用途違い |
| VRC World / VRC Pro | — | RC Racing 関連、VRChat と無関係 | 用途違い |

---

## 3. 第一候補：`vrcphotobook.com`

- **理由**:
  - 用途（VRChat フォトブックサービス）が一目で伝わる
  - `.com` の信頼性 / 普遍性 / 検索結果での優位
  - **Cloudflare Registrar at-cost で年額 $10.46**（候補中最安、Cloudflare DNS / Workers / R2 すべて統合済み）
  - **既存サービスと重複なし**（Web 検索でヒットなし）
  - 商標観点で VRChat 公式と誤認させるリスクは低い（一般語 + サービス用途を表す名称）

- **懸念**:
  - 14 文字 + `.com` でやや長い
  - **VRC Pic（vrcpic.com）が VRChat 写真共有ジャンルで実在**するため、ユーザーがブランドを混同する可能性 → LP / About で**フォトブック（縦スクロールまとめ型）であることを明示**して差別化

- **購入前チェック**（§6 で詳述）:
  - Cloudflare Dashboard で実取得可否
  - WHOIS で前所有歴
  - Wayback Machine で過去履歴
  - VRChat Trademark Guidelines 最終確認

---

## 4. 第二候補：`vrcphotobook.app`

- **理由**:
  - 第一候補と同じく用途明確
  - **`.app` は HSTS preload 強制**（ブラウザが HTTPS を必須化、Secure Cookie の前提と完全に整合）→ ADR-0003 §決定の `Secure: true` Cookie 方針が**ドメイン側でも担保**される
  - 開発者向け新 TLD として技術ブランドが立つ

- **懸念**:
  - 年額 $14.20（`.com` 比 +$3.74、3 年で +$11、誤差レベル）
  - `.app` は `.com` より一般認知度が低い（古めの世代には伝わりにくい）
  - HSTS 強制のため、誤って HTTP-only の検証用ホストを立てると即座に動かない（事故防止としてはむしろ有利）

- **購入前チェック**: 第一候補と同じ + `.app` 特有の HSTS 強制を運用ルールに織り込む（既に ADR-0001 §決定で HTTPS 必須なので追加対応不要）

---

## 5. 避ける候補

| 候補 | 避ける理由 |
|---|---|
| `vrcpb.app` / `vrcpb.net` | 略称のため意味伝達が弱い。**VRCX / VRC Pic / VRC List / VRC Pro / VRC World** 等、VRChat 界隈に「VRC + 短縮」のサードパーティ / 商標が複数存在し、新規 `vrcpb` は埋もれる。SNS で `vrcpb` と言われても何のサービスか想起されない |
| `vrc-photobook.com` | ハイフン入りは口頭伝達不利、SEO でハイフン無し版に検索流入を取られる |
| `vrcphotobook.io` | `.io` は Chagos 諸島の主権問題で **将来的に廃止可能性**が指摘されている TLD。価格も高め。MVP の長期運用に向かない |

---

## 6. 購入前チェック（ユーザー側で実行）

| # | 項目 | やること |
|---|---|---|
| 1 | **Cloudflare Registrar で取得可否**| `https://domains.cloudflare.com/` で `vrcphotobook.com` を検索、価格 / 取得可否を最終確認 |
| 2 | **WHOIS 確認** | `https://whois.domaintools.com/vrcphotobook.com` 等で前所有歴・登録状況 |
| 3 | **Wayback Machine 確認** | `https://web.archive.org/web/*/vrcphotobook.com` で過去にどんな Web サイトとして使われていたか確認（スパム / 不正利用履歴の回避）|
| 4 | **VRChat Trademark Guidelines 最終確認** | `https://hello.vrchat.com/legal` / `https://wiki.vrchat.com/wiki/Legal_&_Guidelines` を再読、本サービスの「非公式ファンメイド」運用と整合 |
| 5 | **既存サービス重複再確認** | Google / X / Discord で `vrcphotobook` を再検索、ローンチ後にぶつかるサービスがないか |
| 6 | **商標リスク確認** | Justia 等の商標 DB で `VRCHAT` / `VRC PHOTOBOOK` を検索（VRChat Inc. の保有商標のみ。本ドメイン名そのものは商標登録されていないと推定）|

---

## 7. 非公式表記の必要性（VRChat 商標観点）

VRChat 商標は「公式と誤認させない範囲」での使用が許可される（VRChat Terms of Service / Brand Guidelines）。本サービスは VRChat 公式ではないため、以下に明記する運用が必須:

| 場所 | 推奨表記（例）|
|---|---|
| **LP（ファーストビュー or footer）**| 「本サービスは VRChat 公式とは関係のないファンメイドサービスです。"VRChat" は VRChat Inc. の商標です。」|
| **footer（全ページ）**| 同上の短縮版「VRChat 公式ではありません」|
| **利用規約**| 「本サービスは VRChat Inc. が提供するサービスではなく、運営者個人が独立して提供するファンメイドサービスです。VRChat の名称・ロゴ・キャラクターに関する権利は VRChat Inc. に帰属します。」|
| **About ページ / OGP description**| 「VRChat の写真をフォトブック形式でまとめる**非公式ファンメイド**サービス」|
| **管理 URL ページ**| 不要（管理者しか見ない、noindex）|

これらは v4 §7.1 利用規約 / §7.2 プライバシーポリシーの **MVP 必須記載項目に追記**する形で対応。M3 / M4 で利用規約・プライバシーポリシーを書く際にあわせて反映する。

---

## 8. 推奨結論

| 項目 | 結論 |
|---|---|
| **第一候補** | **`vrcphotobook.com`** |
| **第二候補** | **`vrcphotobook.app`** |
| **取得元** | **Cloudflare Registrar**（at-cost、Cloudflare DNS / Workers との統合最短）|
| **購入のタイミング** | **本書の段階では購入しない**。M2 本実装骨格が固まり、Cookie Domain / URL 設計 / SendGrid 送信ドメイン / Turnstile 本番 widget hostname の利用タイミングが近づいた段階で購入する（§9）|
| **購入前にユーザーが行うこと** | §6 のチェック 6 項目（Cloudflare Dashboard / WHOIS / Wayback / 商標 / 既存サービス / 商標 DB）|

---

## 9. 第一候補確定の合意と購入延期方針（2026-04-26）

> **2026-04-26 後段の更新**: 実購入は **`vrc-photobook.com`（ハイフン入り）** で確定（§9.5）。
> §1 / §3 の `vrcphotobook.com`（ハイフン無し）の比較分析は当時の検討経緯として残すが、
> 以後の実装・運用上の正は `vrc-photobook.com`。

### 9.1 合意事項（当時）

- **第一候補は `vrcphotobook.com` に確定**（ユーザー判断、2026-04-26 前段）
- 第二候補 `vrcphotobook.app` はバックアップとして保持

### 9.2 購入延期の方針

ユーザー判断により、**ドメイン購入は延期**する:

- **延期する理由**:
  1. まずローカル / 本実装の骨格をもう少し固める
  2. Cookie Domain / URL 設計 / SendGrid 送信ドメイン / Turnstile 本番 widget hostname の利用タイミングが近づいた段階で購入する
  3. 今は実リソース操作を増やさない

### 9.3 購入解禁の判断基準（参考）

以下のいずれかが近づいたタイミングで、本書を再度参照して購入手続きへ進む:

- M2 本実装の `frontend/` / `backend/` 骨格が確定（§F-4 優先度 D の最初の数コミット）
- Cloud SQL Step B（§F-2 優先度 B）の実機検証で **U2 解消が必要**になったタイミング
- SendGrid 実送信 PoC（§F-3 優先度 C）で **送信ドメイン認証**が必要になったタイミング
- Turnstile 本番 widget 発行で hostname 確定が必要になったタイミング

### 9.4 延期中の暫定運用（〜 2026-04-26 後段の購入まで）

- Frontend Workers: `https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev` で運用
- Backend Cloud Run: `https://vrcpb-spike-api-7eosr3jcfa-an.a.run.app` で運用
- U2 別オリジン Cookie 不通の状態は **想定通り** として継続
- 24h / 7 日後 Safari ITP 観察は継続（起点 2026-04-26）

### 9.5 購入確定: `vrc-photobook.com`（2026-04-26 後段）

- **実購入したドメインは `vrc-photobook.com`（ハイフン入り）**
- Cloudflare Registrar 経由、年額 $10.46 相当、自動更新 ON、WHOIS Privacy 有効を外部 RDAP で確認済
- 当初の第一候補 `vrcphotobook.com`（ハイフン無し）ではなく、ハイフン入りで購入が確定したため、
  以後は **`vrc-photobook.com` を正** とする
- ハイフン入りの懸念（口頭共有のしにくさ / 入力性 / SEO 微影響）は許容する判断
- キャンセル / 名義変更は実質不可（ICANN 規則 + Cloudflare at-cost 価格）
- 第二候補 `vrcphotobook.app` は今回使わない

---

## 10. 次にユーザーが判断すべきこと（更新版）

`vrc-photobook.com` 購入後（2026-04-26）の次ステップ:

1. **DNS 設定** — Cloudflare で `app` / `api` レコードを追加（CNAME、Proxy 設定は API 側 OFF が推奨）
2. **Workers Custom Domain** — `app.vrc-photobook.com` を `vrcpb-frontend` Worker に紐付け
3. **Backend API ドメイン方針** — 案 A（`api.vrc-photobook.com` を Cloud Run Domain Mapping 直結）を確定（[`m2-domain-purchase-checklist.md`](./m2-domain-purchase-checklist.md) §4.2 推奨どおり）
4. **環境変数の確定** — `NEXT_PUBLIC_BASE_URL=https://app.vrc-photobook.com` /
   `NEXT_PUBLIC_API_BASE_URL=https://api.vrc-photobook.com` /
   `COOKIE_DOMAIN=.vrc-photobook.com` /
   Backend `ALLOWED_ORIGINS=https://app.vrc-photobook.com`
5. **本番 deploy + Safari / iPhone Safari 実機確認**（[`m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md) §8 切替手順 + [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)）

---

## 11. 関連ドキュメント

- [`docs/plan/m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md)（U2 Cookie Domain 解消の全体計画）
- [`harness/work-logs/2026-04-26_project-roadmap-overview.md`](../../harness/work-logs/2026-04-26_project-roadmap-overview.md) §F-1
- [`docs/adr/0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md) §13 U2

## 12. 参照した外部情報源

- [VRChat Terms of Service](https://hello.vrchat.com/legal)
- [VRChat Copyright](https://hello.vrchat.com/copyright)
- [Legal & Guidelines - VRChat Wiki](https://wiki.vrchat.com/wiki/Legal_&_Guidelines)
- [Cloudflare Registrar | Domain Registration & Renewal](https://www.cloudflare.com/products/registrar/)
- [Cloudflare Domain Pricing – Registration & Renewal Costs for All TLDs](https://cfdomainpricing.com/)
- [Search and register available domain names | Cloudflare Registrar](https://domains.cloudflare.com/)
- [Google announced HSTS as default for all .app domains](https://comodosslstore.com/blog/google-launches-app-top-level-domain-with-hsts-as-a-default.html)
- [HSTS Preload List Submission](https://hstspreload.org/)
- [VRC Pic - Social VR Photo Sharing Platform](https://vrcpic.com/)
- [VRCX – Powerful VRChat desktop companion app](https://vrcx.org/)

## 13. 履歴

| 日付 | 変更 |
|---|---|
| 2026-04-26 | 初版作成。7 候補を比較、第一候補 `vrcphotobook.com` / 第二候補 `vrcphotobook.app` / 取得元 Cloudflare Registrar を推奨。**ユーザー合意により第一候補 `vrcphotobook.com` を確定**。購入は M2 本実装骨格確定後に**延期**（§9） |
| 2026-04-26（後段） | M2 本実装の PR9 / PR10 / PR10.5 完了後、ユーザーが Cloudflare Registrar で実購入。**実購入は `vrc-photobook.com`（ハイフン入り）** で確定（§9.5）。WHOIS Privacy は外部 RDAP（Verisign / Cloudflare）で REDACTED が確認済み。§1 / §3 の `vrcphotobook.com` 比較分析は履歴として保存、§10 を実購入ドメインに合わせて更新 |
