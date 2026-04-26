package manage_url_token_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("正常_43文字_衝突なし1000回", func(t *testing.T) {
		// Given: なし, When: Generate を 1000 回, Then: 全 43 文字 / 衝突なし
		seen := make(map[string]struct{}, 1000)
		for i := 0; i < 1000; i++ {
			tok, err := manage_url_token.Generate()
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			enc := tok.Encode()
			if got := len(enc); got != 43 {
				t.Fatalf("len = %d", got)
			}
			if tok.IsZero() {
				t.Fatalf("zero")
			}
			if _, dup := seen[enc]; dup {
				t.Fatalf("duplicate at %d", i)
			}
			seen[enc] = struct{}{}
		}
	})
}

func TestParse(t *testing.T) {
	t.Parallel()

	valid, err := manage_url_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	tests := []struct {
		name        string
		description string
		input       string
		wantErr     error
	}{
		{name: "正常_有効な43文字", description: "Given: encoded raw, When: Parse, Then: OK", input: valid.Encode()},
		{name: "異常_長さ違い", description: "Given: 42 文字, When: Parse, Then: ErrInvalidLength", input: valid.Encode()[:42], wantErr: manage_url_token.ErrInvalidLength},
		{name: "異常_範囲外文字", description: "Given: + / 含む 43 文字, When: Parse, Then: ErrInvalidLength", input: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA+/", wantErr: manage_url_token.ErrInvalidLength},
		{name: "異常_空文字", description: "Given: 空, When: Parse, Then: ErrInvalidLength", input: "", wantErr: manage_url_token.ErrInvalidLength},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manage_url_token.Parse(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
		})
	}
}
