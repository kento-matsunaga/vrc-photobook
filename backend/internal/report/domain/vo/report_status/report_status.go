// Package report_status は Report の status 値オブジェクト。
//
// 設計参照:
//   - docs/design/aggregates/report/ドメイン設計.md §4.4
//
// CHECK 制約と一致: 5 種（v4）。
// PR35b MVP では `submitted` 作成と Moderation 連動の `resolved_action_taken` 遷移
// のみを実運用する。`under_review` / `resolved_no_action` / `dismissed` は CHECK 上
// 受け入れるが、UseCase は後続 PR で追加。
package report_status

import (
	"errors"
	"fmt"
)

var ErrInvalidReportStatus = errors.New("invalid report status")

// ReportStatus は reports.status 値オブジェクト。
type ReportStatus struct{ v string }

const (
	rawSubmitted             = "submitted"
	rawUnderReview           = "under_review"
	rawResolvedActionTaken   = "resolved_action_taken"
	rawResolvedNoAction      = "resolved_no_action"
	rawDismissed             = "dismissed"
)

// 5 種のコンストラクタ。
func Submitted() ReportStatus           { return ReportStatus{v: rawSubmitted} }
func UnderReview() ReportStatus         { return ReportStatus{v: rawUnderReview} }
func ResolvedActionTaken() ReportStatus { return ReportStatus{v: rawResolvedActionTaken} }
func ResolvedNoAction() ReportStatus    { return ReportStatus{v: rawResolvedNoAction} }
func Dismissed() ReportStatus           { return ReportStatus{v: rawDismissed} }

// Parse は DB / 入力からの文字列を ReportStatus に復元する。
func Parse(s string) (ReportStatus, error) {
	switch s {
	case rawSubmitted, rawUnderReview, rawResolvedActionTaken, rawResolvedNoAction, rawDismissed:
		return ReportStatus{v: s}, nil
	default:
		return ReportStatus{}, fmt.Errorf("%w: %q", ErrInvalidReportStatus, s)
	}
}

// IsTerminal は終端状態（再オープン不可）か判定。
func (s ReportStatus) IsTerminal() bool {
	return s.v == rawResolvedActionTaken || s.v == rawResolvedNoAction || s.v == rawDismissed
}

// String / Equal / IsZero。
func (s ReportStatus) String() string                { return s.v }
func (s ReportStatus) Equal(other ReportStatus) bool { return s.v == other.v }
func (s ReportStatus) IsZero() bool                  { return s.v == "" }
