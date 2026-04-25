#!/usr/bin/env bash
# turnstile-consume-race.sh
#
# 100 並列で /sandbox/upload-intent/consume を叩き、
# 「成功 == AllowedIntentCount（既定 20）/ 失敗 == 100 - AllowedIntentCount（80）」
# を確認するレース検証スクリプト。
#
# 前提:
#   - PostgreSQL と spike backend が起動済みであること
#   - .env.example の値で TURNSTILE_SECRET_KEY="" （mock モード）かつ
#     UPLOAD_VERIFICATION_INTENT_LIMIT=20 のとき本スクリプトは検証目的に合致する
#
# 使い方:
#   ./scripts/turnstile-consume-race.sh [BASE_URL]
#
# 既定 BASE_URL は http://localhost:8090
#
# セキュリティ方針:
#   - secret / token を echo しない（出力する集計は成功/失敗カウントのみ）
#   - 取得した verification_session_token は変数に閉じ込め、画面・ログには出さない
#   - 結果ファイルは集計 JSON 列だけ保持し、成功時に削除する

set -euo pipefail

BASE_URL="${1:-http://localhost:8090}"
PHOTOBOOK_ID="11111111-1111-1111-1111-111111111111"
PARALLEL=100

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

echo "[1/3] Turnstile siteverify でセッション発行..."
verify_response=$(curl -fsS -X POST "${BASE_URL}/sandbox/turnstile/verify" \
  -H 'Content-Type: application/json' \
  -d "$(cat <<EOF
{"turnstile_token":"DUMMY_OK_${RANDOM}","photobook_id":"${PHOTOBOOK_ID}"}
EOF
)")

allowed=$(printf '%s' "${verify_response}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["allowed_intent_count"])')
session_token=$(printf '%s' "${verify_response}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["verification_session_token"])')

echo "  allowed_intent_count = ${allowed}"
echo "  expected: success=${allowed} fail=$((PARALLEL - allowed))"

echo "[2/3] ${PARALLEL} 並列 consume を発射..."

# 並列 curl をバックグラウンドで起動。-o /dev/null でレスポンスボディは捨て、
# -w '%{http_code}' で HTTP ステータスのみ収集（token は出力しない）。
for i in $(seq 1 "${PARALLEL}"); do
  (
    code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "${BASE_URL}/sandbox/upload-intent/consume" \
      -H 'Content-Type: application/json' \
      -d "{\"verification_session_token\":\"${session_token}\",\"photobook_id\":\"${PHOTOBOOK_ID}\"}") || code="ERR"
    echo "${code}" >> "${WORKDIR}/codes.txt"
  ) &
done
# wait は子プロセスの非 0 終了を伝播させるため、set -e を一時無効化して結果集計に進む
set +e
wait
set -e

success_count=$(grep -c '^200$' "${WORKDIR}/codes.txt" || true)
forbidden_count=$(grep -c '^403$' "${WORKDIR}/codes.txt" || true)
# grep -v は不一致 0 行のとき終了コード 1 を返すため、pipefail/set -e に巻き込まれないよう || true
other_count=$( (grep -vE '^(200|403)$' "${WORKDIR}/codes.txt" || true) | wc -l | awk '{print $1}')

echo "[3/3] 集計"
echo "  HTTP 200 (consumed): ${success_count}"
echo "  HTTP 403 (rejected): ${forbidden_count}"
echo "  その他            : ${other_count}"

expected_fail=$((PARALLEL - allowed))
if [ "${success_count}" -eq "${allowed}" ] && [ "${forbidden_count}" -eq "${expected_fail}" ] && [ "${other_count}" -eq 0 ]; then
  echo "PASS: success=${success_count} / forbidden=${forbidden_count} (期待値と一致)"
  exit 0
else
  echo "FAIL: 期待値 (success=${allowed} / forbidden=${expected_fail}) と一致しません"
  exit 1
fi
