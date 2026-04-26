// Package mime_type は ImageVariant.mime_type の VO。
//
// 配信用 MIME に限定: image/jpeg / image/png / image/webp。
// HEIC は内部で JPG / WebP に変換済みのため variant には出現しない。
//
// 設計参照:
//   - docs/design/aggregates/image/データモデル設計.md §4
package mime_type

import (
	"errors"
	"fmt"
)

// ErrInvalidMimeType は未知の MIME を渡したときのエラー。
var ErrInvalidMimeType = errors.New("invalid mime type")

// MimeType は variant の mime_type 列に対応する VO。
type MimeType struct {
	v string
}

const (
	rawJpeg = "image/jpeg"
	rawPng  = "image/png"
	rawWebp = "image/webp"
)

func Jpeg() MimeType { return MimeType{v: rawJpeg} }
func Png() MimeType  { return MimeType{v: rawPng} }
func Webp() MimeType { return MimeType{v: rawWebp} }

// Parse は DB / 入力からの文字列を MimeType に復元する。
func Parse(s string) (MimeType, error) {
	switch s {
	case rawJpeg:
		return Jpeg(), nil
	case rawPng:
		return Png(), nil
	case rawWebp:
		return Webp(), nil
	default:
		return MimeType{}, fmt.Errorf("%w: %q", ErrInvalidMimeType, s)
	}
}

func (m MimeType) String() string             { return m.v }
func (m MimeType) Equal(other MimeType) bool  { return m.v == other.v }
