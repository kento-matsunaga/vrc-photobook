// Package draft_edit_token は draft 編集 URL に乗る raw 値の VO。
//
// 設計参照:
//   - docs/design/aggregates/photobook/ドメイン設計.md §4 / §13.1
//   - docs/adr/0003-frontend-token-session-flow.md
//   - docs/plan/m2-photobook-session-integration-plan.md §5
//
// 仕様:
//   - 256bit (32 バイト) 乱数を crypto/rand で生成
//   - base64url（パディングなし）43 文字
//   - DB には保存しない（保存対象は DraftEditTokenHash）
//
// セキュリティ:
//   - String() / GoStringer は意図的に実装しない（不用意なログ出力を避ける）
//   - raw 取り出しは Reveal() / Encode() を明示呼び出し時のみ
//   - ログ・diff・テストログ・URL 全体（/draft/{token}）の出力は禁止
package draft_edit_token

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

var ErrInvalidLength = errors.New("draft edit token must be 32 bytes (43 base64url chars)")

const rawBytesLen = 32
const encodedLen = 43

// DraftEditToken は draft 編集 URL の raw 値。
type DraftEditToken struct {
	raw [rawBytesLen]byte
}

// Generate は crypto/rand から 32 バイトの乱数を読んで DraftEditToken を作る。
func Generate() (DraftEditToken, error) {
	var t DraftEditToken
	if _, err := rand.Read(t.raw[:]); err != nil {
		return DraftEditToken{}, fmt.Errorf("rand read: %w", err)
	}
	return t, nil
}

// Parse は URL / 入力からの base64url 文字列を DraftEditToken に復元する。
func Parse(encoded string) (DraftEditToken, error) {
	if len(encoded) != encodedLen {
		return DraftEditToken{}, ErrInvalidLength
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return DraftEditToken{}, ErrInvalidLength
	}
	if len(decoded) != rawBytesLen {
		return DraftEditToken{}, ErrInvalidLength
	}
	var t DraftEditToken
	copy(t.raw[:], decoded)
	return t, nil
}

// Reveal は raw 32 バイトを返す。**ログには出さないこと**。
// hash 化用途でのみ呼び出す。
func (t DraftEditToken) Reveal() [rawBytesLen]byte { return t.raw }

// Encode は base64url（padding なし、43 文字）の文字列を返す。
// /draft/{token} URL を作成者に渡すときの表示でのみ使う（ログ禁止）。
func (t DraftEditToken) Encode() string {
	return base64.RawURLEncoding.EncodeToString(t.raw[:])
}

// IsZero はゼロ値（無効）かどうかを返す。
func (t DraftEditToken) IsZero() bool {
	for _, b := range t.raw {
		if b != 0 {
			return false
		}
	}
	return true
}
