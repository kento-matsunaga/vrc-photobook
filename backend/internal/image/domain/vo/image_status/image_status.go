// Package image_status は Image.status の VO。
//
// CHECK 制約と一致: uploading / processing / available / failed / deleted / purged。
// `rejected` は採用しない（v4 / 付録C P0-10）。
//
// 設計参照:
//   - docs/design/aggregates/image/ドメイン設計.md §4
//   - docs/design/aggregates/image/データモデル設計.md §3
package image_status

import (
	"errors"
	"fmt"
)

// ErrInvalidImageStatus は未知の値を渡したときのエラー。
var ErrInvalidImageStatus = errors.New("invalid image status")

// ImageStatus は status 列に対応する VO。
type ImageStatus struct {
	v string
}

const (
	rawUploading  = "uploading"
	rawProcessing = "processing"
	rawAvailable  = "available"
	rawFailed     = "failed"
	rawDeleted    = "deleted"
	rawPurged     = "purged"
)

func Uploading() ImageStatus  { return ImageStatus{v: rawUploading} }
func Processing() ImageStatus { return ImageStatus{v: rawProcessing} }
func Available() ImageStatus  { return ImageStatus{v: rawAvailable} }
func Failed() ImageStatus     { return ImageStatus{v: rawFailed} }
func Deleted() ImageStatus    { return ImageStatus{v: rawDeleted} }
func Purged() ImageStatus     { return ImageStatus{v: rawPurged} }

// Parse は DB / 入力からの文字列を ImageStatus に復元する。
func Parse(s string) (ImageStatus, error) {
	switch s {
	case rawUploading:
		return Uploading(), nil
	case rawProcessing:
		return Processing(), nil
	case rawAvailable:
		return Available(), nil
	case rawFailed:
		return Failed(), nil
	case rawDeleted:
		return Deleted(), nil
	case rawPurged:
		return Purged(), nil
	default:
		return ImageStatus{}, fmt.Errorf("%w: %q", ErrInvalidImageStatus, s)
	}
}

func (s ImageStatus) String() string                { return s.v }
func (s ImageStatus) IsUploading() bool             { return s.v == rawUploading }
func (s ImageStatus) IsProcessing() bool            { return s.v == rawProcessing }
func (s ImageStatus) IsAvailable() bool             { return s.v == rawAvailable }
func (s ImageStatus) IsFailed() bool                { return s.v == rawFailed }
func (s ImageStatus) IsDeleted() bool               { return s.v == rawDeleted }
func (s ImageStatus) IsPurged() bool                { return s.v == rawPurged }
func (s ImageStatus) Equal(other ImageStatus) bool  { return s.v == other.v }
