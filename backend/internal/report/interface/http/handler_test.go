package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// extractRemoteIP のテスト。Cf-Connecting-Ip 優先、X-Forwarded-For 末尾 fallback、
// RemoteAddr fallback の順序を確認する。
func TestExtractRemoteIPPriority(t *testing.T) {
	tests := []struct {
		name        string
		description string
		setup       func(r *httptest.ResponseRecorder, req *httptest.Server)
		cfHeader    string
		xffHeader   string
		remoteAddr  string
		want        string
	}{
		{
			name:        "正常_CfConnectingIp_優先",
			description: "Cf + XFF + RemoteAddr 全部あれば Cf を使う",
			cfHeader:    "203.0.113.1",
			xffHeader:   "10.0.0.1, 192.0.2.99",
			remoteAddr:  "127.0.0.1:8080",
			want:        "203.0.113.1",
		},
		{
			name:        "正常_XFF_先頭_fallback",
			description: "Cf 無し + XFF あれば XFF 先頭（client IP）を使う",
			xffHeader:   "203.0.113.5, 10.0.0.1",
			remoteAddr:  "127.0.0.1:8080",
			want:        "203.0.113.5",
		},
		{
			name:        "正常_RemoteAddr_fallback",
			description: "Cf / XFF 無しなら RemoteAddr の host 部",
			remoteAddr:  "203.0.113.9:54321",
			want:        "203.0.113.9",
		},
		{
			name:        "正常_Cf_前後空白trim",
			description: "Cf header の前後空白を trim",
			cfHeader:    "  203.0.113.1  ",
			remoteAddr:  "127.0.0.1:8080",
			want:        "203.0.113.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/public/photobooks/abc/reports", nil)
			if tt.cfHeader != "" {
				req.Header.Set("Cf-Connecting-Ip", tt.cfHeader)
			}
			if tt.xffHeader != "" {
				req.Header.Set("X-Forwarded-For", tt.xffHeader)
			}
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}
			got := extractRemoteIP(req)
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

// L4 多層防御 Turnstile ガード（`.agents/rules/turnstile-defensive-guard.md`）。
//
// 空白のみのトークンが UseCase / Cloudflare siteverify に渡らずに 400 で弾かれること
// を確認する。`PublicHandlers.handlers` を nil で構築し、Submit に到達したら nil
// 参照で panic するため、到達せず 400 が返ることでガードを保証する。
func TestSubmitReport_L4_BlankTurnstileToken_Rejected(t *testing.T) {
	tests := []struct {
		name        string
		description string
		token       string
	}{
		{
			name:        "異常_空文字tokenで400",
			description: "Given: turnstile_token=\"\", When: SubmitReport, Then: 400 invalid_payload",
			token:       "",
		},
		{
			name:        "異常_空白のみtokenで400",
			description: "Given: turnstile_token=\"   \", When: SubmitReport, Then: 400 invalid_payload",
			token:       "   ",
		},
		{
			name:        "異常_タブ改行のみtokenで400",
			description: "Given: turnstile_token=\"\\t\\n\", When: SubmitReport, Then: 400 invalid_payload",
			token:       "\t\n",
		},
	}

	h := NewPublicHandlers(nil) // handlers nil: L4 で 400 を返さなければ Submit で panic
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"reason":"other","detail":"","reporter_contact":"","turnstile_token":"` +
				escapeJSON(tt.token) + `"}`
			req := httptest.NewRequest(http.MethodPost,
				"/api/public/photobooks/uqfwfti7glarva5saj/reports",
				bytes.NewReader([]byte(body)))
			req.Header.Set("Content-Type", "application/json")
			// chi の URLParam は RouteContext から読まれる
			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("slug", "uqfwfti7glarva5saj")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

			rec := httptest.NewRecorder()
			h.SubmitReport(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d want %d (token=%q): %s",
					rec.Code, http.StatusBadRequest, tt.token, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "invalid_payload") {
				t.Errorf("body = %q want contains invalid_payload", rec.Body.String())
			}
		})
	}
}

// escapeJSON は JSON 文字列リテラル中に safe に埋め込めるよう最低限の escape を行う。
// テスト用なので "\\" / "\"" / "\t" / "\n" のみ対応。
func escapeJSON(s string) string {
	r := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\t", "\\t",
		"\n", "\\n",
	)
	return r.Replace(s)
}
