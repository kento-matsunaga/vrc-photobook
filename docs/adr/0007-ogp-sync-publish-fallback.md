# ADR-0007 OGP 同期生成 + outbox fallback ハイブリッド方式

## ステータス

**Proposed（2026-05-11）**

実装計画書 [`docs/plan/m2-ogp-sync-publish-plan.md`](../plan/m2-ogp-sync-publish-plan.md) と
同時起票。STOP β 実装着手前の最終承認をもって **Accepted** に遷移する。

---

## 文脈

業務知識 v4 §3.2 / §3.8 で OGP は「公開時に自動生成し X / Discord / Slack 等の SNS で見やすい
形で共有できる」と定義されている。`docs/design/cross-cutting/ogp-generation.md` および
`docs/design/cross-cutting/outbox.md` §6.2 では、`PhotobookPublished` outbox event を
非同期 worker (`vrcpb-outbox-worker`) で処理し OGP を生成する非同期前提の設計が採用されている。

しかし MVP 運用上、以下 2 つの問題が観測された:

1. **outbox-worker は Cloud Scheduler 未設定で運営手動 execute 運用**（CLAUDE.md / final-roadmap
   §1.3「Cloud Scheduler 作成（outbox-worker 自動回し）→ 当面は手動 Job execute、PR33e で
   要否判断」）。worker が動いていない時間帯に publish された photobook の OGP は **手動 execute
   までずっと未生成**のままになる。
2. **publish UC の `WithTx` 内で `photobook_ogp_images` 行を INSERT していない**
   (`backend/internal/photobook/internal/usecase/publish_from_draft.go`)。pending 行は
   worker 側 `GenerateOgpForPhotobook.CreatePending` で初めて作成される。よって publish 直後の
   crawler 1st access は **`photobook_ogp_images` row 不在 → not_found → `/og/default.png`
   302 redirect** を踏む。

具体事象として 2026-05-10 publish 直後に X に貼ったところ **teal の default placeholder** が
表示された（手動 Job execute が間に合わなかった）。**X / Twitter / Discord 等の crawler は
OGP を数日〜1 週間 cache** するため、後追い generated 化では救えない。X Card Validator で
手動 refresh は可能だが user 操作が必要で現実的でない。

---

## 制約と前提

- **業務知識 v4 §3.2「OGP 失敗でも公開自体は成功させる」を維持する**: publish API は OGP 失敗で
  rollback しない（user の公開意思は OGP 副作用に依存させない）
- **`docs/design/cross-cutting/outbox.md` §2「副作用は非同期ワーカーで実行」原則**: 例外を許容する
  場合は ADR で明文化する（本 ADR が当該例外）
- **publish レイテンシ SLO 不在**: ADR-0001 では publish 数値 SLO 未定義。本 ADR で
  「publish API 追加レイテンシ p50 < 300 ms / p95 < 1000 ms / timeout 2.5 s」を初期目標
  として宣言する
- **renderer は pure Go**（`image/draw` + `opentype` + `imaging`、cgo 不使用）。STOP α benchmark
  で warm 10-13 ms / op を実測（i7-14700F）、Cloud Run 1 vCPU 換算 **25-65 ms**
- **OGP renderer は cover image を取込んでいない**（`generate_ogp.go:166` "PR33b では取得しない
  （fallback 描画）"）。cover 取込み本実装は別 ADR / 別 STOP で扱う
- **`vrcpb-image-processor-tick` Cloud Scheduler は既存**（image-processor の自動回し、`* * * * *`
  ENABLED）。outbox-worker でも同パターンの Scheduler を作る素地はある

---

## 検討した選択肢

| 案 | 概要 |
|---|---|
| **A 純同期** | publish handler の WithTx 内で OGP 生成（render + R2 PUT + DB UPDATE）。失敗で publish も rollback |
| **B Complete polling のみ** | 既存 worker 非同期維持、Complete 画面で polling して generated になるまで「共有」ボタン disable |
| **C Jobs API kick** | publish handler から Cloud Run Jobs API で outbox-worker を ad-hoc 起動 + 同期 wait |
| **D Scheduler 1 min only** | 既存 worker を Cloud Scheduler `* * * * *` で常時回す、Complete 画面で polling fallback |
| **E pre-publish 生成** | `/edit` cover 確定時に OGP を draft staging 領域に生成、publish 時は rename / metadata flip |
| **H1 ハイブリッド（採用）** | publish 同 TX で pending 行先行 INSERT + commit 後 best-effort 同期 + 失敗時 outbox fallback + Cloud Scheduler 1 min ENABLED |

