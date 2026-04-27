// PR30 同一 TX 統合テスト: ProcessImage が available / failed に遷移したとき、
// outbox_events に対応 event が 1 行 INSERT されることを確認。
package usecase_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	processorusecase "vrcpb/backend/internal/imageprocessor/internal/usecase"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	uploadtests "vrcpb/backend/internal/imageupload/tests"
)

func truncateOutboxAndImages(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"TRUNCATE TABLE outbox_events, image_variants, images, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
}

func countOutboxByType(t *testing.T, pool *pgxpool.Pool, eventType string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*)::int FROM outbox_events WHERE event_type = $1",
		eventType).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestProcessImageOutboxAvailable(t *testing.T) {
	pool := dbPool(t)
	truncateOutboxAndImages(t, pool)
	ctx := context.Background()

	pid := seedPhotobook(t, pool)
	img := seedProcessingImage(t, pool, pid, "jpg")

	body := makeIntegrationJPEG(t, 64, 48)
	fakeKey := "photobooks/X/images/Y/original/Z.jpg"
	fakeR2 := &uploadtests.FakeR2Client{
		ListObjectsFn: func(_ context.Context, _ string) (r2.ListObjectsOutput, error) {
			return r2.ListObjectsOutput{Keys: []string{fakeKey}}, nil
		},
		GetObjectFn: func(_ context.Context, _ string) (r2.GetObjectOutput, error) {
			return r2.GetObjectOutput{
				Body:          io.NopCloser(bytes.NewReader(body)),
				ContentLength: int64(len(body)),
				ContentType:   "image/jpeg",
			}, nil
		},
	}
	uc := processorusecase.NewProcessImage(pool, fakeR2, nil)
	out, err := uc.Execute(ctx, processorusecase.ProcessImageInput{ImageID: img, Now: time.Now().UTC()})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Status != "available" {
		t.Fatalf("status=%s want available", out.Status)
	}
	if got := countOutboxByType(t, pool, "image.became_available"); got != 1 {
		t.Errorf("image.became_available count=%d want 1", got)
	}
	if got := countOutboxByType(t, pool, "image.failed"); got != 0 {
		t.Errorf("image.failed count=%d want 0", got)
	}
}

func TestProcessImageOutboxFailed(t *testing.T) {
	pool := dbPool(t)
	truncateOutboxAndImages(t, pool)
	ctx := context.Background()

	pid := seedPhotobook(t, pool)
	img := seedProcessingImage(t, pool, pid, "jpg")

	fakeR2 := &uploadtests.FakeR2Client{
		ListObjectsFn: func(_ context.Context, _ string) (r2.ListObjectsOutput, error) {
			return r2.ListObjectsOutput{Keys: []string{}}, nil
		},
	}
	uc := processorusecase.NewProcessImage(pool, fakeR2, nil)
	_, err := uc.Execute(ctx, processorusecase.ProcessImageInput{ImageID: img, Now: time.Now().UTC()})
	if err == nil {
		t.Fatalf("expected ErrorWithReason for object_not_found")
	}
	if got := countOutboxByType(t, pool, "image.failed"); got != 1 {
		t.Errorf("image.failed count=%d want 1", got)
	}
	if got := countOutboxByType(t, pool, "image.became_available"); got != 0 {
		t.Errorf("image.became_available count=%d want 0", got)
	}
}

// makeIntegrationJPEG は test fixture 用の小さい JPEG（imaging package で decode 可能）を返す。
func makeIntegrationJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0xC0, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}
