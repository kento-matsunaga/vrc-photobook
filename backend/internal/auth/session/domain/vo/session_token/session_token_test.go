package session_token_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
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
			description: "Given: なし, When: Generate を呼ぶ, Then: 43 文字の base64url（パディングなし）が返る",
			runs:        1,
		},
		{
			name:        "正常_衝突なし1000回",
			description: "Given: なし, When: Generate を 1000 回呼ぶ, Then: すべて異なる値（弱い衝突回帰チェック）",
			runs:        1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(map[string]struct{}, tt.runs)
			for i := 0; i < tt.runs; i++ {
				tok, err := session_token.Generate()
				if err != nil {
					t.Fatalf("Generate: %v", err)
				}
				enc := tok.Encode()
				if got, want := len(enc), 43; got != want {
					t.Fatalf("encoded length = %d, want %d", got, want)
				}
				if tok.IsZero() {
					t.Fatalf("token must not be zero")
				}
				if _, dup := seen[enc]; dup {
					t.Fatalf("duplicate token at run %d", i)
				}
				seen[enc] = struct{}{}
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	valid, err := session_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	validEncoded := valid.Encode()

	tests := []struct {
		name        string
		description string
		input       string
		wantErr     error
	}{
		{
			name:        "正常_有効な43文字",
			description: "Given: Generate→Encode した 43 文字, When: Parse, Then: エラーなし",
			input:       validEncoded,
			wantErr:     nil,
		},
		{
			name:        "異常_長さ違い",
			description: "Given: 42 文字, When: Parse, Then: ErrInvalidLength",
			input:       validEncoded[:42],
			wantErr:     session_token.ErrInvalidLength,
		},
		{
			name:        "異常_base64url外文字混入",
			description: "Given: 43 文字だが + 含む（standard base64 用）, When: Parse, Then: ErrInvalidLength",
			input:       "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA+/", // 43 文字、+ / は raw URL では不可
			wantErr:     session_token.ErrInvalidLength,
		},
		{
			name:        "異常_空文字",
			description: "Given: 空, When: Parse, Then: ErrInvalidLength",
			input:       "",
			wantErr:     session_token.ErrInvalidLength,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := session_token.Parse(tt.input)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
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

	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "正常_EncodeしてParseすると同値",
			description: "Given: 生成した SessionToken, When: Encode→Parse, Then: Reveal の結果が一致",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig, err := session_token.Generate()
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			parsed, err := session_token.Parse(orig.Encode())
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if orig.Reveal() != parsed.Reveal() {
				t.Fatalf("roundtrip mismatch")
			}
		})
	}
}
