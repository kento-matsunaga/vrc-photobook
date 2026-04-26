// Package caption は photobook_pages.caption / photobook_photos.caption の VO。
//
// 設計参照:
//   - docs/design/aggregates/photobook/データモデル設計.md §4 / §6（length 0..200）
//
// 不変条件:
//   - rune 数で 0..=200
//   - 制御文字（\x00 等）は禁止（業務 lint 観点で安全側）
package caption

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// エラー。
var (
	ErrTooLong            = errors.New("caption too long (max 200 runes)")
	ErrControlCharNotAllowed = errors.New("caption must not contain control characters")
)

const maxRunes = 200

// Caption は photobook_pages.caption / photobook_photos.caption に対応する VO。
type Caption struct {
	v string
}

// New は string を Caption に変換する。空文字列は valid（NULL 相当として扱う場合は
// 呼び出し側で nil ポインタにする）。
func New(s string) (Caption, error) {
	if r := []rune(s); len(r) > maxRunes {
		return Caption{}, fmt.Errorf("%w: %d", ErrTooLong, len(r))
	}
	for _, r := range s {
		if unicode.IsControl(r) && !isAllowedWhitespace(r) {
			return Caption{}, ErrControlCharNotAllowed
		}
	}
	return Caption{v: s}, nil
}

// MustNew はテストで失敗時 panic。
func MustNew(s string) Caption {
	c, err := New(s)
	if err != nil {
		panic(err)
	}
	return c
}

// String は内部値を返す。
func (c Caption) String() string             { return c.v }
func (c Caption) Equal(o Caption) bool       { return c.v == o.v }
func (c Caption) IsEmpty() bool              { return strings.TrimSpace(c.v) == "" }

func isAllowedWhitespace(r rune) bool {
	return r == '\n' || r == '\t' || r == '\r'
}
