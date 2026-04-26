package cookie_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/auth/session/cookie"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
)

func mustPID(t *testing.T) photobook_id.PhotobookID {
	t.Helper()
	pid, err := photobook_id.FromUUID(uuid.New())
	if err != nil {
		t.Fatalf("photobook_id.FromUUID: %v", err)
	}
	return pid
}

func TestName(t *testing.T) {
	t.Parallel()

	pid := mustPID(t)

	tests := []struct {
		name        string
		description string
		st          session_type.SessionType
		wantPrefix  string
	}{
		{
			name:        "正常_draftはvrcpb_draft_prefix",
			description: "Given: type=draft, pid=<uuid>, When: Name, Then: 'vrcpb_draft_<uuid>'",
			st:          session_type.Draft(),
			wantPrefix:  "vrcpb_draft_",
		},
		{
			name:        "正常_manageはvrcpb_manage_prefix",
			description: "Given: type=manage, pid=<uuid>, When: Name, Then: 'vrcpb_manage_<uuid>'",
			st:          session_type.Manage(),
			wantPrefix:  "vrcpb_manage_",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cookie.Name(tt.st, pid)
			want := tt.wantPrefix + pid.String()
			if got != want {
				t.Fatalf("Name = %q, want %q", got, want)
			}
		})
	}
}

func TestBuildIssue(t *testing.T) {
	t.Parallel()

	pid := mustPID(t)
	tok, err := session_token.Generate()
	if err != nil {
		t.Fatalf("session_token.Generate: %v", err)
	}
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		description string
		policy      cookie.Policy
		expiresAt   time.Time
		wantDomain  string
		wantErr     error
	}{
		{
			name:        "正常_Domain未設定",
			description: "Given: COOKIE_DOMAIN=空, When: BuildIssue, Then: Domain属性は空（host-only Cookie）",
			policy:      cookie.Policy{Domain: ""},
			expiresAt:   now.Add(7 * 24 * time.Hour),
			wantDomain:  "",
		},
		{
			name:        "正常_Domain指定",
			description: "Given: Domain=.vrc-photobook.com, When: BuildIssue, Then: Domain属性が反映",
			policy:      cookie.Policy{Domain: ".vrc-photobook.com"},
			expiresAt:   now.Add(24 * time.Hour),
			wantDomain:  ".vrc-photobook.com",
		},
		{
			name:        "異常_expiresAt_=_now",
			description: "Given: expires=now, When: BuildIssue, Then: ErrInvalidExpiry",
			policy:      cookie.Policy{},
			expiresAt:   now,
			wantErr:     cookie.ErrInvalidExpiry,
		},
		{
			name:        "異常_expiresAt_<_now",
			description: "Given: expires<now, When: BuildIssue, Then: ErrInvalidExpiry",
			policy:      cookie.Policy{},
			expiresAt:   now.Add(-1 * time.Second),
			wantErr:     cookie.ErrInvalidExpiry,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := tt.policy.BuildIssue(session_type.Draft(), pid, tok, now, tt.expiresAt)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !c.HttpOnly {
				t.Error("HttpOnly must be true")
			}
			if !c.Secure {
				t.Error("Secure must be true")
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Errorf("SameSite = %v, want Strict", c.SameSite)
			}
			if c.Path != "/" {
				t.Errorf("Path = %q, want /", c.Path)
			}
			if c.Domain != tt.wantDomain {
				t.Errorf("Domain = %q, want %q", c.Domain, tt.wantDomain)
			}
			if c.MaxAge <= 0 {
				t.Errorf("MaxAge = %d, must be > 0", c.MaxAge)
			}
			if !strings.HasPrefix(c.Name, "vrcpb_draft_") {
				t.Errorf("Name = %q, must start with vrcpb_draft_", c.Name)
			}
			if c.Value == "" {
				t.Error("Value must not be empty")
			}
			if err := cookie.AssertSecureAttributes(c); err != nil {
				t.Errorf("AssertSecureAttributes: %v", err)
			}
		})
	}
}

func TestBuildClear(t *testing.T) {
	t.Parallel()

	pid := mustPID(t)
	tests := []struct {
		name        string
		description string
		policy      cookie.Policy
		st          session_type.SessionType
	}{
		{
			name:        "正常_draft_clear",
			description: "Given: draft, When: BuildClear, Then: MaxAge=-1 / Value 空 / Secure 属性が立つ",
			policy:      cookie.Policy{Domain: ".example.com"},
			st:          session_type.Draft(),
		},
		{
			name:        "正常_manage_clear",
			description: "Given: manage, When: BuildClear, Then: MaxAge=-1 / Value 空",
			policy:      cookie.Policy{},
			st:          session_type.Manage(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.policy.BuildClear(tt.st, pid)
			if c.Value != "" {
				t.Errorf("Value = %q, want empty", c.Value)
			}
			if c.MaxAge != -1 {
				t.Errorf("MaxAge = %d, want -1", c.MaxAge)
			}
			if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
				t.Error("Secure attributes must be set on clear cookie too")
			}
			if err := cookie.AssertSecureAttributes(c); err != nil {
				t.Errorf("AssertSecureAttributes: %v", err)
			}
		})
	}
}
