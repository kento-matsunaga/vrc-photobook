# 2026-04-26 M1 実環境デプロイ検証ログ（Step 7〜10）

> 上流: `docs/plan/m1-live-deploy-verification-plan.md`
> 関連 failure-log:
> - `harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md`
> - `harness/failure-log/2026-04-26_wsl-cwd-drift-recurrence.md`
> - `harness/failure-log/2026-04-26_gcloud-install-verification-mismatch.md`
> - `harness/failure-log/2026-04-26_sudo-noninteractive-shell-limit.md`
> - `harness/failure-log/2026-04-26_gcp-account-billing-mismatch.md`

## 確定したリソース

| 項目 | 値 |
|---|---|
| **Frontend Workers URL** | `https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev` |
| Workers Version ID | `d66d3aae-5763-4574-b110-fc6157ceb30c` |
| Worker 名 | `vrcpb-spike-frontend` |
| Worker Startup Time | 19 ms |
| **Backend Cloud Run URL** | `https://vrcpb-spike-api-7eosr3jcfa-an.a.run.app` |
| Backend revision | `vrcpb-spike-api-00003-mxl`（traffic 100%）|
| Backend image digest | `sha256:46a1a2743e70531a421cd68da100ea0749e2792502ed793fff017990e5572f2a`（タグ `m1-live-health`）|
| GCP プロジェクト | `project-1c310480-335c-4365-8a8` |
| リージョン | `asia-northeast1` |

## Step 9: Frontend → Backend 結合確認

### 確認結果（実機ブラウザ + curl）

| 項目 | 結果 |
|---|---|
| Workers URL `/integration/backend-check` 表示 | ✅ |
| 画面の「API base URL」表示が Cloud Run URL（`https://vrcpb-spike-api-7eosr3jcfa-an.a.run.app`） | ✅ `NEXT_PUBLIC_API_BASE_URL` の inline が成立 |
| `GET /health (no credentials)` ボタン | ✅ 200 `{"status":"ok"}` |
| `POST /sandbox/origin-check (credentials: include)` ボタン | ✅ 200 `{"origin_allowed":true}`、`Access-Control-Allow-Origin` に Workers URL が反射 |
| `GET /sandbox/session-check (credentials: include)` ボタン | ⚠️ `{"draft_cookie_present":false,"manage_cookie_present":false}` ← **想定通り**（後述）|
| `GET /sandbox/session-check (credentials: omit)` ボタン | ✅ 同上 |
| `/draft/sample-draft-token` → `/edit/sample-photobook-id` redirect | ✅、URL から token 消失、`Set-Cookie: vrcpb_draft_*` 発行 |
| `/manage/token/sample-manage-token` → `/manage/sample-photobook-id` redirect | ✅、URL から token 消失、`Set-Cookie: vrcpb_manage_*` 発行 |
| Cookie 属性 | ✅ HttpOnly / Secure / SameSite=Strict / Path=/、draft 7 日 / manage 24 時間 |
| preflight (OPTIONS `/sandbox/origin-check`) | ✅ 204、`Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS`、`Access-Control-Max-Age: 600` |
| 許可外 Origin (`https://example.invalid`) | ✅ 403 `origin_not_allowed`、CORS ヘッダなし |
| ブラウザコンソールに CORS エラーなし | ✅ |

### `session-check` が false/false である理由（重要）

- Frontend は `*.workers.dev`、Backend は `*.run.app` の **別オリジン**
- ブラウザ仕様で、別オリジンに `credentials: include` で fetch しても、Cookie は **発行ホストにしか付かない**
- `*.workers.dev` で発行された `vrcpb_draft_*` / `vrcpb_manage_*` Cookie は Backend `*.run.app` には届かない
- **これは設計失敗ではなく、ブラウザ仕様 + 計画書 §6 で想定した挙動**
- ADR-0003 §13 未解決事項 **U2（Cookie Domain / 同一オリジン化判断）**の確定材料として記録（後述）

### Cloud Run 側の起動ログ（slog JSON、新 revision）

```
server starting
db pool nil; turnstile/consume sandbox endpoints will return 503
turnstile secret not configured; running in MOCK mode (PoC only)
CORS allowed origins configured  count=1
```

→ ALLOWED_ORIGINS 反映成功、DB / Turnstile は意図通り未設定で起動継続。

