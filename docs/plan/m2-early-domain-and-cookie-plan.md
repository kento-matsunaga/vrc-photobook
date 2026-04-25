# M2 早期計画 — 独自ドメイン + U2 Cookie Domain / 同一オリジン化

> **位置付け**: M2 早期 4 ブロックのうち **優先度 A**（最優先）の計画書。本書は計画であって本実装ではない。記載された手順は実行前に本書をレビューし、ユーザーの承認後に着手する。
>
> **作成日**: 2026-04-26
>
> **上流**:
> - [`harness/work-logs/2026-04-26_project-roadmap-overview.md`](../../harness/work-logs/2026-04-26_project-roadmap-overview.md) §F-1（優先度 A）
> - [`harness/work-logs/2026-04-26_m1-completion-judgment.md`](../../harness/work-logs/2026-04-26_m1-completion-judgment.md) §3 優先度 A
> - [`docs/adr/0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md) §13 未解決事項 U2 / §M1 検証結果
> - [`docs/plan/m1-live-deploy-verification-plan.md`](./m1-live-deploy-verification-plan.md) §7 Cookie Domain U2 検証案
> - [`harness/work-logs/2026-04-26_m1-live-deploy-verification.md`](../../harness/work-logs/2026-04-26_m1-live-deploy-verification.md)（U2 確定材料）
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
>
> **重要前提**:
> - **Cloud SQL は本ブロックでは作らない**（M2 早期 §F-2 優先度 B、本ブロック完了後）
> - **Cloud Run Jobs / Scheduler / SendGrid / Turnstile 本番 widget も作らない**
> - **AWS SES は採用不可**（ADR-0004 再選定済）
> - **Safari / iPhone Safari を常に検証対象**にする（`.agents/rules/safari-verification.md`）
> - **本書の段階ではドメインは購入しない**

---

## 1. 検証目的

| 目的 | 解消する問題 |
|---|---|
| **U2 Cookie Domain 問題の解消** | Workers `*.workers.dev` で発行された Cookie が Cloud Run `*.run.app` に渡らない（M1 で確認、ADR-0003 §M1 検証結果）|
| Workers / Backend API のドメイン設計確定 | M2 本実装に向け、URL 構造・サブドメイン設計を固定する |
| Safari / iPhone Safari で Cookie / redirect / ITP 観点の再確認 | 別オリジン → 同一親ドメインへの切替後、ITP の挙動が変わる可能性 |
| OGP / 管理 URL 等の絶対 URL 基底の固定 | `metadataBase` / 管理 URL のドメインが本番想定と一致する |

→ M2 早期 4 ブロック（A→B→C→D）の **A** を完了させ、後続の B（Cloud SQL）/ C（SendGrid / Turnstile）が本物の URL 構造で検証できるようにする。

---

## 2. 前提・制約

### 2.1 状態前提（2026-04-26 時点）

- M1 は完了済（`harness/work-logs/2026-04-26_m1-completion-judgment.md`）
- Frontend Workers: `https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev` で稼働中
- Backend Cloud Run: `https://vrcpb-spike-api-7eosr3jcfa-an.a.run.app` で稼働中
- 別オリジンのため `credentials: include` で session Cookie が Backend に渡らない（U2 既知）

### 2.2 着手範囲の制約

- **U2 を本ブロックで解消する**（最優先）
- Cloud SQL は **作らない**（M2 早期 §F-2 優先度 B、本ブロック後）
- Cloud Run Jobs / Cloud Scheduler は **作らない**
- SendGrid 実送信 PoC / Turnstile 本番 widget は **作らない**（M2 早期 §F-3 優先度 C）
- 本実装 `frontend/` / `backend/` への移植は **行わない**（§F-4 優先度 D）
- AWS SES は **採用しない**（ADR-0004 で運用不可と確定）

### 2.3 検証対象ブラウザ

- macOS Safari 最新（必須）
- iPhone Safari 最新（必須）
- Chrome 最新（ベースライン）

`.agents/rules/safari-verification.md` の必須項目を踏襲。Cookie / redirect / OGP / レスポンスヘッダ変更時に Safari 確認は必須。

---

## 3. 独自ドメイン候補（提案、購入はしない）

ドメイン名は最終的にユーザー判断。以下は提案候補（個人情報・本名・会社名を含めない方針）。

| # | 候補 | 性質 | 強み | 弱み |
|---|---|---|---|---|
| 1 | `vrcpb.app` | 短縮形 + `.app` TLD | 短い、`.app` は HSTS preload で HTTPS 強制（セキュリティに有利）、技術ブランド寄り | 略称のため意味が伝わりにくい、`.app` は年額 $14〜$20 程度と中価格帯 |
| 2 | `vrcphotobook.com` | フルネーム + `.com` | 用途が明確、`.com` の信頼性、長期運用に強い | 長め（14 文字 + .com）、`.com` の人気 / 残存性確認必須 |
| 3 | `vrcphotobook.app` | フルネーム + `.app` | 用途明確 + HTTPS 強制、技術寄り | 長い |
| 4 | `vrcphotobook.net` | フルネーム + `.net` | `.com` が取れない場合の標準 | `.com` より信頼度低い印象 |
| 5 | `vrcpb.net` / `vrcpb.dev` 等 | 略称 + 別 TLD | 短い | 意味伝わりにくい |

### 3.1 取得前に必ず確認すること

- **商標調査**: 「VRC PhotoBook」「VRChat」関連の既存商標と衝突しないか（VRChat は VRChat Inc. の商標、`vrc` 略称はサービスを示唆するが商標として登録されていない場合がある）
- **既存サービスとの重複**: 同名ドメインで既存サービスが運営されていないか（Web 検索 + Whois 検索）
- **VRChat 公式との関係性表示**: 「公式 / 提携と誤認させない」運用が必要（VRChat 商標ガイドライン参照）。本サービスは **非公式のファンメイド** であることを LP / 利用規約で明記

### 3.2 購入は本書のレビュー後

候補から決定 → Whois / 商標確認 → ユーザー判断 → 購入、の順で進める。**本書の段階では購入しない**。

---

## 4. 取得元比較

| Registrar | Cloudflare との相性 | DNS 設定 | WHOIS privacy | 価格傾向 | 移管しやすさ | 管理画面 |
|---|---|---|---|---|---|---|
| **Cloudflare Registrar**（推奨）| ★★★★★（DNS / Workers 統合自動）| ワンクリックで Cloudflare DNS に即統合 | **デフォルト無料**で privacy 有効 | At-cost（卸売価格、`.com` $10 程度）| 60 日制限あり（ICANN 規定）| Cloudflare Dashboard 内、シンプル |
| お名前.com | ★★（DNS は別途 Cloudflare へ向け直し）| Cloudflare ネームサーバへ手動切替 | 別料金 | `.com` 1,500 円程度（初年度）+ 翌年〜更新費 | 移管時の認証コード取得が手間 | UI 複雑、付帯サービスの押し売りが多い |
| Namecheap | ★★★（DNS を Cloudflare へ向け直し）| Cloudflare ネームサーバへ切替 | デフォルト無料 | `.com` $10〜13 程度 | 比較的容易 | UI 良好、英語 |
| Google Domains 相当 | — | サービス終了済（Squarespace へ移管された） | — | — | — | 採用しない |

### 4.1 推奨：Cloudflare Registrar

**理由**:
- Cloudflare Workers / Cloud Run Domain Mapping のいずれを採用しても DNS 設定が最短（Workers Custom Domain は Cloudflare DNS なら自動）
- WHOIS privacy がデフォルト無料 → 個人情報保護
- At-cost 価格（更新も同額）→ 長期運用で安価
- TLS 証明書も Cloudflare が自動発行（Universal SSL）

**懸念**:
- Cloudflare アカウントへの依存度が高まる（既に Workers / R2 で依存しているため、追加リスクは小）
- ICANN 規定で取得後 60 日は他レジストラへ移管できない（通常運用では問題なし）

### 4.2 採用しない案

- お名前.com: DNS / Cloudflare 統合の手間 + 付帯サービスの煩雑さ。**採用しない**
- Namecheap: Cloudflare Registrar より優位性なし。**採用しない**
- Google Domains 相当: サービス終了済。**採用しない**

---

## 5. DNS / サブドメイン設計（3 案比較）

### 案 A（推奨）: 階層型サブドメイン + Cookie Domain 案 A

```
<domain>           → LP（将来）or 「app へリダイレクト」
app.<domain>       → Frontend Workers（OpenNext）
api.<domain>       → Backend Cloud Run（Domain Mapping）
Cookie Domain      → .<domain>（共通親ドメイン）
```

**メリット**:
- ADR-0003 §M1 検証結果で「**M2 早期に独自ドメイン取得 → 案 A 採用**」を一次方針として確定済
- Cookie が `.<domain>` で `app.<domain>` ↔ `api.<domain>` の両方に渡る
- Frontend / Backend の物理分離を維持しつつ Cookie 共有が成立
- LP / 管理ページ / API の責務分離が綺麗

**デメリット**:
- DNS レコードが 3 件以上必要
- Cloud Run Domain Mapping の追加設定が必要（§6 参照）
- Cookie が `.<domain>` 全体に乗るため、将来 `<sub>.<domain>` を追加する場合に Cookie が漏れる懸念（MVP 範囲では問題なし）

### 案 B: ルートを LP、機能はサブドメイン

```
<domain>      → LP
app.<domain>  → App
api.<domain>  → API
```

**メリット**: ルートが LP 専用で運用しやすい

**デメリット**: 案 A の単なる解釈違い（実質的にサブドメイン構造は案 A と同じ）。**案 A に統合**

### 案 C: Workers `/api/*` プロキシで同一オリジン化

```
<domain>           → LP（将来）
app.<domain>       → Frontend Workers
                     - 通常パス: Next.js SSR
                     - /api/* パス: Workers 内で Cloud Run へ fetch（リバースプロキシ）
api.<domain>       → 公開しない（Cloud Run 直アクセスは内部のみ）
Cookie Domain      → app.<domain>（単一ホスト）
```

**メリット**:
- 完全に同一オリジンになるため Cookie Domain 設定不要
- CORS / preflight も不要（同一オリジン fetch）
- 別オリジン Cookie 配信のリスク（ITP 等）が構造的に消える

**デメリット**:
- Workers が API 経路を抱えるためレイテンシ +1 hop（Workers → Cloud Run）
- Workers の制限（CPU 時間 / リクエストサイズ）に Backend API も巻き込まれる
- Cloud Run の Set-Cookie を Workers が正しく転送する実装が必要（実装コスト）
- 画像アップロード（presigned PUT）は **Cloud Run が直接発行する**ため Cloud Run のドメインは結局必要 → `api.<domain>` を内部利用としても DNS 上は隠す設計が複雑化

### 5.1 推奨：案 A

**理由**:
- ADR-0003 で確定済の一次方針
- Workers のリバースプロキシ実装コストを避けられる
- 案 C の Workers 制限巻き込みリスクを避けられる
- Cloud Run Domain Mapping は無料・標準機能で十分実用

**例外条件**: Cloud Run Domain Mapping が地域・組織制限で使えなかった場合 → 案 C にフォールバック（§12 失敗時の判断）

---

## 6. Backend API ドメイン方針（2 案比較）

### 案 P1（推奨）: Cloud Run Domain Mapping で `api.<domain>` を Cloud Run 直結

```
api.<domain>  --DNS CNAME-->  ghs.googlehosted.com  --Cloud Run Domain Mapping-->  vrcpb-spike-api
```

**メリット**:
- 設定がシンプル（Cloud Run の Domain Mapping 機能、自動 TLS 発行）
- Workers の制限を回避
- Cloud Run のヘルスチェック / Cloud Logging / トレースが Backend 単独で完結
- デバッグ容易（Cloud Run のリクエストログがそのまま見られる）

**デメリット**:
- Cloud Run Domain Mapping は一部リージョンで Preview / 未提供（asia-northeast1 は GA 確認済、要再確認）
- 独自ドメインのため証明書発行に **DNS 検証**が必要（数分〜数時間）

### 案 P2: Workers `/api/*` プロキシ（案 C と同じ）

**メリット**:
- 同一オリジン構造で Cookie 設定不要
- DNS は `app.<domain>` 1 件のみ

**デメリット**: §5 案 C と同じ（Workers レイテンシ / 制限 / Set-Cookie 転送実装コスト）

### 6.1 採用しない案

- **Cloudflare Tunnel**: MVP では不要。Cloud Run は HTTPS で直接公開可能で、Tunnel の追加レイヤーはオーバーキル。Phase 2 以降で社内ツール公開等が必要になれば検討
- **API Gateway / Cloud Endpoints**: Cloud Run の機能で十分、追加サービスは不要

### 6.2 推奨：案 P1（Cloud Run Domain Mapping）

判断観点別の比較:

| 観点 | 案 P1 | 案 P2 |
|---|---|---|
| Cookie Domain | `.<domain>` で共有可、案 A と整合 | 同一オリジンで Cookie Domain 不要だが Cookie 制御が Workers 経由で複雑化 |
| Safari ITP | 別ホストでも First-party Cookie として扱われる（同一親ドメイン）| 完全に同一オリジン、ITP 影響最小 |
| CORS 複雑性 | Origin: `app.<domain>` を ALLOWED_ORIGINS に追加するだけ | CORS 不要 |
| 運用の単純さ | Cloud Run 直接、ログ・メトリクスもそのまま | Workers が Backend を抱えるため運用範囲広い |
| 将来の本番構成 | 案 P1 の構造はそのまま本番で使える | 案 P2 は将来の API クライアント（モバイル等）追加時に再設計が必要 |
| デバッグしやすさ | curl で直接 `api.<domain>` を叩ける | Workers 経由のため Workers ログも見る必要あり |

→ **案 P1（Cloud Run Domain Mapping）を推奨**。Cloud Run Domain Mapping が asia-northeast1 で使えない / 何らかの障害が発生した場合のみ案 P2 へフォールバック（§12）。

---

## 7. U2 Cookie Domain 実装方針（ADR-0003 §13 U2 解消）

### 7.1 Cookie 属性

ADR-0003 §13 通り（再確認）:

| 属性 | 値 |
|---|---|
| `HttpOnly` | `true`（必須）|
| `Secure` | `true`（必須、HTTPS のみ）|
| `SameSite` | `Strict`（必須、CSRF 一次対策）|
| `Path` | `/`（全パスで利用）|
| **`Domain`（M2 早期で確定）** | **`.<domain>`**（共通親ドメイン）← 案 A 採用 |

### 7.2 Cookie 名（既存維持）

```
vrcpb_draft_{photobook_id}     # draft session
vrcpb_manage_{photobook_id}    # manage session
```

### 7.3 期限（既存維持）

- draft session: `Max-Age = 604800`（7 日、Photobook の `draft_expires_at` まで）
- manage session: `Max-Age = 86400`（24 時間〜7 日、長期化しない）

### 7.4 token → session 交換フロー（既存維持）

ADR-0003 §決定 / 全体方針通り:

1. `app.<domain>/draft/{draft_edit_token}` へアクセス
2. Backend が `api.<domain>` 経由で token を hash 検証（または Frontend Server Component が Backend へ HTTPS 呼び出し）
3. 256bit 乱数 `session_token` を生成、SHA-256 を DB に保存
4. **`Set-Cookie: vrcpb_draft_{photobook_id}=...; Domain=.<domain>; HttpOnly; Secure; SameSite=Strict; Path=/`**
5. `app.<domain>/edit/{photobook_id}` に redirect（URL から token 除去）
6. 以後の API は `api.<domain>` への `credentials: include` fetch で Cookie 付与

### 7.5 history.replaceState / redirect で URL から token を消す

- 入場ルート（`/draft/{token}` / `/manage/token/{token}`）は **302 redirect**で URL から token を除去
- ブラウザ履歴に raw token が残る場合は `history.replaceState` で履歴自体を書き換える（UX 検討、Phase 2 でも可）
- Referrer-Policy: no-referrer は維持（外部リンク経由での token 漏洩防止）

### 7.6 Safari で確認すべきこと

`.agents/rules/safari-verification.md` 必須項目に加え、本ブロック特有の確認:

| # | 確認項目 |
|---|---|
| 1 | `app.<domain>/draft/{token}` → `app.<domain>/edit/{photobook_id}` redirect で `Set-Cookie: Domain=.<domain>` が macOS Safari / iPhone Safari で受け入れられる |
| 2 | DevTools / Web Inspector で Cookie の Domain 属性が `.<domain>` になっていること目視 |
| 3 | `app.<domain>/integration/backend-check` の `GET /sandbox/session-check (credentials: include)` が **`{"draft_cookie_present":true,"manage_cookie_present":true}`** に変わる（M1 では false/false だった） |
| 4 | `api.<domain>/sandbox/session-check` を curl で叩いた場合と、ブラウザで叩いた場合の差異確認 |
| 5 | iPhone Safari の ITP が `.<domain>` 配下を **First-party Cookie** として扱う（24h / 7 日後の Cookie 残存観察起点を更新）|
| 6 | プライベートブラウジングでも Cookie が機能する（参考、必須ではない） |

---

## 8. 切替手順（**まだ実行しない**、計画段階）

実行は本書のレビュー → ユーザー承認後。

### Step 1. ドメイン取得

- Cloudflare Registrar で候補から選定したドメインを購入
- **DNS は Cloudflare に自動統合**（Cloudflare Registrar 経由なら追加設定不要）
- WHOIS privacy デフォルト有効を確認

### Step 2. Cloudflare DNS レコード追加

Cloudflare Dashboard → DNS で以下を追加:

```
タイプ  名前         値                              プロキシ
CNAME  app          vrcpb-spike-frontend.k-matsunaga-biz.workers.dev   ❌（Workers Custom Domain は別経路）
CNAME  api          ghs.googlehosted.com           ❌（Cloud Run Domain Mapping 経路）
A/AAAA @            （LP 用、後で確定）              ❌
```

> 実際は Workers Custom Domain / Cloud Run Domain Mapping 設定時に Cloudflare 側が必要なレコードを案内する場合がある。手動追加する前に **Cloudflare Dashboard / Cloud Run の指示に従う**。

### Step 3. Workers Custom Domain 設定（`app.<domain>`）

- Cloudflare Dashboard → Workers & Pages → `vrcpb-spike-frontend` → Custom Domains
- `app.<domain>` を追加
- Cloudflare が自動で TLS 証明書を発行
- `wrangler.jsonc` の `routes` セクションへの追記は **不要**（Custom Domain UI 経由で管理）

### Step 4. Backend API ドメイン設定（`api.<domain>`）

```sh
# Cloud Run Domain Mapping（asia-northeast1 GA 確認後）
gcloud run domain-mappings create \
  --service=vrcpb-spike-api \
  --domain=api.<domain> \
  --region=asia-northeast1
```

- 出力に従って Cloudflare DNS に CNAME / A / AAAA レコードを追加
- Cloud Run が Let's Encrypt 経由で証明書発行（数分）
- `gcloud run domain-mappings describe` で `READY` 確認

### Step 5. Frontend 側の環境変数更新

`harness/spike/frontend/.env.production` を更新:

```
NEXT_PUBLIC_API_BASE_URL=https://api.<domain>
NEXT_PUBLIC_BASE_URL=https://app.<domain>
```

> 旧 `vrcpb-spike-api-7eosr3jcfa-an.a.run.app` / 旧 Workers URL の値は **コメントとして残し**、ロールバック可能にする。

### Step 6. Cloud Run ALLOWED_ORIGINS 更新

```sh
gcloud run services update vrcpb-spike-api \
  --region=asia-northeast1 \
  --update-env-vars=ALLOWED_ORIGINS=https://app.<domain>
```

> 移行期間中は **`ALLOWED_ORIGINS=https://app.<domain>,https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev`** のように **両方残す**ことも検討（§10 旧 URL 扱い参照）。

### Step 7. Backend 側の Cookie Domain 設定変更

Backend の token → session 交換 Route Handler / API（M1 PoC では Frontend Workers 側に実装、M2 本実装では Backend Cloud Run 側）が `Set-Cookie` する箇所で **`Domain=.<domain>`** を付与する変更を入れる。

> M1 PoC の `harness/spike/frontend/app/draft/[token]/route.ts` 等は本ブロックで Backend 移譲する想定。Frontend だけで済ませている現状のスケッチを保つなら、Frontend の `Set-Cookie` 生成箇所に `Domain` 属性を追加する小修正のみ。

### Step 8. Frontend 再 build / 再 deploy

```sh
npm --prefix harness/spike/frontend run cf:build
npm --prefix harness/spike/frontend run cf:deploy
```

### Step 9. Backend 再 deploy（Cookie Domain 変更を反映）

```sh
# Dockerfile / コードに変更があれば再 build + push
docker build -f harness/spike/backend/Dockerfile -t <IMAGE>:m2-domain harness/spike/backend
docker push <IMAGE>:m2-domain
gcloud run deploy vrcpb-spike-api --image=<IMAGE>:m2-domain --region=asia-northeast1
```

### Step 10. curl 確認

```sh
# /health（Workers Origin で）
curl -i -H "Origin: https://app.<domain>" https://api.<domain>/health

# CORS
curl -i -X POST -H "Origin: https://app.<domain>" -H "Content-Type: application/json" -d '{}' \
  https://api.<domain>/sandbox/origin-check

# preflight
curl -i -X OPTIONS -H "Origin: https://app.<domain>" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type" \
  https://api.<domain>/sandbox/origin-check
```

### Step 11. Chrome 確認

- `https://app.<domain>/` 表示
- `/draft/sample-draft-token` → `/edit/sample-photobook-id` redirect、`Set-Cookie: Domain=.<domain>`
- DevTools Application → Cookies で Domain 属性確認
- `/integration/backend-check` で `session-check` が **true / true** を返す

### Step 12. Safari / iPhone Safari 実機確認

§9 検証手順を Safari 実機で実施。

---

## 9. 検証手順

### 9.1 必須確認項目

| # | 項目 | 期待結果 |
|---|---|---|
| 1 | `GET https://api.<domain>/health` | 200 `{"status":"ok"}` |
| 2 | `GET https://api.<domain>/readyz` | DB 未設定なら 503 `db_not_configured`（Step B 後は 200）|
| 3 | `GET https://app.<domain>/draft/sample-draft-token` | 302 redirect → `/edit/sample-photobook-id`、`Set-Cookie: vrcpb_draft_*; Domain=.<domain>; HttpOnly; Secure; SameSite=Strict; Path=/` |
| 4 | `GET https://app.<domain>/manage/token/sample-manage-token` | 302 → `/manage/sample-photobook-id`、`Set-Cookie: vrcpb_manage_*; Domain=.<domain>` |
| 5 | `GET https://app.<domain>/integration/backend-check` 表示 | 200、API base URL = `https://api.<domain>` |
| 6 | ボタン: `GET /health` | 200 `{"status":"ok"}` |
| 7 | ボタン: `POST /sandbox/origin-check (credentials: include)` | 200 `{"origin_allowed":true}` |
| 8 | ボタン: `GET /sandbox/session-check (credentials: include)` | **`{"draft_cookie_present":true,"manage_cookie_present":true}`**（M1 では false/false、ここが変わる）|
| 9 | DevTools Cookie 属性目視 | Domain=`.<domain>` / HttpOnly / Secure / SameSite=Strict / Path=/ |
| 10 | 別オリジン（例: `https://example.invalid`）からの `POST /sandbox/origin-check` | 403 `origin_not_allowed`（CORS ヘッダなし）|

### 9.2 Safari Web Inspector 確認

`.agents/rules/safari-verification.md` 通り:

- macOS Safari → 開発 → Web Inspector → Storage → Cookies で Domain 属性確認
- iPhone Safari → Mac の Safari と USB 接続 → Web Inspector で同様確認
- ページ再読込後も session 維持
- Cookie 値・raw token を console / 画面 / スクリーンショットに出さない

### 9.3 24h / 7 日後 ITP 継続観察（起点更新）

- M1 完了時の起点 2026-04-26 はそのまま、**独自ドメイン切替日**を新たな起点として `.agents/rules/safari-verification.md` §履歴に追記
- 観察対象 URL: `https://app.<domain>/edit/sample-photobook-id` / `/manage/sample-photobook-id`
- 24h 後 / 7 日後アクセスで `draft session found` / `manage session found` 表示維持を確認

### 9.4 失敗を観察したらすぐ failure-log

- Cookie が消える / Domain 属性が反映されない / Safari で session が短時間で消える 等が起きたら **`harness/failure-log/`** に記録

---

## 10. 旧 URL の扱い

### 10.1 併存期間

- **新ドメイン切替直後〜M2 中盤**: 旧 `vrcpb-spike-frontend.k-matsunaga-biz.workers.dev` / `vrcpb-spike-api-7eosr3jcfa-an.a.run.app` は**併存させる**
- 理由:
  - 切替直後の障害ロールバック先として機能
  - M1 検証時の URL がドキュメント / failure-log に残っているため即時削除すると参照が壊れる
  - 旧 URL は無料枠内でほぼコストゼロ

### 10.2 旧 URL の機能制限

- **旧 Workers** (`vrcpb-spike-frontend.k-matsunaga-biz.workers.dev`):
  - 同じ Worker から配信されるため URL 自体は止められない（Cloudflare 仕様、Custom Domain 設定後も `*.workers.dev` は併存）
  - 影響軽減策: トップページに「**新ドメイン `app.<domain>` へ移行しました**」の redirect or 案内を仕込む（任意）
- **旧 Cloud Run URL** (`vrcpb-spike-api-7eosr3jcfa-an.a.run.app`):
  - Cloud Run service にはデフォルト URL と Custom Domain の両方が紐づく
  - `ALLOWED_ORIGINS` を新ドメインのみに更新すれば、旧 Workers から旧 Cloud Run を呼んでも CORS で拒否される（実質機能停止）

### 10.3 廃止タイミング（候補）

- **M2 中盤**（本実装移植時、§F-4 優先度 D 着手と同時）に旧 Workers / 旧 Cloud Run service を**削除**するのが自然
- それまでは月額数十円なので残置

### 10.4 redirect するか？

- **MVP では旧 URL に redirect を仕込まない**:
  - 公開していない URL のため SEO 影響なし（もともと noindex）
  - 関係者しか知らない URL のため、移行案内テキスト or 削除で十分
- Phase 2 で広報系 URL になった場合のみ 301 redirect を検討

---

## 11. 課金影響

### 11.1 増分

| 項目 | 月額・年額 | 備考 |
|---|---|---|
| **ドメイン年額** | `.com` $10 / `.app` $14〜$20 / `.net` $11 程度（Cloudflare Registrar、at-cost） | 年単位、初年度〜更新も同額 |
| Cloudflare DNS | **無料** | 標準機能 |
| Workers Custom Domain | **無料**（Free / Paid プラン共通、無料枠内）| Cloudflare 公式 |
| Cloud Run Domain Mapping | **無料** | Google 公式（asia-northeast1 で利用可） |
| TLS 証明書（Cloudflare Universal SSL / Let's Encrypt）| **無料** | 自動発行・更新 |

### 11.2 維持

| 項目 | 月額（M1 完了時点と同じ）|
|---|---|
| Cloud Run service | 無料枠内（min=0） |
| Artifact Registry | 9MB（無料枠内） |
| Secret Manager R2_* | 約 $0.30/month |
| Cloud SQL | **0 円**（未作成、本ブロックでも作らない） |
| Cloud Run Jobs / Scheduler | **0 円**（未作成） |

### 11.3 Budget Alert

- **1,000 円 / 月のまま維持**
- ドメイン年額（最大 $20 ≈ 3,000 円）は単発支払いのため Budget Alert の月額には影響しない
- Cloud SQL を立てる Step B（M2 早期 §F-2）に進む際は別計画書で **Step C 停止 / 削除まで一気通貫**で進める方針を明記

---

## 12. 失敗時の判断

| 症状 | 第一対応 | 代替案 |
|---|---|---|
| **ドメイン取得後に Workers Custom Domain が通らない** | Cloudflare Dashboard で DNS と Custom Domain 設定を再確認、`dig app.<domain>` で名前解決確認 | 24h 待って再試行 → それでも通らなければ別ドメインで再取得 |
| **`api.<domain>` を Cloud Run へ向けられない**（Domain Mapping 失敗） | `gcloud run domain-mappings describe` でエラー確認、DNS 検証レコードを正確に追加 | asia-northeast1 で機能制限がある場合 → §6 案 P2（Workers `/api/*` プロキシ）にフォールバック |
| **Cookie Domain が Safari で効かない**（`.<domain>` 設定でも Cookie が `app.<domain>` ↔ `api.<domain>` 間で渡らない） | Cookie ヘッダを Web Inspector で精査、Public Suffix List との衝突がないか確認 | §5 案 C（Workers `/api/*` プロキシで完全同一オリジン化）にフォールバック |
| **`SameSite=Strict` で想定外の挙動**（POST 後の redirect で Cookie が落ちる等）| 該当パスの redirect ステータスコード（302 / 303 / 307）見直し | `SameSite=Lax` への一時切替を検討（ただし ADR-0003 §決定の SameSite=Strict 方針改訂が必要 → ADR 改訂レビュー）|
| **Safari ITP で session が短時間で消える** | `.<domain>` 共通親なので **First-party** 扱いになるはず。それでも消えるなら ITP の追加対策（`partitioned` 属性等）を検討 | ADR-0003 §13 U2 に追記、案 C（同一オリジン）への切替検討 |
| **案 A から案 C への切替判断**| 案 A で 1〜2 日試して上記いずれかの致命的問題が出た場合 | 案 C（Workers `/api/*` プロキシ）の実装計画書を別途作成、ADR-0003 §13 U2 を案 C 採用に更新 |

### 12.1 ロールバック手順

- `harness/spike/frontend/.env.production` を旧 Workers / 旧 Cloud Run URL に戻す
- Cloud Run の `ALLOWED_ORIGINS` を旧 Workers URL に戻す
- Frontend を再 build / 再 deploy
- 旧 URL が併存している間は数分で復旧可能

---

## 13. ユーザー判断事項

最終的に以下をユーザーに決定いただく必要があります。

| # | 判断項目 | 推奨 / 提案 | 状態 |
|---|---|---|---|
| 1 | **ドメイン名** | §3 候補から 1 つ選定 | ✅ **`vrcphotobook.com` で合意**（2026-04-26、`m2-domain-candidate-research.md` §9.1）。第二候補 `vrcphotobook.app` はバックアップ |
| 2 | **取得元** | **Cloudflare Registrar**（§4.1 推奨）| ✅ 合意済 |
| 3 | **app / api のサブドメイン構成** | §5 案 A（`app.<domain>` / `api.<domain>` / Cookie Domain `.<domain>`）| ✅ 推奨採用 |
| 4 | **Backend を `api.<domain>` 直結 vs Workers `/api/*` プロキシ** | §6 案 P1（Cloud Run Domain Mapping）| ✅ 推奨採用 |
| 5 | **購入のタイミング** | M2 本実装骨格が固まり、Cookie Domain / URL 設計 / SendGrid / Turnstile 本番 widget の利用タイミングが近づいた段階 | ⏸ **購入延期**（2026-04-26、ユーザー判断、`m2-domain-candidate-research.md` §9.2）|
| 6 | **M2 早期で Cloud SQL より先に U2 を解消する方針で OK か** | ✅ 推奨（roadmap §F-1 優先度 A）| ✅ 合意済 |
| 7 | **旧 Workers / 旧 Cloud Run の併存期間** | M2 中盤までを推奨（§10.3）| 合意済 |
| 8 | **失敗時に案 A → 案 C へフォールバックする判断基準** | §12 通り、致命的問題が 1〜2 日で改善しなければ切替 | 合意済 |

---

## 14. 関連ドキュメント

- [`harness/work-logs/2026-04-26_project-roadmap-overview.md`](../../harness/work-logs/2026-04-26_project-roadmap-overview.md) §F-1 / §I-3
- [`harness/work-logs/2026-04-26_m1-completion-judgment.md`](../../harness/work-logs/2026-04-26_m1-completion-judgment.md) §3 優先度 A
- [`harness/work-logs/2026-04-26_m1-live-deploy-verification.md`](../../harness/work-logs/2026-04-26_m1-live-deploy-verification.md)（U2 確定材料）
- [`docs/adr/0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md) §13 U2 / §M1 検証結果
- [`docs/plan/m1-live-deploy-verification-plan.md`](./m1-live-deploy-verification-plan.md) §7 Cookie Domain U2 検証案
- [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)

## 15. 履歴

| 日付 | 変更 |
|---|---|
| 2026-04-26 | 初版作成。M2 早期 §F-1 優先度 A の計画書として、独自ドメイン取得 + U2 Cookie Domain 解消の手順を整理。推奨は 案 A（`app.<domain>` / `api.<domain>` / Cookie Domain `.<domain>`）+ 案 P1（Cloud Run Domain Mapping）。本書の段階ではドメインは購入しない。ユーザー判断事項 8 項目を §13 に整理 |
| 2026-04-26 | ドメイン候補調査結果を [`m2-domain-candidate-research.md`](./m2-domain-candidate-research.md) として独立記録。**第一候補 `vrcphotobook.com` で合意**（ユーザー判断）。**購入は M2 本実装骨格確定後に延期**（実リソース操作を増やさず、Cookie Domain / SendGrid / Turnstile 本番 widget の利用タイミングが近づいた段階で購入解禁）。§13 ユーザー判断事項 #1 / #5 を「合意済 / 延期」に更新 |
