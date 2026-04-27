// UseCase の DB + fake R2 統合テスト。
//
// 本 PR では UseCase が renderer + R2 PUT までで停止し、status='generated' への
// 遷移は行わないため、test 期待値も「Rendered=true / Uploaded=true / Generated=false」
// で確認する。
package usecase_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/ogp/infrastructure/renderer"
	ogprdb "vrcpb/backend/internal/ogp/infrastructure/repository/rdb"
	ogpusecase "vrcpb/backend/internal/ogp/internal/usecase"
	photobookid "vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

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
		"TRUNCATE TABLE photobook_ogp_images, image_variants, images, photobook_page_metas, photobook_photos, photobook_pages, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}

// fakeFetcher は固定 PhotobookView を返す test double。
type fakeFetcher struct {
	view ogpusecase.PhotobookView
	err  error
}

func (f *fakeFetcher) FetchForOgp(_ context.Context, id photobookid.PhotobookID) (ogpusecase.PhotobookView, error) {
	if f.err != nil {
		return ogpusecase.PhotobookView{}, f.err
	}
	v := f.view
	v.ID = id
	return v, nil
}

// fakeR2 は PutObject を記録する test double（PR23 / PR31 と同パターン）。
type fakeR2 struct {
	mu          sync.Mutex
	puts        []r2.PutObjectInput
	putErr      error
}

