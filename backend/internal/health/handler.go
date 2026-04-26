// Package health はヘルスチェックハンドラを提供する。
//
// `/health` は process liveness のみを返す（Cloud Run / 本番監視 / startup probe / liveness probe 用）。
// `/healthz` は使わない。Cloud Run / Google Frontend が `/healthz` を intercept する事象を確認済（M1）。
// 詳細: harness/failure-log/2026-04-26_cloud-run-healthz-intercepted.md / docs/adr/0001-tech-stack.md
//
// `/readyz` は readiness を返す。
// PR3 段階では DB 接続有無に応じて以下を返す:
//   - pool == nil（DATABASE_URL 未設定） → 503 `db_not_configured`
//   - pool.Ping(ctx) 失敗                → 503 `db_unreachable`
//   - 成功                                → 200 `ready`
//
// セキュリティ: Ping エラー詳細はクライアントに返さない（DSN / 内部構造の漏洩抑止）。
// サーバ側 slog でのみ追跡する。
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Health は process liveness を返す（DB 接続は見ない）。
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// Ready は readiness を返す。pool が nil の場合は 503 db_not_configured を返す。
func Ready(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if pool == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"db_not_configured"}`))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			// エラー詳細はクライアントに返さない
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"db_unreachable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}
}
