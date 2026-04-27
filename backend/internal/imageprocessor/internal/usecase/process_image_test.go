// ProcessImage UseCase の実 DB + fake R2 統合テスト。
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//	go -C backend test ./internal/imageprocessor/...
//
// 観点（plan §10A.2 / §13）:
//   - HEIC source_format は MarkFailed(unsupported_format) で短絡
//   - R2 ListObjects 0 件は MarkFailed(object_not_found)
//   - decode 失敗は MarkFailed(decode_failed)
//   - 正常 JPEG は MarkAvailable + display + thumbnail variants
package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/image/domain/vo/failure_reason"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	imagerdb "vrcpb/backend/internal/image/infrastructure/repository/rdb"
	processorusecase "vrcpb/backend/internal/imageprocessor/internal/usecase"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	uploadtests "vrcpb/backend/internal/imageupload/tests"
	photobookdomaintests "vrcpb/backend/internal/photobook/domain/tests"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookmarshaller "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/marshaller"
	photobooksqlc "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/sqlcgen"
)

// dbPool は DATABASE_URL が無ければ skip する pool 取得ヘルパ。
func dbPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(),
		"TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}

// seedPhotobook は draft photobook を 1 件 INSERT する。
func seedPhotobook(t *testing.T, pool *pgxpool.Pool) photobook_id.PhotobookID {
	t.Helper()
	pb := photobookdomaintests.NewPhotobookBuilder().Build(t)
	params, err := photobookmarshaller.ToCreateParams(pb)
	if err != nil {
		t.Fatalf("ToCreateParams: %v", err)
	}
	if err := photobooksqlc.New(pool).CreateDraftPhotobook(context.Background(), params); err != nil {
		t.Fatalf("CreateDraftPhotobook: %v", err)
	}
	return pb.ID()
}

// seedProcessingImage は images に status='processing' の row を直接 INSERT する。
//
// 通常 flow（IssueUploadIntent + CompleteUpload）を経由せずテスト前提条件を作るため。
func seedProcessingImage(
	t *testing.T,
	pool *pgxpool.Pool,
	pid photobook_id.PhotobookID,
	sourceFormat string,
) image_id.ImageID {
	t.Helper()
	iid, err := image_id.New()
	if err != nil {
		t.Fatalf("image_id.New: %v", err)
	}
	now := time.Now().UTC()
	_, err = pool.Exec(context.Background(), `
		INSERT INTO images
		    (id, owner_photobook_id, usage_kind, source_format, status,
		     uploaded_at, created_at, updated_at)
		VALUES
		    ($1, $2, 'photo', $3, 'processing', $4, $4, $4)
	`,
		pgtype.UUID{Bytes: iid.UUID(), Valid: true},
		pgtype.UUID{Bytes: pid.UUID(), Valid: true},
		sourceFormat,
		pgtype.Timestamptz{Time: now, Valid: true},
	)
	if err != nil {
		t.Fatalf("seed processing image: %v", err)
	}
	return iid
}

// makeJPEG は w x h の単純な JPEG を返す（test fixture）。
func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0xA0, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}

// nopCloser は io.NopCloser のテスト helper（bytes.Reader を ReadCloser に変換）。
func nopCloser(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(b))
}

