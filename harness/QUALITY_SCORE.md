# 品質スコア

## 概要

モジュール × レイヤーごとの品質を可視化し、改善優先度を判断する。

## スコア基準

| グレード | 基準 |
|---------|------|
| A | ルール準拠、テスト充実、ドキュメント整合 |
| B | 概ね準拠、軽微な改善点あり |
| C | 一部ルール違反あり、テスト不足 |
| D | 重大なルール違反、テスト欠如 |
| F | 未評価 / 評価不能 |

## スコアボード

> **本表は M1 PoC（`harness/spike/`）の暫定スコアである**。本実装ディレクトリ `backend/` / `frontend/` は M2 着手後に別行で評価する。
> PoC は意図的にテストを書かない方針（`harness/spike/backend/README.md` 参照）のため、テスト欄は構造的に低スコアになる。

| モジュール | ドメイン | インフラ | ユースケース | コントローラー | テスト | ドキュメント | 総合 |
|-----------|---------|---------|------------|-------------|-------|------------|------|
| spike-frontend（OpenNext / Workers） | — | B | — | B | F | A | B |
| spike-backend（Go chi / pgx / sqlc） | — | B | — | B | F | A | B |
| spike-r2-upload（presigned PUT / HeadObject） | — | B | — | B | F | A | B |
| spike-turnstile-upload-verification（siteverify / SHA-256 / consume） | — | B | — | B | F | A | B |
| spike-outbox-reconciler（FOR UPDATE SKIP LOCKED / retry） | — | B | — | B | F | A | B |

### スコア注記

- **ドメイン / ユースケース欄が `—` の理由**: PoC は意図的に集約構造を持たず、`internal/sandbox/` に直書きの handler 群だけを置く設計。本実装（M2）では `domain-standard.md` 構造に従って再設計する
- **テスト欄が一律 F の理由**: PoC は「動作確認だけ」で完結させる方針（`harness/spike/backend/README.md` §既知の制限）。本実装でテーブル駆動 + Builder（`testing.md` 準拠）を必須化
- **ドキュメント欄 A の根拠**: 各 PoC が README で検証手順 / 検証結果 / 制限を網羅している、ADR と相互参照されている

### 主要 PoC の達成内容（2026-04-25 時点）

#### spike-frontend
- Next.js 15 App Router + OpenNext (`@opennextjs/cloudflare`) で Cloudflare Workers + Static Assets binding 構成成立
- SSR / OGP / `noindex` / `Referrer-Policy` 出し分け / `Set-Cookie` (HttpOnly / Secure / SameSite=Strict) / 302 redirect / Server Component での Cookie 読取をすべて成立
- macOS Safari / iPhone Safari 実機検証成功

**未確認事項**:
- Cloudflare Workers 実環境（`*.workers.dev`）デプロイ
- 24 時間後 / 7 日後 Safari ITP 影響評価
- Backend と異なるホスト下での Cookie Domain（U2、ADR-0003）

#### spike-backend
- Go 1.24 + chi v5 + pgx v5 + sqlc v1.30 + goose v3.22 で Cloud Run 互換 Docker image（distroless static-debian12:nonroot）成立
- `/healthz`（DB 不要 200）/ `/readyz`（DB 接続確認 503/200）/ graceful shutdown / slog JSON
- DB 未設定でも起動継続（pool nil / queries nil ガードで sandbox は 503 を返す設計）
- Docker image 26.3MB（api 15MB + outbox-worker 8.8MB + base、2026-04-26 2 バイナリ化後）

**未確認事項**:
- Cloud Run 実環境デプロイ（コールドスタート / Cloud SQL 接続 / Cloud Logging slog JSON / SIGTERM graceful shutdown）
- Cloud Run 東京 ↔ R2 レイテンシ計測

