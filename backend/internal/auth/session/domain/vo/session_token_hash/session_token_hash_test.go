package session_token_hash_test

import (
	"crypto/sha256"
	"errors"
	"testing"

	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
)

func TestOf(t *testing.T) {
	t.Parallel()

	tok1, err := session_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	tok2, err := session_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	tests := []struct {
		name        string
		description string
		check       func(t *testing.T)
	}{
		{
			name:        "正常_長さ32B",
			description: "Given: 任意の SessionToken, When: Of, Then: Bytes() の長さが 32",
			check: func(t *testing.T) {
				h := session_token_hash.Of(tok1)
				if got := len(h.Bytes()); got != 32 {
					t.Fatalf("len = %d, want 32", got)
				}
			},
		},
		{
			name:        "正常_idempotent",
			description: "Given: 同じ SessionToken を 2 回 Of, When: Equal 比較, Then: 一致",
			check: func(t *testing.T) {
				h1 := session_token_hash.Of(tok1)
				h2 := session_token_hash.Of(tok1)
				if !h1.Equal(h2) {
					t.Fatalf("idempotent failed")
				}
			},
		},
		{
			name:        "正常_異なるtokenから異なるhash",
			description: "Given: 別の 2 つの SessionToken, When: Of, Then: Equal=false",
			check: func(t *testing.T) {
				h1 := session_token_hash.Of(tok1)
				h2 := session_token_hash.Of(tok2)
				if h1.Equal(h2) {
					t.Fatalf("collision: 別 token なのに同じ hash")
				}
			},
		},
		{
			name:        "正常_SHA-256と一致",
			description: "Given: 既知の raw, When: Of, Then: crypto/sha256.Sum256 と一致",
			check: func(t *testing.T) {
				raw := tok1.Reveal()
				want := sha256.Sum256(raw[:])
				h := session_token_hash.Of(tok1)
				got := h.Bytes()
				for i, b := range want {
					if got[i] != b {
						t.Fatalf("mismatch at %d: got=%x want=%x", i, got[i], b)
					}
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.check)
	}
}

func TestFromBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		input       []byte
		wantErr     error
	}{
		{
			name:        "正常_32B",
			description: "Given: 32 バイト, When: FromBytes, Then: エラーなし",
			input:       make([]byte, 32),
			wantErr:     nil,
		},
		{
			name:        "異常_31B",
			description: "Given: 31 バイト, When: FromBytes, Then: ErrInvalidLength",
			input:       make([]byte, 31),
			wantErr:     session_token_hash.ErrInvalidLength,
		},
		{
			name:        "異常_33B",
			description: "Given: 33 バイト, When: FromBytes, Then: ErrInvalidLength",
			input:       make([]byte, 33),
			wantErr:     session_token_hash.ErrInvalidLength,
		},
		{
			name:        "異常_空",
			description: "Given: 空, When: FromBytes, Then: ErrInvalidLength",
			input:       []byte{},
			wantErr:     session_token_hash.ErrInvalidLength,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := session_token_hash.FromBytes(tt.input)
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
