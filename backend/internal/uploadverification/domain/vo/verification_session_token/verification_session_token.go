// Package verification_session_token は upload-verification の raw session token VO。
//
// 設計参照:
//   - docs/plan/m2-upload-verification-plan.md §4
//   - docs/adr/0005-image-upload-flow.md §upload_verification_session の保存先
//
// 仕様:
//   - 32 byte（256bit）の暗号論的乱数
//   - base64url（padding なし）で 43 文字
//   - DB には raw を保存しない（hash のみ、別 VO）
//
// セキュリティ:
//   - String() / GoStringer は意図的に実装しない（不用意なログ出力を避ける）
//   - raw 取り出しは Reveal() / Encode() を明示呼び出し時のみ
package verification_session_token

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

const rawBytesLen = 32

// エラー。
var (
	ErrInvalidLength = errors.New("verification session token must be 43 base64url chars")
)

// VerificationSessionToken は upload-verification の raw session 値。
type VerificationSessionToken struct {
	raw [rawBytesLen]byte
}

// Generate は暗号論的乱数で新規 raw token を作る。
func Generate() (VerificationSessionToken, error) {
	var t VerificationSessionToken
	if _, err := rand.Read(t.raw[:]); err != nil {
		return VerificationSessionToken{}, fmt.Errorf("rand read: %w", err)
	}
	return t, nil
}

// Parse は base64url（padding なし、43 文字）の文字列を VerificationSessionToken に変換する。
func Parse(encoded string) (VerificationSessionToken, error) {
	if len(encoded) != 43 {
		return VerificationSessionToken{}, fmt.Errorf("%w: got %d", ErrInvalidLength, len(encoded))
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return VerificationSessionToken{}, fmt.Errorf("base64url decode: %w", err)
	}
	if len(decoded) != rawBytesLen {
		return VerificationSessionToken{}, ErrInvalidLength
	}
	var t VerificationSessionToken
	copy(t.raw[:], decoded)
	return t, nil
}

// Reveal は raw 32 バイトを返す。**ログに出さないこと**。
func (t VerificationSessionToken) Reveal() [rawBytesLen]byte { return t.raw }

// Encode は base64url（padding なし、43 文字）の文字列を返す。
func (t VerificationSessionToken) Encode() string {
	return base64.RawURLEncoding.EncodeToString(t.raw[:])
}

// IsZero は未初期化判定。
func (t VerificationSessionToken) IsZero() bool {
	for _, b := range t.raw {
		if b != 0 {
			return false
		}
	}
	return true
}
