# M2 Moderation / Ops（PR34）計画書

> 上流: [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md) PR34 章
> 関連: [`docs/design/aggregates/moderation/`](../design/aggregates/moderation/)（v4 ドメイン設計 + データモデル設計）/ [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §5.4 / §6.19 / §7.3 / §7.4 / [ADR-0002](../adr/0002-ops-execution-model.md)

---

## 0. このドキュメントの位置付け

- 本書は **PR34（実装PR）の計画書**であり、計画書段階ではコード変更 / migration 適用 / 実リソース操作は **行わない**
- v4 完全設計（Moderation 集約 6 ActionKind + Report 連携）は
  [`docs/design/aggregates/moderation/`](../design/aggregates/moderation/) に既出。
  本計画は **MVP スコープ（hide / unhide / show のみ）**に絞り、残りは PR35（Report） /
  PR34 拡張（後続）に持ち越す
- 本書の判断対象は §13 にまとめる。MVP スコープ外と判断したものは §12 に記録

---

## 1. 現状整理（PR33d 完了時点）

### 1.1 `hidden_by_operator` カラムの現在地

| 場所 | 状態 |
|---|---|
| `photobooks.hidden_by_operator` (DB column) | 存在（migration `00003_create_photobooks.sql`、`bool NOT NULL DEFAULT false`） |
| Photobook domain entity / VO | 値として保持（`HiddenByOperator bool`）|
| 公開 Viewer SQL (`FindActivePublicPhotobookBySlug`) | `WHERE status='published' AND hidden_by_operator = false` で active 公開のみ |
| 公開 Viewer SQL (`FindPhotobookBySlugAny`) | 制限なし lookup（status / hidden / visibility は UseCase 側で判定）|
| 公開 Viewer UseCase (`get_public_photobook.go`) | `ErrPublicGone`（status=published かつ hidden=true）/ `ErrPublicNotFound`（draft / deleted / private 等）を区別なく返す |
| OGP lookup SQL (`GetOgpDeliveryByPhotobookID`) | `p.hidden_by_operator` を `delivery.hidden_by_operator` として返却。`get_public_ogp.go` UseCase が `not_public` 判定に使用 |
| Manage handler (`manage_handler.go`) | レスポンス JSON に `hidden_by_operator` を含めて creator に返す |
| Frontend manage UI (`app/(manage)/manage/[photobookId]/page.tsx`) | **`hiddenByOperator` を表示していない**（lib のマッパーには通っているが UI で未消費）|

### 1.2 hidden 中の挙動（PR33d STOP κ で実機検証済）

- Backend `/api/public/photobooks/<SLUG>` → **HTTP 410 / `{"status":"gone"}`**
- Backend `/api/public/photobooks/<PID>/ogp` → `{"status":"not_public", "image_url_path":"/og/default.png"}`
- Workers `/ogp/<PID>?v=1` → **HTTP 302 / Location: /og/default.png**（fallback 動作）
- `/p/<SLUG>` HTML → HTTP 200 / `<title>VRC PhotoBook</title>` / 既定 OGP / body に `gone` テンプレ表示
- 履歴データ（outbox processed event / photobook_ogp_images generated row / R2 PNG）は **保持**

### 1.3 hidden 中にも管理者・作成者ができるべきこと（MVP の方針案）

| 操作 | 期待される MVP 挙動 |
|---|---|
| 作成者が manage URL で開く | manage data 取得は API 上は可能（仕様未確定）。UI 表示で「運営により非公開中」を出す案あり（§7） |
| 作成者が編集 | MVP 方針: **編集自体は許容**（draft 復帰 / publish-replace は別議論）|
| 作成者が unhide（restore） | **不可**（運営判断、§9） |
| 作成者が削除 | MVP 方針: **不可**（softDelete / purge は PR34 範囲外） |
| 運営が photobook を確認 | **新規実装対象**（cmd/ops の参照系）|
| 運営が hide / unhide | **新規実装対象**（cmd/ops の更新系）|
| 運営が hide 履歴を確認 | **新規実装対象**（audit log の参照系）|

### 1.4 既存の moderation 関連設計資産

- `docs/design/aggregates/moderation/ドメイン設計.md`（v4 完全版、6 ActionKind + Report 連携）
- `docs/design/aggregates/moderation/データモデル設計.md`（`moderation_actions` テーブル、append-only、FK なし）
- `moderation_actions` テーブル / `internal/moderation/` パッケージ自体は **未作成**
- 業務知識 v4 §5.4: 運営は DB 直接操作ではなく cmd/ops 経由（運営 HTTP API なし）
- ADR-0002: ops execution model を CLI に固定

---

## 2. PR34 のゴール（MVP）

PR34 で達成する最低限のゴール:

### G1. 運営が photobook を**安全に確認**できる

- photobook_id または slug から photobook の **公開関連状態**（status / visibility /
  hidden_by_operator / version / published_at / 直近 moderation action 概要）を CLI で取得
- 取得結果に **manage URL / raw token / storage_key 完全値 / R2 credentials / DATABASE_URL は含まない**

### G2. 運営が `hide` できる

- 対象 photobook の `hidden_by_operator=true` 化
- `moderation_actions` への append-only 記録（actor / reason / detail / executed_at）
- 公開 Viewer / OGP は **即座に非表示 / fallback** に倒れる（既存挙動を活用、追加配線不要）
- 同一 TX で更新（v4 P0-19、§4 / §5）

### G3. 運営が `unhide` できる（誤操作・合意解消の戻し）

- 対象 photobook の `hidden_by_operator=false` 化
- `moderation_actions` への append-only 記録
- correlation_id（直前の `hide` action id）任意で紐付け
- 公開 Viewer / OGP は再度 200 / generated に戻る（既存挙動）

### G4. **理由 / 操作ログを残せる**

- すべての hide / unhide は `moderation_actions` に行が増える（不可逆 / immutable）
- actor_label（個人情報なし）/ reason / detail / executed_at は必須
- 監査クエリで「直近どの photobook を誰がいつ hide / unhide したか」「ある photobook の操作履歴」を引ける

### G5. **誤操作時に戻せる**（rollback フロー）

- `--dry-run` がデフォルト、`--execute` で初めて実行（v4 §6.19 / 設計書 §15.4）
- 確認プロンプトで対象 photobook の概要を表示し、operator が yes 入力した場合のみ実行
- CI / non-interactive 用に `--yes` を許可
- 実行後は `unhide` で元に戻すフローが標準。`hide` の DELETE は禁止（append-only）

### G6. **raw token / Cookie / manage URL / Secret を扱わない**

- cmd/ops 内で raw draft / manage token は触らない（取得しない / ログに出さない）
- `manage_url_token` / `manage_url_token_version` は本 MVP では更新しない（reissue は PR34 範囲外、§12）
- DATABASE_URL / R2_* / Turnstile は環境変数経由のみ、CLI 引数や stdout に値を出さない

### G7. **公開導線が動いている前提を壊さない**

- 同一 TX 4 要素のうち、PR34 MVP では **outbox_events INSERT も含める**（PhotobookHidden / PhotobookUnhidden）
- ただし worker 側 handler は **no-op + log のみ**（CDN purge / OGP cache invalidation は MVP 範囲外、PR33e 以降）
- これにより future の Outbox handler 実装時に backfill 不要で繋げられる

---

## 3. PR34 で扱うこと / 扱わないこと

### 3.1 扱うこと（MVP）

- `hide` / `unhide` の UseCase + cmd/ops サブコマンド
- `show` 参照系の cmd/ops サブコマンド（操作を伴わない確認）
- `list-hidden` 参照系（hidden 状態の photobook 一覧）
- `moderation_actions` テーブル新設 + migration（**§4 で案 A / 案 B を提示、ユーザー判断**）
- `internal/moderation/` パッケージ新設（domain / VO / Repository / UseCase）
- `cmd/ops/main.go` 新設（CLI、Photobook 集約と Moderation 集約を同 TX 操作）
- `outbox_events` への `PhotobookHidden` / `PhotobookUnhidden` INSERT（同 TX）。
  ただし outbox-worker 側 handler は no-op + log（CDN / OGP 無効化は後続）
- Tests（UseCase + Repository + cmd/ops dry-run）
- Runbook（`docs/runbook/ops-moderation.md` 新設）

### 3.2 扱わないこと（PR34 範囲外、対応 PR を明示）

| 項目 | 対応 PR / 状態 |
|---|---|
| `soft_delete` / `restore`（論理削除復元） / `purge` | PR34 拡張 or 別 PR（§12） |
| `reissue_manage_url`（管理URL 再発行 + Session revoke + ManageUrlDelivery） | Email Provider 確定後（PR32c 以降） |
| Report 集約（通報受付 + 運営対応自動連動） | PR35 |
| Report state 自動更新（`source_report_id` 連動） | PR35 |
| moderation Web admin UI / dashboard | MVP 範囲外（v4 §6.19） |
| Bulk moderation（複数 photobook 一括） | 任意 |
| Legal takedown フロー固有処理（rights_claim 等） | 必要なら PR35〜37 で扱う |
| 作成者への通知（hide/unhide 時） | Email Provider 未確定 |
| OGP の自動再生成（unhide 時） / R2 stale cleanup | PR33e（任意） |
| moderation actor の認証 / 認可（複数運営対応） | MVP は単一運用者前提（§9） |
| Cloud Run Job 化 / Scheduler 化 | 不要（cmd/ops はローカル運用） |

---

## 4. DB 設計

### 4.1 案 A: `photobooks.hidden_by_operator` のみ使う（最小）

- 既存カラムだけで hide / unhide の **状態反映**は可能
- migration 不要、Cloud SQL 適用 STOP 不要
- **欠点**: 「誰が・なぜ・いつ・どの reason で」hide したかの **audit log が残らない**
- 監査クエリ（直近の hide / unhide 履歴・理由分布）は実装できない
- v4 P0-19（4 要素同一 TX 原則）の趣旨に反する（moderation_actions 追記が無い）

### 4.2 案 B: `moderation_actions` テーブルを追加（推奨）

- `docs/design/aggregates/moderation/データモデル設計.md` §3 の v4 スキーマをそのまま採用
- カラム（v4 既存設計を維持）:
  - `id uuid PK`
  - `kind text NOT NULL CHECK (kind IN ('hide','unhide','soft_delete','restore','purge','reissue_manage_url'))`
    （MVP は hide/unhide のみ INSERT、CHECK は将来用に 6 種類受け入れる）
  - `target_photobook_id uuid NOT NULL`（FK なし、設計書 §4）
  - `source_report_id uuid NULL`（FK なし、PR34 では常に NULL）
  - `actor_label text NOT NULL`
  - `reason text NOT NULL CHECK (reason IN (... 9 種 ...))`
    （MVP は §6.2 の絞り込み版を運用ガイドで案内）
  - `detail text NULL`（≤ 2000 char）
  - `correlation_id uuid NULL`
  - `executed_at timestamptz NOT NULL DEFAULT now()`
- INDEX（設計書 §5）:
  - `(target_photobook_id, executed_at DESC)`
  - `source_report_id WHERE source_report_id IS NOT NULL`
  - `(kind, executed_at DESC)`
  - `(actor_label, executed_at DESC)`
  - `(reason, executed_at DESC)`
  - `correlation_id WHERE correlation_id IS NOT NULL`
- **append-only**: アプリ層で UPDATE / DELETE 文を発行しない。trigger による DB 強制は §6.3 で
  ユーザー判断（trigger 実装は PoC で振る舞いに不安なら見送り、アプリ層責務でも MVP は十分）
- **欠点**: migration 1 本追加、Cloud SQL 適用 STOP 必要。実装 PR でローカル goose up
  + Cloud SQL Auth Proxy 経由の本番 goose up を行う必要

### 4.3 推奨

**案 B 推奨**。理由:

1. 公開導線が動いている本番に運営介入の monitoring / 監査クエリが無いのは脆い
2. 後続 PR35（Report）と接続するときに `source_report_id` のスキーマが揃っていると追加コストが小さい
3. v4 設計書 §3 の v4 スキーマをそのまま使え、設計再検討コスト 0
4. CHECK 制約で 6 種類を最初から受け入れることで、後続の `soft_delete` / `purge` /
   `reissue_manage_url` 追加時に migration 不要

ただし、**最終判断はユーザー**（§13 の判断項目 1）。案 A で MVP を回し、PR34 拡張で B
に移行する選択肢も残す。

### 4.4 既存テーブル変更の有無

- `photobooks` テーブルへのカラム追加は **しない**（`hidden_by_operator` は既存）
- `version` インクリメントの扱いは §5.6 で議論（不要案を推奨）

---

## 5. UseCase 設計

### 5.1 一覧（PR34 MVP）

| UseCase | 操作種別 | 同一 TX に含まれるもの（案 B 採用時）|
|---|---|---|
| `HidePhotobookByOperator` | 状態更新 | photobooks 更新 + moderation_actions INSERT + outbox_events INSERT |
| `UnhidePhotobookByOperator` | 状態更新 | 同上 |
| `GetPhotobookForOps` | 参照 | 単純 SELECT（公開状態 + 直近 moderation action 概要）|
| `ListHiddenPhotobooks` | 参照 | 単純 SELECT（hidden=true 一覧、ページング）|

### 5.2 入力

```text
HidePhotobookByOperator:
  - PhotobookID（または slug → photobook_id 解決）
  - Reason（ActionReason VO）
  - ActorLabel（OperatorLabel VO）
  - Detail（任意、≤ 2000 char）
  - SourceReportID（PR34 MVP では常に nil）

UnhidePhotobookByOperator:
  - PhotobookID
  - Reason（VO）
  - ActorLabel（VO）
  - Detail（任意）
  - CorrelationID（任意、対応する hide action の id）

GetPhotobookForOps:
  - PhotobookID または Slug

ListHiddenPhotobooks:
  - Limit / Offset または cursor
```

### 5.3 振る舞いと事前条件

#### `HidePhotobookByOperator`

- 事前条件: photobook が存在する（id / slug 解決成功）
- **status による分岐**:
  - `published` + `hidden=false` → hide 実行（**主要ケース**）
  - `published` + `hidden=true` → **冪等で nil 返却**（または `ErrAlreadyHidden` を返して CLI 側で skip 表示）
  - `draft` → **非対応**（draft は public 露出していないため hide の意味がない、§12 参照）
  - `deleted` / `purged` → **エラー返却**（既に削除済、hide は無意味）
- 本 PR では **status=published のみ受け付ける**設計を推奨（§13 判断項目 4）

#### `UnhidePhotobookByOperator`

- 事前条件: photobook が存在し、`hidden_by_operator = true`
- `hidden=false` の場合 → **冪等で nil**（または `ErrAlreadyUnhidden`）
- `deleted` / `purged` の場合 → **エラー返却**（restore（論理復元）は別操作）
- correlation_id は任意。指定されていれば moderation_actions に記録、なければ NULL

#### `GetPhotobookForOps`

- 出力（**Secret / token を含めない**）:
  - `photobook_id`
  - `slug`（published なら非空、draft なら空）
  - `title` / `creator_display_name`
  - `type` / `visibility` / `status` / `hidden_by_operator`
  - `version`
  - `published_at` / `created_at` / `updated_at`
  - 直近 moderation action 概要 ≤ 5 件（kind / reason / actor_label / executed_at、detail は **省略**）
- **絶対に出さない**: `draft_edit_token_hash` / `manage_url_token` 任意 / storage_key 完全値 / DATABASE_URL

#### `ListHiddenPhotobooks`

- 出力: hidden=true の photobook について `id / slug / title / hidden_at（直近 hide action の executed_at）` を返す。Limit/Offset で簡易ページング

### 5.4 同一 TX のスコープ（v4 P0-19 準拠、案 B 採用時）

```text
[tx start]
 1. SELECT ... FROM photobooks WHERE id = $1 FOR UPDATE     # 行ロック + 現状取得
 2. （事前条件チェック、必要なら early return + rollback）
 3. UPDATE photobooks SET hidden_by_operator = $2, updated_at = $now WHERE id = $1
 4. INSERT INTO moderation_actions (id, kind, target_photobook_id, ..., executed_at)
 5. INSERT INTO outbox_events (id, aggregate_type='photobook', aggregate_id=$pid,
      event_type='PhotobookHidden' / 'PhotobookUnhidden', payload, ...)
[tx commit]
```

失敗時は全体 rollback。

### 5.5 エラー設計

| Sentinel | 戻し条件 | CLI 側挙動 |
|---|---|---|
| `ErrPhotobookNotFound` | id / slug 不存在 | exit 1 + メッセージ |
| `ErrInvalidStatusForHide` | status が published 以外 | exit 1 + 状態説明（draft/deleted/purged） |
| `ErrAlreadyHidden` / `ErrAlreadyUnhidden` | 冪等（既に目的状態） | exit 0 + 「変更なし」 |
| `ErrOptimisticLockConflict` | 行ロック失敗 / version 不一致（§5.6 採用時） | exit 2 + retry 推奨 |

### 5.6 `version` をインクリメントするか

- v4 設計書 §5.5（Photobook 集約）では `Photobook.version` は draft 編集 / publish 経路の OCC キー
- hide / unhide で `version` を上げると、編集中の作成者の OCC が破壊される可能性
- **推奨: hide / unhide では `version` を上げない**（`updated_at` のみ更新）。
- 代わりに新たな `moderation_action_id` の append-only 行で「いつ・誰が・なぜ」を表現
- 判断は §13 判断項目 5

---

## 6. cmd/ops 方針

### 6.1 配置と起動

```text
backend/cmd/ops/main.go            # ルーター
backend/cmd/ops/photobook_show.go  # 参照系
backend/cmd/ops/photobook_hide.go  # 状態更新
backend/cmd/ops/photobook_unhide.go
backend/cmd/ops/photobook_list_hidden.go
```

- ローカル運用者が **Cloud SQL Auth Proxy 経由 + DATABASE_URL 環境変数**で起動
- Cloud Run Job 化 / Scheduler 化は **しない**（v4 §6.19 / 本計画 §3.2）
- Web admin UI / HTTP endpoint も **作らない**（同上）

### 6.2 サブコマンド一覧

| 命令 | 安全性 | dry-run | 例 |
|---|---|---|---|
| `ops photobook show --id <UUID>` | 参照のみ | 不要 | `--slug` 切替も対応 |
| `ops photobook list-hidden [--limit N] [--offset M]` | 参照のみ | 不要 | hidden=true 一覧 |
| `ops photobook hide --id <UUID> --reason <R> --actor <L> [--detail <D>]` | 状態更新 | **必須**（既定 dry-run、`--execute` で実行） | reason は §6.4 |
| `ops photobook unhide --id <UUID> --reason <R> --actor <L> [--detail <D>] [--correlation <ACTION_ID>]` | 状態更新 | **必須** | 同上 |

### 6.3 安全策

- **`--execute` 明示が必要**（既定 dry-run）。dry-run は対象 photobook の `show` 結果と
  予定 INSERT サマリ（kind / reason / actor）を出力するだけで DB 更新しない
- **対話確認プロンプト**: `--execute` 指定時、`title / status / visibility / hidden_by_operator`
  の現状を表示し `Type 'yes' to proceed:` を出す。yes 以外は exit 0 で abort
- **`--yes`** で CI / non-interactive 用に確認プロンプトを skip 可能（運用 runbook で
  「対話なしで `--yes` を使う場面は通常ない」と注記）
- **raw token / manage URL / storage_key 完全値は表示しない**
- **stdout に出してよい列**（§5.3 の `GetPhotobookForOps` 出力の通り）
- **stderr に出してよい列**: 同上 + エラー詳細（Secret 値を含まない）
- **DATABASE_URL / R2_* は env 経由のみ**、CLI 引数や stdout に値を出さない

### 6.4 Reason の運用（MVP）

v4 設計書 §4.2 で 9 種類定義済。MVP では以下に絞って運用ガイドする:

| MVP で許容 | 用途 |
|---|---|
| `policy_violation_other` | その他規約違反（最も汎用） |
| `report_based_harassment` | 通報経由（PR35 接続前なので任意で受け付け） |
| `report_based_unauthorized_repost` | 同上 |
| `report_based_sensitive_violation` | 同上 |
| `report_based_minor_related` | 未成年関連（最優先対応、v4 §7.4） |
| `rights_claim` | 権利侵害申立て（v4 §7.3） |
| `erroneous_action_correction` | unhide で誤 hide を戻すとき |

- DB CHECK 制約は v4 設計書通り 9 種類すべて受け入れる（将来用）
- アプリ層 / CLI で MVP 許容セットに絞ってもよいが、推奨は **DB CHECK のみ強制 +
  運用 runbook で reason の使い分けガイド**

---

## 7. Public viewer / OGP / R2 への影響

### 7.1 Public viewer

| 状態 | 既存挙動（PR33d 検証済） | PR34 で追加するもの |
|---|---|---|
| status=published + hidden=true | 410 Gone（`/api/public/.../<slug>` JSON `{"status":"gone"}`、Frontend `/p/<slug>` は gone テンプレ表示） | **追加配線なし**（既存 query / UseCase をそのまま活用） |
| status=published + hidden=false（hide 解除直後） | 200 / 通常 viewer | 同上、追加なし |

### 7.2 OGP

| 状態 | 既存挙動 | PR34 で追加するもの |
|---|---|---|
| hidden=true | Backend `/api/public/.../<id>/ogp` → `{"status":"not_public", "image_url_path":"/og/default.png"}`、Workers `/ogp/<id>` → 302 → `/og/default.png` | **追加配線なし** |
| hide 解除（unhide）直後 | Backend lookup → `{"status":"generated", ...}` に戻る、Workers proxy → 200 image/png に戻る | **追加配線なし**（OGP は再生成不要、既存 R2 object をそのまま流用） |

### 7.3 R2 object への影響（推奨方針）

- **hide 時**: R2 object は **削除しない**
- **unhide 時**: 既存の R2 object をそのまま流用（OGP の再生成不要）
- **soft_delete / purge** 時: R2 object 削除は **PR34 範囲外**（PR33e Reconcile + 別 PR）
- 理由:
  1. unhide で即座に元に戻せる（運用上の戻しコストゼロ）
  2. R2 削除は副作用 worker handler が必要で、MVP 範囲を超える
  3. Reconcile（PR33e 任意）で stale cleanup は別途整備

### 7.4 Outbox 連携

- PR34 では `PhotobookHidden` / `PhotobookUnhidden` event を **同一 TX で INSERT** する
  （v4 P0-19 / G7）
- worker handler は **no-op + log のみ**（既存の image / photobook.published handler と
  同じ扱い、副作用は将来 PR で実装）
- これにより、PR33e 以降で CDN purge / OGP cache invalidation を入れるときに backfill 不要

---

## 8. Manage page への影響

### 8.1 現状

- Backend `manage_handler.go` は `hidden_by_operator` を JSON 返却している
- Frontend `lib/managePhotobook.ts` は `hiddenByOperator: boolean` でマッピング済
- ただし `app/(manage)/manage/[photobookId]/page.tsx` の UI では未表示

### 8.2 PR34 での対応案（§13 判断項目 6）

| 案 | 内容 | リスク / メリット |
|---|---|---|
| **a. PR34 で何もしない** | 現状維持。creator は manage page を開いても「運営 hide 状態」を知る手段がない（公開ページが gone になっているのを見て気付く） | リスク: 作成者が混乱して問い合わせが来る可能性。メリット: 範囲縮小、scope creep を避ける |
| **b. PR34 で manage UI に「運営により非公開中」表示を追加** | 既存 `hiddenByOperator` を消費して banner / 注記を表示 | メリット: 作成者が状況を理解できる。リスク: Frontend 変更の Workers redeploy が増える |
| **c. PR34 で manage UI 編集機能を hide 中は無効化** | 編集を抑止 | リスク: 編集自体を禁止すべきかは別議論（運営 hide 中に内容修正したい場合もある）|

**推奨: 案 b（manage UI に banner 表示のみ追加）**。
- creator に状態を伝える効果が大きく、編集を抑制する強制は不要
- Workers redeploy は同時の deploy STOP に組み込めば追加コスト小

ただし最終判断はユーザー（§13 判断項目 6）。

### 8.3 hide 中の manage 認可

- manage Cookie session 自体は revoke しない（hide は publish 取り下げではないため）
- `reissue_manage_url` を伴う hide は PR34 では扱わない（§3.2、Email Provider 確定後）

---

## 9. Security 方針

### 9.1 Secret / token の取り扱い

| 対象 | 方針 |
|---|---|
| `DATABASE_URL` 実値 | env 経由のみ、CLI 引数・stdout・log に出さない |
| `R2_*` 実値 | 同上（hide / unhide では R2 操作なし、誤って入っても出さない）|
| `manage_url_token` 任意 | DB から読まない / API 戻り値に含めない / log に出さない |
| `draft_edit_token_hash` | 同上 |
| `Set-Cookie` / Cookie 値 | cmd/ops は HTTP を経由しないので発生しない |
| `storage_key` 完全値 | `GetPhotobookForOps` 戻り値・log に出さない（必要なら `octet_length` / 先頭数文字のみ）|
| `presigned URL` | cmd/ops では発行も表示もしない |
| OGP `image_url_path` | 公開 path（`/ogp/<PID>?v=1`）なので可、ただし MVP は本フローで露出する必要なし |

### 9.2 Actor 認証 / 認可（MVP）

- MVP では **単一運用者前提**（kento-matsunaga）
- `--actor <label>` で記録する label は運営内識別子（個人情報を含まない、例: `ops-1`）
- v4 設計書 §4.3 の正規表現 `^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}[a-zA-Z0-9]$` を VO バリデーションで強制
- Cloud SQL Auth Proxy 経由のローカル CLI 実行が前提のため、DB 接続できる時点で
  運営権限相当（DB password / IAM が事実上の認証）
- **Cloud Run Job 化しない / Web admin UI 化しない**ことで、複数アクターの認可機構を作らずに済ませる
- 将来 OperatorId 化 / 認可機構が必要になったタイミングで再検討（§12）

### 9.3 監査ログ

- 設計書 §5 の INDEX を活用して以下の監査クエリが取れる:
  - `(target_photobook_id, executed_at DESC)` — 特定 photobook の操作履歴
  - `(actor_label, executed_at DESC)` — 運営別の活動ログ
  - `(reason, executed_at DESC)` — reason 別の集計（minor_related の頻度 etc.）
- runbook（§11）で監査クエリ例を提示

---

## 10. 実リソース操作（実装 PR で発生するもの）

### 10.1 計画書 PR（本書）

- **実リソース操作: なし**（計画書のみ）
- 本書 commit の影響範囲: docs のみ
- Secret 注入なし

### 10.2 実装 PR（PR34）

| 操作 | タイミング | STOP / 手順 |
|---|---|---|
| migration goose up（`moderation_actions`、案 B 採用時）| 実装完了 + ローカル goose up + Cloud SQL Auth Proxy 経由本番 goose up | **Cloud SQL 適用 STOP** |
| Backend Cloud Build deploy | migration 後 | runbook `docs/runbook/backend-deploy.md` 通り manual submit、traffic-to-latest 確認 |
| Workers redeploy（manage UI banner、案 b 採用時）| Frontend 側完了後 | `npm run cf:build` + `wrangler deploy` |
| Cloud Run Jobs / Scheduler | **不要**（cmd/ops はローカル実行） | - |
| spike 削除 / public repo 化 | **不要**（PR40 / PR38） | - |

---

## 11. Tests 方針

### 11.1 単体（DB 不要 or DB あり）

| 範囲 | 内容 |
|---|---|
| `internal/moderation/domain/entity/moderation_action_test.go` | コンストラクタ / 不変条件（v4 設計書 I1〜I7 のうち適用分）|
| VO test | `ActionKind` / `ActionReason` / `OperatorLabel`（正規表現）/ `ActionDetail` / `ActionId` |
| `internal/moderation/internal/usecase/hide_photobook_test.go` | 正常 / status=draft 拒否 / status=deleted 拒否 / 既に hidden=true 冪等 / OCC（採用時）|
| `internal/moderation/internal/usecase/unhide_photobook_test.go` | 正常 / hidden=false 冪等 / status=deleted 拒否 / correlation_id 任意 |
| Repository test（DB あり） | append-only INSERT / FK なし確認 / INDEX 経由 SELECT |
| `internal/photobook/internal/usecase/get_public_photobook_test.go` 拡張 | hidden=true → ErrPublicGone（既存 + 既存挙動を再確認）|
| `internal/ogp/internal/usecase/get_public_ogp_test.go` 拡張 | hidden=true → not_public（既存）|

### 11.2 cmd/ops dry-run / 確認プロンプト

| テスト | 内容 |
|---|---|
| `cmd/ops/...` `--dry-run` | DB 更新しないこと（INSERT / UPDATE 0 行）/ 出力サマリの確認 |
| `cmd/ops/...` 確認プロンプト | yes 以外は abort（exit 0、DB 不変）|
| `--yes` | プロンプト skip + 実行 |
| Secret 値が stdout / stderr に出ないことの grep |
| `--actor` の正規表現バリデーション |

### 11.3 統合 / 実機（実装 PR で別途）

- ローカル DB に対し `hide` → public viewer 確認 → `unhide` → public viewer 復活 確認
- Outbox `PhotobookHidden` / `PhotobookUnhidden` event が同 TX で INSERT されたか確認
- worker（手動 Job execute）で no-op log が出ること

---

## 12. 後回し事項 / 懸念

### 12.1 後回し（運用 / 別 PR）

| 項目 | 再開・解消条件 |
|---|---|
| `soft_delete` / `restore`（論理復元） / `purge` | PR34 拡張、または別 PR。purge は R2 削除を伴うので Reconcile 整備（PR33e）と組で扱うのが安全 |
| `reissue_manage_url`（管理URL 再発行 + Session revoke + ManageUrlDelivery）| Email Provider 確定後（PR32c 以降）。Session revoke 機構は実装済（auth/session）|
| Report 集約（通報受付 + 自動連動）| PR35 で扱う。本計画では `source_report_id` カラムは nullable で受け、PR35 接続時にアプリ層で連動を追加 |
| Web admin UI / dashboard | MVP 範囲外（v4 §6.19）。複数運営者・遠隔運用が必要になった時点で再検討 |
| Bulk moderation（複数 photobook 一括）| 任意。最初は loop で十分 |
| Legal takedown 固有手続き（rights_claim 詳細 / 通知）| PR35〜37 で扱う |
| 作成者への通知（hide / unhide メール）| Email Provider 確定後 |
| OGP 自動再生成（unhide 後）/ R2 stale cleanup | PR33e（任意）|
| moderation actor 認証（複数運営対応）| 単一運営前提を脱するときに再検討 |

### 12.2 懸念

| 懸念 | 対応 |
|---|---|
| append-only DB 強制（trigger）採用時に運用バッチが書けなくなる | trigger を入れず、アプリ層責務にする（v4 設計書 §6 例示と整合）|
| `version` を上げない方針で OCC が破壊されないか | 編集側 UseCase は `version` を上げる、moderation 側は上げない。ロック粒度は SELECT FOR UPDATE で十分 |
| audit log retention（個人情報）| MVP は detail に個人情報を書かない運用ガイドのみ。法的 retention はローンチ前運用整備で再判断 |
| Email Provider 未確定の影響 | hide / unhide では送信不要のため影響なし。reissue_manage_url が必要になった時点で再開 |
| moderation_actions が肥大化したとき | INDEX で十分。本格 partition は数年単位の話、MVP では未対応 |

### 12.3 未検証

- Outbox `PhotobookHidden` / `PhotobookUnhidden` の同 TX INSERT 経路は新規。
  実装 PR で `internal/photobook` 側の publish_from_draft.go と同様のテストパターン（同 TX で commit / rollback 同時失敗）を入れる
- 同時編集中の hide（編集セッション中の hide が起きたときの作成者 UX）は実機検証なし。
  PR34 実装 PR で safari verification rule に従い実機確認

---

## 13. ユーザー判断事項

> 計画書承認時にユーザーが意思決定するもの。判断結果は本書を更新して記録。

| # | 判断項目 | 候補 / 推奨 | 影響 |
|---|---|---|---|
| 1 | `moderation_actions` テーブルを PR34 で作るか | **推奨: 案 B（作成）**。最小は案 A（既存 hidden_by_operator のみ）| §4 |
| 2 | append-only を DB trigger で強制するか | 推奨: **アプリ層責務のみ**（trigger は MVP 範囲外）。設計書 §6 と整合 | §4 |
| 3 | reason CHECK の許容セット | 推奨: **DB は v4 設計書通り 9 種類すべて、運用は §6.4 の 7 種に絞る**。アプリ層で絞ると将来追加時に再 deploy | §6.4 |
| 4 | hide 対象を published のみにするか / draft も許容するか | 推奨: **published のみ**（draft は public 露出していないので hide の意味がない）| §5.3 |
| 5 | hide / unhide で `version` を上げるか | 推奨: **上げない**（編集 OCC を壊さない）| §5.6 |
| 6 | manage UI に「運営により非公開中」表示を追加するか | 推奨: **案 b 追加**。最小は案 a 何もしない | §8.2 |
| 7 | actor 認証 | 推奨: **MVP は単一運用者前提、`--actor` ラベルのみ記録**。複数運営対応は別 PR | §9.2 |
| 8 | hide 時に R2 object を削除するか | 推奨: **削除しない**（unhide で即戻せる）| §7.3 |
| 9 | hide / unhide 時に Outbox event を INSERT するか（worker handler は no-op）| 推奨: **INSERT する**（v4 P0-19 + 将来の handler 追加で backfill 不要）| §5.4 / §7.4 |
| 10 | Cloud SQL は引き続き `vrcpb-api-verify` 名のままで PR34 を進めるか | 推奨: **そのまま**（本番化 / rename は PR39）| §10 |

---

## 14. 完了条件

PR34 計画書（本書）の完了条件:

- [ ] §1 現状整理（hidden_by_operator の利用箇所 / PR33d 検証済挙動）が事実と一致
- [ ] §2 ゴール / §3 スコープがユーザー承認可能
- [ ] §4 DB 設計案 A / B が提示されており、推奨と判断材料が整理済
- [ ] §5〜§9 が v4 設計書（既存）と整合
- [ ] §10 実リソース操作が実装 PR まで発生しないことが明示
- [ ] §11 Tests 方針が PR34 実装 PR で実行可能
- [ ] §12 後回し事項が roadmap / 別 PR に紐付け済
- [ ] §13 ユーザー判断事項 10 件が漏れなく列挙
- [ ] check-stale-comments + Secret grep をクリア（commit 時に確認）

PR34 実装 PR の完了条件:

- migration goose up（案 B 採用時）が Cloud SQL に適用済
- `internal/moderation/` パッケージが domain / VO / Repository / UseCase / test 揃っている
- `cmd/ops` の `photobook show / list-hidden / hide / unhide` が dry-run + 確認プロンプト + `--yes` で動作
- ローカル DB で hide → public viewer 410 → unhide → 200 が再現
- Backend deploy 完了（runbook 通り）
- Frontend 案 b 採用時、Workers redeploy 完了
- runbook（`docs/runbook/ops-moderation.md`）が整備されている
- pr-closeout（コメント整合 + Secret grep + 後回し事項記録）通過

---

## 15. 関連ドキュメント

- 上流設計: [`docs/design/aggregates/moderation/ドメイン設計.md`](../design/aggregates/moderation/ドメイン設計.md) / [`データモデル設計.md`](../design/aggregates/moderation/データモデル設計.md)
- 業務知識: [`docs/spec/vrc_photobook_business_knowledge_v4.md`](../spec/vrc_photobook_business_knowledge_v4.md) §5.4 / §6.19 / §7.3 / §7.4
- ADR: [`docs/adr/0002-ops-execution-model.md`](../adr/0002-ops-execution-model.md) / [`0003-frontend-token-session-flow.md`](../adr/0003-frontend-token-session-flow.md)
- 横断: [`docs/design/cross-cutting/outbox.md`](../design/cross-cutting/outbox.md)
- 既存 PR 結果: [`harness/work-logs/2026-04-28_ogp-outbox-handler-result.md`](../../harness/work-logs/2026-04-28_ogp-outbox-handler-result.md)
- 公開 Viewer / Manage 計画書: [`docs/plan/m2-public-viewer-and-manage-plan.md`](./m2-public-viewer-and-manage-plan.md)
- ロードマップ: [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md) PR34 章

---

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版作成。PR34 MVP（hide / unhide / show）スコープと v4 完全設計（6 ActionKind + Report 連携）の差分整理。`moderation_actions` テーブル新設案 B 推奨 + ユーザー判断事項 10 件 |
