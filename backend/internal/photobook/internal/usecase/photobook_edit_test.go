// PR19 Photobook 編集系 UseCase の実 DB 統合テスト。
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//	go -C backend test ./internal/photobook/internal/usecase/...
package usecase_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	imagedomain "vrcpb/backend/internal/image/domain"
	imagebuilders "vrcpb/backend/internal/image/domain/tests"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/normalized_format"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/domain"
	photobooktests "vrcpb/backend/internal/photobook/domain/tests"
	"vrcpb/backend/internal/photobook/domain/vo/display_order"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

func dbPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set; skipping (set to run repository test)")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(),
		"TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}

// seedPhotobook は draft photobook を 1 件 INSERT して返す。
func seedPhotobook(t *testing.T, pool *pgxpool.Pool) domain.Photobook {
	t.Helper()
	pb := photobooktests.NewPhotobookBuilder().Build(t)
	repo := photobookrdb.NewPhotobookRepository(pool)
	if err := repo.CreateDraft(context.Background(), pb); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	return pb
}

// seedAvailableImage は対象 photobook 所有の available image を 1 件作る。
func seedAvailableImage(t *testing.T, pool *pgxpool.Pool, ownerID photobook_id.PhotobookID) imagedomain.Image {
	t.Helper()
	repo := imagerdb.NewImageRepository(pool)
	img := imagebuilders.NewImageBuilder().WithOwnerPhotobookID(ownerID).Build(t)
	ctx := context.Background()
	if err := repo.CreateUploading(ctx, img); err != nil {
		t.Fatalf("CreateUploading: %v", err)
	}
	now := time.Now().UTC()
	processed, _ := img.MarkProcessing(now)
	if err := repo.MarkProcessing(ctx, processed); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}
	dims, _ := image_dimensions.New(800, 600)
	bs, _ := byte_size.New(50_000)
	avail, err := processed.MarkAvailable(imagedomain.MarkAvailableParams{
		NormalizedFormat:   normalized_format.Webp(),
		OriginalDimensions: dims,
		OriginalByteSize:   bs,
		MetadataStrippedAt: now.Add(time.Second),
		Now:                now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("MarkAvailable: %v", err)
	}
	if err := repo.MarkAvailable(ctx, avail); err != nil {
		t.Fatalf("repo.MarkAvailable: %v", err)
	}
	return avail
}

func TestAddPage(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("正常_最初のpageを追加できる", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := seedPhotobook(t, pool)
		uc := usecase.NewAddPage(pool)
		out, err := uc.Execute(ctx, usecase.AddPageInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("AddPage: %v", err)
		}
		if out.Page.DisplayOrder().Int() != 0 {
			t.Errorf("first page must be order=0, got %d", out.Page.DisplayOrder().Int())
		}
	})

	t.Run("異常_30page超過はErrPageLimitExceeded", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := seedPhotobook(t, pool)
		uc := usecase.NewAddPage(pool)
		ver := pb.Version()
		for i := 0; i < domain.MaxPagesPerPhotobook; i++ {
			if _, err := uc.Execute(ctx, usecase.AddPageInput{
				PhotobookID:     pb.ID(),
				ExpectedVersion: ver,
				Now:             now.Add(time.Duration(i) * time.Second),
			}); err != nil {
				t.Fatalf("AddPage[%d]: %v", i, err)
			}
			ver++
		}
		// 31 件目
		_, err := uc.Execute(ctx, usecase.AddPageInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: ver,
			Now:             now.Add(time.Hour),
		})
		if !errors.Is(err, domain.ErrPageLimitExceeded) {
			t.Errorf("err = %v want ErrPageLimitExceeded", err)
		}
	})

	t.Run("異常_version不一致はErrOptimisticLockConflict", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := seedPhotobook(t, pool)
		uc := usecase.NewAddPage(pool)
		_, err := uc.Execute(ctx, usecase.AddPageInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version() + 99, // 不一致
			Now:             now,
		})
		if !errors.Is(err, photobookrdb.ErrOptimisticLockConflict) {
			t.Errorf("err = %v want ErrOptimisticLockConflict", err)
		}
	})
}

