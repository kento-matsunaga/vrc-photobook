package ogp_status_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/ogp/domain/vo/ogp_status"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		input       string
		wantErr     bool
	}{
		{name: "正常_pending", description: "OK", input: "pending"},
		{name: "正常_generated", description: "OK", input: "generated"},
		{name: "正常_failed", description: "OK", input: "failed"},
		{name: "正常_fallback", description: "OK", input: "fallback"},
		{name: "正常_stale", description: "OK", input: "stale"},
		{name: "異常_未知の値", description: "Given: 'invalid', Then: error", input: "invalid", wantErr: true},
		{name: "異常_空文字", description: "空", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ogp_status.Parse(tt.input)
			if tt.wantErr {
				if err == nil || !errors.Is(err, ogp_status.ErrInvalidOgpStatus) {
					t.Errorf("err mismatch: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if s.String() != tt.input {
				t.Errorf("roundtrip mismatch: %q", s.String())
			}
		})
	}
}

func TestPredicates(t *testing.T) {
	if !ogp_status.Pending().IsPending() {
		t.Errorf("Pending().IsPending() should be true")
	}
	if !ogp_status.Generated().IsGenerated() {
		t.Errorf("Generated().IsGenerated() should be true")
	}
	if !ogp_status.Failed().IsFailed() {
		t.Errorf("Failed().IsFailed() should be true")
	}
	if !ogp_status.Fallback().IsFallback() {
		t.Errorf("Fallback().IsFallback() should be true")
	}
	if !ogp_status.Stale().IsStale() {
		t.Errorf("Stale().IsStale() should be true")
	}
}
