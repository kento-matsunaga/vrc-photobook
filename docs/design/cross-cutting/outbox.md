# Outbox 設計書

> 上流: [業務知識定義書 v4](../../spec/vrc_photobook_business_knowledge_v4.md) §2.8, §6.11
>
> 集約の状態変更と副作用（OGP生成、メール送信、CDNキャッシュパージ、画像物理削除等）を**トランザクション的に保証**するための Transactional Outbox パターン。MVP から採用する。

---

## 1. 目的

MVPでは以下の問題を防ぐことを目的とする。

- Photobookを公開したが OGP 生成が走っていない
- 運営が一時非表示にしたが CDN キャッシュが消えていない
- Photobook を物理削除したが所属 Image が残っている
- 管理URL再発行したがメールが送られていない

これらは「状態変更」と「副作用実行」が別トランザクションで行われることに起因する典型的な不整合。Outboxパターンで、状態変更時に**同一DBトランザクション内で** `outbox_events` にINSERTし、別ワーカーが確実に処理する。

---

## 2. 基本ルール

- 集約の状態変更（Photobook/Report/Moderation/Image/ManageUrlDelivery 等）と `outbox_events` へのINSERTは**同一DBトランザクション**で実行
- 実際の副作用（外部API呼び出し、メール送信、CDN操作等）は**非同期ワーカー**で実行
- 失敗時は指数バックオフで retry、上限超過で `failed` 状態に遷移
- `failed` 状態の outbox は Reconcile 対象（`outbox_failed.sh`）

---

## 3. `outbox_events` テーブル

### 3.1 カラム定義

| カラム | 型 | NULL | 既定 | 制約・備考 |
|-------|-----|------|------|----------|
| `id` | `uuid` | NOT NULL | `gen_random_uuid()` | PK |
| `aggregate_type` | `text` | NOT NULL | - | `photobook / report / moderation / image / manage_url_delivery` |
| `aggregate_id` | `uuid` | NOT NULL | - | 対象集約のID |
| `event_type` | `text` | NOT NULL | - | イベント種別（§4 参照） |
| `payload_json` | `jsonb` | NOT NULL | `'{}'` | イベントペイロード |
| `status` | `text` | NOT NULL | `'pending'` | CHECK: `pending / processing / processed / failed` |
| `retry_count` | `int` | NOT NULL | `0` | 試行回数 |
| `next_retry_at` | `timestamptz` | NULL | - | 次回試行予定時刻 |
| `created_at` | `timestamptz` | NOT NULL | `now()` | |
| `processing_started_at` | `timestamptz` | NULL | - | 直近の処理開始時刻 |
| `processed_at` | `timestamptz` | NULL | - | 成功時刻 |
| `failed_at` | `timestamptz` | NULL | - | `failed` 確定時刻 |
| `failure_reason` | `text` | NULL | - | 失敗理由（簡潔） |

### 3.2 索引

| 索引 | カラム | 用途 |
|------|-------|------|
| PK | `id` | |
| INDEX | `status, next_retry_at` WHERE `status IN ('pending', 'failed')` | ワーカーのピック対象抽出 |
| INDEX | `aggregate_type, aggregate_id, created_at DESC` | 特定集約のイベント履歴 |
| INDEX | `event_type, status` | イベント種別別集計 |
| INDEX | `status, failed_at DESC` WHERE `status='failed'` | Reconcile 対象抽出 |

---

## 4. 業務イベント種別（v4 §2.8）

集約別にまとめる。

### 4.1 Photobook 関連

| event_type | 発火条件 | 副作用 |
|-----------|---------|-------|
| `PhotobookPublished` | Photobook が公開された | OGP生成ジョブ作成、公開URL確認初期処理 |
| `PhotobookUpdated` | 公開済Photobookの内容更新 | OGPをstaleに、OGP再生成ジョブ、CDNキャッシュ無効化 |
| `PhotobookHidden` | 運営が一時非表示 | 公開ページCDN無効化、OGPキャッシュ無効化 |
| `PhotobookUnhidden` | 運営が解除 | 公開ページCDN無効化、必要ならOGP再生成 |
| `PhotobookSoftDeleted` | 論理削除 | 公開ページCDN無効化、OGPキャッシュ無効化 |
| `PhotobookRestored` | 論理削除から復元 | 公開ページCDN無効化、OGP再生成 |
| `PhotobookPurged` | 物理削除 | 所属ImageとOGP画像の物理削除ジョブ、CDN削除 |
| `ManageUrlReissued` | 管理URL再発行 | 旧URL無効化、新URL控え送付（任意）、ModerationAction記録 |

