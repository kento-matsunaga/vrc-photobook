// Package manage_url_token_version は Photobook.manage_url_token_version の VO。
//
// 役割（業務知識 v4 §3.5 / Session ドメイン I-S6 / I-S10）:
//   - publish 時に 0 で初期化される
//   - reissueManageUrl で +1
//   - manage session 発行時に snapshot として記録（sessions.token_version_at_issue）
//   - reissueManageUrl 時に oldVersion 以下の manage session を一括 revoke
package manage_url_token_version

import (
	"errors"
	"fmt"
)

var ErrNegativeVersion = errors.New("manage url token version must be non-negative")

// ManageUrlTokenVersion は manage_url_token_version 列に対応する VO。
type ManageUrlTokenVersion struct {
	v int
}

// New は 0 以上の int を ManageUrlTokenVersion に変換する。
func New(v int) (ManageUrlTokenVersion, error) {
	if v < 0 {
		return ManageUrlTokenVersion{}, fmt.Errorf("%w: %d", ErrNegativeVersion, v)
	}
	return ManageUrlTokenVersion{v: v}, nil
}

// Zero は publish 直後の初期値を返す。
func Zero() ManageUrlTokenVersion {
	return ManageUrlTokenVersion{v: 0}
}

// Int は int 表現を返す。永続化層との境界でのみ使用する。
func (m ManageUrlTokenVersion) Int() int {
	return m.v
}

// Increment は +1 した新しい値を返す（不変、reissueManageUrl で使う）。
func (m ManageUrlTokenVersion) Increment() ManageUrlTokenVersion {
	return ManageUrlTokenVersion{v: m.v + 1}
}

func (m ManageUrlTokenVersion) Equal(other ManageUrlTokenVersion) bool {
	return m.v == other.v
}
