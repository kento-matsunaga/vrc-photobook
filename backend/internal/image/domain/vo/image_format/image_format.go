// Package image_format は Image.source_format の VO。
//
// CHECK 制約と一致: jpg / png / webp / heic（受付対応）。
//
// 設計参照:
//   - docs/design/aggregates/image/ドメイン設計.md §4
//   - docs/design/aggregates/image/データモデル設計.md §3
package image_format

import (
	"errors"
	"fmt"
)

// ErrInvalidImageFormat は未知の値を渡したときのエラー。
var ErrInvalidImageFormat = errors.New("invalid image format")

// ImageFormat は source_format 列に対応する VO。
type ImageFormat struct {
	v string
}

const (
	rawJpg  = "jpg"
	rawPng  = "png"
	rawWebp = "webp"
	rawHeic = "heic"
)

func Jpg() ImageFormat  { return ImageFormat{v: rawJpg} }
func Png() ImageFormat  { return ImageFormat{v: rawPng} }
func Webp() ImageFormat { return ImageFormat{v: rawWebp} }
func Heic() ImageFormat { return ImageFormat{v: rawHeic} }

// Parse は DB / 入力からの文字列を ImageFormat に復元する。
func Parse(s string) (ImageFormat, error) {
	switch s {
	case rawJpg:
		return Jpg(), nil
	case rawPng:
		return Png(), nil
	case rawWebp:
		return Webp(), nil
	case rawHeic:
		return Heic(), nil
	default:
		return ImageFormat{}, fmt.Errorf("%w: %q", ErrInvalidImageFormat, s)
	}
}

func (f ImageFormat) String() string                { return f.v }
func (f ImageFormat) Equal(other ImageFormat) bool  { return f.v == other.v }
