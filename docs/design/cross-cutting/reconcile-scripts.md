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

### 2.1 責務分割 <!-- 付録C P0-30, P0-31 -->

- **巨大な1本にしない**。責務ごとに独立したスクリプトに分ける
- スクリプト間の相互依存を最小化
- 単独で `--dry-run` 実行できる

#### 自動 reconciler と 手動 scripts/ops/reconcile の 2 系統

付録C P0-30 / P0-31 に従い、Reconcile を 2 系統に分ける（v4 §6.16）:

| 系統 | 起動方法 | 対象 | 責務 |
|------|---------|------|------|
| **自動 reconciler**（付録C P0-30） | cron 起動（Cloud Run Jobs スケジューラ等、U11） | `draft_expired` / `outbox_failed_retry` / `stale_ogp_enqueue` / `delivery_expired_to_permanent` | 期限切れ・失敗 retry・OGP stale 検出など、**ルーチン的な定期メンテナンス** |
| **手動 `scripts/ops/reconcile/`**（付録C P0-31） | 運営判断で実行（cmd/ops 経由、ADR-0002） | `image_references.sh` / `photobook_image_integrity.sh` / `cdn_cache_force_purge.sh` / `outbox_failed.sh` / `ogp_stale.sh` / `draft_expired.sh` | アラート対応・整合性監査・CDN 強制パージなど、**判断を伴う対応** |

**重要**:
- **既存の `*.sh` スクリプトはすべて手動枠**（運営が判断して実行）
- 自動 reconciler は **別レイヤー**（cron + バイナリ常駐 / Cloud Run Jobs）として実装し、本ファイル §3 の自動 reconciler 節で責務を定義
- 同名（`draft_expired` 等）でも、自動 reconciler は無人実行向けの「期限切れ検出 → Outbox enqueue or 連鎖削除」、手動スクリプトは「運営による状態確認 + 必要時の手動 GC」と責務が異なる
- 全手動スクリプトは `--dry-run` をデフォルトとする（ADR-0002）

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

> **§3.0〜§3.6 は手動 `scripts/ops/reconcile/` 系統**（付録C P0-31、運営判断で実行）。
> **§3.7 は自動 reconciler 系統**（付録C P0-30、cron 起動、無人実行）。

### 3.0 各スクリプトの系統一覧 <!-- 付録C P0-30, P0-31 -->

| スクリプト / reconciler | 系統 | 起動 | 責務 |
|-----------------------|------|------|------|
| `image_references.sh` | 手動 | 運営判断 | 孤児 Image の検出と修復 |
| `outbox_failed.sh` | 手動 | アラート対応 | failed Outbox の手動再投入 |
| `ogp_stale.sh` | 手動 | 運営判断 | stale OGP の手動再生成 |
| `draft_expired.sh` | 手動 | 運営判断 | 期限切れ draft の手動 GC |
| `photobook_image_integrity.sh` | 手動 | 整合性監査 | Photobook と Image の参照整合性検査 |
| `cdn_cache_force_purge.sh` | 手動 | 個別対応 | CDN キャッシュ強制パージ |
| `draft_expired`（auto） | 自動 | cron | 期限切れ draft の Outbox enqueue（連鎖削除を起動） |
| `outbox_failed_retry`（auto） | 自動 | cron | failed Outbox の retry 再投入 |
| `stale_ogp_enqueue`（auto） | 自動 | cron | stale OGP の再生成キューイング |
| `delivery_expired_to_permanent`（auto） | 自動 | cron | failed_retryable + expire_at 到達で failed_permanent 確定 |



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

### 3.7 自動 reconciler（cron 起動、付録C P0-30） <!-- 付録C P0-30 -->

自動 reconciler は **cron で無人起動**される定期メンテナンス処理。実装は専用バイナリ（`cmd/reconciler` 想定）または Cloud Run Jobs スケジューラで構築する（U11、ADR-0001 の M1 スパイク結果に依存）。

#### 3.7.1 `draft_expired`（auto）

