// Package entity は Moderation 集約のエンティティ。
//
// 設計参照:
//   - docs/design/aggregates/moderation/ドメイン設計.md §3 / §5
//   - docs/plan/m2-moderation-ops-plan.md
//
// ModerationAction は **追記型・イミュータブル**（v4 不変条件 I2 / I3）。
// 生成後の属性変更は認めない。補正は新規 ModerationAction として追加する。
package entity

import (
	"errors"
	"time"

	"vrcpb/backend/internal/moderation/domain/vo/action_detail"
	"vrcpb/backend/internal/moderation/domain/vo/action_id"
	"vrcpb/backend/internal/moderation/domain/vo/action_kind"
	"vrcpb/backend/internal/moderation/domain/vo/action_reason"
	"vrcpb/backend/internal/moderation/domain/vo/operator_label"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// ErrInvalidExecutedAt は executedAt が zero value のときのエラー。
var ErrInvalidExecutedAt = errors.New("moderation action executed_at must be non-zero")

// ModerationAction は集約ルート。生成後 immutable。
type ModerationAction struct {
	id              action_id.ActionID
	kind            action_kind.ActionKind
	targetID        photobook_id.PhotobookID
	sourceReportID  *action_id.ActionID // PR34b では常に nil（PR35 接続後に有効化）
	actorLabel      operator_label.OperatorLabel
	reason          action_reason.ActionReason
	detail          action_detail.ActionDetail
	correlationID   *action_id.ActionID
	executedAt      time.Time
}

// NewParams は New 関数の入力。
type NewParams struct {
	ID             action_id.ActionID
	Kind           action_kind.ActionKind
	TargetID       photobook_id.PhotobookID
	SourceReportID *action_id.ActionID
	ActorLabel     operator_label.OperatorLabel
	Reason         action_reason.ActionReason
	Detail         action_detail.ActionDetail
	CorrelationID  *action_id.ActionID
	ExecutedAt     time.Time
}

// New は ModerationAction を組み立てる。生成後の状態変更は許容しない。
//
// I1: kind / targetPhotobookId / actorLabel / reason / executedAt は必須。
func New(p NewParams) (ModerationAction, error) {
	if p.ID.IsZero() {
		return ModerationAction{}, action_id.ErrInvalidActionID
	}
	if p.Kind.IsZero() {
		return ModerationAction{}, action_kind.ErrInvalidActionKind
	}
	if p.ActorLabel.IsZero() {
		return ModerationAction{}, operator_label.ErrInvalidOperatorLabel
	}
	if p.Reason.IsZero() {
		return ModerationAction{}, action_reason.ErrInvalidActionReason
	}
	if p.ExecutedAt.IsZero() {
		return ModerationAction{}, ErrInvalidExecutedAt
	}
	return ModerationAction{
		id:             p.ID,
		kind:           p.Kind,
		targetID:       p.TargetID,
		sourceReportID: p.SourceReportID,
		actorLabel:     p.ActorLabel,
		reason:         p.Reason,
		detail:         p.Detail,
		correlationID:  p.CorrelationID,
		executedAt:     p.ExecutedAt,
	}, nil
}

// アクセサ（フィールドは export しない、Repository / marshaller がここから値を取る）。
func (m ModerationAction) ID() action_id.ActionID                   { return m.id }
func (m ModerationAction) Kind() action_kind.ActionKind             { return m.kind }
func (m ModerationAction) TargetID() photobook_id.PhotobookID       { return m.targetID }
func (m ModerationAction) SourceReportID() *action_id.ActionID      { return m.sourceReportID }
func (m ModerationAction) ActorLabel() operator_label.OperatorLabel { return m.actorLabel }
func (m ModerationAction) Reason() action_reason.ActionReason       { return m.reason }
func (m ModerationAction) Detail() action_detail.ActionDetail       { return m.detail }
func (m ModerationAction) CorrelationID() *action_id.ActionID       { return m.correlationID }
func (m ModerationAction) ExecutedAt() time.Time                    { return m.executedAt }
