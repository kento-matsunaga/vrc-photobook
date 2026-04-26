package verification_session_token_hash_test

import (
	"bytes"
	"errors"
	"testing"

	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token_hash"
)

func TestOfAndBytes(t *testing.T) {
	t.Parallel()
	tok, err := verification_session_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	h := verification_session_token_hash.Of(tok)
	if got, want := len(h.Bytes()), 32; got != want {
		t.Errorf("hash len = %d want %d", got, want)
	}
	// 同じ token から同じ hash が出る（決定的）
	h2 := verification_session_token_hash.Of(tok)
	if !bytes.Equal(h.Bytes(), h2.Bytes()) {
		t.Errorf("hash not deterministic")
	}
}

func TestFromBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{name: "正常_32B", input: bytes.Repeat([]byte{0xab}, 32)},
		{name: "異常_31B", input: bytes.Repeat([]byte{0xab}, 31), wantErr: true},
		{name: "異常_33B", input: bytes.Repeat([]byte{0xab}, 33), wantErr: true},
		{name: "異常_空", input: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := verification_session_token_hash.FromBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, verification_session_token_hash.ErrInvalidLength) {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	t.Parallel()
	a, _ := verification_session_token.Generate()
	b, _ := verification_session_token.Generate()
	ha := verification_session_token_hash.Of(a)
	hb := verification_session_token_hash.Of(b)
	if ha.Equal(hb) {
		t.Errorf("different tokens should yield different hashes")
	}
	hac := verification_session_token_hash.Of(a)
	if !ha.Equal(hac) {
		t.Errorf("same tokens should yield equal hashes")
	}
}
