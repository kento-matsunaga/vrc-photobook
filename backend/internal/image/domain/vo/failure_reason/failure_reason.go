// Package failure_reason は Image.failure_reason の VO。
//
// 12 種固定（v4 / 付録C P0-11）。CHECK 制約で値域固定。
//
// 設計参照:
//   - docs/design/aggregates/image/データモデル設計.md §3.0
//   - docs/design/aggregates/image/ドメイン設計.md §4 / §11
package failure_reason

import (
	"errors"
	"fmt"
)

// ErrInvalidFailureReason は未知の値を渡したときのエラー。
var ErrInvalidFailureReason = errors.New("invalid failure reason")

// FailureReason は failure_reason 列に対応する VO。
type FailureReason struct {
	v string
}

const (
	rawFileTooLarge             = "file_too_large"
	rawSizeMismatch             = "size_mismatch"
	rawUnsupportedFormat        = "unsupported_format"
	rawSvgNotAllowed            = "svg_not_allowed"
	rawAnimatedImageNotAllowed  = "animated_image_not_allowed"
	rawDimensionsTooLarge       = "dimensions_too_large"
	rawDecodeFailed             = "decode_failed"
	rawExifStripFailed          = "exif_strip_failed"
	rawHeicConversionFailed     = "heic_conversion_failed"
	rawVariantGenerationFailed  = "variant_generation_failed"
	rawObjectNotFound           = "object_not_found"
	rawUnknown                  = "unknown"
)

func FileTooLarge() FailureReason             { return FailureReason{v: rawFileTooLarge} }
func SizeMismatch() FailureReason             { return FailureReason{v: rawSizeMismatch} }
func UnsupportedFormat() FailureReason        { return FailureReason{v: rawUnsupportedFormat} }
func SvgNotAllowed() FailureReason            { return FailureReason{v: rawSvgNotAllowed} }
func AnimatedImageNotAllowed() FailureReason  { return FailureReason{v: rawAnimatedImageNotAllowed} }
func DimensionsTooLarge() FailureReason       { return FailureReason{v: rawDimensionsTooLarge} }
func DecodeFailed() FailureReason             { return FailureReason{v: rawDecodeFailed} }
func ExifStripFailed() FailureReason          { return FailureReason{v: rawExifStripFailed} }
func HeicConversionFailed() FailureReason     { return FailureReason{v: rawHeicConversionFailed} }
func VariantGenerationFailed() FailureReason  { return FailureReason{v: rawVariantGenerationFailed} }
func ObjectNotFound() FailureReason           { return FailureReason{v: rawObjectNotFound} }
func Unknown() FailureReason                  { return FailureReason{v: rawUnknown} }

// Parse は DB / 入力からの文字列を FailureReason に復元する。
func Parse(s string) (FailureReason, error) {
	switch s {
	case rawFileTooLarge,
		rawSizeMismatch,
		rawUnsupportedFormat,
		rawSvgNotAllowed,
		rawAnimatedImageNotAllowed,
		rawDimensionsTooLarge,
		rawDecodeFailed,
		rawExifStripFailed,
		rawHeicConversionFailed,
		rawVariantGenerationFailed,
		rawObjectNotFound,
		rawUnknown:
		return FailureReason{v: s}, nil
	default:
		return FailureReason{}, fmt.Errorf("%w: %q", ErrInvalidFailureReason, s)
	}
}

func (r FailureReason) String() string                  { return r.v }
func (r FailureReason) Equal(other FailureReason) bool  { return r.v == other.v }
