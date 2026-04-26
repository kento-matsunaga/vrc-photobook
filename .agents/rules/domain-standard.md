---
description: "ドメインモデル標準パターン — ドメイン層の実装に適用"
globs: ["**/domain/**/*.go", "**/domain/**/*.ts"]
---

# ドメインモデル標準パターン

## 構造

```
{module}/
├── domain/
│   ├── entity/
│   │   ├── {entity_name}.go          # エンティティ定義
│   │   ├── {entity_name}_test.go     # テスト
│   │   └── tests/
│   │       └── {entity_name}_builder.go  # テスト用Builder
│   └── vo/
│       └── {vo_name}/
│           ├── {vo_name}.go          # 値オブジェクト定義
│           ├── {vo_name}_test.go     # テスト
│           └── tests/
│               └── builder.go        # テスト用Builder
├── infrastructure/                    # データアクセス層
│   ├── repository/rdb/               # 永続化
│   │   ├── marshaller/               # ドメイン ↔ DB変換
│   │   ├── tests/                    # テスト用Builder
│   │   └── mock/                     # モック
│   └── query/rdb/                    # 参照系（CQRS Query側）
└── internal/                          # アプリケーション層
    ├── usecase/                       # コマンド & クエリ
    └── controller/                    # APIハンドラー
```

## エンティティルール

1. **コンストラクタで不変条件を保証** — `New*()` はバリデーション済みのエンティティを返す
2. **状態変更はメソッド経由のみ** — フィールド直接変更禁止
3. **ドメインイベントで副作用を伝播** — 直接的な外部呼び出し禁止
4. **値オブジェクトで型安全性を確保** — プリミティブ型の直接使用を最小化

## 値オブジェクトルール

1. **不変** — 生成後の変更不可
2. **等価性** — 値による比較（`Equal()` メソッド）
3. **自己完結** — バリデーションロジックをVO内に封じ込める

## 集約子テーブルと親 version OCC ルール（v4 / 2026-04-27 Audit）

集約子テーブル（例: `photobook_pages` / `photobook_photos` / `photobook_page_metas` /
`image_variants`）の更新は、必ず**集約ルートの version bump と同一 TX**で行う。

### ルール

1. **子テーブル更新は親 version+1 と同一 TX**
   - 例: `AddPhoto` は `photobooks.version+1` UPDATE と `photobook_photos` INSERT を
     同 TX 内で実行する
   - 同 TX に出来ない外部公開 Repository メソッドは作らない（子テーブル単独 UPDATE
     を直接公開する Repository メソッドは原則禁止）
2. **子テーブル単独 Repository 更新の外部公開禁止**
   - `PhotobookRepository.AddPhoto` のように、Repository が「親 version bump + 子
     INSERT/UPDATE/DELETE」を 1 メソッドにまとめて提供する
   - UseCase はこの集約境界を尊重し、子テーブルだけを直接触る経路を作らない
3. **page/photo/display_order の更新は UseCase 経由のみ**
   - HTTP handler / Frontend が直接 SQL / Repository の子テーブル操作を触らない
   - 全操作 (Add / Remove / Reorder / SetCover / ClearCover / UpsertMeta) は
     UseCase 経由
4. **OCC（楽観ロック）違反時のエラー分類**
   - photobook UPDATE の WHERE `version = $expected AND status = 'draft'` で 0 行
     → `ErrOptimisticLockConflict`
   - 「draft 以外」「version 不一致」を **区別しない**（敵対者が status を観測する
     のを防ぐ、設計上 published 後編集は MVP 不可）

### display_order の連続性ルール

1. **DB は uniqueness のみを担保**
   - `UNIQUE (parent_id, display_order)` で重複だけを禁止
   - **連続性（0,1,2,...）は DB では担保しない**
2. **連続性は domain / UseCase で保証**
   - `AddPage` / `AddPhoto` は新規行の `display_order` を「現在の COUNT(*)」で末尾追加
   - `RemovePage` / `RemovePhoto` 後の詰め直しは UseCase の責務（PR19 段階では
     UseCase 内で詰め直しは未実装、必要時に追加）
