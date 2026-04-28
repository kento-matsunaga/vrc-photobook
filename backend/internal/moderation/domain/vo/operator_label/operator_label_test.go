package operator_label_test

import (
	"errors"
	"strings"
	"testing"

	"vrcpb/backend/internal/moderation/domain/vo/operator_label"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		wantErr     bool
	}{
		{name: "正常_最短3文字", description: "Given: 'abc', When: Parse, Then: ok", in: "abc"},
		{name: "正常_数字混在", description: "Given: 'ops-1', When: Parse, Then: ok", in: "ops-1"},
		{name: "正常_dot", description: "Given: 'legal.team', When: Parse, Then: ok", in: "legal.team"},
		{name: "正常_underscore", description: "Given: 'admin_2', When: Parse, Then: ok", in: "admin_2"},
		{name: "正常_最大長64", description: "Given: 64文字, When: Parse, Then: ok", in: "a" + strings.Repeat("b", 62) + "z"},
		{name: "異常_2文字以下", description: "Given: 'ab', When: Parse, Then: error", in: "ab", wantErr: true},
		{name: "異常_先頭symbol", description: "Given: '-x', When: Parse, Then: error", in: "-x1", wantErr: true},
		{name: "異常_末尾symbol", description: "Given: 'x-', When: Parse, Then: error", in: "x1-", wantErr: true},
		{name: "異常_中間に空白", description: "Given: 'a b', When: Parse, Then: error", in: "a b", wantErr: true},
		{name: "異常_中間にカンマ", description: "Given: 'a,b', When: Parse, Then: error", in: "a,b", wantErr: true},
		{name: "異常_全角文字", description: "Given: 'あbc', When: Parse, Then: error", in: "あbc", wantErr: true},
		{name: "異常_65文字", description: "Given: 65文字, When: Parse, Then: error", in: "a" + strings.Repeat("b", 63) + "z", wantErr: true},
		{name: "異常_空文字", description: "Given: '', When: Parse, Then: error", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := operator_label.Parse(tt.in)
			if tt.wantErr {
				if !errors.Is(err, operator_label.ErrInvalidOperatorLabel) {
					t.Errorf("expected ErrInvalidOperatorLabel, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.String() != tt.in {
				t.Errorf("got %q want %q", got.String(), tt.in)
			}
		})
	}
}
