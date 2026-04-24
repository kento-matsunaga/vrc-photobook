# Reconcile スクリプト設計書

> 上流: [業務知識定義書 v4](../../spec/vrc_photobook_business_knowledge_v4.md) §2.8, §6.12, §5.4
>
> Outbox失敗・状態不整合・孤児データの**検出と修復**を担う運用スクリプト群。MVPから用意する。

---

## 1. 目的

v4 では以下の不整合を許容しない運用とする。

- 画像参照の孤立
- Outbox failed の放置
- OGP stale / failed の放置
- Draft 期限切れの残留
- Photobook ↔ Image の整合性ズレ
- CDN キャッシュの残留

これらを検出・修復するため、責務ごとに分割された Reconcile スクリプトを MVP から提供する。

---

## 2. 設計原則

### 2.1 責務分割

- **巨大な1本にしない**。責務ごとに独立したスクリプトに分ける
- スクリプト間の相互依存を最小化
- 単独で `--dry-run` 実行できる

### 2.2 共通オプション

全スクリプトに以下を用意する。

| オプション | 説明 | 必須 |
|-----------|-----|------|
| `--dry-run` | 検出のみ、修復を実行しない | - |
| `--execute` / `--fix` | 修復を実際に実行する | `--dry-run` と排他 |
| `--limit N` | 処理件数の上限（安全弁） | - |
| `--photobook-id UUID` | 対象を特定のPhotobookに限定 | - |
| `--since DATE` | 対象期間の開始 | - |
| `--verbose` | 詳細ログ | - |

**デフォルトは `--dry-run`**。明示的に `--execute` を渡さない限り変更しない。

### 2.3 配置

```
scripts/ops/reconcile/
├── image_references.sh
├── outbox_failed.sh
├── ogp_stale.sh
├── draft_expired.sh
├── photobook_image_integrity.sh
└── cdn_cache_force_purge.sh
```

MVP では shell スクリプトでラップし、内部でアプリケーションの CLI（`go run ./cmd/reconcile/...` 等）を呼ぶ構造にする。

---

## 3. 各スクリプト

### 3.1 `image_references.sh`

**目的**: `Image.owner_photobook_id` と Photobook の状態を見て、不要画像や孤児画像を検出・修復する。

**検査内容**:

| 検査項目 | 検出条件 | 修復アクション |
|---------|---------|--------------|
| 所有者喪失 | `owner_photobook_id` が存在しないPhotobookを指す | Image を `deleted` に遷移 |
| 所有者が purged | `owner_photobook_id` が `status=purged` | Image を `deleted` に遷移 |
| 所有者が deleted | `owner_photobook_id` が `status=deleted` かつ保持期間経過 | Image を `deleted` に遷移 |
| 未使用の available | `status=available` なのに Photobook の本文から参照されていない | Image を `deleted` に遷移 |
| 長期 failed | `status=failed` かつ `failed_at` から 7日経過 | Image を `deleted` に遷移 |

**実行例**:

```bash
# 検出のみ
./scripts/ops/reconcile/image_references.sh --dry-run

# 修復実行
./scripts/ops/reconcile/image_references.sh --execute --limit 100
```

---

### 3.2 `outbox_failed.sh`

**目的**: `failed` 状態または retry 上限に近い `outbox_events` を確認し、再実行または調査対象にする。

**処理**:

| 機能 | 説明 |
|------|-----|
| list | `failed` イベント一覧、`retry_count` 降順 |
| list-near-limit | retry_count が上限の 80% 以上のイベント |
| requeue | 指定イベントID / aggregate ID の再キュー投入（`status=pending`, `retry_count` リセット） |
| discard | 指定イベントIDを放棄（`processed` として扱い削除） |

**実行例**:

```bash
# failed一覧
./scripts/ops/reconcile/outbox_failed.sh --dry-run

# 特定イベントの再投入
./scripts/ops/reconcile/outbox_failed.sh --execute --event-id <UUID>

# 特定Photobookに関連する全failedイベントを再投入
./scripts/ops/reconcile/outbox_failed.sh --execute --photobook-id <UUID>
```

**注意**: 再投入は副作用を再度実行するため、**冪等性が保証されているイベント**でのみ安全。

---

### 3.3 `ogp_stale.sh`

**目的**: Photobook の更新状態と OGP 生成状態を比較し、古い OGP や失敗 OGP を検出する。

**検査内容**:

| 検査項目 | 検出条件 | 修復アクション |
|---------|---------|--------------|
| stale 化漏れ | `photobook.updated_at > photobook_ogp_images.generated_at` かつ `status != stale` | `status=stale` に遷移、再生成ジョブ投入 |
| failed の再試行 | `status=failed` かつ `failed_at` から 24時間経過 | `status=pending` に戻し、再生成ジョブ投入 |
| 画像喪失 | `status=generated` だが `image_id IS NULL` または指す Image が存在しない | `status=failed` に遷移、再生成ジョブ投入 |
| Image非有効 | `image_id` が指す Image の `status != available` | `status=failed`、再生成ジョブ投入 |

**実行例**:

```bash
./scripts/ops/reconcile/ogp_stale.sh --dry-run
./scripts/ops/reconcile/ogp_stale.sh --execute --since 2026-04-01
```

---

### 3.4 `draft_expired.sh`

**目的**: 期限切れ draft Photobook を削除し、紐づく Image も削除対象にする。

**検査内容**:

| 検査項目 | 検出条件 | 修復アクション |
|---------|---------|--------------|
| 期限切れ draft | `status='draft' AND draft_expires_at < now()` | Photobook を削除、`owner_photobook_id` が一致する Image を `deleted` に |

