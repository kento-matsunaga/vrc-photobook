// PR27 編集 UI 本格化用 UseCase の実 DB 統合テスト。
//
// 観点:
//   - UpdatePhotoCaption: 正常 / OCC conflict / 未知 photo
//   - BulkReorderPhotosOnPage: 正常 swap / OCC conflict / UNIQUE 衝突回避
//   - UpdatePhotobookSettings: 正常 / status≠draft で 409
//   - GetEditView: 正常 / status≠draft で ErrEditNotAllowed / 不存在で ErrEditPhotobookNotFound
package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	imagedomain "vrcpb/backend/internal/image/domain"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/mime_type"
	imagestoragekey "vrcpb/backend/internal/image/domain/vo/storage_key"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	uploadtests "vrcpb/backend/internal/imageupload/tests"
	"vrcpb/backend/internal/photobook/domain/vo/caption"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

func TestUpdatePhotoCaption(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("正常_caption設定", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		// page + photo を seed
		addPage := usecase.NewAddPage(pool)
		pageOut, _ := addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pb.ID(), ExpectedVersion: pb.Version(), Now: now,
		})
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbAfterPage, _ := repo.FindByID(ctx, pb.ID())
		img := seedAvailableImage(t, pool, pb.ID())
		addPhoto := usecase.NewAddPhoto(pool)
		addOut, err := addPhoto.Execute(ctx, usecase.AddPhotoInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			ImageID: img.ID(), ExpectedVersion: pbAfterPage.Version(), Now: now,
		})
		if err != nil {
			t.Fatalf("AddPhoto: %v", err)
		}
		pbAfterPhoto, _ := repo.FindByID(ctx, pb.ID())
		c := caption.MustNew("hello")
		uc := usecase.NewUpdatePhotoCaption(pool)
		if err := uc.Execute(ctx, usecase.UpdatePhotoCaptionInput{
			PhotobookID: pb.ID(), PhotoID: addOut.Photo.ID(),
			Caption: &c, ExpectedVersion: pbAfterPhoto.Version(), Now: now,
		}); err != nil {
			t.Fatalf("UpdatePhotoCaption: %v", err)
		}
	})

	t.Run("異常_OCC_conflict", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		addPage := usecase.NewAddPage(pool)
		pageOut, _ := addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pb.ID(), ExpectedVersion: pb.Version(), Now: now,
		})
		repo := photobookrdb.NewPhotobookRepository(pool)
		pbAfterPage, _ := repo.FindByID(ctx, pb.ID())
		img := seedAvailableImage(t, pool, pb.ID())
		addPhoto := usecase.NewAddPhoto(pool)
		addOut, _ := addPhoto.Execute(ctx, usecase.AddPhotoInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			ImageID: img.ID(), ExpectedVersion: pbAfterPage.Version(), Now: now,
		})
		// 古い version で UPDATE
		c := caption.MustNew("conflict")
		uc := usecase.NewUpdatePhotoCaption(pool)
		err := uc.Execute(ctx, usecase.UpdatePhotoCaptionInput{
			PhotobookID: pb.ID(), PhotoID: addOut.Photo.ID(),
			Caption: &c, ExpectedVersion: pb.Version(), // 古い
			Now: now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Errorf("err=%v want OptimisticLockConflict", err)
		}
	})
}

func TestBulkReorderPhotosOnPage(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("正常_2_photoのswap", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		addPage := usecase.NewAddPage(pool)
		pageOut, _ := addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pb.ID(), ExpectedVersion: pb.Version(), Now: now,
		})
		repo := photobookrdb.NewPhotobookRepository(pool)
		pb1, _ := repo.FindByID(ctx, pb.ID())
		img1 := seedAvailableImage(t, pool, pb.ID())
		img2 := seedAvailableImage(t, pool, pb.ID())
		addPhoto := usecase.NewAddPhoto(pool)
		out1, _ := addPhoto.Execute(ctx, usecase.AddPhotoInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			ImageID: img1.ID(), ExpectedVersion: pb1.Version(), Now: now,
		})
		pb2, _ := repo.FindByID(ctx, pb.ID())
		out2, _ := addPhoto.Execute(ctx, usecase.AddPhotoInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			ImageID: img2.ID(), ExpectedVersion: pb2.Version(), Now: now,
		})
		pb3, _ := repo.FindByID(ctx, pb.ID())

		// 1 番目を 1、2 番目を 0 に swap（UNIQUE 衝突しがち）
		newOrder0, _ := display_order.New(1)
		newOrder1, _ := display_order.New(0)
		uc := usecase.NewBulkReorderPhotosOnPage(pool)
		if err := uc.Execute(ctx, usecase.BulkReorderPhotosOnPageInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			Assignments: []usecase.PhotoOrderItem{
				{PhotoID: out1.Photo.ID(), NewOrder: newOrder0},
				{PhotoID: out2.Photo.ID(), NewOrder: newOrder1},
			},
			ExpectedVersion: pb3.Version(),
			Now:             now,
		}); err != nil {
			t.Fatalf("BulkReorder: %v", err)
		}
		// 並び順が反転したか確認
		photos, _ := repo.ListPhotosByPageID(ctx, pageOut.Page.ID())
		if len(photos) != 2 {
			t.Fatalf("photo count=%d", len(photos))
		}
		// display_order ASC で並ぶので、index 0 が元 out2、index 1 が元 out1
		if photos[0].ID().UUID() != out2.Photo.ID().UUID() {
			t.Errorf("photos[0] expected = out2")
		}
		if photos[1].ID().UUID() != out1.Photo.ID().UUID() {
			t.Errorf("photos[1] expected = out1")
		}
	})

	t.Run("異常_OCC_conflict", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		addPage := usecase.NewAddPage(pool)
		pageOut, _ := addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pb.ID(), ExpectedVersion: pb.Version(), Now: now,
		})
		uc := usecase.NewBulkReorderPhotosOnPage(pool)
		err := uc.Execute(ctx, usecase.BulkReorderPhotosOnPageInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			Assignments:     []usecase.PhotoOrderItem{},
			ExpectedVersion: pb.Version(), // 古い（AddPage で +1 済）
			Now:             now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Errorf("err=%v want OptimisticLockConflict", err)
		}
	})
}

