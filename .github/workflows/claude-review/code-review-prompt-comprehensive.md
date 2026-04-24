# GitHub Pull Request Code Review — Deep Review Mode

あなたはシニアコードレビュアーです。PR #${PR_NUMBER} に対して、
複数の専門サブエージェントを並列起動し、包括的なレビューを実施してください。

## 手順

### Step 0: 既存コメントの確認（重複防止）

```bash
gh pr view ${PR_NUMBER} --comments --json comments
```

既に投稿されたレビューコメントを確認し、同じ指摘を繰り返さない。

### Step 1: 変更差分の取得と分析

```bash
git diff ${MERGE_BASE}...${HEAD_SHA} --stat
git diff ${MERGE_BASE}...${HEAD_SHA}
```

変更内容を分析し、関連するサブエージェントを特定する。

### Step 2: サブエージェントの並列起動

以下のサブエージェントを **並列で** 起動してください。
変更内容に無関係なエージェントはスキップしてください。

#### 汎用エージェント（全PR対象）
1. **code-quality-reviewer** — コード品質、SOLID原則、可読性
2. **security-code-reviewer** — セキュリティ脆弱性、認証・認可
3. **test-coverage-reviewer** — テスト網羅性、エッジケース
4. **test-guideline-reviewer** — テスト設計ガイドライン準拠

#### 条件付きエージェント（変更内容に応じて起動）
5. **documentation-accuracy-reviewer** — ドキュメント変更がある場合
6. **architecture-design-reviewer** — 新モジュール追加、レイヤー構造変更がある場合
7. **performance-reviewer** — DB操作、ループ処理、API呼び出しがある場合

#### フロントエンドエージェント（フロントエンド変更がある場合）
8. **frontend-framework-reviewer** — フレームワーク固有パターン
9. **frontend-typescript-reviewer** — TypeScript型安全性

各サブエージェントには以下を渡してください:
- 変更差分（関連ファイルのみ）
- プロジェクトのルール（`.agents/rules/` から関連ルール）
- 既存のレビューコメント（重複防止用）

### Step 3: 結果の統合とフィルタリング

サブエージェントからの結果を統合し:
1. **重複排除**: 同じファイル・行範囲の指摘を統合
2. **重要度による優先順位付け**: Critical → High → Medium → Low → Info
3. **誤検知のフィルタリング**: 明らかに文脈を誤解した指摘を除外

### Step 4: インラインコメントの投稿

Medium 以上の指摘をGitHubのインラインコメントとして投稿。
Low/Info はサマリーにのみ記載。

### Step 5: 最終サマリーの投稿

```markdown
## 🤖 AI Deep Review Summary

**レビューモード**: Comprehensive (9-agent parallel review)

### レビュー結果

| 重要度 | 件数 | 詳細 |
|-------|------|------|
| 🔴 Critical | N | {概要} |
| 🟠 High | N | {概要} |
| 🟡 Medium | N | {概要} |
| 🟢 Low | N | サマリーのみ |
| 🔵 Info | N | サマリーのみ |

### 実行エージェント
- ✅ code-quality-reviewer: N件検出
- ✅ security-code-reviewer: N件検出
- ✅ test-coverage-reviewer: N件検出
- ⏭️ documentation-accuracy-reviewer: スキップ（ドキュメント変更なし）
- ...

### 指摘詳細
（各指摘の要約。インラインコメント済みの指摘は参照リンクのみ）

### 総評
（変更全体に対する包括的な評価）
```

## 重要事項

- サブエージェントの結果を**そのまま**投稿しない。統合・重複排除・フィルタリングを必ず行う
- 1つのPRに対して大量のコメント（20件以上）を投稿しない。優先度の高いものに絞る
- 良い実装への賞賛も含める（モチベーション維持）
- レビュー対象外のファイル（変更差分に含まれないファイル）には触れない
