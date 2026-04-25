# M1 Spike: Frontend PoC

> **目的**: M1 スパイク検証計画 [`docs/plan/m1-spike-plan.md`](../../../docs/plan/m1-spike-plan.md) の優先順位 1・2 に対応する最小 PoC。
>
> Next.js 15 App Router + Cloudflare Pages + Cookie / Session の成立確認のみを目的とし、本実装には流用しない。

---

## 重要な前提

- **本実装ディレクトリ `frontend/` は触らない**。本 PoC は `harness/spike/frontend/` に閉じる。
- **PoC コードを本実装に流用しない**。M2 の本実装は別途 `frontend/` で書き直す。
- **秘密情報・実在 token・API キーをコミットしない**。`.env` 系はすべて `.gitignore` 対象。
- **Cookie 値・raw token を `console.log` / 画面に出さない**。存在の有無のみ表示する。
- 実装は粗くてよい。ただし検証手順とその結果記入欄は明確にする。

---

## 検証ルート一覧

| ルート | 種別 | 目的 |
|--------|------|------|
| `/` | ページ | リンク集（トップ） |
| `/p/[slug]` | Server Component | SSR / OGP メタタグ / noindex / `Referrer-Policy: strict-origin-when-cross-origin` |
| `/draft/[token]` | Route Handler (GET) | token → `vrcpb_draft_{photobook_id}` Cookie 発行 + `/edit/{photobook_id}` redirect |
| `/edit/[photobook_id]` | Server Component | draft session Cookie 読取・存在確認 |
| `/manage/token/[token]` | Route Handler (GET) | token → `vrcpb_manage_{photobook_id}` Cookie 発行 + `/manage/{photobook_id}` redirect |
| `/manage/[photobook_id]` | Server Component | manage session Cookie 読取・存在確認 |

固定値（PoC のため）:

- `photobook_id` = `sample-photobook-id`
- draft token = `sample-draft-token`（任意の値で代替可、検証では値を見ない）
- manage token = `sample-manage-token`（同上）
- session 値はダミー固定文字列（本実装では 32 バイト乱数 + DB 保存の hash）

---

## ヘッダ制御の方針

`middleware.ts` で全リクエストに対して以下を出し分ける:

| パス | `Referrer-Policy` | `X-Robots-Tag` |
|------|------------------|----------------|
| `/draft/*`, `/manage/*`, `/edit/*` | `no-referrer` | `noindex, nofollow` |
| その他（`/`, `/p/*` 等） | `strict-origin-when-cross-origin` | `noindex, nofollow` |

加えて HTML 内に `<meta name="robots" content="noindex">` を `generateMetadata` から出力する（ADR-0003 / v4 §7.6 準拠）。

---

## OpenNext / next-on-pages の選択

M1 計画では両者の比較が検証対象。本 PoC は **`@cloudflare/next-on-pages` を第一候補として開始する**。

理由:
- Cloudflare 公式メンテナンス
- Next.js 15 App Router + Edge runtime のサポートが進んでいる
- `wrangler pages dev` でローカル Cloudflare 互換環境で確認できる

**OpenNext との比較は M1 中に別途行う**（本 PoC でブロックが見つかった場合のみ）。比較結果は本 README の「OpenNext 比較メモ」セクションに追記する。

---

## ローカルで確認する手順

### 前提

- Node.js 20+ 推奨（Cloudflare Pages の Node 互換性に合わせる）
- npm / pnpm / yarn のいずれか

### 1. 依存インストール

```sh
cd harness/spike/frontend
npm install
```

### 2. ローカル開発サーバ（Next.js 標準）

```sh
npm run dev
```

→ `http://localhost:3000` でアクセス。SSR / Cookie / redirect の基本動作を確認。

ただし Next.js 標準サーバは Cloudflare Pages の Edge runtime と完全には一致しないため、最終確認は次の手順で行う。

### 3. Cloudflare Pages 互換ローカル実行