func TestAddPhoto(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("正常_available_imageをattachできる", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := seedPhotobook(t, pool)
		img := seedAvailableImage(t, pool, pb.ID())
		// page を 1 つ作る
		pageOut, err := usecase.NewAddPage(pool).Execute(ctx, usecase.AddPageInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("AddPage: %v", err)
		}
		// AddPage で version+1 されたので expected = pb.Version() + 1
		photoOut, err := usecase.NewAddPhoto(pool).Execute(ctx, usecase.AddPhotoInput{
			PhotobookID:     pb.ID(),
			PageID:          pageOut.Page.ID(),
			ImageID:         img.ID(),
			ExpectedVersion: pb.Version() + 1,
			Now:             now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("AddPhoto: %v", err)
		}
		if !photoOut.Photo.ImageID().Equal(img.ID()) {
			t.Errorf("image_id mismatch")
		}
		if photoOut.Photo.DisplayOrder().Int() != 0 {
			t.Errorf("first photo order must be 0")
		}
	})

	t.Run("異常_別photobook所有のimageは拒否", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb1 := seedPhotobook(t, pool)
		pb2 := photobooktests.NewPhotobookBuilder().Build(t)
		repo := photobookrdb.NewPhotobookRepository(pool)
		if err := repo.CreateDraft(ctx, pb2); err != nil {
			t.Fatalf("CreateDraft pb2: %v", err)
		}
		img := seedAvailableImage(t, pool, pb2.ID()) // pb2 所有
		pageOut, _ := usecase.NewAddPage(pool).Execute(ctx, usecase.AddPageInput{
			PhotobookID:     pb1.ID(),
			ExpectedVersion: pb1.Version(),
			Now:             now,
		})
		_, err := usecase.NewAddPhoto(pool).Execute(ctx, usecase.AddPhotoInput{
			PhotobookID:     pb1.ID(),
			PageID:          pageOut.Page.ID(),
			ImageID:         img.ID(),
			ExpectedVersion: pb1.Version() + 1,
			Now:             now.Add(time.Second),
		})
		if !errors.Is(err, photobookrdb.ErrImageNotAttachable) {
			t.Errorf("err = %v want ErrImageNotAttachable", err)
		}
	})

	t.Run("異常_uploadingのimageは拒否", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := seedPhotobook(t, pool)
		// uploading のままの image を作る
		img := imagebuilders.NewImageBuilder().WithOwnerPhotobookID(pb.ID()).Build(t)
		if err := imagerdb.NewImageRepository(pool).CreateUploading(ctx, img); err != nil {
			t.Fatalf("CreateUploading: %v", err)
		}
		pageOut, _ := usecase.NewAddPage(pool).Execute(ctx, usecase.AddPageInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		_, err := usecase.NewAddPhoto(pool).Execute(ctx, usecase.AddPhotoInput{
			PhotobookID:     pb.ID(),
			PageID:          pageOut.Page.ID(),
			ImageID:         img.ID(),
			ExpectedVersion: pb.Version() + 1,
			Now:             now.Add(time.Second),
		})
		if !errors.Is(err, photobookrdb.ErrImageNotAttachable) {
			t.Errorf("err = %v want ErrImageNotAttachable", err)
		}
	})

	t.Run("異常_20photo超過はErrPhotoLimitExceeded", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := seedPhotobook(t, pool)
		pageOut, _ := usecase.NewAddPage(pool).Execute(ctx, usecase.AddPageInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		ver := pb.Version() + 1
		for i := 0; i < domain.MaxPhotosPerPage; i++ {
			img := seedAvailableImage(t, pool, pb.ID())
			if _, err := usecase.NewAddPhoto(pool).Execute(ctx, usecase.AddPhotoInput{
				PhotobookID:     pb.ID(),
				PageID:          pageOut.Page.ID(),
				ImageID:         img.ID(),
				ExpectedVersion: ver,
				Now:             now.Add(time.Duration(i) * time.Second),
			}); err != nil {
				t.Fatalf("AddPhoto[%d]: %v", i, err)
			}
			ver++
		}
		// 21 件目
		extra := seedAvailableImage(t, pool, pb.ID())
		_, err := usecase.NewAddPhoto(pool).Execute(ctx, usecase.AddPhotoInput{
			PhotobookID:     pb.ID(),
			PageID:          pageOut.Page.ID(),
			ImageID:         extra.ID(),
			ExpectedVersion: ver,
			Now:             now.Add(time.Hour),
		})
		if !errors.Is(err, domain.ErrPhotoLimitExceeded) {
			t.Errorf("err = %v want ErrPhotoLimitExceeded", err)
		}
	})
}

