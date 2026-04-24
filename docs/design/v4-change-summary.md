# VRC PhotoBook v4 設計変更サマリとP0/P1チェックリスト

> 上流: [業務知識定義書 v4](../spec/vrc_photobook_business_knowledge_v4.md)
>
> 本書は v3 → v4 修正指示書のすべての P0 / P1 項目が、どのドキュメントで反映されたかを追跡するためのチェックリスト。

---

## 1. v4 主要変更点サマリ

| 変更領域 | v3 | v4 |
|---------|----|----|
| **画像所有モデル** | `reference_count` で共有参照 | **`owner_photobook_id` + `usage_kind`** による所有モデル |
| **Image 状態** | uploading/processing/available/deleted/purged | 左記 + **`failed`** |
| **画像安全検証** | 記載なし | **MIME/マジックナンバー/実デコード、SVG禁止、アニメ拒否、最大長辺8192px/40MP、ボム対策** |
| **EXIF / XMP / IPTC** | EXIFのGPS等のみ除去 | **公開画像から原則全除去**。Author/Artist も Photobook メタ情報として明示入力 |
| **正規化形式** | 曖昧 | **JPG または WebP**。PNGは入力のみ |
| **作成途中の保持** | ブラウザローカル | **server draft + `draft_edit_token`**（7日延長） |
| **OGP管理** | 暗黙 | **`photobook_ogp_images` で独立管理**（5状態） |
| **Cover** | 別テーブル `photobook_covers` | **`photobooks` にインライン化** |
| **Creator Avatar** | MVP対応 | **Phase 2へ延期** |
| **楽観ロック** | なし | **`version` / `lock_version`** |
| **通報カテゴリ** | 5種 | **6種（`minor_safety_concern` 追加）** |
| **通報→Photobook参照** | FK方針未定 | **FKなし + snapshot保持** |
| **運営トランザクション** | Photobook + ModerationAction | **+ Report + Outbox を同一TX** |
| **集約間イベント** | なし | **Transactional Outbox を MVP 採用** |
| **整合性保証** | なし | **Reconcile スクリプト群をMVPから** |
| **ManageUrlDelivery トークンver** | `manage_url_token_version` | **`manage_url_token_version_at_send`**（意味明示化） |
| **管理URL漏洩対策** | 秘匿方針のみ | **`Referrer-Policy: no-referrer` 必須、外部リソース禁止、破壊的操作の二重確認** |
| **Slug 復元ルール** | 不明瞭 | **deleted内は維持・再利用不可、purge後は解放・restore不可** |
| **SEO / robots** | 記載なし | **MVPは全noindex、/manage/ /draft/ Disallow** |
| **IPハッシュソルト** | UsageLimit独自 | **UsageLimit と Report で共有、version管理** |

---

## 2. 新規ドキュメント

v4 で新規に追加したドキュメント：

| ドキュメント | パス |
|------------|-----|
| 業務知識定義書 v4 | [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) |
| Outbox 設計書 | [`docs/design/cross-cutting/outbox.md`](./cross-cutting/outbox.md) |
| OGP 生成設計書 | [`docs/design/cross-cutting/ogp-generation.md`](./cross-cutting/ogp-generation.md) |
| Reconcile スクリプト設計書 | [`docs/design/cross-cutting/reconcile-scripts.md`](./cross-cutting/reconcile-scripts.md) |
| 本書（v4変更サマリ） | [`docs/design/v4-change-summary.md`](./v4-change-summary.md) |

---

## 3. 更新ドキュメント

v3 から v4 に更新した設計書：

| 集約 | ドメイン設計 | データモデル設計 |
|------|------------|----------------|
| Photobook | [ドメイン設計](./aggregates/photobook/ドメイン設計.md) | [データモデル設計](./aggregates/photobook/データモデル設計.md) |
| Image | [ドメイン設計](./aggregates/image/ドメイン設計.md) | [データモデル設計](./aggregates/image/データモデル設計.md) |
| Report | [ドメイン設計](./aggregates/report/ドメイン設計.md) | [データモデル設計](./aggregates/report/データモデル設計.md) |
| Moderation | [ドメイン設計](./aggregates/moderation/ドメイン設計.md) | [データモデル設計](./aggregates/moderation/データモデル設計.md) |
| ManageUrlDelivery | [ドメイン設計](./aggregates/manage-url-delivery/ドメイン設計.md) | [データモデル設計](./aggregates/manage-url-delivery/データモデル設計.md) |
| UsageLimit | [ドメイン設計](./aggregates/usage-limit/ドメイン設計.md) | [データモデル設計](./aggregates/usage-limit/データモデル設計.md) |