```sh
npm run pages:build       # @cloudflare/next-on-pages でビルド
npm run pages:preview     # wrangler pages dev で起動
```

→ `http://localhost:8788` 等でアクセス。Cloudflare Pages 環境に近い挙動で確認できる。

`Set-Cookie` / `redirect` / Edge runtime の組み合わせが動くかをここで判定する。

---

## Cloudflare Pages へのデプロイ手順（参考）

Cloudflare ダッシュボード or `wrangler` 経由でデプロイ可能。M1 PoC では「動くか」だけを確認するため、最小手順を記載する。

### A. Wrangler 経由

```sh
# 1. ビルド
npm run pages:build

# 2. プロジェクト作成（初回のみ、ダッシュボードで先に作るのが楽）
npx wrangler pages project create vrcpb-spike --production-branch=main

# 3. デプロイ
npx wrangler pages deploy .vercel/output/static --project-name=vrcpb-spike
```

### B. Git 連携経由

1. Cloudflare Dashboard → Pages → Create a project → Connect to Git
2. リポジトリと `harness/spike/frontend` を指定
3. ビルドコマンド: `npx @cloudflare/next-on-pages`
4. ビルド出力ディレクトリ: `.vercel/output/static`
5. 環境変数: 不要（PoC は秘密情報を使わない）

**注意**: `harness/spike/frontend` をルートにしたモノレポビルドが Cloudflare Pages で扱いにくい場合、PoC 専用のリポジトリにコピーして検証することも検討する。

---

## 検証チェックリスト

検証実施時に本欄を埋めること。

### Chrome / Edge（最低限のベースライン）

- [ ] `/p/sample-slug` で View Source して以下を確認:
  - [ ] `<meta property="og:title" content="...">` が出る
  - [ ] `<meta property="og:description" content="...">` が出る
  - [ ] `<meta property="og:image" content="/og-sample.png">` が出る
  - [ ] `<meta name="twitter:card" content="summary_large_image">` が出る
  - [ ] `<meta name="robots" content="noindex">` が出る
- [ ] `/p/sample-slug` の Response Headers で以下を確認:
  - [ ] `X-Robots-Tag: noindex, nofollow`
  - [ ] `Referrer-Policy: strict-origin-when-cross-origin`
- [ ] `/draft/sample-draft-token` にアクセス → `/edit/sample-photobook-id` に 302 redirect される
- [ ] redirect 後の URL に token が残っていない
- [ ] `/edit/sample-photobook-id` で `draft session found` 表示
- [ ] DevTools Cookies で以下を確認:
  - [ ] `vrcpb_draft_sample-photobook-id` が存在
  - [ ] HttpOnly = ✓
  - [ ] Secure = ✓
  - [ ] SameSite = Strict
  - [ ] Path = /
- [ ] `/manage/token/sample-manage-token` にアクセス → `/manage/sample-photobook-id` に redirect
- [ ] `/manage/sample-photobook-id` で `manage session found` 表示
- [ ] `/edit/*` / `/manage/*` の Response Headers で `Referrer-Policy: no-referrer` が出る

### Safari（macOS）

- [ ] 上記 Chrome の全項目を Safari でも再確認
- [ ] ページ再読み込み後も `draft session found` のまま
- [ ] ページ再読み込み後も `manage session found` のまま
- [ ] Web Inspector → Storage → Cookies で属性確認

### iPhone Safari（iOS Safari）

- [ ] iOS Safari 最新で全項目再確認
- [ ] iOS Safari 1 世代前で再確認
- [ ] **redirect 後に Cookie が引き継がれる**（最重要）
- [ ] **数分〜数時間後にアクセスし直しても session found のまま**
- [ ] **24 時間後 / 7 日後の Cookie 残存確認**（ITP 影響）
- [ ] プライベートブラウジングでの動作（参考）

### CSRF / Origin 検証

- [ ] 別オリジンからのリンク遷移で `SameSite=Strict` により Cookie が送信されない
- [ ] 自オリジンの遷移では Cookie が送信される

