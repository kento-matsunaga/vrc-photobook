# m2-prepare-resilience-and-throughput PR 計画書

> 作成: 2026-05-02
> 状態: **STOP α（設計判断資料）**。STOP β 実装承認待ちで停止
> 起点:
>   - 2026-05-02 STOP ε 実機 smoke 中に 2 件の致命的 UX 問題を観察
>     1. `/prepare` で 15 枚 upload → 全 tile が「処理中」のまま 5〜15 分待機
>     2. ユーザが reload すると client-side queue が完全消失（server には image 残存しているが UI 復元不能）
>   - 観察根拠: `https://app.vrc-photobook.com/prepare/<photobookId-redacted>` での 15 件 upload + Cloud Run logs で確認
>     - upload-verifications **2 セット**発行（reload による再取得の証跡）
>     - complete 50 件超（gcloud limit、reload 後の再 upload 含む）
>     - image-processor 当時 5 min Scheduler で 13:09 complete → 13:10/13:15/13:20 の 3 tick で 25 件処理（11 分）
> 関連 docs:
>   - [`docs/plan/m2-upload-staging-plan.md`](./m2-upload-staging-plan.md) — Upload Staging 設計の正典、本書はその resilience / throughput 改修
>   - [`docs/plan/m2-image-processor-plan.md`](./m2-image-processor-plan.md) §15 / §17 Q9 — claim 並列性が「PR23 では考慮外」と明示されている根拠
>   - [`docs/plan/m2-image-processor-job-automation-plan.md`](./m2-image-processor-job-automation-plan.md) §9 / §11.3 — Scheduler 5 min を「許容範囲」とした判断、本書で修正
>   - [`docs/plan/m2-frontend-edit-ui-fullspec-plan.md`](./m2-frontend-edit-ui-fullspec-plan.md) — edit-view 既存仕様
> 関連 ADR:
>   - [`docs/adr/0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md) — draft session cookie
>   - [`docs/adr/0005-image-upload-flow.md`](../adr/0005-image-upload-flow.md) — image status 6 値
> 関連 rules:
>   - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) — raw ID / Cookie / Secret 不記録
>   - [`.agents/rules/feedback-loop.md`](../../.agents/rules/feedback-loop.md) — 失敗 → 必須起票
>   - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
>   - [`.agents/rules/pr-closeout.md`](../../.agents/rules/pr-closeout.md)

---

## 0. 本計画書の使い方

- §1〜§3 で **目的 / 原因 / P0 実装方針**を確定
- §4 で **UX 表示仕様**（progress count / reload 復元 / 待機メッセージ / enable 条件）を確定
- §5 で **Bulk attach API** 設計、§6 で **Issue B（Turnstile UX）** の後段位置づけ
- §7 で **STOP 設計**、§8 で **failure-log 起票**、§9 で制約遵守、§10 で履歴
- 実装は STOP β 承認後に着手。**本書段階ではコード変更なし**

---

## 1. 目的

1. **`/prepare` を reload しても進捗が消えない**よう server ground truth に基づく復元を実装する
2. **画像処理が「終わらない」体感を排除**し、通常 1〜2 分で全 available 化が観測できる Scheduler / 起動戦略を確立する
3. ユーザに **「同期的に進んでいる」進捗フィードバック**（n/m count + 段階表示）を提示する
4. 「編集へ進む」時に **available image を photobook の page に bulk attach** し、`/edit` 表示で見えるようにする
5. `/prepare` の **Bot 認証 2 度要求 UX** を draft session 認可 + UsageLimit で代替（後段、§6）

非ゴール（roadmap で別判断）:
- 同期 image-processor 実装（plan §3.2 m2-image-processor-job-automation で却下、引き続き非対象）
- HEIC / RAW 対応
- 並列 worker 化の本格実装（overlap guard 後の P1 候補、本書では設計のみ）
- preview 即時表示（client-side `URL.createObjectURL`、P1 候補）

---

## 2. 原因整理（事実ベース、コード根拠付き）

### 2.1 client polling が Cookie を送らず 401 になり得る（要確認）

`frontend/lib/editPhotobook.ts:fetchEditView` は SSR 経路で `cookieHeader` を引数にとる仕様。一方 client-side polling から呼ぶ際の credentials 設定が `fetch` の default のまま。**Server Component 用 (Cookie 手動転送)** と **Client Component 用 (`credentials: "include"`)** が分離されておらず、client polling で 401 が出る可能性がある。

*影響*: `/prepare` の polling が 401 → reconcile 不発 → UI 上「processing のまま」。

### 2.2 queue が client React state のみで reload 復元不能

`frontend/app/(draft)/prepare/[photobookId]/PrepareClient.tsx`:
- `const [queue, setQueue] = useState<QueueState>(() => emptyQueue())` (line 113)
- `verificationCacheRef` も `useRef` で初期化（reload で消失）
- SSR 側で `initialView: EditView` を受けるが、`view.processingCount`（カウント）と `view.pages.*.photos`（**available + page 配置済のみ**）しか含まれない
- → reload 後 UI 上 `summary` が「合計 0 / 完了 0 / 処理中 0 / 失敗 0」となり、ユーザは「全部消えた」と判定
- 実際は server に image record + R2 object が残っている（永続化済）

### 2.3 edit-view が processing / failed / unplaced available image list を返していない

`backend/internal/photobook/internal/usecase/edit_view.go`（および lib/editPhotobook.ts の型 `EditView`）:
- `pages: EditPage[]` → 各 page の photos のみ列挙
- `processingCount: number` / `failedCount: number` → 集計値のみ、imageId 不可視
- **`unplacedImages: []` のような全 image 一覧 field が無い**
- → server に image はあるが Frontend から「どの image がどの状態か」が見えない

### 2.4 attach 経路の確定結果（既存設計の重大欠陥）

**結論**: 既存 production には **upload した image を `photobook_photos` に attach する HTTP 経路が存在しない**。

事実関係:
- `backend/internal/imageprocessor/internal/usecase/process_image.go:111-347`: variant 生成 + `MarkAvailable` + `AttachVariant` のみ。`photobook_photos` には触れない
- `backend/internal/photobook/internal/usecase/add_photo.go:37-90`: `AddPhoto` UseCase は **存在する**（page_id + image_id を受け、20 上限 + OCC + image owner/status 検証 + 同一 TX 内で `photobook_photos` INSERT）
- **`AddPhoto` の HTTP handler が無い**: `grep -rn 'AddPhoto' backend/internal/photobook/interface/http/ backend/internal/http/ backend/cmd/api/ --include='*.go'` のヒット 0 件
- `frontend/lib/editPhotobook.ts` に `addPhoto` 系 export 無し（`addPage` のみ）

帰結:
- 既存 EditClient で upload しても、image-processor が available 化した後に **photobook_photos には何も入らない**
- `/edit` の `view.pages.*.photos` には photo が並ばない
- 既存 production の `/edit` 利用者も実は同じ問題を踏んでいた可能性が高いが、create-entry 完成前に publish まで到達したユーザがいなかったため発覚していなかった
- /prepare で 15 枚 upload → 全 available 化 → `/edit` で見えない、という今回の症状はこの欠陥の **初観測**

→ **§3.4 で bulk attach API は "必須"**（オプションではない）。これがないと /edit に画像が出ず、publish 経路も成立しない。

### 2.5 Scheduler 5 min が VRChat 撮影セッション UX に合わない

`m2-image-processor-job-automation-plan.md` §9.1 で「max 5 分は許容」と判定したが:
- VRChat 撮影は 1 セッション 10〜30 枚が典型
- max-images 10 + 5 min interval で **30 枚 = 15 分待ち**
- 体感で「終わらない」と認識される

**2026-05-02 13:19 UTC 時点で Scheduler を `* * * * *`（1 min）に更新済**（user 承認 STOP immediate）。

### 2.6 max-images 増加には overlap guard が必要

`backend/internal/imageprocessor/internal/usecase/process_pending.go` 冒頭 comment 抜粋:

> "race の可能性: claim TX 内で他 worker が同じ row に到達することはない（FOR UPDATE SKIP LOCKED）。一方 claim TX commit 後に同じ row を別 worker が再度取り出す可能性はあるが、PR23 では single-worker 前提のため考慮外。"

`backend/internal/image/infrastructure/repository/rdb/queries/image.sql:49-68` の claim クエリは `WHERE status='processing' FOR UPDATE SKIP LOCKED` で 1 件取り、**短い TX で commit して lock を解放**してから R2 GetObject / decode / encode / PutObject の重い処理に入る。

危険シナリオ:
- Scheduler 1 min interval
- max-images 30 で 1 起動の処理時間が 60〜180s（重い処理 + R2 I/O）
- 次 tick で **同じ row が再 claim される**（status は依然 processing、lock 解放済）
- → 二重処理 / R2 PUT 重複 / OCC 衝突

→ **max-images 増加 / parallelism 増加の前に overlap guard が必須**。本書 §3.5.3 で 3 案を提示。

---

## 3. P0 実装方針

### 3.1 P0-a: client polling 401 修正

- `frontend/lib/editPhotobook.ts` を **SSR 用 / Client 用に経路分離**:
  - SSR: 既存 `fetchEditView(photobookId, cookieHeader)` を維持
  - Client: 新規 `fetchEditViewClient(photobookId)` を追加し、`fetch(url, { credentials: "include" })` で Cookie を自動送信
- `PrepareClient.tsx` の polling は Client 経路で呼ぶ
- 401 / 404 は既存 `EditApiError` で type-safe ハンドリング

### 3.2 P0-b: prepare-view / edit-view 拡張（**Backend 変更必須**）

採用案 **A-3**: edit-view を拡張して全 image list を含める。新 endpoint は作らない。

response 型拡張:
```ts
type EditView = {
  ...
  pages: EditPage[];
  processingCount: number;
  failedCount: number;
  // 新規追加
  images: Array<{
    imageId: string;
    status: "uploading" | "processing" | "available" | "failed";
    byteSize: number;
    sourceFormat: "jpg" | "png" | "webp";
    failureReason?: string; // failed の時のみ、user-friendly redact 後
    createdAt: string;       // ISO
  }>;
};
```

セキュリティ:
- **`storage_key` / R2 endpoint URL / upload URL を返さない**
- `imageId` は UUID、UI 表示は先頭 8 桁 + size + status のみ（**raw imageId は data-attribute / DOM に出さない**）
- failure_reason は domain 12 種を user-friendly な 3〜5 種にマッピング

### 3.3 P0-c: reload 復元（A-3 server ground truth + A-1 localStorage 補助）

採用方針: **server を ground truth とし、localStorage は file name 等の表示補助のみ**。

PrepareClient initialization:
1. SSR で edit-view（`images` 拡張版）取得
2. Client mount 時に `view.images` を queue tile として **復元**:
   - 各 image を `QueueTile` に変換: `{ id: imageId, file: null, status: imageStatusToTileStatus(image.status), imageId, byteSize, displayLabel }`
   - 注: `file: null` （reload では File オブジェクト不在）
   - 表示は `byteSize` + `displayLabel`（後述）
3. localStorage から file name を best-effort 復元:
   - upload 開始時に `localStorage.setItem(\`prepare:\${photobookId}:\${imageId}\`, JSON.stringify({ name, savedAt }))`
   - reload 時 `imageId` 一致で name を引く（同端末同ブラウザの再来訪のみ）
   - 別端末 / Private モード / clearStorage 後は name 不在 → `displayLabel` を `Image #N (size, status)` にフォールバック
   - localStorage に raw token / Cookie / draft_edit_token 等は保存しない

### 3.4 P0-d: 「編集へ進む」時 bulk attach（**必須、Backend 変更必須**）

§2.4 の確認結果より、**`photobook_photos` に attach する HTTP 経路が現行 production に存在しない**。本 PR で必ず公開する。これがないと:
- `/edit` で image が見えない
- publish 経路（page.photos が 1 枚以上必要）も成立しない
- 既存の /edit single upload 経路も実は壊れている

採用案: **`POST /api/photobooks/{id}/prepare/attach-images`** 新設（既存 `AddPhoto` UseCase を内部で呼んで bulk 化）。

仕様（詳細は §5）:
- 認可: draft session cookie
- 処理:
  - photobook の `status=available && unplaced` image をすべて draft photobook の photo として attach
  - 既存 page が無ければ作成、20 枚で page 分割（業務知識 v4 §6.x の page 20 枚上限と整合、§5.4 の P-1 案）
  - 同一 TX で `photobook_photos` INSERT + photobook OCC version+1
  - `image.became_available` の outbox event とは独立（image-processor の責務分離は維持）
- response: `{ attached_count, page_count, skipped_count }` のみ、raw ID は返さない
- 失敗時: 部分成功は許容しない（rollback）

**副次効果**: 既存 `/edit` の single upload も attach 経路が必要なので、同 endpoint を `/edit` からも呼べるようにする（または別途 single attach handler を公開するかは §5.x で判断、最小スコープなら attach-images 単一で兼用）。

### 3.5 P0-e: throughput（user feedback に基づき immediate trigger を P0 候補化）

#### 3.5.1 Scheduler 1 min（実施済 STOP immediate）

- 2026-05-02T13:19:59 UTC に `vrcpb-image-processor-tick` を `*/5` → `* * * * *` に更新済
- 直後の natural tick (13:20:26) で picked=5 / success=5、続く 13:21:17 で picked=0 / success=0 で drain 完了
- 前 photobook の 25 件 success（11 分）→ 1 min 化後に同様の量を **drain 約 3〜4 分**で吸収可能と推定
- ただし「待ち時間ゼロ」ではない。次 §3.5.4 immediate trigger と組合せて初めて **「即座に処理が始まる」体感**が成立する

#### 3.5.2 overlap guard（max-images / parallelism 増加 / immediate trigger 採用の前提）

要 user 判断、STOP β で 1 案を採用:

| 案 | 内容 | migration | 複雑度 | リスク |
|---|---|---|---|---|
| **G-1** | **PostgreSQL advisory lock**: claim TX 後も `pg_try_advisory_lock(image_id_hash)` を保持し、processing 完了で `pg_advisory_unlock`。重い処理中は lock 維持 | 不要 | 中 | crash で lock leak（pgxpool セッション切れで自動解放されるが要確認） |
| **G-2** | **`claimed_at timestamptz` column 追加**: claim 時に `claimed_at = now()`、再 claim 条件を `status='processing' AND (claimed_at IS NULL OR claimed_at < now() - interval '5 min')` に | 必要 | 中 | migration、stale claim の救済間隔チューニング |
| **G-3** | **`status='claimed'` 中間状態追加**: `processing → claimed → available/failed`、claim TX で status='claimed' に遷移、重い処理中は claimed | 必要 | 大 | image_status enum 拡張、既存 query 全変更、domain VO 変更 |

**推奨: G-1（advisory lock）**
- migration 不要、最小 invasive
- crash 時の lock leak は pgx の session-scoped lock が接続切断で解放されるため実害低
- §3.5.3 / §3.5.4 の前提条件（overlap guard 不在では throughput を上げられない）

#### 3.5.3 max-images 増加（条件付き、明確な閾値で判断）

「いつ何を満たしたら上げるか」を **基準 (acceptance criteria) ベース**で確定:

| 段階 | max-images | 適用条件 | 期待効果 |
|---|---|---|---|
| **現状** | 10 | Scheduler 1 min + max-images 10（暫定）| 30 枚で max 3〜4 min |
| **昇格候補 1** | **30** | G-1 advisory lock 実装 + STOP ε で 30 枚 smoke が **3 分以内**に収まらない場合 | 30 枚で max 1〜2 min |
| **昇格候補 2** | **50** | 上記 30 でも収まらず、かつ image-processor の memory / CPU / Cloud SQL connection に余裕がある場合 | 50 枚で max 2 min 想定 |

**Scheduler 1 min + max-images 10 は暫定改善であり最終解ではない**ことを本 PR で明記。STOP ε smoke の実測値を根拠に閾値判定。

#### 3.5.4 immediate trigger（**user feedback 受領で P0 候補に昇格**）

「Scheduler 1 min ですらまだ "待ち" であり、user 要求は "できればすぐ処理してほしい"」とのフィードバックを受け、**P0 設計比較**に昇格させる。

##### 設計案 IT-1: complete handler から Cloud Run Job を即時 invoke

```
complete handler (HTTP 200 返した後の background job として):
  - photobook_id ごとに「直近の complete 後 N 秒間 trigger を抑制」debounce
  - Cloud Run Admin API "POST /apis/run.googleapis.com/v1/.../jobs/vrcpb-image-processor:run"
  - OAuth token（既存 Cloud Scheduler と同 SA、roles/run.invoker は付与済）
```

##### 設計案 IT-2: complete handler が outbox に "process_now" event を入れる

```
complete handler 同 TX で outbox INSERT (event_type=image.process_requested)
outbox-worker が拾って Cloud Run Job を invoke
```
→ 既存 outbox-worker は手動実行 / Scheduler 化していないため、間接化メリット薄い。**却下**。

##### 設計案 IT-3: complete handler が image-processor を直接呼ぶ（同期処理）

→ `m2-image-processor-job-automation-plan.md` §3.2 で既に却下（Cloud Run timeout / メモリ / retry）。**却下**。

##### 設計案 IT-4: Frontend が attach 直前に Job を invoke（`/prepare/attach-images` handler 内で trigger）

```
attach-images handler:
  - photobook_photos INSERT が成功したら、追加で Cloud Run Job invoke
  - ただし attach 時には image は既に available（attach 不可なら failed）なので、ここで trigger する意味が薄い
```
→ trigger するなら「complete 直後」が正しい。**却下**。

##### IT-1 採用時の詳細仕様

- **debounce**: photobook_id を key とした in-memory map で「直近 30 秒以内に trigger 済 → skip」（多重起動を抑止）
- **idempotency**: Job 側で advisory lock (G-1) があれば多重起動しても安全
- **fallback**: trigger 失敗（Cloud Run Admin API 障害 / IAM 失効）でも Scheduler 1 min が backup
- **権限**: 既存 compute SA に `roles/run.invoker` は Cloud Run Job レベルで付与済（STOP γ で適用）。本 PR で追加付与不要
- **コスト**: Job 起動回数増（1 photobook 1 起動 + Scheduler 1 起動 / min）、月 < $5 想定

##### 採用判定の鍵

- **G-1 advisory lock が前提**（IT-1 では複数 trigger / Scheduler tick が同時に走る可能性が現実化、G-1 がないと二重処理リスク）
- 実装 cost 中（complete handler に async trigger 追加 + debounce）

→ **G-1 + IT-1 の組合せを P0 として採用候補に**。STOP β-2 で実装、STOP ε smoke で acceptance criteria 達成判定。

### 3.6 acceptance criteria（**user feedback 反映、STOP ε 合格条件**）

STOP ε（Chrome/Edge smoke）で以下を **すべて満たす**ことを合格条件とする:

| # | criteria | 期待値 |
|---|---|---|
| AC-1 | **10 枚 upload**: complete から全 available 化までの実測時間（最後の 1 枚） | **通常 1〜2 分以内** |
| AC-2 | **15 枚 upload**: 同上 | **通常 2 分以内** |
| AC-3 | **30 枚 upload**: 同上 | **3 分以内**（max-images 10 のままで G-1 + IT-1 採用なら達成想定。未達なら max-images 30 に昇格） |
| AC-4 | **10 分以上経過**しても全 available にならない場合 | UI が「処理が遅れています。再読み込みしても進捗は保持されます」を **表示**する |
| AC-5 | **processing 中に reload** | progress count（n/m、6 段階）が **同じ値で復元**される。tile が消失しない |
| AC-6 | **「編集へ進む」押下後**: bulk attach 完了 → /edit redirect | `/edit` の grid に 全 image が photo として並ぶ |
| AC-7 | smoke 中に発生する R2 / Cloud SQL / Cloud Run のエラー | **0 件**（ログで確認） |
| AC-8 | smoke 期間中に raw `imageId` / `storage_key` / Cookie / Secret | DOM / Console / Network response に **露出 0 件** |

達成判定:
- AC-3 が 3 分超過 → **max-images 30 に昇格して再 smoke**（STOP ε 内で 1 回まで）
- AC-3 が再 smoke でも未達 → max-images 50 / immediate trigger debounce 値短縮 を別 PR で検討
- AC-1〜AC-2 が達成できない場合は IT-1 設計の見直し（debounce / fallback）を STOP β-2 に差し戻し

---

## 4. UX 表示仕様（user 追加要件、本 PR の核）

### 4.1 progress count（n/m）の表示種別 6 種

`/prepare` summary セクションを以下の 6 段階 count に置換:

```
合計 m 枚
├ アップロード中   n_uploading / m
├ アップロード完了 n_uploaded / m   ← complete 後・processing 前の中間状態は表示しない（即 processing に遷移）
├ 変換待ち         n_processing_queued / m  ← server processing 状態のうち、Job がまだ claim していないもの
├ 変換中           n_processing_active / m  ← server processing 状態のうち、Job が claim 中（advisory lock 保持中）
├ 変換完了         n_available / m
└ 失敗             n_failed / m
```

実装:
- 「アップロード中 / 変換待ち / 変換中 / 失敗 / 変換完了」を queue + view から計算
- `processing_queued` vs `processing_active` の区別: G-1 advisory lock 採用時は server side で「lock holder の有無」を edit-view に含める（要 backend）
  - 簡素化案: 区別せず「変換中: n / m」1 行に統合（MVP）
- 文言は user-friendly 日本語、半角スペースで count を強調

### 4.2 reload 後の進捗復元

- SSR の `view.images` から queue 復元（§3.3）
- localStorage から file name best-effort 復元
- 復元できない場合の表示: `Image #1 (1.6 MB, 変換中)` のような形（raw imageId は出さない）

### 4.3 待機メッセージ

- **置換**: 「画像処理は最大 5 分ほどかかります」 → 「**通常 1〜2 分で完了します。画面はそのままで OK です。**」（Scheduler 1 min 化後の現実値）
- 10 分以上進まない場合（client side で経過時間を計測）: 「**処理が遅れています。再読み込みしても進捗は保持されます。**」を別文言として赤系 banner で表示
- ユーザに reload を「危険」と感じさせない文言

### 4.4 「編集へ進む」enable 条件

- 全 queue tile が `available` か `failed`
- かつ 1 枚以上の available
- かつ サーバ側 unplaced available image（attach 候補）が存在する場合は attach 可能
- 押下時に **bulk attach API** 呼び出し（§5）→ 成功で `/edit/<photobookId>` redirect

旧条件（plan §6.7）の `serverProcessingCount === 0` 単独判定は撤回。

### 4.5 raw ID を UI に出さない方針（再確認）

- DOM / data-attribute / aria-label / console / response body に raw `imageId` / `photobook_id` / `storage_key` を出さない
- tile id は client UUID（imageId とは別）
- データ要素は `imageId` を React key に使うが、**DOM に直接書き出さない**（先頭 8 桁などへ縮約は OK）

---

## 5. Bulk attach API 設計案

### 5.1 endpoint

`POST /api/photobooks/{id}/prepare/attach-images`

または既存 `/edit/` 配下に置く案も検討:
- 候補1: `POST /api/photobooks/{id}/prepare/attach-images`（**第一候補**、UX 経路と一致）
- 候補2: `POST /api/photobooks/{id}/images/attach-all`（image 集約配下）

### 5.2 認可

- draft session cookie（`/edit` と同じ）
- manage URL token は対象外（draft 段階のみの操作）

### 5.3 処理仕様

```sql
-- 概念的な処理フロー（実装は usecase 層）
BEGIN;
SELECT * FROM photobooks WHERE id = $1 AND status='draft' AND draft_expires_at > now() FOR UPDATE;
-- OCC: version+1
UPDATE photobooks SET version = version + 1, updated_at = $now WHERE id = $1 AND version = $expected_version;
-- 配置候補の取得
SELECT id, byte_size, source_format
FROM images
WHERE owner_photobook_id = $1
  AND status = 'available'
  AND id NOT IN (SELECT image_id FROM photobook_photos WHERE photobook_id = $1);
-- page 計算（既存 page の photos.count を見て 20 で分割）
-- 不足 page を CREATE
-- photobook_photos INSERT（page_id + display_order 計算）
COMMIT;
```

### 5.4 page 分割ルール（要 user 判断、STOP β で確定）

| 案 | 内容 |
|---|---|
| **P-1** | MVP: **新規 page を必要数だけ作成**し、20 枚ごとに分割（既存 page には touch しない） |
| P-2 | 既存 page の末尾に追加し、20 枚超過分は新 page を作成 |
| P-3 | UI で page 配置の hint を取り、Backend は受動的に attach（複雑） |

**推奨: P-1**（MVP、UI/UX 単純）。/edit で page reorder / move が既存 UI で対応済なので、後でユーザが整理できる。

### 5.5 response

```json
{
  "attached_count": 15,
  "page_count": 1,
  "skipped_count": 0
}
```

- raw `imageId` / `photoId` / `pageId` は返さない
- skipped_count は「server に available があるが対象外（既に配置済 等）」の件数

### 5.6 失敗時

- OCC 衝突: HTTP 409 + `{"status":"version_conflict"}`
- 認可失敗: HTTP 401 + `{"status":"unauthorized"}`
- partial 成功は許容しない（TX rollback、全 or 無）
- 20 枚 cap 超過は本 API では起こらない（image 上限 20 枚は upload-verifications 側で別途 enforce）

### 5.7 image-processor outbox event との関係

- image-processor は引き続き `image.became_available` を outbox に INSERT（変更なし）
- bulk attach は **手動 trigger（ユーザの「編集へ進む」操作）**で実行
- 自動 attach は本 PR では行わない（PR23 plan の「image-processor は variant 生成のみ」責務維持）

---

## 6. Issue B（**設計のみ本 plan に記載、実装は別 STOP / 別 PR で分離**）

### 6.1 user feedback による分離方針

> "Issue B を同 PR に含めるのは少し危険。Bot 認証削除は重要ですが、P0 が多すぎます。いま同時にやると、reload / attach / throughput の検証が濁ります。私なら Issue B は同 plan に載せるが、実装は別 STOP / 別 PR に分けます。"

→ **本 plan には設計（§6.2〜§6.4）を記載するが、実装・deploy・smoke は本 PR の STOP β/γ/δ/ε から除外**。`m2-prepare-upload-turnstile-ux-relax` を別 PR として、本 PR closeout 後に独立計画化する。

### 6.2 課題

`/create` で Bot 検証済みなのに `/prepare` で Turnstile widget が再表示される UX。ユーザは「2 度認証要求」を冗長と感じる。

### 6.3 案比較

| 案 | Frontend | Backend | UX | 実装 cost |
|---|---|---|---|---|
| **B-1** | `/prepare` から Turnstile widget を削除 | `POST /api/photobooks/{id}/upload-verifications/` の Turnstile 必須を draft session 認可に置き換え。`UsageLimit (upload_verification.issue)` で spam 抑止 | 良（Bot 検証 1 回） | 中 |
| B-2 | invisible Turnstile（widget 非表示で内部 siteverify） | 不変 | 中 | 小 |
| B-3 | 現状維持 | — | 後退 | 0 |

**別 PR での推奨: B-1**

### 6.4 セキュリティレビュー（別 PR で実施）

B-1 採用時の attack surface 検討:
- draft session cookie を持つ攻撃者 = `/create` を突破済 = Bot 検証 1 回通過済
- → upload-verifications/ に Turnstile を強制する追加防御は冗長
- spam 抑止は UsageLimit `upload_verification.issue`（既存）で十分（rate limit + window）
- 既存 PR36 で UsageLimit は実装済、別 PR で再 review

### 6.5 実装着手の前提条件（本 PR 完了が条件）

- 本 PR `m2-prepare-resilience-and-throughput` の STOP ε / final closeout 完了
- 別 PR `m2-prepare-upload-turnstile-ux-relax` の STOP α 計画書を新規作成
- → 本 plan §7 STOP 設計から β-3/β-4 を除外（次節で更新）

---

## 7. Deploy / STOP 設計（**Issue B 分離後の最終形**）

| STOP | 内容 | 課金 | コード変更 | 承認 |
|---|---|---|---|---|
| **α**（**本 commit / v2 で再提示**） | 計画書 push（コード変更なし） | なし | あり（plan のみ） | 完了 |
| **immediate**（**実施済**） | Scheduler 5min → 1min（gcloud 1 行）+ work-log 記録 | 軽 | なし | 完了 |
| **β-1** | Frontend: P0-a（client fetch credentials 経路分離）+ P0-c（reload 復元の client 側、SSR images から queue 復元 + localStorage 補助）+ §4 UX 表示仕様（n/m 6 段階 / 待機メッセージ / enable 条件）+ tests | なし | 中 | 別途 |
| **β-2** | Backend: P0-b（edit-view を `images: [{imageId, status, byteSize, sourceFormat, ...}]` で拡張）+ P0-d（bulk attach handler `POST /api/photobooks/{id}/prepare/attach-images`）+ G-1 advisory lock（image-processor claim）+ IT-1（complete handler から Cloud Run Job 即時 invoke + debounce）+ tests | なし | 大 | 別途 |
| ~~β-3~~ | ~~B-1 Turnstile 緩和~~ → **別 PR `m2-prepare-upload-turnstile-ux-relax` に分離**（user feedback 反映） | — | — | 別 PR |
| ~~β-4~~ | ~~B-1 Turnstile widget 削除~~ → **同上、別 PR に分離** | — | — | 別 PR |
| **γ** | Backend deploy（β-2 反映、Cloud Build manual submit） | 軽 | なし | **要承認** |
| **δ** | Workers redeploy（β-1 反映、subshell wrangler deploy） | 軽 | なし | **要承認** |
| **ε** | Chrome/Edge smoke: §3.6 acceptance criteria AC-1〜AC-8 全達成判定。10/15/30 枚 upload で時間計測、reload 中の復元検証、「編集へ進む」→ `/edit` で全 image 表示 | 軽（draft 1 件残置） | なし | **要承認** |
| **ζ** | Safari / iPhone Safari smoke（同 acceptance criteria + Safari 特有観点） | 軽 | なし | **要承認** |
| **final** | work-log / roadmap / runbook / failure-log §8 起票 / docs 更新（CLAUDE.md / `m2-image-processor-job-automation-plan.md` の Scheduler 設定値 / `m2-upload-staging-plan.md` の reload 復元欠落記述） | なし | あり（docs のみ） | 完了報告 |

### 7.1 STOP β の細分理由（v2、β-3/β-4 削除後）

P0 を 2 つに分けたのは:
- β-1 は **Frontend のみ**（Workers redeploy で完結）
- β-2 は **Backend のみ**（Cloud Build deploy）
- 互い独立に implement / test 可能で、レビュー単位を小さく保てる
- **deploy 順は β-2（Backend 先行）→ β-1（Frontend）が安全**（Backend 先行 = Frontend が古い API 期待でも fallback 可、逆順だと Frontend が新 API を呼んで 404）

### 7.2 Issue B 別 PR の起票タイミング

本 PR `m2-prepare-resilience-and-throughput` の **final closeout 完了後**に、新 PR `m2-prepare-upload-turnstile-ux-relax` の STOP α 計画書を独立で起票する。本 PR では plan §6 に設計を残すのみで、実装・deploy・smoke は別 PR に集約。

---

## 8. failure-log 起票（必須、本 PR の closeout 時に記録）

`harness/failure-log/2026-05-02_prepare-resilience-and-throughput-failures.md` を新規起票。記録する事象:

| # | 事象 | 根本原因 | 再発防止 |
|---|---|---|---|
| 1 | reload で client queue が消滅し UX 上「全部消えた」 | client React state のみで queue 保持、server ground truth が無い | rule: 「client-side state を持つ UI は必ず reload 復元経路を design 段階で含める」 |
| 2 | `/prepare` polling が credentials 不足で 401 を返す可能性 | Server Component と Client Component で Cookie 経路を分離していない | rule: 「lib API は SSR / Client 用を経路分離して export し、混在時は型レベルで防ぐ」 |
| 3 | edit-view が processing / failed image を imageId 単位で返さず集計のみ | アグリゲートのみ返す API 設計 | rule: 「polling 対象の view API は ground truth list を返す」 |
| 4 | image-processor が available 化のみで photo attach しない設計のまま `/prepare` を作った | PR22→PR27 の責務移行で attach がどこにも残らなかった | rule: 「集約子テーブル更新の責務をどの UseCase が持つか PR レビューで明示」 |
| 5 | Scheduler 5 min を「許容範囲」と判断したサイジング誤り | VRChat 撮影セッション 10〜30 枚という現実 traffic を計測せず | rule: 「外部 trigger の interval は実 traffic 量と batch サイズで sizing する。`max_items / interval = throughput` を計算せず採用しない」 |
| 6 | concurrency=2 並列 upload で issueUploadVerification race condition | `selectNextRunnable` の concurrency unit test が pure logic のみで async fetch race を verify していなかった | rule: 「concurrency を持つ async flow は test で並列 race を必ず再現する」 |
| 7 | overlap guard なしで Scheduler 1 min + max-images 増加を提案しかけた | claim TX 後の lock 解放期間に再 claim される設計を読まずに提案 | rule: 「外部 trigger の interval / batch を変更する前に claim 設計（lock / advisory / status）を読み直す」 |

各事象を `.agents/rules/*.md` 化するかは closeout 時に判断（重複 rule は集約）。

---

## 9. 制約遵守

- 本書に raw `photobook_id` / `image_id` / `slug` / token / Cookie / `storage_key` / upload URL / R2 endpoint URL の実値 / `DATABASE_URL` / Secret 値 / Bearer / sk_live / sk_test を記載していない
- 観測根拠の screenshot / production URL は本書に記載しない（user 共有値も内部解析のみ）
- 本書作成時点で実 GCP 操作は **Scheduler 1 min 変更のみ**（user 承認 STOP immediate）
- production DB cleanup / Job execute / Scheduler 追加 / Job spec / image / env / Secret / cloudsql-instances / parallelism / max-images の変更は **本 PR では実施しない**（STOP γ 以降の承認後に限定）
- `.claude/scheduled_tasks.lock` / `TESTImage/` / `ChaeckImage/` は commit しない

---

## 10. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-02 | 初版作成。STOP ε 中に発覚した reload-loss + throughput 不足 + UX 進捗フィードバック不足を統合 PR として独立計画化。Scheduler 1 min 化を STOP immediate として先行実施 |
| 2026-05-02 (v2) | user feedback (75 点評価) に基づき改訂: (1) §2.4 を確認結果として確定（既存に attach 経路 HTTP handler 無し）、(2) §3.4 で bulk attach API を必須と明記、(3) §3.5.4 immediate trigger を P0 候補に昇格 + 設計案 IT-1〜IT-4 の比較追加、(4) §3.5.3 max-images 増加条件を criteria 化、(5) §3.6 acceptance criteria AC-1〜AC-8 を新規追加（10/15/30 枚の時間目標 + reload 復元 + 遅延通知 + Secret 非露出）、(6) §6 Issue B を「設計のみ記載、実装は別 PR」に分離、§7 から β-3/β-4 を削除、(7) §7.2 Issue B 別 PR 起票タイミングを明記 |
