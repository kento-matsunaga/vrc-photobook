# frontend/

VRC PhotoBook の **本実装 Frontend**。M2 以降の段階的 PR で機能を追加していく。

## 位置付け

- 本ディレクトリは **M2 本実装**（[`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md)）。
- `harness/spike/frontend/` は M1 PoC、**コードを直接コピペしない**方針（同 §11）。
- 本実装は ADR-0001 / ADR-0003 / 業務知識 v4 / [`.agents/rules/safari-verification.md`](../.agents/rules/safari-verification.md) 準拠。

## 現在のスコープ（PR5）

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

PR5 で **未実装**（後続 PR で追加）:

- draft / manage の token redirect ルート / Cookie 発行（ADR-0003）
- Backend API クライアント / 結合検証ページ
- 画像アップロード UI / Turnstile widget
- Workers Custom Domain / Cloud Run Domain Mapping（M2 早期 §F-1、ドメイン取得後）
- Safari / iPhone Safari の実機確認（後続 PR で UI / Cookie / redirect が入った時点で実施）

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

## ディレクトリ（PR5 時点）

```
frontend/
├── package.json / package-lock.json
├── tsconfig.json
├── next.config.mjs
├── postcss.config.mjs
├── tailwind.config.ts
├── middleware.ts          ← PR5 追加（ヘッダ一本化）
├── open-next.config.ts    ← PR5 追加（OpenNext 設定）
├── wrangler.jsonc         ← PR5 追加（Workers 設定）
├── .gitignore
├── .env.local.example
├── .env.production.example
├── README.md（本書）
├── public/
│   └── .gitkeep
└── app/
    ├── globals.css
    ├── layout.tsx          ← PR5 で metadataBase / robots を追加
    └── page.tsx
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
