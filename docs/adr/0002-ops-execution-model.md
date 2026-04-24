# ADR-0002 運営操作方式

## ステータス
Accepted

## 作成日
2026-04-25

## 最終更新
2026-04-25

## コンテキスト

Moderation 集約で扱う運営操作（hide / unhide / restore / purge / reissue_manage_url / resolve_report / list_reports / list_moderation_actions）は、外部に公開できない性質を持つ。以下の要件があるため、安易に HTTP API 化できない。

- 運営操作は全件 ModerationAction として監査ログに残す必要がある（v4、同一トランザクション必須）
- Photobook 状態変更・ModerationAction INSERT・Report 状態更新・outbox_events INSERT の4要素を同一TX で原子実行する必要がある
- 管理URL再発行、フォトブック物理削除、minor_safety_concern 対応など、誤実行時のリカバリ困難な操作が含まれる
- MVP では運営ダッシュボード UI を作る工数がない
- 運営操作の実行頻度は想定で週数件〜十数件程度と低い

HTTP API にすると、認可（mTLS / トークン / IP 許可）、監査、秘匿、UI または CLI クライアントなど周辺コストが加算される。MVP の規模ではこれらが過剰である。一方で DB 直接操作は 4要素同一TX や不変条件（ModerationAction 必須、immutable、Outbox 副作用）を守れない。中間として「Go CLI + shell ラッパー」方式を採用する。

## 決定

### 全体方針

- MVP では運営用 HTTP API (`/internal/ops/*`) を作らない。
- 運営操作は **`scripts/ops/*.sh` + Go CLI (`cmd/ops`)** で実行する。
- Go CLI は **`cmd/ops` 単一バイナリ + サブコマンド方式** にする（Cobra 採用を推奨）。
- 運営 UseCase は Application 層に実装し、`cmd/ops` のサブコマンドはその薄いラッパーに徹する。
- Phase 2 で運営 UI を作る場合も、同じ UseCase を HTTP API から呼び直せばよい（再実装不要）。

### 採用構成

**Go CLI 実体**（Cobra + urfave/cli は比較の上、Cobra を推奨）:

```
cmd/ops/
├── main.go
└── cmd/
    ├── photobook_hide.go
    ├── photobook_unhide.go
    ├── photobook_restore.go
    ├── photobook_purge.go
    ├── photobook_reissue_manage_url.go
    ├── report_resolve.go
    ├── reconcile_outbox.go
    ├── reconcile_draft.go
    └── reconcile_image.go
```

**Shell ラッパー**（運用者向けのエントリポイント）:

```
scripts/ops/
├── hide.sh
├── unhide.sh
├── restore.sh
├── purge.sh
├── reissue-manage-url.sh
├── report-resolve.sh
└── reconcile/
    ├── outbox_failed.sh
    ├── draft_expired.sh
    ├── image_references.sh
    ├── photobook_image_integrity.sh
    └── cdn_cache_force_purge.sh
```

Shell ラッパーは Go CLI バイナリを適切な環境変数と必須フラグ付きで呼ぶ薄いスクリプトとする。環境（dev / staging / prod）の接続先切替、Secret Manager からのシークレット注入、gcloud 認証の確認などをラッパー側に押し込むことで、運用者が CLI 引数を覚える負担を減らす。

### 運営操作対象

| 操作 | 影響 | 4要素同一TX |
|------|------|:---:|
| hide | Photobook.hidden_by_operator = true | ✅ |
| unhide | Photobook.hidden_by_operator = false | ✅（Report は自動更新しない） |
| restore | 削除から戻す | ✅（Report は自動更新しない） |
| purge | 物理削除、slug 解放 | ✅ |
| reissue_manage_url | manage_url_token 再発行、ManageUrlDelivery 新規作成 | ✅ |
| resolve_report | Report 状態更新（action_taken / no_action / dismissed） | ✅ |
| list_reports | Report 一覧（minor_safety_concern 優先表示） | 参照のみ |
| list_moderation_actions | 監査ログ一覧 | 参照のみ |
| reconcile 系 | Draft 期限切れ / Outbox 失敗 / 画像参照不整合 等の修復 | 操作別 |

