#!/bin/bash
# check-stale-comments.sh — PR 終了時のコメント整合チェック補助。
#
# 目的:
#   - 実装済なのに「後続 PR」「未接続」「未実装」「SendGrid 前提」など
#     劣化しやすいコードコメントを検出する補助ツール。
#   - 0 件でなくても exit 0（候補は人間 / Claude Code が分類する）。
#
# 使い方:
#   bash scripts/check-stale-comments.sh
#   bash scripts/check-stale-comments.sh --extra "your-extra-keyword"   # 追加キーワード
#
# 判断ルール:
#   - .agents/rules/pr-closeout.md §3 の 4 区分（修正する / 残してよい /
#     過去経緯として残す / 生成元を直す）に各ヒットを振り分ける。
#
# 除外:
#   - harness/work-logs / harness/failure-log（履歴記録は触らない）
#   - node_modules / .next / .open-next / .wrangler（生成物 / 依存関係）
#   - sqlcgen（DO NOT EDIT、生成元 SQL を直す）
#   - migrations（履歴 DDL）
#   - go.sum（依存ハッシュに `PR8nw` 等が偶然含まれる）

set -uo pipefail

# repo root に移動（hook が repo root 起点で動く前提）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ---- キーワード -------------------------------------------------------------
# pr-closeout.md §2 と一致させる。新カテゴリ追加時は同時に更新する。
KEYWORDS_BASE=(
  'PR[0-9]+'
  'PR [0-9]+'
  '後続 PR'
  '後続PR'
  '未接続'
  '未実装'
  'TODO'
  'FIXME'
  'future'
  'later'
  'not connected'
  'not implemented'
  'placeholder'
  'SendGrid'
  '\bSES\b'
  'Outbox INSERT'
  '本 PR では'
  '本PRでは'
  '実装予定'
  '再発行'
  '\bprovider\b'
)

EXTRA=""
if [ "${1:-}" = "--extra" ] && [ -n "${2:-}" ]; then
  EXTRA="${2}"
fi

PATTERN=$(IFS='|'; echo "${KEYWORDS_BASE[*]}")
if [ -n "${EXTRA}" ]; then
  PATTERN="${PATTERN}|${EXTRA}"
fi

# ---- 検索対象 ---------------------------------------------------------------
TARGETS=(
  "${REPO_ROOT}/backend"
  "${REPO_ROOT}/frontend/src"
  "${REPO_ROOT}/frontend/app"
  "${REPO_ROOT}/frontend/middleware.ts"
  "${REPO_ROOT}/frontend/wrangler.jsonc"
  "${REPO_ROOT}/frontend/next.config.mjs"
  "${REPO_ROOT}/frontend/next.config.ts"
  "${REPO_ROOT}/docs"
  "${REPO_ROOT}/.agents"
  "${REPO_ROOT}/CLAUDE.md"
  "${REPO_ROOT}/README.md"
)

EXISTING_TARGETS=()
for t in "${TARGETS[@]}"; do
  if [ -e "${t}" ]; then
    EXISTING_TARGETS+=("${t}")
  fi
done

if [ "${#EXISTING_TARGETS[@]}" -eq 0 ]; then
  echo "no targets found under ${REPO_ROOT}; nothing to do"
  exit 0
fi

# ---- 検索実行 ---------------------------------------------------------------
echo "== check-stale-comments =="
echo "repo:    ${REPO_ROOT}"
echo "pattern: ${PATTERN}"
echo "targets: ${EXISTING_TARGETS[*]}"
echo ""

# grep -RInE は対象が無いとエラーで止まる場合があるので、|| true で握る。
RAW=$(grep -RInE "${PATTERN}" "${EXISTING_TARGETS[@]}" 2>/dev/null || true)

# 除外フィルタ。
FILTERED=$(printf "%s\n" "${RAW}" \
  | grep -v "/harness/work-logs/" \
  | grep -v "/harness/failure-log/" \
  | grep -v "/node_modules/" \
  | grep -v "/.next/" \
  | grep -v "/.open-next/" \
  | grep -v "/.wrangler/" \
  | grep -v "/sqlcgen/" \
  | grep -v "/migrations/" \
  | grep -v "go.sum" \
  | grep -v "package-lock.json" \
  | grep -v "_test.go" \
  | grep -vE '^\s*$' \
  || true)

if [ -z "${FILTERED}" ]; then
  echo "no stale-comment candidates found."
  echo "（フィルタ後 0 件。除外対象に入っているだけの可能性もあるので、PR 内容に応じて --extra でキーワードを追加することを検討してください。）"
  exit 0
fi

COUNT=$(printf "%s\n" "${FILTERED}" | wc -l)
echo "stale candidates: ${COUNT} hits"
echo ""
echo "${FILTERED}"
echo ""
echo "== how to handle =="
echo ""
echo "各ヒットを .agents/rules/pr-closeout.md §3 の 4 区分に分類してください。"
echo "  A. 修正する          — 実装済なのに『未実装』と書いてある等"
echo "  B. 状態ベース TODO で残す — 本当に未実装かつ新正典に予定が記載済"
echo "  C. 過去経緯として残す  — migration / failure-log / 設定ファイル経緯"
echo "  D. 生成元を直す        — sqlcgen 等は直接編集せず生成元を更新"
echo ""
echo "完了報告にチェックリスト（pr-closeout.md §6）を含めてください。"

# 候補が出ても exit 0（補助ツールであり、判断は人間 / Claude Code）
exit 0