### Cloudflare Pages 環境固有

- [ ] `npm run pages:build` がエラーなく完了する
- [ ] `npm run pages:preview` で起動し、上記検証がローカル Cloudflare 互換環境で再現する
- [ ] Cloudflare Pages 上で同じ検証が成立する（デプロイ後）
- [ ] Edge runtime で `cookies()`, `NextResponse.cookies.set()`, `NextResponse.redirect()` が動く

---

## 既知の制限・未検証事項

- **CSP（Content Security Policy）は M1 では設定しない**。M2 で本実装と同時に設定する。
- **Cookie の `Domain` 属性は未指定**（U2、ADR-0003）。本 PoC ではフロント単独での検証のみ。Backend と異なるホスト構成下での Cookie 動作は別途 PoC（優先順位 3 以降）で確認する。
- **本実装の token 検証ロジック（hash 照合・期限チェック）は含まない**。本 PoC は「Cookie が発行できて redirect 後に読めるか」だけが目的。
- **Turnstile / upload-verification は本 PoC に含まない**。優先順位 5 で別 PoC を作る。
- **OGP 画像実体は用意しない**。`og:image` の URL が HTML に出ることのみ確認する。

---

## OpenNext 比較メモ

### 第一候補の再評価（2026-04-25 検証で判明）

- **当初 README 記載**: `@cloudflare/next-on-pages` を第一候補、OpenNext は比較対象
- **2026-04-25 検証で判明**: `@cloudflare/next-on-pages@1.13.16` は **deprecated**。`npm install` 時に Cloudflare 公式が **OpenNext adapter (`@opennextjs/cloudflare`)** を推奨に切替済との警告が出る
- **新しい第一候補**: **OpenNext adapter (`@opennextjs/cloudflare`)** を M2 本実装の第一候補とする
- **next-on-pages PoC の扱い**: 検証目的（SSR/Cookie/redirect/ヘッダ）は完全達成済みのため、本 PoC コードは「動作確認できたベースライン」として保持。M1 中に OpenNext adapter 版 PoC を別途構築し、同等の検証を行う
- **記録**: `harness/failure-log/2026-04-25_cloudflare-next-on-pages-deprecated.md`
- **切替判断基準**: M1 計画 §6.1（案A〜D）— OpenNext で同様に成立すれば案A を維持、不成立なら案B/C/D を検討

### 検証結果（2026-04-25 next-on-pages 版）

#### 成功した項目（CLI 検証で確認済み）

| 項目 | Next.js 標準 dev | wrangler pages dev (Cloudflare 互換) |
|------|:---:|:---:|
| `/p/[slug]` SSR | ✅ | ✅ |
| OGP メタタグ動的出力（og:title / og:description / og:image / og:type / og:image:width / og:image:height） | ✅ | ✅ |
| Twitter card メタタグ（summary_large_image / twitter:title / twitter:description / twitter:image） | ✅ | ✅ |
| HTML 内 `<meta name="robots" content="noindex, nofollow">` | ✅ | ✅ |
| `X-Robots-Tag: noindex, nofollow` ヘッダ | ✅ | ✅ |
| `Referrer-Policy: strict-origin-when-cross-origin`（通常ページ） | ✅ | ✅ |
| `Referrer-Policy: no-referrer`（draft / manage / edit） | ✅ | ✅ |
| `/draft/[token]` → 302 + Set-Cookie + redirect to `/edit/{photobook_id}` | ✅ | ✅ |
| `/manage/token/[token]` → 302 + Set-Cookie + redirect to `/manage/{photobook_id}` | ✅ | ✅ |
| Cookie 属性: HttpOnly / Secure / SameSite=Strict / Path=/ | ✅ | ✅ |
| Cookie Max-Age: draft 7日 / manage 1日 | ✅ | ✅ |
| Server Component で Cookie 読取 → "session found" / "session missing" の分岐 | ✅ | ✅ |
| Edge runtime 動作（`x-edge-runtime: 1`） | ✅ | ✅ |
| Cloudflare Pages 互換ビルド（`@cloudflare/next-on-pages`） | — | ✅（5 Edge Function Routes / 1 Middleware / 2 Prerendered） |

