package action_detail_test

import (
	"errors"
	"strings"
	"testing"

	"vrcpb/backend/internal/moderation/domain/vo/action_detail"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		wantPresent bool
		wantStr     string
		wantErr     bool
	}{
		{
			name:        "正常_空文字_None",
			description: "Given: '', When: Parse, Then: Present=false（DB NULL に対応）",
			in:          "",
			wantPresent: false,
			wantStr:     "",
		},
		{
			name:        "正常_短い文字列",
			description: "Given: '理由メモ', When: Parse, Then: Present=true",
			in:          "理由メモ",
			wantPresent: true,
			wantStr:     "理由メモ",
		},
		{
			name:        "正常_2000_rune_境界",
			description: "Given: 2000 rune, When: Parse, Then: ok",
			in:          strings.Repeat("a", 2000),
			wantPresent: true,
			wantStr:     strings.Repeat("a", 2000),
		},
		{
			name:        "異常_2001_rune_超過",
			description: "Given: 2001 rune, When: Parse, Then: ErrTooLong",
			in:          strings.Repeat("a", 2001),
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := action_detail.Parse(tt.in)
			if tt.wantErr {
				if !errors.Is(err, action_detail.ErrTooLong) {
					t.Errorf("expected ErrTooLong, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.Present() != tt.wantPresent {
				t.Errorf("Present()=%v want %v", got.Present(), tt.wantPresent)
			}
			if got.String() != tt.wantStr {
				t.Errorf("String()=%q want %q", got.String(), tt.wantStr)
			}
		})
	}
}
