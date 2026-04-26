// Package variant_kind は ImageVariant.kind の VO。
//
// CHECK 制約と一致: original / display / thumbnail / ogp。
// MVP では `original` は保持しない（v4 U9）。`display` / `thumbnail` は photo / cover で必須。
//
// 設計参照:
//   - docs/design/aggregates/image/ドメイン設計.md §4
//   - docs/design/aggregates/image/データモデル設計.md §4
package variant_kind

import (
	"errors"
	"fmt"
)

// ErrInvalidVariantKind は未知の値を渡したときのエラー。
var ErrInvalidVariantKind = errors.New("invalid variant kind")

// VariantKind は image_variants.kind に対応する VO。
type VariantKind struct {
	v string
}

const (
	rawOriginal  = "original"
	rawDisplay   = "display"
	rawThumbnail = "thumbnail"
	rawOgp       = "ogp"
)

func Original() VariantKind  { return VariantKind{v: rawOriginal} }
func Display() VariantKind   { return VariantKind{v: rawDisplay} }
func Thumbnail() VariantKind { return VariantKind{v: rawThumbnail} }
func Ogp() VariantKind       { return VariantKind{v: rawOgp} }

// Parse は DB / 入力からの文字列を VariantKind に復元する。
func Parse(s string) (VariantKind, error) {
	switch s {
	case rawOriginal:
		return Original(), nil
	case rawDisplay:
		return Display(), nil
	case rawThumbnail:
		return Thumbnail(), nil
	case rawOgp:
		return Ogp(), nil
	default:
		return VariantKind{}, fmt.Errorf("%w: %q", ErrInvalidVariantKind, s)
	}
}

func (k VariantKind) String() string                { return k.v }
func (k VariantKind) IsOriginal() bool              { return k.v == rawOriginal }
func (k VariantKind) IsDisplay() bool               { return k.v == rawDisplay }
func (k VariantKind) IsThumbnail() bool             { return k.v == rawThumbnail }
func (k VariantKind) IsOgp() bool                   { return k.v == rawOgp }
func (k VariantKind) Equal(other VariantKind) bool  { return k.v == other.v }
