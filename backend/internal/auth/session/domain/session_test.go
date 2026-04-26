package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/auth/session/domain"
	domaintests "vrcpb/backend/internal/auth/session/domain/tests"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
)

// helpers — コンストラクタテストでは Builder を使わず、引数を直接組み立てる（testing.md §コンストラクタテスト）。

func newID(t *testing.T) session_id.SessionID {
	t.Helper()
	id, err := session_id.New()
	if err != nil {
		t.Fatalf("session_id.New: %v", err)
	}
	return id
}

func newHash(t *testing.T) session_token_hash.SessionTokenHash {
	t.Helper()
	tok, err := session_token.Generate()
	if err != nil {
		t.Fatalf("session_token.Generate: %v", err)
	}
	return session_token_hash.Of(tok)
}

func newPhotobookID(t *testing.T) photobook_id.PhotobookID {
	t.Helper()
	pid, err := photobook_id.FromUUID(uuid.New())
	if err != nil {
		t.Fatalf("photobook_id.FromUUID: %v", err)
	}
	return pid
}

func TestNewSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		description string
		modify      func(p *domain.NewSessionParams)
		wantErr     error
	}{
		{
			name:        "正常_draft_既定値",
			description: "Given: type=draft, expires>created, version=0, When: NewSession, Then: エラーなし",
		},
		{
			name:        "正常_manage_version3",
			description: "Given: type=manage, version=3, When: NewSession, Then: エラーなし",
			modify: func(p *domain.NewSessionParams) {
				p.SessionType = session_type.Manage()
				v, _ := token_version_at_issue.New(3)
				p.TokenVersionAtIssue = v
			},
		},
		{
			name:        "異常_expires_at_=_created_at",
			description: "Given: expires_at == created_at, When: NewSession, Then: ErrExpiresBeforeCreated",
			modify: func(p *domain.NewSessionParams) {
				p.ExpiresAt = p.CreatedAt
			},
			wantErr: domain.ErrExpiresBeforeCreated,
		},
		{
			name:        "異常_expires_at_<_created_at",
			description: "Given: expires_at < created_at, When: NewSession, Then: ErrExpiresBeforeCreated",
			modify: func(p *domain.NewSessionParams) {
				p.ExpiresAt = p.CreatedAt.Add(-1 * time.Second)
			},
			wantErr: domain.ErrExpiresBeforeCreated,
		},
		{
			name:        "異常_draft_なのにversion!=0",
			description: "Given: type=draft, version=1, When: NewSession, Then: ErrDraftMustHaveZeroVersion",
			modify: func(p *domain.NewSessionParams) {
				v, _ := token_version_at_issue.New(1)
				p.TokenVersionAtIssue = v
			},
			wantErr: domain.ErrDraftMustHaveZeroVersion,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := domain.NewSessionParams{
				ID:                  newID(t),
				TokenHash:           newHash(t),
				SessionType:         session_type.Draft(),
				PhotobookID:         newPhotobookID(t),
				TokenVersionAtIssue: token_version_at_issue.Zero(),
				CreatedAt:           now,
				ExpiresAt:           now.Add(24 * time.Hour),
			}
			if tt.modify != nil {
				tt.modify(&p)
			}
			s, err := domain.NewSession(p)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !s.ID().Equal(p.ID) {
				t.Errorf("ID mismatch")
			}
			if s.LastUsedAt() != nil {
				t.Errorf("LastUsedAt must be nil on creation")
			}
			if s.RevokedAt() != nil {
				t.Errorf("RevokedAt must be nil on creation")
			}
		})
	}
}

func TestRestoreSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	used := now.Add(1 * time.Hour)
	revoked := now.Add(2 * time.Hour)

	tests := []struct {
		name        string
		description string
		modify      func(p *domain.RestoreSessionParams)
		wantErr     error
	}{
		{
			name:        "正常_LastUsed_in_range",
			description: "Given: last_used が created..expires の範囲内, When: Restore, Then: エラーなし",
			modify: func(p *domain.RestoreSessionParams) {
				p.LastUsedAt = &used
			},
		},
		{
			name:        "異常_LastUsed_before_created",
			description: "Given: last_used < created, When: Restore, Then: ErrLastUsedOutOfRange",
			modify: func(p *domain.RestoreSessionParams) {
				bad := now.Add(-1 * time.Hour)
				p.LastUsedAt = &bad
			},
			wantErr: domain.ErrLastUsedOutOfRange,
		},
		{
			name:        "異常_LastUsed_after_expires",
			description: "Given: last_used > expires, When: Restore, Then: ErrLastUsedOutOfRange",
			modify: func(p *domain.RestoreSessionParams) {
				bad := now.Add(48 * time.Hour)
				p.LastUsedAt = &bad
			},
			wantErr: domain.ErrLastUsedOutOfRange,
		},
		{
			name:        "正常_revoked_after_created",
			description: "Given: revoked >= created, When: Restore, Then: エラーなし",
			modify: func(p *domain.RestoreSessionParams) {
				p.RevokedAt = &revoked
			},
		},
		{
			name:        "異常_revoked_before_created",
			description: "Given: revoked < created, When: Restore, Then: ErrRevokedBeforeCreated",
			modify: func(p *domain.RestoreSessionParams) {
				bad := now.Add(-1 * time.Hour)
				p.RevokedAt = &bad
			},
			wantErr: domain.ErrRevokedBeforeCreated,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := domain.RestoreSessionParams{
				ID:                  newID(t),
				TokenHash:           newHash(t),
				SessionType:         session_type.Draft(),
				PhotobookID:         newPhotobookID(t),
				TokenVersionAtIssue: token_version_at_issue.Zero(),
				CreatedAt:           now,
				ExpiresAt:           now.Add(24 * time.Hour),
			}
			if tt.modify != nil {
				tt.modify(&p)
			}
			_, err := domain.RestoreSession(p)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestSessionStateChecks(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		description string
		build       func(t *testing.T) domain.Session
		probe       time.Time
		wantExpired bool
		wantRevoked bool
		wantActive  bool
	}{
		{
			name:        "正常_有効な範囲内",
			description: "Given: created<=now<expires, When: IsActive, Then: true",
			build: func(t *testing.T) domain.Session {
				return domaintests.NewSessionBuilder().
					WithCreatedAt(now.Add(-1 * time.Hour)).
					WithExpiresAt(now.Add(1 * time.Hour)).
					Build(t)
			},
			probe:      now,
			wantActive: true,
		},
		{
			name:        "正常_期限切れ",
			description: "Given: now=expires, When: IsExpired, Then: true",
			build: func(t *testing.T) domain.Session {
				return domaintests.NewSessionBuilder().
					WithCreatedAt(now.Add(-2 * time.Hour)).
					WithExpiresAt(now.Add(-1 * time.Hour)).
					Build(t)
			},
			probe:       now,
			wantExpired: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.build(t)
			if got := s.IsExpired(tt.probe); got != tt.wantExpired {
				t.Errorf("IsExpired = %v, want %v", got, tt.wantExpired)
			}
			if got := s.IsRevoked(); got != tt.wantRevoked {
				t.Errorf("IsRevoked = %v, want %v", got, tt.wantRevoked)
			}
			if got := s.IsActive(tt.probe); got != tt.wantActive {
				t.Errorf("IsActive = %v, want %v", got, tt.wantActive)
			}
		})
	}
}
