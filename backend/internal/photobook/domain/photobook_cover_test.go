package domain_test

import (
	"errors"
	"testing"
	"time"

	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	domaintests "vrcpb/backend/internal/photobook/domain/tests"
)

func mustSlug(t *testing.T) slug.Slug {
	t.Helper()
	s, err := slug.Parse("test-slug-001-abcd")
	if err != nil {
		t.Fatalf("slug.Parse: %v", err)
	}
	return s
}

func newImageIDForCover(t *testing.T) image_id.ImageID {
	t.Helper()
	id, err := image_id.New()
	if err != nil {
		t.Fatalf("image_id.New: %v", err)
	}
	return id
}

func TestPhotobook_SetCoverImage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	imgID := newImageIDForCover(t)

	t.Run("正常_draft状態でCoverを設定", func(t *testing.T) {
		pb := domaintests.NewPhotobookBuilder().Build(t)
		out, err := pb.SetCoverImage(imgID, now)
		if err != nil {
			t.Fatalf("SetCoverImage: %v", err)
		}
		if out.CoverImageID() == nil || !out.CoverImageID().Equal(imgID) {
			t.Errorf("cover_image_id mismatch")
		}
		if out.Version() != pb.Version()+1 {
			t.Errorf("version not bumped: got %d want %d", out.Version(), pb.Version()+1)
		}
	})

	t.Run("異常_published状態は不可", func(t *testing.T) {
		// Given: draft → publish して status=published, When: SetCoverImage,
		// Then: ErrNotDraft
		drafted := domaintests.NewPhotobookBuilder().WithRightsAgreed(true).Build(t)
		published, err := drafted.Publish(mustSlug(t), newManageHash(t), now)
		if err != nil {
			t.Fatalf("Publish: %v", err)
		}
		_, err = published.SetCoverImage(imgID, now.Add(time.Second))
		if !errors.Is(err, domain.ErrNotDraft) {
			t.Fatalf("err = %v want ErrNotDraft", err)
		}
	})
}

func TestPhotobook_ClearCoverImage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	imgID := newImageIDForCover(t)

	t.Run("正常_設定済みCoverをクリア", func(t *testing.T) {
		pb := domaintests.NewPhotobookBuilder().Build(t)
		set, _ := pb.SetCoverImage(imgID, now)
		out, err := set.ClearCoverImage(now.Add(time.Second))
		if err != nil {
			t.Fatalf("ClearCoverImage: %v", err)
		}
		if out.CoverImageID() != nil {
			t.Errorf("cover_image_id should be nil")
		}
		if out.Version() != set.Version()+1 {
			t.Errorf("version not bumped")
		}
	})
}

func TestPhotobook_CanEdit(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	pb := domaintests.NewPhotobookBuilder().Build(t)
	if err := pb.CanEdit(); err != nil {
		t.Errorf("draft must be editable: %v", err)
	}
	published, _ := pb.Publish(mustSlug(t), newManageHash(t), now)
	if err := published.CanEdit(); !errors.Is(err, domain.ErrNotDraft) {
		t.Errorf("published must NOT be editable: %v", err)
	}
}
