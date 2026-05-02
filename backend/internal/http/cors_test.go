// CORS middleware test。
//
// 観点:
//   - preflight OPTIONS が Access-Control-Allow-Methods に PATCH / DELETE を含む
//   - Origin が許可された値のときのみ Access-Control-Allow-Origin を出す
//   - credentials: true（HttpOnly Cookie 送信）
//
// 2026-05-03 STOP α: PATCH / DELETE 不在で Edit UI 全 mutation が CORS preflight
// 阻止により沈黙失敗していた事故の再発防止 guard test。

package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewCORS_PreflightAllowsPATCHandDELETE(t *testing.T) {
	t.Parallel()

	// Edit UI mutation で実際に使う method 群（cors.go コメントと一致させる）。
	type tc struct {
		name        string
		description string
		method      string
	}
	cases := []tc{
		{
			name:        "正常_PATCH_preflight許可",
			description: "Given: ALLOWED_ORIGINS に許可 origin、When: OPTIONS preflight (Access-Control-Request-Method: PATCH), Then: Access-Control-Allow-Methods に PATCH 含む",
			method:      stdhttp.MethodPatch,
		},
		{
			name:        "正常_DELETE_preflight許可",
			description: "Given: ALLOWED_ORIGINS に許可 origin、When: OPTIONS preflight (Access-Control-Request-Method: DELETE), Then: Access-Control-Allow-Methods に DELETE 含む",
			method:      stdhttp.MethodDelete,
		},
		{
			name:        "正常_GET_preflight許可",
			description: "Given: 既存 GET 経路、When: preflight, Then: Allow-Methods に GET 含む（regression）",
			method:      stdhttp.MethodGet,
		},
		{
			name:        "正常_POST_preflight許可",
			description: "Given: 既存 POST 経路、When: preflight, Then: Allow-Methods に POST 含む（regression）",
			method:      stdhttp.MethodPost,
		},
	}

	const origin = "https://app.vrc-photobook.com"
	mw := NewCORS(origin)
	final := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
	})
	handler := mw(final)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(stdhttp.MethodOptions, "/api/photobooks/00000000-0000-0000-0000-000000000000/settings", nil)
			req.Header.Set("Origin", origin)
			req.Header.Set("Access-Control-Request-Method", c.method)
			req.Header.Set("Access-Control-Request-Headers", "Content-Type")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			allow := rec.Header().Get("Access-Control-Allow-Methods")
			if !containsMethod(allow, c.method) {
				t.Fatalf("Allow-Methods=%q want contains %q", allow, c.method)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Errorf("Allow-Origin=%q want %q", got, origin)
			}
			if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
				t.Errorf("Allow-Credentials=%q want true", got)
			}
		})
	}
}

func TestNewCORS_DefaultOriginWhenEmpty(t *testing.T) {
	t.Parallel()
	const origin = "https://app.vrc-photobook.com"
	mw := NewCORS("") // 空 → default https://app.vrc-photobook.com
	final := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
	})
	handler := mw(final)
	req := httptest.NewRequest(stdhttp.MethodOptions, "/", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", stdhttp.MethodPatch)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Errorf("default origin not applied: got=%q want=%q", got, origin)
	}
}

// containsMethod は comma-separated な Allow-Methods 文字列に target が含まれるかを返す。
func containsMethod(allow, target string) bool {
	for _, p := range strings.Split(allow, ",") {
		if strings.EqualFold(strings.TrimSpace(p), target) {
			return true
		}
	}
	return false
}
