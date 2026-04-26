// Package verification_session_token_hash は upload-verification token の SHA-256 hash VO。
//
// SHA-256(VerificationSessionToken) で 32 バイト固定。Cookie / URL には乗らない、
// DB の hash 比較にのみ使う。
//
// セキュリティ:
//   - String() を意図的に実装しない（hex / base64 表現でログ出力させない）
package verification_session_token_hash

import (
	"crypto/sha256"
	"errors"

	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token"
)

var ErrInvalidLength = errors.New("verification session token hash must be 32 bytes")

const hashLen = 32

// VerificationSessionTokenHash は SHA-256(VerificationSessionToken) の 32 バイト固定値。
type VerificationSessionTokenHash struct {
	v [hashLen]byte
}

// Of は VerificationSessionToken を SHA-256 で hash 化する。
func Of(t verification_session_token.VerificationSessionToken) VerificationSessionTokenHash {
	raw := t.Reveal()
	sum := sha256.Sum256(raw[:])
	return VerificationSessionTokenHash{v: sum}
}

// FromBytes は DB から取り出した 32 バイトの bytea を VerificationSessionTokenHash に復元する。
func FromBytes(b []byte) (VerificationSessionTokenHash, error) {
	if len(b) != hashLen {
		return VerificationSessionTokenHash{}, ErrInvalidLength
	}
	var h VerificationSessionTokenHash
	copy(h.v[:], b)
	return h, nil
}

// Bytes は内部の 32 バイトのコピーを返す。永続化層との境界でのみ使用する。
func (h VerificationSessionTokenHash) Bytes() []byte {
	out := make([]byte, hashLen)
	copy(out, h.v[:])
	return out
}

// Equal は値による等価判定。
func (h VerificationSessionTokenHash) Equal(other VerificationSessionTokenHash) bool {
	return h.v == other.v
}
