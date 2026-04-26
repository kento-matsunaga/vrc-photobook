// Package display_order は photobook_pages.display_order /
// photobook_photos.display_order の VO。
//
// 設計参照:
//   - docs/design/aggregates/photobook/データモデル設計.md §4 / §6
//
// 不変条件:
//   - 0 以上
//   - 連番（0,1,2,...）の制約は集約で担保（DB は uniqueness のみ）
package display_order

import (
	"errors"
	"fmt"
)

// ErrNegativeDisplayOrder は負値を渡したときのエラー。
var ErrNegativeDisplayOrder = errors.New("display_order must be non-negative")

// DisplayOrder は 0始まりの順序値。
type DisplayOrder struct {
	v int
}

// New は int を DisplayOrder に変換する。
func New(v int) (DisplayOrder, error) {
	if v < 0 {
		return DisplayOrder{}, fmt.Errorf("%w: %d", ErrNegativeDisplayOrder, v)
	}
	return DisplayOrder{v: v}, nil
}

// Zero は 0 を返す（先頭要素用）。
func Zero() DisplayOrder { return DisplayOrder{v: 0} }

// Int は int 表現を返す。
func (d DisplayOrder) Int() int               { return d.v }
func (d DisplayOrder) Equal(o DisplayOrder) bool { return d.v == o.v }