### 4.2 Report 関連

| event_type | 発火条件 | 副作用 |
|-----------|---------|-------|
| `ReportSubmitted` | 通報送信 | 運営通知送信（`minor_safety_concern` 等は通知レベルを上げる） |

### 4.3 ManageUrlDelivery 関連

| event_type | 発火条件 | 副作用 |
|-----------|---------|-------|
| `ManageUrlDeliveryRequested` | 作成者が管理URL控えメール送信を要求 | メール送信ジョブ作成 |

### 4.4 Image 関連 <!-- 付録C P0-27 -->

| event_type | 発火条件 | 副作用 |
|-----------|---------|-------|
| `ImageIngestionRequested` | upload-intent + complete 完了時、Image 状態更新と同一 TX で INSERT。image-processor に本検証・変換・variant 生成・EXIF 除去を依頼 | image-processor ワーカーがマジックナンバー検証 / 実デコード / HEIC 変換 / EXIF/XMP/IPTC 除去 / display・thumbnail・OGP variant 生成を実行。成功で Image を `available`、失敗で `failed` + `failure_reason` 記録 |

(参照: v4 §2.9 / ADR-0005 / Image 集約 §11)

---

## 5. 状態遷移

```
              INSERT（同一トランザクション）
                    ↓
              ┌──────────┐
              │  pending │
              └────┬─────┘
                   │ ワーカーがピック
                   ▼
              ┌────────────┐
              │ processing │
              └────┬───────┘
                   │
         ┌─────────┴──────────┐
         │ 成功               │ 失敗
         ▼                    ▼
   ┌──────────┐       ┌──────────┐
   │ processed│       │  pending │ ← retry（指数バックオフ）
   └──────────┘       └────┬─────┘
                           │ retry_count が上限超過
                           ▼
                      ┌──────────┐
                      │  failed  │ ← Reconcile対象
                      └──────────┘
```

### Retry ポリシー

| retry_count | 次回試行までの待機 |
|-------------|------------------|
| 0 → 1 | 1 分 |
| 1 → 2 | 5 分 |
| 2 → 3 | 30 分 |
| 3 → 4 | 2 時間 |
| 4 → 5 | 12 時間 |
| 5 以降 | `failed` に遷移 |

指数バックオフの具体値は運用調整可。`outbox_policies` としてDB化または定数化。

---

## 6. ワーカー実装方針

### 6.1 ピックの排他制御

複数ワーカー並列稼働を想定し、SELECT FOR UPDATE SKIP LOCKED（PostgreSQL）でイベントを取得する。

```sql
UPDATE outbox_events
SET status = 'processing', processing_started_at = now()
WHERE id = (
  SELECT id FROM outbox_events
  WHERE status = 'pending'
    AND (next_retry_at IS NULL OR next_retry_at <= now())
  ORDER BY created_at
  FOR UPDATE SKIP LOCKED
  LIMIT 1
)
RETURNING *;
```

### 6.2 処理ハンドラ

`event_type` ごとにハンドラを登録する。

```
handlers = {
  "PhotobookPublished": generate_ogp_and_init,
  "PhotobookUpdated":   invalidate_cache_and_regen_ogp,
  "PhotobookHidden":    invalidate_cache,
  "PhotobookSoftDeleted": invalidate_cache,
  "PhotobookPurged":    purge_images_and_cdn,
  "ReportSubmitted":    notify_operators,
  "ManageUrlDeliveryRequested": send_mail,
  "ImageIngestionRequested": process_image_async,    // 付録C P0-27 / ADR-0005
  ...
}
```

### 6.3 処理後

- 成功 → `status='processed'`, `processed_at=now()`
- 失敗（リトライ可能） → `status='pending'`, `retry_count++`, `next_retry_at = now() + backoff(retry_count)`
- 失敗（上限） → `status='failed'`, `failed_at=now()`, `failure_reason=...`

---

## 7. Idempotency（冪等性）

副作用は複数回実行されうる（ワーカーのクラッシュ、重複ピック等）。ハンドラは冪等に設計する。

- OGP生成 → 既存OGP行を参照し、同バージョンなら再生成しない
- CDNパージ → 冪等（同じURLを複数回パージしても問題ない）
- メール送信 → `manage_url_delivery_attempts` で attempt_number を持ち、重複送信を抑止
- 画像物理削除 → ストレージから存在確認後に削除

---

## 8. 保持期間と削除