### 必須ルール

- **HTTP の `/internal/ops/*` は MVP では作らない**。誤って実装された場合は CI / lint で検出するルールを追加する。
- 運営 UseCase は Application 層（`backend/internal/{module}/internal/usecase/command/`）に実装する。
- `cmd/ops` は UseCase を直接呼ぶ。CLI 層でドメインロジックを書かない。
- Phase 2 で運営 UI を作る場合は、同じ UseCase を HTTP ハンドラから呼べばよい（再実装しない）。
- **破壊的操作には `--dry-run` を用意し、`--dry-run` をデフォルトとする**。実行には `--execute` を明示的に指定させる。
- **実行時には `--operator` を必須にする**。未指定ならエラー終了（非ゼロ exit code）。
- **ModerationAction を必ず記録する**。
- Photobook 状態変更、ModerationAction INSERT、Report 状態更新、outbox_events INSERT は同一トランザクションで行う。

### 参照系サブコマンドの扱い

`list_reports` / `list_moderation_actions` などの **参照系サブコマンドは DB 状態を変更しないため `--execute` は不要** とする。`--dry-run` フラグも受け付けない（意味を持たないため）。

ただし、参照系であっても以下のルールを守る。

- 出力に個人情報（reporter_contact、recipient_email 等）を含めない。必要があればマスクするか、`--include-sensitive` のような明示フラグ + 監査ログ記録を必須にする。
- 出力に管理 URL（raw token）、draft_edit_token、manage_url_token を**絶対に含めない**。
- 出力に session_token、presigned URL を含めない。
- `--operator` は参照系でも必須にする。誰が閲覧したかの履歴を残したい場合に備え、参照系でも operator を出力ログに刻む。

### operator 識別子

`operator` は個人情報を含まない運営内識別子として扱う。漏洩時に個人特定できないよう、メール・本名・電話・SNS ID そのものは禁止する。

**許可例**: `ops-kento`, `ops-001`, `legal-team`, `support-01`, `security.lead`

**禁止**: 個人メールアドレス、本名フルネーム、電話番号、外部 SNS ID そのもの。

**検証ルール**:
- 3〜64 文字
- 半角英数字、ハイフン、アンダースコア、ドット
- 先頭と末尾は英数字を推奨
- 正規表現: `^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}[a-zA-Z0-9]$`

`ModerationAction.actor_label` にこの operator を記録する。個人と operator ラベルの紐付けは別途運営内部の秘匿台帳で管理し、コードベース・DB には入れない。

### CLI テスト方針

UseCase 自体は Application 層のテーブル駆動テストで担保する（`testing.md` 準拠）。CLI 層は UseCase を呼ぶ薄層として以下を最低限テストする。

- 必須フラグ（`--photobook-id`, `--operator`, `--reason` など）不足時にエラー終了する
- `--operator` 未指定でエラー終了する
- `--dry-run` が有効に働き、DB 書き込みが発生しない
- 破壊的操作で `--execute` なしの場合は実行されない（`--dry-run` デフォルト）
- サブコマンドが正しい UseCase を呼ぶ（モックで検証）
- exit code が成功時 0、失敗時 非ゼロ を返す
- operator 識別子の正規表現バリデーションが効く
- 参照系で `--execute` を受け付けない（あるいは無視される）

Cobra を採用する場合、`Command.RunE` 単位のユニットテストを書く。入出力は `bytes.Buffer` で差し替え、テーブル駆動で統一する。

## 検討した代替案

