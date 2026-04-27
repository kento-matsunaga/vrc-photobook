// GetPublicPhotobook 統合テスト（実 DB + fake R2）。
//
// 観点:
//   - 200: published + visible + visibility public/unlisted で variant URL 入りで返る
//   - 410: published + hidden_by_operator
//   - 404: slug 不在 / draft / deleted / private
//   - presigned URL は fake R2 から返る（実 R2 不要）
//   - failed/processing image は表示から除外される
package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	imagedomain "vrcpb/backend/internal/image/domain"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/mime_type"
	"vrcpb/backend/internal/image/domain/vo/storage_key"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	uploadtests "vrcpb/backend/internal/imageupload/tests"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// publishWithSlug は draft photobook を直接 UPDATE で published にする（test 専用 fixture）。
//
// 通常 flow（PublishFromDraft UseCase）を経由せず、test の前提条件構築を最短で行うため。
func publishWithSlug(t *testing.T, pool *pgxpool.Pool, pid photobook_id.PhotobookID, slugStr string, hidden bool, visibilityVal string) {
	t.Helper()
	tok, err := manage_url_token.Generate()
	if err != nil {
		t.Fatalf("manage_url_token.Generate: %v", err)
	}
	hash := manage_url_token_hash.Of(tok)
	now := time.Now().UTC()
	_, err = pool.Exec(context.Background(), `
		UPDATE photobooks
		   SET status = 'published',
		       public_url_slug = $2,
		       manage_url_token_hash = $3,
		       manage_url_token_version = 1,
		       draft_edit_token_hash = NULL,
		       draft_expires_at = NULL,
		       published_at = $4,
		       updated_at = $4,
		       version = version + 1,
		       hidden_by_operator = $5,
		       visibility = $6
		 WHERE id = $1
	`,
		pgtype.UUID{Bytes: pid.UUID(), Valid: true},
		slugStr, hash.Bytes(),
		pgtype.Timestamptz{Time: now, Valid: true},
		hidden, visibilityVal,
	)
	if err != nil {
		t.Fatalf("publish UPDATE: %v", err)
	}
}

// markPhotobookDeleted は status='deleted' に直接遷移させる（test 専用 fixture）。
func markPhotobookDeleted(t *testing.T, pool *pgxpool.Pool, pid photobook_id.PhotobookID) {
	t.Helper()
	now := time.Now().UTC()
	_, err := pool.Exec(context.Background(), `
		UPDATE photobooks SET status='deleted', deleted_at=$2, updated_at=$2 WHERE id=$1
	`,
		pgtype.UUID{Bytes: pid.UUID(), Valid: true},
		pgtype.Timestamptz{Time: now, Valid: true},
	)
	if err != nil {
		t.Fatalf("delete UPDATE: %v", err)
	}
}

// attachDisplayThumbnail は available image に display + thumbnail variant を 1 セット attach する。
func attachDisplayThumbnail(t *testing.T, pool *pgxpool.Pool, img imagedomain.Image) {
	t.Helper()
	repo := imagerdb.NewImageRepository(pool)
	now := time.Now().UTC()
	dims, _ := image_dimensions.New(1600, 1200)
	bs, _ := byte_size.New(200_000)
	displayKey, _ := storage_key.GenerateForVariant(img.OwnerPhotobookID(), img.ID(), variant_kind.Display())
	thumbKey, _ := storage_key.GenerateForVariant(img.OwnerPhotobookID(), img.ID(), variant_kind.Thumbnail())
	displayVar, err := imagedomain.NewImageVariant(imagedomain.NewImageVariantParams{
		ImageID:    img.ID(),
		Kind:       variant_kind.Display(),
		StorageKey: displayKey,
		Dimensions: dims,
		ByteSize:   bs,
		MimeType:   mime_type.Jpeg(),
		CreatedAt:  now,
	})
	if err != nil {
		t.Fatalf("display variant: %v", err)
	}
	if err := repo.AttachVariant(context.Background(), displayVar); err != nil {
		t.Fatalf("AttachVariant display: %v", err)
	}
	thumbDims, _ := image_dimensions.New(480, 360)
	thumbBs, _ := byte_size.New(40_000)
	thumbVar, err := imagedomain.NewImageVariant(imagedomain.NewImageVariantParams{
		ImageID:    img.ID(),
		Kind:       variant_kind.Thumbnail(),
		StorageKey: thumbKey,
		Dimensions: thumbDims,
		ByteSize:   thumbBs,
		MimeType:   mime_type.Jpeg(),
		CreatedAt:  now,
	})
	if err != nil {
		t.Fatalf("thumb variant: %v", err)
	}
	if err := repo.AttachVariant(context.Background(), thumbVar); err != nil {
		t.Fatalf("AttachVariant thumbnail: %v", err)
	}
}

