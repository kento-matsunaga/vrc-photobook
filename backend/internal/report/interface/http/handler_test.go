package http

import (
	"net/http/httptest"
	"testing"
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
