// Package photobook_type は Photobook の type 列値オブジェクト。
//
// CHECK 制約と一致: event / daily / portfolio / avatar / world / memory / free。
package photobook_type

import (
	"errors"
	"fmt"
)

var ErrInvalidPhotobookType = errors.New("invalid photobook type")

type PhotobookType struct{ v string }

const (
	rawEvent     = "event"
	rawDaily     = "daily"
	rawPortfolio = "portfolio"
	rawAvatar    = "avatar"
	rawWorld     = "world"
	rawMemory    = "memory"
	rawFree      = "free"
)

func Event() PhotobookType     { return PhotobookType{v: rawEvent} }
func Daily() PhotobookType     { return PhotobookType{v: rawDaily} }
func Portfolio() PhotobookType { return PhotobookType{v: rawPortfolio} }
func Avatar() PhotobookType    { return PhotobookType{v: rawAvatar} }
func World() PhotobookType     { return PhotobookType{v: rawWorld} }
func Memory() PhotobookType    { return PhotobookType{v: rawMemory} }
func Free() PhotobookType      { return PhotobookType{v: rawFree} }

func Parse(s string) (PhotobookType, error) {
	switch s {
	case rawEvent, rawDaily, rawPortfolio, rawAvatar, rawWorld, rawMemory, rawFree:
		return PhotobookType{v: s}, nil
	default:
		return PhotobookType{}, fmt.Errorf("%w: %q", ErrInvalidPhotobookType, s)
	}
}

func (t PhotobookType) String() string                  { return t.v }
func (t PhotobookType) Equal(other PhotobookType) bool  { return t.v == other.v }