func TestUpdatePhotobookSettings(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("正常_settings更新", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		desc := "更新後の説明"
		uc := usecase.NewUpdatePhotobookSettings(pool)
		if err := uc.Execute(ctx, usecase.UpdatePhotobookSettingsInput{
			PhotobookID:     pb.ID(),
			Type:            "memory",
			Title:           "Updated Title",
			Description:     &desc,
			Layout:          "simple",
			OpeningStyle:    "light",
			Visibility:      "unlisted",
			ExpectedVersion: pb.Version(),
			Now:             now,
		}); err != nil {
			t.Fatalf("UpdateSettings: %v", err)
		}
		repo := photobookrdb.NewPhotobookRepository(pool)
		got, _ := repo.FindByID(ctx, pb.ID())
		if got.Title() != "Updated Title" {
			t.Errorf("title=%s", got.Title())
		}
		if got.Description() == nil || *got.Description() != desc {
			t.Errorf("description=%v", got.Description())
		}
		if got.Version() != pb.Version()+1 {
			t.Errorf("version not bumped: %d", got.Version())
		}
	})

	t.Run("異常_OCC_conflict_oldVersion", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		uc := usecase.NewUpdatePhotobookSettings(pool)
		// 1 度成功
		_ = uc.Execute(ctx, usecase.UpdatePhotobookSettingsInput{
			PhotobookID: pb.ID(), Type: "memory", Title: "T1",
			Layout: "simple", OpeningStyle: "light", Visibility: "unlisted",
			ExpectedVersion: pb.Version(), Now: now,
		})
		// 同じ古い version で 2 度目 → 409
		err := uc.Execute(ctx, usecase.UpdatePhotobookSettingsInput{
			PhotobookID: pb.ID(), Type: "memory", Title: "T2",
			Layout: "simple", OpeningStyle: "light", Visibility: "unlisted",
			ExpectedVersion: pb.Version(), Now: now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Errorf("err=%v want OptimisticLockConflict", err)
		}
	})

	t.Run("異常_published_になった_photobook_は更新不可", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		// 直接 UPDATE で published 化
		_, err := pool.Exec(ctx, `
			UPDATE photobooks SET status='published', published_at=$2, updated_at=$2,
			       public_url_slug='zz12pp34zz56gh78', manage_url_token_hash=decode(repeat('00',32),'hex'),
			       draft_edit_token_hash=NULL, draft_expires_at=NULL,
			       version=version+1
			 WHERE id=$1
		`,
			pgtype.UUID{Bytes: pb.ID().UUID(), Valid: true},
			pgtype.Timestamptz{Time: now, Valid: true},
		)
		if err != nil {
			t.Fatalf("publish UPDATE: %v", err)
		}
		uc := usecase.NewUpdatePhotobookSettings(pool)
		err = uc.Execute(ctx, usecase.UpdatePhotobookSettingsInput{
			PhotobookID: pb.ID(), Type: "memory", Title: "post-publish edit",
			Layout: "simple", OpeningStyle: "light", Visibility: "unlisted",
			ExpectedVersion: pb.Version() + 1, Now: now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Errorf("published 状態は OCC で 409 集約されるべき: err=%v", err)
		}
	})
}

