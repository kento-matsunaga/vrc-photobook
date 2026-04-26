// Package visibility は Photobook の visibility 列値オブジェクト。
//
// CHECK 制約と一致: public / unlisted / private。既定値は unlisted（業務知識 v4 §3.2 / I8）。
package visibility

import (
	"errors"
	"fmt"
)

var ErrInvalidVisibility = errors.New("invalid visibility")

type Visibility struct{ v string }

const (
	rawPublic   = "public"
	rawUnlisted = "unlisted"
	rawPrivate  = "private"
)

func Public() Visibility   { return Visibility{v: rawPublic} }
func Unlisted() Visibility { return Visibility{v: rawUnlisted} }
func Private() Visibility  { return Visibility{v: rawPrivate} }

func Parse(s string) (Visibility, error) {
	switch s {
	case rawPublic, rawUnlisted, rawPrivate:
		return Visibility{v: s}, nil
	default:
		return Visibility{}, fmt.Errorf("%w: %q", ErrInvalidVisibility, s)
	}
}

func (v Visibility) String() string                { return v.v }
func (v Visibility) Equal(other Visibility) bool   { return v.v == other.v }
