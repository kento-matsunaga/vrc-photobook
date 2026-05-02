# image processing が進んでいるか UI で分からない

## 発生日

2026-05-02 STOP ε 以前。
原因実装は β-2 / β-3 の前。

## 症状

画像 upload 完了後、image-processor が走って display / thumbnail variant を生成するまで `/prepare` の tile は「処理中」表示のみ。何枚中何枚完了したか、通常どれくらいかかるか、何分超過すると遅延扱いか、がユーザに見えなかった。長時間（5〜10 分）待たされたユーザは「壊れた」と判断して reload / 再 upload に走る → さらに状況悪化。

## 根本原因

UI の progress 表示が「処理中: N 枚」のみ。完了率（n/m）も時間経過 hint も無く、また 10 分超過時の「遅延通知」も無かった。Backend 側の image-processor Cloud Run Job tick が 1 min 間隔という仕様もユーザに伝わっていなかった。

事故クラス: **長時間バックグラウンド処理の UX で「現在地」「想定所要」「異常閾値」を表示しない設計**。

## 修正

Frontend (β-3, commit f455fe4) に 3 種の表示を追加:

| 要素 | 内容 | data-testid |
|---|---|---|
| n/m progress | 「進捗 X / Y」（local + server placed の合算） | `prepare-progress` |
| 通常案内 | 「画像の処理は通常 1〜2 分ほどで完了します。画面を開いたままお待ちください。」 | `prepare-normal-notice` |
| 遅延通知 | 「画像の処理に時間がかかっています（10 分以上）。混み合っている可能性があります。一度ブラウザを再読み込みしてもこれまでの進捗は保持されます。」（10 分超過で表示） | `prepare-slow-notice` |

ImageTile の処理中文言も「最大 5 分ほどお待ちください」→「通常 1〜2 分ほどで完了します」に更新。

## 追加した test

`frontend/app/(draft)/prepare/[photobookId]/__tests__/PrepareClient.test.tsx`:
- 「正常_processingCount > 0 のとき通常 1〜2 分案内が出る」
- 「正常_progress UI に n/m 表示が出る」
- 「正常_processing 中のとき normal 通知（10 分未満）が出る、slow 通知は出ない」

各 data-testid の存在 + 文言を SSR markup で確認。

## 今後の検知方法

- 文言や testid が消えたら SSR test が落ちる。
- 「reload で進捗が保持される」という保証は #2026-05-03_prepare-reload-queue-loss.md 側で担保。

## 残る follow-up

- 10 分閾値到達後の behavior test（現状は閾値前の SSR markup のみ）
- Backend 側の image-processor 実 latency を観測して 1〜2 分案内が現実と合うかの裏取り

## 関連

- `harness/failure-log/2026-05-03_prepare-reload-queue-loss.md`
- `docs/plan/m2-prepare-resilience-and-throughput-plan.md` §3.4 P0-c progress UI
