package domain_test

import (
	"errors"
	"testing"
	"time"

	"vrcpb/backend/internal/photobook/domain"
	domaintests "vrcpb/backend/internal/photobook/domain/tests"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/opening_style"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_layout"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_type"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
)

func newID(t *testing.T) photobook_id.PhotobookID {
	t.Helper()
	id, err := photobook_id.New()
	if err != nil {
		t.Fatalf("photobook_id.New: %v", err)
	}
	return id
}

func newDraftHash(t *testing.T) draft_edit_token_hash.DraftEditTokenHash {
	t.Helper()
	tok, err := draft_edit_token.Generate()
	if err != nil {
		t.Fatalf("draft_edit_token.Generate: %v", err)
	}
	return draft_edit_token_hash.Of(tok)
}

func newManageHash(t *testing.T) manage_url_token_hash.ManageUrlTokenHash {
	t.Helper()
	tok, err := manage_url_token.Generate()
	if err != nil {
		t.Fatalf("manage_url_token.Generate: %v", err)
	}
	return manage_url_token_hash.Of(tok)
}

func TestNewDraftPhotobook(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		description string
		modify      func(p *domain.NewDraftPhotobookParams)
		wantErr     error
	}{
		{
			name:        "正常_既定draft",
			description: "Given: 必須項目を指定, When: NewDraftPhotobook, Then: status=draft / version=0 / draft_expires_at が now+ttl",
		},
		{
			name:        "正常_空title許容",
			description: "Given: title='' (任意項目), When: NewDraftPhotobook, Then: 成功（draft 作成時は空文字許容、publish 時は CanPublish で別途検証）",
			modify:      func(p *domain.NewDraftPhotobookParams) { p.Title = "" },
		},
		{
			name:        "異常_長すぎtitle",
			description: "Given: title 81 文字, When: NewDraftPhotobook, Then: ErrTitleTooLong",
			modify: func(p *domain.NewDraftPhotobookParams) {
				p.Title = string(make([]byte, 81))
				for i := range p.Title {
					_ = i
				}
				// 81 文字の ASCII を作る
				b := make([]byte, 81)
				for i := range b {
					b[i] = 'a'
				}
				p.Title = string(b)
			},
			wantErr: domain.ErrTitleTooLong,
		},
		{
			name:        "正常_空creator_name許容",
			description: "Given: creator_display_name='' (任意項目), When: NewDraftPhotobook, Then: 成功（draft 作成時 / publish 時とも空文字許容、2026-05-03 STOP α P0-γ-A hotfix）",
			modify:      func(p *domain.NewDraftPhotobookParams) { p.CreatorDisplayName = "" },
		},
		{
			name:        "異常_負のTTL",
			description: "Given: DraftTTL=-1h, When: NewDraftPhotobook, Then: ErrDraftExpiresInPast",
			modify:      func(p *domain.NewDraftPhotobookParams) { p.DraftTTL = -1 * time.Hour },
			wantErr:     domain.ErrDraftExpiresInPast,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := domain.NewDraftPhotobookParams{
				ID:                 newID(t),
				Type:               photobook_type.Memory(),
				Title:              "Test Photobook",
				Layout:             photobook_layout.Simple(),
				OpeningStyle:       opening_style.Light(),
				Visibility:         visibility.Unlisted(),
				CreatorDisplayName: "Tester",
				RightsAgreed:       true,
				DraftEditTokenHash: newDraftHash(t),
				Now:                now,
				DraftTTL:           7 * 24 * time.Hour,
			}
			if tt.modify != nil {
				tt.modify(&p)
			}
			pb, err := domain.NewDraftPhotobook(p)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if !pb.IsDraft() {
				t.Errorf("must be draft")
			}
			if pb.Version() != 0 {
				t.Errorf("version = %d want 0", pb.Version())
			}
			if pb.DraftEditTokenHash() == nil {
				t.Errorf("draft_edit_token_hash must be set")
			}
			if got := pb.DraftExpiresAt(); got == nil || !got.Equal(now.Add(7*24*time.Hour)) {
				t.Errorf("draft_expires_at = %v want now+7d", got)
			}
			if pb.PublicUrlSlug() != nil || pb.ManageUrlTokenHash() != nil || pb.PublishedAt() != nil {
				t.Errorf("draft must have nil public_url_slug / manage_url_token_hash / published_at")
			}
		})
	}
}

func TestPhotobook_CanPublish(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		description string
		build       func(t *testing.T) domain.Photobook
		wantErr     error
	}{
		{
			name:        "正常_publish可",
			description: "Given: draft + rights_agreed=true + creator_name 非空, When: CanPublish, Then: nil",
			build: func(t *testing.T) domain.Photobook {
				return domaintests.NewPhotobookBuilder().Build(t)
			},
		},
		{
			name:        "異常_rights_agreed_false",
			description: "Given: rights_agreed=false, When: CanPublish, Then: ErrRightsNotAgreed",
			build: func(t *testing.T) domain.Photobook {
				return domaintests.NewPhotobookBuilder().WithRightsAgreed(false).Build(t)
			},
			wantErr: domain.ErrRightsNotAgreed,
		},
		{
			name:        "正常_creator空でもpublish可_hotfix",
			description: "Given: draft + rights_agreed=true + creator_display_name=''（/create で空欄許容され、UpdatePhotobookSettings に creator 列も無い）, When: CanPublish, Then: nil（2026-05-03 STOP α P0-γ-A hotfix の lock-in）",
			build: func(t *testing.T) domain.Photobook {
				return domaintests.NewPhotobookBuilder().WithCreatorName("").Build(t)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := tt.build(t)
			err := pb.CanPublish()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
		})
	}
}

