# frontend/

VRC PhotoBook の **本実装 Frontend**。M2 以降の段階的 PR で機能を追加していく。

## 位置付け

- 本ディレクトリは **M2 本実装**（[`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md)）。
- `harness/spike/frontend/` は M1 PoC、**コードを直接コピペしない**方針（同 §11）。
- 本実装は ADR-0001 / ADR-0003 / 業務知識 v4 / [`.agents/rules/safari-verification.md`](../.agents/rules/safari-verification.md) 準拠。

## 現在のスコープ（〜 PR10.5）

PR4 で導入したもの:

- Next.js 15 App Router 起動
- Tailwind CSS v3（`app/globals.css` で `@tailwind base/components/utilities`）
- 最小トップページ（`app/page.tsx`）

PR5 で追加したもの:

- `app/layout.tsx`
  - `metadataBase` を `NEXT_PUBLIC_BASE_URL` から構築（未設定時は `http://localhost:3000` フォールバック）
  - `robots: { index: false, follow: false }`（MVP 全 noindex、業務知識 v4 §7.6）
- `middleware.ts` 一本化
  - `X-Robots-Tag: noindex, nofollow` を全レスポンスに付与
  - `/draft` / `/manage` / `/edit` には `Referrer-Policy: no-referrer`（token URL 漏洩対策、ADR-0003）
  - それ以外は `Referrer-Policy: strict-origin-when-cross-origin`
  - **`next.config.mjs` の `headers()` には書かない**（M1 学習: 二重出力事故）
- OpenNext for Cloudflare Workers の最小化
  - `@opennextjs/cloudflare` / `wrangler` を devDependencies に追加
  - `open-next.config.ts`（最小 `defineCloudflareConfig({})`）
  - `wrangler.jsonc`（name=`vrcpb-frontend`, `nodejs_compat` + `global_fetch_strictly_public`, ASSETS binding）
  - npm scripts: `cf:build` / `cf:preview` / `cf:check`
- `.env.production.example` の方針コメント追記（`NEXT_PUBLIC_*` は build 時 inline、Secret を入れない）

PR10 で追加したもの:

- `lib/cookies.ts`: Cookie util
  - `buildSessionCookieName('draft' | 'manage', photobookId)` で `vrcpb_draft_<id>` / `vrcpb_manage_<id>` を組み立て
  - `buildSessionCookieOptions(expiresAt)`: HttpOnly / Secure / SameSite=Strict / Path=/ / Max-Age 計算
  - `getCookieDomain()`: Server-only env `COOKIE_DOMAIN` を読む（未設定なら host-only Cookie）
- `lib/api.ts`: Backend API クライアント
  - `exchangeDraftToken(raw)` / `exchangeManageToken(raw)` で `/api/auth/*-session-exchange` を呼ぶ
  - 401 / 400 / network 失敗を `ApiExchangeError` のラベル付きエラーで表現
  - raw token を含むエラー詳細を呼び出し元に渡さない
- `app/(draft)/draft/[token]/route.ts`: Route Handler
  - Backend `/api/auth/draft-session-exchange` 呼び出し → raw `session_token` を **Frontend 側で `Set-Cookie`**
  - `/edit/<photobook_id>` へ 302 redirect（URL から raw token が消える）
  - 失敗時は `/?reason=invalid_draft_token` へ redirect
- `app/(manage)/manage/token/[token]/route.ts`: Route Handler
  - 同様、`/manage/<photobook_id>` へ 302 redirect
- `app/(draft)/edit/[photobookId]/page.tsx`: 最小ページ（PR11 以降で本実装）
- `app/(manage)/manage/[photobookId]/page.tsx`: 最小ページ
- `.env.local.example` / `.env.production.example`: `COOKIE_DOMAIN` を追加（Server-only env）

PR10 で **未実装**（後続 PR で追加）:

- Server Component での session 検証 / Backend protected API（`/api/photobooks/{id}` 等）
- 編集 UI / 管理 UI / 公開ページ
- 画像アップロード UI / Turnstile widget
- Workers Custom Domain / Cloud Run Domain Mapping（独自ドメイン取得後）
- iPhone Safari 実機確認（独自ドメイン取得後の HTTPS 環境で実施、`safari-verification.md`）

## token / Cookie の運用方針（重要）

- raw `draft_edit_token` / `manage_url_token` は URL path に乗るが、Frontend Route Handler が
  Backend で session 化した後に **302 redirect で URL から消す**
- Backend は `Set-Cookie` を出さない。Cookie 発行は **Frontend Route Handler の専権**
- `COOKIE_DOMAIN` 未設定時は host-only Cookie（localhost 開発時の既定）
- 本番では `COOKIE_DOMAIN=.vrc-photobook.com` を設定して
  `app.vrc-photobook.com` ↔ `api.vrc-photobook.com` 間で Cookie 共有する（U2 解消、
  `docs/plan/m2-early-domain-and-cookie-plan.md` §8）
- 本 router に dummy token / 認証バイパス endpoint は作らない（PR9c / PR10 を通じて確認済）

### Next.js dev server のログに関する注意

`next dev` 標準の access log は URL path をそのまま stdout に出すため、
`/draft/<raw token>` のようなパスにアクセスすると raw token が dev server のターミナルに表示される。
これは **本番（OpenNext / Workers）では発生しない**が、ローカル開発時の運用注意点として明記する。
詳細: [`harness/failure-log/2026-04-26_nextjs-dev-server-url-path-log.md`](../harness/failure-log/2026-04-26_nextjs-dev-server-url-path-log.md)

**手動で /draft/&lt;token&gt; や /manage/token/&lt;token&gt; を dev server に叩く確認は避ける**。
成功経路の Set-Cookie / redirect / Cache-Control / token 非露出の検証は、PR10.5 で追加した
Vitest の Route Handler 単体テスト（`global.fetch` を mock）で行う:

```sh
npm --prefix frontend run test
```

テスト内では raw token を console / log に出さず、`fakeToken43(seed)` で seed 由来の動的な
ダミー文字列を生成する（固定 43 文字 token を repo に書かない）。

### Safari / iPhone Safari 実機確認

- macOS Safari は localhost 開発時にも一定の動作確認が可能（手動実施推奨）
- iPhone Safari への localhost 接続は別 PC からの接続設定が必要なため、
  **独自ドメイン取得後（PR11 / M2 早期 §F-1）** の HTTPS 環境で必ず実機確認する
- 確認項目は [`.agents/rules/safari-verification.md`](../.agents/rules/safari-verification.md) のチェックリスト全項目

## OpenNext / Workers ローカル確認

Workers 実環境への deploy は **行わない**（独自ドメイン取得後に別 PR で実施、`docs/plan/m2-early-domain-and-cookie-plan.md` §8）。
PR5 段階ではローカルでの build / preview のみ確認する。

```sh
npm --prefix frontend run cf:build      # OpenNext で .open-next/ を生成
npm --prefix frontend run cf:preview    # ローカル wrangler dev（http://localhost:8787）
npm --prefix frontend run cf:check      # wrangler deploy --dry-run（実 deploy しない）
```

`cf:preview` 起動中の確認（PR5 で実施した内容）:

```sh
curl -sI http://localhost:8787/         # x-robots-tag: noindex, nofollow / referrer-policy: strict-origin-when-cross-origin
curl -sI http://localhost:8787/draft/x  # referrer-policy: no-referrer
```

`X-Robots-Tag` / `Referrer-Policy` がそれぞれ **1 回だけ** 出ること（重複出力なし）を必ず確認する。

## ローカル起動

```sh
npm --prefix frontend install
npm --prefix frontend run dev
# → http://localhost:3000
```

ビルド確認:

```sh
npm --prefix frontend run build
```

型チェック（任意）:

```sh
npm --prefix frontend run typecheck
```

> WSL では `cd` を使わず `--prefix frontend` で固定（[`.agents/rules/wsl-shell-rules.md`](../.agents/rules/wsl-shell-rules.md)）。

## ディレクトリ（PR10 時点）

```
frontend/
├── package.json / package-lock.json
├── tsconfig.json
├── next.config.mjs
├── postcss.config.mjs
├── tailwind.config.ts
├── middleware.ts                                # PR5: X-Robots-Tag / Referrer-Policy 一本化
├── open-next.config.ts                          # PR5
├── wrangler.jsonc                               # PR5
├── .gitignore
├── .env.local.example
├── .env.production.example
├── README.md（本書）
├── lib/                                         # PR10: Server-side util
│   ├── api.ts                                   # Backend API client（fetch wrapper）
│   └── cookies.ts                               # Cookie util（HttpOnly / Secure / SameSite=Strict）
├── public/
│   └── .gitkeep
└── app/
    ├── globals.css
    ├── layout.tsx                               # PR5: metadataBase / robots
    ├── page.tsx
    ├── (draft)/                                 # PR10
    │   ├── draft/[token]/route.ts               # token → session 交換 → /edit redirect
    │   └── edit/[photobookId]/page.tsx          # draft 編集ページ最小実装
    └── (manage)/                                # PR10
        ├── manage/token/[token]/route.ts        # token → session 交換 → /manage redirect
        └── manage/[photobookId]/page.tsx        # 管理ページ最小実装
```

PR6 以降の構造拡張は [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §5 / §12 を参照。

## 環境変数の方針

- `.env.local` / `.env.production` は **git 管理外**（`.gitignore` で除外済み）。
- 値が必要になる PR では `.env.local.example` / `.env.production.example` をコピーして作成する運用。
- **`NEXT_PUBLIC_*` は Next.js のビルド時に bundle に inline される公開値**。Secret 値は絶対に `NEXT_PUBLIC_` プレフィックスで渡さない。
- wrangler の runtime env vars は Next.js の `process.env.NEXT_PUBLIC_*` には届かないため、Workers デプロイでも build 時の `.env.production` が経路となる（M1 学習: [`harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md`](../harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md) と同セッションで確認）。

## CI

PR4 で導入した最小 GitHub Actions（[`.github/workflows/frontend-ci.yml`](../.github/workflows/frontend-ci.yml)）:

- `npm ci`
- `npm run build`
- `npm run typecheck`

`lint` / `prettier` / `cf:build` の CI 化 / `wrangler deploy` は **PR6 以降**で段階的に追加する。
PR5 では `cf:build` / `cf:preview` はローカル確認のみで、CI には載せていない。

## セキュリティ方針（PR5 時点で守る）

- Secret 値を本ディレクトリに書かない（`.env.example` 系もキー名のみ、`wrangler.jsonc` にも値を書かない）
- `NEXT_PUBLIC_*` は **公開値前提**。Secret は絶対に `NEXT_PUBLIC_` プレフィックスで渡さない（build 時 inline されて bundle に焼き込まれるため）
- raw token / 管理 URL / Cookie 値を扱わない（後続 PR の Cookie / token 取り扱いは ADR-0003 / [`.agents/rules/security-guard.md`](../.agents/rules/security-guard.md) 準拠）
- Cookie / redirect / OGP / レスポンスヘッダの変更時は [`.agents/rules/safari-verification.md`](../.agents/rules/safari-verification.md) に従い Safari / iPhone Safari の実機確認を必須とする

## 関連ドキュメント

- [M2 実装ブートストラップ計画](../docs/plan/m2-implementation-bootstrap-plan.md)
- [プロジェクト全体ロードマップ](../harness/work-logs/2026-04-26_project-roadmap-overview.md)
- [業務知識 v4](../docs/spec/vrc_photobook_business_knowledge_v4.md)
- [ADR-0001 技術スタック](../docs/adr/0001-tech-stack.md)
- [ADR-0003 フロントエンド認可フロー](../docs/adr/0003-frontend-token-session-flow.md)
