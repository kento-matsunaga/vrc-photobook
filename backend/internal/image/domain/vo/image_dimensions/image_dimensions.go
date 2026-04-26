// Package image_dimensions は Image の幅・高さの VO。
//
// 業務知識 v4 §3.10 / ADR-0005:
//   - 幅・高さは 1〜8192 px
//   - 合計ピクセル数は 40,000,000 (40MP) 以下
//
// 設計参照:
//   - docs/design/aggregates/image/ドメイン設計.md §4
package image_dimensions

import (
	"errors"
	"fmt"
)

// 寸法エラー。
var (
	ErrDimensionOutOfRange = errors.New("image dimension out of range (1..=8192)")
	ErrPixelsExceedLimit   = errors.New("image total pixels exceed 40MP")
)

const (
	minSide   = 1
	maxSide   = 8192
	maxPixels = 40_000_000
)

// ImageDimensions は幅 × 高さの VO。
type ImageDimensions struct {
	width  int
	height int
}

// New は (width, height) から ImageDimensions を組み立てる。
func New(width, height int) (ImageDimensions, error) {
	if width < minSide || width > maxSide {
		return ImageDimensions{}, fmt.Errorf("%w: width=%d", ErrDimensionOutOfRange, width)
	}
	if height < minSide || height > maxSide {
		return ImageDimensions{}, fmt.Errorf("%w: height=%d", ErrDimensionOutOfRange, height)
	}
	if int64(width)*int64(height) > maxPixels {
		return ImageDimensions{}, fmt.Errorf("%w: %dx%d", ErrPixelsExceedLimit, width, height)
	}
	return ImageDimensions{width: width, height: height}, nil
}

// Width は幅。
func (d ImageDimensions) Width() int { return d.width }

// Height は高さ。
func (d ImageDimensions) Height() int { return d.height }

// Pixels は width * height。
func (d ImageDimensions) Pixels() int64 {
	return int64(d.width) * int64(d.height)
}

// Equal は値による等価判定。
func (d ImageDimensions) Equal(other ImageDimensions) bool {
	return d.width == other.width && d.height == other.height
}
