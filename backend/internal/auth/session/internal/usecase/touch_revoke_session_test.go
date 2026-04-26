package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
	"vrcpb/backend/internal/auth/session/infrastructure/repository/rdb"
	"vrcpb/backend/internal/auth/session/internal/usecase"
	"vrcpb/backend/internal/auth/session/internal/usecase/tests"
)

func TestTouchSession_Execute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	pid, _ := photobook_id.FromUUID(uuid.New())

	cases := []struct {
		name        string
		description string
		setup       func(repo *tests.FakeRepository) session_id.SessionID
		wantErr     error
	}{
		{
			name:        "正常_有効session",
			description: "Given: 発行直後, When: Touch, Then: エラーなし、TouchCalls=1",
			setup: func(repo *tests.FakeRepository) session_id.SessionID {
				repo.Now = func() time.Time { return now }
				out, err := usecase.NewIssueDraftSession(repo).Execute(context.Background(), usecase.IssueDraftSessionInput{
					PhotobookID: pid,
					Now:         now,
					ExpiresAt:   now.Add(24 * time.Hour),
				})
				if err != nil {
					t.Fatalf("issue: %v", err)
				}
				return out.Session.ID()
			},
		},
		{
			name:        "異常_存在しないID",
			description: "Given: 未保存の id, When: Touch, Then: rdb.ErrNotFound",
			setup: func(repo *tests.FakeRepository) session_id.SessionID {
				id, _ := session_id.New()
				return id
			},
			wantErr: rdb.ErrNotFound,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := tests.NewFakeRepository()
			id := tc.setup(repo)
			err := usecase.NewTouchSession(repo).Execute(context.Background(), id)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if repo.TouchCalls != 1 {
				t.Errorf("TouchCalls = %d, want 1", repo.TouchCalls)
			}
		})
	}
}

func TestRevokeSession_Execute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	pid, _ := photobook_id.FromUUID(uuid.New())

	t.Run("正常_発行→revoke", func(t *testing.T) {
		// Given: draft session を発行, When: RevokeSession, Then: エラーなし、RevokeCalls=1
		repo := tests.NewFakeRepository()
		repo.Now = func() time.Time { return now }
		out, err := usecase.NewIssueDraftSession(repo).Execute(context.Background(), usecase.IssueDraftSessionInput{
			PhotobookID: pid,
			Now:         now,
			ExpiresAt:   now.Add(24 * time.Hour),
		})
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		if err := usecase.NewRevokeSession(repo).Execute(context.Background(), out.Session.ID()); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		if repo.RevokeCalls != 1 {
			t.Errorf("RevokeCalls = %d, want 1", repo.RevokeCalls)
		}
	})

	t.Run("異常_存在しないID", func(t *testing.T) {
		// Given: 未保存の id, When: RevokeSession, Then: rdb.ErrNotFound
		repo := tests.NewFakeRepository()
		id, _ := session_id.New()
		err := usecase.NewRevokeSession(repo).Execute(context.Background(), id)
		if !errors.Is(err, rdb.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestRevokeAllDrafts_Execute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	pid, _ := photobook_id.FromUUID(uuid.New())

	t.Run("正常_draftだけrevokeされる", func(t *testing.T) {
		// Given: draft 2 件 + manage 1 件, When: RevokeAllDrafts, Then: 影響行数=2
		repo := tests.NewFakeRepository()
		repo.Now = func() time.Time { return now }
		issuerD := usecase.NewIssueDraftSession(repo)
		issuerM := usecase.NewIssueManageSession(repo)
		for i := 0; i < 2; i++ {
			if _, err := issuerD.Execute(context.Background(), usecase.IssueDraftSessionInput{
				PhotobookID: pid, Now: now, ExpiresAt: now.Add(24 * time.Hour),
			}); err != nil {
				t.Fatal(err)
			}
		}
		tv, _ := token_version_at_issue.New(1)
		if _, err := issuerM.Execute(context.Background(), usecase.IssueManageSessionInput{
			PhotobookID: pid, TokenVersionAtIssue: tv, Now: now, ExpiresAt: now.Add(24 * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
		n, err := usecase.NewRevokeAllDrafts(repo).Execute(context.Background(), pid)
		if err != nil {
			t.Fatal(err)
		}
		if n != 2 {
			t.Errorf("affected = %d, want 2", n)
		}
	})
}

func TestRevokeAllManageByTokenVersion_Execute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	pid, _ := photobook_id.FromUUID(uuid.New())

	t.Run("正常_oldVersion以下を一括revoke", func(t *testing.T) {
		// Given: manage v1, v2, v3, When: RevokeAllManageByTokenVersion(pid, 2),
		// Then: 影響行数=2 (v1, v2)、v3 は残る
		repo := tests.NewFakeRepository()
		repo.Now = func() time.Time { return now }
		for _, v := range []int{1, 2, 3} {
			tv, _ := token_version_at_issue.New(v)
			if _, err := usecase.NewIssueManageSession(repo).Execute(context.Background(), usecase.IssueManageSessionInput{
				PhotobookID: pid, TokenVersionAtIssue: tv, Now: now, ExpiresAt: now.Add(24 * time.Hour),
			}); err != nil {
				t.Fatal(err)
			}
		}
		n, err := usecase.NewRevokeAllManageByTokenVersion(repo).Execute(context.Background(), pid, 2)
		if err != nil {
			t.Fatal(err)
		}
		if n != 2 {
			t.Errorf("affected = %d, want 2", n)
		}
	})
}
