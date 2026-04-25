// Package health はヘルスチェックハンドラを提供する。
//
// `/health` は process liveness のみを返す（Cloud Run / 本番監視 / startup probe / liveness probe 用）。
// `/healthz` は使わない。Cloud Run / Google Frontend が `/healthz` を intercept する事象を確認済（M1）。
// 詳細: harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md / docs/adr/0001-tech-stack.md
//
// PR1: `/health` のみ。`/readyz`（DB 接続込みの readiness）は PR2 以降で追加する。
package health

import "net/http"

// Health は process liveness を返す（DB 接続は見ない）。
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