#### 検証で見つかった発見

1. **`@cloudflare/next-on-pages` が deprecated**
   - 上記「第一候補の再評価」参照
   - M1 検証としては成立だが、M2 本実装は OpenNext へ切替必要
2. **OGP の `og:image` が dev サーバ URL で焼き込まれる**
   - 出力例: `<meta property="og:image" content="http://localhost:3000/og-sample.png"/>`
   - wrangler preview（port 8788）でも `localhost:3000` のまま
   - 原因: Next.js Metadata API が相対 URL を絶対 URL に展開する際、ベース URL を環境変数等から解決する必要がある
   - **M2 対応**: `metadata.metadataBase = new URL(process.env.NEXT_PUBLIC_BASE_URL)` 指定を本実装に組み込む

#### CLI 検証では未確認の項目（実機ブラウザでのみ確認可能）

未確認 = 不成立ではなく、実機検証が必要なもの:

- 実機 Chrome / Edge / Firefox での動作（HTTP プロトコル仕様準拠は curl 確認済み）
- macOS Safari 実機検証
- **iOS Safari 実機検証（最重要）**
- redirect 後の Cookie 引き継ぎ実体験
- ページ再読み込み後の session 維持
- **24 時間後 / 7 日後の Cookie 残存（ITP 影響評価）**
- DevTools / Web Inspector による Cookie 属性目視確認
- 別オリジンからのリンク遷移で SameSite=Strict が効くことの実体験
- Cloudflare Pages 実環境（`*.pages.dev` ドメイン）でのデプロイ検証
- Backend と異なるホスト構成下での Cookie Domain 動作（U2、Backend PoC と統合）

### 検証結果（OpenNext adapter 版）

OpenNext adapter 版の PoC 検証は **M1 中の追加検証として別途実施**。結果はこのセクションに追記する。

```
（OpenNext 検証担当者が記入）
```

---

## ADR / 設計書へのフィードバック候補

検証結果に応じて、以下を更新する想定。M1 計画 §12 と整合。

- [ ] ADR-0001 §M1 で必要なスパイク → 検証結果セクション追記
- [ ] ADR-0003 §13 未解決事項 U2（Cookie Domain 属性）の解消 or 方針追記
- [ ] ADR-0003 §13 未解決事項 Safari ITP 影響評価 → 結果記録
- [ ] ADR-0003 §13 未解決事項 Middleware vs Server Component の判断
- [ ] M1 計画 §6.1 / §6.2 の代替案発動条件の明確化

---

## トラブルシューティング

### Cloudflare Pages で Set-Cookie が効かない場合

1. `runtime = 'edge'` が各 route ファイルで指定されているか確認
2. `wrangler pages dev` のオプションで `--compatibility-flag=nodejs_compat` が指定されているか確認
3. Next.js 15 App Router の `params` が Promise 型であることに注意（`await params` 必須）

### Safari で Cookie が消える場合

ITP の挙動を疑う。M1 計画 §6.2（案E〜G）の代替案を検討する:

- 案E: Frontend と Backend を共通の独自親ドメイン経由に統一
- 案F: token を URL に残す方式（ADR-0003 全面見直し）
- 案G: Cookie Domain を独自親ドメインで切る

### redirect で Cookie が引き継がれない場合

- 302 ではなく 303 や 307 で試す（`NextResponse.redirect` の第 2 引数）
- `meta refresh` での代替（ただし正規ルートではない）
- redirect 先の Path が Cookie の Path と一致しているか確認（`Path: /` で全パス共有しているはず）

---

## ライセンス / 取扱い

本 PoC は内部検証のみを目的とする。外部公開・本実装流用は禁止。
