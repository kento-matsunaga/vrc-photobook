// Package http は HTTP ルーターを組み立てる。
//
// PR2: `/health` + `/readyz` を登録。
// PR3 以降で middleware（CORS / Origin / RequestID / Recoverer / Timeout / Auth）と
// 各集約のルートを追加する。
package http

import (
	"github.com/go-chi/chi/v5"

	"vrcpb/backend/internal/health"
)

// NewRouter は API サーバの chi ルーターを返す。
func NewRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/health", health.Health)
	r.Get("/readyz", health.Ready)
	return r
}
