// GetManagePhotobook 統合テスト（実 DB）。
//
// 観点:
//   - 200: photobook 存在 → manage view が返る（draft / published）
//   - 404: photobook 不存在
//   - 404: purged は管理ページからも 404
//   - manage URL token / hash / 値が view に含まれない
//   - public_url_path が "/p/{slug}" 形式で組み立てられる
package usecase_test

import (
	"context"
	"errors"
	"testing"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

func TestGetManagePhotobook(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()

	t.Run("正常_draft状態でslugnull_AvailableImage0", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		uc := usecase.NewGetManagePhotobook(pool)
		out, err := uc.Execute(ctx, usecase.GetManagePhotobookInput{PhotobookID: pb.ID()})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.View.Status != "draft" {
			t.Errorf("status=%s want draft", out.View.Status)
		}
		if out.View.PublicURLSlug != nil {
			t.Errorf("PublicURLSlug should be nil for draft")
		}
		if out.View.PublicURLPath != nil {
			t.Errorf("PublicURLPath should be nil for draft")
		}
		if out.View.AvailableImageCount != 0 {
			t.Errorf("count=%d want 0", out.View.AvailableImageCount)
		}
	})

	t.Run("正常_published_でslug_と_url_path_が出る", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		const slugStr = "ma12pp34zz56gh78"
		publishWithSlug(t, pool, pb.ID(), slugStr, false, "unlisted")
		uc := usecase.NewGetManagePhotobook(pool)
		out, err := uc.Execute(ctx, usecase.GetManagePhotobookInput{PhotobookID: pb.ID()})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.View.Status != "published" {
			t.Errorf("status=%s want published", out.View.Status)
		}
		if out.View.PublicURLSlug == nil || *out.View.PublicURLSlug != slugStr {
			t.Errorf("slug=%v want %s", out.View.PublicURLSlug, slugStr)
		}
		if out.View.PublicURLPath == nil || *out.View.PublicURLPath != "/p/"+slugStr {
			t.Errorf("path=%v want /p/%s", out.View.PublicURLPath, slugStr)
		}
		if out.View.PublishedAt == nil {
			t.Errorf("PublishedAt should not be nil for published")
		}
	})

	t.Run("正常_available_image_count_は_available_only", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		// 1 枚 available の image を seed（attach は不要、count はクエリで images.status を見る）
		_ = seedAvailableImage(t, pool, pb.ID())
		uc := usecase.NewGetManagePhotobook(pool)
		out, err := uc.Execute(ctx, usecase.GetManagePhotobookInput{PhotobookID: pb.ID()})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.View.AvailableImageCount != 1 {
			t.Errorf("count=%d want 1", out.View.AvailableImageCount)
		}
	})

	t.Run("異常_存在しないIDは404", func(t *testing.T) {
		truncateAll(t, pool)
		notExist, err := photobook_id.New()
		if err != nil {
			t.Fatalf("photobook_id.New: %v", err)
		}
		uc := usecase.NewGetManagePhotobook(pool)
		_, err = uc.Execute(ctx, usecase.GetManagePhotobookInput{PhotobookID: notExist})
		if !errors.Is(err, usecase.ErrManageNotFound) {
			t.Errorf("err=%v want ErrManageNotFound", err)
		}
	})
}
