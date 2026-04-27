# 2026-04-28 Cloud Build deploy で traffic が新 revision に流れない（rollback drill 後の pin）

## 発生状況

- **何をしようとしていたか**: PR30 Outbox 実装の image を Cloud Build manual submit
  経由で本番 Cloud Run (`vrcpb-api`) にデプロイし、新 revision に traffic 100% を
  載せる（PR29 で確立した 1-stage 切替方式）。
- **どのファイル/モジュールで発生したか**:
  - `cloudbuild.yaml`（PR29 で作成、`gcloud run services update --image=` のみ）
  - Cloud Run service `vrcpb-api`（asia-northeast1）の traffic 設定

## 失敗内容

- Cloud Build (`1a9c9a35-5594-4852-9ae7-fa231ac5ccee`) は build / push / deploy /
  smoke のすべての step が **SUCCESS**（duration 3M36S）。
- 新 image `vrcpb-api:019f1d4` は AR に push 済、新 revision `vrcpb-api-00012-6g4`
  も Cloud Run に作成済（`gcloud run revisions list` で確認）。
- しかし `gcloud run services describe vrcpb-api --format='value(status.traffic[0].revisionName,status.traffic[0].percent)'`
  は **`vrcpb-api-00011-xfd` 100%**（旧 revision のまま）を返す。
- 独自ドメイン `https://api.vrc-photobook.com/...` は旧 revision を返している。
- cloudbuild.yaml の smoke step も独自ドメインを叩いているため、**旧 revision を見て
  200 を返した**（false positive で build SUCCESS 扱い）。

## 根本原因

- PR29 STOP 6 で実施したロールバックドリルで `gcloud run services update-traffic
  --to-revisions=vrcpb-api-00011-xfd=100` を実行 → Cloud Run の traffic 設定が
  「特定 revision に pin」状態のまま終了していた。
- pin 状態では `gcloud run services update --image=...` だけでは新 revision に
  traffic が流れない（pin が優先される、Cloud Run の仕様）。
- `cloudbuild.yaml` の deploy step は `services update --image=` のみで、`--to-latest`
  に相当する明示切替が無かった（そもそも `services update` には `--to-latest` フラグ
  が存在せず、`gcloud run services update-traffic --to-latest` を別途呼ぶ必要がある）。
- ロールバックドリル後に traffic を `--to-latest` に戻していなかった運用漏れと、
  `cloudbuild.yaml` 側で pin 解除が組まれていなかった設計不足の二段の失敗が重なって
  PR30 deploy 時に初顕在化した。

## 影響範囲

- 表面的には「新 image が build / push されたのに本番に反映されない」障害。最悪、
  hotfix を急いで deploy したつもりが旧コードのままで顧客影響が長引く可能性。
- Cloud Build の build success と「実本番が新 revision で稼働している」が**等価でない**
  ことが今回のサイクルで確認された。今後の監視 / runbook / 完了報告基準を更新する必要。
- 独自ドメイン経由 smoke が pin 状態でも 200 を返す → smoke 単体ではこの種の
  false positive を検出できない。
- 今後同等の rollback drill / 一時的な traffic 操作を行うと再発する可能性が高い。

## 対策種別

- [x] ルール化（必須事項の追加）
  - `docs/runbook/backend-deploy.md` §1.4 に traffic 一致確認を必須化、§2.2 に
    rollback 後の pin 効果を明記、§5.7 に FAQ を追加
  - `docs/plan/m2-backend-deploy-automation-plan.md` §4.3 に traffic pin 状態の
    挙動と対策を追記
- [x] スキル化（手順の自動化）
  - `cloudbuild.yaml` に `traffic-to-latest` step (id: `traffic-to-latest`) を deploy
    と smoke の間に挟む。これにより pin 状態でも必ず latest revision に traffic 100%
    を向ける（冪等動作なので通常運用にも副作用なし）
- [ ] テスト追加 — 該当なし（実環境 Cloud Run の挙動なので CI で再現困難）
- [ ] フック追加 — 該当なし（runbook の必須チェックでカバー）

### 暫定対応（PR30 deploy 時に実施済）

- ユーザー承認のもと `gcloud run services update-traffic vrcpb-api
  --to-revisions=vrcpb-api-00012-6g4=100` を実行 → 新 revision に traffic 100% 反映
- その後 smoke / 認可 / Secret 漏洩 grep を新 revision 経由で再検証 → 全 OK
- 詳細は `harness/work-logs/2026-04-28_outbox-result.md` の STOP B 章

### 恒久対応（本タスクで実施）

- `cloudbuild.yaml` に `traffic-to-latest` step を追加（commit に含む）
- runbook + 計画書を更新
- 本 failure-log を作成

### 再発防止（運用ルール化）

- **Cloud Build deploy 完了後は `latestReadyRevisionName == status.traffic[0].revisionName`
  の一致を必ず確認する**。build SUCCESS だけで deploy 成功とみなさない（false positive
  対策）。
- rollback drill / 一時的な traffic 操作を行った場合、終了時に必ず `update-traffic
  --to-latest` で pin 解除する（または次回 Cloud Build deploy で `traffic-to-latest`
  step が解除する）。
- runbook §1.4 / §5.7 を Cloud Build deploy のたびに参照する。

## 教訓

- **build success と deploy 成功は別物**。Cloud Build / smoke が SUCCESS でも、traffic
  設定が古い revision に pin されていると本番は新コードで動かない。完了判定には
  必ず traffic 配分（revision 名一致）まで含める。
- Cloud Run の `gcloud run services update --image=` は新 revision を作るだけで、
  traffic 切替は別レイヤー（pin 状態に依存）。`gcloud run services update` には
  `--to-latest` フラグが**存在しない**ため、明示的に `update-traffic --to-latest`
  を呼ぶ必要がある（公式 help が `update-traffic` 側を案内している）。
- ロールバックドリルや一時 pin 等の「片付けが必要な状態」を作ったら、そのターン内に
  必ず元に戻す。後続セッションで顕在化すると原因切り分けが面倒になる。
- 今回は「PR29 のロールバックドリル後始末漏れ」「`cloudbuild.yaml` の pin 解除欠如」
  の二段の失敗が PR30 deploy で初めて顕在化した。**1 つの失敗が独立に検知不能でも、
  別の失敗と組み合わさって表に出る**ことを覚えておく。
- 一見冗長に見える `update-traffic --to-latest` step は、本ケースのような
  「過去 state の意図せぬ持ち越し」に対するチェックポイントとして機能する。

## 関連

- 暫定対応: `harness/work-logs/2026-04-28_outbox-result.md`（PR30 STOP B 章）
- 恒久対応 commit: 本 failure-log と同一 commit
- runbook: `docs/runbook/backend-deploy.md` §1.4 / §2.2 / §5.7
- 計画書: `docs/plan/m2-backend-deploy-automation-plan.md` §4.1 / §4.3
- 関連 PR: PR29（manual submit 方式採用 + ロールバックドリル実施）/ PR30（本事象が
  顕在化したサイクル）
- 関連ルール: `.agents/rules/feedback-loop.md`
