// Package manage_url_token_hash は manage_url_token_hash 列の VO。
//
// SHA-256(ManageUrlToken) で 32 バイト固定。
package manage_url_token_hash

import (
	"crypto/sha256"
	"errors"

	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
)

var ErrInvalidLength = errors.New("manage url token hash must be 32 bytes")

const hashLen = 32

// ManageUrlTokenHash は SHA-256(ManageUrlToken) の 32 バイト固定値。
type ManageUrlTokenHash struct {
	v [hashLen]byte
}

// Of は ManageUrlToken を SHA-256 で hash 化する。
func Of(t manage_url_token.ManageUrlToken) ManageUrlTokenHash {
	raw := t.Reveal()
	sum := sha256.Sum256(raw[:])
	return ManageUrlTokenHash{v: sum}
}

// FromBytes は DB から取り出した 32 バイトの bytea を ManageUrlTokenHash に復元する。
func FromBytes(b []byte) (ManageUrlTokenHash, error) {
	if len(b) != hashLen {
		return ManageUrlTokenHash{}, ErrInvalidLength
	}
	var h ManageUrlTokenHash
	copy(h.v[:], b)
	return h, nil
}

// Bytes は内部の 32 バイトのコピーを返す。永続化層との境界でのみ使用する。
func (h ManageUrlTokenHash) Bytes() []byte {
	out := make([]byte, hashLen)
	copy(out, h.v[:])
	return out
}

func (h ManageUrlTokenHash) Equal(other ManageUrlTokenHash) bool {
	return h.v == other.v
}
