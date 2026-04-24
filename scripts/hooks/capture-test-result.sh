#!/bin/bash
# capture-test-result.sh — テスト失敗を自動検知して failure-log に記録する
# Usage: capture-test-result.sh "$EXIT_CODE" "$COMMAND" "$OUTPUT"

EXIT_CODE="$1"
COMMAND="$2"
OUTPUT="$3"
FAILURE_LOG_DIR="harness/failure-log"
DATE=$(date +%Y-%m-%d_%H%M%S)

# テスト実行コマンドかどうか判定
is_test_command() {
    case "$COMMAND" in
        *"go test"*|*"npm test"*|*"pytest"*|*"make test"*|*"jest"*|*"vitest"*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

if ! is_test_command; then
    exit 0
fi

# テスト成功時: 追跡ファイルからテスト済みを除去
if [ "$EXIT_CODE" = "0" ]; then
    TRACK_FILE="/tmp/ai-driven-edited-files.txt"
    if [ -f "$TRACK_FILE" ]; then
        > "$TRACK_FILE"  # テスト通過でクリア
    fi
    exit 0
fi

# テスト失敗時: failure-log に記録
mkdir -p "$FAILURE_LOG_DIR"

cat > "${FAILURE_LOG_DIR}/${DATE}_test-failure.md" << EOF
# テスト失敗: ${DATE}

## 発生状況
- コマンド: \`${COMMAND}\`
- 終了コード: ${EXIT_CODE}
- 日時: $(date +"%Y-%m-%d %H:%M:%S")

## 失敗出力
\`\`\`
${OUTPUT}
\`\`\`

## 根本原因
（未分析 — failure-to-rule スキルで分析すること）

## 対策種別
- [ ] ルール化
- [ ] スキル化
- [ ] テスト追加
- [ ] フック追加

## 対策ステータス: PENDING
EOF

echo "⚠️ テスト失敗を記録しました: ${FAILURE_LOG_DIR}/${DATE}_test-failure.md"
echo "→ failure-to-rule スキルで対策をルール化してください"
