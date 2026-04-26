// Package http (CORS middleware).
//
// 設計参照:
//   - docs/plan/m2-r2-presigned-url-plan.md §10
//   - PR12 で ALLOWED_ORIGINS env を Cloud Run に注入済
//
// Backend CORS:
//   - Origin: ALLOWED_ORIGINS（カンマ区切り、既定 https://app.vrc-photobook.com）
//   - Methods: GET / POST / OPTIONS
//   - Headers: Content-Type / Authorization
//   - Credentials: true（HttpOnly Cookie 送信のため）
//   - MaxAge: 600 秒
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
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
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