- **mTLS 付き内部 HTTP API**: 認可は堅牢だが、証明書発行・ローテ・運用者の環境セットアップが重く、MVP 規模で過剰。運営人数が1〜2人なら CLI の方が運用シンプル。
- **`X-Ops-Token` のような固定トークン内部 API**: トークン漏洩時の被害範囲が運営全操作になる。ローテ・配布・ログ秘匿の手順が必要になり、MVP で管理コストが高い。
- **DB 直接操作**: 4要素同一TX を守れない。ModerationAction 記録漏れ、Outbox INSERT 漏れ、不変条件違反が必ず起きる。監査ログも残らない。完全禁止。
- **localhost only HTTP handler**: SSH Port Forwarding や踏み台経由で迂回可能、認証ゼロ、ログも弱い。DB 直接操作と同じく危険。
- **個別 Go バイナリを大量に作る**: `hide`、`unhide`、`restore` などを個別バイナリ化すると、ビルド・配布・バージョン整合の複雑さが爆発する。単一バイナリ + サブコマンドの方が管理が楽。
- **shell だけで DB を直接叩く案**: DB 直接操作と同じく 4要素同一TX を守れない。却下。

## 結果

### メリット

- 外部公開される攻撃面がゼロ（HTTP 経路なし）
- UseCase を Application 層で共通化できるため、Phase 2 の運営 UI 化時は HTTP ハンドラを薄く書くだけで済む
- `--dry-run` デフォルト + `--execute` 明示で、誤実行を構造的に防げる
- Go で書くため、ドメイン不変条件（4要素同一TX、ModerationAction 必須）を型で守れる
- operator 識別子の正規表現バリデーションで、個人情報混入を構造的に防げる

### デメリット

- 運営者が gcloud CLI / Cloud Run Jobs / SSH 踏み台などの CLI 操作に習熟する必要がある
- UI がないため、list_reports の結果は CLI 出力（JSON / table）で見ることになる。minor_safety_concern の優先度が視覚的に伝わりづらい
- 運営者が増えた場合、CLI 配布と operator 識別子の管理台帳を運用で維持する必要がある
- 実行環境（Cloud Run Jobs か、踏み台 VM か、開発者ローカルか）の決定が別途必要

### 後続作業への影響

- M1: `cmd/ops/` 雛形と Cobra 設定、Makefile への `make ops/hide` 等のターゲット追加。
- M4: 運営 UseCase（HidePhotobook, UnhidePhotobook, RestorePhotobook, PurgePhotobook, ReissueManageUrl, ResolveReport）を実装する際、CLI からの呼び出しを前提に DI を設計する。
- M6: reconciler は `cmd/ops/cmd/reconcile_*.go` と自動 cron 起動の両経路から同じ処理を呼ぶ構造にする。
- 運用: scripts/ops/\*.sh の実行環境、operator 識別子の管理台帳、実行ログの保存先を Runbook 化する。

## 未解決事項 / 検証TODO

- **サブコマンドライブラリの確定**: Cobra を推奨としたが、MVP の依存数最小化を重視するなら `urfave/cli` も候補。M4 冒頭で最終決定する。
- **CLI 実行環境**: Cloud Run Jobs で走らせるか、専用の踏み台 VM か、ローカルから Cloud SQL Auth Proxy 経由か。セキュリティと運用容易性のトレードオフを評価する。
- **operator 識別子管理台帳の配置**: 運営内秘匿台帳を 1Password / Google Sheets / 別リポジトリのどれで管理するかの運用ルール策定。
- **list_reports の出力フォーマット**: JSON + jq 連携で足りるか、TSV / table で運営者が扱いやすいか。Phase 2 UI 化時のスキーマ互換を見据えて決定する。
- **破壊的操作の多段確認**: `purge` / `reissue_manage_url` に `--confirm "DELETE"` のような追加確認を入れるかをセキュリティレビューで確定する。

## 関連ドキュメント

- `docs/spec/vrc_photobook_business_knowledge_v4.md`（作成予定 / v4相当の業務知識）
- `docs/design/aggregates/moderation/ドメイン設計.md`
- `docs/design/aggregates/moderation/データモデル設計.md`
- `docs/design/aggregates/report/ドメイン設計.md`
- `.agents/rules/security-guard.md`
- `ADR-0001 技術スタック`
- `ADR-0004 メールプロバイダ選定`（reissue_manage_url からの ManageUrlDelivery 新規作成で関連）