| 対象 | 保持期間 | アクション |
|------|---------|----------|
| `status='processed'` | **30日** | 日次バッチで物理削除 |
| `status='failed'` | **無期限**（Reconcile 完了まで） | 手動確認＋再投入または削除 |
| `status='processing'` で長時間滞留 | **1時間** | `pending` に戻す（ワーカークラッシュ想定） |

---

## 9. 監視

以下を監視する：

- `pending` のキュー深度（バックログ）
- `failed` の件数
- `processing` で 1時間以上滞留するレコード数
- retry_count が高いイベントの分布

---

## 10. 他集約との境界

Outbox は**集約ではなく横断コンポーネント**。以下の関係を持つ：

| 集約 | 書き込みイベント | 読み取り（ハンドラ内） |
|------|----------------|-------------------|
| Photobook | Published/Updated/Hidden/Unhidden/SoftDeleted/Restored/Purged/ManageUrlReissued | - |
| Report | ReportSubmitted | Moderationハンドラ内でReport状態更新（ただしOutboxを経由しない、同一TX） |
| Moderation | （Moderation実行時にPhotobook/Reportの上記イベントが発生） | - |
| ManageUrlDelivery | ManageUrlDeliveryRequested | - |
| Image | **`ImageIngestionRequested`**（complete API 完了時、Image 状態更新と同一 TX） + （Photobook経由で間接的に関与、PhotobookPurged ハンドラが Image 削除） | image-processor ハンドラが本検証・variant 生成を実行 <!-- 付録C P0-27 --> |

---

## 11. インフラ層のマッピング

```
backend/outbox/
├── domain/
│   ├── entity/
│   │   └── outbox_event.go
│   └── vo/
│       ├── event_id/
│       ├── aggregate_type/
│       ├── event_type/
│       ├── event_status/
│       └── retry_policy/
├── infrastructure/
│   └── repository/rdb/
│       ├── outbox_repository.go
│       └── worker_picker.go           # SKIP LOCKED 実装
└── internal/
    ├── dispatcher/
    │   └── event_dispatcher.go        # event_type → handler ルーティング
    ├── handlers/
    │   ├── photobook_published.go
    │   ├── photobook_updated.go
    │   ├── photobook_purged.go
    │   ├── report_submitted.go
    │   ├── manage_url_delivery_requested.go
    │   └── image_ingestion_requested.go    # 付録C P0-27 / ADR-0005
    └── worker/
        └── outbox_worker.go           # ワーカー本体
```

---

## 12. v4 業務知識・ADR・付録C との対応

| 項目 | 参照先 | 本書項目 |
|------|-------|---------|
| §2.9 Outbox / Reconcile 用語 | v4 §2.9 | §1 目的, §11 |
| §6.11 集約間イベントは Outbox で伝搬 | v4 §6.11 | §2 基本ルール |
| §6.12 整合性は Reconcile で保証 | v4 §6.12 | §10 他集約境界 / Reconcile 連携 |
| P0-13 outbox_events をMVPから追加 | v3→v4 改訂 | 全体 |
| P0-15 outbox イベント種別の列挙 | v3→v4 改訂 | §4 |
| P0-5 Moderation と Report と Outbox の同一トランザクション | v3→v4 改訂 | §2 基本ルール, §10 |
| 付録C P0-27 `ImageIngestionRequested` を outbox_events に含める | 付録C | §4.4, §6.2, §10 <!-- 付録C P0-27 --> |
| 付録C P0-28 状態変更と同一 TX で INSERT | 付録C | §2 基本ルール <!-- 付録C P0-28 --> |
| 付録C P0-29 failed Outbox は reconcile 対象 | 付録C | §1, §5 状態遷移, §6.3, §9 監視 <!-- 付録C P0-29 --> |
| ADR-0005 R2 presigned upload + 非同期処理 | ADR-0005 | §4.4 ImageIngestionRequested |

---

## 13. 次工程への引き継ぎ事項

本横断設計の P0 反映を後続作業に渡すための申し送り。

### 13.1 M3 マイグレーション

- `outbox_events` テーブル migration（goose）
- 索引（§3.2）、status CHECK 制約

### 13.2 M4 各 UseCase の同一 TX INSERT 責務

- Photobook 集約 8 種 / Report / ManageUrlDelivery / Image（complete API） すべての UseCase で同一 TX INSERT を担保 <!-- 付録C P0-28 -->
- Repository インターフェイスに `EnqueueOutbox(tx, event)` を持たせる（実装は各集約 ApplicationService 経由）

### 13.3 M6 ワーカー実装

