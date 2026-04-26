// Package draft_edit_token_hash は draft_edit_token_hash 列の VO。
//
// SHA-256(DraftEditToken) で 32 バイト固定。Cookie / URL には乗らない、DB の hash 比較にのみ使う。
//
// セキュリティ:
//   - String() を意図的に実装しない（hex / base64 表現でログ出力させない）
package draft_edit_token_hash

import (
	"crypto/sha256"
	"errors"

	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token"
)

var ErrInvalidLength = errors.New("draft edit token hash must be 32 bytes")

const hashLen = 32

// DraftEditTokenHash は SHA-256(DraftEditToken) の 32 バイト固定値。
type DraftEditTokenHash struct {
	v [hashLen]byte
}

// Of は DraftEditToken を SHA-256 で hash 化する。
func Of(t draft_edit_token.DraftEditToken) DraftEditTokenHash {
	raw := t.Reveal()
	sum := sha256.Sum256(raw[:])
	return DraftEditTokenHash{v: sum}
}

// FromBytes は DB から取り出した 32 バイトの bytea を DraftEditTokenHash に復元する。
func FromBytes(b []byte) (DraftEditTokenHash, error) {
	if len(b) != hashLen {
		return DraftEditTokenHash{}, ErrInvalidLength
	}
	var h DraftEditTokenHash
	copy(h.v[:], b)
	return h, nil
}

// Bytes は内部の 32 バイトのコピーを返す。永続化層との境界でのみ使用する。
func (h DraftEditTokenHash) Bytes() []byte {
	out := make([]byte, hashLen)
	copy(out, h.v[:])
	return out
}

// Equal は値による等価判定。
func (h DraftEditTokenHash) Equal(other DraftEditTokenHash) bool {
	return h.v == other.v
}
