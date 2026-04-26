package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
	"vrcpb/backend/internal/auth/session/internal/usecase"
	"vrcpb/backend/internal/auth/session/internal/usecase/tests"
)

func TestIssueManageSession_Execute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	pid, _ := photobook_id.FromUUID(uuid.New())

	cases := []struct {
		name        string
		description string
		version     int
	}{
		{
			name:        "正常_version1",
			description: "Given: version=1, When: Execute, Then: session_type=manage / version=1 が保持",
			version:     1,
		},
		{
			name:        "正常_version5",
			description: "Given: version=5, When: Execute, Then: version=5 が保持",
			version:     5,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := tests.NewFakeRepository()
			uc := usecase.NewIssueManageSession(repo)
			tv, _ := token_version_at_issue.New(tc.version)

			out, err := uc.Execute(context.Background(), usecase.IssueManageSessionInput{
				PhotobookID:         pid,
				TokenVersionAtIssue: tv,
				Now:                 now,
				ExpiresAt:           now.Add(7 * 24 * time.Hour),
			})
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if !out.Session.SessionType().IsManage() {
				t.Errorf("session_type must be manage")
			}
			if got := out.Session.TokenVersionAtIssue().Int(); got != tc.version {
				t.Errorf("version got=%d want=%d", got, tc.version)
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
