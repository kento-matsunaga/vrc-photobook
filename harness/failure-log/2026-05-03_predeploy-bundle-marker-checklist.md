# deploy 前に bundle marker / production route / log status の確認が不十分

## 発生日

2026-05-02 STOP δ / STOP α サイクルで観測。

## 症状

過去複数回、「deploy したつもり」だが実は bundle に新コードが含まれていなかった / 旧 chunk が CDN にキャッシュされていた / route smoke が通っただけで実機で動かなかった、という事象が発生。具体例:

- STOP δ deploy 後 / β-2 commit がまだ Backend のみで、Workers 側の bundle に attach-images 呼出が含まれていないことを deploy 完了後に発見（β-3 が未着手だった、commit scope 整理ミス）
- STOP α 調査で /edit が動かない 3 症状を deploy 前に検知できなかった（Cloud Run logs `/settings PATCH 0 件` を deploy 後の本番動線で初めて観測）
- a8fe0db 後の publish が依然 rights_agreed gate で全 block されることを 9c4fb7d 実装直前に発見（4 番目の bug）

事故クラス: **deploy の「動いた / 動かない」を実 production の bundle 内容 + route + log で確認せず、build / unit test の通過だけで完了扱いしてしまう**。

## 根本原因

deploy 完了の判定基準が:
- `Cloud Build SUCCESS` (= image build OK)
- `wrangler deploy` が `Current Version ID: ...` を出力
- `wrangler deployments list` で Active 100%
- `/health` `/readyz` 200

までだったが、

- **新 chunk が production CDN から実 fetch できるか** （`curl https://app.../_next/static/chunks/...js`）
- **新コードの marker（API path / 文言 / data-testid）が chunk に含まれるか** （`grep`）
- **deploy 直後の Cloud Run logs に旧 vs 新 revision の routing transient が無いか** （chi default plain text 404 への退化など）
- **新機能が本番動線で実際に呼ばれているか** （logs での request URL + status カウント）

を確認していなかった。

特に β-2 → β-3 の境界で「Backend deploy 完了 = 機能 live」と勘違いしたため、Workers 側を見ずに STOP ε に進もうとした。

## 修正

### 既に β-3 / STOP δ2 / STOP α / P0 v2 deploy で実施済の確認手順

新規 Backend deploy / Workers deploy の完了基準を以下まで拡張（runbook 化済 / 本 failure-log で正典化）:

1. **revision / version active 100% 確認**
   - `gcloud run services describe vrcpb-api ... format=json` → `latestReadyRevisionName == traffic[*].revisionName(percent=100, latestRevision=true)`
   - `wrangler deployments list --name vrcpb-frontend` → 最新 deployment の Version 100%

2. **5〜10 分 routing 安定化 wait**（runbook §1.4.1）

3. **post-deploy smoke**:
   - 既存 routes regression（/health, /readyz, /, /create, /about, /terms, /privacy, /help/manage-url, /draft/<bad>, /prepare/<dummy>, /edit/<dummy>, /p/<dummy>, /p/<dummy>/report, /ogp/<dummy>）
   - **新機能の direct verification**（例: CORS preflight で PATCH/DELETE Allow-Methods 含む / publish request body で rights_agreed:false → 409 + reason=rights_not_agreed）

4. **production bundle marker grep**
   - 新 chunk path を curl で fetch（必要に応じ URL-encoded path で 307 redirect 経由）
   - 新機能の文字列 marker（API path / data-testid / UI 文言 / 重要 keyword）が chunk に含まれることを `grep -oE` で確認
   - 旧 antipattern marker が含まれないことも確認（例: 旧固定文言「公開条件に合致しません」）

5. **bindings / Secrets / env / Cloud SQL / Job args / Scheduler interval 維持確認**
   - deploy 前後で snapshot 比較
   - `image tag` のみ変更されたことを確認、それ以外は完全一致

6. **Job image tag 同期**（Backend deploy 時）
   - `gcloud run jobs update vrcpb-image-processor --image=<new>` / `vrcpb-outbox-worker` も同 tag に揃える
   - args 維持確認

7. **Secret / raw 値 grep**
   - Cloud Build log / wrangler deploy log / 新 revision の Cloud Run logs / production chunk file
   - 検査パターン: `DATABASE_URL=` / `TURNSTILE_SECRET_KEY=` / `R2_SECRET_ACCESS_KEY=` / `REPORT_IP_HASH_SALT` / `sk_live_` / `sk_test_` / `password=` / `manage_url=` / `storage_key=` / `reporter_contact=` / `source_ip_hash=` / `turnstile_token=` / `salt=`
   - 唯一許容される hit は Cloud Run system_event audit log の `secretKeyRef.name` メタデータ（参照名のみ、raw 値なし）

8. **報告で raw 値を出さない**
   - raw photobook_id / image_id / token / Cookie / storage_key / Secret / presigned URL は報告から完全除外
   - dummy 値（slug `aaaaaaaaaaaaaaaaaa`、UUID `00000000-0000-0000-0000-000000000000`、token `invalid-token-zzzzz`）のみ使用

## 追加した rule

`.agents/rules/predeploy-verification-checklist.md`（新規）に上記 8 項目をチェックリスト化。

## 追加した test

deploy 自動化はしていないため自動 test は無いが、各 STOP 完了報告で本 checklist 全項目への記録を**必須化**した。

## 今後の検知方法

- deploy 完了報告に checklist 8 項目すべての結果が含まれない場合は完了扱いにしない（PR-closeout 同様）
- 新機能を deploy する PR は post-deploy smoke で「新 marker が production bundle に含まれる」評価を必須

## 残る follow-up

- 自動化: post-deploy smoke を CI / Cloud Build 内で実行できるよう検討（PR40 ローンチ前運用整理）
- 本番 status / latency / Secret grep の dashboard 化（Grafana / Cloud Logging Sink）

## 関連

- `docs/runbook/backend-deploy.md` §1.4 / §1.4.1 / §1.4.2
- `.agents/rules/predeploy-verification-checklist.md`
- 過去 STOP γ / STOP δ / STOP δ2 / STOP α 完了報告
