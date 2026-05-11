# outbox-worker Job に Scheduler 発火 SA の `roles/run.invoker` が無く、Scheduler 初回 attempt が 403 で失敗

## 発生状況

- 2026-05-11 STOP γ にて `vrcpb-outbox-worker-tick` Cloud Scheduler を新規作成
  （ADR-0007 OGP 同期 publish の outbox fallback）
- Scheduler は `state=ENABLED` / `schedule='* * * * *'` で正常起動
- 同 SA (`271979922385-compute@developer.gserviceaccount.com`) で動いている
  `vrcpb-image-processor-tick` は正常動作中

## 失敗内容

- Scheduler の最初の 2 attempt（01:53 / 01:54 UTC）が **HTTP 403 で失敗**
- `gcloud scheduler jobs describe vrcpb-outbox-worker-tick` の `status.code=7`
  （gRPC `PERMISSION_DENIED`）
- `gcloud run jobs executions list --job=vrcpb-outbox-worker` で **新規 execution が増えない**
- Job 自体は手動 execute すれば正常動作する状態

```
# Scheduler logs (5 分):
01:54:07Z  ERROR  403  AttemptFinished
01:53:06Z  ERROR  403  AttemptFinished
```

## 根本原因

`vrcpb-outbox-worker` Cloud Run Job の IAM policy に **Scheduler 発火 SA の
`roles/run.invoker` が付与されていなかった**。

```
$ gcloud run jobs get-iam-policy vrcpb-outbox-worker --region=asia-northeast1
etag: ACAB     ← bindings 空
```

`vrcpb-outbox-worker` Job は 2026-04-28 STOP θ で **手動 execute 運用前提**で作成
された（`harness/work-logs/2026-04-28_ogp-outbox-handler-result.md`）。当時は
Scheduler を作成しなかったため、compute SA への `roles/run.invoker` 付与も不要で、
省略されていた。

STOP γ で Scheduler を新規作成した際に、Job 側 IAM 確認を行わずに作成した。
**Cloud Run Jobs の `roles/run.invoker` は Job 単位**で付与する必要があり、同
プロジェクト内の他 Job （image-processor）に付与されていても継承されない。

## 検出

- Scheduler ENABLED 後、5 分の起動観測 (STOP γ STEP 6) で executions list が増えていない
  ことに気付き、`gcloud scheduler jobs describe` の `status.code=7` を確認
- 過去の同パターン `vrcpb-image-processor` Job の IAM と比較し、bindings 差分を検出

## 修復

```bash
gcloud run jobs add-iam-policy-binding vrcpb-outbox-worker \
  --region=asia-northeast1 --project=project-1c310480-335c-4365-8a8 \
  --member="serviceAccount:271979922385-compute@developer.gserviceaccount.com" \
  --role="roles/run.invoker"
```

付与後 1 分以内に次の Scheduler attempt が HTTP 200 となり、対応 Job execution が
`succeededCount=1` で完了。直後の 3 attempts 連続 200、3 executions 連続成功で復旧確認。

## 今後の予防

### 1. runbook 化（実施済）

`docs/runbook/outbox-worker-ops.md` §4「IAM 確認手順」として、Scheduler 起動異常時の
最初の確認項目に組み込み済。`get-iam-policy` で空 bindings を検出 → `add-iam-policy-binding`
の固定手順を runbook 化。

### 2. Scheduler 新規作成時のチェックリスト

新しい Cloud Scheduler で Cloud Run Jobs を起動する際は、**Scheduler 作成前**に必ず:

- 対象 Job の `get-iam-policy` を取得
- 発火 SA に `roles/run.invoker` があるか確認
- 無ければ `add-iam-policy-binding` で先に付与してから Scheduler を作成

### 3. 一般化された教訓

> **手動 execute 前提で作成された Cloud Run Job を後から Scheduler 化する場合、
> Scheduler 発火 SA の `roles/run.invoker` が Job 側 IAM に欠落している前提で扱う。**

過去の Job 作成時の IAM 設計は「手動 execute する人間 SA」を基準にしているため、
非対話 SA からの invoke 経路は別途付与が必要。同種の Job 自動化（reconcile worker / 他）
を行う際は本パターンに従う。

## 影響範囲

- 一時的: STOP γ STEP 6 で Scheduler 起動が 2 分間 (2 attempts) 失敗
- 恒久: 修復後の運用には影響なし（Job 単位の `roles/run.invoker` 付与は冪等で
  rollback 不要）
- 本番ユーザ影響: 無し。本 Scheduler は新規作成のため STOP γ 完了まで production
  flow には組み込まれておらず、publish 同期 OGP 自体は handler 内で動作する別経路

## 対策種別

- [x] ルール化: runbook §4 と本 failure-log にまとめ。`outbox-worker-ops.md` §6.1
      障害対応の最初の項目に組み込み
- [x] スキル化（手順固定）: runbook §4.2 の `add-iam-policy-binding` ワンライナー
- [ ] テスト追加: 該当なし（gcloud / IAM の状態確認系で、コードレベルの test 化困難）
- [ ] フック追加: 該当なし

## 関連

- `docs/runbook/outbox-worker-ops.md` §4 / §6.1
- `docs/plan/m2-ogp-sync-publish-plan.md` §5 STOP γ receipt（γ-fix 項）
- `docs/adr/0007-ogp-sync-publish-fallback.md`
- `docs/plan/m2-image-processor-job-automation-plan.md` §5.1（image-processor 側
  Scheduler 構成、`roles/run.invoker` 付与必須の記載）
- `harness/work-logs/2026-04-28_ogp-outbox-handler-result.md`（手動 execute 前提で
  Job を作った時の記録）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-11 | 初版作成。STOP γ Scheduler 5 分起動観測で発見、`gcloud run jobs add-iam-policy-binding` で復旧、runbook + plan + failure-log にまとめ |
