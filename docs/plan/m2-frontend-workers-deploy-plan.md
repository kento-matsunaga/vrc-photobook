# M2 Frontend Workers Deploy 計画（PR13）

> 作成日: 2026-04-27
> 位置付け: `frontend/` を Cloudflare Workers（OpenNext）に deploy する **直前** の計画書。
> **本書段階では deploy / Custom Domain / Cloudflare DNS 変更を一切実行しない**。
>
> 上流参照（必読、本書では再記載しない）:
> - [`harness/work-logs/2026-04-27_post-deploy-final-roadmap.md`](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md) §A PR13〜PR17、§F design 参照ルール
> - [`harness/work-logs/2026-04-27_backend-domain-mapping-result.md`](../../harness/work-logs/2026-04-27_backend-domain-mapping-result.md)（Backend HTTPS 疎通完了）
> - [`docs/plan/m2-domain-mapping-execution-plan.md`](./m2-domain-mapping-execution-plan.md) §5 / §8 / §11
> - [`docs/plan/m2-photobook-session-integration-plan.md`](./m2-photobook-session-integration-plan.md) §6 / §12
> - [`docs/adr/0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md)
> - [`frontend/wrangler.jsonc`](../../frontend/wrangler.jsonc) / [`frontend/open-next.config.ts`](../../frontend/open-next.config.ts) / [`frontend/middleware.ts`](../../frontend/middleware.ts)
> - [`frontend/lib/cookies.ts`](../../frontend/lib/cookies.ts) / [`frontend/lib/api.ts`](../../frontend/lib/api.ts)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
> - [Cloudflare Workers Custom Domain 公式](https://developers.cloudflare.com/workers/configuration/routing/custom-domains/)
> - [OpenNext for Cloudflare 公式](https://opennext.js.org/cloudflare)

---

## 0. 本計画書の使い方

- 本書は **計画書のみ**。Workers deploy / Custom Domain / Cloudflare DNS 変更は **行わない**
- §3 / §4 で env 注入方式を確定 → §5 で wrangler / OpenNext 構成確認 → §6 で deploy 手順案 → §11 のユーザー判断項目に答えてから PR14（実施 PR）に進む
- 実施 PR は本書の手順を 1 つずつ実行、各ステップで `curl` / `wrangler` / `dig` で客観確認
- PR13 は「実装ではなく事故防止の手順固定」。特に `COOKIE_DOMAIN` と `NEXT_PUBLIC_*` の境界を本書で確定する

---

## 1. 目的

- `frontend/` を Cloudflare Workers（既存の OpenNext + wrangler 設定、PR5 で確立）に deploy
- 公開 URL（暫定 `https://vrcpb-frontend.<account>.workers.dev`）で:
  - middleware ヘッダ（X-Robots-Tag / Referrer-Policy）正常出力
  - `/draft/<不正 token>` / `/manage/token/<不正 token>` の 302 redirect が動く
  - Backend `https://api.vrc-photobook.com` への fetch が成立（401 が返ってくる）
- まだ `app.vrc-photobook.com` Custom Domain は **設定しない**（PR15 で実施）
- まだ実 token での 200 経路結合は **行わない**（PR16 で実施、§9 にテンプレを文書化のみ）

---

## 2. 対象範囲

### 対象（本書で扱う計画 / PR14 以降で実行）

- `frontend/.env.production` の値設計（**git 管理外、PR14 で生成**）
- `npm --prefix frontend run cf:build`（OpenNext で `.open-next/` 生成）
- `wrangler deploy`（Workers project `vrcpb-frontend` への初回 deploy）
- Workers URL での疎通確認（`curl` ヘッダ / 302 redirect / Backend fetch 成立）
- env 注入方式の確定
- 切戻し手順

### 対象外（後続 PR）

- `app.vrc-photobook.com` Custom Domain 設定（PR15）
- Cloudflare DNS 変更（PR15 で Workers Custom Domain が自動作成）
- Backend deploy / Cloud Run 変更（既に PR12 完了で安定稼働）
- 実 token を使った Cookie 発行 200 経路結合（PR16）
- Safari / iPhone Safari 実機確認（PR17）
- R2 / Turnstile / SendGrid / 編集 UI 等の本実装機能

---

## 3. 環境変数方針（最重要）

### 3.1 設計値

| 変数 | 値 | 種別 |
|---|---|---|
| `NEXT_PUBLIC_BASE_URL` | `https://app.vrc-photobook.com` | **公開**（client bundle に inline されてよい） |
| `NEXT_PUBLIC_API_BASE_URL` | `https://api.vrc-photobook.com` | **公開**（同上） |
| `COOKIE_DOMAIN` | `.vrc-photobook.com` | **Server-only**（Client Component から見えてはいけない） |

### 3.2 原則

- `NEXT_PUBLIC_*` は build 時に **Client bundle へ inline**。**`COOKIE_DOMAIN` には絶対に `NEXT_PUBLIC_` を付けない**
- `COOKIE_DOMAIN` は **Secret ではない**（`.vrc-photobook.com` は公開ドメイン）が、**Server-only env として扱う**:
  - Cookie 属性の制御は Server-side ロジック (Route Handler / Server Component) で行う
  - Client Component が読む必要はない
  - Server / Client の境界を明確化することで、将来 Cookie 属性に Secret 性が混じったときの再設計を回避
- `frontend/.env.production` は **git 管理外**（`.gitignore` で除外済、`.env.production.example` のみ track）
- Frontend Route Handler (`frontend/lib/cookies.ts` の `getCookieDomain()`) は `process.env.COOKIE_DOMAIN ?? ""` を読む実装で、PR10 / PR10.5 で動作確認済

### 3.3 Client Component が誤って読むリスクの遮断

- `frontend/lib/cookies.ts` は Server-side からのみ呼ばれることを意図しているが、**型システム / ESLint で Client から呼べないように制約することは現状していない**
- `'use client'` を付けたファイルから `import { getCookieDomain } from "@/lib/cookies"` すると、Next.js が build 時に `process.env.COOKIE_DOMAIN` を Client bundle へ inline しようとする → **`NEXT_PUBLIC_*` 接頭辞でないため inline されず undefined になる**（Next.js の安全装置）
- つまり「Client から誤って呼んでも空文字に fallback する」だけで漏洩はしないが、Cookie Domain が抜けるという挙動バグになる
- 対策（後続 PR で検討）: ESLint plugin or `'server-only'` import で Server-only の強制

---

## 4. COOKIE_DOMAIN の Workers 注入方式（比較と推奨）

### 4.1 候補

| 案 | 内容 |
|---|---|
| **案 A** | **ローカル `frontend/.env.production` を deploy 時に作成** → `next build` / `cf:build` が `process.env.COOKIE_DOMAIN` を Server-side bundle に inline |
| 案 B | `frontend/wrangler.jsonc` の `vars` セクションに `COOKIE_DOMAIN` を書く |
| 案 C | `wrangler secret put COOKIE_DOMAIN` で Workers Secrets に登録 |

### 4.2 比較

| 観点 | 案 A（.env.production build inline） | 案 B（wrangler vars）| 案 C（Workers Secrets） |
|---|---|---|---|
| Server-only 扱い | ✅（Server bundle のみに inline、Client から読めない） | ⚠️ Workers runtime env として渡る、Next.js Server Component から `process.env.COOKIE_DOMAIN` で読める | ⚠️ runtime env と同等扱い |
| OpenNext / Next Route Handler で確実に読めるか | ✅ build 時 inline で `process.env.COOKIE_DOMAIN` が Server bundle に静的に埋まる | ⚠️ OpenNext が wrangler vars を Workers runtime env から渡すが、Next.js の `process.env` 経由で見える保証はランタイム実装依存 | 同上 |
| build 時 inline と runtime env の違い | build 時で確定、deploy 後の変更は再 build 必須 | runtime で wrangler.jsonc から読む、deploy 後の変更は wrangler 再 deploy | wrangler secret put で更新後 wrangler 再 deploy |
| Secret ではない値を Secret 扱いする必要性 | なし（公開値ではない Server-only env として扱うのは妥当） | なし | **過剰**（Workers Secrets は本来 API key 等のため、公開値を入れるのはミスリーディング） |
| 誤って Client へ露出するリスク | ❌ なし（NEXT_PUBLIC_ 接頭辞でないため Next.js が Client bundle に inline しない） | ⚠️ 同上 | ⚠️ 同上 |
| 運用の簡単さ | ✅ 最もシンプル、`.env.production` を生成 → build → deploy | ⚠️ wrangler.jsonc と .env の二重管理 | ⚠️ wrangler secret 管理が増える |
| 値の変更頻度 | 低（ドメインが変わるとき以外不変） | 同 | 同 |

### 4.3 推奨: **案 A（`.env.production` を deploy 時に作成 → build 時 inline）**

理由:

- Server-only env としての扱いが最もクリーン（Next.js / OpenNext の標準フロー）
- PR5 で確立した build フロー (`cf:build`) と整合
- Cookie Domain の値は変更頻度が低く、deploy ごとに `.env.production` を再生成しても運用負荷低
- `wrangler.jsonc` を repo に track しているため、`vars` に値を書くと git 履歴に残る（公開値とはいえ git に明示的に書く必要は薄い）
- Secret ではない公開値を Workers Secrets API に入れると将来の運用ルールが曖昧になる

### 4.4 案 A の補強策

- `.gitignore` で `frontend/.env.production` が除外されていることを deploy 前に再確認
- `frontend/.env.production.example` には **キー名 + 用途コメントのみ**（PR10 で更新済）
- deploy 後の checklist:
  - `wrangler deployments list` で deployment ID を控える
  - `frontend/.env.production` の中身が想定の 3 行（NEXT_PUBLIC_BASE_URL / NEXT_PUBLIC_API_BASE_URL / COOKIE_DOMAIN）であることを deploy 直前に再確認

### 4.5 案 A が成立しなかった場合のフォールバック

OpenNext の build inline が想定通り効かない場合（例えば Server Action / Edge Runtime での `process.env` 解決問題）、**案 C → 案 B の順で fallback** する:

- 案 C: `wrangler secret put COOKIE_DOMAIN` で登録（Server-only として届く）
- 案 B: `wrangler.jsonc` の `vars` に書く（公開値だが Server runtime で読めれば許容）

判断ポイント:
- 案 A で deploy 後に `https://<workers url>/draft/<不正 token>` の Set-Cookie ヘッダ確認
- もし `Domain=` が付かない場合、`getCookieDomain()` が空文字を返している（inline されていない）
- そのときに案 C へ切替え、wrangler secret put で再 deploy

---

## 5. wrangler / OpenNext 構成確認

### 5.1 既存ファイルの確認（PR5 / PR10 で確立）

#### `frontend/wrangler.jsonc`（PR5 で作成済）

- `name`: `vrcpb-frontend`（M2 本実装名、M1 PoC `vrcpb-spike-frontend` とは別）
- `main`: `.open-next/worker.js`
- `compatibility_date`: `2026-04-01`
- `compatibility_flags`: `["nodejs_compat", "global_fetch_strictly_public"]`
- `assets`: `directory=.open-next/assets`, `binding=ASSETS`
- `observability`: `enabled=true`
- `vars`: 設定なし（**案 A 採用のため空のまま**）

PR13 段階で wrangler.jsonc に変更を入れる必要は **無い**。

#### `frontend/open-next.config.ts`（PR5 で作成済）

- `defineCloudflareConfig({})` の最小構成
- 追加 binding（incremental cache / queue / D1 / KV / R2）は無し
- PR13 段階で変更不要

#### `frontend/middleware.ts`（PR5 で作成済）

- `X-Robots-Tag: noindex, nofollow` を全レスポンスに付与
- `/draft` / `/manage` / `/edit` には `Referrer-Policy: no-referrer`
- それ以外は `Referrer-Policy: strict-origin-when-cross-origin`
- PR13 段階で変更不要

#### `frontend/package.json` の scripts（PR5 で作成済）

```json
{
  "dev": "next dev",
  "build": "next build",
  "start": "next start",
  "typecheck": "tsc --noEmit",
  "test": "vitest run",
  "test:watch": "vitest",
  "cf:build": "opennextjs-cloudflare build",
  "cf:preview": "opennextjs-cloudflare preview",
  "cf:check": "wrangler deploy --dry-run"
}
```

`deploy` script を追加するか:
- 推奨: **追加しない**（PR14 で wrangler コマンドを直接叩く形にし、誤って `npm run deploy` で意図しない deploy が走るリスクを避ける）
- 必要時: PR24（Cloud Build / GitHub Actions 整備）で deploy script を整備

#### `frontend/lib/cookies.ts` / `frontend/lib/api.ts`（PR10 で作成済、PR10.5 でテスト済）

- `process.env.COOKIE_DOMAIN ?? ""` を読む（Server-side）
- `getApiBaseUrl()` は `process.env.NEXT_PUBLIC_API_BASE_URL` を読む
- いずれも **コード変更不要**

#### `frontend/.env.production.example`（PR10 / domain 整合 commit で更新済）

3 つのキーがコメント付きで定義済（COOKIE_DOMAIN は Server-only 明記）。**変更不要**。

### 5.2 deploy 前の依存確認

```sh
# wrangler version
npm --prefix frontend exec -- wrangler --version
# wrangler login 状態
npm --prefix frontend exec -- wrangler whoami
```

- `wrangler whoami` が未ログインなら、ユーザーが `npm --prefix frontend exec -- wrangler login` を実行する必要あり（ブラウザが開く対話 OAuth）
- すでにログイン済なら省略
- API token を使う場合は `CLOUDFLARE_API_TOKEN` env 設定の選択肢もあるが、PR13 では OAuth login を推奨（M1 spike で確立済）

---

## 6. Deploy 手順案（PR14 で実行、本書では実行しない）

### 6.1 事前準備

```sh
# wrangler ログイン状態確認
npm --prefix frontend exec -- wrangler whoami
# 未ログインならユーザーが対話 login 実行
# ! npx --prefix frontend wrangler login

# git status クリーン確認
git -C /home/erenoa6621/dev/vrc_photobook status
# .env.production が生成されていないことを確認（gitignore 済だが念のため）
ls -la frontend/.env.production 2>&1 || echo "(no .env.production yet, OK)"
```

### 6.2 .env.production 生成

```sh
cat > frontend/.env.production <<'EOF'
NEXT_PUBLIC_BASE_URL=https://app.vrc-photobook.com
NEXT_PUBLIC_API_BASE_URL=https://api.vrc-photobook.com
COOKIE_DOMAIN=.vrc-photobook.com
EOF
# 確認（中身は公開値なので grep / cat で見て OK）
cat frontend/.env.production
```

注意:
- `NEXT_PUBLIC_BASE_URL` の値はまだ `app.vrc-photobook.com` Custom Domain が無いが、build 時に inline するため **PR14 段階で書き込んで OK**（PR15 で Custom Domain が立ち上がれば実 URL と一致）
- PR14 deploy 後の Workers URL は `https://vrcpb-frontend.<account>.workers.dev` で、こちらは middleware / Route Handler の動作確認に使う（`NEXT_PUBLIC_BASE_URL` の値とは一致しなくてよい、redirect 先は `https://app.vrc-photobook.com/...` になるが、まだ繋がらない → §7 検証項目で「不正 token redirect 経路」のみ確認、200 経路は PR15 / PR16 後）

### 6.3 typecheck / test / build

```sh
npm --prefix frontend run typecheck
npm --prefix frontend run test
npm --prefix frontend run build
npm --prefix frontend run cf:build
```

各コマンドが OK なことを確認。

### 6.4 deploy（実行）

```sh
# wrangler deploy（cd 不使用、wrangler は frontend ディレクトリの wrangler.jsonc を参照）
npm --prefix frontend exec -- wrangler deploy
```

deploy 後の出力で以下を控える:
- Deployment ID
- 公開 URL（`https://vrcpb-frontend.<account>.workers.dev`）

### 6.5 deploy 後の sanity check

```sh
# wrangler deployments list
npm --prefix frontend exec -- wrangler deployments list
```

---

## 7. 検証項目（PR14 で確認）

### 7.1 Workers URL での確認

```sh
URL=https://vrcpb-frontend.<account>.workers.dev

# /
curl -sI "${URL}/"
# 期待: 200, x-robots-tag: noindex, nofollow,
#       referrer-policy: strict-origin-when-cross-origin
#       上記ヘッダがそれぞれ 1 回だけ出る（M1 二重出力学習との整合）

# /draft/<不正 43 文字 token>
curl -sI "${URL}/draft/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
# 期待: 302
#       location: https://app.vrc-photobook.com/?reason=invalid_draft_token
#       cache-control: no-store
#       referrer-policy: no-referrer
#       Set-Cookie ヘッダ無し
#       Location に raw token が含まれない（リダイレクト先は固定 reason のみ）

# /manage/token/<不正 token>
curl -sI "${URL}/manage/token/BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
# 期待: 302
#       location: https://app.vrc-photobook.com/?reason=invalid_manage_token
#       同上の属性

# /edit/<photobook_id>（最小ページ）
curl -sI "${URL}/edit/01234567-89ab-cdef-0123-456789abcdef"
# 期待: 200

# /manage/<photobook_id>
curl -sI "${URL}/manage/01234567-89ab-cdef-0123-456789abcdef"
# 期待: 200
```

### 7.2 Backend fetch 成立の確認

`/draft/<不正 token>` リクエストの裏で Backend `https://api.vrc-photobook.com/api/auth/draft-session-exchange` への fetch が走るので:

```sh
# Backend logs に 401 リクエストが記録されることを確認
gcloud logging read 'resource.type="cloud_run_revision"
  AND resource.labels.service_name="vrcpb-api"
  AND httpRequest.requestUrl=~"/api/auth/.*-session-exchange"' \
  --limit=10 --format='value(timestamp,httpRequest.requestMethod,httpRequest.requestUrl,httpRequest.status)'
```

期待:
- 直近の検証 curl 実行直後の timestamp に 401 が記録されている
- request body に raw token が含まれていても、Cloud Run logs には body は記録されない（PR9c の方針）

### 7.3 ヘッダ二重出力チェック

`X-Robots-Tag` / `Referrer-Policy` が **1 度だけ** 出ること（PR5 で M1 二重出力事故から学習した方針）:

```sh
curl -sI "${URL}/" | grep -ic "x-robots-tag"
# 期待: 1
curl -sI "${URL}/" | grep -ic "referrer-policy"
# 期待: 1
```

### 7.4 Logs 漏洩 grep

#### Workers logs

```sh
npm --prefix frontend exec -- wrangler tail --format=pretty &
# 別ターミナルで上記 curl を実行
# tail を止め、出力を grep
```

または `wrangler tail` をバックグラウンドで取得 → ファイル化 → grep:

```sh
grep -iE "(SECRET|API_KEY|PASSWORD|PRIVATE|sk_live|sk_test|draft_edit_token|manage_url_token|session_token|set-cookie|DATABASE_URL=|Cookie:)" <wrangler-tail-log>
```

注意: dev server と異なり、Cloudflare Workers の標準 logger は URL path をそのまま記録しない（runtime のリクエストロガーは `console.log` 経由のみ）。本実装では `console.log` を呼んでいないので、URL path に含まれる raw token も Workers logs には漏れない。

#### Backend logs

```sh
gcloud run services logs read vrcpb-api --region=asia-northeast1 --limit=200 |
  grep -iE "(SECRET|API_KEY|PASSWORD|PRIVATE|sk_live|sk_test|draft_edit_token|manage_url_token|session_token|set-cookie|DATABASE_URL=)"
# 期待: マッチなし
```

### 7.5 Cookie 属性の確認は PR15 / PR16 で

Workers URL の hostname は `*.workers.dev` で、`COOKIE_DOMAIN=.vrc-photobook.com` の Cookie は host-only に降格して **Set-Cookie ヘッダ自体が拒否される**可能性が高い（ブラウザは異ドメインの Cookie Domain を拒否）。

→ Cookie 属性の確認は `app.vrc-photobook.com` Custom Domain 設定後の **PR15 / PR16** で行う。PR14 段階では「不正 token 経路で Set-Cookie が出ない」「302 redirect が成立する」までを確認すれば十分。

---

## 8. PR16 前の実 token 取得テンプレ（**コードはコミットしない、手順のみ**）

### 8.1 厳守事項（再掲）

- repo 外の作業ディレクトリ (`~/scratch/vrcpb-token-gen/` 等) に Go プログラムを書く → **コミットしない**
- 本番 router に debug token 発行 endpoint を **追加しない**
- dummy token で成功する経路を **作らない**
- 固定 token を **コミットしない**
- raw token / session_token を README / 作業ログ / コミットメッセージ / chat に **貼らない**
- shell history に raw token を **残さない**（`HISTCONTROL=ignorespace` 行頭スペース or `unset HISTFILE`）
- 取得後 `~/scratch/vrcpb-token-gen/` を `rm -rf` で削除、シェル変数も `unset`

### 8.2 internal package 制約（PR10 / Cloud SQL 検証で経験済）

backend/internal/photobook/internal/usecase は **`backend/internal/photobook/...` 以外から import 不可**。
そのため scratch ディレクトリに Go プログラムを置くと internal package import で失敗する。

### 8.3 推奨手順（テンプレ）

#### Step 1: 一時ディレクトリ + main package を **backend 配下** に作成（コミット禁止）

```sh
mkdir -p backend/internal/photobook/_tokengen
# main.go を書く（中身は §8.4 のテンプレ）
```

> **重要**: `backend/internal/photobook/_tokengen/` は git track 対象だが、**作業セッション完結時に必ず `rm -rf` する**。コミットしない。
> `.gitignore` に追加する案もあるが、track 対象との認識を明確にしないと事故の元なので、**毎回 rm して git status クリーンを確認**する運用とする。

#### Step 2: Cloud SQL Auth Proxy 起動（PR12 / Cloud SQL 検証時と同じ手順）

```sh
# repo root から
~/bin/cloud-sql-proxy --port=5433 \
  project-1c310480-335c-4365-8a8:asia-northeast1:vrcpb-api-verify
# バックグラウンドで動かす場合は & を付ける
```

#### Step 3: DB password を Secret から取得 → DSN 組み立て

```sh
# Secret から DSN を取得（payload を直接 echo しない）
SECRET=$(gcloud secrets versions access latest --secret=DATABASE_URL 2>/dev/null)
# password 部分を抽出（: と @ の間）
DB_PASSWORD=$(printf '%s' "$SECRET" | sed -E 's|^postgres://[^:]+:([^@]+)@.*$|\1|')
unset SECRET
DATABASE_URL_PROXY="postgres://vrcpb_app:${DB_PASSWORD}@127.0.0.1:5433/vrcpb?sslmode=disable"
```

#### Step 4: tokengen を実行 → 一時ファイルに stdout 受け取り → シェル変数に取り込み

```sh
TMPOUT=$(mktemp)
DATABASE_URL="${DATABASE_URL_PROXY}" \
  go -C /home/erenoa6621/dev/vrc_photobook/backend run ./internal/photobook/_tokengen 2>/dev/null > "${TMPOUT}"

# 値を直接 cat / echo しない、grep でシェル変数に取り込み
DRAFT_RAW=$(grep '^DRAFT=' "${TMPOUT}" | cut -d= -f2)
MANAGE_RAW=$(grep '^MANAGE=' "${TMPOUT}" | cut -d= -f2)
rm -f "${TMPOUT}"
echo "DRAFT_RAW length: ${#DRAFT_RAW}"   # 43
echo "MANAGE_RAW length: ${#MANAGE_RAW}" # 43
```

#### Step 5: curl で結合確認（PR16 で実施）

```sh
URL=https://app.vrc-photobook.com  # PR15 後
curl -sS -i -X POST "${URL}/draft/${DRAFT_RAW}"
# 期待: 302 + Set-Cookie: vrcpb_draft_<id>=...; Domain=.vrc-photobook.com; ...
```

#### Step 6: クリーンアップ

```sh
# tokengen 削除
rm -rf /home/erenoa6621/dev/vrc_photobook/backend/internal/photobook/_tokengen

# git status クリーン確認
git -C /home/erenoa6621/dev/vrc_photobook status

# シェル変数破棄
unset DB_PASSWORD DATABASE_URL_PROXY DRAFT_RAW MANAGE_RAW

# Cloud SQL Auth Proxy 停止
pkill -f cloud-sql-proxy || true
```

### 8.4 tokengen main.go テンプレ（**本書には完全コードを書かず、構造のみ提示**）

擬似コード（PR16 実施時に backend/internal/photobook/_tokengen/main.go として作成、削除する）:

```
package main

依存:
- github.com/jackc/pgx/v5/pgxpool
- vrcpb/backend/internal/photobook/domain/vo/(opening_style|photobook_layout|photobook_type|visibility)
- vrcpb/backend/internal/photobook/infrastructure/(repository/rdb|session_adapter)
- vrcpb/backend/internal/photobook/internal/usecase

main():
1. ctx, dsn=os.Getenv("DATABASE_URL"), pool=pgxpool.New
2. repo=NewPhotobookRepository(pool)
3. CreateDraftPhotobookInput { Type=Memory, Title="Verify", Layout=Simple,
                               OpeningStyle=Light, Visibility=Unlisted,
                               CreatorDisplayName="Verify", RightsAgreed=true,
                               Now=time.Now().UTC(), DraftTTL=24h }
4. draftOut = NewCreateDraftPhotobook(repo).Execute(ctx, in)
5. pubOut = NewCreateDraftPhotobook(repo).Execute(ctx, in)  # publish 用に別 photobook
6. publish=NewPublishFromDraft(pool, NewPhotobookTxRepositoryFactory(),
                               NewDraftRevokerFactory(), NewMinimalSlugGenerator())
7. pub = publish.Execute(ctx, { PhotobookID=pubOut.Photobook.ID(),
                                ExpectedVersion=pubOut.Photobook.Version(),
                                Now=time.Now().UTC() })
8. fmt.Printf("DRAFT=%s\n", draftOut.RawDraftToken.Encode())
9. fmt.Printf("MANAGE=%s\n", pub.RawManageToken.Encode())
```

完全なコードは PR16 実施時に書き、使用後 `rm -rf`。本書では構造のみ。

---

## 9. 切戻し手順

### 9.1 wrangler deployments で旧 deployment を確認

```sh
npm --prefix frontend exec -- wrangler deployments list
# 出力例:
# Created      Deployment ID                          Status
# 2026-04-27   abc123-...                             Active
```

### 9.2 旧 deployment へ rollback

```sh
npm --prefix frontend exec -- wrangler rollback <旧 deployment ID>
```

### 9.3 env を修正して再 deploy

```sh
# .env.production を修正
vim frontend/.env.production
# 再 build + deploy
npm --prefix frontend run cf:build
npm --prefix frontend exec -- wrangler deploy
```

### 9.4 Workers URL で旧挙動を確認

```sh
curl -sI "https://vrcpb-frontend.<account>.workers.dev/"
```

期待: rollback 後の middleware / Route Handler の挙動

### 9.5 DNS 影響について

PR13 段階では `app.vrc-photobook.com` Custom Domain を **設定していない** ため、Workers URL（`*.workers.dev`）のみが影響を受ける。Cloudflare DNS の `app` レコードは追加していないので、Workers deploy 失敗 → Frontend 全停止という事態にはならない。

### 9.6 切戻しが必要なケースの作業ログ化

切戻しが発生した場合は `harness/work-logs/2026-04-27_frontend-workers-deploy-rollback.md` 等に経緯を記録（raw 値・Secret は書かない）。

---

## 10. Cloud SQL 残置ガード（再掲）

ロードマップ §A 確定事項:

- **PR17 完了後に Cloud SQL `vrcpb-api-verify` の残置 / 一時削除を必ず判断**
- Image / R2 にすぐ進む（PR18 着手が数日以内）→ **残置**
- 数日以上空く → **一時削除**（`m2-cloud-sql-short-verification-plan.md` §11 の手順）
- 検証用 DB をなし崩しに本番相当扱いしない

PR13 / PR14 / PR15 / PR16 の作業中は **残置継続**（`/readyz 200` を保つため）。
PR17 完了後の判断ポイントを忘れないよう、本書 §10 + ロードマップ §A に二重に明記。

---

## 11. ユーザー操作と Claude Code 操作の分担

### ユーザー手動が必須

- `wrangler login` のブラウザ認証（必要時）
- Cloudflare Dashboard 操作（必要時、本 PR では不要見込み）
- deploy 実行の **承認**（`wrangler deploy` 直前で停止して確認）
- 失敗時の切戻し承認

### Claude Code が実施

- `frontend/.env.production` 生成案内（値は公開値のみ、実 Secret は無いので Bash ツールで作成可）
- `npm run typecheck` / `test` / `build` / `cf:build` の実行
- `wrangler deploy` の実行（ユーザー承認後）
- `curl` / `wrangler deployments list` / `wrangler tail` での検証
- Backend logs grep
- 作業ログ作成 / 切戻しコマンド整備

---

## 12. 実施しないこと（再掲）

本書は **計画書のみ**。以下は実行しない:

- Workers deploy / Workers Custom Domain 設定
- Cloudflare DNS 変更（`app.vrc-photobook.com` の追加は PR15）
- `app.vrc-photobook.com` への接続
- Backend (Cloud Run / Cloud SQL) 変更
- Cloud SQL 削除（PR17 完了後の判断まで残置）
- SendGrid / Turnstile / R2 設定
- 実 token 生成（PR16 で実施、本書は手順テンプレのみ）
- Debug endpoint / dummy token 経路追加
- `frontend/.env.production` の git track（gitignore 維持）

---

## 13. ユーザー判断事項（PR14 着手前に確認）

### 13.1 COOKIE_DOMAIN 注入方式

- [ ] **案 A: `.env.production` build 時 inline**（推奨、§4.3）
- [ ] 案 B: `wrangler.jsonc` vars
- [ ] 案 C: Workers Secrets

### 13.2 deploy script 追加

- [ ] **追加しない**（推奨、PR14 で wrangler コマンド直接、§5.1）
- [ ] 追加（誤実行リスクあり、PR24 の CI 整備で吸収するなら今追加してもよい）

### 13.3 wrangler login 状態

- [ ] 既にログイン済（M1 spike 時から継続）
- [ ] 再 login が必要

### 13.4 PR14 を本書承認後すぐ実施するか

- [ ] **すぐ実施**（推奨、Cloud SQL 残置の課金抑制のため）
- [ ] 別タイミングで実施

---

## 14. 関連ドキュメント

- [Post-deploy Final Roadmap](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md) §A PR13-PR17
- [Backend Domain Mapping 実施結果](../../harness/work-logs/2026-04-27_backend-domain-mapping-result.md)
- [M2 Domain Mapping 実施計画](./m2-domain-mapping-execution-plan.md)
- [M2 Cloud SQL 短時間検証 計画](./m2-cloud-sql-short-verification-plan.md)
- [M2 Photobook Session 接続計画](./m2-photobook-session-integration-plan.md) §6 / §12
- [`frontend/wrangler.jsonc`](../../frontend/wrangler.jsonc) / [`frontend/open-next.config.ts`](../../frontend/open-next.config.ts)
- [`frontend/middleware.ts`](../../frontend/middleware.ts) / [`frontend/lib/cookies.ts`](../../frontend/lib/cookies.ts) / [`frontend/lib/api.ts`](../../frontend/lib/api.ts)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
- [Cloudflare Workers Custom Domain 公式](https://developers.cloudflare.com/workers/configuration/routing/custom-domains/)
- [OpenNext for Cloudflare 公式](https://opennext.js.org/cloudflare)