- `outbox-dispatcher` 常駐ワーカー
- イベント別ハンドラ実装（§6.2 ルーティング表）
- **`image_ingestion_requested.go` ハンドラ**: image-processor 連携（HEIC 変換 / EXIF 除去 / variant 生成）<!-- 付録C P0-27 -->
- 失敗時のリトライ・`failed` 遷移
- failed Outbox は `outbox_failed_retry` 自動 reconciler が拾う <!-- 付録C P0-29, P0-30 -->

### 13.4 集約からの Outbox 連携 セクション

各集約のドメイン設計内に Outbox 連携記述があり、本書はそれらの正本となる。集約からの参照先は本ファイル（`docs/design/cross-cutting/outbox.md`）に揃える。

---

## 14. M1 スパイク検証結果（2026-04-25）

優先順位 7 として `harness/spike/backend/` で本横断設計の最小 PoC を実施し、設計の中核がローカル環境で成立することを確認した。

### 14.1 検証範囲（PoC でカバーした項目）

- **§3 `outbox_events` テーブル**: PoC 用に最小カラム（`id / event_type / aggregate_type / aggregate_id / payload / status / attempts / next_attempt_at / last_error / created_at / processed_at / locked_at`）と CHECK 制約 + `(status, next_attempt_at) WHERE status='pending'` 部分インデックスを `migrations/00003_create_outbox_events.sql` に定義
  - 本実装では §3.1 の正式命名（`retry_count` / `next_retry_at` / `failure_reason` / `processing_started_at` / `failed_at`）に揃えて再整備する
- **§5 状態遷移**: `pending → processing → processed` と `pending → processing → failed` の両経路を sandbox API（`/sandbox/outbox/{enqueue, process-once, retry-failed, list}`）で検証。PoC では指数バックオフを導入せず terminal failed に集約
- **§6.1 ピックの排他制御**: 単一 SQL（CTE 内 `FOR UPDATE SKIP LOCKED LIMIT $1` → 直後に `processing` へ UPDATE）で 30 件 + 2 並列 process-once を実機検証、event_ids の overlap=0、最終 processed=30 を確認
- **§6.2 処理ハンドラ**: `event_type` ごとのルーティングは PoC では mock（`ForceFail` を含む → failed、それ以外 → processed）で代替。ルーティングテーブル自体の妥当性は §6.2 の通り維持
- **§6.3 処理後の状態更新**: `MarkOutboxProcessed`（processed + processed_at）と `MarkOutboxFailed`（failed + last_error）を別関数として分離
- **§4.4 `ImageIngestionRequested`**: PoC で扱える event_type として実機確認（ID 衝突なく claim → processed）
- **§13.3 ワーカー基盤**: `cmd/outbox-worker --once` / `--retry-failed` の CLI を `harness/spike/backend/cmd/outbox-worker/main.go` に追加し、Cloud Run Jobs から起動できる前提の構造を整理。`scripts/outbox-process-once.sh` は Cloud Scheduler 経由の起動を想定したラッパー

### 14.2 残課題（本実装 / Cloud Run 実環境で対応）

- **§5 / §6.3 指数バックオフ**: PoC では未実装。本実装では `attempts` に応じて pending 戻し or failed を選ぶ
- **§8 保持期間と削除**: `processed=30 日 / processing で 1 時間滞留 → pending 戻し`は PoC 範囲外
- **§9 監視**: `pending` キュー深度 / `failed` 件数 / `processing` 滞留のメトリクスは M6 で構築
- **多重起動防止（U11 / reconcile-scripts.md §3.7.6）**: PoC でプロセス並列の SKIP LOCKED は確認済みだが、Cloud Scheduler が 1 cron 起動で複数 Job を発火するケースに対する DB advisory lock or Job スケジューラ側排他制御は本実装で評価
- **同一 TX INSERT の本実装側責務**: PoC では sandbox API の単独 INSERT で代替。本実装の各集約 ApplicationService が `Repository.EnqueueOutbox(tx, event)` を呼ぶ形で担保（§13.2）

### 14.3 PoC 命名と本実装命名の対応表

| PoC（spike/backend） | 本実装（cross-cutting/outbox.md §3.1） | 備考 |
|--------------------|-----------------------------------|------|
| `attempts` | `retry_count` | 本実装で揃える |
| `next_attempt_at` | `next_retry_at` | 本実装で揃える |
| `last_error` | `failure_reason` | 本実装で揃える |
| `locked_at` | `processing_started_at` | 本実装で揃える |
| （未使用） | `failed_at` | 本実装で追加 |
| `payload` | `payload_json` | 本実装で揃える |
