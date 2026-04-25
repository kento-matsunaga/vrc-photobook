// Package http は HTTP ルーターを組み立てる。
//
// PR1: 最小ルートとして `/health` のみ。
// PR2 以降で `/readyz` / middleware / 各集約のルートを追加する。
package http

import (
	"github.com/go-chi/chi/v5"

	"vrcpb/backend/internal/health"
)

// NewRouter は API サーバの chi ルーターを返す。
func NewRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/health", health.Health)
	return r
}
