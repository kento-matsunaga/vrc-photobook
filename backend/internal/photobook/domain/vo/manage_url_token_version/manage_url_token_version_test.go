package manage_url_token_version_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_version"
)

func TestNewAndIncrement(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		description string
		input       int
		wantInt     int
		wantErr     error
	}{
		{name: "正常_0", description: "Given: 0, When: New, Then: 0 / Increment で 1", input: 0, wantInt: 0},
		{name: "正常_5", description: "Given: 5, When: New, Then: 5 / Increment で 6", input: 5, wantInt: 5},
		{name: "異常_負値", description: "Given: -1, When: New, Then: ErrNegativeVersion", input: -1, wantErr: manage_url_token_version.ErrNegativeVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := manage_url_token_version.New(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if v.Int() != tt.wantInt {
				t.Errorf("Int = %d want %d", v.Int(), tt.wantInt)
			}
			if v.Increment().Int() != tt.wantInt+1 {
				t.Errorf("Increment failed")
			}
		})
	}
}

func TestZero(t *testing.T) {
	t.Parallel()
	t.Run("正常_Zero", func(t *testing.T) {
		// Given: なし, When: Zero, Then: Int=0
		if got := manage_url_token_version.Zero().Int(); got != 0 {
			t.Fatalf("Zero.Int = %d want 0", got)
		}
	})
}