func (f *fakeR2) PresignPutObject(_ context.Context, _ r2.PresignPutInput) (r2.PresignPutOutput, error) {
	return r2.PresignPutOutput{}, errors.New("not implemented")
}
func (f *fakeR2) PresignGetObject(_ context.Context, _ r2.PresignGetInput) (r2.PresignGetOutput, error) {
	return r2.PresignGetOutput{}, errors.New("not implemented")
}
func (f *fakeR2) HeadObject(_ context.Context, _ string) (r2.HeadObjectOutput, error) {
	return r2.HeadObjectOutput{}, errors.New("not implemented")
}
func (f *fakeR2) DeleteObject(_ context.Context, _ string) error { return nil }
func (f *fakeR2) GetObject(_ context.Context, _ string) (r2.GetObjectOutput, error) {
	return r2.GetObjectOutput{}, errors.New("not implemented")
}
func (f *fakeR2) ListObjects(_ context.Context, _ string) (r2.ListObjectsOutput, error) {
	return r2.ListObjectsOutput{}, errors.New("not implemented")
}
func (f *fakeR2) PutObject(_ context.Context, in r2.PutObjectInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putErr != nil {
		return f.putErr
	}
	f.puts = append(f.puts, in)
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newRenderer(t *testing.T) *renderer.Renderer {
	t.Helper()
	r, err := renderer.New()
	if err != nil {
		t.Fatalf("renderer.New: %v", err)
	}
	return r
}

// seedDraftPhotobook は最小 photobook を 1 件 INSERT して id を返す。
// draft_edit_token_hash は uuid 派生で test ごとに unique にする。
func seedDraftPhotobook(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, photobookid.PhotobookID) {
	t.Helper()
	pid := uuid.New()
	pidVO, err := photobookid.FromUUID(pid)
	if err != nil {
		t.Fatalf("photobook_id.FromUUID: %v", err)
	}
	hash := make([]byte, 32)
	copy(hash, pid[:])
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO photobooks (id, type, title, layout, opening_style, visibility, sensitive,
			rights_agreed, creator_display_name, manage_url_token_version,
			draft_edit_token_hash, draft_expires_at, status, hidden_by_operator, version,
			created_at, updated_at)
		VALUES ($1, 'memory', 'Test', 'simple', 'light', 'unlisted', false,
			true, 'tester', 0,
			$2, now() + interval '7 days',
			'draft', false, 0, now(), now())
	`, pgtype.UUID{Bytes: pid, Valid: true}, hash); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return pid, pidVO
}

func TestGenerate_Success(t *testing.T) {
	pool := dbPool(t)
	_, pidVO := seedDraftPhotobook(t, pool)

	fetcher := &fakeFetcher{view: ogpusecase.PhotobookView{
		Title: "Test Title", Type: "memory", CreatorDisplayName: "tester",
		IsPublished: true,
	}}
	r2c := &fakeR2{}
	uc := ogpusecase.NewGenerateOgpForPhotobook(pool, fetcher, r2c, newRenderer(t), "vrcpb-images", discardLogger())

	out, err := uc.Execute(context.Background(), ogpusecase.GenerateOgpInput{
		PhotobookID: pidVO,
		Now:         time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.Rendered || !out.Uploaded {
		t.Errorf("Rendered=%v Uploaded=%v want both true", out.Rendered, out.Uploaded)
	}
	if out.Generated {
		t.Errorf("PR33b では Generated=false 期待（PR33c で images row 作成 + MarkGenerated）")
	}
	if len(r2c.puts) != 1 {
		t.Fatalf("R2 PutObject 呼び出し回数 %d != 1", len(r2c.puts))
	}
	put := r2c.puts[0]
	if put.ContentType != "image/png" {
		t.Errorf("content-type=%s want image/png", put.ContentType)
	}
	if !strings.HasPrefix(put.StorageKey, "photobooks/"+pidVO.String()+"/ogp/") {
		t.Errorf("storage_key prefix mismatch: %s", put.StorageKey)
	}
	if !strings.HasSuffix(put.StorageKey, ".png") {
		t.Errorf("storage_key suffix mismatch: %s", put.StorageKey)
	}
	if len(put.Body) < 1024 {
		t.Errorf("body too small: %d", len(put.Body))
	}

	row, err := ogprdb.NewOgpRepository(pool).FindByPhotobookID(context.Background(), pidVO)
	if err != nil {
		t.Fatalf("FindByPhotobookID: %v", err)
	}
	if !row.Status().IsPending() {
		t.Errorf("status=%s want pending（PR33b は MarkGenerated しない）", row.Status().String())
	}
}

func TestGenerate_NotPublishedSkipped(t *testing.T) {
	pool := dbPool(t)
	pidVO, _ := photobookid.FromUUID(uuid.New())

	fetcher := &fakeFetcher{view: ogpusecase.PhotobookView{
		Title: "x", Type: "memory", CreatorDisplayName: "y",
		IsPublished: false,
	}}
	r2c := &fakeR2{}
	uc := ogpusecase.NewGenerateOgpForPhotobook(pool, fetcher, r2c, newRenderer(t), "vrcpb-images", discardLogger())

	_, err := uc.Execute(context.Background(), ogpusecase.GenerateOgpInput{
		PhotobookID: pidVO, Now: time.Now().UTC(),
	})
	if !errors.Is(err, ogpusecase.ErrNotPublished) {
		t.Errorf("err=%v want ErrNotPublished", err)
	}
	if len(r2c.puts) != 0 {
		t.Errorf("R2 PutObject 呼ばれてはいけない (calls=%d)", len(r2c.puts))
	}
}

func TestGenerate_R2PutFailureMarksFailed(t *testing.T) {
	pool := dbPool(t)
	_, pidVO := seedDraftPhotobook(t, pool)

	fetcher := &fakeFetcher{view: ogpusecase.PhotobookView{
		Title: "x", Type: "memory", CreatorDisplayName: "y", IsPublished: true,
	}}
	r2c := &fakeR2{putErr: errors.New("simulated put failure")}
	uc := ogpusecase.NewGenerateOgpForPhotobook(pool, fetcher, r2c, newRenderer(t), "vrcpb-images", discardLogger())

	_, err := uc.Execute(context.Background(), ogpusecase.GenerateOgpInput{
		PhotobookID: pidVO, Now: time.Now().UTC(),
	})
	if err == nil {
		t.Fatalf("expected error from R2 put failure")
	}

	row, err := ogprdb.NewOgpRepository(pool).FindByPhotobookID(context.Background(), pidVO)
	if err != nil {
		t.Fatalf("FindByPhotobookID: %v", err)
	}
	if !row.Status().IsFailed() {
		t.Errorf("status=%s want failed", row.Status().String())
	}
	if row.FailureReason().IsZero() {
		t.Errorf("failure_reason must be set")
	}
}

func TestGenerate_DryRunNoPut(t *testing.T) {
	pool := dbPool(t)
	_, pidVO := seedDraftPhotobook(t, pool)

	fetcher := &fakeFetcher{view: ogpusecase.PhotobookView{
		Title: "Dry", Type: "memory", CreatorDisplayName: "tester", IsPublished: true,
	}}
	r2c := &fakeR2{}
	uc := ogpusecase.NewGenerateOgpForPhotobook(pool, fetcher, r2c, newRenderer(t), "vrcpb-images", discardLogger())

	out, err := uc.Execute(context.Background(), ogpusecase.GenerateOgpInput{
		PhotobookID: pidVO, Now: time.Now().UTC(), DryRun: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.Rendered || out.Uploaded {
		t.Errorf("dry-run: Rendered=%v Uploaded=%v want true/false", out.Rendered, out.Uploaded)
	}
	if len(r2c.puts) != 0 {
		t.Errorf("dry-run なのに R2 PutObject %d 件", len(r2c.puts))
	}
	// dry-run では DB row も作らない（CreatePending を skip）
	if _, err := ogprdb.NewOgpRepository(pool).FindByPhotobookID(context.Background(), pidVO); !errors.Is(err, ogprdb.ErrNotFound) {
		t.Errorf("dry-run なのに row 作成された: %v", err)
	}
}
