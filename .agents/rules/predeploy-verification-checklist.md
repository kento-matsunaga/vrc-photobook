# Deploy 前後 verification チェックリスト

## 適用範囲

`vrcpb-api` Backend (Cloud Run) / `vrcpb-frontend` Workers の本番 deploy を行う **すべての STOP**（hotfix を含む）。

## 原則

> **deploy 完了の判定は「build 通過」「version active 100%」だけでは不十分。production bundle / route / log の内容を実 fetch + grep で確認するまで完了扱いしない。**

理由:
- Cloud Build SUCCESS = image build OK だけ。bundle 内容が要件どおりかは保証しない。
- `wrangler deploy Current Version ID: ...` 出力 = upload OK だけ。CDN routing transient の間は旧 chunk が配信される可能性。
- `/health` `/readyz` 200 = ランタイム起動 OK だけ。新機能の正しい挙動は別 smoke が必要。
- 2026-05-02 STOP δ / STOP α サイクルで複数回、「deploy したつもり」が実際は反映されていなかった / 反映されたが想定挙動と違っていた事故が発生。

## 必須項目（全 STOP の deploy 完了報告に含める）

### 1. revision / version active 100% 確認

**Backend**:
```bash
gcloud run services describe vrcpb-api --region=asia-northeast1 --project=$PROJ --format=json \
  | python3 -c "import sys,json; s=json.load(sys.stdin); ..."
# 期待:
# - image: vrcpb-api:<expected_short_sha>
# - latestReadyRevisionName: vrcpb-api-XXXXX-yyy
# - traffic[*] で latestRevision=true かつ percent=100 が存在
```

**Workers**:
```bash
( cd frontend && npx wrangler deployments list --name vrcpb-frontend )
# 最新 deployment の Version 100% を確認
```

**rollback target を直前 revision として記録**（次に問題が出たら戻せる version ID を控える）。

### 2. routing 安定化 wait（5〜10 分）

`docs/runbook/backend-deploy.md` §1.4.1 に従う。Cloud Run は新旧 revision の routing 切替で transient が発生し、`/api/public/photobooks/{slug}` 等が chi default plain text 404 を返す事象が観測されている（`harness/failure-log/2026-04-29_public-photobook-route-unregistered-after-report-guard-deploy.md`）。

Workers も新 chunk の CDN propagation を待つ。

### 3. post-deploy smoke (route / 既存 + 新機能)

**既存 routes regression**:
- `/health` 200 / `/readyz` 200
- `/api/public/photobooks/<dummy slug>` 404 + JSON `{"status":"not_found"}`（chi default plain text 回避）
- `/p/<dummy>` 404 / `/p/<dummy>/report` 404 / `/ogp/<dummy>` 302 → `/og/default.png`
- `/` 200 / `/create` 200 / `/about` 200 / `/terms` 200 / `/privacy` 200 / `/help/manage-url` 200
- `/prepare/<dummy>` Cookie なし → error UI / `/edit/<dummy>` Cookie なし → error UI / `/draft/<bad>` 302 → `?reason=invalid_draft_token`

**新機能の direct verification**:
- 例: CORS PATCH / DELETE preflight が `Access-Control-Allow-Methods` に含まれる
- 例: `POST /publish` で `rights_agreed:false` 送信 → 409 + `{"status":"publish_precondition_failed","reason":"rights_not_agreed"}`
- 例: 新 endpoint が想定 status を返す

### 4. production bundle marker grep

**Backend**: 直接 fetch できないため、Cloud Run logs で「期待 path / status / latency」を観測（次節）。

**Workers (Frontend)**:
```bash
# 新 chunk path を curl で fetch (() / [] は URL-encoded で 307 → 元 URL に redirect)
URL=https://app.vrc-photobook.com
ENC='/_next/static/chunks/app/%28draft%29/edit/%5BphotobookId%5D/page-XXXXX.js'
curl -sSL -o /tmp/chunk.js -w "  HTTP=%{http_code} size=%{size_download}\n" "${URL}${ENC}"

# 新 marker（API path / data-testid / UI 文言）が含まれることを assert
grep -oE 'attach-images|prepare-progress|publish-rights-agreed|credentials:"include"' /tmp/chunk.js
# 0 件なら deploy 反映が不完全 → 調査
```

**旧 antipattern marker が含まれない**ことも確認:
- 例: 「公開条件に合致しません。最新を取得して再度確認してください。」固定文言 → grep で 0 件