func TestSetCoverImage(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("正常_自所有available_imageをCoverに設定できる", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := seedPhotobook(t, pool)
		img := seedAvailableImage(t, pool, pb.ID())
		err := usecase.NewSetCoverImage(pool).Execute(ctx, usecase.SetCoverImageInput{
			PhotobookID:     pb.ID(),
			ImageID:         img.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("SetCoverImage: %v", err)
		}
		// DB から再取得して cover_image_id を確認
		got, err := photobookrdb.NewPhotobookRepository(pool).FindByID(ctx, pb.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if got.CoverImageID() == nil || !got.CoverImageID().Equal(img.ID()) {
			t.Errorf("cover_image_id not set")
		}
		if got.Version() != pb.Version()+1 {
			t.Errorf("version not bumped")
		}
	})

	t.Run("異常_別photobook所有imageはErrImageNotAttachable", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb1 := seedPhotobook(t, pool)
		pb2 := photobooktests.NewPhotobookBuilder().Build(t)
		repo := photobookrdb.NewPhotobookRepository(pool)
		if err := repo.CreateDraft(ctx, pb2); err != nil {
			t.Fatalf("CreateDraft pb2: %v", err)
		}
		img := seedAvailableImage(t, pool, pb2.ID())
		err := usecase.NewSetCoverImage(pool).Execute(ctx, usecase.SetCoverImageInput{
			PhotobookID:     pb1.ID(),
			ImageID:         img.ID(),
			ExpectedVersion: pb1.Version(),
			Now:             now,
		})
		if !errors.Is(err, photobookrdb.ErrImageNotAttachable) {
			t.Errorf("err = %v want ErrImageNotAttachable", err)
		}
	})
}

