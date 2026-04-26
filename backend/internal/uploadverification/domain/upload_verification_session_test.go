package domain_test

import (
	"errors"
	"testing"
	"time"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/uploadverification/domain"
	"vrcpb/backend/internal/uploadverification/domain/vo/intent_count"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_id"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token_hash"
)

func newID(t *testing.T) verification_session_id.VerificationSessionID {
	t.Helper()
	id, err := verification_session_id.New()
	if err != nil {
		t.Fatalf("verification_session_id.New: %v", err)
	}
	return id
}

func newPhotobookID(t *testing.T) photobook_id.PhotobookID {
	t.Helper()
	id, err := photobook_id.New()
	if err != nil {
		t.Fatalf("photobook_id.New: %v", err)
	}
	return id
}

func newHash(t *testing.T) verification_session_token_hash.VerificationSessionTokenHash {
	t.Helper()
	tok, err := verification_session_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return verification_session_token_hash.Of(tok)
}

func TestNew(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		params  func(t *testing.T) domain.NewParams
		wantErr error
	}{
		{
			name: "正常_最小必須",
			params: func(t *testing.T) domain.NewParams {
				return domain.NewParams{
					ID:          newID(t),
					PhotobookID: newPhotobookID(t),
					TokenHash:   newHash(t),
					Allowed:     intent_count.Default(),
					Now:         now,
				}
			},
		},
		{
			name: "正常_TTL未指定で30分",
			params: func(t *testing.T) domain.NewParams {
				return domain.NewParams{
					ID:          newID(t),
					PhotobookID: newPhotobookID(t),
					TokenHash:   newHash(t),
					Allowed:     intent_count.Default(),
					Now:         now,
				}
			},
		},
		{
			name: "異常_allowed_0",
			params: func(t *testing.T) domain.NewParams {
				return domain.NewParams{
					ID:          newID(t),
					PhotobookID: newPhotobookID(t),
					TokenHash:   newHash(t),
					Allowed:     intent_count.Zero(),
					Now:         now,
				}
			},
			wantErr: domain.ErrAllowedNotPositive,
		},
		{
			name: "異常_TTL負",
			params: func(t *testing.T) domain.NewParams {
				return domain.NewParams{
					ID:          newID(t),
					PhotobookID: newPhotobookID(t),
					TokenHash:   newHash(t),
					Allowed:     intent_count.Default(),
					Now:         now,
					TTL:         -time.Second,
				}
			},
			wantErr: domain.ErrExpiresInPast,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.params(t)
			got, err := domain.New(p)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got.AllowedIntentCount().Int() != p.Allowed.Int() {
				t.Errorf("allowed mismatch")
			}
			if got.UsedIntentCount().Int() != 0 {
				t.Errorf("used must start at 0")
			}
			if !got.ExpiresAt().After(p.Now) {
				t.Errorf("expires_at must be after now")
			}
			expected := p.TTL
			if expected == 0 {
				expected = domain.DefaultTTL
			}
			if got.ExpiresAt().Sub(p.Now) != expected {
				t.Errorf("expires offset mismatch")
			}
		})
	}
}

func TestCanConsume(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	build := func(t *testing.T) domain.UploadVerificationSession {
		s, err := domain.New(domain.NewParams{
			ID:          newID(t),
			PhotobookID: newPhotobookID(t),
			TokenHash:   newHash(t),
			Allowed:     intent_count.MustNew(2),
			Now:         now,
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return s
	}

	t.Run("正常_新規はconsume可", func(t *testing.T) {
		s := build(t)
		if !s.CanConsume(now) {
			t.Error("must be consumable")
		}
	})

	t.Run("異常_期限切れはconsume不可", func(t *testing.T) {
		s := build(t)
		future := now.Add(domain.DefaultTTL + time.Minute)
		if s.CanConsume(future) {
			t.Error("expired must not be consumable")
		}
	})

	t.Run("異常_revoked後はconsume不可", func(t *testing.T) {
		s := build(t)
		revokedAt := now.Add(time.Second)
		restored, err := domain.Restore(domain.RestoreParams{
			ID:                 s.ID(),
			PhotobookID:        s.PhotobookID(),
			TokenHash:          s.TokenHash(),
			AllowedIntentCount: s.AllowedIntentCount(),
			UsedIntentCount:    s.UsedIntentCount(),
			ExpiresAt:          s.ExpiresAt(),
			CreatedAt:          s.CreatedAt(),
			RevokedAt:          &revokedAt,
		})
		if err != nil {
			t.Fatalf("Restore: %v", err)
		}
		if restored.CanConsume(now.Add(2 * time.Second)) {
			t.Error("revoked must not be consumable")
		}
		if !restored.IsRevoked() {
			t.Error("IsRevoked must be true")
		}
	})

	t.Run("異常_used==allowedはconsume不可", func(t *testing.T) {
		s := build(t)
		restored, err := domain.Restore(domain.RestoreParams{
			ID:                 s.ID(),
			PhotobookID:        s.PhotobookID(),
			TokenHash:          s.TokenHash(),
			AllowedIntentCount: s.AllowedIntentCount(),
			UsedIntentCount:    s.AllowedIntentCount(),
			ExpiresAt:          s.ExpiresAt(),
			CreatedAt:          s.CreatedAt(),
		})
		if err != nil {
			t.Fatalf("Restore: %v", err)
		}
		if restored.CanConsume(now) {
			t.Error("used up must not be consumable")
		}
	})
}

func TestRestoreInvariants(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	id := newID(t)
	pid := newPhotobookID(t)
	hash := newHash(t)

	t.Run("異常_used>allowed", func(t *testing.T) {
		_, err := domain.Restore(domain.RestoreParams{
			ID:                 id,
			PhotobookID:        pid,
			TokenHash:          hash,
			AllowedIntentCount: intent_count.MustNew(20),
			UsedIntentCount:    intent_count.MustNew(21),
			ExpiresAt:          now.Add(time.Hour),
			CreatedAt:          now,
		})
		if !errors.Is(err, domain.ErrUsedExceedsAllowed) {
			t.Fatalf("err = %v want ErrUsedExceedsAllowed", err)
		}
	})

	t.Run("異常_allowed=0", func(t *testing.T) {
		_, err := domain.Restore(domain.RestoreParams{
			ID:                 id,
			PhotobookID:        pid,
			TokenHash:          hash,
			AllowedIntentCount: intent_count.Zero(),
			UsedIntentCount:    intent_count.Zero(),
			ExpiresAt:          now.Add(time.Hour),
			CreatedAt:          now,
		})
		if !errors.Is(err, domain.ErrAllowedNotPositive) {
			t.Fatalf("err = %v want ErrAllowedNotPositive", err)
		}
	})
}
