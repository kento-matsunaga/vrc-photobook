# 2026-04-26 Cloud Run 上で `/healthz` (lowercase) が Google Frontend に intercept される

## 発生状況

- **何をしようとしていたか**: M1 実環境デプロイ A-5 で `harness/spike/backend/` の Cloud Run service `vrcpb-spike-api` をデプロイ後、`GET /healthz` で 200 を期待していた。
- **どのファイル/モジュールで発生したか**: Cloud Run service（`asia-northeast1`）への HTTPS リクエスト。Backend は chi `r.Get("/healthz", health.Healthz)` で `/healthz` ハンドラを登録済。

## 失敗内容

- `curl https://vrcpb-spike-api-xxx.run.app/healthz`
  - **HTTP/2 404**
  - `content-type: text/html` の **Google エラーページ**（`<title>Error 404 (Not Found)!!1</title>` / 1568 bytes / `referrer-policy: no-referrer`）
  - 通常の Cloud Run Backend 404 は `content-type: text/plain` なので **応答主体が違う**
- **Cloud Run access log（`gcloud logging read resource.labels.service_name=vrcpb-spike-api`）にこのリクエストが載らない**。一方で `/readyz` / `/sandbox/*` のリクエストはすべて access log に載る
- 比較確認:
  - `GET /HEALTHZ`（大文字）→ HTTP 404 / `content-type: text/plain` / `x-cloud-trace-context` あり = **Backend に到達した chi 標準 404**
  - **`/healthz` 小文字だけ**が Google Frontend (GFE) 層で 404 として吸い込まれている
- HTTP/1.1 強制 / User-Agent 変更でも結果は同じ
- ローカル `docker run` では `/healthz` は **200** を返していた（コミット `9e6a4f6` でビルドした image を実環境にそのまま push、コード変更なし）

## 根本原因

- Cloud Run / GFE 層の予約・特殊扱いのパスとして `/healthz` (lowercase) が intercept されている可能性が高い
- 公式ドキュメントには明示記載が見つからないが、複数の事例（[Stack Overflow: Google Cloud Run weird behaviour only for path /healthz](https://stackoverflow.com/questions/79472006/google-cloud-run-weird-behaviour-only-for-path-healthz)）で同じ事象が報告されており、回避策として `/health` 等のリネームが推奨されている
- ローカル環境では Backend の chi が直接ルーティングするため再現せず、**Cloud Run 実環境特有の挙動**

## 影響範囲

- Cloud Run 上で `/healthz` に依存した監視・startup probe・liveness probe 設定が機能しない
- 本実装（M2 以降）の Cloud Run / Cloud Run Jobs / Cloud Functions 等の Google Cloud サーバーレス系すべてで同種の影響可能性あり
- 計画書 / ADR / README で `/healthz` を Cloud Run 公開パスとして案内すると混乱を招く

## 対策種別

- [x] ルール化（必須事項の追加）
  - `docs/plan/m1-live-deploy-verification-plan.md` § 該当箇所と `docs/adr/0001-tech-stack.md` §M1 検証結果に「Cloud Run 上では `/healthz` を使わず `/health` を正式採用」を追記
- [x] スキル化 — 該当なし（運用ルールで足りる）
- [ ] テスト追加 — 検出は実環境デプロイ後の curl で十分、自動化は M2 以降
- [ ] フック追加 — 該当なし

### コードへの反映

- `harness/spike/backend/cmd/api/main.go` で **`/health` を新設**（`health.Healthz` を共有）
- **`/healthz` はローカル PoC / 既存 README 互換のため残す**
- これにより:
  - Cloud Run / 本番監視 / startup probe / liveness probe では `/health` を使う
  - ローカル `docker compose` / `go run ./cmd/api` では引き続き `/healthz` も使える

### 運用ルール（M2 以降の本実装にも継承）

- **Cloud Run / Cloud Run Jobs にデプロイするバイナリでは、ヘルスチェックパスに `/healthz` を使わない**
- 推奨パス: `/health` / `/_health` / `/api/health` 等
- `/healthz` をローカル開発用に併設する場合は、**Cloud Run 上では到達しない前提**で運用する
- ADR / 計画書のヘルスチェックに関する記述は「`/health`」に統一する

## 教訓

- **ローカルで動いても Cloud Run 実環境で再現するとは限らない**。実環境デプロイ後の curl 確認は必須。
- Cloud Run の予約・特殊扱いパスは公式ドキュメントだけでは網羅されておらず、コミュニティ事例（Stack Overflow / Issue Tracker）も併用して検証する
- ヘルスチェックパスは「文字列 1 つ違う」だけで本番影響が出るため、**実環境で確実に Backend へ届くパス**を選ぶ
- `/healthz` は Kubernetes の慣習だが、Cloud Run（Knative ベースのマネージド環境）では事情が異なる可能性を意識する

## 関連

- A-5 Cloud Run deploy 完了報告（本セッション 2026-04-26）
- `harness/spike/backend/cmd/api/main.go`（`/health` 追加コミット）
- `docs/plan/m1-live-deploy-verification-plan.md` §「ヘルスチェックパス」追記
- `docs/adr/0001-tech-stack.md` §M1 検証結果（Backend / Cloud Run）追記
