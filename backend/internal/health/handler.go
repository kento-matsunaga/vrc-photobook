// Package health はヘルスチェックハンドラを提供する。
//
// `/health` は process liveness のみを返す（Cloud Run / 本番監視 / startup probe / liveness probe 用）。
// `/healthz` は使わない。Cloud Run / Google Frontend が `/healthz` を intercept する事象を確認済（M1）。
// 詳細: harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md / docs/adr/0001-tech-stack.md
//
// `/readyz` は readiness を返す。PR2 段階では DB 接続を未実装のため、常に
// 503 `db_not_configured` を返す。PR3 で pgx pool を導入した時点で以下に拡張する:
//   - pool == nil           → 503 `db_not_configured`（現状維持）
//   - pool.Ping(ctx) 失敗   → 503 `db_unreachable`
//   - 成功                   → 200 `ready`
package health

import "net/http"

// Health は process liveness を返す（DB 接続は見ない）。
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// Ready は readiness を返す。
//
// PR2 時点: DB 接続未実装のため、常に 503 `db_not_configured`。
// PR3 で pgx pool を引数として受け取り、状態に応じて分岐するハンドラに置き換える。
func Ready(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"status":"db_not_configured"}`))
}
