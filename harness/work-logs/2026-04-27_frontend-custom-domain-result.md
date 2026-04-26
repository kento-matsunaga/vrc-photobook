# 2026-04-27 Frontend Custom Domain (`app.vrc-photobook.com`) 実施結果（PR15）

## 概要

`docs/plan/m2-frontend-workers-deploy-plan.md` PR15 計画に基づき、
`vrcpb-frontend` Worker に **`app.vrc-photobook.com`** を Custom Domain として紐付け、
HTTPS 疎通・middleware ヘッダ・Route Handler の reason redirect・Backend
(`api.vrc-photobook.com`) への fetch 成立を確認した。

- 実施日時: 2026-04-27 02:06〜02:08 JST（dig / curl 検証〜logs 確認まで約 2 分）
- Custom Domain: **`app.vrc-photobook.com`** → `vrcpb-frontend` Worker
- 証明書発行元: **Let's Encrypt E8** （Cloudflare Universal SSL 経由）
- 証明書 subject: `vrc-photobook.com`（SAN で `*.vrc-photobook.com` をカバー）
- 証明書有効期限: **2026-07-25**
- 既存 deployment Version ID: `9736fe88-51f3-4fa8-9038-8112627b12e5`（PR14 と同一、再 deploy なし）

## 前提

- PR14 で `vrcpb-frontend` Worker が `https://vrcpb-frontend.k-matsunaga-biz.workers.dev` で稼働中
- DNS は Cloudflare Registrar 管理の `vrc-photobook.com`（PR11 で取得済）
- Custom Domain 追加操作は Cloudflare Dashboard 上の手動 GUI 操作（ユーザー実施）
- Claude Code 側は **検証のみ**（dig / curl / logs）

## タイムライン

| 時刻 (JST) | 出来事 |
|---|---|
| 02:05 頃 | ユーザーが Cloudflare Dashboard で `app.vrc-photobook.com` Custom Domain を追加 |
| 02:06 | `app.vrc-photobook.com` が `vrcpb-frontend` の Custom Domain として表示確認（ユーザー） |
| 02:06 | dig 検証（A レコード Cloudflare anycast IP に解決） |
| 02:06 | curl 検証（HTTPS / 200 / 不正 token redirect / sensitive path ヘッダ） |
| 02:07 | Backend (`api.vrc-photobook.com`) fetch 成立確認（gcloud logging） |
| 02:07 | Backend logs 漏洩 grep 完了 |
| 02:08 | 作業ログ作成 |

## DNS 検証

```
$ dig +short app.vrc-photobook.com
104.21.82.154
172.67.158.239
```

- Cloudflare anycast IP に解決 ✅
- DNS only / Proxy off の前提通り Workers 側で TLS 終端

## HTTPS / 証明書検証

```
HTTP/2 200
server: cloudflare
content-type: text/html; charset=utf-8
x-robots-tag: noindex, nofollow
referrer-policy: strict-origin-when-cross-origin
```

- 証明書: Let's Encrypt **E8**（Cloudflare Universal SSL）
- subject: `CN=vrc-photobook.com`（SAN で `*.vrc-photobook.com` をカバー）
- 有効期限: **2026-07-25**
- 90 日自動更新（Cloudflare 任せ）

## 検証結果

### `GET /` （非 sensitive path）

- HTTP/2 200 / `Content-Type: text/html`
- `x-robots-tag: noindex, nofollow` ✅
- `referrer-policy: strict-origin-when-cross-origin` ✅
- ヘッダ重複なし（`grep -c "x-robots-tag"` = 1、`referrer-policy` = 1）

### `GET /draft/<invalid 43 chars>` (`AAAA...`)

- HTTP/2 **302** ✅
- `location: https://app.vrc-photobook.com/?reason=invalid_draft_token` ✅
  - raw token は Location に含まれない（reason の固定 query のみ）
- `cache-control: no-store` ✅
- `referrer-policy: no-referrer`（sensitive path）✅
- `x-robots-tag: noindex, nofollow` ✅
- **Set-Cookie ヘッダなし** ✅（不正 token 経路）

### `GET /manage/token/<invalid>` (`BBBB...`)

- HTTP/2 **302** ✅
- `location: https://app.vrc-photobook.com/?reason=invalid_manage_token` ✅
- 同様の sensitive path ヘッダ、Set-Cookie 無し ✅

### `GET /edit/<photobook_id>` / `GET /manage/<photobook_id>`

- HTTP/2 **200** + 最小ページ ✅
- `referrer-policy: no-referrer`（sensitive path） ✅

### Backend (`api.vrc-photobook.com`) fetch 成立確認

`gcloud logging read` で `vrcpb-api` の `/api/auth/*-session-exchange` への
POST リクエスト履歴を確認:

| timestamp (Z) | endpoint | status |
|---|---|---|
| 17:06:35 | manage-session-exchange | **401** |
| 17:06:34 | draft-session-exchange | 401 |
| 17:06:34 | draft-session-exchange | 401 |
| 17:06:34 | draft-session-exchange | 401 |
| 17:06:32 | manage-session-exchange | **401** |
| 17:06:32 | draft-session-exchange | 401 |

