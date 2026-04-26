// Package http は HTTP ルーターを組み立てる。
//
// PR2: `/health` + `/readyz`（DB 未実装時 503 固定）。
// PR3: `/readyz` を pool 状態に応じた分岐に置き換え。
// PR4 以降で middleware（CORS / Origin / RequestID / Recoverer / Timeout / Auth）と
// 各集約のルートを追加する。
package http

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/health"
)

// NewRouter は API サーバの chi ルーターを返す。
// pool は nil でも可（その場合 /readyz は 503 db_not_configured）。
func NewRouter(pool *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/health", health.Health)
	r.Get("/readyz", health.Ready(pool))
	return r
}
