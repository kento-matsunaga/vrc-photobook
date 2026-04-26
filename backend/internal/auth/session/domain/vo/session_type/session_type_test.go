package session_type_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		input       string
		wantDraft   bool
		wantManage  bool
		wantErr     error
	}{
		{
			name:        "正常_draft",
			description: "Given: 'draft', When: Parse, Then: IsDraft=true",
			input:       "draft",
			wantDraft:   true,
		},
		{
			name:        "正常_manage",
			description: "Given: 'manage', When: Parse, Then: IsManage=true",
			input:       "manage",
			wantManage:  true,
		},
		{
			name:        "異常_未知",
			description: "Given: 'admin', When: Parse, Then: ErrInvalidSessionType",
			input:       "admin",
			wantErr:     session_type.ErrInvalidSessionType,
		},
		{
			name:        "異常_空",
			description: "Given: '', When: Parse, Then: ErrInvalidSessionType",
			input:       "",
			wantErr:     session_type.ErrInvalidSessionType,
		},
		{
			name:        "異常_大文字混入",
			description: "Given: 'Draft', When: Parse, Then: ErrInvalidSessionType",
			input:       "Draft",
			wantErr:     session_type.ErrInvalidSessionType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := session_type.Parse(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := st.IsDraft(); got != tt.wantDraft {
				t.Errorf("IsDraft = %v, want %v", got, tt.wantDraft)
			}
			if got := st.IsManage(); got != tt.wantManage {
				t.Errorf("IsManage = %v, want %v", got, tt.wantManage)
			}
			if got := st.String(); got != tt.input {
				t.Errorf("String = %q, want %q", got, tt.input)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		a, b        session_type.SessionType
		want        bool
	}{
		{
			name:        "正常_draft同士は等しい",
			description: "Given: Draft, Draft, When: Equal, Then: true",
			a:           session_type.Draft(),
			b:           session_type.Draft(),
			want:        true,
		},
		{
			name:        "正常_manage同士は等しい",
			description: "Given: Manage, Manage, When: Equal, Then: true",
			a:           session_type.Manage(),
			b:           session_type.Manage(),
			want:        true,
		},
		{
			name:        "正常_draftとmanageは等しくない",
			description: "Given: Draft, Manage, When: Equal, Then: false",
			a:           session_type.Draft(),
			b:           session_type.Manage(),
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Fatalf("Equal = %v, want %v", got, tt.want)
			}
		})
	}
}
