package report_status_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/report/domain/vo/report_status"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		wantStr     string
		wantErr     bool
	}{
		{name: "正常_submitted", description: "Given: submitted, When: Parse, Then: ok", in: "submitted", wantStr: "submitted"},
		{name: "正常_under_review", description: "Given: under_review, When: Parse, Then: ok", in: "under_review", wantStr: "under_review"},
		{name: "正常_resolved_action_taken", description: "Given: resolved_action_taken, When: Parse, Then: ok", in: "resolved_action_taken", wantStr: "resolved_action_taken"},
		{name: "正常_resolved_no_action", description: "Given: resolved_no_action, When: Parse, Then: ok", in: "resolved_no_action", wantStr: "resolved_no_action"},
		{name: "正常_dismissed", description: "Given: dismissed, When: Parse, Then: ok", in: "dismissed", wantStr: "dismissed"},
		{name: "異常_未知", description: "Given: unknown, When: Parse, Then: error", in: "unknown", wantErr: true},
		{name: "異常_空", description: "Given: '', When: Parse, Then: error", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := report_status.Parse(tt.in)
			if tt.wantErr {
				if !errors.Is(err, report_status.ErrInvalidReportStatus) {
					t.Errorf("expected ErrInvalidReportStatus, got %v", err)
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

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		name string
		s    report_status.ReportStatus
		want bool
	}{
		{name: "submitted", s: report_status.Submitted(), want: false},
		{name: "under_review", s: report_status.UnderReview(), want: false},
		{name: "resolved_action_taken", s: report_status.ResolvedActionTaken(), want: true},
		{name: "resolved_no_action", s: report_status.ResolvedNoAction(), want: true},
		{name: "dismissed", s: report_status.Dismissed(), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.IsTerminal(); got != tt.want {
				t.Errorf("IsTerminal()=%v want %v", got, tt.want)
			}
		})
	}
}
