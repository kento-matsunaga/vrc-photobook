// Package manage_url_token は管理 URL に乗る raw 値の VO。
//
// 設計参照:
//   - docs/design/aggregates/photobook/ドメイン設計.md §4 / §13.1
//   - docs/adr/0003-frontend-token-session-flow.md
//   - docs/plan/m2-photobook-session-integration-plan.md §5
//
// 仕様:
//   - 256bit (32 バイト) 乱数を crypto/rand で生成
//   - base64url（パディングなし）43 文字
//   - DB には保存しない（保存対象は ManageUrlTokenHash）
//
// セキュリティ:
//   - String() / GoStringer は意図的に実装しない
//   - raw 取り出しは Reveal() / Encode() を明示呼び出し時のみ
package manage_url_token

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

var ErrInvalidLength = errors.New("manage url token must be 32 bytes (43 base64url chars)")

const rawBytesLen = 32
const encodedLen = 43

// ManageUrlToken は管理 URL の raw 値。
type ManageUrlToken struct {
	raw [rawBytesLen]byte
}

// Generate は crypto/rand から 32 バイトの乱数を読んで ManageUrlToken を作る。
func Generate() (ManageUrlToken, error) {
	var t ManageUrlToken
	if _, err := rand.Read(t.raw[:]); err != nil {
		return ManageUrlToken{}, fmt.Errorf("rand read: %w", err)
	}
	return t, nil
}

// Parse は URL / 入力からの base64url 文字列を ManageUrlToken に復元する。
func Parse(encoded string) (ManageUrlToken, error) {
	if len(encoded) != encodedLen {
		return ManageUrlToken{}, ErrInvalidLength
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return ManageUrlToken{}, ErrInvalidLength
	}
	if len(decoded) != rawBytesLen {
		return ManageUrlToken{}, ErrInvalidLength
	}
	var t ManageUrlToken
	copy(t.raw[:], decoded)
	return t, nil
}

// Reveal は raw 32 バイトを返す。**ログには出さないこと**。
func (t ManageUrlToken) Reveal() [rawBytesLen]byte { return t.raw }

// Encode は base64url（padding なし、43 文字）の文字列を返す。
func (t ManageUrlToken) Encode() string {
	return base64.RawURLEncoding.EncodeToString(t.raw[:])
}

// IsZero はゼロ値（無効）かどうかを返す。
func (t ManageUrlToken) IsZero() bool {
	for _, b := range t.raw {
		if b != 0 {
			return false
		}
	}
	return true
}
