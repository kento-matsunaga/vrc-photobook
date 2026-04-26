package verification_session_token_test

import (
	"errors"
	"strings"
	"testing"

	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		runs int
	}{
		{name: "正常_単発生成", runs: 1},
		{name: "正常_衝突なし1000回", runs: 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(map[string]struct{}, tt.runs)
			for i := 0; i < tt.runs; i++ {
				tok, err := verification_session_token.Generate()
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
	valid, _ := verification_session_token.Generate()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "正常_43charsをencode/decode", input: valid.Encode()},
		{name: "異常_42chars", input: strings.Repeat("a", 42), wantErr: true},
		{name: "異常_44chars", input: strings.Repeat("a", 44), wantErr: true},
		{name: "異常_invalid_base64", input: strings.Repeat("!", 43), wantErr: true},
		{name: "異常_空文字", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := verification_session_token.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && tt.input == strings.Repeat("a", 42) {
				if !errors.Is(err, verification_session_token.ErrInvalidLength) {
					t.Errorf("err = %v want ErrInvalidLength", err)
				}
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	a, _ := verification_session_token.Generate()
	b, err := verification_session_token.Parse(a.Encode())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if a.Encode() != b.Encode() {
		t.Errorf("encode roundtrip mismatch")
	}
}
