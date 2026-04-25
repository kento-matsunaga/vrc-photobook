# OGP 生成設計書

> 上流: [業務知識定義書 v4](../../spec/vrc_photobook_business_knowledge_v4.md) §3.2, §3.8 / [Outbox設計](./outbox.md) / [Photobook集約](../aggregates/photobook/) / [Image集約](../aggregates/image/)
>
> X共有時の OGP 画像生成を、Photobook の主カラムから分離して独立管理する。

---

## 1. 目的

v3 では OGP 画像生成の状態管理が曖昧だった。v4 では以下を実現する。

- **Photobook の公開自体は OGP 生成結果に依存しない**（業務知識 v4 §3.2 維持）
- **OGP 生成の状態（pending / generated / failed / fallback / stale）を独立して追跡**
- **Photobook 更新時に OGP を stale に遷移 → 再生成**
- **OGP 画像の実体は Image 集約で保持し、参照を切り出して管理**

---

## 2. 設計原則

- OGP 画像の**状態**は `photobook_ogp_images` で管理
- OGP 画像の**実体（バイナリ／ストレージキー）**は Image 集約（`images` + `image_variants`）で管理
- OGP 生成は**非同期**（Outbox 経由）
- 生成失敗時は fallback（既定OGP画像）を使用し、公開は成功扱い

---

## 3. `photobook_ogp_images` テーブル

### 3.1 カラム定義

| カラム | 型 | NULL | 既定 | 制約・備考 |
|-------|-----|------|------|----------|
| `id` | `uuid` | NOT NULL | `gen_random_uuid()` | PK |
| `photobook_id` | `uuid` | NOT NULL | - | FK → `photobooks.id` ON DELETE CASCADE |
| `status` | `text` | NOT NULL | `'pending'` | CHECK: `pending / generated / failed / fallback / stale` |
| `image_id` | `uuid` | NULL | - | FK → `images.id` ON DELETE SET NULL。生成前/失敗時は NULL |
| `version` | `int` | NOT NULL | `1` | Photobook更新ごとにインクリメント |
| `generated_at` | `timestamptz` | NULL | - | `status=generated` 遷移時 |
| `failed_at` | `timestamptz` | NULL | - | `status=failed` 遷移時 |
| `failure_reason` | `text` | NULL | - | 失敗理由（簡潔） |
| `created_at` | `timestamptz` | NOT NULL | `now()` | |
| `updated_at` | `timestamptz` | NOT NULL | `now()` | |

### 3.2 索引

| 索引 | カラム | 用途 |
|------|-------|------|
| PK | `id` | |
| UNIQUE | `photobook_id` | 1 Photobook につき 1 OGP 行 |
| INDEX | `status, updated_at` | Reconcile 対象抽出（stale / failed） |

### 3.3 設計判断

- **`photobook_id` UNIQUE**: 1 Photobook につき OGP 行は1つ。バージョン管理は `version` で行う
- **`image_id` は NULL許容**: 生成前・失敗時は画像実体がない。`fallback` 状態でも既定OGPを返すため `image_id` は NULL
- **バージョン管理**: Photobook 更新時に version をインクリメントし、再生成トリガ。並行生成時の整合性確保

---

## 4. 状態遷移

```
              PhotobookPublished
                    ↓
              INSERT（photobook_ogp_images）
              status=pending, version=1
                    ↓
              OGP生成ジョブ実行
                    ↓
         ┌──────────┴──────────┐
         │                     │
         ▼                     ▼
   ┌──────────┐          ┌──────────┐
   │ generated│          │  failed  │ ← failure_reason記録
   └────┬─────┘          └────┬─────┘
        │                     │ 重要な失敗 or リトライ上限
        │                     ▼
        │              ┌──────────┐
        │              │ fallback │ ← 既定OGP使用
        │              └──────────┘
        │
        │ PhotobookUpdated
        ▼
   ┌──────────┐
   │   stale  │ ← version++、再生成トリガ
   └────┬─────┘
        │
        └─→ 再生成で pending → generated へ戻る
```

### 状態の意味

| 状態 | 意味 | OGP画像の配信 |
|------|-----|--------------|
| `pending` | 生成ジョブ投入後、まだ完了していない | 既定OGP（fallback）を返す |
| `generated` | 生成成功、`image_id` が有効 | `image_id` の画像を返す |
| `failed` | 生成失敗（リトライ可能性あり or 上限） | 既定OGP（fallback）を返す |
| `fallback` | 既定OGPを永続的に使用することが決定 | 既定OGP |
| `stale` | Photobook が更新され、OGP が古くなった | 旧 `image_id` を返す（再生成中） |

---

## 5. Photobook との連動（Outbox イベント経由）

### 5.1 `PhotobookPublished` ハンドラ

