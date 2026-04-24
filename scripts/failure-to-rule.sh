#!/bin/bash
# failure-to-rule.sh — テスト失敗から新しいルールのテンプレートを生成する
# Usage: ./scripts/failure-to-rule.sh "失敗の説明" "カテゴリ"
# カテゴリ: testing, domain, security, infrastructure, process

set -euo pipefail

DESCRIPTION="${1:?Usage: failure-to-rule.sh \"失敗の説明\" \"カテゴリ\"}"
CATEGORY="${2:?カテゴリを指定してください: testing, domain, security, infrastructure, process}"
DATE=$(date +%Y-%m-%d)
RULE_NAME=$(echo "$DESCRIPTION" | tr ' ' '-' | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9-]//g' | head -c 50)
RULES_DIR=".agents/rules"
FAILURE_LOG_DIR="harness/failure-log"

# バリデーション
case "$CATEGORY" in
    testing|domain|security|infrastructure|process)
        ;;
    *)
        echo "エラー: 無効なカテゴリ: $CATEGORY"
        echo "有効なカテゴリ: testing, domain, security, infrastructure, process"
        exit 1
        ;;
esac

# ルールテンプレート生成
RULE_FILE="${RULES_DIR}/${CATEGORY}-${RULE_NAME}.md"

cat > "$RULE_FILE" << EOF
---
description: "${DESCRIPTION}"
globs: ["TODO: 適用対象のファイルパターンを指定"]
---

# ${DESCRIPTION}

## 必須/禁止事項
- TODO: 具体的なルールを記述

## 正しい例
\`\`\`
// ✅ こうする
TODO: 正しいコード例
\`\`\`

## 誤った例
\`\`\`
// ❌ これは禁止
TODO: 誤ったコード例
\`\`\`

## Why
発生日: ${DATE}
根本原因: TODO — 5 Whys分析を実施して記述すること

## 対策テスト
TODO: このルールが機能することを検証するテストを追加
EOF

echo "✅ ルールテンプレートを生成しました: $RULE_FILE"
echo ""
echo "次のステップ:"
echo "  1. $RULE_FILE の TODO を埋める"
echo "  2. globs を正しいパターンに設定する"
echo "  3. テストを追加して検証する"
echo "  4. harness/QUALITY_SCORE.md を更新する"

# 失敗ログにも記録
mkdir -p "$FAILURE_LOG_DIR"
cat > "${FAILURE_LOG_DIR}/${DATE}_${RULE_NAME}.md" << EOF
# 失敗記録: ${DESCRIPTION}
日時: ${DATE}
カテゴリ: ${CATEGORY}
対策ルール: ${RULE_FILE}
対策ステータス: IN_PROGRESS
EOF

echo "📝 失敗ログを記録しました: ${FAILURE_LOG_DIR}/${DATE}_${RULE_NAME}.md"
