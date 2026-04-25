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

# 追跡対象: ソース / 設計 / 設定 / Docker / DB スキーマ
# Secret 系（.env / .env.local 等）は明示的に除外する。.env.example のみ追跡対象。
case "$FILEPATH" in
    # --- Secret を含む可能性が高いファイルは追跡しない ---
    *.env|*.env.local|*.env.production|*.env.staging|*.env.dev|*.env.test)
        exit 0
        ;;
    # --- テストファイル自体は追跡しない（変更検知の対象外） ---
    *_test.go|*.test.ts|*.test.tsx|*.spec.ts|*_test.py)
        exit 0
        ;;
    # --- ソースコード（既存） ---
    *.go|*.ts|*.tsx|*.py)
        echo "$FILEPATH" >> "$TRACK_FILE"
        sort -u "$TRACK_FILE" -o "$TRACK_FILE"
        ;;
    # --- ドキュメント / 設計（M1 はこちらが中心） ---
    *.md|*.mdx)
        echo "$FILEPATH" >> "$TRACK_FILE"
        sort -u "$TRACK_FILE" -o "$TRACK_FILE"
        ;;
    # --- 設定 / インフラ定義 ---
    *.json|*.yaml|*.yml|*.toml)
        echo "$FILEPATH" >> "$TRACK_FILE"
        sort -u "$TRACK_FILE" -o "$TRACK_FILE"
        ;;
    # --- DB スキーマ / クエリ ---
    *.sql)
        echo "$FILEPATH" >> "$TRACK_FILE"
        sort -u "$TRACK_FILE" -o "$TRACK_FILE"
        ;;
    # --- Docker / VCS / 公開可能な env サンプル ---
    *Dockerfile|*.dockerignore|*.gitignore|*.env.example)
        echo "$FILEPATH" >> "$TRACK_FILE"
        sort -u "$TRACK_FILE" -o "$TRACK_FILE"
        ;;
esac
