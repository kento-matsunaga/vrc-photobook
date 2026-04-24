#!/bin/bash
# track-edit.sh — 変更ファイルを追跡してテスト未実行を検知する
# Usage: track-edit.sh "$FILEPATH"

FILEPATH="$1"
TRACK_FILE="/tmp/ai-driven-edited-files.txt"

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
