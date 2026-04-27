package ogp_failure_reason_test

import (
	"errors"
	"strings"
	"testing"

	"vrcpb/backend/internal/ogp/domain/vo/ogp_failure_reason"
)

func TestSanitize(t *testing.T) {
	long := strings.Repeat("x", 500)
	tests := []struct {
		name        string
		description string
		err         error
		wantPrefix  string
		wantNotIn   []string
		wantMaxLen  int
	}{
		{
			name:        "正常_短い通常エラーはそのまま返す",
			description: "Given: 短い msg, When: sanitize, Then: そのまま",
			err:         errors.New("render failed: cover decode error"),
			wantMaxLen:  ogp_failure_reason.MaxLen,
		},
		{
			name:        "正常_200超は切詰",
			description: "Given: 500文字, When: sanitize, Then: 200文字以下",
			err:         errors.New(long),
			wantMaxLen:  ogp_failure_reason.MaxLen,
		},
		{
			name:        "異常_DATABASE_URL含みでredact",
			description: "Given: DSN付きエラー, When: sanitize, Then: [REDACTED]",
			err:         errors.New("dial postgres://USER:PASS@h/db failed"),
			wantPrefix:  "[REDACTED]",
			wantNotIn:   []string{"postgres://", "USER", "PASS"},
			wantMaxLen:  ogp_failure_reason.MaxLen,
		},
		{
			name:        "異常_presigned含みでredact",
			description: "Given: presigned URL含みmsg, When: sanitize, Then: [REDACTED]",
			err:         errors.New("got 403 from presigned URL https://r2/..."),
			wantPrefix:  "[REDACTED]",
			wantNotIn:   []string{"presigned", "https://r2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ogp_failure_reason.Sanitize(tt.err)
			s := got.String()
			if tt.wantPrefix != "" && !strings.HasPrefix(s, tt.wantPrefix) {
				t.Errorf("prefix mismatch: got=%q want prefix=%q", s, tt.wantPrefix)
			}
			if tt.wantMaxLen > 0 && len(s) > tt.wantMaxLen {
				t.Errorf("len(got)=%d > max %d", len(s), tt.wantMaxLen)
			}
			for _, ng := range tt.wantNotIn {
				if strings.Contains(s, ng) {
					t.Errorf("forbidden token leaked: %q in %q", ng, s)
				}
			}
		})
	}
}

func TestSanitize_NilReturnsZero(t *testing.T) {
	got := ogp_failure_reason.Sanitize(nil)
	if !got.IsZero() {
		t.Errorf("expected zero VO, got %q", got.String())
	}
}

func TestFromTrustedString_Limit(t *testing.T) {
	if _, err := ogp_failure_reason.FromTrustedString(strings.Repeat("a", 201)); err == nil {
		t.Errorf("expected error for 201 chars, got nil")
	}
	if _, err := ogp_failure_reason.FromTrustedString(strings.Repeat("a", 200)); err != nil {
		t.Errorf("unexpected error for 200 chars: %v", err)
	}
}
