// Repository test は実 PostgreSQL を必要とする（testing.md §テスト階層）。
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//	go -C backend test ./internal/image/infrastructure/repository/rdb/...
package rdb_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	imagedomain "vrcpb/backend/internal/image/domain"
	imagetests "vrcpb/backend/internal/image/domain/tests"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/failure_reason"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/normalized_format"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	photobookdomaintests "vrcpb/backend/internal/photobook/domain/tests"
	photobookmarshaller "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/marshaller"
	photobooksqlc "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/sqlcgen"
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
		"TRUNCATE TABLE image_variants, images, sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}


func TestImageRepository_CreateAndFind(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := imagerdb.NewImageRepository(pool)

	t.Run("正常_uploadingを作成してfindできる", func(t *testing.T) {
		// Given: draft photobook seed + uploading image
		// When:  CreateUploading + FindByID
		// Then:  ID 一致 / status=uploading / variants は空
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := photobookdomaintests.NewPhotobookBuilder().Build(t)
		params, err := photobookmarshaller.ToCreateParams(pb)
		if err != nil {
			t.Fatalf("ToCreateParams: %v", err)
		}
		if err := photobooksqlc.New(pool).CreateDraftPhotobook(ctx, params); err != nil {
			t.Fatalf("seed photobook: %v", err)
		}
		img := imagetests.NewImageBuilder().WithOwnerPhotobookID(pb.ID()).Build(t)
		if err := repo.CreateUploading(ctx, img); err != nil {
			t.Fatalf("CreateUploading: %v", err)
		}
		got, err := repo.FindByID(ctx, img.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if !got.ID().Equal(img.ID()) {
			t.Errorf("ID mismatch")
		}
		if !got.IsUploading() {
			t.Errorf("status must be uploading, got %s", got.Status().String())
		}
		if len(got.Variants()) != 0 {
			t.Errorf("variants should be empty initially")
		}
	})

	t.Run("異常_owner_photobook_idがDB上に存在しないとFK違反", func(t *testing.T) {
		// Given: photobook seed なし, When: CreateUploading, Then: pgx FK 違反
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		img := imagetests.NewImageBuilder().Build(t)
		err := repo.CreateUploading(ctx, img)
		if err == nil {
			t.Fatalf("expected FK violation")
		}
	})

	t.Run("異常_存在しないIDのFindByIDはErrNotFound", func(t *testing.T) {
		// Given: 何も入っていない, When: FindByID, Then: ErrNotFound
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		img := imagetests.NewImageBuilder().Build(t)
		_, err := repo.FindByID(ctx, img.ID())
		if !errors.Is(err, imagerdb.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestImageRepository_StateTransitions(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := imagerdb.NewImageRepository(pool)

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	dims, _ := image_dimensions.New(1024, 768)
	bs, _ := byte_size.New(500_000)
	availParams := imagedomain.MarkAvailableParams{
		NormalizedFormat:   normalized_format.Webp(),
		OriginalDimensions: dims,
		OriginalByteSize:   bs,
		MetadataStrippedAt: now.Add(2 * time.Second),
		Now:                now.Add(3 * time.Second),
	}

	prepare := func(t *testing.T) imagedomain.Image {
		t.Helper()
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := photobookdomaintests.NewPhotobookBuilder().Build(t)
		params, err := photobookmarshaller.ToCreateParams(pb)
		if err != nil {
			t.Fatalf("ToCreateParams: %v", err)
		}
		if err := photobooksqlc.New(pool).CreateDraftPhotobook(ctx, params); err != nil {
			t.Fatalf("seed photobook: %v", err)
		}
		img := imagetests.NewImageBuilder().WithOwnerPhotobookID(pb.ID()).WithNow(now).Build(t)
		if err := repo.CreateUploading(ctx, img); err != nil {
			t.Fatalf("CreateUploading: %v", err)
		}
		return img
	}

	t.Run("正常_uploading→processing→available", func(t *testing.T) {
		img := prepare(t)
		p, err := img.MarkProcessing(now.Add(time.Second))
		if err != nil {
			t.Fatalf("MarkProcessing: %v", err)
		}
		if err := repo.MarkProcessing(ctx, p); err != nil {
			t.Fatalf("repo.MarkProcessing: %v", err)
		}
		a, err := p.MarkAvailable(availParams)
		if err != nil {
			t.Fatalf("MarkAvailable: %v", err)
		}
		if err := repo.MarkAvailable(ctx, a); err != nil {
			t.Fatalf("repo.MarkAvailable: %v", err)
		}
		got, err := repo.FindByID(ctx, img.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if !got.IsAvailable() {
			t.Fatalf("status must be available, got %s", got.Status().String())
		}
		if got.NormalizedFormat() == nil || got.OriginalDimensions() == nil ||
			got.OriginalByteSize() == nil || got.MetadataStrippedAt() == nil ||
			got.AvailableAt() == nil {
			t.Errorf("required fields must be populated")
		}
	})

	t.Run("正常_uploading→failed", func(t *testing.T) {
		img := prepare(t)
		f, err := img.MarkFailed(failure_reason.FileTooLarge(), now.Add(time.Second))
		if err != nil {
			t.Fatalf("MarkFailed: %v", err)
		}
		if err := repo.MarkFailed(ctx, f); err != nil {
			t.Fatalf("repo.MarkFailed: %v", err)
		}
		got, err := repo.FindByID(ctx, img.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if !got.IsFailed() {
			t.Fatalf("status must be failed")
		}
		if got.FailureReason() == nil || got.FailureReason().String() != "file_too_large" {
			t.Errorf("failure_reason mismatch")
		}
	})

	t.Run("異常_processingでないimageへのMarkProcessingはErrConflict", func(t *testing.T) {
		img := prepare(t)
		// 1 度 processing に進めた後、もう一度 MarkProcessing しても更新されない
		p, _ := img.MarkProcessing(now.Add(time.Second))
		if err := repo.MarkProcessing(ctx, p); err != nil {
			t.Fatalf("first: %v", err)
		}
		err := repo.MarkProcessing(ctx, p)
		if !errors.Is(err, imagerdb.ErrConflict) {
			t.Errorf("err = %v, want ErrConflict", err)
		}
	})

	t.Run("正常_available→deleted", func(t *testing.T) {
		img := prepare(t)
		p, _ := img.MarkProcessing(now.Add(time.Second))
		_ = repo.MarkProcessing(ctx, p)
		a, _ := p.MarkAvailable(availParams)
		_ = repo.MarkAvailable(ctx, a)
		d, err := a.MarkDeleted(now.Add(10 * time.Second))
		if err != nil {
			t.Fatalf("MarkDeleted: %v", err)
		}
		if err := repo.MarkDeleted(ctx, d); err != nil {
			t.Fatalf("repo.MarkDeleted: %v", err)
		}
		got, err := repo.FindByID(ctx, img.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if !got.IsDeleted() || got.DeletedAt() == nil {
			t.Errorf("status must be deleted with deleted_at set")
		}
	})
}

func TestImageRepository_AttachVariant(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := imagerdb.NewImageRepository(pool)

	prepare := func(t *testing.T) imagedomain.Image {
		t.Helper()
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := photobookdomaintests.NewPhotobookBuilder().Build(t)
		params, err := photobookmarshaller.ToCreateParams(pb)
		if err != nil {
			t.Fatalf("ToCreateParams: %v", err)
		}
		if err := photobooksqlc.New(pool).CreateDraftPhotobook(ctx, params); err != nil {
			t.Fatalf("seed photobook: %v", err)
		}
		img := imagetests.NewImageBuilder().WithOwnerPhotobookID(pb.ID()).Build(t)
		if err := repo.CreateUploading(ctx, img); err != nil {
			t.Fatalf("CreateUploading: %v", err)
		}
		return img
	}

	t.Run("正常_displayをattachして取得できる", func(t *testing.T) {
		img := prepare(t)
		v := imagetests.MakeDisplayVariant(t, img)
		if err := repo.AttachVariant(ctx, v); err != nil {
			t.Fatalf("AttachVariant: %v", err)
		}
		got, err := repo.FindByID(ctx, img.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if len(got.Variants()) != 1 {
			t.Fatalf("variants len mismatch")
		}
		if !got.Variants()[0].Kind().Equal(variant_kind.Display()) {
			t.Errorf("variant kind mismatch")
		}
	})

	t.Run("異常_同kindの2回目はErrDuplicateVariantKind", func(t *testing.T) {
		img := prepare(t)
		v1 := imagetests.MakeDisplayVariant(t, img)
		if err := repo.AttachVariant(ctx, v1); err != nil {
			t.Fatalf("AttachVariant#1: %v", err)
		}
		v2 := imagetests.MakeDisplayVariant(t, img)
		err := repo.AttachVariant(ctx, v2)
		if !errors.Is(err, imagerdb.ErrDuplicateVariantKind) {
			t.Errorf("err = %v, want ErrDuplicateVariantKind", err)
		}
	})
}

func TestImageRepository_OnDeleteRestrict(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := imagerdb.NewImageRepository(pool)

	t.Run("異常_imageが残っているphotobookを削除しようとするとFK違反", func(t *testing.T) {
		// Given: photobook + image, When: DELETE photobook, Then: 23503 FK 違反
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := photobookdomaintests.NewPhotobookBuilder().Build(t)
		params, err := photobookmarshaller.ToCreateParams(pb)
		if err != nil {
			t.Fatalf("ToCreateParams: %v", err)
		}
		if err := photobooksqlc.New(pool).CreateDraftPhotobook(ctx, params); err != nil {
			t.Fatalf("seed photobook: %v", err)
		}
		img := imagetests.NewImageBuilder().WithOwnerPhotobookID(pb.ID()).Build(t)
		if err := repo.CreateUploading(ctx, img); err != nil {
			t.Fatalf("CreateUploading: %v", err)
		}
		_, err = pool.Exec(ctx, "DELETE FROM photobooks WHERE id = $1", pb.ID().UUID())
		if err == nil {
			t.Fatalf("expected FK violation on photobook delete")
		}
	})
}

func TestImageRepository_ListActiveByPhotobookID(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := imagerdb.NewImageRepository(pool)

	t.Run("正常_削除済を除外して発行順に返す", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb := photobookdomaintests.NewPhotobookBuilder().Build(t)
		params, err := photobookmarshaller.ToCreateParams(pb)
		if err != nil {
			t.Fatalf("ToCreateParams: %v", err)
		}
		if err := photobooksqlc.New(pool).CreateDraftPhotobook(ctx, params); err != nil {
			t.Fatalf("seed photobook: %v", err)
		}

		base := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
		var imgs []imagedomain.Image
		for i := 0; i < 3; i++ {
			img := imagetests.NewImageBuilder().
				WithOwnerPhotobookID(pb.ID()).
				WithNow(base.Add(time.Duration(i) * time.Minute)).
				Build(t)
			if err := repo.CreateUploading(ctx, img); err != nil {
				t.Fatalf("CreateUploading[%d]: %v", i, err)
			}
			imgs = append(imgs, img)
		}
		// imgs[0] を削除済にする (uploading→failed→deleted)
		f, _ := imgs[0].MarkFailed(failure_reason.Unknown(), base.Add(10*time.Second))
		_ = repo.MarkFailed(ctx, f)
		d, _ := f.MarkDeleted(base.Add(20 * time.Second))
		if err := repo.MarkDeleted(ctx, d); err != nil {
			t.Fatalf("MarkDeleted: %v", err)
		}
		got, err := repo.ListActiveByPhotobookID(ctx, pb.ID())
		if err != nil {
			t.Fatalf("ListActiveByPhotobookID: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len=%d want 2", len(got))
		}
		// 発行順（uploaded_at ASC）
		if !got[0].ID().Equal(imgs[1].ID()) || !got[1].ID().Equal(imgs[2].ID()) {
			t.Errorf("order mismatch")
		}
	})
}