---

## 決定

**H1 ハイブリッド方式を採用する**。

### 4 つの構成要素

#### 1. publish 同 TX で `photobook_ogp_images.pending` 行を先行 INSERT

publish UC の `WithTx` 内に `EnsureCreatedPending` を追加。1st crawl 時点で必ず row が存在する
状態を不変条件として確立する（row 不在による fallback 経路を消す）。

worker 側 `CreatePending` は `EnsureCreatedPending` に名前変更し冪等化（ON CONFLICT DO NOTHING）。
publish 時と worker 時のどちらから呼ばれても duplicate にならない。

#### 2. commit 後 best-effort 同期 OGP 生成

publish UC の `WithTx` commit 後・response 返却前に **context timeout 2.5 s で best-effort 実行**:

- `renderer.Render` → 期待 25-65 ms（Cloud Run warm 換算）
- R2 PUT → 期待 30-80 ms（asia-northeast1 → R2 ap 同リージョン、~50 KB PNG）
- 完了 TX（images + image_variants INSERT + `MarkGenerated`）→ 期待 20-50 ms
- **合計 expected p50 < 300 ms / p95 < 1000 ms / timeout 2.5 s**

**失敗・timeout 時は publish 200 を維持**（業務知識 v4 §3.2 整合）。同 TX で既に `outbox_events
PhotobookPublished` が pending として INSERT されているため、既存 worker chain で retry される。

#### 3. Cloud Scheduler `vrcpb-outbox-worker-tick` を常時 ENABLED

`* * * * *`（1 分間隔）、OIDC 認証、image-processor-tick と同パターン。同期失敗時の最大遅延を
**60 s 以内に SLO 化**。手動 execute 運用を恒久解除。

#### 4. Frontend Complete 画面 polling fallback

`/api/public/photobooks/{id}/ogp` を 2 s 間隔で polling（最大 30 s）。`generated` 確認後に
「共有」ボタン enable、30 s 経過時は info で「OGP は数十秒後に反映されます」+ ボタン enable
（user が早めに paste しても crawler は通常数秒〜数分の cache 取り直しで救える）。

### 受入基準

**Hard**: ユーザが共有 URL を SNS に paste → crawler が 1st access した時点で `status=generated`
が返ること。

**Soft target**:
- 同期成功率 **95% 以上**を初期 KPI（3 か月後再評価）
- 同期成功時 共有ボタン待ち時間 0 秒
- 同期失敗時 polling 最大 30 秒、それでも未 generated なら ready=false で fallback

---

## 却下した選択肢と理由

### A 純同期（OGP 失敗で publish も rollback）

業務知識 v4 §3.2「OGP 失敗でも公開自体は成功させる」と直接矛盾する。部分 commit に分岐させると
orphan 設計が増える割に、H1 と同等の hard 保証しか得られない。

### B Complete polling のみ

worker が動いていない前提のままだと **現状再現**。Scheduler 化と組合せないと soft target も
達成不可で、Hard requirement は worker 起動を待つ間ずっと保証されない。

### C Cloud Run Jobs API kick

Cloud Run **Job** cold start で publish レイテンシ +5〜10 s が決定的に重い。Cloud Run Admin API
SDK 追加 + IAM `run.jobs.run` 権限付与の工数も多い割に H1 比でメリットなし。

### D Cloud Scheduler 1 min + polling のみ（同期化なし）

Hard requirement に対して **最大 60 s race window** が残る。publish 直後の paste で teal が出る
現事象を完全には解消しない。

### E pre-publish 生成

`/edit` での cover 確定 / title 変更頻度が高く、再生成コストが嵩む。`photobook_ogp_images` の
ライフサイクル変更 + migration が重く、M-1b（destructive actions）と並行で集約境界を弄ると
blast radius が大きい。同期化が成立すれば pre-publish の優位性は薄い。

---

## 影響

