package action_reason_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/moderation/domain/vo/action_reason"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		wantStr     string
		wantErr     bool
	}{
		{name: "正常_harassment", description: "Given: harassment, When: Parse, Then: ok", in: "report_based_harassment", wantStr: "report_based_harassment"},
		{name: "正常_unauthorized_repost", description: "Given: 無断転載, When: Parse, Then: ok", in: "report_based_unauthorized_repost", wantStr: "report_based_unauthorized_repost"},
		{name: "正常_sensitive_violation", description: "Given: センシティブ違反, When: Parse, Then: ok", in: "report_based_sensitive_violation", wantStr: "report_based_sensitive_violation"},
		{name: "正常_minor_related", description: "Given: 未成年関連, When: Parse, Then: ok", in: "report_based_minor_related", wantStr: "report_based_minor_related"},
		{name: "正常_subject_removal", description: "Given: 被写体削除依頼, When: Parse, Then: ok", in: "report_based_subject_removal", wantStr: "report_based_subject_removal"},
		{name: "正常_rights_claim", description: "Given: 権利侵害申立て, When: Parse, Then: ok", in: "rights_claim", wantStr: "rights_claim"},
		{name: "正常_creator_request_manage_url_reissue", description: "Given: 作成者からの再発行, When: Parse, Then: ok", in: "creator_request_manage_url_reissue", wantStr: "creator_request_manage_url_reissue"},
		{name: "正常_erroneous_action_correction", description: "Given: 誤操作補正, When: Parse, Then: ok", in: "erroneous_action_correction", wantStr: "erroneous_action_correction"},
		{name: "正常_policy_violation_other", description: "Given: その他規約違反, When: Parse, Then: ok", in: "policy_violation_other", wantStr: "policy_violation_other"},
		{name: "異常_未知", description: "Given: spam, When: Parse, Then: ErrInvalidActionReason", in: "spam", wantErr: true},
		{name: "異常_空", description: "Given: '', When: Parse, Then: ErrInvalidActionReason", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := action_reason.Parse(tt.in)
			if tt.wantErr {
				if !errors.Is(err, action_reason.ErrInvalidActionReason) {
					t.Errorf("expected ErrInvalidActionReason, got %v", err)
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
