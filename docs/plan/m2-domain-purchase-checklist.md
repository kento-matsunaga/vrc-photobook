# M2 ドメイン購入前チェックリスト（`vrcphotobook.com`）

> 作成日: 2026-04-26
> 位置付け: `vrcphotobook.com` 購入の **直前** に確認するチェック表。本書の作成時点では **まだ購入しない**。
>
> 上流参照（必読、本書では再記載しない）:
> - [`docs/plan/m2-domain-candidate-research.md`](./m2-domain-candidate-research.md)（候補比較・推奨理由・WHOIS / Wayback 確認手順、§5）
> - [`docs/plan/m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md)（DNS 構成案・Cookie Domain・切替手順、§7-8）
> - [`docs/plan/m2-implementation-bootstrap-plan.md`](./m2-implementation-bootstrap-plan.md) §10 ユーザー判断事項 #6 / §12
> - [`docs/plan/m2-photobook-session-integration-plan.md`](./m2-photobook-session-integration-plan.md) §14.10
> - [`docs/adr/0001-tech-stack.md`](../adr/0001-tech-stack.md) / [`docs/adr/0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md)
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
> - [`harness/work-logs/2026-04-26_project-roadmap-overview.md`](../../harness/work-logs/2026-04-26_project-roadmap-overview.md)

---

## 0. 本チェック表の使い方

- 購入は **ユーザーが Cloudflare Dashboard 上で手動実施**する。Claude Code は本チェック表 / 後続の DNS / Custom Domain 計画 / コマンド整備を担当する。
- §1〜§3 を購入直前に上から順に確認、§4 で API ドメイン方針を最終確定、§5 で購入後の作業順序を確認、§7 のユーザー判断項目に答えてから購入ボタンを押す。
- 購入操作は決済確定の前で 1 回 Pause し、§1〜§3 のチェック項目すべてに ✅ が付いてから確定する。

---

## 1. 購入前確認（Cloudflare Registrar 上で 1 つずつ確認）

### 1.1 取得可否・価格

- [ ] Cloudflare Dashboard `Domain Registration → Register domains` で `vrcphotobook.com` を検索
- [ ] 「取得可能」表示が出ること
- [ ] 年額が **$10.46 前後**であること（at-cost 価格、`m2-domain-candidate-research.md` §3 と一致）
- [ ] 為替で日本円表示が出る場合、概算 **¥1,500〜¥1,800/年** 程度であることを確認
- [ ] 取れない場合のフォールバック: 第二候補 `vrcphotobook.app`（年額 $14.20）→ §7 でユーザー判断

### 1.2 WHOIS / 過去履歴

