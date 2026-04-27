// Package ogp_version は photobook_ogp_images.version の VO（int >= 1）。
//
// Photobook が更新されるたびに stale 化 + version++、再生成完了で generated に戻る。
// SNS crawler 側 cache を分離するため URL クエリに使う（PR33c）。
package ogp_version

import (
	"errors"
	"fmt"
)

var ErrInvalidOgpVersion = errors.New("ogp version must be >= 1")

type OgpVersion struct {
	v int
}

func New(v int) (OgpVersion, error) {
	if v < 1 {
		return OgpVersion{}, fmt.Errorf("%w: got %d", ErrInvalidOgpVersion, v)
	}
	return OgpVersion{v: v}, nil
}

func One() OgpVersion { return OgpVersion{v: 1} }

func (v OgpVersion) Int() int                    { return v.v }
func (v OgpVersion) Equal(other OgpVersion) bool { return v.v == other.v }
func (v OgpVersion) Increment() OgpVersion       { return OgpVersion{v: v.v + 1} }
