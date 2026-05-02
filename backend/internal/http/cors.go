// Package http (CORS middleware).
//
// 設計参照:
//   - docs/plan/m2-r2-presigned-url-plan.md §10
//   - PR12 で ALLOWED_ORIGINS env を Cloud Run に注入済
//
// Backend CORS:
//   - Origin: ALLOWED_ORIGINS（カンマ区切り、既定 https://app.vrc-photobook.com）
//   - Methods: GET / POST / PATCH / DELETE / OPTIONS
//   - Headers: Content-Type / Authorization
//   - Credentials: true（HttpOnly Cookie 送信のため）
//   - MaxAge: 600 秒
//
// PATCH / DELETE は Edit UI の mutation（settings 保存 / caption / reorder /
// cover 設定 / cover クリア / photo 削除）で使用する。preflight Allow-Methods
// に含まれていないとブラウザが本体送信を中止し、Frontend 側に generic な
// network error として伝わる（2026-05-03 STOP α 調査で `/settings` の OPTIONS
// 200 + PATCH 0 件の挙動として観測）。
package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/cors"
)

// NewCORS は ALLOWED_ORIGINS 文字列から chi-cors middleware を組み立てる。
func NewCORS(allowedOrigins string) func(http.Handler) http.Handler {
	origins := splitAndTrim(allowedOrigins)
	if len(origins) == 0 {
		origins = []string{"https://app.vrc-photobook.com"}
	}
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{},
		AllowCredentials: true,
		MaxAge:           600,
	})
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
