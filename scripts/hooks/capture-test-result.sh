#!/bin/bash
# capture-test-result.sh — テスト失敗を自動検知して failure-log に記録する
# Input: Claude Code PostToolUse(Bash) hook JSON via stdin, or positional args for manual use

EXIT_CODE=""
COMMAND=""
OUTPUT=""

# Claude Code フックは JSON を stdin 経由で渡す
if [ ! -t 0 ]; then
    INPUT=$(cat)
    if [ -n "$INPUT" ]; then
        PARSED=""
        if command -v jq >/dev/null 2>&1; then
            PARSED=$(echo "$INPUT" | jq -r '[.tool_input.command // "", .tool_response.stdout // "", .tool_response.stderr // "", (.tool_response.exit_code // "" | tostring)] | @tsv' 2>/dev/null)
        elif command -v python3 >/dev/null 2>&1; then
            PARSED=$(echo "$INPUT" | python3 -c '
import json, sys
d = json.load(sys.stdin)
ti = d.get("tool_input", {})
tr = d.get("tool_response", {})
ec = tr.get("exit_code", "")
print("\t".join([ti.get("command",""), tr.get("stdout",""), tr.get("stderr",""), str(ec) if ec != "" else ""]))
' 2>/dev/null)
        fi
        if [ -n "$PARSED" ]; then
            IFS=$'\t' read -r COMMAND STDOUT STDERR EXIT_CODE <<< "$PARSED"
            OUTPUT="${STDOUT}${STDERR:+
${STDERR}}"
            # exit_code が取れなければ stderr 有無で推定
            if [ -z "$EXIT_CODE" ]; then
                if [ -n "$STDERR" ]; then
                    EXIT_CODE="1"
                else
                    EXIT_CODE="0"
                fi
            fi
        fi
    fi
fi

# 手動実行用フォールバック
[ -z "$EXIT_CODE" ] && EXIT_CODE="$1"
[ -z "$COMMAND" ] && COMMAND="$2"
[ -z "$OUTPUT" ] && OUTPUT="$3"

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
