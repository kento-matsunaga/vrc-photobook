package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/report/domain/entity"
	"vrcpb/backend/internal/report/domain/vo/report_detail"
	"vrcpb/backend/internal/report/domain/vo/report_id"
	"vrcpb/backend/internal/report/domain/vo/report_reason"
	"vrcpb/backend/internal/report/domain/vo/reporter_contact"
	"vrcpb/backend/internal/report/domain/vo/target_snapshot"
)

func mustSnapshot(t *testing.T) target_snapshot.TargetSnapshot {
	t.Helper()
	creator := "Tester"
	s, err := target_snapshot.New("uqfwfti7glarva5saj", "Test Title", &creator)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return s
}

func TestNewSubmitted(t *testing.T) {
	validID, _ := report_id.New()
	validPB, _ := photobook_id.FromUUID(uuid.MustParse("019dd1bb-774f-7341-91a4-fd0fbd279320"))
	validReason := report_reason.HarassmentOrDoxxing()
	validNow := time.Now().UTC()

	tests := []struct {
		name        string
		description string
		params      entity.NewSubmittedParams
		wantErrIs   error
	}{
		{
			name:        "正常_必須項目すべて",
			description: "ID + targetPB + reason + snapshot + submittedAt が揃う",
			params: entity.NewSubmittedParams{
				ID:                validID,
				TargetPhotobookID: validPB,
				TargetSnapshot:    mustSnapshot(t),
				Reason:            validReason,
				SubmittedAt:       validNow,
			},
		},
		{
			name:        "正常_detail_contact_present",
			description: "Detail / ReporterContact あり",
			params: func() entity.NewSubmittedParams {
				d, _ := report_detail.Parse("通報内容")
				c, _ := reporter_contact.Parse("contact@vrc.test")
				return entity.NewSubmittedParams{
					ID:                validID,
					TargetPhotobookID: validPB,
					TargetSnapshot:    mustSnapshot(t),
					Reason:            validReason,
					Detail:            d,
					ReporterContact:   c,
					SubmittedAt:       validNow,
				}
			}(),
		},
		{
			name:        "異常_id_zero",
			description: "ID 必須",
			params: entity.NewSubmittedParams{
				TargetPhotobookID: validPB,
				TargetSnapshot:    mustSnapshot(t),
				Reason:            validReason,
				SubmittedAt:       validNow,
			},
			wantErrIs: report_id.ErrInvalidReportID,
		},
		{
			name:        "異常_reason_zero",
			description: "Reason 必須",
			params: entity.NewSubmittedParams{
				ID:                validID,
				TargetPhotobookID: validPB,
				TargetSnapshot:    mustSnapshot(t),
				SubmittedAt:       validNow,
			},
			wantErrIs: report_reason.ErrInvalidReportReason,
		},
		{
			name:        "異常_submittedAt_zero",
			description: "SubmittedAt 必須",
			params: entity.NewSubmittedParams{
				ID:                validID,
				TargetPhotobookID: validPB,
				TargetSnapshot:    mustSnapshot(t),
				Reason:            validReason,
			},
			wantErrIs: entity.ErrInvalidSubmittedAt,
		},
		{
			name:        "異常_snapshot_zero",
			description: "Snapshot 必須（I6）",
			params: entity.NewSubmittedParams{
				ID:                validID,
				TargetPhotobookID: validPB,
				Reason:            validReason,
				SubmittedAt:       validNow,
			},
			wantErrIs: target_snapshot.ErrInvalidTitle,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := entity.NewSubmitted(tt.params)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Errorf("expected %v, got %v", tt.wantErrIs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.Status().String() != "submitted" {
				t.Errorf("status should be submitted, got %s", got.Status().String())
			}
			if got.ID().IsZero() {
				t.Error("ID should not be zero")
			}
		})
	}
}