3. **DEFERRABLE UNIQUE は MVP では採用しない**
   - PostgreSQL の `INITIALLY DEFERRED` UNIQUE は同 TX 内で一時的な重複を許容できるが、
     MVP では採用しない（複雑化 / debug 困難 / レビュー負荷）
4. **Reorder 実装は「一時退避」または「一括再採番」で UNIQUE 衝突を避ける**
   - 単純な単一行 UPDATE は、新 order が既に他行に取られていれば 23505 で失敗する
   - 二者間 swap や complex reorder は次のいずれかで実装:
     - **一時退避**: 一旦 `display_order >= 1000` の領域に逃がし、順次 UPDATE
     - **一括再採番**: 全行の display_order を 0..N-1 で再計算 + 順次 UPDATE
   - SQL クエリ / Repository / test に **「単純 UPDATE は新 order が空いている前提」**
     を明示するコメントを残す（PR19 `UpdatePhotobookPhotoOrder` クエリと
     `TestReorderPhoto_UniqueViolation` テストで実装済）

### 時刻パラメータ化（now() の扱い）

1. **migration の `DEFAULT now()` は許容**
   - `created_at timestamptz NOT NULL DEFAULT now()` 等の列既定値は OK
2. **クエリ条件の `now()` は原則 `$now` 引数化**
   - `WHERE expires_at > now()` のような条件式での `now()` は、可能な限り
     **Application 層から `$now` を渡す**形にリファクタする
   - 理由: test の Clock 固定 / 監査時刻整合 / 並行 TX での時刻ジッタ防止
3. **既存 `now()` 使用箇所の取り扱い（2026-04-27 Audit 時点）**
   - 2026-04-27 Audit で `upload_verification.sql` の Consume / Count を `$now` 化
   - `auth/session/queries/session.sql`（FindActive / Touch / RevokeAll の `expires_at > now()`）
     と `photobook/queries/photobook.sql`（FindByDraftEditTokenHash の `draft_expires_at > now()`）
     は段階的にリファクタ予定（後続 PR で時間境界 test を追加するタイミングで）
   - 新規実装では **原則 `$now` を渡す方針を厳守**

### 親 version bump SQL のテンプレート

```sql
-- 親集約の楽観ロック + draft state 確認 + version+1
UPDATE photobooks
   SET version = version + 1, updated_at = $now
 WHERE id = $photobook_id
   AND version = $expected_version
   AND status = 'draft';
-- 0 行影響 → ErrOptimisticLockConflict
```

子テーブル INSERT/UPDATE/DELETE は同一 TX 内で続けて実行。失敗時は全体 rollback。

## 禁止事項（追加）

- 集約子テーブルを直接更新する SQL を Repository インタフェースに **public で出す**
  （集約境界の破壊）
- DEFERRABLE UNIQUE / 集約子テーブル単独 trigger
- クエリ内で `now()` を新規追加（`$now` を Application 層から渡す）
- 親 version bump 抜きの子テーブル更新 UseCase

## Why

エージェントがドメイン層の実装で以下の問題を起こしたため:
- エンティティのフィールドを直接変更し、不変条件が破壊された
- プリミティブ型の使用により、異なるIDが混同された（UserID と TenantID の取り違え）
- バリデーションがアプリケーション層に散逸し、ドメインモデルが貧血化した

2026-04-27 Audit で追加:
- 集約子テーブル単独更新 Repository が公開されると、UseCase が親 version bump を
  忘れて整合性が崩れる
- DB の `now()` を直接条件に使うと、Repository test で期限境界を固定できず
  flaky test の温床になる
- DEFERRABLE UNIQUE / trigger は便利に見えるが、MVP の認知負荷を増やす

## 関連

- [`docs/plan/m2-photobook-image-connection-plan.md`](../../docs/plan/m2-photobook-image-connection-plan.md) §6 / §7 (page/photo 操作の OCC)
- [`docs/plan/m2-upload-verification-plan.md`](../../docs/plan/m2-upload-verification-plan.md) §7 (atomic consume の `$now`)
- [`docs/security/public-repo-checklist.md`](../../docs/security/public-repo-checklist.md)
