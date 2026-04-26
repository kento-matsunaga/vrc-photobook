# frontend/

VRC PhotoBook の **本実装 Frontend**。M2 以降の段階的 PR で機能を追加していく。

## 位置付け

- 本ディレクトリは **M2 本実装**（[`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md)）。
- `harness/spike/frontend/` は M1 PoC、**コードを直接コピペしない**方針（同 §11）。
- 本実装は ADR-0001 / ADR-0003 / 業務知識 v4 / [`.agents/rules/safari-verification.md`](../.agents/rules/safari-verification.md) 準拠。

## 現在のスコープ（PR4）

- Next.js 15 App Router 起動
- Tailwind CSS v3（`app/globals.css` で `@tailwind base/components/utilities`）
- 最小トップページ（`app/page.tsx`）
- `app/layout.tsx` の最小 metadata（`title` / `description` のみ）

PR4 で **未実装**（後続 PR で追加）:

- `metadataBase`（OGP `og:image` の絶対 URL 解決）/ `noindex` メタ / `Referrer-Policy` / `X-Robots-Tag` の出し分け（PR5 で `middleware.ts` 一本化）
- OpenNext (`@opennextjs/cloudflare`) / `wrangler.jsonc` / `open-next.config.ts`（PR5）
- draft / manage の token redirect ルート / Cookie 発行
- Backend API クライアント / 結合検証ページ
- 画像アップロード UI / Turnstile widget
- Workers Custom Domain / Cloud Run Domain Mapping（M2 早期 §F-1、ドメイン取得後）
- Safari / iPhone Safari の実機確認（PR5 以降の機能追加とあわせて）

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

## ディレクトリ（PR4 時点）

```
frontend/
├── package.json / package-lock.json
├── tsconfig.json
├── next.config.mjs
├── postcss.config.mjs
├── tailwind.config.ts
├── .gitignore
├── .env.local.example
├── .env.production.example
├── README.md（本書）
├── public/
│   └── .gitkeep
└── app/
    ├── globals.css
    ├── layout.tsx
    └── page.tsx
```

PR5 以降の構造拡張は [`docs/plan/m2-implementation-bootstrap-plan.md`](../docs/plan/m2-implementation-bootstrap-plan.md) §5 を参照。

## 環境変数の方針

- `.env.local` / `.env.production` は **git 管理外**（`.gitignore` で除外済み）。
- 値が必要になる PR では `.env.local.example` / `.env.production.example` をコピーして作成する運用。
- **`NEXT_PUBLIC_*` は Next.js のビルド時に bundle に inline される公開値**。Secret 値は絶対に `NEXT_PUBLIC_` プレフィックスで渡さない。
- wrangler の runtime env vars は Next.js の `process.env.NEXT_PUBLIC_*` には届かないため、Workers デプロイでも build 時の `.env.production` が経路となる（M1 学習: [`harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md`](../harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md) と同セッションで確認）。

## CI

PR4 で最小 GitHub Actions を追加:

- `npm ci`
- `npm run build`
- `npm run typecheck`

`lint` / `prettier` / `OpenNext build` / `wrangler deploy` は **PR5 以降**で段階的に追加する。

## セキュリティ方針（PR4 時点で守る）

- Secret 値を本ディレクトリに書かない（`.env.example` 系もキー名のみ）
- raw token / 管理 URL / Cookie 値を扱わない（PR5 以降の Cookie / token 取り扱いは ADR-0003 / [`.agents/rules/security-guard.md`](../.agents/rules/security-guard.md) 準拠）

## 関連ドキュメント

- [M2 実装ブートストラップ計画](../docs/plan/m2-implementation-bootstrap-plan.md)
- [プロジェクト全体ロードマップ](../harness/work-logs/2026-04-26_project-roadmap-overview.md)
- [業務知識 v4](../docs/spec/vrc_photobook_business_knowledge_v4.md)
- [ADR-0001 技術スタック](../docs/adr/0001-tech-stack.md)
- [ADR-0003 フロントエンド認可フロー](../docs/adr/0003-frontend-token-session-flow.md)