## Step 10: Safari / iPhone Safari 実機確認

### 確認結果

| 項目 | macOS Safari | iPhone Safari |
|---|:-:|:-:|
| `/p/sample-slug` の OGP / `<meta name="robots" content="noindex">` 出力 | ✅ | ✅ |
| `X-Robots-Tag: noindex, nofollow` ヘッダ | ✅ | ✅ |
| `Referrer-Policy: strict-origin-when-cross-origin`（通常ページ） | ✅ | ✅ |
| `Referrer-Policy: no-referrer`（draft / manage / edit） | ✅ | ✅ |
| `/draft/sample-draft-token` → `/edit/sample-photobook-id` redirect | ✅ | ✅ |
| URL から token が消える | ✅ | ✅ |
| `Set-Cookie: vrcpb_draft_*` 属性目視（HttpOnly / Secure / SameSite=Strict / Path=/）| ✅ | ✅ |
| `/manage/token/sample-manage-token` → `/manage/sample-photobook-id` redirect | ✅ | ✅ |
| ページ再読込後も session 維持 | ✅ | ✅ |
| `/integration/backend-check` で Backend Cloud Run に CORS 疎通 | ✅ | ✅ |
| 別オリジン下で Cookie が Backend へ渡らない（U2 確認） | ✅ 想定通り | ✅ 想定通り |
| **大きな問題なし** | ✅ | ✅ |

### 継続観察項目（M1 完了後も追跡）

- [ ] **24 時間後 Cookie 残存**（ITP 影響評価）
- [ ] **7 日後 Cookie 残存**（ITP 長期影響評価）
- [ ] iOS Safari 1 世代前 / iPad Safari / プライベートブラウジング動作

→ **観察起点 = 2026-04-26（本日 Workers 実環境デプロイ完了時点）**。
→ 24 時間後 / 7 日後にユーザー側で `https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev/edit/sample-photobook-id` 等を再アクセスし、`draft session found` / `manage session found` 表示維持を確認。`.agents/rules/safari-verification.md` §履歴に追記する運用。

## U2 Cookie Domain / 同一オリジン化判断への引き継ぎ

### 確認できた事実

1. Workers `*.workers.dev` で発行した Cookie（HttpOnly / Secure / SameSite=Strict / Path=/）が、Cloud Run `*.run.app` への `credentials: include` fetch では **送信されない**
2. これは ADR-0003 §M1 検証結果の §「Cookie Domain（U2、Backend と異なるホスト構成下）」で予測されていた通り
3. CORS / preflight / Origin 反射 / `Allow-Credentials: true` はすべて成立しているため、**ブラウザ仕様としての別オリジン Cookie 不通**が唯一のブロッカー

### M1 計画書 §7 案 A/B/C の評価

| 案 | 内容 | M1 確認結果 | M2 推奨 |
|---|---|---|---|
| **案A** | 共通親ドメイン `app.example.com` / `api.example.com` + Cookie `Domain=.example.com` | M1 では未実施（独自ドメイン未取得）| **第一候補**。M2 早期に独自ドメイン取得 → 案A 採用 |
| **案B** | Workers `/api/*` プロキシ経由で同一オリジン化 | M1 では未実施（Workers fetch プロキシ実装なし）| 第二候補（独自ドメイン取得が遅延した場合）|
| **案C** | Cookie 共有しない、Backend 認可方式を再設計 | — | **採用しない**（ADR-0003 根本変更を避ける）|

### M1 で確定した判断

> **「M2 早期に独自ドメインを取得して案 A を採る」を一次方針として ADR-0003 §13 U2 に追記する。**

これにより Step 9 の `session-check false/false` は U2 確定材料として消化済。

## 課金ガードの現状

| リソース | 状態 | 課金 |
|---|---|---|
| Cloud Run service `vrcpb-spike-api` | revision 003 / min=0 max=2 / 1vCPU/256Mi | 無料枠内 |
| Cloud Run revisions | 3 件、traffic は 003 のみ | 旧 revision はインスタンス無し、課金なし |
| **Cloud Run Jobs** | **0 件** | 0 円 |
| **Cloud Scheduler（asia-northeast1）** | **0 件** | 0 円 |
| **Cloud SQL** | **0 件** | 0 円 |
| Cloudflare Workers `vrcpb-spike-frontend` | 1 つ | 無料枠内 |
| Artifact Registry `vrcpb-spike` | image 1 件（9MB） | 無料枠 0.5GB の 1.8% |
| Secret Manager R2_* | 5 active versions | 約 $0.30/month |
| Cloud Logging | M1 検証ログ | 無料枠内 |
| Budget Alert | 1,000 円設定済 | — |

