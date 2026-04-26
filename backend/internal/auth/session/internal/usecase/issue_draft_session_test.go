package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/internal/usecase"
	"vrcpb/backend/internal/auth/session/internal/usecase/tests"
)

func TestIssueDraftSession_Execute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	pid, _ := photobook_id.FromUUID(uuid.New())

	cases := []struct {
		name        string
		description string
		repoErr     error
		expiresAt   time.Time
		wantErr     bool
	}{
		{
			name:        "正常_発行",
			description: "Given: 有効な expiresAt, When: Execute, Then: session_type=draft / version=0 / repo.Create が呼ばれる",
			expiresAt:   now.Add(7 * 24 * time.Hour),
		},
		{
			name:        "異常_repository_create_失敗",
			description: "Given: repo.CreateErr 設定, When: Execute, Then: エラー伝播",
			expiresAt:   now.Add(24 * time.Hour),
			repoErr:     errors.New("db down"),
			wantErr:     true,
		},
		{
			name:        "異常_expiresAt_=_now",
			description: "Given: expiresAt=now, When: Execute, Then: domain.ErrExpiresBeforeCreated 経由でエラー",
			expiresAt:   now,
			wantErr:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := tests.NewFakeRepository()
			repo.CreateErr = tc.repoErr
			uc := usecase.NewIssueDraftSession(repo)

			out, err := uc.Execute(context.Background(), usecase.IssueDraftSessionInput{
				PhotobookID: pid,
				Now:         now,
				ExpiresAt:   tc.expiresAt,
			})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if !out.Session.SessionType().IsDraft() {
				t.Errorf("session_type must be draft")
			}
			if out.Session.TokenVersionAtIssue().Int() != 0 {
				t.Errorf("token_version_at_issue must be 0")
			}
			if !out.Session.PhotobookID().Equal(pid) {
				t.Errorf("photobook_id mismatch")
			}
			if !out.Session.ExpiresAt().Equal(tc.expiresAt) {
				t.Errorf("expires_at mismatch: got=%s want=%s", out.Session.ExpiresAt(), tc.expiresAt)
			}
			if repo.CreateCalls != 1 {
				t.Errorf("Create calls = %d, want 1", repo.CreateCalls)
			}
			if out.RawToken.IsZero() {
				t.Errorf("raw token must not be zero")
			}
		})
	}
}
