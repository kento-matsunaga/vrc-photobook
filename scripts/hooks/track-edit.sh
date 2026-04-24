#!/bin/bash
# track-edit.sh — 変更ファイルを追跡してテスト未実行を検知する
# Input: Claude Code PostToolUse hook JSON via stdin, or positional arg for manual use

TRACK_FILE="/tmp/ai-driven-edited-files.txt"
FILEPATH=""

# Claude Code フックは JSON を stdin 経由で渡す
if [ ! -t 0 ]; then
    INPUT=$(cat)
    if [ -n "$INPUT" ]; then
        if command -v jq >/dev/null 2>&1; then
            FILEPATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null)
        elif command -v python3 >/dev/null 2>&1; then
            FILEPATH=$(echo "$INPUT" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("tool_input",{}).get("file_path",""))' 2>/dev/null)
        fi
    fi
fi

# 手動実行用フォールバック
[ -z "$FILEPATH" ] && FILEPATH="$1"

if [ -z "$FILEPATH" ]; then
    exit 0
fi

# 追跡対象: ソースファイルのみ（テストファイルは除外）
case "$FILEPATH" in
    *_test.go|*.test.ts|*.test.tsx|*.spec.ts|*_test.py)
        exit 0  # テストファイル自体は追跡しない
        ;;
    *.go|*.ts|*.tsx|*.py)
        echo "$FILEPATH" >> "$TRACK_FILE"
        # 重複除去
        sort -u "$TRACK_FILE" -o "$TRACK_FILE"
        ;;
esac