1. `photobook_ogp_images` に `INSERT (status=pending, version=1)`
2. 実際の生成ジョブを投入（内部キューや Outbox の追加イベント）
3. 生成完了時に `image_id` を設定し `status=generated` に

### 5.2 `PhotobookUpdated` ハンドラ

1. 該当 `photobook_ogp_images` を `status=stale, version++` に更新
2. 再生成ジョブを投入
3. 再生成完了で `status=generated` に戻る、`image_id` は新しい画像を指す
4. 旧 `image_id` の Image は孤児となるため、Image 集約側で削除対象となる

### 5.3 `PhotobookHidden` / `PhotobookSoftDeleted` ハンドラ

- OGP 画像の配信停止のため、CDN キャッシュ無効化
- `photobook_ogp_images.status` 自体は変更しない（復元時に再利用するため）

### 5.4 `PhotobookPurged` ハンドラ

- `photobook_ogp_images` 行は CASCADE で削除
- OGP 画像の物理削除は Image 集約側で実行（`usage_kind='ogp'` の Image）

---

## 6. OGP 画像の実体（Image 集約との連携）

### 6.1 Image 側の扱い

OGP 画像は Image 集約で以下のように保持される。

```
images テーブル:
  owner_photobook_id = 対象Photobook
  usage_kind = 'ogp'
  status = 'available'
  （派生画像 variant は OGP 専用サイズのみ）
```

### 6.2 画像サイズ

| variant | サイズ | 備考 |
|---------|-------|------|
| OGP | 1200 × 630 px | X / Twitter 標準 |

OGP 画像は `image_variants` に1行のみ持つ（display/thumbnail は不要）。

### 6.3 既定OGP画像

`fallback` / `pending` / `failed` の状態で配信する既定OGPは、アプリケーションのアセットとして静的配信。

- `public/og/default-{type}.png`（タイプ別に用意）
- `public/og/default.png`（共通）

### 6.4 生成方法

具体的な生成技術は ADR で確定。候補：

- サーバーサイドで Canvas / Skia 等で描画
- Satori + Resvg（Next.js なら OG Image API）
- Cloudflare Workers の画像変換

フォトブックタイプごとに以下を描画する目安：

- タイトル
- タイプバッジ（イベント/ワールド 等）
- 代表画像のサムネイル（表紙の `cover_image_id` または先頭ページ）
- 作成者名（`creator_display_name`）
- サービスロゴ

---

## 7. 配信（OGPメタタグ）

### 7.1 メタタグ出力

Photobook 閲覧ページの `<head>` で以下を出力：

```html
<meta property="og:title" content="{title}">
<meta property="og:description" content="{description}">
<meta property="og:image" content="{ogp_image_url}">
<meta property="og:url" content="{public_url}">
<meta property="og:type" content="website">
<meta name="twitter:card" content="summary_large_image">
```

### 7.2 `ogp_image_url` の解決

```
photobook_ogp_images.status:
  generated → images 経由で配信URL（CDN URL）
  pending / failed / fallback / stale → 既定OGP URL
```

実装上は閲覧ハンドラでこのロジックを通す。

### 7.3 キャッシュ戦略

- OGP画像URLに version クエリを付与（`?v={photobook_ogp_images.version}`）
- Photobook 更新 → version++ → 新URL → X 側で再取得

---

## 8. 失敗時の取扱い（v4 P0-4）

業務知識 v4 §3.2 に従い、**OGP 生成失敗時もフォトブック公開自体は成功**。

### 8.1 失敗ケース

- 画像合成ライブラリのクラッシュ
- 外部API（Satori等）のタイムアウト
- 代表画像のフォーマット問題

### 8.2 失敗時のフロー

1. OGP生成ジョブが失敗
2. `photobook_ogp_images.status = failed`, `failure_reason` を記録
3. 既定OGPが配信される（fallback）
4. 作成者には「共有画像の生成に失敗しましたが、公開は完了しています」と伝える（UI側）
5. Reconcile スクリプト（`ogp_stale.sh`）で定期的に failed を抽出し、再生成または fallback 確定

### 8.3 `failed` → `fallback` への決定

Reconcile 実行時、以下のいずれかで `fallback` に確定：

- retry_count が一定数を超えた
- 運営が手動で fallback に決定
- 特定の failure_reason（画像形式非対応等）

---

## 9. Reconcile 連携 <!-- 付録C P0-30 -->

OGP の Reconcile は **2 系統**で運用する（v4 §6.16、Reconcile 設計書 §2.1）。

### 9.1 自動 reconciler `stale_ogp_enqueue`（付録C P0-30）

cron 起動の自動 reconciler。`status='stale'` の `photobook_ogp_images` を検出し、`outbox_events` に `PhotobookUpdated` 相当（または OGP 専用イベント）を enqueue して再生成キューを起動する。

