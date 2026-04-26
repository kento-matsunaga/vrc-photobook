# 2026-04-27 Frontend Workers Deploy 実施結果（PR14）

## 概要

`docs/plan/m2-frontend-workers-deploy-plan.md` PR13 計画に基づき、`frontend/` を
Cloudflare Workers にデプロイし、Workers URL 上で middleware ヘッダ・
Route Handler の failure redirect・Backend fetch 成立を確認した。

- 実施日時: 2026-04-27 01:46〜01:47 JST（deploy 〜 検証完了まで約 1 分）
- Workers project: **`vrcpb-frontend`**
- Workers URL: **`https://vrcpb-frontend.k-matsunaga-biz.workers.dev`**
- Deployment Version ID: `9736fe88-51f3-4fa8-9038-8112627b12e5`
- COOKIE_DOMAIN 注入方式: 案 A（`.env.production` build 時 inline）

## タイムライン

| 時刻 (JST) | 出来事 |
|---|---|
| 01:43 | wrangler whoami / version / .env.production 不在 確認 |
| 01:44 | `frontend/.env.production` 生成（3 行、Secret なし） |
| 01:44〜01:46 | typecheck / test (16/16) / next build / cf:build すべて成功 |
| 01:46 | `wrangler deploy` 1 回目失敗（cwd 起点の相対パス解決問題） |
| 01:46 | サブシェル `( cd frontend && wrangler deploy )` で再実行 → 成功 |
| 01:47 | Workers URL での疎通確認、Backend fetch 成立確認、logs 漏洩 grep 完了 |

## build 結果

| 項目 | 結果 |
|---|---|
| typecheck | OK |
| test (vitest) | 3 files / 16 tests PASS（duration 733ms） |
| next build | OK / Middleware 34.1 kB / 4 routes 生成 |
| cf:build (OpenNext) | `Worker saved in '.open-next/worker.js'` 成功 |

## .env.production の内容（公開値のみ、Secret なし）

```
NEXT_PUBLIC_BASE_URL=https://app.vrc-photobook.com
NEXT_PUBLIC_API_BASE_URL=https://api.vrc-photobook.com
COOKIE_DOMAIN=.vrc-photobook.com
```

- `frontend/.gitignore` で `.env.production` 除外確認済（`git check-ignore -v` で確認）
- COOKIE_DOMAIN は **NEXT_PUBLIC_ 接頭辞無し**、Server-only env として OpenNext build 時に
  Server bundle に inline される

## deploy 結果

```
Total Upload: 4259.57 KiB / gzip: 883.38 KiB
Worker Startup Time: 33 ms
Bindings: env.ASSETS (Assets)
Uploaded vrcpb-frontend (9.14 sec)
Deployed vrcpb-frontend triggers (1.38 sec)
URL: https://vrcpb-frontend.k-matsunaga-biz.workers.dev
Version ID: 9736fe88-51f3-4fa8-9038-8112627b12e5
```

## 検証結果

### `GET /`

- HTTP/2 200 / Content-Type: text/html
- `x-robots-tag: noindex, nofollow` ✅
- `referrer-policy: strict-origin-when-cross-origin` ✅（非 sensitive path）
- 二重出力なし（`grep -c "x-robots-tag"` = 1、`referrer-policy` = 1）

### `GET /draft/<invalid 43 chars>` (`AAAA...`)

- HTTP/2 **302** ✅
- `location: https://app.vrc-photobook.com/?reason=invalid_draft_token` ✅
  - raw token は Location に含まれない（reason の固定 query のみ）
- `cache-control: no-store` ✅
- `referrer-policy: no-referrer`（sensitive path）✅
- `x-robots-tag: noindex, nofollow`
- **Set-Cookie ヘッダなし** ✅（不正 token 経路）

### `GET /manage/token/<invalid>` (`BBBB...`)

- HTTP/2 **302** ✅
- `location: https://app.vrc-photobook.com/?reason=invalid_manage_token` ✅
- 同様の属性、Set-Cookie 無し ✅

### `GET /edit/<photobook_id>` / `GET /manage/<photobook_id>`

- HTTP/2 **200** + 最小ページ ✅
- `referrer-policy: no-referrer`（sensitive path）

### Backend fetch 成立確認

`gcloud logging read` で `vrcpb-api` の `/api/auth/*-session-exchange` への
POST リクエスト履歴を確認:

| timestamp (Z) | endpoint | status |
|---|---|---|
| 16:47:35 | draft-session-exchange | **401** |
| 16:47:34 | draft-session-exchange | 401 |
| 16:47:33 | draft-session-exchange | 401 |
| 16:47:19 | manage-session-exchange | **401** |
| 16:47:18 | draft-session-exchange | 401 |