各ドキュメント末尾に「v3からの主な変更点」セクションを設けている。

---

## 4. P0 必須修正 チェックリスト

すべての P0 項目は実装前に反映済み。

| # | 項目 | 主な反映先 | 状態 |
|---|-----|----------|------|
| P0-1 | `Image.reference_count` を廃止し `owner_photobook_id` 方式へ変更 | Image集約（ドメイン§3, §4 / データ§3）、業務知識 §3.10, §6.14 | ✅ |
| P0-2 | `ImageStatus` に `failed` を追加 | Image集約（ドメイン§4, §6 / データ§3 CHECK） | ✅ |
| P0-3 | 画像処理失敗時の遷移、`failed_at`, `failure_reason` を定義 | Image集約（ドメイン§3, §6 / データ§3） | ✅ |
| P0-4 | OGP生成状態を `photobook_ogp_images` 独立テーブルで管理 | [OGP設計書](./cross-cutting/ogp-generation.md) | ✅ |
| P0-5 | ModerationActionExecutor が `sourceReportId` 付きで Report 状態を同一TXで更新 | Moderation集約（ドメイン§6.1 ModerationActionExecutor, §8 フロー, §9 状態更新ルール）、Report集約（ドメイン §6 操作） | ✅ |
| P0-6 | `ManageUrlDelivery.manage_url_token_version` を `manage_url_token_version_at_send` にリネーム | ManageUrlDelivery集約（ドメイン§3 / データ§3） | ✅ |
| P0-7 | 管理URLページに `Referrer-Policy: no-referrer` 必須化 | 業務知識 §3.4, §6.13 / Photobook集約（ドメイン §13 管理URL漏洩対策） | ✅ |
| P0-8 | 管理URL経由の破壊的操作に二重確認 | 業務知識 §3.4 / Photobook集約（ドメイン §13） | ✅ |
| P0-9 | server draft + `draft_edit_token` 方式を採用 | 業務知識 §2.3, §3.1 / Photobook集約（ドメイン §3, §8, §10 / データ §3） | ✅ |
| P0-10 | `ReportReason` に `minor_safety_concern` を追加 | 業務知識 §3.6 / Report集約（ドメイン §4 / データ §3 CHECK） | ✅ |
| P0-11 | Report は FKなし + snapshot 保持 | Report集約（ドメイン §3 / データ §3） | ✅ |
| P0-12 | 画像アップロードの MIME/マジックナンバー/デコード/最大ピクセル/アニメーション拒否ルール | 業務知識 §3.7, §3.10 / Image集約（ドメイン §7, §11 / データ §3） | ✅ |
| P0-13 | `outbox_events` を MVP から追加 | [Outbox設計書](./cross-cutting/outbox.md) / 各集約ドメイン設計のイベント発火記述 | ✅ |
| P0-14 | EXIF / XMP / IPTC は公開画像から原則除去。Author/Artist は Photobook メタ情報として明示入力 | 業務知識 §3.10 / Image集約（ドメイン §12） | ✅ |
| P0-15 | outbox イベント種別を業務知識レベルで列挙 | 業務知識 §2.8 / [Outbox設計書 §4](./cross-cutting/outbox.md) | ✅ |

---

## 5. P1 強く推奨修正 チェックリスト

すべて MVP 設計に反映済み。

| # | 項目 | 主な反映先 | 状態 |
|---|-----|----------|------|
| P1-1 | Cover を MVP では `photobooks` にインライン化 | 業務知識 §2.1 / Photobook集約（データ §3, `photobook_covers` 削除） | ✅ |
| P1-2 | `creator_avatar_image` は MVP から除外 | 業務知識 §2.1, §5.2 / Photobook集約（ドメイン §4 CreatorInfo / データ §3） | ✅ |
| P1-3 | `normalizedFormat` の PNG 方針を明記 | 業務知識 §3.10 / Image集約（ドメイン §4） | ✅ |
| P1-4 | UsageLimit / Report の IP ハッシュソルト共有方針 | UsageLimit集約（ドメイン §11）、Report集約（データ §3） | ✅ |
| P1-5 | noindex / SEO 方針。MVPは全noindex | 業務知識 §7.6 | ✅ |
| P1-6 | Photobook に `version` / `lock_version` を追加 | Photobook集約（ドメイン §4 / データ §3） | ✅ |
| P1-7 | ManageUrlDelivery の `failed_retryable` が `expire_at` 到達時の扱い | ManageUrlDelivery集約（ドメイン §5 不変条件 / データ §5） | ✅ |
| P1-8 | CDNキャッシュパージと失敗時 reconcile を定義 | [Reconcile設計書 §3.6 `cdn_cache_force_purge.sh`](./cross-cutting/reconcile-scripts.md) | ✅ |
| P1-9 | Referrer-Policy を通常ページと管理URLページで使い分け | 業務知識 §3.3, §3.4 / Photobook集約 §13 | ✅ |
| P1-10 | メール送信プロバイダ選定時に管理URL本文ログ保持の有無を確認 | ManageUrlDelivery集約（ドメイン §11） | ✅ |
| P1-11 | Slug 復元ルールを明文化 | 業務知識 §3.4 / Photobook集約（ドメイン §12） | ✅ |

