package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/moderation/domain/entity"
	"vrcpb/backend/internal/moderation/domain/vo/action_detail"
	"vrcpb/backend/internal/moderation/domain/vo/action_id"
	"vrcpb/backend/internal/moderation/domain/vo/action_kind"
	"vrcpb/backend/internal/moderation/domain/vo/action_reason"
	"vrcpb/backend/internal/moderation/domain/vo/operator_label"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

func mustOperator(t *testing.T, s string) operator_label.OperatorLabel {
	t.Helper()
	v, err := operator_label.Parse(s)
	if err != nil {
		t.Fatalf("operator_label.Parse: %v", err)
	}
	return v
}

func TestNew(t *testing.T) {
	validID, _ := action_id.New()
	validPB, _ := photobook_id.FromUUID(uuid.MustParse("019dd1bb-774f-7341-91a4-fd0fbd279320"))
	validKind := action_kind.Hide()
	validReason := action_reason.PolicyViolationOther()
	validOperator := mustOperator(t, "ops-1")
	validExecutedAt := time.Now()
	validDetail, _ := action_detail.Parse("test detail")

	tests := []struct {
		name        string
		description string
		params      entity.NewParams
		wantErrIs   error
	}{
		{
			name:        "正常_必須項目すべて埋まる",
			description: "Given: 必須5項目すべて, When: New, Then: ok",
			params: entity.NewParams{
				ID:         validID,
				Kind:       validKind,
				TargetID:   validPB,
				ActorLabel: validOperator,
				Reason:     validReason,
				Detail:     validDetail,
				ExecutedAt: validExecutedAt,
			},
		},
		{
			name:        "正常_detail_None_でも作れる",
			description: "Given: detail 未指定（None）, When: New, Then: ok",
			params: entity.NewParams{
				ID:         validID,
				Kind:       validKind,
				TargetID:   validPB,
				ActorLabel: validOperator,
				Reason:     validReason,
				Detail:     action_detail.None(),
				ExecutedAt: validExecutedAt,
			},
		},
		{
			name:        "異常_id_zero",
			description: "Given: ID zero, When: New, Then: ErrInvalidActionID",
			params: entity.NewParams{
				Kind:       validKind,
				TargetID:   validPB,
				ActorLabel: validOperator,
				Reason:     validReason,
				ExecutedAt: validExecutedAt,
			},
			wantErrIs: action_id.ErrInvalidActionID,
		},
		{
			name:        "異常_kind_zero",
			description: "Given: kind zero, When: New, Then: ErrInvalidActionKind",
			params: entity.NewParams{
				ID:         validID,
				TargetID:   validPB,
				ActorLabel: validOperator,
				Reason:     validReason,
				ExecutedAt: validExecutedAt,
			},
			wantErrIs: action_kind.ErrInvalidActionKind,
		},
		{
			name:        "異常_actor_label_zero",
			description: "Given: actor zero, When: New, Then: ErrInvalidOperatorLabel",
			params: entity.NewParams{
				ID:         validID,
				Kind:       validKind,
				TargetID:   validPB,
				Reason:     validReason,
				ExecutedAt: validExecutedAt,
			},
			wantErrIs: operator_label.ErrInvalidOperatorLabel,
		},
		{
			name:        "異常_reason_zero",
			description: "Given: reason zero, When: New, Then: ErrInvalidActionReason",
			params: entity.NewParams{
				ID:         validID,
				Kind:       validKind,
				TargetID:   validPB,
				ActorLabel: validOperator,
				ExecutedAt: validExecutedAt,
			},
			wantErrIs: action_reason.ErrInvalidActionReason,
		},
		{
			name:        "異常_executed_at_zero",
			description: "Given: executedAt zero, When: New, Then: ErrInvalidExecutedAt",
			params: entity.NewParams{
				ID:         validID,
				Kind:       validKind,
				TargetID:   validPB,
				ActorLabel: validOperator,
				Reason:     validReason,
			},
			wantErrIs: entity.ErrInvalidExecutedAt,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := entity.New(tt.params)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Errorf("expected %v, got %v", tt.wantErrIs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.ID().IsZero() {
				t.Error("ID should not be zero")
			}
			if got.Kind().String() != tt.params.Kind.String() {
				t.Errorf("kind mismatch: %q vs %q", got.Kind().String(), tt.params.Kind.String())
			}
			if got.ActorLabel().String() != tt.params.ActorLabel.String() {
				t.Errorf("actor mismatch")
			}
		})
	}
}
