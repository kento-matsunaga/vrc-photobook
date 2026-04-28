package action_kind_test

import (
	"errors"
	"testing"

	"vrcpb/backend/internal/moderation/domain/vo/action_kind"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		description string
		in          string
		wantStr     string
		wantErr     bool
	}{
		{
			name:        "正常_hide",
			description: "Given: 'hide', When: Parse, Then: ActionKind.String=='hide'",
			in:          "hide",
			wantStr:     "hide",
		},
		{
			name:        "正常_unhide",
			description: "Given: 'unhide', When: Parse, Then: ActionKind.String=='unhide'",
			in:          "unhide",
			wantStr:     "unhide",
		},
		{
			name:        "正常_soft_delete",
			description: "Given: 'soft_delete', When: Parse, Then: ok",
			in:          "soft_delete",
			wantStr:     "soft_delete",
		},
		{
			name:        "正常_restore",
			description: "Given: 'restore', When: Parse, Then: ok",
			in:          "restore",
			wantStr:     "restore",
		},
		{
			name:        "正常_purge",
			description: "Given: 'purge', When: Parse, Then: ok",
			in:          "purge",
			wantStr:     "purge",
		},
		{
			name:        "正常_reissue_manage_url",
			description: "Given: 'reissue_manage_url', When: Parse, Then: ok",
			in:          "reissue_manage_url",
			wantStr:     "reissue_manage_url",
		},
		{
			name:        "異常_未知の文字列",
			description: "Given: 不明値, When: Parse, Then: ErrInvalidActionKind",
			in:          "delete",
			wantErr:     true,
		},
		{
			name:        "異常_空文字",
			description: "Given: '', When: Parse, Then: ErrInvalidActionKind",
			in:          "",
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := action_kind.Parse(tt.in)
			if tt.wantErr {
				if !errors.Is(err, action_kind.ErrInvalidActionKind) {
					t.Errorf("expected ErrInvalidActionKind, got %v", err)
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

func TestConstructors(t *testing.T) {
	tests := []struct {
		name string
		k    action_kind.ActionKind
		want string
	}{
		{name: "Hide", k: action_kind.Hide(), want: "hide"},
		{name: "Unhide", k: action_kind.Unhide(), want: "unhide"},
		{name: "SoftDelete", k: action_kind.SoftDelete(), want: "soft_delete"},
		{name: "Restore", k: action_kind.Restore(), want: "restore"},
		{name: "Purge", k: action_kind.Purge(), want: "purge"},
		{name: "ReissueManageURL", k: action_kind.ReissueManageURL(), want: "reissue_manage_url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.k.String() != tt.want {
				t.Errorf("got %q want %q", tt.k.String(), tt.want)
			}
		})
	}
}

func TestEqualAndIsZero(t *testing.T) {
	a := action_kind.Hide()
	b := action_kind.Hide()
	c := action_kind.Unhide()
	if !a.Equal(b) {
		t.Error("Hide==Hide should be Equal")
	}
	if a.Equal(c) {
		t.Error("Hide==Unhide should NOT be Equal")
	}
	var zero action_kind.ActionKind
	if !zero.IsZero() {
		t.Error("zero value should IsZero=true")
	}
	if a.IsZero() {
		t.Error("Hide IsZero should be false")
	}
}
