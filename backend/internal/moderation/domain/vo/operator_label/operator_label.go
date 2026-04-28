// Package operator_label は ModerationAction の actor_label 値オブジェクト。
//
// 設計参照:
//   - docs/design/aggregates/moderation/ドメイン設計.md §4.3
//   - docs/plan/m2-moderation-ops-plan.md §9.2
//
// 個人情報を含まない運営内識別子（例: "ops-1" / "legal-team"）。
// MVP では単一運用者前提だが、将来 OperatorId 化に備えて VO で型を保つ。
//
// 正規表現: ^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}[a-zA-Z0-9]$
//   - 先頭 / 末尾は英数字
//   - 中間は英数 + . _ - のみ
//   - 全長 3〜64 文字（migration の CHECK と整合）
package operator_label

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrInvalidOperatorLabel = errors.New("invalid operator label")
	// labelRe は v4 設計書 §4.3 の正規表現と一致。
	labelRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}[a-zA-Z0-9]$`)
)

// OperatorLabel は moderation_actions.actor_label 値オブジェクト。
type OperatorLabel struct{ v string }

// Parse は CLI 入力 / DB 文字列を OperatorLabel に変換する。
func Parse(s string) (OperatorLabel, error) {
	if !labelRe.MatchString(s) {
		return OperatorLabel{}, fmt.Errorf("%w: %q (must match ^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}[a-zA-Z0-9]$)", ErrInvalidOperatorLabel, s)
	}
	return OperatorLabel{v: s}, nil
}

// String / Equal / IsZero。
func (l OperatorLabel) String() string                 { return l.v }
func (l OperatorLabel) Equal(other OperatorLabel) bool { return l.v == other.v }
func (l OperatorLabel) IsZero() bool                   { return l.v == "" }
