package token_version_at_issue_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		input       int
		wantInt     int
		wantErr     error
	}{
		{
			name:        "正常_0",
			description: "Given: 0, When: New, Then: IsZero=true",
			input:       0,
			wantInt:     0,
		},
		{
			name:        "正常_1",
			description: "Given: 1, When: New, Then: Int=1",
			input:       1,
			wantInt:     1,
		},
		{
			name:        "正常_大きな値",
			description: "Given: 1_000_000, When: New, Then: 受理",
			input:       1_000_000,
			wantInt:     1_000_000,
		},
		{
			name:        "異常_負の値",
			description: "Given: -1, When: New, Then: ErrNegativeVersion",
			input:       -1,
			wantErr:     token_version_at_issue.ErrNegativeVersion,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := token_version_at_issue.New(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := tv.Int(); got != tt.wantInt {
				t.Errorf("Int = %d, want %d", got, tt.wantInt)
			}
			if got := tv.IsZero(); got != (tt.wantInt == 0) {
				t.Errorf("IsZero = %v, want %v", got, tt.wantInt == 0)
			}
		})
	}
}
