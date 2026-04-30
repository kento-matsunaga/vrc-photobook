// action の単体テスト。テーブル駆動 + description（`.agents/rules/testing.md`）。
package action

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		want        Action
		wantErr     bool
	}{
		{
			name:        "正常_report_submit",
			description: "Given: \"report.submit\", When: Parse, Then: ReportSubmit",
			in:          "report.submit",
			want:        ReportSubmit(),
		},
		{
			name:        "正常_upload_verification_issue",
			description: "Given: \"upload_verification.issue\", When: Parse, Then: UploadVerificationIssue",
			in:          "upload_verification.issue",
			want:        UploadVerificationIssue(),
		},
		{
			name:        "正常_publish_from_draft",
			description: "Given: \"publish.from_draft\", When: Parse, Then: PublishFromDraft",
			in:          "publish.from_draft",
			want:        PublishFromDraft(),
		},
		{
			name:        "異常_空文字列",
			description: "Given: empty, When: Parse, Then: ErrInvalidAction",
			in:          "",
			wantErr:     true,
		},
		{
			name:        "異常_未知の文字列",
			description: "Given: \"unknown.action\", When: Parse, Then: ErrInvalidAction",
			in:          "unknown.action",
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in)
			if tt.wantErr {
				if err == nil || !errors.Is(err, ErrInvalidAction) {
					t.Fatalf("err = %v want ErrInvalidAction", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v want nil", err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("got = %q want %q", got.String(), tt.want.String())
			}
		})
	}
}
