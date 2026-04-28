// Package entity は Report 集約のエンティティ。
//
// 設計参照:
//   - docs/design/aggregates/report/ドメイン設計.md §3 / §5
//
// Report は **ID + state machine** 集約。submitted（作成時）/ under_review /
// resolved_action_taken / resolved_no_action / dismissed の 5 状態を持ち、終端
// 状態（I4）は再オープンしない。PR35b MVP では submitted 作成 + Moderation 連動
// による resolved_action_taken 遷移のみを実運用する。
package entity

import (
	"errors"
	"time"

	"vrcpb/backend/internal/moderation/domain/vo/action_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/report/domain/vo/report_detail"
	"vrcpb/backend/internal/report/domain/vo/report_id"
	"vrcpb/backend/internal/report/domain/vo/report_reason"
	"vrcpb/backend/internal/report/domain/vo/report_status"
	"vrcpb/backend/internal/report/domain/vo/reporter_contact"
	"vrcpb/backend/internal/report/domain/vo/target_snapshot"
)

// 共通エラー。
var (
	ErrInvalidSubmittedAt = errors.New("report submittedAt must be non-zero")
)

// Report は集約ルート。
//
// 不変条件 I1〜I8（ドメイン設計 §5）:
//   - I1: targetPhotobookId 必須
//   - I2: reason 必須（6 種）
//   - I3: status='submitted' 遷移時 submittedAt 必ず記録
//   - I4: 終端状態は再オープン不可
//   - I5: reporterContact が空のとき通報者通知なし（運用層）
//   - I6: targetSnapshot 必須
//   - I7: targetPhotobookId に FK なし（DB 設計、本 entity は満たす）
//   - I8: resolvedByModerationActionId は status='resolved_action_taken' のときに限る
type Report struct {
	id                           report_id.ReportID
	targetPhotobookID            photobook_id.PhotobookID
	targetSnapshot               target_snapshot.TargetSnapshot
	reason                       report_reason.ReportReason
	detail                       report_detail.ReportDetail
	reporterContact              reporter_contact.ReporterContact
	status                       report_status.ReportStatus
	submittedAt                  time.Time
	reviewedAt                   *time.Time
	resolvedAt                   *time.Time
	resolutionNote               *string
	resolvedByModerationActionID *action_id.ActionID
	sourceIPHash                 []byte // ソルト + sha256（生 IP は保持しない）
}

// NewSubmittedParams は NewSubmitted の入力。
type NewSubmittedParams struct {
	ID                report_id.ReportID
	TargetPhotobookID photobook_id.PhotobookID
	TargetSnapshot    target_snapshot.TargetSnapshot
	Reason            report_reason.ReportReason
	Detail            report_detail.ReportDetail
	ReporterContact   reporter_contact.ReporterContact
	SubmittedAt       time.Time
	SourceIPHash      []byte // 任意（nil 可）。ソルト + sha256 済みのバイト列のみ受け付ける
}

// NewSubmitted は status='submitted' の新規 Report を組み立てる。
//
// I1 / I2 / I3 / I6 を満たすことを検証する。
func NewSubmitted(p NewSubmittedParams) (Report, error) {
	if p.ID.IsZero() {
		return Report{}, report_id.ErrInvalidReportID
	}
	if p.Reason.IsZero() {
		return Report{}, report_reason.ErrInvalidReportReason
	}
	if p.SubmittedAt.IsZero() {
		return Report{}, ErrInvalidSubmittedAt
	}
	if p.TargetSnapshot.IsZero() {
		return Report{}, target_snapshot.ErrInvalidTitle // I6: snapshot 必須
	}
	return Report{
		id:                p.ID,
		targetPhotobookID: p.TargetPhotobookID,
		targetSnapshot:    p.TargetSnapshot,
		reason:            p.Reason,
		detail:            p.Detail,
		reporterContact:   p.ReporterContact,
		status:            report_status.Submitted(),
		submittedAt:       p.SubmittedAt,
		sourceIPHash:      p.SourceIPHash,
	}, nil
}

// アクセサ。
func (r Report) ID() report_id.ReportID                            { return r.id }
func (r Report) TargetPhotobookID() photobook_id.PhotobookID       { return r.targetPhotobookID }
func (r Report) TargetSnapshot() target_snapshot.TargetSnapshot    { return r.targetSnapshot }
func (r Report) Reason() report_reason.ReportReason                { return r.reason }
func (r Report) Detail() report_detail.ReportDetail                { return r.detail }
func (r Report) ReporterContact() reporter_contact.ReporterContact { return r.reporterContact }
func (r Report) Status() report_status.ReportStatus                { return r.status }
func (r Report) SubmittedAt() time.Time                            { return r.submittedAt }
func (r Report) ReviewedAt() *time.Time                            { return r.reviewedAt }
func (r Report) ResolvedAt() *time.Time                            { return r.resolvedAt }
func (r Report) ResolutionNote() *string                           { return r.resolutionNote }
func (r Report) ResolvedByModerationActionID() *action_id.ActionID { return r.resolvedByModerationActionID }
func (r Report) SourceIPHash() []byte                              { return r.sourceIPHash }
