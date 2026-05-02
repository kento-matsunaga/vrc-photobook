# M2 Upload Staging 導線 PR 計画書（m2-upload-staging）

> 作成: 2026-05-01
> 状態: **STOP α（設計判断資料）** ユーザ承認待ち。STOP β 以降の実装は本書承認後に着手
> 起点: m2-image-processor-job-automation 完了後の運用観測で、`/edit/<photobookId>` が「upload / processing 待ち / 編集」を 1 画面に同居させているため、image-processor の処理遅延（5 min Scheduler tick + Job 実行時間）がそのまま編集体験を壊す構造的問題が顕在化。複数画像の一括投入も `<input type="file">` 単枚のみで未対応
>
> ⚠ **2026-05-03 更新**: 本書内の「Scheduler 5 min」「最大 5 分ほどお待ちください」の前提は
> [`m2-prepare-resilience-and-throughput-plan.md`](./m2-prepare-resilience-and-throughput-plan.md) §2.5 / §3.5 / §6 で **1 min interval + 通常 1〜2 分案内** に supersede 済。
> 実 Scheduler `vrcpb-image-processor-tick` は 2026-05-02T13:19:59 UTC に
> `*/5 * * * *` → `* * * * *` (1 min) に更新済、Frontend 側の「最大 5 分」表示も
> β-3 (commit f455fe4) で「通常 1〜2 分ほどで完了します」+ 10 分超過遅延通知に更新済。
> 本書内の「5 min」記述は計画策定時の履歴情報として残すが、現在地は §3.5 plan v2 の値。
>
> 関連 docs:
> - [`docs/plan/m2-image-processor-job-automation-plan.md`](./m2-image-processor-job-automation-plan.md)（image-processor 自動化、Scheduler 5 min 設定）
> - [`docs/plan/m2-frontend-upload-ui-plan.md`](./m2-frontend-upload-ui-plan.md)（PR22 で 1 枚ずつ upload を「MVP 範囲外」と先送りした記録）
> - [`docs/plan/m2-image-upload-plan.md`](./m2-image-upload-plan.md)
> - [`docs/plan/m2-frontend-edit-ui-fullspec-plan.md`](./m2-frontend-edit-ui-fullspec-plan.md)
> - [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md)（closeout で更新）
>
> 関連 ADR:
> - [`docs/adr/0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md)（draft session cookie の scope 定義）
> - [`docs/adr/0005-image-upload-flow.md`](../adr/0005-image-upload-flow.md)（image upload 4 ステップ）
>
> 関連 rules:
> - [`.agents/rules/turnstile-defensive-guard.md`](../../.agents/rules/turnstile-defensive-guard.md)（Turnstile L0–L4）
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)（raw token / Cookie / Secret 不記録）
> - [`.agents/rules/pr-closeout.md`](../../.agents/rules/pr-closeout.md)

---

## 0. 本計画書の使い方

- §1〜§3 で **目的 / 現状 / 採用導線**を提示
- §4〜§6 で **route / 認可 / UI 仕様**を比較・確定
- §7〜§9 で **edit 画面側の変更 / Backend / image-processor との関係**
- §10 で **Safari smoke 観点**
- §11 で **P0 / P1 / P2 分割**
- §12 で **STOP 設計**
- §13 で **実装対象ファイル案**
- §14 で **リスク / 制約**
- §15 で **承認文案**

実装・gcloud 操作は **本書承認後の STOP β 以降で行う**。本書では資料化のみ。

---

## 1. 目的

1. **画像投稿と編集を分離**する。`/create` 完了直後にユーザが入る画面を、編集機能込みの `/edit` ではなく **画像投稿専用の Upload Staging 画面**に切替える
2. **複数画像の一括投入**をサポート（`<input type="file" multiple>` + 並列キュー処理）
3. **processing / available / failed の状態を画像ごとに可視化**し、ユーザが「何を待っているか」「失敗が出たか」を理解できるようにする
4. **全画像が available になってから「編集へ進む」**で `/edit/<photobookId>` に遷移させる。processing 中は編集画面に入らせない（image-processor 遅延が編集体験を直接壊さないようにする）
5. **image-processor 5 min Scheduler の遅延を UX 上で受容可能な範囲に整える**（待機画面で明示 + 進捗表示）

非ゴール（本 PR では扱わない）:
- image-processor Scheduler を 1 min 化、または Backend から Cloud Run Job を即時 invoke する経路（後述 §9.2、別 PR / P2 候補）
- preview 即時表示（`URL.createObjectURL` 経由）（P1）
- drag & drop（PC のみ。P1）
- HEIC 対応 / 大画像クライアントサイドリサイズ（既存 image-processor 仕様、別 PR）

---

## 2. 現状整理（事実ベース）

### 2.1 現行導線（problem statement）

```
LP (/)
 → /create
 → /draft/<token>      （raw token を session cookie に交換、即 redirect）
 → /edit/<photobookId> （ここで upload / processing 待ち / 編集 が同居）
```

### 2.2 `/edit/<photobookId>` が抱えるギャップ

| 工程 | ファイル:行 | 問題 |
|---|---|---|
| 画像 upload UI | `frontend/app/(draft)/edit/[photobookId]/EditClient.tsx:462-468` | `<input type="file">` に `multiple` 属性なし |
| File 受け取り | EditClient.tsx:289 | `e.target.files?.[0]` で 1 枚のみ取得、残りを破棄 |
| upload state | EditClient.tsx:312-352 | `pendingFile: File \| null` で 1 枚 / queue 不在 |
| processing 表示 | EditClient.tsx:412 / `frontend/lib/editPhotobook.ts:81,250` | `view.processingCount` / `view.failedCount` で枚数のみ表示。誰が processing なのか個別 tile 単位は出ない |
| 編集開始の制御 | EditClient.tsx 全体 | processing 中も grid / caption / cover / settings に触れる。実装上は触れても OCC で守られるが、UX 上は壊れている |
| polling | EditClient.tsx:121-127 | `processingCount > 0` の間 5 秒間隔で `fetchEditView` を再取得。timeout / abort なし |

### 2.3 `/draft/[token]/route.ts` の遷移先固定

`frontend/app/(draft)/draft/[token]/route.ts:55-60`:

```typescript
const res = NextResponse.redirect(
    buildEditPageUrl(out.photobookId),  // ← `/edit/<id>` ハードコード
    302,
);
```

→ **/draft session 交換成功時、`/edit/<id>` ではなく Upload Staging 画面に遷移させる**には本ファイルを修正する必要がある（後述 §5.2）。

### 2.4 middleware の auth boundary

`frontend/middleware.ts:21`:

```typescript
const SENSITIVE_PATH_PREFIXES = ["/draft", "/manage", "/edit"];
```

→ 新規 route prefix（`/prepare` 想定）を **必ずこの配列に追加**する（`Referrer-Policy: no-referrer` 付与のため、token URL 漏洩対策と整合）。

### 2.5 既存 API 棚卸し（再利用可能）

| API | path | 用途 | /prepare で再利用可 |
|---|---|---|---|
| upload-verifications | `POST /api/photobooks/{id}/upload-verifications/` | Turnstile siteverify + verification session 発行（max 20 intent / 30 分） | ✓（複数 upload で **session 1 つを使い回せる**） |
| upload-intent | `POST /api/photobooks/{id}/images/upload-intent` | presigned PUT URL 発行 | ✓ |
| R2 PUT（直接） | presigned URL | original object upload | ✓ |
| complete | `POST /api/photobooks/{id}/images/{imageId}/complete` | upload 完了通知、`status='processing'` 遷移 + R2 HeadObject 検証 | ✓ |
| edit-view | `GET /api/photobooks/{id}/edit-view` | image list + processingCount + failedCount | ✓（/prepare は edit-view をそのまま polling、または専用 view が必要かは §8 で判断） |
| removePhoto | `DELETE /api/photobooks/{id}/photos/{photoId}` | **page に配置済の photo** を削除 | ✗（page-attached photo の削除であり、未配置の raw image 削除には対応していない可能性） |

→ **新規 Backend API は P0 では不要**（既存 upload 系 + edit-view で /prepare の最小機能は構成可能）。`removeImage` 系は P1 候補（§11）。

---

## 3. 採用導線（推奨）

```
LP (/)
 → /create
 → /draft/<token>
 → /prepare/<photobookId>      （新規、画像投稿専用画面）
   ├─ 複数画像をまとめて選択
   ├─ R2 PUT を concurrency=2 で並列実行
   ├─ 各画像 tile に queued/uploading/processing/available/failed を表示
   ├─ 5 sec polling で edit-view を再取得し status を更新
   ├─ 「編集へ進む」を **すべて available** で enable
 → /edit/<photobookId>         （既存、upload UI を最小化、編集に集中）
```

**画像 upload は /prepare の責務**、**page 配置 / caption / cover / publish settings は /edit の責務**として分離する。

---

## 4. 推奨 route 比較

| 候補 | 評価 |
|---|---|
| `/upload/<photobookId>` | 短く分かりやすい。**ただし将来「画像準備」「処理待ち」「再試行」「未配置画像のレビュー」まで含む可能性があり、"upload" だけでは scope が狭い**（ユーザコメントと一致） |
| **`/prepare/<photobookId>`**（**第一候補**） | "編集前の準備画面" として scope が広く、上記の機能拡張に耐える。短く非技術的 |
| `/draft/<token>/images` | raw token を URL に含む経路を増やすことになり、ADR-0003 の「raw token を URL bar に残さない」原則と矛盾。**却下** |
| `/edit/<photobookId>/images` | 認可・編集配下で自然だが、ユーザには URL が技術的。`/prepare` と比較して将来の機能拡張余地で劣る |

**推奨: `/prepare/<photobookId>`**（ユーザの第一候補と一致）。

### 4.1 後方互換性

- 既存ユーザの `/edit/<photobookId>` リンク: 維持（直接アクセス時は /edit が引き続き動作、編集画面のまま）
- 新規ユーザの `/draft/<token>` 経由: **`/prepare/<photobookId>` に redirect**（後述 §5.2）
- 既存ユーザが `/edit` を bookmark していたケース: 引き続き /edit に到達できるため UX 後退なし

---

## 5. 認可 / session 設計

### 5.1 既存 draft session cookie をそのまま使う

`/edit/<photobookId>` で使われている cookie は `buildSessionCookieName("draft", out.photobookId)` 形式（`frontend/lib/cookies.ts`）。**photobook id ごとに scope 化**された draft session token。`/prepare/<photobookId>` も同じ photobook id に対する操作なので、**同 cookie をそのまま使える**。

- 認可: middleware 経由ではなく、サーバ側 (Backend API) が cookie を検証する既存方式を踏襲
- manage token とは完全に分離（`/prepare` は draft 状態のみ対応、published 後は `/manage/<id>` に集約済の既存設計に従う）

### 5.2 `/draft/[token]/route.ts` の redirect 先変更

`frontend/app/(draft)/draft/[token]/route.ts` の `buildEditPageUrl(out.photobookId)` を **`buildPreparePageUrl(out.photobookId)`** に変更する。

採用案（**A**）:
- 関数 `buildPreparePageUrl` を新設（`frontend/lib/cookies.ts` または新規 `frontend/lib/routes.ts`）
- /draft/[token]/route.ts の redirect 先を `/prepare/<id>` 一択にハードコード
- 既存 `buildEditPageUrl` は edit 画面遷移（manage URL からの edit / 直接アクセス等）で残す

採用しない案（**B**: `next=` query で柔軟化）:
- `/draft/<token>?next=/prepare/<id>` のようにクエリで遷移先を指定
- 攻撃面が広がる（任意 URL を指定されると open redirect 化）。MVP では **A** 一択にハードコードし、将来必要になったら B に拡張する判断にする

### 5.3 middleware への `/prepare` 追加（必須）

`frontend/middleware.ts:21`:

```typescript
const SENSITIVE_PATH_PREFIXES = ["/draft", "/manage", "/edit", "/prepare"];
```

→ `/prepare/*` も `Referrer-Policy: no-referrer` を付与（既存 sensitive path と整合）。X-Robots-Tag: noindex, nofollow は全ページ付与済なので追加対応不要。

---

## 6. Upload Staging 画面の仕様（`/prepare/<photobookId>`）

### 6.1 画面構成（MVP / P0）

```
┌─────────────────────────────────────────────────┐
│ ① Header                                       │
│   "フォトブックに追加する写真を選んでください"  │
│   "全部の写真が処理完了したら編集画面に進めます"│
├─────────────────────────────────────────────────┤
│ ② File Picker                                  │
│   [📷 写真を選択]  ← <input multiple>          │
│   または [⬆ ここにドロップ]（PC のみ、P1）     │
├─────────────────────────────────────────────────┤
│ ③ Image Tiles Grid                             │
│   ┌──┐ ┌──┐ ┌──┐ ┌──┐                          │
│   │①│ │②│ │③│ │④│                            │
│   └──┘ └──┘ └──┘ └──┘                          │
│   各 tile: status badge + filename + 進捗      │
├─────────────────────────────────────────────────┤
│ ④ Summary                                      │
│   "5 枚: 完了 3 / 処理中 2 / 失敗 0"           │
├─────────────────────────────────────────────────┤
│ ⑤ Actions                                      │
│   [編集へ進む] (disabled until all available) │
│   [保存して後で続ける]（任意、P1）             │
└─────────────────────────────────────────────────┘
```

### 6.2 file input

- `<input type="file" multiple accept="image/jpeg,image/png,image/webp">`
- mobile: Photos picker、PC: file picker
- drag & drop は **P1**（MVP は file picker のみ）

### 6.3 client-side queue + concurrency

- 選択ファイルを `queue: File[]` に追加
- 状態 enum: `queued` → `verifying` → `uploading` → `completing` → `processing` → `available` / `failed`
- **concurrency = 2**（R2 PUT 同時 2 本まで、UsageLimit `upload_verification.issue` の固定窓 RateLimit / Cloud SQL connection / Turnstile session 上限と整合）
- 各 file の進捗（uploading 中の bytes 進捗 / 全体 % bar）を tile に表示

### 6.4 Turnstile / upload-verification session の使い回し

- /prepare に入った時点で Turnstile widget 表示（既存 `TurnstileWidget` component を再利用）
- `POST /api/photobooks/{id}/upload-verifications/` で発行された **`upload_verification_session` 1 つを 20 image intent まで使い回せる**（PR20 設計、30 分有効）
- 21 枚目以降は新しい session が必要。MVP は **20 枚上限** UI ガード + 21 枚目で「分割して投稿してください」を表示

### 6.5 各 tile の状態表現

| status | badge | tile UI |
|---|---|---|
| `queued` | 灰色 "待機中" | 半透明 placeholder、filename のみ |
| `verifying` | 青 "認証中" | spinner |
| `uploading` | 青 "送信中 N%" | プログレスバー |
| `completing` | 青 "完了処理中" | spinner |
| `processing` | オレンジ "処理中" | spinner + 「最大 5 分ほどお待ちください」（Scheduler 5 min を考慮） |
| `available` | 緑 "完了" | thumbnail variant URL を表示 |
| `failed` | 赤 "失敗" | failure_reason を redact し「再試行」「削除」ボタン（P1） |

### 6.6 polling 設計

- /prepare 入場時に `fetchEditView` で初期 image list を取得
- `view.processingCount > 0 \|\| queue に upload 中のものあり` の間、**5 秒間隔**で `fetchEditView` を再取得し各 tile の status を更新
- max polling duration: **10 分**（Scheduler 5 min × 2 サイクル + バッファ）。10 分超で「処理が遅延しています。再読み込みしてください」+ 手動 reload ボタン
- exponential backoff: 5s → 5s → 10s → 20s → 60s 上限（Cloud Run / Cloud SQL 過負荷回避）
- polling は abort 可能（page unmount で確実に止める、useEffect cleanup）

### 6.7 「編集へ進む」ボタン enable 条件

```typescript
const canProceedToEdit =
  view.images.length > 0 &&                       // 1 枚以上あること
  queue.every((q) => q.status === "available") && // queue 内全 available
  view.processingCount === 0 &&                   // server 側 processing も 0
  !uploadingAny;                                  // upload 中なし
```

クリックで `window.location.assign(\`/edit/\${photobookId}\`)` で /edit に遷移（cookie はそのまま使い回せる）。

### 6.8 失敗ハンドリング

- failed image tile に「再試行（P1）」「削除（P1）」ボタンを将来配置するための space を確保
- MVP P0 では failed tile を表示するだけで、再試行 / 削除はサポートしない（ユーザは page reload で全 queue を初期化、または /edit に進んで個別対応）
- failure_reason は user-friendly に redact（"対応していない形式です" / "サイズが大きすぎます" 等の固定文言、敵対者対策で詳細は出さない）

---

## 7. /edit 画面側の変更

### 7.1 MVP P0 では最小変更

- 既存の upload UI（`<input type="file">`）は **残すが目立たせない**（"画像を追加" ボタンを画面下部に小さく置く、メインは grid 編集）
- processing 中の placeholder 表示は維持（既存挙動）
- /edit 直アクセス時の挙動は不変（manage URL 経由 / bookmark 経由は引き続き動く）

### 7.2 後続 PR で削減検討

- /edit から upload UI を完全に削除し、`/prepare/<id>` に集約する判断は P1 / 別 PR
- MVP では「/prepare がメイン」「/edit は緊急用に upload も残す」のハイブリッド

---

## 8. Backend / API の追加要否

### 8.1 P0 では Backend 変更ゼロ

- /prepare は **既存 5 API をそのまま再利用**（upload-verifications / upload-intent / R2 PUT / complete / edit-view）
- edit-view が image list + status + processingCount + failedCount を返すため、polling だけで /prepare の状態管理は完結
- Backend deploy 不要、migration 不要、Secret / env 変更不要

### 8.2 P1 候補（必要に応じて別 PR）

| API | 必要性 | 備考 |
|---|---|---|
| `DELETE /api/photobooks/{id}/images/{imageId}` (raw image 削除) | failed image / 配置前 image を /prepare から削除する場合に必要 | 既存 `removePhoto` は page 配置済 photo 用、raw image 削除は別 endpoint が必要 |
| `POST /api/photobooks/{id}/images/{imageId}/retry` (再試行) | failed image を retry させる場合 | image-processor 側でも再 claim 可能にする必要あり（status を failed → processing に戻す API） |
| `GET /api/photobooks/{id}/prepare-view` (専用 view) | edit-view より軽量にしたい場合 | 不要（edit-view で十分軽量、新規 endpoint を増やすのは scope 外） |
| `POST /api/photobooks/{id}/process-now` (即時 image-processor trigger) | Scheduler 5 min 待ちを短縮 | §9 で検討、現状は不要 |

---

## 9. image-processor との関係

### 9.1 現状の Scheduler 5 min は MVP では維持

- /prepare で適切に「最大 5 分ほどお待ちください」を表示すれば UX 上受容可能
- Scheduler 1 min 化はコスト微増 + Job 起動オーバヘッド増（既存 plan §5.5 の cost analysis 通り、月数 USD オーダー）
- まず /prepare の UX を整えてから判断する。1 min 化は別 PR（後追い）

### 9.2 即時 trigger の代案（採用しない、判断記録）

| 案 | 評価 |
|---|---|
| Backend handler が Cloud Run Admin API で Job 即時 invoke | 認可・課金・複雑性が増す。MVP では不採用 |
| Scheduler 1 min 化 | 月数 USD コスト微増、Job 多重起動の恐れ（FOR UPDATE SKIP LOCKED で守られるが余計な起動）。**P2 候補** |
| complete handler 同期 image-processor 呼び出し | Cloud Run timeout（5 min default）、メモリ消費、retry 設計の複雑化。**不採用**（既存 plan §3.2 の判断と整合） |

→ **MVP は Scheduler 5 min 維持 + /prepare で適切な待機 UX**。

---

## 10. Safari / iPhone Safari 確認観点

### 10.1 必須観点（STOP ε で実施）

| # | 観点 | macOS Safari | iPhone Safari |
|---|---|---|---|
| 1 | `<input type="file" multiple>` で **Photos picker から複数選択できる** | ✓ | ✓（iOS 14+ で標準対応、要実機確認） |
| 2 | `/prepare/<id>` 表示 OK（grid / picker / Turnstile / 進捗 / button） | ✓ | ✓（縦向き、横スクロールなし） |
| 3 | concurrency=2 並列 upload 中に UI が固まらない |  |  |
| 4 | upload 中に screen lock / background でも復帰時に再開できる | — | ✓（ITP / Service Worker 制約あり、再開しなくても最低 polling で status 取得継続） |
| 5 | processing 中の polling が ITP で session 失効しない |  |  |
| 6 | 「編集へ進む」が disabled → enabled の状態遷移を視認できる |  |  |
| 7 | `/edit/<id>` 遷移後に既存編集画面が破綻なく表示 |  |  |
| 8 | Console / Network に Secret / raw token / Cookie 値が出ていない |  |  |
| 9 | 5 分以上の長期 polling 後の Cookie 残存（24 h は smoke 範囲外、運用観測） | — | — |

### 10.2 既存 rule に従う

- `.agents/rules/safari-verification.md`: Cookie / redirect / OGP / ヘッダ変更時の Safari 必須確認 → /prepare 新設は middleware 変更を伴うので **Safari 必須**
- `.agents/rules/turnstile-defensive-guard.md` L0–L4: /prepare の Turnstile widget も既存 ReportForm / Upload と同パターンで実装

---

## 11. P0 / P1 / P2 分割

### 11.1 P0（本 PR の MVP）

- `/prepare/<photobookId>` route + page 新設
- `<input type="file" multiple>` で複数選択
- client-side queue + concurrency=2 並列 upload
- 各 tile に queued/uploading/processing/available/failed status badge
- 5 sec polling + max 10 min duration + exponential backoff
- 「編集へ進む」ボタン enable 条件（全 available）
- Turnstile widget 1 回完了 + upload-verification session 1 つで複数 intent
- 20 枚上限 UI ガード
- `/draft/[token]/route.ts` の redirect 先を `/prepare/<id>` に変更
- middleware の `SENSITIVE_PATH_PREFIXES` に `/prepare` 追加
- Workers redeploy（Frontend のみ、Backend / Job 変更なし）
- Safari 実機 smoke

### 11.2 P1（後続 PR）

- `URL.createObjectURL` で **upload 完了前の preview 即時表示**
- failed image の **「再試行」「削除」**ボタン + 必要なら Backend `removeImage` / `retryImage` API
- drag & drop（PC のみ）
- 進捗 bar の細粒度化（XHR 経由の bytes 進捗）
- `/edit` 画面から upload UI を縮小・削除
- `polling` の max duration / backoff の調整
- 21 枚目以降の自動分割 / 案内

### 11.3 P2（運用観測後の改善）

- image-processor Scheduler を 1 min に短縮（Budget / multi-fire 検証後）
- Backend 経由の即時 image-processor trigger（Cloud Run Admin API call）
- HEIC / 大画像のクライアントサイドリサイズ
- batch concurrency=4 への引き上げ（負荷テスト後）

---

## 12. STOP 設計

| STOP | 内容 | 実 GCP / 課金影響 | 必要承認 |
|---|---|---|---|
| **α**（本書） | 設計判断資料、route / 認可 / UI 仕様 / Backend / Scheduler / smoke 観点を確定 | なし | ユーザ承認待ち |
| **β** | Frontend 実装: `/prepare/<id>` page + queue + Turnstile + tile UI + tests + `/draft/[token]/route.ts` redirect 先変更 + middleware `/prepare` 追加 + 単一 commit + push | なし | β 実装着手承認 |
| **γ** | Backend deploy: **不要**（本 PR は Frontend のみ） | なし | スキップ |
| **δ** | Workers redeploy（Frontend bundle に新 `/prepare` route + middleware 変更が含まれるため必須） | 微（Workers deploy のみ） | δ deploy 承認 |
| **ε** | Safari 実機 smoke: macOS / iPhone Safari で `/` → `/create` → `/draft/<token>` → `/prepare/<id>` → 複数 image upload → 全 available 待機 → 「編集へ進む」 → `/edit/<id>` までの 1 周。observation のみ、写真は smoke 用無害値、submit 完了後は draft_expires_at 自然 GC | 微（smoke で R2 / DB に書き込みあり、自然 GC 方針で残置） | ε smoke 承認 |
| **final** | work-log / roadmap / runbook / failure-log 判断 / commit + push、本 PR closeout | なし | 完了報告 |

---

## 13. 実装対象ファイル案（STOP β で着手）

### 13.1 新規ファイル

| ファイル | 内容 |
|---|---|
| `frontend/app/(draft)/prepare/[photobookId]/page.tsx` | Server Component。draft session cookie を SSR で読み、`fetchEditView` で初期 image list 取得、PrepareClient に渡す |
| `frontend/app/(draft)/prepare/[photobookId]/PrepareClient.tsx` | "use client"。queue state / concurrency / Turnstile / polling / tile UI |
| `frontend/app/(draft)/prepare/[photobookId]/__tests__/PrepareClient.test.tsx` | SSR レンダリング検証（複数 tile、status badge、enable 条件、Cookie / token 非露出） |
| `frontend/components/Prepare/ImageTile.tsx` | 1 image tile component（status badge + filename + 進捗） |
| `frontend/components/Prepare/UploadQueue.ts` | client-side queue 実装（File[] → 並列 R2 PUT + complete chain）。pure function で test 容易化 |
| `frontend/components/Prepare/__tests__/UploadQueue.test.ts` | concurrency / queue 状態遷移 / failed handling のユニットテスト |
| `frontend/lib/preparePhotobook.ts`（任意） | /prepare 専用の薄い wrapper（`fetchEditView` の polling helper 等）。新規 Backend API は呼ばない |

### 13.2 既存ファイル修正

| ファイル | 修正内容 |
|---|---|
| `frontend/app/(draft)/draft/[token]/route.ts` | `buildEditPageUrl` → `buildPreparePageUrl` に変更（line 55-60） |
| `frontend/lib/cookies.ts` または新規 `frontend/lib/routes.ts` | `buildPreparePageUrl(photobookId)` を export |
| `frontend/middleware.ts` | `SENSITIVE_PATH_PREFIXES` に `"/prepare"` を追加 |
| `frontend/app/(draft)/edit/[photobookId]/EditClient.tsx` | upload UI を画面下部に移動 / 縮小（P0 では削除しない、ハイブリッド） |
| `frontend/app/__tests__/middleware.test.ts`（あれば） | `/prepare` prefix が sensitive 扱いされることを assert |

### 13.3 Backend / Scheduler / Job

- **変更なし**（本 PR は Frontend のみ、既存 5 API + edit-view + image-processor 自動化基盤を再利用）

---

## 14. リスク / 制約

| # | リスク | 影響 | 緩和 |
|---|---|---|---|
| R1 | iPhone Safari の Photos picker が想定通り複数選択できない | upload 機能の根幹が成立しない | STOP ε 実機 smoke で必ず確認、failback として「1 枚ずつ」モードを維持 |
| R2 | concurrency=2 で R2 / Cloud SQL 接続が瞬間的に増加し 503 / connection limit | upload 失敗 | 初期は concurrency=2 から、観測しながら調整。R2 はスロット制限緩い、Cloud SQL は instance 上限を STOP δ で確認 |
| R3 | Turnstile session 上限 20 を超える 21 枚目で詰まる | 21 枚以上の photobook を作りたいユーザがブロック | UI で 20 枚上限ガード + 「分割投稿してください」明示。session 再発行は P1 |
| R4 | Scheduler 5 min 待機が UX 上長い | ユーザ離脱 | /prepare で「最大 5 分」明示 + 進捗表示 + page 離脱しても resume 可能（cookie scope 内なら次回戻ってくれば polling 再開） |
| R5 | /draft/[token] の redirect 先を `/prepare/<id>` に変えると、既存ユーザの初回経路が変わる | 既存 draft 持ちが `/edit` を期待している | /edit は引き続き動作（直接アクセス可）。redirect 先変更は新規作成の挙動だけ。既存 draft へは影響なし |
| R6 | failed image を P0 で削除できない | 失敗が貯まると UI が見にくい | MVP は page reload で初期化を案内、P1 で removeImage API を追加 |
| R7 | polling が page background でも継続して battery / data を消費 | mobile で問題 | Page Visibility API で background 時は polling を一時停止、foreground 復帰で再開（P0 でも実装簡易） |
| R8 | edit と prepare の状態が乖離（prepare で available だが edit では未反映） | 一貫性のない見え方 | edit-view を polling で取得しているため、最新が常に reflect される。prepare → edit 遷移時に prepare が知っている全 image が edit にも見えることを smoke で確認 |
| R9 | Workers redeploy が Backend と同期せず、`/prepare` 配信前に古い `/draft/[token]` が `/edit` を返す瞬間がある | 短時間の経路混在 | STOP δ で deploy 直後 5 分の routing transient を観測（既存 runbook §1.4.1 と同方針） |
| R10 | raw photobook_id が URL bar に残る | 既知（既存 `/edit/<id>` も同じ）。**raw token は残らない**（draft session cookie は HttpOnly） | 既存設計と整合、追加リスクなし |

---

## 15. 制約遵守

- 本書に raw `photobook_id` / `image_id` / `slug` / token / Cookie / `storage_key` / upload URL / R2 endpoint / `DATABASE_URL` / Secret 値を記載していない（URL 例示は `<id-redacted>` または `<photobookId>` placeholder）
- 本書作成時点で実 GCP 操作 / 本番 DB 書き込み / Job execute は行っていない
- production DB / Job execute / Scheduler 変更は STOP β 以降の承認後に限定
- `.claude/scheduled_tasks.lock` は触らない

---

## 16. closeout で更新する資料

- `harness/work-logs/2026-05-XX_upload-staging-result.md`（新規）— STOP β 実装範囲 / δ deploy 結果 / ε smoke 結果（redacted） / 観測 SLO
- `docs/plan/vrc-photobook-final-roadmap.md` — 本 PR を新 PR 番号で記録、image-processor PR / create-entry PR との依存関係を明記
- `docs/plan/m2-frontend-upload-ui-plan.md` — PR22 で「複数同時は MVP 範囲外」と先送りした記述を「本 PR で解消」に更新
- `docs/plan/m2-image-processor-job-automation-plan.md` §10 / §11 — Scheduler 5 min 維持判断を追記
- `docs/plan/m2-frontend-edit-ui-fullspec-plan.md` — /edit から upload UI を P0 では残し、P1 で縮小する旨を追記
- `CLAUDE.md` 主要動線で実装済の機能欄 — "upload staging（複数画像投入 + processing 待ち分離）" を追加
- `harness/failure-log/` — `/edit` への upload + processing 同居が UX を壊した経緯（本 PR の起点となった observation）を起票するか判断
- `.agents/rules/` — 「非同期処理が長い operation は専用画面に分け、結果待ち中の編集機能と同居させない」をルール化するか判断

---

## 17. 承認文案（本書承認時にユーザが返してよい雛形）

```text
m2-upload-staging STOP α 設計を承認します。

確定事項:
1. route は /prepare/<photobookId> を採用
2. /draft/[token] の redirect 先を /prepare/<id> にハードコード変更
3. middleware の SENSITIVE_PATH_PREFIXES に /prepare 追加
4. Backend / migration / Secret / Scheduler / Job spec の変更なし、Frontend のみ
5. P0 範囲: §11.1 の通り（複数選択 / concurrency=2 / 5sec polling / 編集へ進む enable 条件）
6. P1 / P2 は別 PR で扱う（preview 即時 / drag & drop / failed retry / Scheduler 1 min 化等）
7. Scheduler は 5 min 維持
8. STOP 設計は §12 の通り（β 実装 → γ skip → δ Workers deploy → ε Safari 実機 smoke → final closeout）

STOP β 実装に進んでよい。raw token / ID / Cookie / Secret は記録しない方針を維持。
.claude/scheduled_tasks.lock は触らない。
```

---

## 18. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-01 | 初版作成（STOP α）。/edit 同居が image-processor 遅延で UX を壊す問題と、複数画像同時投入未対応を Upload Staging 画面導線で解決する PR として独立計画化 |