Workers から `https://api.vrc-photobook.com` への HTTPS fetch が成立、
Backend が 401 を返している。**URL path に raw token は記録されていない**
（POST body のため Cloud Run access log には記録されず）。

### Logs 漏洩 grep

```
gcloud run services logs read vrcpb-api --region=asia-northeast1 --limit=200 |
  grep -iE "(SECRET|API_KEY|PASSWORD=|PRIVATE|sk_live|sk_test|draft_edit_token|
            manage_url_token|session_token|set-cookie|DATABASE_URL=)"
```

→ **マッチなし**（漏洩なし） ✅

Frontend code 内の `console.log/error/warn/info/debug`:
- `frontend/lib/` / `frontend/app/(draft)/` / `frontend/app/(manage)/` / `frontend/middleware.ts`
  全 grep でマッチなし ✅

curl response body にも raw token は含まれない（302 redirect は body 空）。

## 注意点

### Cookie 属性の検証は PR15 / PR16 で

Workers URL `*.workers.dev` では `COOKIE_DOMAIN=.vrc-photobook.com` の Cookie は
ドメインミスマッチでブラウザに拒否される可能性が高いため、**Cookie 属性の検証は**
**`app.vrc-photobook.com` Custom Domain 設定後の PR15 / PR16 で実施**。
本 PR14 では「不正 token 経路で Set-Cookie が出ない / 302 redirect が成立する / Backend fetch
が成立する」までを確認した。

### deploy 1 回目の失敗（記録のみ、再発防止のため）

- 原因: `npm --prefix frontend exec -- wrangler deploy` を repo root から実行すると
  wrangler の cwd が repo root になり、`wrangler.jsonc` の相対パス `.open-next/assets`
  を `repo_root/.open-next/assets` として探すため失敗
- 対処: `( cd frontend && wrangler deploy )` のサブシェル経由で実行（wsl-shell-rules.md §1
  で許容のサブシェルパターン）
- 後続の wrangler コマンドは同様にサブシェル経由を推奨。あるいは
  `frontend/package.json` の scripts に `"deploy": "wrangler deploy"` を追加する案もあるが、
  PR13 §13.2 の「deploy script 追加しない」判断により未採用

## 実施しなかったこと

- `app.vrc-photobook.com` Workers Custom Domain 設定（PR15）
- Cloudflare DNS の `app` レコード追加（PR15 で Workers Custom Domain が自動作成）
- Backend (Cloud Run / Cloud SQL) 変更
- 実 token を使った Cookie 発行 200 経路結合（PR16）
- Safari / iPhone Safari 実機確認（PR17）
- SendGrid / Turnstile / R2 / 編集 UI 等
- Cloud SQL `vrcpb-api-verify` 削除（残置中、PR17 完了後の判断まで継続）
- DATABASE_URL Secret 削除
- 既存 spike Workers / Cloud Run 削除
- raw token / Cookie / Secret / DATABASE_URL の本書・チャット・コミットメッセージへの記録
- debug endpoint 追加 / dummy token 経路追加
- `.env.production` の git track（gitignore 維持）

## 切戻し手順（参考、本書では実施しない）

### deployments 履歴確認

```sh
( cd frontend && wrangler deployments list )
```

### 旧 deployment へ rollback

```sh
( cd frontend && wrangler rollback <旧 deployment ID> )
```

### env を修正して再 deploy

```sh
vim frontend/.env.production
npm --prefix frontend run cf:build
( cd frontend && wrangler deploy )
```

### DNS 影響について

PR14 段階では `app.vrc-photobook.com` Custom Domain を設定していないため、
Workers URL（`*.workers.dev`）のみが影響を受ける。Frontend 全停止という事態にはならない。

## 費用

- Workers deploy 自体は無料枠内（リクエスト 100k/日 まで無料）
- Cloud SQL 残置継続中、累計経過 ~3 時間 / ~¥7

## 次のステップ

PR15: `app.vrc-photobook.com` Workers Custom Domain 設定
- Cloudflare Dashboard → Workers & Pages → vrcpb-frontend → Settings → Triggers → Custom Domains（**ユーザー手動**）
- DNS / 証明書は Cloudflare 自動
- HTTPS 疎通: `curl https://app.vrc-photobook.com/`
- 不正 token redirect で reason redirect が機能 + Cookie 属性確認の準備

## 関連

- [PR13 計画書](../../docs/plan/m2-frontend-workers-deploy-plan.md)
- [Backend Domain Mapping 実施結果（PR12）](./2026-04-27_backend-domain-mapping-result.md)
- [Post-deploy Final Roadmap §A](./2026-04-27_post-deploy-final-roadmap.md)
- [`frontend/wrangler.jsonc`](../../frontend/wrangler.jsonc) / [`frontend/middleware.ts`](../../frontend/middleware.ts)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