func TestRemovePage(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("正常_Page削除でphotosもCASCADE削除される", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := seedPhotobook(t, pool)
		img := seedAvailableImage(t, pool, pb.ID())
		pageOut, _ := usecase.NewAddPage(pool).Execute(ctx, usecase.AddPageInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		photoOut, err := usecase.NewAddPhoto(pool).Execute(ctx, usecase.AddPhotoInput{
			PhotobookID:     pb.ID(),
			PageID:          pageOut.Page.ID(),
			ImageID:         img.ID(),
			ExpectedVersion: pb.Version() + 1,
			Now:             now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("AddPhoto: %v", err)
		}
		// 削除
		err = usecase.NewRemovePage(pool).Execute(ctx, usecase.RemovePageInput{
			PhotobookID:     pb.ID(),
			PageID:          pageOut.Page.ID(),
			ExpectedVersion: pb.Version() + 2,
			Now:             now.Add(2 * time.Second),
		})
		if err != nil {
			t.Fatalf("RemovePage: %v", err)
		}
		// CASCADE 確認: photo が消えている
		repo := photobookrdb.NewPhotobookRepository(pool)
		photos, err := repo.ListPhotosByPageID(ctx, pageOut.Page.ID())
		if err != nil {
			t.Fatalf("ListPhotosByPageID: %v", err)
		}
		if len(photos) != 0 {
			t.Errorf("photos must be cascaded: got %d", len(photos))
		}
		_ = photoOut // unused warning suppression
	})
}

func TestReorderPhoto_UniqueViolation(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("MVP制約_既存order占有先への単純Reorderは23505で失敗する", func(t *testing.T) {
		// Given: 同一 page に photo A (order=0) と photo B (order=1) がある
		// When:  photo A を order=1 に移そうとする（B が占有中）
		// Then:  UNIQUE (page_id, display_order) 違反で 23505 が表面化
		// 設計判断: MVP では UpdatePhotobookPhotoOrder が単純 UPDATE であり、
		//   DEFERRABLE UNIQUE / 一時退避は採用しない（domain-standard.md / SQL コメント参照）
		//   将来「2 photo swap」「複数 reorder」を扱う場合は UseCase 側で
		//   一時退避 prefix（display_order>=1000）に逃がしてから順次 UPDATE する実装が必要
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := seedPhotobook(t, pool)
		imgA := seedAvailableImage(t, pool, pb.ID())
		imgB := seedAvailableImage(t, pool, pb.ID())
		pageOut, _ := usecase.NewAddPage(pool).Execute(ctx, usecase.AddPageInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		photoAOut, err := usecase.NewAddPhoto(pool).Execute(ctx, usecase.AddPhotoInput{
			PhotobookID:     pb.ID(),
			PageID:          pageOut.Page.ID(),
			ImageID:         imgA.ID(),
			ExpectedVersion: pb.Version() + 1,
			Now:             now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("AddPhoto A: %v", err)
		}
		_, err = usecase.NewAddPhoto(pool).Execute(ctx, usecase.AddPhotoInput{
			PhotobookID:     pb.ID(),
			PageID:          pageOut.Page.ID(),
			ImageID:         imgB.ID(),
			ExpectedVersion: pb.Version() + 2,
			Now:             now.Add(2 * time.Second),
		})
		if err != nil {
			t.Fatalf("AddPhoto B: %v", err)
		}
		// photo A を order=1 に動かそうとする → B が占有中
		newOrder, _ := display_order.New(1)
		err = usecase.NewReorderPhoto(pool).Execute(ctx, usecase.ReorderPhotoInput{
			PhotobookID:     pb.ID(),
			PhotoID:         photoAOut.Photo.ID(),
			NewOrder:        newOrder,
			ExpectedVersion: pb.Version() + 3,
			Now:             now.Add(3 * time.Second),
		})
		if err == nil {
			t.Fatalf("expected UNIQUE violation on order conflict, got nil")
		}
		// pgconn の SQLState 23505 が原因。UseCase 経由では Wrap されるが、err != nil で十分。
	})
}

func TestImageOnDeleteRestrict(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("異常_attach済みImageの削除はFK違反で拒否される", func(t *testing.T) {
		// Given: photobook + page + photo (image attach 済), When: image.MarkDeleted
		// では image 状態を deleted に遷移するだけで row は残る。実際の row DELETE を
		// 試すと photobook_photos の FK ON DELETE RESTRICT で拒否される。
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := seedPhotobook(t, pool)
		img := seedAvailableImage(t, pool, pb.ID())
		pageOut, _ := usecase.NewAddPage(pool).Execute(ctx, usecase.AddPageInput{
			PhotobookID:     pb.ID(),
			ExpectedVersion: pb.Version(),
			Now:             now,
		})
		if _, err := usecase.NewAddPhoto(pool).Execute(ctx, usecase.AddPhotoInput{
			PhotobookID:     pb.ID(),
			PageID:          pageOut.Page.ID(),
			ImageID:         img.ID(),
			ExpectedVersion: pb.Version() + 1,
			Now:             now.Add(time.Second),
		}); err != nil {
			t.Fatalf("AddPhoto: %v", err)
		}
		// image row を物理削除しようとすると ON DELETE RESTRICT
		_, err := pool.Exec(ctx, "DELETE FROM images WHERE id = $1", img.ID().UUID())
		if err == nil {
			t.Errorf("expected FK violation on image delete")
		}
	})
}
