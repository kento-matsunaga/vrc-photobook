// scope_type の単体テスト。テーブル駆動 + description（`.agents/rules/testing.md`）。
package scope_type

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		want        ScopeType
		wantErr     bool
	}{
		{
			name:        "正常_source_ip_hash",
			description: "Given: \"source_ip_hash\", When: Parse, Then: SourceIPHash",
			in:          "source_ip_hash",
			want:        SourceIPHash(),
		},
		{
			name:        "正常_draft_session_id",
			description: "Given: \"draft_session_id\", When: Parse, Then: DraftSessionID",
			in:          "draft_session_id",
			want:        DraftSessionID(),
		},
		{
			name:        "正常_manage_session_id",
			description: "Given: \"manage_session_id\", When: Parse, Then: ManageSessionID",
			in:          "manage_session_id",
			want:        ManageSessionID(),
		},
		{
			name:        "正常_photobook_id",
			description: "Given: \"photobook_id\", When: Parse, Then: PhotobookID",
			in:          "photobook_id",
			want:        PhotobookID(),
		},
		{
			name:        "異常_空文字列",
			description: "Given: empty, When: Parse, Then: ErrInvalidScopeType",
			in:          "",
			wantErr:     true,
		},
		{
			name:        "異常_未知の文字列",
			description: "Given: \"unknown\", When: Parse, Then: ErrInvalidScopeType",
			in:          "unknown",
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in)
			if tt.wantErr {
				if err == nil || !errors.Is(err, ErrInvalidScopeType) {
					t.Fatalf("err = %v want ErrInvalidScopeType", err)
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

func TestEqualAndIsZero(t *testing.T) {
	tests := []struct {
		name        string
		description string
		a           ScopeType
		b           ScopeType
		equal       bool
		aIsZero     bool
	}{
		{
			name:        "正常_同種は等価",
			description: "SourceIPHash と SourceIPHash は等価、ゼロ値ではない",
			a:           SourceIPHash(),
			b:           SourceIPHash(),
			equal:       true,
			aIsZero:     false,
		},
		{
			name:        "正常_異種は非等価",
			description: "SourceIPHash と DraftSessionID は非等価",
			a:           SourceIPHash(),
			b:           DraftSessionID(),
			equal:       false,
		},
		{
			name:        "正常_ゼロ値検出",
			description: "未初期化 ScopeType は IsZero=true、non-zero とは非等価",
			a:           ScopeType{},
			b:           SourceIPHash(),
			equal:       false,
			aIsZero:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.equal {
				t.Errorf("Equal = %v want %v", got, tt.equal)
			}
			if got := tt.a.IsZero(); got != tt.aIsZero {
				t.Errorf("IsZero = %v want %v", got, tt.aIsZero)
			}
		})
	}
}
