// scope_hash の単体テスト。テーブル駆動 + description（`.agents/rules/testing.md`）。
//
// セキュリティ: テスト中も raw hash 完全値を log に出さない。Redacted() / Prefix() を使う。
package scope_hash

import (
	"errors"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		wantErr     bool
	}{
		{
			name:        "正常_64hex_sha256想定",
			description: "Given: 64 hex chars, When: Parse, Then: 成功",
			in:          strings.Repeat("a", 64),
		},
		{
			name:        "正常_uuid_dash付き_36char",
			description: "Given: UUID dash 形式 36 char, When: Parse, Then: 成功",
			in:          "019dd1bb-774f-7341-91a4-fd0fbd279320",
		},
		{
			name:        "正常_最小長8",
			description: "Given: 8 文字, When: Parse, Then: 成功（境界 OK）",
			in:          "01234567",
		},
		{
			name:        "正常_最大長128",
			description: "Given: 128 文字, When: Parse, Then: 成功（境界 OK）",
			in:          strings.Repeat("0", 128),
		},
		{
			name:        "正常_前後空白trim",
			description: "Given: 前後 whitespace, When: Parse, Then: trim 後成功",
			in:          "  " + strings.Repeat("a", 16) + "  ",
		},
		{
			name:        "異常_空文字列",
			description: "Given: empty, When: Parse, Then: ErrInvalidScopeHash",
			in:          "",
			wantErr:     true,
		},
		{
			name:        "異常_空白のみ",
			description: "Given: \"   \", When: Parse, Then: ErrInvalidScopeHash",
			in:          "   ",
			wantErr:     true,
		},
		{
			name:        "異常_短すぎる",
			description: "Given: 7 文字, When: Parse, Then: ErrInvalidScopeHash",
			in:          "0123456",
			wantErr:     true,
		},
		{
			name:        "異常_長すぎる",
			description: "Given: 129 文字, When: Parse, Then: ErrInvalidScopeHash",
			in:          strings.Repeat("0", 129),
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in)
			if tt.wantErr {
				if err == nil || !errors.Is(err, ErrInvalidScopeHash) {
					t.Fatalf("err = %v want ErrInvalidScopeHash", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v want nil", err)
			}
			if got.IsZero() {
				t.Fatalf("got is zero")
			}
			// 完全値はテストでも assertion ログに出さない（Redacted を使う）
			if got.Redacted() == "" {
				t.Fatalf("Redacted = empty")
			}
		})
	}
}

func TestRedactedAndPrefix(t *testing.T) {
	tests := []struct {
		name         string
		description  string
		in           string
		wantPrefix   string
		wantRedacted string
	}{
		{
			name:         "正常_長い文字列はprefixのみ",
			description:  "Given: 32 文字, When: Prefix/Redacted, Then: 先頭 8 文字 / \"<prefix>...\"",
			in:           "0123456789abcdef0123456789abcdef",
			wantPrefix:   "01234567",
			wantRedacted: "01234567...",
		},
		{
			name:         "正常_短い文字列もprefixで切らない",
			description:  "Given: 8 文字（境界）, When: Prefix, Then: そのまま",
			in:           "01234567",
			wantPrefix:   "01234567",
			wantRedacted: "01234567...",
		},
		{
			name:         "正常_ゼロ値はempty表示",
			description:  "Given: zero, When: Redacted, Then: \"<empty>\"",
			in:           "",
			wantPrefix:   "",
			wantRedacted: "<empty>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h ScopeHash
			if tt.in != "" {
				parsed, err := Parse(tt.in)
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				h = parsed
			}
			if got := h.Prefix(); got != tt.wantPrefix {
				t.Errorf("Prefix = %q want %q", got, tt.wantPrefix)
			}
			if got := h.Redacted(); got != tt.wantRedacted {
				t.Errorf("Redacted = %q want %q", got, tt.wantRedacted)
			}
		})
	}
}
