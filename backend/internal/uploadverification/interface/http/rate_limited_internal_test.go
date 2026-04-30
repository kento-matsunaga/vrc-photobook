// PR36 commit 3.5: writeRateLimited（unexported）の単体テスト。
//
// 同 package _internal_test で handler の 429 出力契約を固定する。
// Cloudflare 公式の Retry-After header / Cache-Control / X-Robots-Tag を検証。
// scope_hash / count / IP / Cookie / Secret 値が body / header に出ないことも確認。
package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteRateLimited(t *testing.T) {
	tests := []struct {
		name              string
		description       string
		inRetryAfter      int
		wantRetryAfterHdr string
		wantBody          string
	}{
		{
			name:              "正常_120秒",
			description:       "Given: 120, Then: Retry-After:120 + body",
			inRetryAfter:      120,
			wantRetryAfterHdr: "120",
			wantBody:          `{"status":"rate_limited","retry_after_seconds":120}`,
		},
		{
			name:              "正常_最低1秒に底上げ",
			description:       "Given: 0, Then: Retry-After:1（負/0 を底上げ）",
			inRetryAfter:      0,
			wantRetryAfterHdr: "1",
			wantBody:          `{"status":"rate_limited","retry_after_seconds":1}`,
		},
		{
			name:              "正常_負も1秒底上げ",
			description:       "Given: -3, Then: Retry-After:1",
			inRetryAfter:      -3,
			wantRetryAfterHdr: "1",
			wantBody:          `{"status":"rate_limited","retry_after_seconds":1}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeRateLimited(rec, tt.inRetryAfter)
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
			// 漏洩 grep: 値埋め込みパターン
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
