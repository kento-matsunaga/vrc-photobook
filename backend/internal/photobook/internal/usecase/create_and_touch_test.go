package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/opening_style"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_layout"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_type"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
	"vrcpb/backend/internal/photobook/internal/usecase"
	"vrcpb/backend/internal/photobook/internal/usecase/tests"
)

func defaultCreateInput(now time.Time) usecase.CreateDraftPhotobookInput {
	return usecase.CreateDraftPhotobookInput{
		Type:               photobook_type.Memory(),
		Title:              "Test Photobook",
		Layout:             photobook_layout.Simple(),
		OpeningStyle:       opening_style.Light(),
		Visibility:         visibility.Unlisted(),
		CreatorDisplayName: "Tester",
		RightsAgreed:       true,
		Now:                now,
		DraftTTL:           7 * 24 * time.Hour,
	}
}

func TestCreateDraftPhotobook_Execute(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		description string
		modify      func(in *usecase.CreateDraftPhotobookInput)
		wantErr     bool
		wantStatus  string
	}{
		{
			name:        "正常_既定draft",
			description: "Given: 必須項目 + rights_agreed=true, When: Execute, Then: status=draft / RawDraftToken!=zero / CreateCalls=1",
			wantStatus:  "draft",
		},
		{
			name:        "異常_空title",
			description: "Given: title='', When: Execute, Then: ErrEmptyTitle",
			modify:      func(in *usecase.CreateDraftPhotobookInput) { in.Title = "" },
			wantErr:     true,
		},
		{
			name:        "異常_空creator",
			description: "Given: creator_display_name='', When: Execute, Then: ErrEmptyCreatorName",
			modify:      func(in *usecase.CreateDraftPhotobookInput) { in.CreatorDisplayName = "" },
			wantErr:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := tests.NewFakePhotobookRepository()
			uc := usecase.NewCreateDraftPhotobook(repo)
			in := defaultCreateInput(now)
			if tc.modify != nil {
				tc.modify(&in)
			}
			out, err := uc.Execute(context.Background(), in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if !out.Photobook.IsDraft() {
				t.Errorf("status must be draft")
			}
			if out.RawDraftToken.IsZero() {
				t.Errorf("raw draft token must not be zero")
			}
			if repo.CreateCalls != 1 {
				t.Errorf("CreateCalls = %d want 1", repo.CreateCalls)
			}
		})
	}
}

func TestTouchDraft_Execute(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		description string
		setup       func(t *testing.T) (*tests.FakePhotobookRepository, photobook_id.PhotobookID, int)
		wantErr     error
		wantTouch   int
	}{
		{
			name:        "正常_touch成功",
			description: "Given: 作成直後 draft, When: TouchDraft(expectedVersion=0), Then: TouchCalls=1",
			setup: func(t *testing.T) (*tests.FakePhotobookRepository, photobook_id.PhotobookID, int) {
				repo := tests.NewFakePhotobookRepository()
				out, err := usecase.NewCreateDraftPhotobook(repo).Execute(context.Background(), defaultCreateInput(now))
				if err != nil {
					t.Fatalf("Create: %v", err)
				}
				return repo, out.Photobook.ID(), out.Photobook.Version()
			},
			wantTouch: 1,
		},
		{
			name:        "異常_version不一致",
			description: "Given: 作成直後 draft, When: TouchDraft(expectedVersion=99), Then: ErrDraftConflict",
			setup: func(t *testing.T) (*tests.FakePhotobookRepository, photobook_id.PhotobookID, int) {
				repo := tests.NewFakePhotobookRepository()
				out, err := usecase.NewCreateDraftPhotobook(repo).Execute(context.Background(), defaultCreateInput(now))
				if err != nil {
					t.Fatalf("Create: %v", err)
				}
				return repo, out.Photobook.ID(), 99
			},
			wantErr:   usecase.ErrDraftConflict,
			wantTouch: 1,
		},
		{
			name:        "異常_負のTTL",
			description: "Given: TTL=-1h, When: TouchDraft, Then: domain.ErrDraftExpiresInPast",
			setup: func(t *testing.T) (*tests.FakePhotobookRepository, photobook_id.PhotobookID, int) {
				repo := tests.NewFakePhotobookRepository()
				out, err := usecase.NewCreateDraftPhotobook(repo).Execute(context.Background(), defaultCreateInput(now))
				if err != nil {
					t.Fatalf("Create: %v", err)
				}
				return repo, out.Photobook.ID(), out.Photobook.Version()
			},
			wantErr:   domain.ErrDraftExpiresInPast,
			wantTouch: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, id, ver := tc.setup(t)
			ttl := 7 * 24 * time.Hour
			if tc.name == "異常_負のTTL" {
				ttl = -1 * time.Hour
			}
			err := usecase.NewTouchDraft(repo).Execute(context.Background(), usecase.TouchDraftInput{
				PhotobookID:     id,
				ExpectedVersion: ver,
				Now:             now,
				DraftTTL:        ttl,
			})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v want %v", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if repo.TouchCalls != tc.wantTouch {
				t.Errorf("TouchCalls = %d want %d", repo.TouchCalls, tc.wantTouch)
			}
		})
	}
}