---

## 6. 維持する設計判断（変更しない）

v3 から引き継ぐ設計判断：

- Photobook 中心の集約設計
- Image / Report / Moderation / UsageLimit / ManageUrlDelivery の分離
- Page 概念（MVPでは 1 Page = 1 Photo を基本）
- 管理URL方式（ログイン不要）
- SNS化しない
- X共有前提
- スマホファースト
- Webフォトブック体験重視

---

## 7. 新しく導入した横断コンポーネント

| コンポーネント | 位置 | 責務 |
|--------------|-----|-----|
| Transactional Outbox | [`cross-cutting/outbox.md`](./cross-cutting/outbox.md) | 集約間イベントの配送保証 |
| OGP 生成管理 | [`cross-cutting/ogp-generation.md`](./cross-cutting/ogp-generation.md) | OGP画像の状態管理と再生成 |
| Reconcile スクリプト群 | [`cross-cutting/reconcile-scripts.md`](./cross-cutting/reconcile-scripts.md) | 画像参照/Outbox/OGP/Draft/CDN の整合性維持 |

---

## 8. テスト観点（各集約への追加要件）

v4 指示書 §19 で指定された各集約のテスト観点は、各集約のドメイン設計・データモデル設計に記述している。

| 集約 | 主なテスト観点 |
|------|-------------|
| Photobook | draft作成、draft_edit_token検証、publish時のtoken発行、rights_agreedなし公開不可、visibilityごとの閲覧可否、状態遷移、lock_version不一致時の更新拒否 |
| Image | 対応形式のみavailable、SVG拒否、壊れた画像はfailed、EXIF除去後のみavailable、display/thumbnail揃わないとavailable不可、owner_photobook_idで削除対象引ける |
| Report | minor_safety_concern受付可能、snapshot保持、FKなしでもID保持、status遷移 |
| Moderation | hide時にPhotobook/ModerationAction/Report/Outboxが同一TXで更新、soft_delete同様、unhide/restore時にReport自動変更されない |
| Outbox | 状態変更と同一TXでINSERT、retry、retry上限後failed、reconcileで再投入 |
| ManageUrlDelivery | `manage_url_token_version_at_send` が送信時snapshot、メアド原文が保持期間後NULL化、failed_retryable が expire_at到達時の扱い |

---

## 9. 実装前の残タスク（v4 外）

v4 設計確定後に進めるべき項目：

- [ ] 技術スタック選定（ADR-0001）
- [ ] DB選定（PostgreSQL想定だが未確定）
- [ ] ストレージ選定（S3 / R2 等）
- [ ] メール送信プロバイダ選定（P1-10 のチェック観点で確認）
- [ ] CDN選定（Cloudflare等）
- [ ] bot検証プロバイダ選定（Turnstile想定）
- [ ] OGP生成ライブラリ選定（Satori / Canvas / Skia 等）
- [ ] 保持期間の最終確定（draftの7日、論理削除の30日、OGP failedの24時間等）
- [ ] ハッシュソルト管理方針（Secret Manager/環境変数）
- [ ] CI/CDパイプライン構築

---

## 10. 関連ドキュメント索引

- [業務知識定義書 v3](../spec/vrc_photobook_business_knowledge_v3.md)（履歴として保持）
- [業務知識定義書 v4](../spec/vrc_photobook_business_knowledge_v4.md)
- [集約一覧 README](./aggregates/README.md)
- [ディレクトリマッピング](../ディレクトリマッピング.md)

---

## 11. 最終メッセージ

v4 修正を入れた結果、VRC PhotoBook の MVP 設計は**実装に進める水準**になった。

最も重要だった改善：

1. Image を `owner_photobook_id` 方式に変更（v3の曖昧な参照モデルを解消）
2. `draft_edit_token` 方式の採用（ブラウザローカル依存の脱却）
3. OGP 生成状態を独立管理（公開成功と分離）
4. Image の `failed` 状態追加（失敗の明示）
5. Moderation・Report・Outbox の同一トランザクション化
6. 管理URL漏洩対策の明文化
7. Outbox と Reconcile を MVP から導入