- `app.vrc-photobook.com` (Workers Custom Domain) → `api.vrc-photobook.com`
  (Cloud Run Domain Mapping) への HTTPS fetch が成立 ✅
- 期待通り Backend が **401** を返している（不正 token）
- URL path に raw token は記録されていない（POST body のため Cloud Run access log には記録されない）

### Logs 漏洩 grep

```
gcloud run services logs read vrcpb-api --region=asia-northeast1 --limit=200 |
  grep -iE "(SECRET|API_KEY|PASSWORD|PRIVATE|sk_live|sk_test|draft_edit_token|
            manage_url_token|session_token|set-cookie|DATABASE_URL=)"
```

→ **マッチなし**（漏洩なし） ✅

## 注意点

### Cookie 属性の最終検証は PR16 / PR17 で

PR15 段階では「不正 token 経路で Set-Cookie が出ない」「302 redirect が成立する」「Backend fetch
が成立する」までを確認した。

`COOKIE_DOMAIN=.vrc-photobook.com` が実 token 経路で **Domain=.vrc-photobook.com** + **HttpOnly +
Secure + SameSite=Strict + Path=/** で発行されることの検証は、実 token 結合 (PR16) と
Safari / iPhone Safari 実機検証 (PR17) で行う。

### Custom Domain 追加は GUI 手動

`wrangler` CLI には Custom Domain を追加する API が無い（あるいは限定的）ため、
Cloudflare Dashboard 上の手動 GUI 操作で追加した。今後の Frontend Custom Domain 追加も
同様の手順を想定する。

## 実施しなかったこと

- Frontend 再 deploy（Custom Domain 追加のみで Worker 自体は PR14 deploy のまま）
- `wrangler.jsonc` 変更（Custom Domain は Dashboard 側設定で wrangler.jsonc には現れない）
- Backend (Cloud Run / Cloud SQL) 変更
- 実 token を使った Cookie 発行 200 経路結合（PR16）
- Safari / iPhone Safari 実機確認（PR17）
- SendGrid / Turnstile / R2 / 編集 UI 等
- Cloud SQL `vrcpb-api-verify` 削除（残置中、PR17 完了後の判断まで継続）
- DATABASE_URL Secret 削除
- 既存 spike Workers / Cloud Run 削除
- raw token / Cookie / Secret / DATABASE_URL の本書・チャット・コミットメッセージへの記録
- debug endpoint 追加 / dummy token 経路追加

## 切戻し手順（参考、本書では実施しない）

### Custom Domain 削除

Cloudflare Dashboard → Workers & Pages → `vrcpb-frontend` → Settings → Triggers →
Custom Domains → `app.vrc-photobook.com` の右側「...」→ Delete（**ユーザー手動**）

削除後は `vrcpb-frontend.k-matsunaga-biz.workers.dev` のみ稼働、`app.vrc-photobook.com`
は名前解決はするが Workers 側ルーティングが消えて 404 / 521 等になる。

### DNS 影響について

`app.vrc-photobook.com` は Workers Custom Domain 削除と同時に Cloudflare の DNS レコード
(自動生成された Worker Route 用) も削除される（Custom Domain 削除動作の一部）。
Frontend 全停止という事態にはならないが、`app.vrc-photobook.com` への直接アクセスは不能になる。

## 費用

- Custom Domain 追加自体は無料（Cloudflare Workers Free プラン内）
- Workers リクエストは無料枠内（100k/日）
- Cloud SQL 残置継続中、累計経過 ~5 時間 / ~¥11

## 次のステップ

PR16: Frontend ↔ Backend 実 token 結合
- repo 外で token-gen Go プログラムを実行（`backend/internal/photobook/internal/usecase`
  の internal package 制約により、`backend/_tokengen/main.go` のような repo 内の小さな
  CLI を一時的に追加するか、repo 外の独立プログラムとして実行する）
- 発行した draft / manage raw token で `https://app.vrc-photobook.com/draft/<token>`
  にアクセス、HttpOnly + Secure + SameSite=Strict + Path=/ + Domain=.vrc-photobook.com
  の Set-Cookie が発行されることを **DevTools** で確認
- raw token / session_token は **チャット・work-log・コミットメッセージに残さない**

PR17: Safari / iPhone Safari 実機確認

PR17 完了後: Cloud SQL `vrcpb-api-verify` の保持/削除判定（必須判断）

## 関連

- [PR13 計画書](../../docs/plan/m2-frontend-workers-deploy-plan.md)
- [PR14 Frontend Workers Deploy 結果](./2026-04-27_frontend-workers-deploy-result.md)
- [PR12 Backend Domain Mapping 実施結果](./2026-04-27_backend-domain-mapping-result.md)
- [Post-deploy Final Roadmap §A](./2026-04-27_post-deploy-final-roadmap.md)
- [`frontend/wrangler.jsonc`](../../frontend/wrangler.jsonc) / [`frontend/middleware.ts`](../../frontend/middleware.ts)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
