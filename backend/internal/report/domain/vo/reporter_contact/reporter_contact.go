// Package reporter_contact は Report の reporter_contact 値オブジェクト（任意）。
//
// 設計参照:
//   - docs/design/aggregates/report/ドメイン設計.md §4.3
//
// 0–200 文字、任意。メールアドレス / X ID / 自由形式（フォーマット強制しない）。
// 通報対応以外の用途に用いない運用 (v4 §3.6 / §7.2) は runbook で示す。
package reporter_contact

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

const maxLen = 200

var (
	ErrTooLong          = errors.New("reporter contact too long")
	ErrControlCharInContact = errors.New("reporter contact contains forbidden control character")
)

// ReporterContact は reports.reporter_contact 値オブジェクト（任意）。
type ReporterContact struct {
	v     string
	valid bool
}

// None は contact 未指定。
func None() ReporterContact { return ReporterContact{} }

// Parse は HTTP body / CLI 入力等を ReporterContact に変換。空文字列は None。
func Parse(s string) (ReporterContact, error) {
	if s == "" {
		return None(), nil
	}
	if utf8.RuneCountInString(s) > maxLen {
		return ReporterContact{}, fmt.Errorf("%w: rune_count=%d max=%d", ErrTooLong, utf8.RuneCountInString(s), maxLen)
	}
	for _, r := range s {
		// 改行 / 制御文字は連絡先に不要。\t も拒否。print 不能文字を全弾く。
		if r < 0x20 || r == 0x7F {
			return ReporterContact{}, fmt.Errorf("%w: rune=%U", ErrControlCharInContact, r)
		}
	}
	return ReporterContact{v: s, valid: true}, nil
}

// Present は値が存在するかを返す。
func (c ReporterContact) Present() bool { return c.valid }

// String は値を返す（Present=false なら ""）。
func (c ReporterContact) String() string { return c.v }

// Equal は値による等価判定。
func (c ReporterContact) Equal(other ReporterContact) bool {
	return c.valid == other.valid && c.v == other.v
}
