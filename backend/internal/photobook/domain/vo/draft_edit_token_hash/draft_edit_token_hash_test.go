package draft_edit_token_hash_test

import (
	"crypto/sha256"
	"errors"
	"testing"

	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
)

func TestOf(t *testing.T) {
	t.Parallel()

	tok1, err := draft_edit_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	tok2, err := draft_edit_token.Generate()
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
			description: "Given: token, When: Of, Then: Bytes() が 32 バイト",
			check: func(t *testing.T) {
				if l := len(draft_edit_token_hash.Of(tok1).Bytes()); l != 32 {
					t.Fatalf("len = %d want 32", l)
				}
			},
		},
		{
			name:        "正常_idempotent",
			description: "Given: 同じ token を 2 回, When: Of, Then: Equal=true",
			check: func(t *testing.T) {
				h1 := draft_edit_token_hash.Of(tok1)
				h2 := draft_edit_token_hash.Of(tok1)
				if !h1.Equal(h2) {
					t.Fatalf("idempotent failed")
				}
			},
		},
		{
			name:        "正常_異なるtokenから異なるhash",
			description: "Given: 別 2 token, When: Of, Then: Equal=false",
			check: func(t *testing.T) {
				if draft_edit_token_hash.Of(tok1).Equal(draft_edit_token_hash.Of(tok2)) {
					t.Fatalf("collision")
				}
			},
		},
		{
			name:        "正常_SHA-256と一致",
			description: "Given: token, When: Of, Then: sha256.Sum256 と一致",
			check: func(t *testing.T) {
				raw := tok1.Reveal()
				want := sha256.Sum256(raw[:])
				got := draft_edit_token_hash.Of(tok1).Bytes()
				for i, b := range want {
					if got[i] != b {
						t.Fatalf("mismatch at %d", i)
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
		{name: "正常_32B", description: "Given: 32B, When: FromBytes, Then: OK", input: make([]byte, 32)},
		{name: "異常_31B", description: "Given: 31B, When: FromBytes, Then: ErrInvalidLength", input: make([]byte, 31), wantErr: draft_edit_token_hash.ErrInvalidLength},
		{name: "異常_33B", description: "Given: 33B, When: FromBytes, Then: ErrInvalidLength", input: make([]byte, 33), wantErr: draft_edit_token_hash.ErrInvalidLength},
		{name: "異常_空", description: "Given: 空, When: FromBytes, Then: ErrInvalidLength", input: []byte{}, wantErr: draft_edit_token_hash.ErrInvalidLength},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := draft_edit_token_hash.FromBytes(tt.input)
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
