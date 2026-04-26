package draft_edit_token_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		runs        int
	}{
		{
			name:        "正常_単発生成",
			description: "Given: なし, When: Generate, Then: 43 文字 base64url で IsZero=false",
			runs:        1,
		},
		{
			name:        "正常_衝突なし1000回",
			description: "Given: なし, When: Generate を 1000 回, Then: すべて異なる値",
			runs:        1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(map[string]struct{}, tt.runs)
			for i := 0; i < tt.runs; i++ {
				tok, err := draft_edit_token.Generate()
				if err != nil {
					t.Fatalf("Generate: %v", err)
				}
				enc := tok.Encode()
				if got, want := len(enc), 43; got != want {
					t.Fatalf("len = %d want %d", got, want)
				}
				if tok.IsZero() {
					t.Fatalf("must not be zero")
				}
				if _, dup := seen[enc]; dup {
					t.Fatalf("duplicate at run %d", i)
				}
				seen[enc] = struct{}{}
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	valid, err := draft_edit_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	tests := []struct {
		name        string
		description string
		input       string
		wantErr     error
	}{
		{
			name:        "正常_有効な43文字",
			description: "Given: Generate→Encode した値, When: Parse, Then: エラーなし",
			input:       valid.Encode(),
		},
		{
			name:        "異常_長さ違い",
			description: "Given: 42 文字, When: Parse, Then: ErrInvalidLength",
			input:       valid.Encode()[:42],
			wantErr:     draft_edit_token.ErrInvalidLength,
		},
		{
			name:        "異常_範囲外文字",
			description: "Given: + / 含む 43 文字, When: Parse, Then: ErrInvalidLength",
			input:       "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA+/",
			wantErr:     draft_edit_token.ErrInvalidLength,
		},
		{
			name:        "異常_空文字",
			description: "Given: 空, When: Parse, Then: ErrInvalidLength",
			input:       "",
			wantErr:     draft_edit_token.ErrInvalidLength,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := draft_edit_token.Parse(tt.input)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncodeRoundtrip(t *testing.T) {
	t.Parallel()
	t.Run("正常_EncodeしてParseすると同値", func(t *testing.T) {
		// Given: 生成した DraftEditToken, When: Encode→Parse, Then: Reveal が一致
		orig, err := draft_edit_token.Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		parsed, err := draft_edit_token.Parse(orig.Encode())
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if orig.Reveal() != parsed.Reveal() {
			t.Fatalf("roundtrip mismatch")
		}
	})
}