#### spike-r2-upload
- `aws-sdk-go-v2/service/s3` v1.100 で R2 S3 互換 API 接続成立
- HeadBucket / ListObjects / presigned PUT（15 分有効）/ R2 への実 PUT / HeadObject / バリデーション 8 ケース成立
- `Content-Length` を SignedHeaders に含む挙動を実機で観測（宣言サイズと実 PUT サイズの一致が必要、不一致時 `403 SignatureDoesNotMatch`）
- ログ漏洩 grep 0 ヒット（presigned URL / Secret / storage_key）

**未確認事項**:
- Cloud Run 上での R2 アクセス（Step A-6 で確認予定）
- 期限切れ presigned URL の挙動（15 分待機が必要）

#### spike-turnstile-upload-verification
- Cloudflare Turnstile siteverify を公開サンドボックス secret（always-pass / always-fail）で実呼び出し成立
- 32 バイト乱数 → base64url → SHA-256 → bytea 32B として `upload_verification_sessions` に保存（raw token は DB に残さない）
- 単一 SQL の 5 条件 AND UPDATE で原子消費。逐次 21 回検証で `[1〜20] 200 / [21] 403`
- 100 並列 race で `success=20 / forbidden=80`、PostgreSQL Read Committed の単一行 UPDATE で原子性確認
- 拒否カテゴリは `consume_rejected` の単一カテゴリに集約、攻撃者は脱落条件を判別できない
- secret / Turnstile token / verification_session_token のログ漏洩 0 ヒット

**未確認事項**:
- Cloud Run 上での siteverify 呼び出し（DB 必須のため Step B 以降）
- 本番 widget 発行（Workers hostname 確定後の M2 早期）

#### spike-outbox-reconciler
- `outbox_events` テーブル + CHECK 制約 + 部分インデックス + sqlc 生成 8 クエリ
- CTE 内 `FOR UPDATE SKIP LOCKED` + `processing` への原子 UPDATE で claim
- enqueue → claim → processed / failed → `outbox_failed_retry` で pending 再投入 → 再 claim
- 30 件 + 2 並列 process-once で event_ids overlap=0、最終 processed=30
- `cmd/outbox-worker --once` / `--retry-failed` CLI、`scripts/outbox-process-once.sh` ラッパー
- payload / Secret / `last_error` のログ漏洩 0 ヒット

**未確認事項**:
- Cloud Run Jobs + Cloud Scheduler 実環境起動（U11、Step A-? / Step 11 で確認予定）
- 指数バックオフ（M2 で実装）
- 保持期間クリーンアップ（processed=30 日 / processing 1 時間滞留）

## 改善履歴

| 日付 | モジュール | レイヤー | Before → After | 理由 |
|------|-----------|---------|---------------|------|
| 2026-04-26 | spike-backend | インフラ | F → B | M1 PoC 完了、Cloud Run 互換 Docker image 成立、2 バイナリ化（コミット `9e6a4f6`）+ `.dockerignore` 追加（`36a1e93`） |
| 2026-04-26 | spike-r2-upload | インフラ | F → B | R2 S3 API 実接続検証完了、ログ漏洩 0（コミット `83cf628`） |
| 2026-04-26 | spike-turnstile-upload-verification | インフラ / コントローラー | F → B | Turnstile siteverify + SHA-256 永続化 + 100 並列 race 成立（コミット `53fa568`） |
| 2026-04-26 | spike-outbox-reconciler | インフラ / コントローラー | F → B | outbox_events + 自動 reconciler 最小実装、二重処理防止確認（コミット `91be6de`） |
| 2026-04-26 | spike-frontend | インフラ / コントローラー / ドキュメント | F → B | OpenNext + Workers + Safari 実機検証成立 |

## 使い方

1. 新モジュール追加時にスコアボードに行を追加
2. self-verification 後にスコアを更新
3. C 以下のセルを優先的に改善
4. 改善時は履歴に記録

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-26 | 初回反映。M1 PoC 5 モジュール（spike-frontend / spike-backend / spike-r2-upload / spike-turnstile-upload-verification / spike-outbox-reconciler）を追加。スコア注記と未確認事項を記録 |
