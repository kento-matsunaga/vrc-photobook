// Package action_kind は ModerationAction の kind 値オブジェクト。
//
// 設計参照:
//   - docs/design/aggregates/moderation/ドメイン設計.md §4.1
//   - docs/plan/m2-moderation-ops-plan.md §13 ユーザー判断 #3
//
// CHECK 制約と一致: hide / unhide / soft_delete / restore / purge / reissue_manage_url。
// MVP では hide / unhide のみ実運用。残りは後続 PR で UseCase 追加時に活用する。
package action_kind

import (
	"errors"
	"fmt"
)

var ErrInvalidActionKind = errors.New("invalid moderation action kind")

// ActionKind は moderation_actions.kind 値オブジェクト。
type ActionKind struct{ v string }

const (
	rawHide             = "hide"
	rawUnhide           = "unhide"
	rawSoftDelete       = "soft_delete"
	rawRestore          = "restore"
	rawPurge            = "purge"
	rawReissueManageURL = "reissue_manage_url"
)

// MVP で使うコンストラクタ。
func Hide() ActionKind   { return ActionKind{v: rawHide} }
func Unhide() ActionKind { return ActionKind{v: rawUnhide} }

// 後続 PR で利用するコンストラクタ。CHECK 制約は最初から受け入れる。
func SoftDelete() ActionKind       { return ActionKind{v: rawSoftDelete} }
func Restore() ActionKind          { return ActionKind{v: rawRestore} }
func Purge() ActionKind            { return ActionKind{v: rawPurge} }
func ReissueManageURL() ActionKind { return ActionKind{v: rawReissueManageURL} }

// Parse は DB / 入力からの文字列を ActionKind に復元する。
func Parse(s string) (ActionKind, error) {
	switch s {
	case rawHide, rawUnhide, rawSoftDelete, rawRestore, rawPurge, rawReissueManageURL:
		return ActionKind{v: s}, nil
	default:
		return ActionKind{}, fmt.Errorf("%w: %q", ErrInvalidActionKind, s)
	}
}

// String / Equal / IsZero。
func (k ActionKind) String() string             { return k.v }
func (k ActionKind) Equal(other ActionKind) bool { return k.v == other.v }
func (k ActionKind) IsZero() bool                { return k.v == "" }
