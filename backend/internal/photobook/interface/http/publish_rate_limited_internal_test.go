// PR36 commit 3.5: writePublishRateLimited（unexported）の単体テスト。
//
// 同 package _internal_test で publish handler の 429 出力契約を固定する。
package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWritePublishRateLimited(t *testing.T) {
	tests := []struct {
		name              string
		description       string
		inRetryAfter      int
		wantRetryAfterHdr string
		wantBody          string
	}{
		{
			name:              "正常_3600秒",
			description:       "Given: 3600（1 時間 5 冊上限の reset 待ち最大）, Then: Retry-After:3600",
			inRetryAfter:      3600,
			wantRetryAfterHdr: "3600",
			wantBody:          `{"status":"rate_limited","retry_after_seconds":3600}`,
		},
		{
			name:              "正常_最低1秒",
			description:       "Given: 0, Then: Retry-After:1（底上げ）",
			inRetryAfter:      0,
			wantRetryAfterHdr: "1",
			wantBody:          `{"status":"rate_limited","retry_after_seconds":1}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writePublishRateLimited(rec, tt.inRetryAfter)
			if rec.Code != http.StatusTooManyRequests {
				t.Errorf("status = %d want 429", rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != tt.wantRetryAfterHdr {
				t.Errorf("Retry-After = %q want %q", got, tt.wantRetryAfterHdr)
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store, must-revalidate" {
				t.Errorf("Cache-Control = %q", got)
			}
			if got := rec.Header().Get("X-Robots-Tag"); got != "noindex, nofollow" {
				t.Errorf("X-Robots-Tag = %q", got)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Errorf("Content-Type = %q", got)
			}
			body := rec.Body.String()
			if body != tt.wantBody {
				t.Errorf("body = %q want %q", body, tt.wantBody)
			}
			// 漏洩 grep
			for _, leak := range []string{
				`"scope_hash"`, `"count"`, `"limit"`, `"ip_hash"`, `"cookie"`, `"salt"`, `"secret"`,
				"scope_hash=", "count=", "ip_hash=", "Cookie:", "salt=", "secret=",
			} {
				if strings.Contains(body, leak) {
					t.Errorf("body contains forbidden token %q: %q", leak, body)
				}
			}
		})
	}
}