- [ ] WHOIS で前所有歴がない / クリーン（既に `m2-domain-candidate-research.md` §5 #2 で「未登録の可能性高」と確認、購入直前に再確認）
- [ ] [Wayback Machine](https://web.archive.org/web/*/vrcphotobook.com) で過去にスパム / 不正サイトとして使われていないこと
- [ ] Google Safe Browsing 等で blacklist 登録歴がないこと

### 1.3 WHOIS Privacy / Registrant 情報

- [ ] **WHOIS Privacy が有効**（Cloudflare Registrar はデフォルト ON、ICANN への登録者情報も Cloudflare の代理として登録される）
- [ ] Registrant 情報に **本名 / 自宅住所が WHOIS 公開されない**ことを確認
- [ ] Cloudflare アカウント自体の認証メールが gmail / 個人メール等で長期維持できること
- [ ] 失念時のリカバリ経路（バックアップメール / 2FA 復旧コード）を保存

### 1.4 自動更新・支払い

- [ ] **自動更新を ON** にする（推奨。M2 / M3 段階で更新忘れによる失効を避ける）
- [ ] 支払いカードの有効期限が次年度に届くこと
- [ ] Cloudflare アカウント上の Billing 設定でメール通知 ON（更新失敗時に気付ける）

### 1.5 ドメイン名自体のセルフチェック

- [ ] 個人情報 / 本名 / 居住地が **ドメイン名に含まれていない**こと（`vrcphotobook.com` は OK）
- [ ] 商標や他社サービス名と被っていないこと（§2 で詳細チェック）
- [ ] スペルミス・タイポでないこと
- [ ] 一度購入するとキャンセル / 名前変更が事実上不可能（同名取り直しは年額再支払い）であることを認識

---

## 2. 名称・商標・非公式表記

### 2.1 VRChat 公式との誤認回避

- [ ] **VRChat 公式と誤認されない運用**を確約する（`m2-domain-candidate-research.md` §6 のリスク評価）
- [ ] LP / footer / 利用規約 / About に **非公式表記**を必ず入れる（§2.2 文言案）
- [ ] OGP / メタタグの description に「非公式 / fan-made」と明記
- [ ] VRChat 公式ロゴ / トレードドレスを一切使わない

### 2.2 非公式表記 文言案

**英語版（OGP / 利用規約 footer など）**:

> "VRC PhotoBook is an independent fan-made tool and is not affiliated with, endorsed by, or sponsored by VRChat Inc."

**日本語版（LP / 利用規約 / About）**:

> 「VRC PhotoBook は VRChat 公式（VRChat Inc.）とは一切関係のない、独立した非公式ファンメイドツールです。」

**短縮版（footer / OGP description）**:

> "Unofficial fan-made tool. Not affiliated with VRChat Inc."
> 「非公式・ファンメイド。VRChat Inc. とは無関係です。」

### 2.3 「VRC」略称の扱い

- VRChat コミュニティで `VRC` は事実上の略称として広く使われており、**それ単体での商標的問題はないと判断**（独自確認、訴訟リスクゼロは保証できないが、運用上の慣行として広く許容されている）
- 万が一 VRChat Inc. から DMCA / 名称変更要請が来た場合の対応:
  - ドメインを別名（例: `photobookforvr.com` 等）に切り替え可能なよう、コード内のドメイン参照を環境変数に集約済（`COOKIE_DOMAIN` / `NEXT_PUBLIC_BASE_URL` / `NEXT_PUBLIC_API_BASE_URL` / `ALLOWED_ORIGINS`）
- リスク発生時は本書を更新し、`harness/failure-log/` に経緯を記録する

### 2.4 公開時の文書整備チェック

購入後・公開前に必ず:

- [ ] LP（`/`）の footer に非公式表記
- [ ] 利用規約 ToS に「VRChat Inc. と無関係」を明記
- [ ] About / Privacy ページに同様の表記
- [ ] OGP メタタグの `og:site_name` / `description` に非公式キーワード

---

## 3. 購入後の構成案（既定、`m2-early-domain-and-cookie-plan.md` §7 案 A 採用）

```
app.vrcphotobook.com  → Frontend Workers（OpenNext for Cloudflare）
api.vrcphotobook.com  → Backend Cloud Run（Domain Mapping、§4 で再確認）
Cookie Domain         → .vrcphotobook.com（共通親ドメイン、Set-Cookie の Domain 属性）
```

メリット:
- `.vrcphotobook.com` Cookie が `app.*` / `api.*` 両方に届く（U2 解消、ADR-0003）
- DNS 構成がシンプル（CNAME 2 本）
- Backend を curl で直接叩けるためデバッグ容易
- Safari ITP の First-party Cookie 扱い（共通親ドメイン）

採用済の根拠: `m2-early-domain-and-cookie-plan.md` §7 / §10、本書では再採用を確認するのみ。

---

## 4. Backend API ドメイン方針（最終確認）

### 4.1 比較

| 観点 | 案 A: `api.<domain>` Cloud Run 直結 | 案 B: `app.<domain>/api/*` Workers proxy | 案 C: `api.<domain>` も Workers Custom Domain（Workers 経由で Cloud Run へ proxy） |
|---|---|---|---|
| Cookie Domain | `.<domain>` で両ホストに渡る | 同一ホスト（`app.<domain>`）、Cookie Domain 不要 | `.<domain>` で両ホストに渡る |
| Safari ITP | First-party Cookie（共通親ドメイン）で安定 | 同一ホストで最も安定 | 案 A と同等 |
| CORS | `ALLOWED_ORIGINS=https://app.<domain>` を Backend に追加で済む | 不要（同一オリジン） | 案 A と同等 |
| Cloud Run 制約 | Cloud Run Domain Mapping は **一部リージョンで GA、asia-northeast1 は対応**（[公式: Mapping custom domains](https://docs.cloud.google.com/run/docs/mapping-custom-domains)）| Cloud Run の前段に Workers が立つため、リージョン制約を回避できる | 案 B と同様、Cloud Run domain mapping を使わない |
| デバッグ容易性 | `curl https://api.<domain>/health` で直接叩ける | Workers 経由のため、Workers ログも併読が必要 | 案 B と同等 |
| レイテンシ | DNS → Cloud Run の最短経路 | Workers → Cloud Run の 1 hop 増 | 同上 |
| Workers 利用料 | Workers は frontend のみ（無料枠で当面足りる） | Workers が API も処理、計算量増（無料枠は 100k req/日、超過は $5/10M req） | 同上 |
| 将来の運用 | Cloud Run 制約（同時実行・タイムアウト 60min 等）が直接効く | Workers の 30 秒タイムアウト / CPU 制約が乗る（API では大きな制約） | 同上 |
| 設定の複雑さ | `app` `api` の 2 系統管理 | 1 系統で済む | `app` `api` 両方 Workers、proxy 配線が増える |

### 4.2 推奨：**案 A（`api.<domain>` Cloud Run Domain Mapping 直結）**

採用根拠:

- `m2-early-domain-and-cookie-plan.md` §10 で **案 A を推奨確定済**
- Cloud Run の Domain Mapping は asia-northeast1 で対応（公式ドキュメント）
- API のレスポンスタイム / タイムアウトを Workers の制約に縛られない
- curl デバッグの容易性（M2 段階の手動検証で重要）
- Cookie 共有は `.<domain>` で問題なく成立（M1 PoC で別オリジン NG を確認済）

### 4.3 案 A が困難になった場合のフォールバック手順

以下のいずれかが起きた場合、案 C（Workers proxy）に切り替える:

- Cloud Run Domain Mapping が asia-northeast1 で利用不可（API 制約変更）
- Cloud Run 証明書発行が長期間 PROVISIONING のまま進まない
- 独自ドメイン購入後の DNS 設定で `ghs.googlehosted.com` への CNAME が解決しない

切替時の作業: `m2-early-domain-and-cookie-plan.md` §12 のフォールバック手順を参照。

### 4.4 Cloud Run リージョン確認

- 既存リソース: `asia-northeast1`（東京、M1 で `vrcpb-spike-api` を deploy 済）
- Domain Mapping 対応: asia-northeast1 は GA（Cloud Run 公式ドキュメント）
- 確認は購入後・Domain Mapping 設定前に再度 `gcloud run domain-mappings list` で利用可否を確認

---

## 5. 購入後の作業順序（**まだ実行しない**、順番のみ）

### 5.1 全体フロー

1. **vrcphotobook.com 購入**（Cloudflare Registrar、ユーザー手動）
2. **Cloudflare zone の自動作成を確認**（Registrar 購入で自動的に zone が作られる）
3. **DNS 状態確認**（NS が Cloudflare、A/AAAA レコードがデフォルト状態）
4. **Workers Custom Domain: `app.vrcphotobook.com`** を `vrcpb-frontend` Worker に紐付け
5. **Backend API ドメイン方針を §4 で確定したものに従って実施**（案 A: Cloud Run Domain Mapping）
   - Cloud Run Domain Mapping `api.vrcphotobook.com` → `vrcpb-api` (asia-northeast1)
   - Cloudflare DNS に CNAME `api` → `ghs.googlehosted.com`（Cloudflare Proxy は **OFF**、TLS 証明書を Google 側で発行させるため）
   - Cloud Run の証明書 PROVISIONING を待つ（最大数時間）
6. **Frontend `.env.production`**:
   - `NEXT_PUBLIC_BASE_URL=https://app.vrcphotobook.com`
   - `NEXT_PUBLIC_API_BASE_URL=https://api.vrcphotobook.com`
   - `COOKIE_DOMAIN=.vrcphotobook.com`
7. **Backend env**:
   - `ALLOWED_ORIGINS=https://app.vrcphotobook.com`（CORS 設定、後続 PR で middleware 追加時）
   - `APP_ENV=production` / `PORT=8080` / `DATABASE_URL=...`（Secret Manager 経由）
8. **Frontend build/deploy**: `npm --prefix frontend run cf:build` → `wrangler deploy`
9. **Backend deploy**: Cloud Build → Artifact Registry → Cloud Run revision
10. **Safari / iPhone Safari 実機確認**（`safari-verification.md` の全項目）
    - `https://app.vrcphotobook.com/draft/<実 token>` → 302 → `/edit/<id>`
    - DevTools / Web Inspector で Cookie 属性確認（HttpOnly / Secure / SameSite=Strict / Path=/ / Domain=.vrcphotobook.com）
    - URL から raw token が消えていること
    - 24 時間後 / 7 日後の Cookie 残存（運用開始後の継続観察）
11. **旧 PoC リソース整理**（§6.5 参照）

### 5.2 各ステップの実行コマンド雛形

詳細コマンドは購入後の別 PR で整備（本 PR では順序のみ）。雛形:

- Workers Custom Domain: Cloudflare Dashboard `Workers & Pages → vrcpb-frontend → Settings → Triggers → Custom Domains` で `app.vrcphotobook.com` を追加
- Cloud Run Domain Mapping: `gcloud beta run domain-mappings create --service=vrcpb-api --domain=api.vrcphotobook.com --region=asia-northeast1`
- Cloudflare DNS: Dashboard `DNS → Records` で `app` (CNAME → `vrcpb-frontend.<account>.workers.dev`) / `api` (CNAME → `ghs.googlehosted.com`、Proxy OFF)
- Wrangler: `npm --prefix frontend run cf:build && wrangler deploy --env production`

### 5.3 切替後の検証チェック（Safari 確認の前段）

- [ ] `dig app.vrcphotobook.com` が Cloudflare の IP を返す
- [ ] `dig api.vrcphotobook.com` が `ghs.googlehosted.com` の CNAME → Google IP を返す
- [ ] `curl -i https://api.vrcphotobook.com/health` が 200 を返す
- [ ] `curl -i https://app.vrcphotobook.com/` が 200 を返す
- [ ] HTTPS 証明書が有効（`curl --cacert ...` で警告無し）

---

## 6. 費用・後片付け

### 6.1 ランニング費用（年額・月額）

| 項目 | 費用 | 備考 |
|---|---|---|
| ドメイン `vrcphotobook.com` | $10.46 / 年 | Cloudflare Registrar at-cost |
| Cloudflare DNS（基本機能） | $0 | Free プラン範囲 |
| Workers Custom Domain | $0 | Workers Free プラン範囲（カスタム ドメイン自体は無料） |
| Workers リクエスト | $0 | Free 100k req/日 まで、MVP では十分 |
| Cloud Run Domain Mapping | $0 | サービス料金は Domain Mapping で変わらない（実行時間課金のみ） |
| Cloud Run 実行 | 〜$0〜数百円 / 月 | アイドル時は $0、リクエスト時のみ課金 |
| Cloud SQL（M2 後期で導入予定） | 〜$10 / 月 | db-f1-micro（最小） |
| **合計** | **〜$10.5/年 + $0〜10/月** | M2 段階の最小構成 |

### 6.2 Budget Alert は ¥1,000 維持

- M1 で設定済の **¥1,000 / 月** Budget Alert（Cloud Billing）はそのまま維持
- ドメイン購入は Cloudflare 側のため Budget Alert に影響しない
- Cloud Run / Cloud SQL の課金が Budget Alert の対象（既設定）

### 6.3 旧 PoC リソースの扱い

| リソース | 状態 | 扱い |
|---|---|---|
| `vrcpb-spike-frontend.k-matsunaga-biz.workers.dev` | M1 PoC で deploy 済 | 独自ドメイン切替後、検証完了したら **削除**（または非公開化）|
| `vrcpb-spike-api-7eosr3jcfa-an.a.run.app` | 同上、Cloud Run service | Domain Mapping 切替後、`vrcpb-api`（本実装名）と統合 or 別 service として削除 |
| Workers / Cloud Run の secrets / env vars | M1 設定 | 本実装名 `vrcpb-frontend` / `vrcpb-api` に再設定、旧名のものは削除 |
| Artifact Registry の M1 image | M1 build | 残しても無料枠内、ただし整理する場合は `gcloud artifacts docker images list` で削除候補を確認 |

### 6.4 削除タイミング

- 独自ドメイン切替 + Safari 実機確認 + 24h / 7 日後の Cookie 残存確認 が **すべて完了してから**削除
- 削除前に必ず削除可能なリソースを `harness/work-logs/` の作業ログに列挙して確認

---

## 7. ユーザー判断事項

購入操作の前に以下を確定してください。

### 7.1 ドメイン

- [ ] **`vrcphotobook.com` を本当に買う**（推奨、§1 / §2 のチェックを通過する前提）
- [ ] `.com` が取れなかった場合のフォールバック: 第二候補 `vrcphotobook.app`（年額 $14.20）に切り替え
- [ ] Cloudflare Registrar で買う（推奨、§3 / §6.1）
- [ ] 自動更新を ON（推奨、§1.4）
- [ ] WHOIS Privacy 有効を確認

### 7.2 構成

- [ ] **API ドメインは案 A（Cloud Run Domain Mapping 直結）**（推奨、§4.2）
- [ ] フォールバック条件を §4.3 の通りで合意

### 7.3 タイミング

- [ ] **購入後すぐに DNS / Workers Custom Domain / Cloud Run Domain Mapping へ進む**
- [ ] それとも、購入だけ先にして、DNS 設定は別 PR でユーザー判断のうえ実施
- [ ] Safari 実機確認は本実装デプロイ後（推奨）

### 7.4 旧 PoC リソース

- [ ] 独自ドメイン切替完了後、旧 `vrcpb-spike-*` リソースを削除する（推奨、§6.3）
- [ ] それとも当面残す（再検証のため）

---

## 8. 実施しないこと（再掲）

本書は **計画書のみ**。以下は実施しない:

- ドメイン購入（**ユーザー手動**で本書 §1〜§3 のチェック通過後に実施）
- DNS 変更（購入後の別 PR で実施）
- Workers Custom Domain 設定 / Cloud Run Domain Mapping
- Cloud Run deploy / Workers deploy
- Cloud SQL 作成
- SendGrid 設定 / Turnstile 本番 widget 作成
- R2 設定変更
- 既存 PoC リソース削除

---

## 9. ユーザーが Cloudflare Dashboard で次にやること

1. ブラウザで [Cloudflare Dashboard](https://dash.cloudflare.com/) にログイン
2. 左メニュー `Domain Registration → Register domains`（または `Domain Registration → Search domains`）
3. 検索ボックスに `vrcphotobook.com` を入力
4. 表示される情報を本書 §1.1 と照合:
   - 取得可否（Available）
   - 年額（$10.46 前後）
5. 取得可能なら、**支払い確定の直前まで**進めて画面を見せてください（本書 §7 の判断と一致するか確認）
6. **支払い確定はまだ押さない**。本書 §1〜§3 の全チェックが ✅ になってから確定

取得できない場合は第二候補 `vrcphotobook.app` で同手順を繰り返し、§7.1 のフォールバックに従う。

---

## 10. 関連ドキュメント

- [M2 ドメイン候補リサーチ](./m2-domain-candidate-research.md)
- [M2 早期ドメイン + Cookie 計画](./m2-early-domain-and-cookie-plan.md)
- [M2 Session auth 実装計画](./m2-session-auth-implementation-plan.md)
- [M2 Photobook session 接続計画](./m2-photobook-session-integration-plan.md)
- [プロジェクト全体ロードマップ](../../harness/work-logs/2026-04-26_project-roadmap-overview.md)
- [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md) / [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
- [Cloudflare Registrar 公式ドキュメント](https://developers.cloudflare.com/registrar/get-started/register-domain/)
- [Cloud Run Domain Mapping 公式ドキュメント](https://docs.cloud.google.com/run/docs/mapping-custom-domains)
