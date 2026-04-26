package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/opening_style"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_layout"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_type"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
	"vrcpb/backend/internal/photobook/internal/usecase"
	"vrcpb/backend/internal/photobook/internal/usecase/tests"
)

func setupDraftInRepo(t *testing.T) (*tests.FakePhotobookRepository, draft_edit_token.DraftEditToken) {
	t.Helper()
	repo := tests.NewFakePhotobookRepository()
	now := time.Now().UTC()
	out, err := usecase.NewCreateDraftPhotobook(repo).Execute(context.Background(), usecase.CreateDraftPhotobookInput{
		Type:               photobook_type.Memory(),
		Title:              "Test",
		Layout:             photobook_layout.Simple(),
		OpeningStyle:       opening_style.Light(),
		Visibility:         visibility.Unlisted(),
		CreatorDisplayName: "Tester",
		RightsAgreed:       true,
		Now:                now,
		DraftTTL:           24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	return repo, out.RawDraftToken
}

func TestExchangeDraftTokenForSession_Execute(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		description string
		mutate      func(t *testing.T, repo *tests.FakePhotobookRepository, raw draft_edit_token.DraftEditToken) (draft_edit_token.DraftEditToken, *tests.FakePhotobookRepository)
		wantErr     error
		wantTouch   int
	}{
		{
			name:        "正常_交換成功",
			description: "Given: 有効な draft token, When: Execute, Then: RawSessionToken が返り TouchCalls=0",
			mutate: func(_ *testing.T, repo *tests.FakePhotobookRepository, raw draft_edit_token.DraftEditToken) (draft_edit_token.DraftEditToken, *tests.FakePhotobookRepository) {
				return raw, repo
			},
			wantTouch: 0,
		},
		{
			name:        "異常_zero_token",
			description: "Given: zero token, When: Execute, Then: ErrInvalidDraftToken",
			mutate: func(_ *testing.T, repo *tests.FakePhotobookRepository, _ draft_edit_token.DraftEditToken) (draft_edit_token.DraftEditToken, *tests.FakePhotobookRepository) {
				return draft_edit_token.DraftEditToken{}, repo
			},
			wantErr: usecase.ErrInvalidDraftToken,
		},
		{
			name:        "異常_存在しないtoken",
			description: "Given: 別の generate された token, When: Execute, Then: ErrInvalidDraftToken",
			mutate: func(t *testing.T, repo *tests.FakePhotobookRepository, _ draft_edit_token.DraftEditToken) (draft_edit_token.DraftEditToken, *tests.FakePhotobookRepository) {
				other, err := draft_edit_token.Generate()
				if err != nil {
					t.Fatalf("Generate: %v", err)
				}
				return other, repo
			},
			wantErr: usecase.ErrInvalidDraftToken,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, raw := setupDraftInRepo(t)
			tok, repoToUse := tc.mutate(t, repo, raw)
			issuer := tests.NewFakeDraftSessionIssuer()
			uc := usecase.NewExchangeDraftTokenForSession(repoToUse, issuer)
			out, err := uc.Execute(context.Background(), usecase.ExchangeDraftTokenForSessionInput{
				RawToken: tok,
				Now:      now,
			})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if out.RawSessionToken.IsZero() {
				t.Errorf("session token must not be zero")
			}
			if repoToUse.TouchCalls != tc.wantTouch {
				t.Errorf("TouchCalls = %d, want %d (touchDraft must NOT be called in exchange)", repoToUse.TouchCalls, tc.wantTouch)
			}
			if issuer.Calls != 1 {
				t.Errorf("issuer.Calls = %d want 1", issuer.Calls)
			}
		})
	}
}

func TestExchangeDraftTokenForSession_HashConsistency(t *testing.T) {
	t.Parallel()
	t.Run("正常_hash経由でphotobookを引ける", func(t *testing.T) {
		// Given: CreateDraft で発行された raw token, When: その raw を Of(hash) して Repo.FindByDraftEditTokenHash, Then: 同 photobook が引ける
		repo, raw := setupDraftInRepo(t)
		hash := draft_edit_token_hash.Of(raw)
		_, err := repo.FindByDraftEditTokenHash(context.Background(), hash)
		if err != nil {
			t.Fatalf("FindByDraftEditTokenHash: %v", err)
		}
	})
}
