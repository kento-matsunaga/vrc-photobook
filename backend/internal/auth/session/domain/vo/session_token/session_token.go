// Package session_token は SessionToken 値オブジェクトを提供する。
//
// SessionToken は **Cookie に乗る raw 値**。256bit 乱数（32 バイト）を crypto/rand で生成し、
// base64url（パディングなし、43 文字）でエンコードする。
//
// 設計参照:
//   - docs/adr/0003-frontend-token-session-flow.md
//   - docs/plan/m2-session-auth-implementation-plan.md §5
//
// セキュリティ:
//   - DB には保存しない（保存対象は SessionTokenHash）
//   - String() / fmt.Sprintf("%v") などで不用意に raw を出さないため、Stringer / GoStringer は
//     意図的に実装しない。raw 取り出しは Reveal() を明示呼び出しした場合のみ。
//   - ログ・diff・テストログに raw を載せない（security-guard.md / shared/logging.go の禁止リスト）
package session_token

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// ErrInvalidLength は base64url 復元時に長さが想定外だったときのエラー。
var ErrInvalidLength = errors.New("session token must be 32 bytes (43 base64url chars)")

// rawBytesLen は raw token のバイト数（256bit）。
const rawBytesLen = 32

// encodedLen は base64url（padding なし）でエンコードした後の文字数。
const encodedLen = 43

// SessionToken は Cookie に乗る raw token。
//
// 値の取り出しは Reveal() を明示呼び出ししたときのみ可能。
// 構造体のゼロ値は無効な token として扱う（IsZero() で判定）。
type SessionToken struct {
	raw [rawBytesLen]byte
}

// Generate は crypto/rand から 32 バイトの乱数を読み出して SessionToken を作る。
//
// crypto/rand.Read は Go の暗号論的乱数源（POSIX なら getrandom / /dev/urandom）に直結する。
// math/rand は使用しない。
func Generate() (SessionToken, error) {
	var t SessionToken
	if _, err := rand.Read(t.raw[:]); err != nil {
		return SessionToken{}, fmt.Errorf("rand read: %w", err)
	}
	return t, nil
}

// Parse は Cookie / リクエストから受け取った base64url 文字列を SessionToken に復元する。
//
// 想定外の長さ・base64url 違反は ErrInvalidLength で弾く（タイミング攻撃に neutral な扱い）。
func Parse(encoded string) (SessionToken, error) {
	if len(encoded) != encodedLen {
		return SessionToken{}, ErrInvalidLength
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return SessionToken{}, ErrInvalidLength
	}
	if len(decoded) != rawBytesLen {
		return SessionToken{}, ErrInvalidLength
	}
	var t SessionToken
	copy(t.raw[:], decoded)
	return t, nil
}

// Reveal は raw 32 バイトを返す。**ログには出さないこと**。
//
// この関数は SHA-256 ハッシュ生成 / Cookie への書き込みでのみ呼ぶ。
// 呼び出し元は戻り値をどこに渡したかをコードレビューで追えるようにする。
func (t SessionToken) Reveal() [rawBytesLen]byte {
	return t.raw
}

// Encode は base64url（padding なし、43 文字）の文字列を返す。
// Cookie 値として Set-Cookie ヘッダに書く際に呼ぶ。
//
// **ログには出さない**。Set-Cookie ヘッダ自体もログ禁止。
func (t SessionToken) Encode() string {
	return base64.RawURLEncoding.EncodeToString(t.raw[:])
}

// IsZero は SessionToken がゼロ値（無効）かどうかを返す。
func (t SessionToken) IsZero() bool {
	for _, b := range t.raw {
		if b != 0 {
			return false
		}
	}
	return true
}
