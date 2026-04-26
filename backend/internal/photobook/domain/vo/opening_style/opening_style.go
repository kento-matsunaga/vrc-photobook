// Package opening_style は Photobook の opening_style 列値オブジェクト。
//
// CHECK 制約と一致: light / cover_first_view。
package opening_style

import (
	"errors"
	"fmt"
)

var ErrInvalidOpeningStyle = errors.New("invalid opening style")

type OpeningStyle struct{ v string }

const (
	rawLight          = "light"
	rawCoverFirstView = "cover_first_view"
)

func Light() OpeningStyle          { return OpeningStyle{v: rawLight} }
func CoverFirstView() OpeningStyle { return OpeningStyle{v: rawCoverFirstView} }

func Parse(s string) (OpeningStyle, error) {
	switch s {
	case rawLight, rawCoverFirstView:
		return OpeningStyle{v: s}, nil
	default:
		return OpeningStyle{}, fmt.Errorf("%w: %q", ErrInvalidOpeningStyle, s)
	}
}

func (t OpeningStyle) String() string                  { return t.v }
func (t OpeningStyle) Equal(other OpeningStyle) bool   { return t.v == other.v }
