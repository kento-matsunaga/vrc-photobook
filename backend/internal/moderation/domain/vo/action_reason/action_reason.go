// Package action_reason は ModerationAction の reason 値オブジェクト。
//
// 設計参照:
//   - docs/design/aggregates/moderation/ドメイン設計.md §4.2
//   - docs/plan/m2-moderation-ops-plan.md §6.4 / §13 ユーザー判断 #3
//
// CHECK 制約と一致: 9 種すべてを最初から受け入れる。アプリ層では MVP 運用ガイド
// （runbook）で「policy_violation_other / report_based_* / rights_claim /
// erroneous_action_correction」7 種を案内する。
package action_reason

import (
	"errors"
	"fmt"
)

var ErrInvalidActionReason = errors.New("invalid moderation action reason")

// ActionReason は moderation_actions.reason 値オブジェクト。
type ActionReason struct{ v string }

const (
	rawReportBasedHarassment          = "report_based_harassment"
	rawReportBasedUnauthorizedRepost  = "report_based_unauthorized_repost"
	rawReportBasedSensitiveViolation  = "report_based_sensitive_violation"
	rawReportBasedMinorRelated        = "report_based_minor_related"
	rawReportBasedSubjectRemoval      = "report_based_subject_removal"
	rawRightsClaim                    = "rights_claim"
	rawCreatorRequestManageURLReissue = "creator_request_manage_url_reissue"
	rawErroneousActionCorrection      = "erroneous_action_correction"
	rawPolicyViolationOther           = "policy_violation_other"
)

// 9 種すべてのコンストラクタ。
func ReportBasedHarassment() ActionReason {
	return ActionReason{v: rawReportBasedHarassment}
}
func ReportBasedUnauthorizedRepost() ActionReason {
	return ActionReason{v: rawReportBasedUnauthorizedRepost}
}
func ReportBasedSensitiveViolation() ActionReason {
	return ActionReason{v: rawReportBasedSensitiveViolation}
}
func ReportBasedMinorRelated() ActionReason {
	return ActionReason{v: rawReportBasedMinorRelated}
}
func ReportBasedSubjectRemoval() ActionReason {
	return ActionReason{v: rawReportBasedSubjectRemoval}
}
func RightsClaim() ActionReason {
	return ActionReason{v: rawRightsClaim}
}
func CreatorRequestManageURLReissue() ActionReason {
	return ActionReason{v: rawCreatorRequestManageURLReissue}
}
func ErroneousActionCorrection() ActionReason {
	return ActionReason{v: rawErroneousActionCorrection}
}
func PolicyViolationOther() ActionReason {
	return ActionReason{v: rawPolicyViolationOther}
}

// Parse は DB / 入力からの文字列を ActionReason に復元する。
func Parse(s string) (ActionReason, error) {
	switch s {
	case rawReportBasedHarassment, rawReportBasedUnauthorizedRepost,
		rawReportBasedSensitiveViolation, rawReportBasedMinorRelated,
		rawReportBasedSubjectRemoval, rawRightsClaim,
		rawCreatorRequestManageURLReissue, rawErroneousActionCorrection,
		rawPolicyViolationOther:
		return ActionReason{v: s}, nil
	default:
		return ActionReason{}, fmt.Errorf("%w: %q", ErrInvalidActionReason, s)
	}
}

// String / Equal / IsZero。
func (r ActionReason) String() string                { return r.v }
func (r ActionReason) Equal(other ActionReason) bool { return r.v == other.v }
func (r ActionReason) IsZero() bool                  { return r.v == "" }