### 5. bindings / Secrets / env / Cloud SQL / Job args / Scheduler 維持確認

deploy 前後 snapshot 比較で **image tag のみ変更、それ以外は完全一致**:

**Backend (Cloud Run service)**:
- env: plain / secretKeyRef の name / count（10 entries 期待）
- annotations: `cloudsql-instances` / `maxScale` / `startup-cpu-boost`
- container args / command（None / 既定）

**Backend (Cloud Run jobs)**:
- `vrcpb-image-processor` args: `--all-pending --max-images 10 --timeout 60s`
- `vrcpb-outbox-worker` args: `--once --max-events 1 --timeout 60s`
- 両 Job の image tag を新 commit に同期（Backend deploy 時のみ）

**Workers**:
- bindings: `OGP_BUCKET (R2 vrcpb-images)` + `ASSETS`
- Workers Secrets: `[]`（不変）
- compatibility_flags / compatibility_date / name

**Scheduler**:
- `vrcpb-image-processor-tick`: `* * * * *` ENABLED

### 6. Cloud Build / Cloud Run / wrangler logs Secret 漏洩 grep

検査パターン (`security-guard.md` の禁止リスト):
```
DATABASE_URL=
TURNSTILE_SECRET_KEY=
R2_SECRET_ACCESS_KEY=
R2_ACCESS_KEY_ID=
REPORT_IP_HASH_SALT
sk_live_
sk_test_
password=
manage_url=
storage_key=
reporter_contact=
source_ip_hash=
turnstile_token=
salt=
```

唯一許容される hit:
- Cloud Run `cloudaudit.googleapis.com/system_event` audit log の `secretKeyRef.name` メタデータ（参照名のみ、raw 値なし。revision 作成イベントの仕様）

**Frontend production chunk**: 上記パターンで grep → 0 件必須

### 7. 報告で raw 値を出さない

raw photobook_id / image_id / token / Cookie / storage_key / Secret / presigned URL は **deploy 完了報告から完全除外**。dummy 値のみ使用:
- slug: `aaaaaaaaaaaaaaaaaa`
- UUID: `00000000-0000-0000-0000-000000000000`
- token: `invalid-token-zzzzz`

GCP / Cloudflare の identifier（revision 名 / build ID / version ID / image tag）は運用情報なので出して可。

### 8. follow-up の再記録

deploy 中に発見した「次の deploy 前までに片付けるべき項目」を `harness/failure-log/` 起票 + Task 化 + 完了報告 §「follow-up 状況」に記録。

## チェックリスト（コピー用）

deploy 完了報告に以下のチェックリストを必ず含める:

- [ ] **1.** revision / version active 100% 確認、rollback target 控え済
- [ ] **2.** 5〜10 分 routing 安定化 wait 実施
- [ ] **3.** post-deploy smoke（既存 routes regression + 新機能 direct verification）全 PASS
- [ ] **4.** production bundle marker grep（新 marker 含む / 旧 antipattern 含まない）
- [ ] **5.** bindings / Secrets / env / Cloud SQL / Job args / Scheduler 維持確認、Job image tag 同期
- [ ] **6.** Cloud Build / Cloud Run / wrangler logs Secret grep 0 件
- [ ] **7.** 報告で raw 値を一切出していない（dummy 値のみ）
- [ ] **8.** follow-up 再記録（次 deploy 前までに片付けるべき項目）

## Why

2026-05-03 STOP α 調査で「3 deploy（β-3 / a8fe0db / 9c4fb7d）にわたって publish が動かなかった」事象が発覚。各 deploy の完了報告は形式上通っていたが、deploy 直後に実 production の publish 試行を smoke していれば早期検知できた可能性があった。

事故クラスは「deploy 完了基準が浅い」設計レベルの問題。本ルール + STOP 完了報告での checklist 強制で再発防止。

## 関連

- `docs/runbook/backend-deploy.md` §1.4 / §1.4.1 / §1.4.2
- `.agents/rules/pr-closeout.md` PR 完了処理（似た思想を deploy にも展開）
- `.agents/rules/security-guard.md` Secret 禁止リスト
- `harness/failure-log/2026-05-03_predeploy-bundle-marker-checklist.md`
- 過去 STOP γ / STOP δ / STOP δ2 / STOP α / P0 v2 完了報告

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-03 | 初版作成。STOP δ / STOP α 経験を deploy verification ルール化 |