### 集約境界 / 設計原則への影響

- **outbox.md §2「副作用は非同期ワーカーで実行」原則の例外を本 ADR で明文化**。同 TX outbox INSERT
  は維持し、同期化は publish response 前の **best-effort layer の追加**として扱う。失敗時は
  既存 worker chain にそのまま倒れるため、`PhotobookPublished` event の処理経路は **常に 2 通り
  存在し、どちらでも同じ最終状態に収束する**（冪等）
- Photobook 集約から OGP 集約への **同期呼出**を許容する。ただし呼出は best-effort（失敗を許容、
  publish 結果に影響させない）に限定し、強整合性は持ち込まない
- `ogp-generation.md` §5.1 `PhotobookPublished` ハンドラ手順に「publish 同 TX で pending 行を
  INSERT」を追加（`EnsureCreatedPending` 責務の所属を明確化）

### 運用への影響

- 手動 Job execute 運用の恒久解除（運営負荷削減）
- Cloud Scheduler 1 invocation/min × 24h × 30d = **月 ~43,200 起動**。Cloud Run Jobs 課金は
  image-processor-tick と同オーダー（推定 月 < $5）
- 新 runbook `docs/runbook/outbox-worker-ops.md` を追加（Scheduler 起動確認 / Job 障害対応）

### コード変更影響

- publish UC: 同 TX に `EnsureCreatedPending` 追加 + commit 後 best-effort 同期 path
- OGP UC: `CreatePending` → `EnsureCreatedPending` 冪等化 + 新 method `SyncGenerate`
- 新 port `OgpEnsurePending` / `OgpSyncGenerator`（photobook usecase ports）
- 新 adapter（photobook infrastructure ogp_adapter）
- Frontend Complete view に polling 追加（小規模）
- **DB migration 不要**

### Safari / モバイル

`safari-verification.md` の「OGP / Twitter card / 構造化データ」変更該当 → STOP δ で実機確認必須。
共有ボタン timing 変更は iPhone Safari 動作確認対象。

### 既存 deploy 整合性

- M-1a deploy 完了済（HEAD `f4e7ba4`）
- M-1b（destructive actions）とは独立（`get_public_ogp.go` の `publicAllowed` 判定に
  `hidden_by_owner` / `status='deleted'` を bool で追加するだけで吸収可能、本 ADR の同期 path
  はこの判定を再利用する）

---

## 運用 KPI（3 か月後再評価）

| 指標 | 目標値 | 計測 |
|---|---|---|
| 同期 OGP 成功率（publish 全件中、同期成功で `generated` になった割合） | ≥ 95% | slog `event=ogp_sync_result` の outcome=success / total |
| 同期 OGP latency p50 | < 300 ms | slog `duration_ms` の median |
| 同期 OGP latency p95 | < 1000 ms | slog `duration_ms` の 95 percentile |
| Scheduler 経由の generated 化（同期失敗時）SLA | ≤ 60 s | outbox `processed_at - created_at` 差分 |
| 1st crawl で `generated` 返却割合 | ≥ 99%（hard requirement の運用指標） | Cloud Run logs `/api/public/photobooks/{id}/ogp` 200 + status=generated の割合 |

KPI 未達時は以下を再評価:
- 同期 timeout 短縮（2.5 s → 1.5 s 等、失敗を早く outbox に倒す）
- Cloud Run CPU / memory 増（render / R2 PUT 余裕度向上）
- pre-publish 生成（候補 E）への移行検討

---

## 関連

- 実装計画: [`docs/plan/m2-ogp-sync-publish-plan.md`](../plan/m2-ogp-sync-publish-plan.md)
- 関連 ADR: ADR-0001（tech stack / SLO）/ ADR-0002（ops execution model）/ ADR-0005（image upload flow）
- 関連 docs: `docs/design/cross-cutting/outbox.md` §2 / §6.2 / `docs/design/cross-cutting/ogp-generation.md` §5.1
- 業務知識: v4 §3.2 / §3.8 / §6.17
- 既存事象: 2026-05-10 publish 直後 X teal placeholder 観測（failure-log 起票は STOP β で実装と
  併せて検討）

---

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-11 | 初版作成（Proposed）。STOP α plan と同時起票、実装着手前承認待ち |
