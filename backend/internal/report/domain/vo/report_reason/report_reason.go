// Package report_reason は Report の reason 値オブジェクト。
//
// 設計参照:
//   - docs/design/aggregates/report/ドメイン設計.md §4.1
//   - 業務知識 v4 §3.6 通報カテゴリ表
//
// CHECK 制約と一致: 6 種（minor_safety_concern を含む、v4 P0-10）。
package report_reason

import (
	"errors"
	"fmt"
)

var ErrInvalidReportReason = errors.New("invalid report reason")

// ReportReason は reports.reason 値オブジェクト。
type ReportReason struct{ v string }

const (
	rawSubjectRemovalRequest  = "subject_removal_request"
	rawUnauthorizedRepost     = "unauthorized_repost"
	rawSensitiveFlagMissing   = "sensitive_flag_missing"
	rawHarassmentOrDoxxing    = "harassment_or_doxxing"
	rawMinorSafetyConcern     = "minor_safety_concern"
	rawOther                  = "other"
)

// 6 種のコンストラクタ。
func SubjectRemovalRequest() ReportReason { return ReportReason{v: rawSubjectRemovalRequest} }
func UnauthorizedRepost() ReportReason    { return ReportReason{v: rawUnauthorizedRepost} }
func SensitiveFlagMissing() ReportReason  { return ReportReason{v: rawSensitiveFlagMissing} }
func HarassmentOrDoxxing() ReportReason   { return ReportReason{v: rawHarassmentOrDoxxing} }
func MinorSafetyConcern() ReportReason    { return ReportReason{v: rawMinorSafetyConcern} }
func Other() ReportReason                 { return ReportReason{v: rawOther} }

// Parse は DB / 入力からの文字列を ReportReason に復元する。
func Parse(s string) (ReportReason, error) {
	switch s {
	case rawSubjectRemovalRequest, rawUnauthorizedRepost, rawSensitiveFlagMissing,
		rawHarassmentOrDoxxing, rawMinorSafetyConcern, rawOther:
		return ReportReason{v: s}, nil
	default:
		return ReportReason{}, fmt.Errorf("%w: %q", ErrInvalidReportReason, s)
	}
}

// IsMinorSafetyConcern は v4 §3.6 / §7.4 最優先対応かどうか判定。
// Outbox handler 側で通知レベルを上げるときに使う。
func (r ReportReason) IsMinorSafetyConcern() bool { return r.v == rawMinorSafetyConcern }

// String / Equal / IsZero。
func (r ReportReason) String() string                { return r.v }
func (r ReportReason) Equal(other ReportReason) bool { return r.v == other.v }
func (r ReportReason) IsZero() bool                  { return r.v == "" }