func TestPhotobook_Publish(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	t.Run("正常_publish遷移", func(t *testing.T) {
		// Given: draft + 公開要件OK, When: Publish(slug, manageHash, now+1h),
		// Then: status=published / version+1 / draft_* が nil / public_url_slug / manage_* セット
		pb := domaintests.NewPhotobookBuilder().Build(t)
		s, err := slug.Parse("test-slug-001-abcd")
		if err != nil {
			t.Fatalf("slug: %v", err)
		}
		mh := newManageHash(t)
		published, err := pb.Publish(s, mh, now.Add(1*time.Hour))
		if err != nil {
			t.Fatalf("Publish: %v", err)
		}
		if !published.IsPublished() {
			t.Errorf("status must be published")
		}
		if published.Version() != pb.Version()+1 {
			t.Errorf("version not incremented")
		}
		if published.DraftEditTokenHash() != nil || published.DraftExpiresAt() != nil {
			t.Errorf("draft_* must be nil after publish")
		}
		if published.PublicUrlSlug() == nil || !published.PublicUrlSlug().Equal(s) {
			t.Errorf("public_url_slug mismatch")
		}
		if published.ManageUrlTokenHash() == nil || !published.ManageUrlTokenHash().Equal(mh) {
			t.Errorf("manage_url_token_hash mismatch")
		}
		if published.ManageUrlTokenVersion().Int() != 0 {
			t.Errorf("manage_url_token_version must be 0 at publish")
		}
		if published.PublishedAt() == nil {
			t.Errorf("published_at must be set")
		}
	})

	t.Run("異常_rights_未同意で_publish", func(t *testing.T) {
		// Given: rights_agreed=false, When: Publish, Then: ErrRightsNotAgreed
		pb := domaintests.NewPhotobookBuilder().WithRightsAgreed(false).Build(t)
		s, _ := slug.Parse("test-slug-001-abcd")
		mh := newManageHash(t)
		_, err := pb.Publish(s, mh, now)
		if !errors.Is(err, domain.ErrRightsNotAgreed) {
			t.Fatalf("err = %v want ErrRightsNotAgreed", err)
		}
	})
}

func TestPhotobook_ReissueManageUrl(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	t.Run("正常_published_でreissue", func(t *testing.T) {
		// Given: published Photobook, When: ReissueManageUrl,
		// Then: manage_url_token_hash 更新 / manage_url_token_version+1 / version+1 / oldVersion 戻る
		pb := domaintests.NewPhotobookBuilder().Build(t)
		s, _ := slug.Parse("test-slug-001-abcd")
		published, err := pb.Publish(s, newManageHash(t), now)
		if err != nil {
			t.Fatalf("Publish: %v", err)
		}
		newHash := newManageHash(t)
		reissued, oldVersion, err := published.ReissueManageUrl(newHash, now.Add(1*time.Hour))
		if err != nil {
			t.Fatalf("ReissueManageUrl: %v", err)
		}
		if oldVersion.Int() != 0 {
			t.Errorf("oldVersion = %d want 0", oldVersion.Int())
		}
		if reissued.ManageUrlTokenVersion().Int() != 1 {
			t.Errorf("new version = %d want 1", reissued.ManageUrlTokenVersion().Int())
		}
		if reissued.Version() != published.Version()+1 {
			t.Errorf("photobook.version not incremented")
		}
		if !reissued.ManageUrlTokenHash().Equal(newHash) {
			t.Errorf("manage_url_token_hash not updated")
		}
	})

	t.Run("異常_draft_でreissue", func(t *testing.T) {
		// Given: draft Photobook, When: ReissueManageUrl, Then: ErrNotPublishedOrDeleted
		pb := domaintests.NewPhotobookBuilder().Build(t)
		_, _, err := pb.ReissueManageUrl(newManageHash(t), now)
		if !errors.Is(err, domain.ErrNotPublishedOrDeleted) {
			t.Fatalf("err = %v want ErrNotPublishedOrDeleted", err)
		}
	})
}

func TestPhotobook_TouchDraft(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		description string
		build       func(t *testing.T) domain.Photobook
		wantErr     error
	}{
		{
			name:        "正常_draft_延長",
			description: "Given: draft, When: TouchDraft(now+...,7d), Then: draft_expires_at = touchNow+7d / version+1",
			build: func(t *testing.T) domain.Photobook {
				return domaintests.NewPhotobookBuilder().WithNow(now).Build(t)
			},
		},
		{
			name:        "異常_published_でtouch",
			description: "Given: published, When: TouchDraft, Then: ErrNotDraft",
			build: func(t *testing.T) domain.Photobook {
				pb := domaintests.NewPhotobookBuilder().Build(t)
				s, _ := slug.Parse("test-slug-001-abcd")
				p, _ := pb.Publish(s, newManageHash(t), now)
				return p
			},
			wantErr: domain.ErrNotDraft,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := tt.build(t)
			touchTime := now.Add(2 * time.Hour)
			out, err := pb.TouchDraft(touchTime, 7*24*time.Hour)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if got := out.DraftExpiresAt(); got == nil || !got.Equal(touchTime.Add(7*24*time.Hour)) {
				t.Errorf("draft_expires_at not extended: %v", got)
			}
			if out.Version() != pb.Version()+1 {
				t.Errorf("version not incremented")
			}
		})
	}
}
