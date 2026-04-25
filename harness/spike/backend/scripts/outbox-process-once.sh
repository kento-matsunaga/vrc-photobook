#!/usr/bin/env bash
# outbox-process-once.sh
#
# Outbox ワーカーの最小 CLI を 1 回起動する（M1 priority 7 PoC）。
#
# 想定起動経路:
#   - ローカル: `./scripts/outbox-process-once.sh`
#   - 本実装: Cloud Run Jobs + Cloud Scheduler（U11、cross-cutting/reconcile-scripts.md §3.7.5）
#
# 環境変数:
#   - DATABASE_URL: pgx 接続文字列。.env.local で読み込み済みなら不要。
#   - OUTBOX_LIMIT: 1 回の claim で処理する最大件数（既定 50）
#   - OUTBOX_RETRY_FAILED=1: failed → pending 再投入（outbox_failed_retry 相当）

set -euo pipefail

LIMIT="${OUTBOX_LIMIT:-50}"
SPIKE_BACKEND_DIR="$(cd "$(dirname "$0")/.." && pwd)"

# DATABASE_URL が未設定なら .env.local から読み込む。
# Secret 値（R2_*, TURNSTILE_*）は echo / printenv しない。
if [ -z "${DATABASE_URL:-}" ] && [ -f "${SPIKE_BACKEND_DIR}/.env.local" ]; then
  set -a
  # shellcheck disable=SC1091
  . "${SPIKE_BACKEND_DIR}/.env.local"
  set +a
fi

if [ "${OUTBOX_RETRY_FAILED:-0}" = "1" ]; then
  go -C "${SPIKE_BACKEND_DIR}" run ./cmd/outbox-worker --retry-failed
else
  go -C "${SPIKE_BACKEND_DIR}" run ./cmd/outbox-worker --once --limit "${LIMIT}"
fi
