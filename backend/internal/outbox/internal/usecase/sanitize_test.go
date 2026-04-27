// sanitizeLastError の単体テスト（DB 不要）。
//
// last_error 列は CHECK 制約で 200 char 以下、かつ Secret 値が誤って入らないよう
// worker が sanitize して書く（plan §10.8 / pr-closeout.md）。
package usecase

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeLastError(t *testing.T) {
	long := strings.Repeat("x", 500)

	tests := []struct {
		name        string
		description string
		err         error
		wantPrefix  string // 想定 prefix（"" なら whole match）
		wantMaxLen  int
		wantContain string // 想定 substring
		wantNot     []string
	}{
		{
			name:        "正常_短い通常エラーはそのまま返す",
			description: "Given: 短い error message / When: sanitize / Then: そのまま返り、200 char 以下",
			err:         errors.New("handler failed: image not found"),
			wantContain: "image not found",
			wantMaxLen:  DefaultLastErrorMax,
		},
		{
			name:        "正常_200 超は truncate",
			description: "Given: 500 char message / When: sanitize / Then: 200 以下に truncate",
			err:         errors.New(long),
			wantMaxLen:  DefaultLastErrorMax,
		},
		{
			name:        "異常_DATABASE_URL 様の値が含まれていたら全 redact",
			description: "Given: error msg に postgres:// 値 / When: sanitize / Then: [REDACTED] になる",
			err:         errors.New("dial postgres://USER:PASS@host:5432/db failed"),
			wantPrefix:  "[REDACTED]",
			wantNot:     []string{"postgres://", "USER", "PASS"},
			wantMaxLen:  DefaultLastErrorMax,
		},
		{
			name:        "異常_Bearer token を含む msg は redact",
			description: "Given: Authorization Bearer token を含む msg / When: sanitize / Then: redact",
			err:         errors.New("upstream rejected Bearer abcdefghij"),
			wantPrefix:  "[REDACTED]",
			wantNot:     []string{"abcdefghij", "Bearer"},
		},
		{
			name:        "異常_Set-Cookie 含み redact",
			description: "Given: msg に Set-Cookie / Then: redact",
			err:         errors.New("got Set-Cookie: vrcpb_draft_xxx=raw"),
			wantPrefix:  "[REDACTED]",
			wantNot:     []string{"vrcpb_draft_", "Set-Cookie"},
		},
		{
			name:        "正常_nil error は空文字",
			description: "Given: nil error / When: sanitize / Then: 空文字",
			err:         nil,
			wantContain: "",
			wantMaxLen:  DefaultLastErrorMax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeLastError(tt.err)
			if tt.err == nil {
				if got != "" {
					t.Errorf("got=%q want empty", got)
				}
				return
			}
			if tt.wantPrefix != "" && !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("prefix mismatch: got=%q want prefix=%q", got, tt.wantPrefix)
			}
			if tt.wantContain != "" && !strings.Contains(got, tt.wantContain) {
				t.Errorf("substring missing: got=%q want=%q", got, tt.wantContain)
			}
			if tt.wantMaxLen > 0 && len(got) > tt.wantMaxLen {
				t.Errorf("len(got)=%d > max %d", len(got), tt.wantMaxLen)
			}
			for _, banned := range tt.wantNot {
				if strings.Contains(got, banned) {
					t.Errorf("banned substring leaked: %q in %q", banned, got)
				}
			}
		})
	}
}