func TestGetEditView(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("正常_draft_photo_あり", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		addPage := usecase.NewAddPage(pool)
		pageOut, _ := addPage.Execute(ctx, usecase.AddPageInput{
			PhotobookID: pb.ID(), ExpectedVersion: pb.Version(), Now: now,
		})
		repo := photobookrdb.NewPhotobookRepository(pool)
		pb1, _ := repo.FindByID(ctx, pb.ID())
		img := seedAvailableImage(t, pool, pb.ID())
		// variant attach
		dispKey, _ := imagestoragekey.GenerateForVariant(pb.ID(), img.ID(), variant_kind.Display())
		thumbKey, _ := imagestoragekey.GenerateForVariant(pb.ID(), img.ID(), variant_kind.Thumbnail())
		dDims, _ := image_dimensions.New(1600, 1200)
		tDims, _ := image_dimensions.New(480, 360)
		dBs, _ := byte_size.New(150_000)
		tBs, _ := byte_size.New(20_000)
		imgRepo := imagerdb.NewImageRepository(pool)
		dispVar, _ := imagedomain.NewImageVariant(imagedomain.NewImageVariantParams{
			ImageID: img.ID(), Kind: variant_kind.Display(), StorageKey: dispKey,
			Dimensions: dDims, ByteSize: dBs, MimeType: mime_type.Jpeg(), CreatedAt: now,
		})
		thumbVar, _ := imagedomain.NewImageVariant(imagedomain.NewImageVariantParams{
			ImageID: img.ID(), Kind: variant_kind.Thumbnail(), StorageKey: thumbKey,
			Dimensions: tDims, ByteSize: tBs, MimeType: mime_type.Jpeg(), CreatedAt: now,
		})
		_ = imgRepo.AttachVariant(ctx, dispVar)
		_ = imgRepo.AttachVariant(ctx, thumbVar)

		addPhoto := usecase.NewAddPhoto(pool)
		_, _ = addPhoto.Execute(ctx, usecase.AddPhotoInput{
			PhotobookID: pb.ID(), PageID: pageOut.Page.ID(),
			ImageID: img.ID(), ExpectedVersion: pb1.Version(), Now: now,
		})

		fakeR2 := &uploadtests.FakeR2Client{
			PresignGetObjectFn: func(_ context.Context, in r2.PresignGetInput) (r2.PresignGetOutput, error) {
				return r2.PresignGetOutput{URL: "https://fake.r2.test/get/" + in.StorageKey, ExpiresAt: time.Now().Add(in.ExpiresIn)}, nil
			},
		}
		uc := usecase.NewGetEditView(pool, fakeR2)
		out, err := uc.Execute(ctx, usecase.GetEditViewInput{PhotobookID: pb.ID()})
		if err != nil {
			t.Fatalf("GetEditView: %v", err)
		}
		if out.View.Status != "draft" {
			t.Errorf("status=%s", out.View.Status)
		}
		if len(out.View.Pages) != 1 {
			t.Fatalf("pages=%d", len(out.View.Pages))
		}
		if len(out.View.Pages[0].Photos) != 1 {
			t.Fatalf("photos=%d", len(out.View.Pages[0].Photos))
		}
		if out.View.Pages[0].Photos[0].Display.URL == "" {
			t.Errorf("display URL empty")
		}
		if out.View.ProcessingCount != 0 {
			t.Errorf("processing=%d", out.View.ProcessingCount)
		}
	})

	t.Run("異常_published_は_ErrEditNotAllowed", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		_, err := pool.Exec(ctx, `
			UPDATE photobooks SET status='published', published_at=$2, updated_at=$2,
			       public_url_slug='ne12dd34xy56gh78', manage_url_token_hash=decode(repeat('00',32),'hex'),
			       draft_edit_token_hash=NULL, draft_expires_at=NULL
			 WHERE id=$1
		`,
			pgtype.UUID{Bytes: pb.ID().UUID(), Valid: true},
			pgtype.Timestamptz{Time: now, Valid: true},
		)
		if err != nil {
			t.Fatalf("UPDATE: %v", err)
		}
		fakeR2 := &uploadtests.FakeR2Client{}
		uc := usecase.NewGetEditView(pool, fakeR2)
		_, err = uc.Execute(ctx, usecase.GetEditViewInput{PhotobookID: pb.ID()})
		if !errors.Is(err, usecase.ErrEditNotAllowed) {
			t.Errorf("err=%v want ErrEditNotAllowed", err)
		}
	})

	t.Run("異常_不存在は_ErrEditPhotobookNotFound", func(t *testing.T) {
		truncateAll(t, pool)
		fakeR2 := &uploadtests.FakeR2Client{}
		uc := usecase.NewGetEditView(pool, fakeR2)
		// seedPhotobook の前に find する
		pb := seedPhotobook(t, pool)
		// 削除して確認
		_, _ = pool.Exec(ctx, `DELETE FROM photobooks WHERE id=$1`,
			pgtype.UUID{Bytes: pb.ID().UUID(), Valid: true})
		_, err := uc.Execute(ctx, usecase.GetEditViewInput{PhotobookID: pb.ID()})
		if !errors.Is(err, usecase.ErrEditPhotobookNotFound) {
			t.Errorf("err=%v want ErrEditPhotobookNotFound", err)
		}
	})
}