- 頻度: 30 分に 1 回 程度（Reconcile 設計書 §3.7.3）
- 起動基盤: U11（Cloud Run Jobs + Cloud Scheduler が MVP 基本案）
- 詳細は Reconcile 設計書 §3.7.3 を参照

### 9.2 手動 `ogp_stale.sh`（付録C P0-31）

運営判断による手動再生成。自動 reconciler が拾わないケース（明示的な再生成、運営による品質確認後の再投入等）に対応する。

検査・修復対象（手動・自動共通）:

- `photobook.updated_at > photobook_ogp_images.generated_at` で stale 化漏れ
- `status=failed` かつ retry 対象
- `status=generated` だが `image_id` が NULL または存在しない
- `image_id` が指す Image の `status != available`

---

## 10. 保持期間

| 対象 | 保持期間 | アクション |
|------|---------|----------|
| `photobook_ogp_images` 行 | Photobook 物理削除まで | CASCADE で削除 |
| `status=failed` で `generated_at` が NULL かつ 30日経過 | - | `fallback` に確定 |
| 古い version の OGP画像（Image側） | 新版 generated で孤児 | Image 集約の孤児GC で削除 |

---

## 11. インフラ層のマッピング

```
backend/ogp/
├── domain/
│   ├── entity/
│   │   └── photobook_ogp_image.go
│   └── vo/
│       ├── ogp_status/
│       └── ogp_version/
├── infrastructure/
│   ├── repository/rdb/
│   │   └── ogp_repository.go
│   └── renderer/
│       ├── ogp_renderer.go
│       └── satori_renderer.go        # 実装例
└── internal/
    ├── handlers/
    │   ├── on_photobook_published.go  # Outbox ハンドラ
    │   ├── on_photobook_updated.go
    │   └── on_photobook_purged.go
    └── usecase/
        └── resolve_ogp_url.go          # 閲覧時のURL解決
```

---

## 12. v4 業務知識・ADR・付録C との対応

| 項目 | 参照先 | 本書項目 |
|------|-------|---------|
| §3.2 OGP生成失敗でも公開は成功 | v4 §3.2 | §8 |
| §3.8 X共有支援（OGPプレビュー） | v4 §3.8 | §7 |
| §6.17 OGP の独立管理 | v4 §6.17 | 全体（§3 / §6 / §9） |
| §6.16 自動 / 手動 reconciler 分類 | v4 §6.16 | §9.1 / §9.2 |
| ADR-0005 storage_key 命名規則 | ADR-0005 | §6 Image 集約との連携 |
| Image 集約 `usage_kind = 'ogp'` | Image §3, §10.2 | §6 |
| P0-4 OGP状態を独立テーブル管理 | v3→v4 改訂 | §3 |
| 付録B「OGP生成管理：photobook_ogp_images で独立管理」 | v3→v4 改訂 | 全体 |
| 付録C P0-30 `stale_ogp_enqueue` 自動 reconciler | 付録C | §9.1 <!-- 付録C P0-30 --> |
| 付録C P0-31 `ogp_stale.sh` 手動 reconcile | 付録C | §9.2 <!-- 付録C P0-31 --> |
| Image 集約 §7 責務分離（`failure_reason` vs `ogp_failure_reason`） | Image §7.2 | §3 / §8 |

---

## 13. 次工程への引き継ぎ事項

### 13.1 M3 マイグレーション

- `photobook_ogp_images` テーブル migration（既存 §3 定義）
- `image_id` への FK は `images.id` ON DELETE SET NULL（生成失敗時の image_id 不整合に耐えるため）

### 13.2 M6 ワーカー実装

- OGP 生成ワーカー: `outbox_events` の `PhotobookPublished` / `PhotobookUpdated` ハンドラから起動
- 自動 reconciler `stale_ogp_enqueue`: cron 起動（U11）、stale 検出 → Outbox enqueue
- 失敗時: `status='failed'` 記録、30 日経過で `fallback` 確定（§10）

### 13.3 Image 集約との責務境界（既存設計、コミット済み）

- 実ファイルは Image 集約（`usage_kind='ogp'`、`storage_key` は ADR-0005 命名規則）
- 状態管理は本集約（pending/generated/failed/fallback/stale）
- 失敗理由カラムは別語彙（Image: `failure_reason` / OGP: `ogp_failure_reason`、Image データモデル §7.2 参照）

### 13.4 Photobook 集約からの参照（既存設計、コミット済み）

- Photobook publish / update 時の Outbox イベントが OGP 再生成のトリガ
- Photobook 集約は OGP 状態を直接持たず、本集約の存在を意識するのみ（v4 §6.17）
