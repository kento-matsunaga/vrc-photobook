// Package normalized_format は Image.normalized_format の VO。
//
// CHECK 制約と一致: jpg / webp（HEIC / PNG は内部で変換）。
//
// 設計参照:
//   - docs/design/aggregates/image/ドメイン設計.md §4.2
//   - docs/design/aggregates/image/データモデル設計.md §3
package normalized_format

import (
	"errors"
	"fmt"
)

// ErrInvalidNormalizedFormat は未知の値を渡したときのエラー。
var ErrInvalidNormalizedFormat = errors.New("invalid normalized format")

// NormalizedFormat は normalized_format 列に対応する VO。
type NormalizedFormat struct {
	v string
}

const (
	rawJpg  = "jpg"
	rawWebp = "webp"
)

func Jpg() NormalizedFormat  { return NormalizedFormat{v: rawJpg} }
func Webp() NormalizedFormat { return NormalizedFormat{v: rawWebp} }

// Parse は DB / 入力からの文字列を NormalizedFormat に復元する。
func Parse(s string) (NormalizedFormat, error) {
	switch s {
	case rawJpg:
		return Jpg(), nil
	case rawWebp:
		return Webp(), nil
	default:
		return NormalizedFormat{}, fmt.Errorf("%w: %q", ErrInvalidNormalizedFormat, s)
	}
}

func (f NormalizedFormat) String() string                     { return f.v }
func (f NormalizedFormat) Equal(other NormalizedFormat) bool  { return f.v == other.v }
