// Package httpx は HTTP まわりの最小ミドルウェア置き場。
// M1 PoC 専用。本実装には流用しない。
package httpx

import "net/http"

// CORS は許可オリジンのみに対して credentials 付きクロスオリジン要求を許可する
// 最小ミドルウェア。OPTIONS preflight を 204 で返す。
//
// セキュリティ:
//   - Access-Control-Allow-Origin にはリクエストの Origin を反射する（ホワイトリスト一致時のみ）
//   - 一致しない Origin / Origin 空に対しては CORS ヘッダを一切付けない
//   - `Access-Control-Allow-Credentials: true` を返すため、`*` ワイルドカードは絶対に使わない
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o != "" {
			allowed[o] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Add("Vary", "Origin")
				}
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				// preflight
				w.Header().Set("Access-Control-Allow-Methods",
					"GET, POST, PUT, PATCH, DELETE, OPTIONS")
				reqHeaders := r.Header.Get("Access-Control-Request-Headers")
				if reqHeaders != "" {
					w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
				} else {
					w.Header().Set("Access-Control-Allow-Headers",
						"Content-Type, Authorization")
				}
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
