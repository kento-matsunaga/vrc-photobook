package reporter_contact_test

import (
	"errors"
	"strings"
	"testing"

	"vrcpb/backend/internal/report/domain/vo/reporter_contact"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		wantPresent bool
		wantErrIs   error
	}{
		{name: "正常_空_None", description: "Given: '', When: Parse, Then: Present=false", in: "", wantPresent: false},
		{name: "正常_メールアドレス", description: "Given: example@vrc.test, When: Parse, Then: Present=true", in: "example@vrc.test", wantPresent: true},
		{name: "正常_X_ID", description: "Given: @vrcphotobook, When: Parse, Then: Present=true", in: "@vrcphotobook", wantPresent: true},
		{name: "正常_自由形式", description: "Given: 自由形式 200 rune 境界, When: Parse, Then: ok", in: strings.Repeat("a", 200), wantPresent: true},
		{name: "異常_201_rune_超過", description: "Given: 201 rune, When: Parse, Then: ErrTooLong", in: strings.Repeat("a", 201), wantErrIs: reporter_contact.ErrTooLong},
		{name: "異常_改行混入", description: "Given: \\n 含む, When: Parse, Then: ErrControlChar", in: "abc\ndef", wantErrIs: reporter_contact.ErrControlCharInContact},
		{name: "異常_NULL_文字", description: "Given: \\x00 含む, When: Parse, Then: ErrControlChar", in: "x\x00y", wantErrIs: reporter_contact.ErrControlCharInContact},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reporter_contact.Parse(tt.in)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Errorf("expected %v, got %v", tt.wantErrIs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.Present() != tt.wantPresent {
				t.Errorf("Present()=%v want %v", got.Present(), tt.wantPresent)
			}
		})
	}
}