→ **目下の月額予測は数十円〜100円程度**。Budget Alert 1,000 円に対して十分余裕。

## 既知問題（後続修正候補）

| # | 内容 | 影響 | 対応案 |
|---|---|---|---|
| C | OGP `og:image` が `http://localhost:3000/og-sample.png` のまま | 外部 OGP プレビューで画像が出ない | M2 本実装で `metadataBase: new URL(process.env.NEXT_PUBLIC_BASE_URL)` を `app/layout.tsx` 等に設定（次コミット候補）|
| D | レスポンスヘッダ `X-Robots-Tag: noindex, nofollow, noindex, nofollow` の重複 | noindex 自体は効くが値が重複、運用上のノイズ | `next.config.mjs` の `headers()` から `X-Robots-Tag` を削除し、middleware.ts に一本化（次コミット候補）|
| U2 | `*.workers.dev` ↔ `*.run.app` 別オリジン下で session Cookie 不通 | M1 計画通りの想定 | M2 早期に独自ドメイン取得 → 案 A（共通親ドメイン）採用 |
| ITP | 24h / 7 日後の Safari Cookie 残存 | 継続観察必須 | 観察起点 2026-04-26、`.agents/rules/safari-verification.md` 履歴に追記 |

## M1 完了状態（暫定）

| 項目 | 状態 |
|---|---|
| Frontend PoC（OpenNext / Workers / Safari 初回検証） | ✅ |
| Backend PoC（Go chi / Cloud Run 互換 Docker image / `/health` 正式採用） | ✅ |
| R2 接続（S3 API / presigned PUT / HeadObject）| ✅ |
| Turnstile + upload_verification_session | ✅（PoC 範囲、Cloud Run 実機検証は DB 必須のため Step B で実施予定 or M2 早期に持ち越し）|
| Outbox + 自動 reconciler 起動基盤 | ✅（PoC 範囲）|
| Email Provider 選定（ADR-0004 Accepted、SendGrid 第一 / Mailgun 第二、AWS SES 運用不可）| ✅ |
| **Cloudflare Workers 実環境デプロイ** | ✅（本日完了）|
| **Cloud Run 実環境デプロイ + R2 接続** | ✅（本日完了）|
| **Frontend / Backend CORS / Origin / 結合確認** | ✅（本日完了）|
| **macOS Safari / iPhone Safari 実環境再確認** | ✅（本日完了）|
| 24h / 7 日後 Safari ITP 影響評価 | ⏳ 継続観察（起点 2026-04-26）|
| Cloud SQL（Step B、必要時のみ短時間起動）| ⏳ 未起動（計画書 §4.4 段階化に従う）|
| Cloud Run Jobs + Cloud Scheduler（Step 11）| ⏳ 未着手（U11 確認は Step B 後）|
| 独自ドメイン取得 + Cookie Domain U2 確定 | ⏳ M2 早期 |
| SendGrid 実送信 PoC | ⏳ M2 早期 |
| Turnstile 本番 widget | ⏳ Workers hostname 確定済、必要なら M2 早期 |

## 次工程候補

1. **C + D 修正**（既知問題の小さな潰し込み、`metadataBase` 設定 + middleware 一本化）→ 再 deploy → OGP / ヘッダ再確認
2. その後の判断:
   - U2 案 A の独自ドメイン取得 / Cookie Domain 検証へ
   - Cloud SQL Step B 起動（DB 必須 sandbox / Outbox / Cloud Run Jobs / Scheduler 確認）
   - SendGrid 実送信 PoC（M2 早期）
   - Turnstile 本番 widget 発行

## 後片付け（M1 検証完了時、計画書 §14 通り）

- Cloudflare: Worker 停止 / R2 テストオブジェクト削除 / R2 API Token Revoke
- GCP: Cloud Run service 停止 / Artifact Registry image 削除 / Secret Manager Secret 削除 / Cloud SQL（未作成）
- ローカル: `.env.local` / `.env.production` の値クリア / 本ログを `harness/work-logs/` に保存（**完了済**）
