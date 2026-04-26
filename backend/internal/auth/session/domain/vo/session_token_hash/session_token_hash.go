// Package session_token_hash は SessionTokenHash 値オブジェクトを提供する。
//
// SessionTokenHash は **DB に保存される値**。SHA-256(SessionToken) で 32 バイト固定。
// Cookie には乗らない、DB への INSERT / SELECT 条件にのみ使う。
//
// 設計参照:
//   - docs/adr/0003-frontend-token-session-flow.md
//   - docs/plan/m2-session-auth-implementation-plan.md §5
//
// セキュリティ:
//   - String() を意図的に実装しない（hex / base64 で hash をログに出さない）
//   - ログには「session_token_hash 出力禁止」（shared/logging.go）
//
// なお SHA-256 のソース raw token は 256bit エントロピーがあるため、
// ストレッチング・ソルト不要（ADR-0003）。
package session_token_hash

import (
	"crypto/sha256"
	"errors"

	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
)

// ErrInvalidLength は復元時にバイト長が 32 でなかったときのエラー。
var ErrInvalidLength = errors.New("session token hash must be 32 bytes")

// hashLen は SHA-256 の出力バイト数。
const hashLen = 32

// SessionTokenHash は SHA-256(raw SessionToken) の 32 バイト固定値。
type SessionTokenHash struct {
	v [hashLen]byte
}

// Of は SessionToken を SHA-256 で hash 化する。
func Of(t session_token.SessionToken) SessionTokenHash {
	raw := t.Reveal()
	sum := sha256.Sum256(raw[:])
	return SessionTokenHash{v: sum}
}

// FromBytes は DB から取り出した 32 バイトの bytea を SessionTokenHash に復元する。
func FromBytes(b []byte) (SessionTokenHash, error) {
	if len(b) != hashLen {
		return SessionTokenHash{}, ErrInvalidLength
	}
	var h SessionTokenHash
	copy(h.v[:], b)
	return h, nil
}

// Bytes は内部の 32 バイトのコピーを返す。永続化層との境界でのみ使用する。
//
// **ログには出さない**。SQL の引数 / sqlc 生成物への受け渡しのみで使う。
func (h SessionTokenHash) Bytes() []byte {
	out := make([]byte, hashLen)
	copy(out, h.v[:])
	return out
}

// Equal は値による等価判定。
func (h SessionTokenHash) Equal(other SessionTokenHash) bool {
	return h.v == other.v
}
