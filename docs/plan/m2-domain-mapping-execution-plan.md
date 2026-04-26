# M2 ドメイン Mapping 実施計画（DNS / Workers Custom Domain / Cloud Run Domain Mapping）

> 作成日: 2026-04-26
> 位置付け: 購入済み `vrc-photobook.com` を実環境（Frontend Workers + Backend Cloud Run）に
> 紐付ける作業の **計画書のみ**。本書の段階では DNS 変更 / Custom Domain 設定 / Domain Mapping /
> deploy は **一切実行しない**。
>
> 上流参照（必読、本書では再記載しない）:
> - [`docs/plan/m2-domain-purchase-checklist.md`](./m2-domain-purchase-checklist.md)（購入完了記録、§4 で API ドメイン方針 案 A 確定）
> - [`docs/plan/m2-domain-candidate-research.md`](./m2-domain-candidate-research.md) §9.5（購入確定）
> - [`docs/plan/m2-early-domain-and-cookie-plan.md`](./m2-early-domain-and-cookie-plan.md) §5-§8 / §13（DNS 構成案・切替手順）
> - [`docs/adr/0001-tech-stack.md`](../adr/0001-tech-stack.md) / [`docs/adr/0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md)
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md) / [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`.agents/rules/wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
> - [Cloudflare Workers Custom Domain 公式](https://developers.cloudflare.com/workers/configuration/routing/custom-domains/)
> - [Cloud Run mapping custom domains 公式](https://docs.cloud.google.com/run/docs/mapping-custom-domains)

---

## 0. 本計画書の使い方

- 本書は **計画書のみ**。実コマンドは整備するが、実行は次の「実施 PR」で行う。
- §4 を上から順にチェック → §5-§6 で個別計画を確認 → §9 で deploy 順序を決定 → §13 で分担合意してから次の実施 PR に進む。
- 実施 PR は本書の手順を 1 つずつ進めるたびに、Claude Code が `dig` / `curl` / `gcloud` / `wrangler` で **客観確認** する。`.agents/rules/wsl-shell-rules.md` の sudo / cwd ルールに従う。

---

## 1. 目的

- **`app.vrc-photobook.com`** を Frontend Workers (`vrcpb-frontend`) に紐付け、`https://app.vrc-photobook.com/` で配信
- **`api.vrc-photobook.com`** を Backend Cloud Run service (`vrcpb-api`) に紐付け、`https://api.vrc-photobook.com/` で配信
- **Cookie Domain `.vrc-photobook.com`** を Frontend Route Handler の `Set-Cookie` で発行可能にし、`app` ↔ `api` 間で draft / manage session Cookie を共有（U2 解消、ADR-0003）
- 上記 3 点を満たした上で、Safari / iPhone Safari 実機で `/draft/<token>` → `/edit/<id>` redirect が成立する前提を作る

---

## 2. 前提

- `vrc-photobook.com` は **購入済**（2026-04-26、Cloudflare Registrar、`m2-domain-purchase-checklist.md` 冒頭）
- DNS 管理は Cloudflare（Registrar 購入で zone 自動作成、Name Server は `LIA.NS.CLOUDFLARE.COM` / `RAZVAN.NS.CLOUDFLARE.COM` に向き済み）
- 本書段階では DNS レコード追加 / Workers Custom Domain / Cloud Run Domain Mapping は **未実施**
- Cloud Run / Workers 本番 deploy は **別 PR**（本書では deploy 順序の整理のみ）
- Cloud SQL は本書範囲外（M2 後期で別途）
- SendGrid / Turnstile 本番 widget / R2 変更は本書範囲外
- PR10.5 までで **Frontend Route Handler の token → session Cookie flow は自動テスト済み**（Vitest 16/16 PASS、`.vrc-photobook.com` Domain 反映を含む）

---

## 3. 目標構成

```
ユーザー（ブラウザ） ──HTTPS──→ app.vrc-photobook.com  →  Cloudflare Workers (vrcpb-frontend, OpenNext)
                                                          │
                                                          └ /draft/[token] route handler
                                                            ↓ HTTPS
                                                     api.vrc-photobook.com  →  Cloud Run (vrcpb-api, asia-northeast1)
                                                                              ↓
                                                                       PostgreSQL（M2 後期で Cloud SQL）
```

| 項目 | 値 |
|---|---|
| Frontend host | `app.vrc-photobook.com` |
| Backend host | `api.vrc-photobook.com` |
| `NEXT_PUBLIC_BASE_URL` (build env) | `https://app.vrc-photobook.com` |
| `NEXT_PUBLIC_API_BASE_URL` (build env) | `https://api.vrc-photobook.com` |
| `COOKIE_DOMAIN` (Frontend Server-only env) | `.vrc-photobook.com` |
| `ALLOWED_ORIGINS` (Backend env) | `https://app.vrc-photobook.com` |
| Cookie 属性 | `HttpOnly; Secure; SameSite=Strict; Path=/; Domain=.vrc-photobook.com; Max-Age=...` |
| Cloud Run service | `vrcpb-api`（本実装名、M2 段階で別途作成 or 既存 spike rename か判断） |
| Cloud Run region | `asia-northeast1`（東京、Domain Mapping GA 対応） |
| Workers project | `vrcpb-frontend`（M2 本実装名、PR5 で作成済の wrangler.jsonc 設定） |

---

## 4. 実施前チェック（事前確認）

以下を**実施 PR 着手前** に上から順にチェック:

### 4.1 Cloudflare 側

- [ ] Cloudflare Dashboard で `vrc-photobook.com` zone が **Active** であること
- [ ] Name Server: `*.ns.cloudflare.com` x2 が確認できる（WHOIS で `LIA.NS.CLOUDFLARE.COM` / `RAZVAN.NS.CLOUDFLARE.COM` を確認済）
- [ ] DNSSEC: 未署名（`unsigned`）でも進めて OK。本実装後に `signed` 化の判断
- [ ] 既存 DNS レコード: zone 作成直後の状態（`@` の A/AAAA、MX 自動設定があれば確認）。`app` / `api` レコードはまだ存在しないことを確認
- [ ] Cloudflare Registrar の自動更新 ON（購入時に確認済）
- [ ] Workers project `vrcpb-frontend` が Cloudflare 上に存在（または PR5 の wrangler.jsonc から `wrangler deploy --dry-run` で接続だけ確認）

### 4.2 GCP 側

- [ ] GCP project: `project-1c310480-335c-4365-8a8`（M1 で確定済）
- [ ] Region: `asia-northeast1`
- [ ] Billing 有効化済（M1 で確認済、Budget Alert ¥1,000 設定済）
- [ ] 必要 API:
  - [ ] `run.googleapis.com`（Cloud Run）
  - [ ] `artifactregistry.googleapis.com`（Artifact Registry）
  - [ ] `secretmanager.googleapis.com`（Secret Manager）
  - [ ] `cloudbuild.googleapis.com`（後続 deploy で使用）
- [ ] gcloud CLI が認証済（`gcloud auth list` で確認）
- [ ] 既存リソース調査:
  - 現在の Cloud Run service 名（`gcloud run services list --region=asia-northeast1`）
  - 現在の Artifact Registry repo（`gcloud artifacts repositories list`）
  - 既存の `vrcpb-spike-api` を **rename** して使うか、**新規 `vrcpb-api`** を作るか（§4.3 で判断）

### 4.3 既存 spike リソースの扱い（重要判断）

| 案 | 内容 | 利点 | 欠点 |
|---|---|---|---|
| 案 X | 既存 `vrcpb-spike-api` を Domain Mapping 対象にする | 早く動かせる、設定差分が小さい | 名前と本実装の規約が一致しない、後で rename / migration が必要 |
| 案 Y | 新規 `vrcpb-api` を作成、Domain Mapping 対象にする、旧 spike は併存 | 本実装名で統一、移行リスクが時間軸で分離 | image build / deploy / Secret 設定の作業量が増える |

**推奨: 案 Y（新規 `vrcpb-api` 作成）**。理由:
- M2 本実装の Backend は spike とは別物（PoC コードを直接流用しない方針）
- Domain Mapping だけ移すと、本実装 Backend deploy 時に再 mapping が必要で 2 度手間
- 旧 spike を残しておくと切戻しの参照点として使える

ただし Cloud Run service の作成と本実装 Backend の deploy（image build / Cloud Build / Cloud SQL or 一時 DB 接続）は **別 PR の deploy 計画 §で扱う**。本書の Domain Mapping は「deploy 後の `vrcpb-api` を api.vrc-photobook.com に紐付ける」段階を扱う。

→ 結論: 本書の **Domain Mapping は Backend deploy 計画 PR より後**に実施する（§9）。

---

## 5. `app.vrc-photobook.com` Workers Custom Domain 計画

### 5.1 流れ

1. Cloudflare Dashboard → `Workers & Pages → vrcpb-frontend → Settings → Triggers → Custom Domains`
2. `Add Custom Domain` → `app.vrc-photobook.com` を入力
3. Cloudflare が **自動で DNS レコード（CNAME）を作成**（Workers Custom Domain は DNS レコードを Cloudflare 側で勝手に作る／管理する）
4. SSL/TLS 証明書は Cloudflare Universal SSL で自動発行（数秒〜数分）
5. 状態が `Active` になったら確認（次節）

### 5.2 wrangler / dashboard 操作

- Cloudflare Dashboard 経由が確実（GUI 1 操作で完結）
- `wrangler` CLI でも `wrangler triggers add custom-domain ...` が可能だが、本書段階では Dashboard 推奨（変更点が少ない）
- `wrangler deployments list` で現在の deploy 状況確認

### 5.3 確認コマンド

```sh
# DNS 解決
dig app.vrc-photobook.com +short
# 期待: Cloudflare の anycast IP（104.x.x.x など）

# HTTPS 接続 + middleware ヘッダ
curl -sI https://app.vrc-photobook.com/
# 期待: HTTP/2 200, x-robots-tag: noindex, nofollow,
#       referrer-policy: strict-origin-when-cross-origin

# 不正 token redirect の動作（PR10）
curl -sI "https://app.vrc-photobook.com/draft/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
# 期待: 302, Location: https://app.vrc-photobook.com/?reason=invalid_draft_token,
#       Set-Cookie が出ない（不正 token 経路）, Cache-Control: no-store
```

### 5.4 失敗時の切戻し

- Cloudflare Dashboard `Workers & Pages → vrcpb-frontend → Settings → Triggers → Custom Domains` で該当ドメインを `Remove`
- 自動作成された DNS レコードは Custom Domain 解除と同時に Cloudflare が削除する
- Frontend は引き続き旧 `https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev/` で動作（PR5 の wrangler 設定に基づく `vrcpb-frontend` 名前へ deploy 後は `vrcpb-frontend.<account>.workers.dev` になる、別 PR）

### 5.5 注意

- Workers Custom Domain は **Cloudflare 自身が CNAME / Proxy を内部管理する仕組み**。手動で DNS レコードを編集すると壊れる
- 通常の DNS レコード（`@` / `www`）と Workers Custom Domain は別管理
- `app.vrc-photobook.com` は Workers 専用ドメインとなり、他のサービスに割り当て不可

---

## 6. `api.vrc-photobook.com` Cloud Run Domain Mapping 計画

### 6.1 流れ

1. Cloud Run 側で Domain Mapping を作成:
   ```sh
   gcloud beta run domain-mappings create \
     --service=vrcpb-api \
     --domain=api.vrc-photobook.com \
     --region=asia-northeast1
   ```
   （`run domain-mappings` は asia-northeast1 では 2024〜 GA、`gcloud beta` は当面 alias として動作）
2. Cloud Run が **DNS で証明する CNAME 値** を返す（通常 `ghs.googlehosted.com` への CNAME）
3. Cloudflare Dashboard `DNS → Records` で `api` → `ghs.googlehosted.com` の **CNAME** を作成
   - **Proxy status: DNS only**（オレンジ雲オフ、灰色）。理由は §7
4. Cloud Run 側の証明書 PROVISIONING を待つ（最大数時間、通常 10〜30 分）
5. `gcloud run domain-mappings describe ...` で `Ready` になったら確認（次節）

### 6.2 gcloud コマンド候補

```sh
# 作成
gcloud beta run domain-mappings create \
  --service=vrcpb-api \
  --domain=api.vrc-photobook.com \
  --region=asia-northeast1

# 一覧
gcloud beta run domain-mappings list --region=asia-northeast1

# 詳細（状態 / 必要 DNS レコード値）
gcloud beta run domain-mappings describe api.vrc-photobook.com \
  --region=asia-northeast1

# 削除（切戻し）
gcloud beta run domain-mappings delete api.vrc-photobook.com \
  --region=asia-northeast1
```

### 6.3 確認コマンド

```sh
# DNS 解決
dig api.vrc-photobook.com +short
# 期待: ghs.googlehosted.com -> Google IP（216.x.x.x など）
# Cloudflare proxy がオフであることが前提

# HTTPS 接続 + 証明書発行確認
curl -sI https://api.vrc-photobook.com/health
# 期待: HTTP/2 200, Content-Type: application/json
#       証明書 issuer は Google Trust Services

# 証明書発行ステータスを gcloud で
gcloud beta run domain-mappings describe api.vrc-photobook.com \
  --region=asia-northeast1 \
  --format='value(status.conditions)'
```

### 6.4 失敗時の切戻し

```sh
# Cloud Run 側
gcloud beta run domain-mappings delete api.vrc-photobook.com \
  --region=asia-northeast1

# Cloudflare 側: Dashboard DNS → Records で `api` の CNAME を削除
```

旧 Backend は `https://vrcpb-spike-api-7eosr3jcfa-an.a.run.app/health` で引き続き動作（spike を残している前提）。

### 6.5 注意

- Domain Mapping の証明書発行が長時間 PROVISIONING のまま進まない場合、`api` レコードの CNAME が誤った値（`ghs.googlehosted.com` 以外）になっていないかを `dig` で再確認
- DNS only（Proxy オフ）が必須。Proxy オン状態だと Cloud Run 側が DNS 検証に失敗する
- ICANN 登録から 24 時間以内の新規ドメインは Google 側で「ドメイン所有確認」を求められる場合があるが、Cloud Run Domain Mapping では通常スキップされる

---

## 7. Cloudflare Proxy 方針（DNS only vs Proxied）

`api.vrc-photobook.com` を Cloudflare Proxy 経由（Proxied）にするか、Proxy を切って Cloud Run へ直接通すか（DNS only）。

### 7.1 比較

| 観点 | DNS only（Proxy オフ）| Proxied（Proxy オン）|
|---|---|---|
| Cloud Run Domain Mapping との相性 | 公式推奨。証明書発行と検証が確実に通る | Cloudflare が証明書を上書きするため、Cloud Run 側の Domain Mapping 検証が **しばしば失敗** |
| TLS 証明書 | Google Trust Services（Cloud Run 発行） | Cloudflare Universal SSL（Cloudflare 発行）+ Cloudflare → Cloud Run 間は別証明書 |
| Cloudflare WAF / cache | 効かない | 効く（API レスポンスをキャッシュさせない設定が必要、設定漏れリスク）|
| CORS | 単純（Cloud Run の `ALLOWED_ORIGINS` のみ）| Cloudflare の Transform Rules / Page Rules が割り込む可能性 |
| デバッグしやすさ | `curl https://api.vrc-photobook.com` がそのまま Cloud Run に届く、ログが Cloud Run で見える | Cloudflare のログと Cloud Run のログを両方見る必要 |
| Safari / Cookie | Cloud Run の Set-Cookie がそのまま届く（が、PR9c 時点では Backend は Set-Cookie を出さない） | Cloudflare が `__cf_bm` 等の追加 Cookie を発行する可能性。Privacy / Cookie Domain の単純化を阻害 |
| DDoS 防御 | 無し（Cloud Run 側のスケール / quota だけ）| Cloudflare の WAF / Rate Limit が利用可能 |
| 将来運用 | 必要になったら Proxied に切替可能（DNS の 1 行変更）| 既に有効、ただし Cloud Run と Cloudflare の二重管理になる |

### 7.2 推奨: **DNS only（初回検証時、その後の切替は別 PR で判断）**

理由:

- **Cloud Run Domain Mapping の証明書発行が確実に通る**（公式推奨）
- 初回検証で「Cookie が `app.*` ↔ `api.*` で共有できる」「Safari ITP で First-party Cookie として扱われる」を**最小構成**で確認できる
- Proxied への切替は DNS の 1 操作で可能（リスク無し）
- API への DDoS / WAF が必要になった段階で Proxied 化を検討（M3 以降）

### 7.3 切替ポイント

以下のいずれかが必要になった段階で Proxied への切替を検討（別 PR で判断）:

- 大量の不正アクセス / botnet による Cloud Run 課金スパイク
- Backend に CORS preflight を Cloudflare で吸収させたい（不要な OPTIONS を Cloud Run に届かせない）
- API レスポンスを Cloudflare cache で短時間キャッシュしたい

---

## 8. 環境変数・Secret 更新計画

### 8.1 Frontend build env（`frontend/.env.production`、git 管理外）

```env
NEXT_PUBLIC_BASE_URL=https://app.vrc-photobook.com
NEXT_PUBLIC_API_BASE_URL=https://api.vrc-photobook.com
COOKIE_DOMAIN=.vrc-photobook.com
```

注意:
- `NEXT_PUBLIC_*` は build 時に bundle へ inline される **公開値**（Workers runtime env では届かない）
- `COOKIE_DOMAIN` は **Server-only env**（`NEXT_PUBLIC_` プレフィックス無し）。Route Handler / Server Component からのみ参照可能
- 値はすべて Secret ではないが、`.env.production` 自体は `.gitignore` で **git 管理外**
- Workers Secrets API を使う場合（runtime 注入）は別 PR で検討。本書段階は build 時 inline で十分

### 8.2 Backend env（Cloud Run 環境変数）

| 変数 | 値 | Secret か | 配置 |
|---|---|---|---|
| `APP_ENV` | `production` または `staging`（初回検証は `staging` 推奨）| 否 | Cloud Run env vars |
| `PORT` | `8080` | 否 | Cloud Run env vars（Cloud Run が PORT を自動セット、明示不要） |
| `DATABASE_URL` | Cloud SQL 接続文字列 or 暫定 PostgreSQL DSN | **はい** | **Secret Manager → Cloud Run env vars に注入** |
| `ALLOWED_ORIGINS` | `https://app.vrc-photobook.com` | 否 | Cloud Run env vars |
| `COOKIE_DOMAIN` | （Backend は Set-Cookie を出さないため不要、設定しない） | - | - |

注意:
- `DATABASE_URL` は **Secret Manager に格納し、Cloud Run のサービスアカウントで読み取り**（`m2-implementation-bootstrap-plan.md` §10.1）
- 本書段階で Cloud SQL は作っていない。Cloud Run deploy までに DB 接続先を確定する必要がある（本 PR 範囲外、deploy 計画 PR で扱う）
- Backend は本実装の段階で **CORS middleware を持たない**（PR9c 時点）。`ALLOWED_ORIGINS` は env として用意するが、middleware 接続は別 PR

### 8.3 Secret Manager に入れるもの / 入れないもの

| 値 | Secret Manager |
|---|---|
| `DATABASE_URL` | ✅ 入れる |
| 将来の `SENDGRID_API_KEY` | ✅ 入れる |
| 将来の `TURNSTILE_SECRET_KEY` | ✅ 入れる |
| 将来の `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` | ✅ 入れる |
| `APP_ENV` / `ALLOWED_ORIGINS` | ❌ 入れない（公開値、Cloud Run env vars に直接） |
| `NEXT_PUBLIC_*` / `COOKIE_DOMAIN` | ❌ 入れない（公開値、Frontend bundle / Workers env） |

### 8.4 git 管理上の制約

- `.env.production` の **実値はコミットしない**（`.gitignore` で除外済）
- `.env.*.example` は **キー名 + コメント** のみ（PR10 で更新済）
- Workers Secrets / Cloud Run env を Dashboard 上で設定する操作はユーザー手動、Claude Code は値の設計だけ提示

---

## 9. deploy 順序との関係

### 9.1 推奨順序

DNS 設定だけでアプリは完成しないため、**Backend deploy → Backend Domain Mapping → Frontend deploy → Frontend Custom Domain → 検証** の順で進める。

| # | ステップ | 担当 | 内容 |
|---|---|---|---|
| 1 | **本書の確認 + ユーザー判断**（§13）| Claude Code + ユーザー | 本書を読み、§4 のチェックを通す |
| 2 | **Backend deploy 計画 PR**（別 PR） | Claude Code | Cloud Run service 作成、Cloud Build / Artifact Registry / Secret Manager 設定の整理 |
| 3 | **Backend deploy 実施**（実 PR） | ユーザー手動 + Claude Code 検証 | Cloud Build → image push → Cloud Run revision、`ALLOWED_ORIGINS` 設定 |
| 4 | **Backend Domain Mapping**（実 PR） | ユーザー操作 or Claude Code（gcloud） | `gcloud beta run domain-mappings create ...` + Cloudflare DNS CNAME（Proxy オフ） |
| 5 | **Frontend env 更新 + build**（実 PR） | Claude Code | `.env.production` を Workers Secrets/build env として注入、`cf:build` |
| 6 | **Frontend Workers deploy**（実 PR） | ユーザー操作 or Claude Code | `wrangler deploy --env production` |
| 7 | **Frontend Custom Domain**（実 PR） | ユーザー手動 or Claude Code | Cloudflare Dashboard で `app.vrc-photobook.com` を `vrcpb-frontend` に紐付け |
| 8 | **curl 検証**（実 PR） | Claude Code | `dig` / `curl -sI` で Frontend / Backend 両方の応答確認 |
| 9 | **ブラウザ検証**（実 PR） | ユーザー | 実 token を生成し `/draft/<token>` にアクセス、Cookie 属性 / redirect / URL から token 消失を確認 |
| 10 | **Safari / iPhone Safari 実機確認**（実 PR） | ユーザー | `safari-verification.md` のチェックリスト全項目 |

### 9.2 「Workers Custom Domain を先に作る」案との比較

| 案 | 利点 | 欠点 |
|---|---|---|
| 推奨（Backend → Frontend）| Frontend が `api.vrc-photobook.com` を呼べる状態で Workers deploy できる | Backend / Cloud Run の deploy が前段にあるため作業時間が長い |
| 逆（Frontend Custom Domain を先）| `app.vrc-photobook.com` が早く立ち上がる | Backend が無いと `/draft/<token>` で 401（Backend が無いから fetch 失敗 → redirect） / token 交換 200 経路は確認できない |

→ **推奨案を採用**。`app.vrc-photobook.com` だけ先に立てても、Backend が無いと Cookie 発行経路が動かないため、結局 Backend 先行が効率的。

---

## 10. 検証項目（実施 PR で確認）

### 10.1 Backend 単独検証

- [ ] `dig api.vrc-photobook.com +short` が `ghs.googlehosted.com` 経由の Google IP を返す
- [ ] `curl -sI https://api.vrc-photobook.com/health` → 200 + `{"status":"ok"}`
- [ ] `curl -sI https://api.vrc-photobook.com/readyz` → 200 + `{"status":"ready"}`（Cloud SQL 設定済の前提）
- [ ] `curl -sI -X POST https://api.vrc-photobook.com/api/auth/draft-session-exchange` → 400 `bad_request`、`Cache-Control: no-store`、`Set-Cookie` 無し
- [ ] `curl -sI -X POST -H 'Content-Type: application/json' -d '{"draft_edit_token":"AAAA...43chars"}' https://api.vrc-photobook.com/api/auth/draft-session-exchange` → 401 `unauthorized`
- [ ] CORS preflight `curl -i -X OPTIONS -H 'Origin: https://app.vrc-photobook.com' https://api.vrc-photobook.com/api/auth/draft-session-exchange` の挙動確認（Backend に CORS middleware を入れた段階で OK）
- [ ] 証明書: `curl -v https://api.vrc-photobook.com/health 2>&1 | grep "issuer"` で `Google Trust Services`

### 10.2 Frontend 単独検証

- [ ] `dig app.vrc-photobook.com +short` が Cloudflare anycast IP を返す
- [ ] `curl -sI https://app.vrc-photobook.com/` → 200 + middleware ヘッダ（`x-robots-tag` / `referrer-policy`）
- [ ] `curl -sI "https://app.vrc-photobook.com/draft/<不正token>"` → 302 + `Location: https://app.vrc-photobook.com/?reason=invalid_draft_token` + `Cache-Control: no-store` + `Set-Cookie` 無し
- [ ] `curl -sI "https://app.vrc-photobook.com/manage/token/<不正token>"` → 同様
- [ ] `https://app.vrc-photobook.com/?reason=invalid_draft_token` 表示（PR10 段階では `/` にトップページ）
- [ ] OGP / metadataBase: `curl -s https://app.vrc-photobook.com/ | grep -E '(og:|metadataBase|robots)'` で `https://app.vrc-photobook.com` が絶対 URL として展開されている

### 10.3 結合検証（ブラウザ）

- [ ] 実 token を生成（PR9c の Backend を使い、`docker compose` の postgres + 一時 Backend 経由で生成、または本実装 deploy 後の Cloud Run + Cloud SQL 経由）
- [ ] `https://app.vrc-photobook.com/draft/<実 raw draft_edit_token>` にアクセス
- [ ] 302 redirect → `/edit/<photobook_id>` に着地
- [ ] DevTools / Web Inspector で Cookie 確認:
  - 名前: `vrcpb_draft_<photobook_id>`
  - `HttpOnly`: ✅
  - `Secure`: ✅
  - `SameSite=Strict`: ✅
  - `Path=/`: ✅
  - **`Domain=.vrc-photobook.com`**: ✅
  - `Max-Age` が正の値（draft_expires_at - now）
- [ ] URL に raw token が **残っていない**（`/edit/<photobook_id>` のみ）
- [ ] `https://app.vrc-photobook.com/manage/token/<実 raw manage_url_token>` でも同様（`vrcpb_manage_<id>` Cookie）
- [ ] redirect 後にページ再読込しても、Cookie が消えず、`/edit/<id>` / `/manage/<id>` に再着地できる

### 10.4 Cookie Domain 共有検証

- [ ] `app.vrc-photobook.com` で発行された Cookie が、ブラウザの `api.vrc-photobook.com` への fetch に乗る（`credentials: 'include'` 設定の API 呼び出しで確認、PR11 以降）
- [ ] `app.vrc-photobook.com` と `api.vrc-photobook.com` の両方に同じ Cookie が届く（`Domain=.vrc-photobook.com` の効果）
- [ ] M1 で確認できなかった「`/integration/backend-check` の `session-check` が `true/true`」相当の挙動が成立

---

## 11. Safari / iPhone Safari 確認計画

### 11.1 確認環境

- macOS Safari（最新）
- iPhone Safari（最新）+ できれば 1 世代前
- iOS Safari Private Browsing
- Cookie Storage タブ（Web Inspector）

### 11.2 確認項目（`safari-verification.md` 全項目 + 本実装独自項目）

- [ ] `https://app.vrc-photobook.com/draft/<token>` → 302 → `/edit/<id>` redirect 成立
- [ ] `https://app.vrc-photobook.com/manage/token/<token>` → 302 → `/manage/<id>` redirect 成立
- [ ] DevTools で Cookie 属性確認: HttpOnly / Secure / SameSite=Strict / Path=/ / **Domain=.vrc-photobook.com**
- [ ] redirect 後の URL に raw token が **残らない**（`/edit/<id>` / `/manage/<id>` のみ）
- [ ] ブラウザ history（戻るボタン）にも raw token URL が残らないこと（302 なので一般に残らないが念のため確認）
- [ ] ページ再読込後も session が維持される
- [ ] **Private Browsing** で同様のフローが成立する（一時的でよい、永続不要）
- [ ] iPhone Safari の **ITP で `.vrc-photobook.com` が First-party Cookie として扱われる**こと
- [ ] iPad Safari でも同様（可能なら）
- [ ] **24 時間後 / 7 日後の Cookie 残存**（運用開始後の継続観察、`safari-verification.md` §継続観察 / 起点を `本書実施日` に再セット）

### 11.3 失敗時のリスク

- iPhone Safari ITP が `.vrc-photobook.com` を 7 日後に第三者 Cookie 扱いし始めると、作成者が編集できなくなる重大事故
- 観察起点を `harness/work-logs/` に新規作業ログとして記録（`2026-XX-XX_safari-itp-observation.md`）
- 24 時間後 / 7 日後に Claude Code が再確認チェックを依頼するための schedule を作業ログに明記

---

## 12. 失敗時の切戻し（実施 PR で発動条件を判断）

### 12.1 Workers Custom Domain 解除

```
Cloudflare Dashboard → Workers & Pages → vrcpb-frontend
→ Settings → Triggers → Custom Domains → app.vrc-photobook.com → Remove
```
- 自動作成された DNS レコードが削除される
- Frontend は引き続き旧 `*.workers.dev` URL で動作

### 12.2 Cloud Run Domain Mapping 削除

```sh
gcloud beta run domain-mappings delete api.vrc-photobook.com \
  --region=asia-northeast1
```
- Cloudflare DNS の `api` CNAME も Dashboard から手動削除
- Backend は引き続き `*.run.app` URL で動作

### 12.3 env を spike URL に戻す

```env
# Frontend .env.production
NEXT_PUBLIC_BASE_URL=https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev
NEXT_PUBLIC_API_BASE_URL=https://vrcpb-spike-api-7eosr3jcfa-an.a.run.app
COOKIE_DOMAIN=
```
- `COOKIE_DOMAIN` を空に戻す（host-only Cookie に再帰）
- 旧 deploy が残っている前提

### 12.4 旧 URL での動作確認

- `curl https://vrcpb-spike-frontend.k-matsunaga-biz.workers.dev/`
- `curl https://vrcpb-spike-api-7eosr3jcfa-an.a.run.app/health`

### 12.5 DNS 伝播待ちの扱い

- DNS 変更 / 削除後、TTL（Cloudflare デフォルト 300 秒、5 分）が経過するまで古い結果が残る
- 切戻し後 5〜10 分は `dig` の結果が安定するまで待つ
- グローバル伝播は通常 30 分以内、最大 24 時間
- 切戻し中に再度切替を試みる場合、**TTL 経過待ち**を必ず挟む

### 12.6 切戻し判断のトリガー

以下のいずれかが起きた場合、本切戻しを実施:

- Cloud Run Domain Mapping の証明書発行が **6 時間以上** PROVISIONING のまま進まない
- Workers Custom Domain が `Active` にならず、Dashboard で error 表示
- Safari / iPhone Safari で Cookie 共有が成立しない（24 時間以内に発見）
- 予想外の課金（Workers / Cloud Run）が発生

---

## 13. ユーザー操作と Claude Code 操作の分担

### 13.1 ユーザーがやること

- Cloudflare Dashboard 上の購入済みドメイン状態確認（zone Active / DNSSEC / 自動更新）
- Cloudflare Dashboard で **Workers Custom Domain `Add Custom Domain`** クリック操作
- Cloudflare Dashboard で **DNS レコード追加**（`api` CNAME → `ghs.googlehosted.com`、Proxy オフ）
- GCP の課金 / 権限 / API 有効化が絡む操作の **承認**（gcloud 認証は既にユーザーアカウント）
- 本番 `.env.production` / Cloud Run env vars / Secret Manager の **実値登録**
- ブラウザ実機 + Safari / iPhone Safari 実機での **手動確認**
- 失敗時の切戻し操作の **承認**

### 13.2 Claude Code がやること

- `gcloud beta run domain-mappings create / list / describe / delete` の **コマンド実行**（gcloud 認証はユーザー、Bash 経由で Claude Code が叩く）
- `wrangler deploy --dry-run` 等の **検証コマンド実行**（実 deploy は別 PR）
- `dig` / `curl -sI` での **DNS / HTTPS 検証**
- 設定値の **提案 / 雛形作成**
- `frontend/.env.production` / Cloud Run env vars の **キー名と値の設計**（実値登録はユーザー）
- README / 計画書 / 作業ログの **更新**
- ログ・diff・コミットメッセージへの **Secret 漏洩チェック** (`grep` ベース)
- 失敗 / 異常時の **failure-log 起票**

### 13.3 双方が必要な作業

- 各ステップ完了時の **状態報告 + 次ステップ確認**（ユーザー判断 + Claude Code 客観確認の両立、`wsl-shell-rules.md` §sudo / インストール検証）

---

## 14. 実施しないこと（再掲）

本書は **計画書のみ**。次の項目は **本書段階で実施しない**:

- Cloudflare DNS レコード追加・削除・変更
- Workers Custom Domain 設定
- Cloud Run Domain Mapping
- Cloud Run service 作成 / deploy
- Workers deploy
- Cloud SQL 作成
- SendGrid 設定 / Turnstile 本番 widget 作成 / R2 設定変更
- 既存 spike リソース削除（切替検証完了後の別 PR で扱う）
- `frontend/.env.production` の実値登録 / Workers Secrets 設定
- Cloud Run env vars / Secret Manager の実値登録

---

## 15. 関連ドキュメント

- [M2 ドメイン購入チェックリスト + 購入記録](./m2-domain-purchase-checklist.md)
- [M2 ドメイン候補リサーチ](./m2-domain-candidate-research.md)
- [M2 早期ドメイン + Cookie 計画](./m2-early-domain-and-cookie-plan.md)
- [M2 実装ブートストラップ計画](./m2-implementation-bootstrap-plan.md)
- [M2 Session auth 実装計画](./m2-session-auth-implementation-plan.md)
- [M2 Photobook session 接続計画](./m2-photobook-session-integration-plan.md)
- [プロジェクト全体ロードマップ](../../harness/work-logs/2026-04-26_project-roadmap-overview.md)
- [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md) / [`security-guard.md`](../../.agents/rules/security-guard.md) / [`wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
- [Cloudflare Workers Custom Domain 公式](https://developers.cloudflare.com/workers/configuration/routing/custom-domains/)
- [Cloud Run mapping custom domains 公式](https://docs.cloud.google.com/run/docs/mapping-custom-domains)
