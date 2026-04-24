# Claude PR レビュー & アシスト

## 概要

AIによる自動コードレビューシステム。PRの作成・更新時に自動レビューし、
条件を満たすPRを自動承認する。

## レビューモード

### 1. Standard Review（標準）
- **トリガー**: PR作成時に自動実行、または `@claude review`
- **内容**: 単一パスのコードレビュー（バグ、セキュリティ、設計、テスト、パフォーマンス）
- **用途**: 通常のPRレビュー

### 2. Deep Review（深層）
- **トリガー**: `@claude deepreview`
- **内容**: 最大9つの専門サブエージェントを並列起動した包括的レビュー
- **用途**: 重要な変更、アーキテクチャ変更、リリース前の最終チェック

### 3. Frontend Review（フロントエンド専用）
- **トリガー**: フロントエンドのみのPR作成時に自動実行、または `@claude frontreview`
- **内容**: フロントエンド専門の4サブエージェントによるレビュー + 自動承認判定
- **用途**: フロントエンドのPR

### 4. Assist（アシスト）
- **トリガー**: `@claude` + 質問やタスク（PR/Issueコメント内）
- **内容**: 調査、コード修正、質問回答
- **用途**: レビュー以外のインタラクティブな支援

## セットアップ

### 必須シークレット
| シークレット名 | 説明 |
|-------------|------|
| `ANTHROPIC_API_KEY` | Anthropic API キー |

### オプションシークレット
| シークレット名 | 説明 |
|-------------|------|
| `MCP_CONFIG` | MCP設定JSON（Notion等の外部サービス連携） |

### リポジトリ変数（vars）
| 変数名 | デフォルト | 説明 |
|-------|-----------|------|
| `CLAUDE_ACTION_MODEL_REVIEW` | `claude-sonnet-4-5-20250929` | レビュー用モデル |
| `CLAUDE_ACTION_MODEL_FRONTREVIEW` | `claude-sonnet-4-5-20250929` | フロントレビュー用モデル |
| `CLAUDE_ACTION_MODEL_ASSIST` | `claude-sonnet-4-5-20250929` | アシスト用モデル |

## ファイル構成

```
.github/workflows/
├── claude-review.yml              # 標準 + 深層レビューワークフロー
├── claude-frontreview.yml         # フロントエンド専用レビュー（オプション）
├── claude-assist.yml              # インタラクティブアシスト
└── claude-review/
    ├── code-review-prompt.md      # 標準レビュープロンプト
    ├── code-review-prompt-comprehensive.md  # 深層レビュープロンプト
    ├── code-review-prompt-frontend-only.md  # フロントレビュー + 自動承認
    └── README.md                  # このファイル

.claude/agents/review/             # レビューサブエージェント定義
├── code-quality-reviewer.md
├── security-code-reviewer.md
├── test-coverage-reviewer.md
├── test-guideline-reviewer.md
├── documentation-accuracy-reviewer.md
├── architecture-design-reviewer.md
├── performance-reviewer.md
├── frontend-framework-reviewer.md
├── frontend-architecture-reviewer.md
├── frontend-typescript-reviewer.md
└── frontend-performance-reviewer.md
```

## 自動承認について

フロントエンドレビューでは、条件を満たすPRを自動承認（APPROVE）します:
- Medium 以上の指摘が 0 件 → APPROVE
- 既存の指摘がすべて解決済み + 新規 Medium+ が 0 件 → APPROVE
- 返信キーワード（"問題なし", "仕様です" 等）で重要度をダウングレード可能

詳細は `code-review-prompt-frontend-only.md` を参照。

## カスタマイズ

### サブエージェントの追加
1. `.claude/agents/review/` に新しいエージェント定義を作成
2. 対応するプロンプトファイルのサブエージェントリストに追加

### レビュー観点の変更
プロンプトファイル（`.github/workflows/claude-review/`）を編集。

### フロントエンドレビューを無効化
`claude-frontreview.yml` を削除するだけでOK。
