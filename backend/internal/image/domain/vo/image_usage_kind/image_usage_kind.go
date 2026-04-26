// Package image_usage_kind は Image.usage_kind の VO。
//
// CHECK 制約と一致: photo / cover / ogp。Phase 2 で `creator_avatar` 追加可能（MVP では追加しない）。
//
// 設計参照:
//   - docs/design/aggregates/image/ドメイン設計.md §4.1
//   - docs/design/aggregates/image/データモデル設計.md §3
package image_usage_kind

import (
	"errors"
	"fmt"
)

// ErrInvalidImageUsageKind は未知の値を渡したときのエラー。
var ErrInvalidImageUsageKind = errors.New("invalid image usage kind")

// ImageUsageKind は usage_kind 列に対応する VO。
type ImageUsageKind struct {
	v string
}

const (
	rawPhoto = "photo"
	rawCover = "cover"
	rawOgp   = "ogp"
)

// Photo は本文ページに使われる写真。MVP 標準。
func Photo() ImageUsageKind { return ImageUsageKind{v: rawPhoto} }

// Cover は表紙画像。
func Cover() ImageUsageKind { return ImageUsageKind{v: rawCover} }

// Ogp は OGP 共有画像の実体。
func Ogp() ImageUsageKind { return ImageUsageKind{v: rawOgp} }

// Parse は DB / 入力からの文字列を ImageUsageKind に復元する。
func Parse(s string) (ImageUsageKind, error) {
	switch s {
	case rawPhoto:
		return Photo(), nil
	case rawCover:
		return Cover(), nil
	case rawOgp:
		return Ogp(), nil
	default:
		return ImageUsageKind{}, fmt.Errorf("%w: %q", ErrInvalidImageUsageKind, s)
	}
}

func (k ImageUsageKind) String() string                  { return k.v }
func (k ImageUsageKind) IsPhoto() bool                   { return k.v == rawPhoto }
func (k ImageUsageKind) IsCover() bool                   { return k.v == rawCover }
func (k ImageUsageKind) IsOgp() bool                     { return k.v == rawOgp }
func (k ImageUsageKind) Equal(other ImageUsageKind) bool { return k.v == other.v }
