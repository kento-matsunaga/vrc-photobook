# 2026-04-27 Backend Domain Mapping 実施結果

## 概要

`docs/plan/m2-domain-mapping-execution-plan.md` §6 と
`2026-04-27_post-deploy-final-roadmap.md` §A PR12 に基づき、
`api.vrc-photobook.com` を Cloud Run service `vrcpb-api`（asia-northeast1）に
紐付け、HTTPS 経由で `/readyz 200` と token exchange 400/401 を確認した。

- 実施日: 2026-04-27（深夜帯）
- 対象ドメイン: `api.vrc-photobook.com`
- Cloud Run service: `vrcpb-api`（revision `vrcpb-api-00002-pdn`、DB あり）
- Cloudflare Proxy status: **DNS only**（灰色雲）

## タイムライン

| 時刻 (JST) | 出来事 |
|---|---|
| ~23:30 | gcloud beta run domain-mappings create で Domain Mapping 作成 |
| ~23:30 | Cloud Run 提示: api → CNAME → ghs.googlehosted.com |
| ~23:35 | Cloudflare DNS に api CNAME (DNS only) をユーザーが追加 |
| 23:35Z | 1 度目の certificate challenge 失敗（"challenge data was not visible through the public internet"）|
| 23:55Z | 2 度目の retry で発行成功 |
| 00:55:57 | polling が `Ready=True / CertificateProvisioned=True` を検知 |
| 00:56〜 | HTTPS 疎通確認、logs 漏洩 grep、本作業ログ作成 |

経過時間: Domain Mapping 作成から証明書発行まで **約 57 分**。
1 度目の challenge は DNS 伝播タイミングのズレで失敗したが、
自動 retry の 15 分間隔で 2 度目に成功。

## 実施した DNS レコード（Cloudflare）

| Type | Name | Content | Proxy status |
|---|---|---|---|
| TXT | `vrc-photobook.com` (`@`) | `google-site-verification=...`（domain verify 用） | DNS only（自動） |
| **CNAME** | **`api`** | **`ghs.googlehosted.com`** | **DNS only**（**灰色雲**） |

## 検証結果

### HTTPS 疎通

```
GET  https://api.vrc-photobook.com/health  → 200 {"status":"ok"}
GET  https://api.vrc-photobook.com/readyz  → 200 {"status":"ready"}
POST https://api.vrc-photobook.com/api/auth/draft-session-exchange  (空 body)
  → 400 {"status":"bad_request"} + Cache-Control: no-store + Set-Cookie 無し
POST https://api.vrc-photobook.com/api/auth/manage-session-exchange (空 body)
  → 400 {"status":"bad_request"} 同上
POST https://api.vrc-photobook.com/api/auth/draft-session-exchange
  (不正 token 43 文字 'A') → 401 {"status":"unauthorized"} + Cache-Control: no-store + Set-Cookie 無し
```

すべて期待通り。

### 証明書

```
issuer  = C = US, O = Google Trust Services, CN = WR3
subject = CN = api.vrc-photobook.com
notBefore = Apr 26 14:48:07 2026 GMT
notAfter  = Jul 25 15:37:17 2026 GMT
```

Google Trust Services 発行、有効期限 約 90 日。Cloud Run が自動更新する想定。

### Cloud Run logs 漏洩 grep

```
gcloud run services logs read vrcpb-api --region=asia-northeast1 --limit=500 |
  grep -iE "(SECRET_KEY|API_KEY|PASSWORD=|PRIVATE_KEY|sk_live|sk_test|
            draft_edit_token|manage_url_token|session_token|set-cookie|DATABASE_URL=)"
```

→ **マッチなし**（漏洩なし）。

## 実施しなかったこと

- app.vrc-photobook.com Workers Custom Domain 設定（PR15）
- Workers deploy（PR14）
- Cloudflare Proxy ON 化（DNS only のまま）
- Cloud SQL 削除（残置中、`2026-04-26_cloud-sql-short-verification-result.md` §残す判断）
- DATABASE_URL Secret 削除（残置中）
- 既存 spike リソース削除
- SendGrid / Turnstile / R2 変更
- Budget Alert 変更
- raw token / DATABASE_URL / password の本書・チャット・コミットメッセージへの記録

## 注意点

- **Cloudflare Proxy は引き続き DNS only に維持**。Proxy ON にすると Cloud Run の
  証明書自動更新が失敗するリスクがある（ACME challenge 経路の妨害）
- 証明書は約 90 日後（2026-07-25）に切れる前に Cloud Run が自動更新するが、
  M3 段階で更新失敗の監視を Cloud Monitoring に追加する判断
- 1 度目の challenge 失敗は DNS 伝播タイミングのズレが原因と推測されるため、
  本書で再発防止の対応は不要

## 失敗時切戻し（参考、本書では実施しない）

```sh
# Domain Mapping 削除
gcloud beta run domain-mappings delete --domain=api.vrc-photobook.com \
  --region=asia-northeast1 --quiet

# Cloudflare DNS の api CNAME をユーザーが Dashboard で削除
# → Backend は引き続き run.app URL で動作
```

## 次のステップ（`2026-04-27_post-deploy-final-roadmap.md` §A）

PR13: Frontend Workers deploy 計画書
- COOKIE_DOMAIN の Workers 注入方式を必ず決める
- PR16 前の repo 外 token 取得テンプレを文書化
- PR17 完了後の Cloud SQL 残置/一時削除を必須判断

## 関連

- [`docs/plan/m2-domain-mapping-execution-plan.md`](../../docs/plan/m2-domain-mapping-execution-plan.md)
- [`docs/plan/m2-domain-purchase-checklist.md`](../../docs/plan/m2-domain-purchase-checklist.md)
- [`harness/work-logs/2026-04-26_backend-cloud-run-deploy-result.md`](./2026-04-26_backend-cloud-run-deploy-result.md)
- [`harness/work-logs/2026-04-26_cloud-sql-short-verification-result.md`](./2026-04-26_cloud-sql-short-verification-result.md)
- [`harness/work-logs/2026-04-27_post-deploy-final-roadmap.md`](./2026-04-27_post-deploy-final-roadmap.md)
