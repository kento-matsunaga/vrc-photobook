// Package sandbox は M1 PoC 用の動作確認エンドポイント置き場。
// 本実装には流用しない。
package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DBPing は SELECT now() を実行して現在時刻を返す。
// pool が nil なら 503、クエリ失敗時は 500（詳細はクライアントに返さない）。
func DBPing(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if pool == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"db_not_configured"}`))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var now time.Time
		if err := pool.QueryRow(ctx, "SELECT now()").Scan(&now); err != nil {
			// エラー詳細はクライアントに出さない。サーバ側ログで追う。
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"db_query_failed"}`))
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"now": now.UTC().Format(time.RFC3339Nano),
		})
	}
}
