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
| Image | （Photobook経由で間接的に関与、PhotobookPurged ハンドラが Image 削除） | - |

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
    │   └── manage_url_delivery_requested.go
    └── worker/
        └── outbox_worker.go           # ワーカー本体
```

---

## 12. v4 業務知識との対応

| v4節 | 本書項目 |
|------|---------|
| §2.8 Outbox / Reconcile 用語 | §1 目的, §11 |
| §6.11 集約間イベントは Outbox で伝搬 | §2 基本ルール |
| §6.12 整合性は Reconcile で保証 | §10 Reconcile連携 |
| P0-13 outbox_events をMVPから追加 | 全体 |
| P0-15 outbox イベント種別の列挙 | §4 |
| P0-5 Moderation と Report と Outbox の同一トランザクション | §2 基本ルール, §10 |
