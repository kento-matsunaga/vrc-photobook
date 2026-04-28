package report_reason_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/report/domain/vo/report_reason"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		wantStr     string
		wantErr     bool
	}{
		{name: "正常_subject_removal_request", description: "Given: subject_removal_request, When: Parse, Then: ok", in: "subject_removal_request", wantStr: "subject_removal_request"},
		{name: "正常_unauthorized_repost", description: "Given: unauthorized_repost, When: Parse, Then: ok", in: "unauthorized_repost", wantStr: "unauthorized_repost"},
		{name: "正常_sensitive_flag_missing", description: "Given: sensitive_flag_missing, When: Parse, Then: ok", in: "sensitive_flag_missing", wantStr: "sensitive_flag_missing"},
		{name: "正常_harassment_or_doxxing", description: "Given: harassment_or_doxxing, When: Parse, Then: ok", in: "harassment_or_doxxing", wantStr: "harassment_or_doxxing"},
		{name: "正常_minor_safety_concern", description: "Given: minor_safety_concern, When: Parse, Then: ok", in: "minor_safety_concern", wantStr: "minor_safety_concern"},
		{name: "正常_other", description: "Given: other, When: Parse, Then: ok", in: "other", wantStr: "other"},
		{name: "異常_未知", description: "Given: spam, When: Parse, Then: ErrInvalidReportReason", in: "spam", wantErr: true},
		{name: "異常_空", description: "Given: '', When: Parse, Then: ErrInvalidReportReason", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := report_reason.Parse(tt.in)
			if tt.wantErr {
				if !errors.Is(err, report_reason.ErrInvalidReportReason) {
					t.Errorf("expected ErrInvalidReportReason, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.String() != tt.wantStr {
				t.Errorf("got %q want %q", got.String(), tt.wantStr)
			}
		})
	}
}

func TestIsMinorSafetyConcern(t *testing.T) {
	if !report_reason.MinorSafetyConcern().IsMinorSafetyConcern() {
		t.Error("MinorSafetyConcern should be true")
	}
	if report_reason.Other().IsMinorSafetyConcern() {
		t.Error("Other should be false")
	}
	if report_reason.HarassmentOrDoxxing().IsMinorSafetyConcern() {
		t.Error("HarassmentOrDoxxing should be false")
	}
}
