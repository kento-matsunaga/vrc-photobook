#!/bin/bash
# setup.sh — AI駆動開発テンプレートを新規プロジェクトに適用する
# Usage: ./scripts/setup.sh /path/to/your-project

set -euo pipefail

TEMPLATE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TARGET_DIR="${1:?Usage: setup.sh /path/to/your-project}"

if [ ! -d "$TARGET_DIR" ]; then
    echo "エラー: ディレクトリが見つかりません: $TARGET_DIR"
    exit 1
fi

echo "=== AI駆動開発テンプレート適用 ==="
echo "テンプレート: $TEMPLATE_DIR"
echo "適用先: $TARGET_DIR"
echo ""

# .agents/ をコピー
echo "[1/9] .agents/ をコピー中..."
cp -r "$TEMPLATE_DIR/.agents" "$TARGET_DIR/"

# .claude/ を作成してシンボリックリンク
echo "[2/9] .claude/ をセットアップ中..."
mkdir -p "$TARGET_DIR/.claude"
ln -sfn "../.agents/rules" "$TARGET_DIR/.claude/rules"
ln -sfn "../.agents/skills" "$TARGET_DIR/.claude/skills"
cp "$TEMPLATE_DIR/.claude/settings.json" "$TARGET_DIR/.claude/settings.json"

# .claude/agents/review/ をコピー（レビューサブエージェント）
echo "[3/9] レビューサブエージェントをコピー中..."
mkdir -p "$TARGET_DIR/.claude/agents/review"
cp "$TEMPLATE_DIR"/.claude/agents/review/*.md "$TARGET_DIR/.claude/agents/review/"

# .github/workflows/ をコピー（AIレビューワークフロー）
echo "[4/9] GitHub Actionsワークフローをコピー中..."
mkdir -p "$TARGET_DIR/.github/workflows/claude-review"
cp "$TEMPLATE_DIR"/.github/workflows/claude-review.yml "$TARGET_DIR/.github/workflows/"
cp "$TEMPLATE_DIR"/.github/workflows/claude-assist.yml "$TARGET_DIR/.github/workflows/"
cp "$TEMPLATE_DIR"/.github/workflows/claude-review/*.md "$TARGET_DIR/.github/workflows/claude-review/"
# フロントエンドレビューはオプション
if [ -f "$TEMPLATE_DIR/.github/workflows/claude-frontreview.yml" ]; then
    cp "$TEMPLATE_DIR/.github/workflows/claude-frontreview.yml" "$TARGET_DIR/.github/workflows/"
    echo "  フロントエンドレビューを含めました（不要なら claude-frontreview.yml を削除）"
fi

# harness/ をコピー
echo "[5/9] harness/ をコピー中..."
cp -r "$TEMPLATE_DIR/harness" "$TARGET_DIR/"

# docs/ をコピー
echo "[6/9] docs/ をコピー中..."
mkdir -p "$TARGET_DIR/docs"
cp "$TEMPLATE_DIR/docs/ディレクトリマッピング.md" "$TARGET_DIR/docs/"

# scripts/ をコピー
echo "[7/9] scripts/ をコピー中..."
cp -r "$TEMPLATE_DIR/scripts" "$TARGET_DIR/"
chmod +x "$TARGET_DIR/scripts/"*.sh
chmod +x "$TARGET_DIR/scripts/hooks/"*.sh

# CLAUDE.md をコピー（既存がなければ）
echo "[8/9] CLAUDE.md をセットアップ中..."
if [ ! -f "$TARGET_DIR/CLAUDE.md" ]; then
    cp "$TEMPLATE_DIR/CLAUDE.md" "$TARGET_DIR/"
    echo "  CLAUDE.md を作成しました"
else
    echo "  CLAUDE.md は既に存在します。スキップしました。"
fi

# .gitignore を追加（既存がなければ）
echo "[9/9] .gitignore をセットアップ中..."
if [ ! -f "$TARGET_DIR/.gitignore" ]; then
    cp "$TEMPLATE_DIR/.gitignore" "$TARGET_DIR/"
    echo "  .gitignore を作成しました"
else
    echo "  .gitignore は既に存在します。手動でマージしてください。"
fi

echo ""
echo "=== 適用完了 ==="
echo ""
echo "次のステップ:"
echo "  1. $TARGET_DIR/CLAUDE.md をプロジェクトに合わせてカスタマイズ"
echo "  2. $TARGET_DIR/docs/ディレクトリマッピング.md を更新"
echo "  3. $TARGET_DIR/harness/QUALITY_SCORE.md にモジュールを追加"
echo "  4. GitHub Secrets に ANTHROPIC_API_KEY を設定"
echo "  5. テスト実行: cd $TARGET_DIR && bash scripts/run-tests.sh"
