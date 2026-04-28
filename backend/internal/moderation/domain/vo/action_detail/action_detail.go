// Package action_detail は ModerationAction の detail 値オブジェクト（任意）。
//
// 設計参照:
//   - docs/design/aggregates/moderation/ドメイン設計.md §4.4
//   - docs/plan/m2-moderation-ops-plan.md §9.1
//
// 内部参照用。≤ 2000 char。MVP 運用ガイドで「個人情報を書かない」を案内する。
package action_detail

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

const maxLen = 2000

var ErrTooLong = errors.New("moderation action detail too long")

// ActionDetail は moderation_actions.detail 値オブジェクト（任意）。
type ActionDetail struct {
	v     string
	valid bool // 空でも明示セットされたか区別する（Parse 経由）
}

// None は detail 未指定を表す。
func None() ActionDetail { return ActionDetail{} }

// Parse は CLI 入力等を ActionDetail に変換する。
//
// 空文字列は None と等価扱い（DB の NULL に対応）。
func Parse(s string) (ActionDetail, error) {
	if s == "" {
		return None(), nil
	}
	if utf8.RuneCountInString(s) > maxLen {
		return ActionDetail{}, fmt.Errorf("%w: rune_count=%d max=%d", ErrTooLong, utf8.RuneCountInString(s), maxLen)
	}
	return ActionDetail{v: s, valid: true}, nil
}

// Present は detail が存在するか（DB INSERT 時の NULL/値分岐に使う）。
func (d ActionDetail) Present() bool { return d.valid }

// String は値を返す（Present=false なら "" を返す）。
func (d ActionDetail) String() string { return d.v }

// Equal は値による等価判定。
func (d ActionDetail) Equal(other ActionDetail) bool {
	return d.valid == other.valid && d.v == other.v
}