**実行例**:

```bash
./scripts/ops/reconcile/draft_expired.sh --dry-run
./scripts/ops/reconcile/draft_expired.sh --execute --limit 50
```

**業務ルール**: draft は公開物ではないため、保持期間を設けず即時削除してよい（業務知識 v4 §3.1, §5.5）。

---

### 3.5 `photobook_image_integrity.sh`

**目的**: Photobook 本文の参照 Image と、`Image.owner_photobook_id` の整合性を確認する。

**検査内容**:

| 検査項目 | 検出条件 | 修復アクション |
|---------|---------|--------------|
| 参照先不在 | `photobook_photos.image_id` が存在しない Image を指す | 運営判断（通常は起きてはいけない） |
| 所有者ズレ | Photobook が参照する Image の `owner_photobook_id` が別 Photobook | ログ出力、運営判断 |
| OGP所有者ズレ | OGP画像の `owner_photobook_id` が対象Photobookと不一致 | `owner_photobook_id` を修正 |
| Cover画像不在 | `photobooks.cover_image_id` が存在しない | ログ出力、作成者に通知 |
| 非available画像の公開利用 | 公開Photobookが `status != available` の Image を使用 | Photobook を一時非表示（`hide`）、運営通知 |

**実行例**:

```bash
./scripts/ops/reconcile/photobook_image_integrity.sh --dry-run --verbose
```

**注意**: この整合性チェックは読み取りが中心。修復は慎重に運営判断を通す。

---

### 3.6 `cdn_cache_force_purge.sh`

**目的**: CDN キャッシュを手動で強制無効化する。

**対象**:

| 対象 | キャッシュキー |
|------|------------|
| 公開ページHTML | `/{public_url_slug}` |
| OGP画像 | `/og/{photobook_id}.png?v={version}` |
| display画像 | `/img/display/{image_id}/*` |
| thumbnail画像 | `/img/thumb/{image_id}/*` |

**用途**:

- 通報対応後の一時非表示
- 削除対応後の残留キャッシュ除去
- OGP 再生成後の反映
- 画像差し替え後（将来的な機能）

**実行例**:

```bash
# 特定Photobookの全キャッシュパージ
./scripts/ops/reconcile/cdn_cache_force_purge.sh --execute --photobook-id <UUID>

# OGPのみパージ
./scripts/ops/reconcile/cdn_cache_force_purge.sh --execute --photobook-id <UUID> --target ogp
```

**注意**: CDN プロバイダ依存。実装時はプロバイダ（Cloudflare等）のAPIアダプタを通す。

---

## 4. 実行スケジュール（推奨）

MVP 運用では以下の頻度で実行する想定。

| スクリプト | 頻度 | 備考 |
|-----------|------|------|
| `image_references.sh` | 日次 | 孤児画像の蓄積を防ぐ |
| `outbox_failed.sh` | 手動 | アラートが出たとき |
| `ogp_stale.sh` | 日次 | 更新追従 |
| `draft_expired.sh` | 日次 | ストレージ節約 |
| `photobook_image_integrity.sh` | 週次 | 整合性監査 |
| `cdn_cache_force_purge.sh` | 手動 | 個別対応 |

---

## 5. アラート連携

以下の状況で運営にアラートを出す想定（MVPは手動監視でも可）。

- `outbox_failed.sh` 実行で failed が閾値を超える
- `ogp_stale.sh` で failed が継続的に増える
- `photobook_image_integrity.sh` で不整合が検出される

---

## 6. 監査ログ

Reconcile 実行結果は `harness/work-logs/` に記録する想定。

```
harness/work-logs/
└── reconcile/
    ├── 2026-04-25_image_references.log
    ├── 2026-04-25_outbox_failed.log
    └── ...
```

実行者・検出件数・修復件数・失敗件数を記録する。

---

## 7. 安全装置

- **`--dry-run` がデフォルト**: 明示的な `--execute` がない限り変更しない
- **`--limit` の推奨**: `--execute` 時は必ず `--limit` を付ける運用にする（大量の不整合が見つかったときの事故防止）
- **変更は1トランザクション1スクリプト**: 異なる Reconcile を同時実行しない
- **本番 DB への直接接続ではなく、運営専用アプリケーションユーザー経由**

---

## 8. インフラ層のマッピング

```
backend/cmd/reconcile/
├── image_references/
│   └── main.go
├── outbox_failed/
│   └── main.go
├── ogp_stale/
│   └── main.go
├── draft_expired/
│   └── main.go
├── photobook_image_integrity/
│   └── main.go
└── cdn_cache_force_purge/
    └── main.go

backend/internal/reconcile/
├── image_reference_checker.go
├── outbox_failed_handler.go
├── ogp_stale_checker.go
├── draft_expired_collector.go
├── photobook_integrity_checker.go
└── cdn_purger.go

scripts/ops/reconcile/
├── image_references.sh          # go run ./cmd/reconcile/image_references をラップ
├── outbox_failed.sh
├── ogp_stale.sh
├── draft_expired.sh
├── photobook_image_integrity.sh
└── cdn_cache_force_purge.sh
```

---

## 9. v4 業務知識との対応

| v4節 | 本書項目 |
|------|---------|
| §2.8 Reconcile 用語 | §1, §2 |
| §5.4 運営対応の最低限手段 | §2.3, §7 |
| §6.11 Outbox | §3.2 `outbox_failed.sh` |
| §6.12 Reconcile で整合性保証 | 全体 |
| §6.14 画像の所有と削除連鎖 | §3.1, §3.5 |
| P1-8 CDNキャッシュパージと reconcile | §3.6 |