- **目的**: `draft_expires_at < now()` の draft Photobook を検出し、削除フローへ載せる
- **動作**: `outbox_events` に `PhotobookSoftDeleted` 相当を enqueue（または直接 Photobook 集約の `softDelete()` を呼ぶ、実装判断）
- **頻度**: 1 時間に 1 回 程度（運用調整）
- **手動版との違い**: 手動 `draft_expired.sh` は運営が件数確認後に GC 実行、自動版は閾値以下の件数を黙々と処理する
- **整合性**: 運営が手動でも自動でも同じ集約メソッドを呼ぶため、結果は一致 (参照: v4 §6.16)

#### 3.7.2 `outbox_failed_retry`（auto）

- **目的**: `outbox_events` の `failed` 件数のうち、`retry_count` がしきい値以下のものを `pending` に戻す
- **動作**: `failed → pending` UPDATE、`next_retry_at = now()` セット、`retry_count` を維持
- **頻度**: 5 分に 1 回 程度
- **手動版との違い**: 手動 `outbox_failed.sh` は運営がアラート受信後に判断して特定イベントを再投入、自動版は無条件再投入（ただし retry_count しきい値で打ち切り） (参照: v4 §6.16)

#### 3.7.3 `stale_ogp_enqueue`（auto）

- **目的**: `photobook_ogp_images.status='stale'` の行を検出し、再生成キューを起動
- **動作**: `outbox_events` に `PhotobookUpdated` 相当（または OGP 専用イベント）を enqueue
- **頻度**: 30 分に 1 回 程度
- **手動版との違い**: 手動 `ogp_stale.sh` は運営が個別に再生成、自動版は stale 検出 → 即時 enqueue (参照: OGP 設計書 §9 / v4 §6.16)

#### 3.7.4 `delivery_expired_to_permanent`（auto）

- **目的**: `manage_url_deliveries.status='failed_retryable'` で `expire_at` 到達したものを `failed_permanent` に確定
- **動作**: status を `failed_permanent` に UPDATE、`failure_reason='expired_during_retry'` を記録（v4 P1-7）
- **頻度**: 1 時間に 1 回 程度
- **手動版**: なし（自動のみ） (参照: ManageUrlDelivery ドメイン §6.2 / 同 §13.3)

#### 3.7.5 自動 reconciler の起動基盤（U11）

- **MVP 基本案**: Cloud Run Jobs + Cloud Scheduler（ADR-0001 の Cloud Run 第一候補方針と整合）
- **代替**: GitHub Actions cron / 専用 worker（VPS 常駐）
- **M1 スパイクで確定**: Cloudflare Pages との Webhook 連携 / Cloud Scheduler の信頼性 / 多重起動防止（distributed lock の必要性）
- 起動結果は `harness/work-logs/` に記録（§6 監査ログと同方針）

#### 3.7.6 自動 reconciler の安全装置

- 各 reconciler に `--max-batch-size`（1 回の実行で処理する最大件数）を持たせる
- 実行間隔より処理時間が長くなる場合の多重起動防止（DB advisory lock or Job スケジューラ側の排他制御）
- 例外発生時は監視アラート発火（§5 アラート連携）

---

## 4. 実行スケジュール（推奨） <!-- 付録C P0-30, P0-31 -->

MVP 運用では以下の頻度で実行する想定。

| スクリプト / reconciler | 系統 | 頻度 | 備考 |
|-----------------------|------|------|------|
| `image_references.sh` | 手動 | 日次（運営判断） | 孤児画像の蓄積を防ぐ。重大時は手動で複数回 |
| `outbox_failed.sh` | 手動 | 手動 | アラートが出たとき。特定イベントを再投入 |
| `ogp_stale.sh` | 手動 | 日次（運営判断） | 更新追従。自動 `stale_ogp_enqueue` が拾わないものを補完 |
| `draft_expired.sh` | 手動 | 日次（運営判断） | ストレージ節約。自動 reconciler が拾わないものを補完 |
| `photobook_image_integrity.sh` | 手動 | 週次 | 整合性監査 |
| `cdn_cache_force_purge.sh` | 手動 | 手動 | 個別対応 |
| `draft_expired`（auto） | 自動 | 1 時間 / 回 | 期限切れ Photobook を Outbox 経由で連鎖削除 |
| `outbox_failed_retry`（auto） | 自動 | 5 分 / 回 | failed → pending 再投入 |
| `stale_ogp_enqueue`（auto） | 自動 | 30 分 / 回 | stale OGP 再生成キューイング |
| `delivery_expired_to_permanent`（auto） | 自動 | 1 時間 / 回 | failed_retryable → failed_permanent 確定（v4 P1-7） |

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

