// Package photobook_layout は Photobook の layout 列値オブジェクト。
//
// CHECK 制約と一致: simple / magazine / card / large。
package photobook_layout

import (
	"errors"
	"fmt"
)

var ErrInvalidPhotobookLayout = errors.New("invalid photobook layout")

type PhotobookLayout struct{ v string }

const (
	rawSimple   = "simple"
	rawMagazine = "magazine"
	rawCard     = "card"
	rawLarge    = "large"
)

func Simple() PhotobookLayout   { return PhotobookLayout{v: rawSimple} }
func Magazine() PhotobookLayout { return PhotobookLayout{v: rawMagazine} }
func Card() PhotobookLayout     { return PhotobookLayout{v: rawCard} }
func Large() PhotobookLayout    { return PhotobookLayout{v: rawLarge} }

func Parse(s string) (PhotobookLayout, error) {
	switch s {
	case rawSimple, rawMagazine, rawCard, rawLarge:
		return PhotobookLayout{v: s}, nil
	default:
		return PhotobookLayout{}, fmt.Errorf("%w: %q", ErrInvalidPhotobookLayout, s)
	}
}

func (t PhotobookLayout) String() string                    { return t.v }
func (t PhotobookLayout) Equal(other PhotobookLayout) bool  { return t.v == other.v }