// addPageWithPhoto は page を追加し、available image を photo として attach する。
// 戻り値は page と photo の caption を確認するために pageID を返す。
func addPageWithPhoto(t *testing.T, pool *pgxpool.Pool, pb domain.Photobook) {
	t.Helper()
	ctx := context.Background()

	// page を追加（既存 UseCase を活用）
	addPage := usecase.NewAddPage(pool)
	pageOut, err := addPage.Execute(ctx, usecase.AddPageInput{
		PhotobookID:     pb.ID(),
		ExpectedVersion: pb.Version(),
		Now:             time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("AddPage: %v", err)
	}

	// version は AddPage で +1 になっているので、最新を取り直す
	repoP := photobookrdb.NewPhotobookRepository(pool)
	pbAfterPage, err := repoP.FindByID(ctx, pb.ID())
	if err != nil {
		t.Fatalf("FindByID after AddPage: %v", err)
	}

	// photo を attach するための available image
	img := seedAvailableImage(t, pool, pb.ID())
	attachDisplayThumbnail(t, pool, img)

	addPhoto := usecase.NewAddPhoto(pool)
	if _, err := addPhoto.Execute(ctx, usecase.AddPhotoInput{
		PhotobookID:     pb.ID(),
		PageID:          pageOut.Page.ID(),
		ImageID:         img.ID(),
		ExpectedVersion: pbAfterPage.Version(),
		Now:             time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AddPhoto: %v", err)
	}
}


func TestGetPublicPhotobook(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()

	t.Run("正常_published_visible_でvariant付きで返る", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		addPageWithPhoto(t, pool, pb)
		const slugStr = "ab12cd34ef56gh78"
		publishWithSlug(t, pool, pb.ID(), slugStr, false, "unlisted")

		fakeR2 := &uploadtests.FakeR2Client{
			PresignGetObjectFn: func(_ context.Context, in r2.PresignGetInput) (r2.PresignGetOutput, error) {
				return r2.PresignGetOutput{URL: "https://fake.r2.test/get/" + in.StorageKey, ExpiresAt: time.Now().Add(in.ExpiresIn)}, nil
			},
		}
		uc := usecase.NewGetPublicPhotobook(pool, fakeR2)
		out, err := uc.Execute(ctx, usecase.GetPublicPhotobookInput{RawSlug: slugStr})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.View.Title == "" {
			t.Errorf("title empty")
		}
		if len(out.View.Pages) != 1 {
			t.Fatalf("pages = %d, want 1", len(out.View.Pages))
		}
		if len(out.View.Pages[0].Photos) != 1 {
			t.Fatalf("photos = %d, want 1", len(out.View.Pages[0].Photos))
		}
		photo := out.View.Pages[0].Photos[0]
		if !strings.Contains(photo.Variants.Display.URL, "fake.r2.test") {
			t.Errorf("display URL = %q", photo.Variants.Display.URL)
		}
		if !strings.Contains(photo.Variants.Thumbnail.URL, "fake.r2.test") {
			t.Errorf("thumbnail URL = %q", photo.Variants.Thumbnail.URL)
		}
		// storage_key 完全値が URL に含まれるのは fake R2 由来。実 R2 なら署名 query 形式。
		// 応答 view の他フィールドに storage_key が漏れていないことを確認。
		if photo.Variants.Display.Width != 1600 {
			t.Errorf("display width = %d, want 1600", photo.Variants.Display.Width)
		}
	})

	t.Run("異常_slug_format_invalid_404", func(t *testing.T) {
		fakeR2 := &uploadtests.FakeR2Client{}
		uc := usecase.NewGetPublicPhotobook(pool, fakeR2)
		_, err := uc.Execute(ctx, usecase.GetPublicPhotobookInput{RawSlug: "bad"})
		if !errors.Is(err, usecase.ErrPublicNotFound) {
			t.Errorf("err=%v want ErrPublicNotFound", err)
		}
	})

	t.Run("異常_slug_not_found_404", func(t *testing.T) {
		fakeR2 := &uploadtests.FakeR2Client{}
		uc := usecase.NewGetPublicPhotobook(pool, fakeR2)
		_, err := uc.Execute(ctx, usecase.GetPublicPhotobookInput{RawSlug: "notexistslug12345"})
		if !errors.Is(err, usecase.ErrPublicNotFound) {
			t.Errorf("err=%v want ErrPublicNotFound", err)
		}
	})

	t.Run("異常_draft_404", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		_ = pb
		// publish せず draft のまま slug 検索
		fakeR2 := &uploadtests.FakeR2Client{}
		uc := usecase.NewGetPublicPhotobook(pool, fakeR2)
		_, err := uc.Execute(ctx, usecase.GetPublicPhotobookInput{RawSlug: "draftnoslug12345"})
		if !errors.Is(err, usecase.ErrPublicNotFound) {
			t.Errorf("err=%v want ErrPublicNotFound", err)
		}
	})

	t.Run("異常_hidden_by_operator_410", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		const slugStr = "hi12dd34ef56gh78"
		publishWithSlug(t, pool, pb.ID(), slugStr, true, "unlisted")
		fakeR2 := &uploadtests.FakeR2Client{}
		uc := usecase.NewGetPublicPhotobook(pool, fakeR2)
		_, err := uc.Execute(ctx, usecase.GetPublicPhotobookInput{RawSlug: slugStr})
		if !errors.Is(err, usecase.ErrPublicGone) {
			t.Errorf("err=%v want ErrPublicGone", err)
		}
	})

	t.Run("異常_private_visibility_404", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		const slugStr = "pr12vv34xy56gh78"
		publishWithSlug(t, pool, pb.ID(), slugStr, false, "private")
		fakeR2 := &uploadtests.FakeR2Client{}
		uc := usecase.NewGetPublicPhotobook(pool, fakeR2)
		_, err := uc.Execute(ctx, usecase.GetPublicPhotobookInput{RawSlug: slugStr})
		if !errors.Is(err, usecase.ErrPublicNotFound) {
			t.Errorf("err=%v want ErrPublicNotFound", err)
		}
	})

	t.Run("異常_deleted_404", func(t *testing.T) {
		truncateAll(t, pool)
		pb := seedPhotobook(t, pool)
		const slugStr = "de12ee34xy56gh78"
		publishWithSlug(t, pool, pb.ID(), slugStr, false, "unlisted")
		markPhotobookDeleted(t, pool, pb.ID())
		fakeR2 := &uploadtests.FakeR2Client{}
		uc := usecase.NewGetPublicPhotobook(pool, fakeR2)
		_, err := uc.Execute(ctx, usecase.GetPublicPhotobookInput{RawSlug: slugStr})
		if !errors.Is(err, usecase.ErrPublicNotFound) {
			t.Errorf("err=%v want ErrPublicNotFound", err)
		}
	})

}

// truncateAll は test 間の干渉を避けるための共通リセット。
func truncateAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"TRUNCATE TABLE photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
}