## 9. v4 業務知識・ADR・付録C との対応

| 項目 | 参照先 | 本書項目 |
|------|-------|---------|
| §2.9 Reconcile 用語 | v4 §2.9 | §1, §2 |
| §5.4 運営対応の最低限手段 | v4 §5.4 / ADR-0002 | §2.3, §7 |
| §6.11 Outbox | v4 §6.11 | §3.2, §3.7.2 |
| §6.12 Reconcile で整合性保証 | v4 §6.12 | 全体 |
| §6.14 画像の所有と削除連鎖 | v4 §6.14 | §3.1, §3.5 |
| §6.16 自動 reconciler / 手動 reconcile の分類 | v4 §6.16 | §2.1, §3.7, §4 <!-- 付録C P0-30, P0-31 --> |
| 付録C P0-29 failed Outbox は reconcile 対象 | 付録C | §3.2, §3.7.2 <!-- 付録C P0-29 --> |
| 付録C P0-30 自動 reconciler 4 種 | 付録C | §3.7.1〜§3.7.4 <!-- 付録C P0-30 --> |
| 付録C P0-31 手動 scripts/ops/reconcile 6 種 | 付録C | §3.1〜§3.6 <!-- 付録C P0-31 --> |
| P1-8 CDNキャッシュパージと reconcile | v3→v4 改訂 | §3.6 |
| ADR-0002 cmd/ops + scripts/ops 経由 | ADR-0002 | §2.3, §7 |
| ADR-0001 Cloud Run Jobs + Scheduler（U11） | ADR-0001 | §3.7.5 |

---

## 10. 次工程への引き継ぎ事項

### 10.1 M3 マイグレーション

- 自動 reconciler が触るテーブル（`photobooks`, `outbox_events`, `photobook_ogp_images`, `manage_url_deliveries` 等）はすべて既存集約 / 横断設計で migration 定義済み
- 自動 reconciler 自体は専用テーブルを持たない（実行ログは `harness/work-logs/` のファイル運用、§6）

### 10.2 M6 実装

- 自動 reconciler 4 種（§3.7.1〜§3.7.4）を `cmd/reconciler/` 配下に実装
- 手動 reconcile 6 種（§3.1〜§3.6）を `cmd/ops/reconcile/` 配下に実装、`scripts/ops/reconcile/*.sh` でラップ（ADR-0002）
- 自動 / 手動とも UseCase（Application 層）を共有し、同じドメインメソッドを呼ぶ

### 10.3 起動基盤（U11）

- MVP 基本案: Cloud Run Jobs + Cloud Scheduler
- M1 スパイクで多重起動防止 / 信頼性を検証 (参照: ADR-0001)
- 検証結果次第で GitHub Actions cron / 専用 worker に切替

### 10.4 監視

- §5 アラート連携と統合
- 自動 reconciler の処理件数、失敗件数、実行時間をメトリクス化（M6 で構築）

---

## 11. 未解決事項

### U11: 自動 reconciler の起動基盤

- **MVP 基本案**: Cloud Run Jobs + Cloud Scheduler（ADR-0001 の Cloud Run 第一候補方針と整合）
- **比較対象として残す**: GitHub Actions cron / 専用 worker（VPS 常駐）
- **M1 スパイクおよび ADR-0001 の検証結果により最終決定**
- 多重起動防止（distributed lock）の要否、Cloud Scheduler の信頼性、コストを評価
- 確定後、本ファイル §3.7.5 を更新
