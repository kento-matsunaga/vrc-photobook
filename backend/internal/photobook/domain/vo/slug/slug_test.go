package slug_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/photobook/domain/vo/slug"
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		description string
		input       string
		wantErr     error
	}{
		{name: "正常_最短12文字", description: "Given: 12 文字英数, When: Parse, Then: OK", input: "abc123def456"},
		{name: "正常_最長20文字", description: "Given: 20 文字英数ハイフン, When: Parse, Then: OK", input: "abc-def-ghi-1234567x"},
		{name: "正常_数字始まり", description: "Given: 数字始まり, When: Parse, Then: OK", input: "9abcdef12345"},
		{name: "異常_11文字", description: "Given: 11 文字, When: Parse, Then: ErrInvalidLength", input: "abc123def45", wantErr: slug.ErrInvalidLength},
		{name: "異常_21文字", description: "Given: 21 文字, When: Parse, Then: ErrInvalidLength", input: "abc-def-ghi-12345678x", wantErr: slug.ErrInvalidLength},
		{name: "異常_大文字", description: "Given: 大文字混入, When: Parse, Then: ErrInvalidFormat", input: "ABCdef123456", wantErr: slug.ErrInvalidFormat},
		{name: "異常_先頭ハイフン", description: "Given: 先頭ハイフン, When: Parse, Then: ErrInvalidFormat", input: "-abc12345678", wantErr: slug.ErrInvalidFormat},
		{name: "異常_末尾ハイフン", description: "Given: 末尾ハイフン, When: Parse, Then: ErrInvalidFormat", input: "abc12345678-", wantErr: slug.ErrInvalidFormat},
		{name: "異常_アンダースコア", description: "Given: _ 含む, When: Parse, Then: ErrInvalidFormat", input: "abc_def_1234", wantErr: slug.ErrInvalidFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := slug.Parse(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if s.String() != tt.input {
				t.Errorf("String = %q want %q", s.String(), tt.input)
			}
		})
	}
}
