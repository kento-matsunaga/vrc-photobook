#!/bin/bash
# check-untested.sh — 変更したがテスト未実行のファイルを警告する
# Usage: エージェント停止時に自動実行

TRACK_FILE="/tmp/ai-driven-edited-files.txt"

if [ ! -f "$TRACK_FILE" ] || [ ! -s "$TRACK_FILE" ]; then
    exit 0
fi

echo ""
echo "=========================================="
echo "⚠️  テスト未実行の変更ファイルがあります"
echo "=========================================="
echo ""

while IFS= read -r file; do
    echo "  - $file"
done < "$TRACK_FILE"

echo ""
echo "テストを実行してから停止してください。"
echo "=========================================="

# 未テストファイル数を返す（非ゼロ = 警告あり）
wc -l < "$TRACK_FILE" | tr -d ' '
