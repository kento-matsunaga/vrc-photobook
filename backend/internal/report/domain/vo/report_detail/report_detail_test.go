package report_detail_test

import (
	"errors"
	"strings"
	"testing"

	"vrcpb/backend/internal/report/domain/vo/report_detail"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		wantPresent bool
		wantStr     string
		wantErrIs   error
	}{
		{name: "正常_空文字_None", description: "Given: '', When: Parse, Then: Present=false", in: "", wantPresent: false, wantStr: ""},
		{name: "正常_短い文字列", description: "Given: '通報内容', When: Parse, Then: Present=true", in: "通報内容", wantPresent: true, wantStr: "通報内容"},
		{name: "正常_改行_タブ_許可", description: "Given: 改行・タブ, When: Parse, Then: ok", in: "line1\nline2\ttab", wantPresent: true, wantStr: "line1\nline2\ttab"},
		{name: "正常_2000_rune_境界", description: "Given: 2000 rune, When: Parse, Then: ok", in: strings.Repeat("a", 2000), wantPresent: true, wantStr: strings.Repeat("a", 2000)},
		{name: "異常_2001_rune_超過", description: "Given: 2001 rune, When: Parse, Then: ErrTooLong", in: strings.Repeat("a", 2001), wantErrIs: report_detail.ErrTooLong},
		{name: "異常_NULL_制御文字", description: "Given: \\x00 含む, When: Parse, Then: ErrControlChar", in: "abc\x00def", wantErrIs: report_detail.ErrControlCharInDetail},
		{name: "異常_DEL_制御文字", description: "Given: \\x7F 含む, When: Parse, Then: ErrControlChar", in: "abc\x7Fdef", wantErrIs: report_detail.ErrControlCharInDetail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := report_detail.Parse(tt.in)
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
			if got.String() != tt.wantStr {
				t.Errorf("String()=%q want %q", got.String(), tt.wantStr)
			}
		})
	}
}
