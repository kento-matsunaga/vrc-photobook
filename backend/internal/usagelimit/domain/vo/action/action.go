// Package action は UsageLimit の action 値オブジェクト。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §4.2 / §6.2
//   - migrations/00018_create_usage_counters.sql の CHECK 制約と同期
//
// MVP 対象 3 種:
//   - report.submit              : POST /api/public/photobooks/{slug}/reports
//   - upload_verification.issue  : POST /api/photobooks/{id}/upload-verifications
//   - publish.from_draft         : PublishFromDraft（業務知識 v4 §3.7「1 時間 5 冊」）
//
// 拡張時は本 enum + migration の CHECK 制約 + 計画書 §4 を同時更新する。
package action

import (
	"errors"
	"fmt"
)

// ErrInvalidAction は許容外の action 文字列を渡されたとき。
var ErrInvalidAction = errors.New("invalid usagelimit action")

// Action は usage_counters.action 値オブジェクト。
type Action struct{ v string }

const (
	rawReportSubmit              = "report.submit"
	rawUploadVerificationIssue   = "upload_verification.issue"
	rawPublishFromDraft          = "publish.from_draft"
)

// ReportSubmit は通報受付 endpoint の action。
func ReportSubmit() Action { return Action{v: rawReportSubmit} }

// UploadVerificationIssue は upload verification session 発行 endpoint の action。
func UploadVerificationIssue() Action { return Action{v: rawUploadVerificationIssue} }

// PublishFromDraft は draft → published 公開操作の action。
func PublishFromDraft() Action { return Action{v: rawPublishFromDraft} }

// Parse は DB / 入力からの文字列を Action に復元する。
func Parse(s string) (Action, error) {
	switch s {
	case rawReportSubmit, rawUploadVerificationIssue, rawPublishFromDraft:
		return Action{v: s}, nil
	default:
		return Action{}, fmt.Errorf("%w: %q", ErrInvalidAction, s)
	}
}

// String は DB / 出力用の文字列表現。
func (a Action) String() string { return a.v }

// Equal は 2 つの Action を比較する。
func (a Action) Equal(o Action) bool { return a.v == o.v }

// IsZero は VO が未初期化（ゼロ値）かを返す。
func (a Action) IsZero() bool { return a.v == "" }