func TestProcessImage(t *testing.T) {
	pool := dbPool(t)
	now := time.Now().UTC()

	t.Run("HEIC短絡_unsupported_format_でMarkFailed", func(t *testing.T) {
		pid := seedPhotobook(t, pool)
		iid := seedProcessingImage(t, pool, pid, "heic")
		fakeR2 := &uploadtests.FakeR2Client{}
		uc := processorusecase.NewProcessImage(pool, fakeR2, nil)
		out, err := uc.Execute(context.Background(), processorusecase.ProcessImageInput{ImageID: iid, Now: now})

		reason, ok := processorusecase.IsProcessFailedReason(err)
		if !ok || !reason.Equal(failure_reason.UnsupportedFormat()) {
			t.Fatalf("err=%v, want ErrorWithReason(unsupported_format)", err)
		}
		if out.Status != "failed" {
			t.Errorf("status=%s want failed", out.Status)
		}

		// DB 確認
		repo := imagerdb.NewImageRepository(pool)
		got, err := repo.FindByID(context.Background(), iid)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if !got.IsFailed() {
			t.Errorf("status = %s want failed", got.Status().String())
		}
		if got.FailureReason() == nil || !got.FailureReason().Equal(failure_reason.UnsupportedFormat()) {
			t.Errorf("failure_reason=%v", got.FailureReason())
		}
	})

	t.Run("ListObjects_0件_でMarkFailed_object_not_found", func(t *testing.T) {
		pid := seedPhotobook(t, pool)
		iid := seedProcessingImage(t, pool, pid, "jpg")
		fakeR2 := &uploadtests.FakeR2Client{
			ListObjectsFn: func(ctx context.Context, prefix string) (r2.ListObjectsOutput, error) {
				return r2.ListObjectsOutput{Keys: []string{}}, nil
			},
		}
		uc := processorusecase.NewProcessImage(pool, fakeR2, nil)
		_, err := uc.Execute(context.Background(), processorusecase.ProcessImageInput{ImageID: iid, Now: now})
		reason, ok := processorusecase.IsProcessFailedReason(err)
		if !ok || !reason.Equal(failure_reason.ObjectNotFound()) {
			t.Fatalf("err=%v, want ObjectNotFound", err)
		}
	})

	t.Run("GetObject_NoSuchKey_でMarkFailed", func(t *testing.T) {
		pid := seedPhotobook(t, pool)
		iid := seedProcessingImage(t, pool, pid, "jpg")
		fakeKey := "photobooks/X/images/Y/original/Z.jpg"
		fakeR2 := &uploadtests.FakeR2Client{
			ListObjectsFn: func(ctx context.Context, prefix string) (r2.ListObjectsOutput, error) {
				return r2.ListObjectsOutput{Keys: []string{fakeKey}}, nil
			},
			GetObjectFn: func(ctx context.Context, key string) (r2.GetObjectOutput, error) {
				return r2.GetObjectOutput{}, r2.ErrObjectNotFound
			},
		}
		uc := processorusecase.NewProcessImage(pool, fakeR2, nil)
		_, err := uc.Execute(context.Background(), processorusecase.ProcessImageInput{ImageID: iid, Now: now})
		reason, ok := processorusecase.IsProcessFailedReason(err)
		if !ok || !reason.Equal(failure_reason.ObjectNotFound()) {
			t.Fatalf("err=%v, want ObjectNotFound", err)
		}
	})

	t.Run("decode失敗_でMarkFailed_decode_failed", func(t *testing.T) {
		pid := seedPhotobook(t, pool)
		iid := seedProcessingImage(t, pool, pid, "jpg")
		fakeKey := "photobooks/X/images/Y/original/Z.jpg"
		fakeR2 := &uploadtests.FakeR2Client{
			ListObjectsFn: func(ctx context.Context, prefix string) (r2.ListObjectsOutput, error) {
				return r2.ListObjectsOutput{Keys: []string{fakeKey}}, nil
			},
			GetObjectFn: func(ctx context.Context, key string) (r2.GetObjectOutput, error) {
				return r2.GetObjectOutput{Body: nopCloser([]byte("garbage")), ContentLength: 7, ContentType: "image/jpeg"}, nil
			},
		}
		uc := processorusecase.NewProcessImage(pool, fakeR2, nil)
		_, err := uc.Execute(context.Background(), processorusecase.ProcessImageInput{ImageID: iid, Now: now})
		reason, ok := processorusecase.IsProcessFailedReason(err)
		if !ok || !reason.Equal(failure_reason.DecodeFailed()) {
			t.Fatalf("err=%v, want DecodeFailed", err)
		}
	})

	t.Run("正常_JPEG_でMarkAvailable_2variants", func(t *testing.T) {
		pid := seedPhotobook(t, pool)
		iid := seedProcessingImage(t, pool, pid, "jpg")
		fakeKey := "photobooks/X/images/Y/original/Z.jpg"
		body := makeJPEG(t, 800, 600)

		var putKeys []string
		var deletedKey string
		fakeR2 := &uploadtests.FakeR2Client{
			ListObjectsFn: func(ctx context.Context, prefix string) (r2.ListObjectsOutput, error) {
				return r2.ListObjectsOutput{Keys: []string{fakeKey}}, nil
			},
			GetObjectFn: func(ctx context.Context, key string) (r2.GetObjectOutput, error) {
				return r2.GetObjectOutput{
					Body:          nopCloser(body),
					ContentLength: int64(len(body)),
					ContentType:   "image/jpeg",
				}, nil
			},
			PutObjectFn: func(ctx context.Context, in r2.PutObjectInput) error {
				putKeys = append(putKeys, in.StorageKey)
				return nil
			},
			DeleteObjectFn: func(ctx context.Context, key string) error {
				deletedKey = key
				return nil
			},
		}
		uc := processorusecase.NewProcessImage(pool, fakeR2, nil)
		out, err := uc.Execute(context.Background(), processorusecase.ProcessImageInput{ImageID: iid, Now: now})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.Status != "available" || out.VariantCount != 2 {
			t.Fatalf("out=%+v want available/2", out)
		}
		if len(putKeys) != 2 {
			t.Errorf("put count = %d want 2", len(putKeys))
		}
		if deletedKey != fakeKey {
			t.Errorf("deletedKey=%s want %s", deletedKey, fakeKey)
		}

		// DB 確認
		repo := imagerdb.NewImageRepository(pool)
		got, err := repo.FindByID(context.Background(), iid)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if !got.IsAvailable() {
			t.Errorf("status = %s want available", got.Status().String())
		}
		if len(got.Variants()) != 2 {
			t.Errorf("variants = %d want 2", len(got.Variants()))
		}
	})

	t.Run("R2_PUT失敗_processing据え置きでretry可能", func(t *testing.T) {
		pid := seedPhotobook(t, pool)
		iid := seedProcessingImage(t, pool, pid, "jpg")
		fakeKey := "photobooks/X/images/Y/original/Z.jpg"
		body := makeJPEG(t, 200, 200)
		fakeR2 := &uploadtests.FakeR2Client{
			ListObjectsFn: func(ctx context.Context, prefix string) (r2.ListObjectsOutput, error) {
				return r2.ListObjectsOutput{Keys: []string{fakeKey}}, nil
			},
			GetObjectFn: func(ctx context.Context, key string) (r2.GetObjectOutput, error) {
				return r2.GetObjectOutput{Body: nopCloser(body), ContentLength: int64(len(body)), ContentType: "image/jpeg"}, nil
			},
			PutObjectFn: func(ctx context.Context, in r2.PutObjectInput) error {
				return errors.New("simulated R2 PUT failure")
			},
		}
		uc := processorusecase.NewProcessImage(pool, fakeR2, nil)
		_, err := uc.Execute(context.Background(), processorusecase.ProcessImageInput{ImageID: iid, Now: now})
		if !errors.Is(err, processorusecase.ErrR2Unavailable) {
			t.Fatalf("err=%v want ErrR2Unavailable", err)
		}

		// 状態は processing のまま
		repo := imagerdb.NewImageRepository(pool)
		got, err := repo.FindByID(context.Background(), iid)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if !got.IsProcessing() {
			t.Errorf("status=%s want processing", got.Status().String())
		}
	})
}

// _ = uuid.New を保つ（後続で fixture 拡張時に使用予定）。
var _ = uuid.New
