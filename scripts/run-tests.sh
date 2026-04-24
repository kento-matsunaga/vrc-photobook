#!/bin/bash
# run-tests.sh — すべてのテストを実行する
# Usage: ./scripts/run-tests.sh

set -euo pipefail

TEMPLATE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TOTAL_PASS=0
TOTAL_FAIL=0
FAILED_SUITES=()

echo "========================================"
echo "  AI駆動開発テンプレート — テスト実行"
echo "========================================"
echo ""

run_test_suite() {
    local name="$1"
    local script="$2"

    echo ">>> $name"
    if bash "$TEMPLATE_DIR/$script"; then
        echo "  → PASS"
    else
        FAILED_SUITES+=("$name")
        echo "  → FAIL"
    fi
    echo ""
}

# テストスイート実行
run_test_suite "ハーネス構造テスト" "tests/harness_test.sh"
run_test_suite "ルール品質テスト" "tests/rules_test.sh"
run_test_suite "AIレビューシステムテスト" "tests/review_test.sh"
run_test_suite "品質チェックフックテスト" "tests/quality_hooks_test.sh"
run_test_suite "テンプレート適用テスト" "tests/template_test.sh"

# 総合結果
echo "========================================"
echo "  総合結果"
echo "========================================"

if [ ${#FAILED_SUITES[@]} -gt 0 ]; then
    echo ""
    echo "❌ 失敗したテストスイート:"
    for suite in "${FAILED_SUITES[@]}"; do
        echo "  - $suite"
    done
    echo ""
    exit 1
else
    echo ""
    echo "✅ すべてのテストスイートが通りました。"
    echo ""
    exit 0
fi
