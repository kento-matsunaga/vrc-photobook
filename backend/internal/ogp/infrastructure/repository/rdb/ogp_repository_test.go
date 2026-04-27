// OgpRepository の実 DB 統合テスト。
//
// 実行方法:
//   docker compose -f backend/docker-compose.yaml up -d postgres
//   export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//   go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//   go -C backend test ./internal/ogp/infrastructure/repository/rdb/...
package rdb_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/ogp/domain"
	"vrcpb/backend/internal/ogp/domain/vo/ogp_failure_reason"
	ogprdb "vrcpb/backend/internal/ogp/infrastructure/repository/rdb"
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

// seedDraftPhotobook は最小 photobook を 1 件 INSERT して id を返す。
//
// status='draft' の最小列構成。draft_edit_token_hash は test ごとに unique
// （32 byte の id 由来）にして UNIQUE 制約衝突を避ける。
func seedDraftPhotobook(t *testing.T, pool *pgxpool.Pool) photobookid.PhotobookID {
	t.Helper()
	pid, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
	hash := make([]byte, 32)
	copy(hash, pid[:])
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO photobooks (
			id, type, title, layout, opening_style, visibility, sensitive,
			rights_agreed, creator_display_name,
			manage_url_token_version,
			draft_edit_token_hash, draft_expires_at,
			status, hidden_by_operator, version,
			created_at, updated_at
		) VALUES (
			$1, 'memory', 'test photobook', 'simple', 'light', 'unlisted', false,
			true, 'tester',
			0,
			$2, now() + interval '7 days',
			'draft', false, 0,
			now(), now()
		)
	`, pgtype.UUID{Bytes: pid, Valid: true}, hash); err != nil {
		t.Fatalf("seed photobook: %v", err)
	}
	pidVO, err := photobookid.FromUUID(pid)
	if err != nil {
		t.Fatalf("photobook_id.FromUUID: %v", err)
	}
	return pidVO
}

func TestOgpRepository_CreatePending_FindByPhotobookID(t *testing.T) {
	pool := dbPool(t)
	pid := seedDraftPhotobook(t, pool)
	now := time.Now().UTC().Truncate(time.Second)

	repo := ogprdb.NewOgpRepository(pool)
	ev, err := domain.NewPending(domain.NewPendingParams{PhotobookID: pid, Now: now})
	if err != nil {
		t.Fatalf("NewPending: %v", err)
	}
	if err := repo.CreatePending(context.Background(), ev); err != nil {
		t.Fatalf("CreatePending: %v", err)
	}

	found, err := repo.FindByPhotobookID(context.Background(), pid)
	if err != nil {
		t.Fatalf("FindByPhotobookID: %v", err)
	}
	if found.PhotobookID().String() != pid.String() {
		t.Errorf("photobook_id mismatch: %s vs %s", found.PhotobookID().String(), pid.String())
	}
	if !found.Status().IsPending() {
		t.Errorf("status=%s want pending", found.Status().String())
	}
	if found.Version().Int() != 1 {
		t.Errorf("version=%d want 1", found.Version().Int())
	}
}

func TestOgpRepository_FindByPhotobookID_NotFound(t *testing.T) {
	pool := dbPool(t)
	pid := seedDraftPhotobook(t, pool)

	repo := ogprdb.NewOgpRepository(pool)
	if _, err := repo.FindByPhotobookID(context.Background(), pid); !errors.Is(err, ogprdb.ErrNotFound) {
		t.Errorf("err mismatch: %v want ErrNotFound", err)
	}
}

func TestOgpRepository_MarkFailed(t *testing.T) {
	pool := dbPool(t)
	pid := seedDraftPhotobook(t, pool)
	now := time.Now().UTC().Truncate(time.Second)

	repo := ogprdb.NewOgpRepository(pool)
	ev, _ := domain.NewPending(domain.NewPendingParams{PhotobookID: pid, Now: now})
	if err := repo.CreatePending(context.Background(), ev); err != nil {
		t.Fatalf("CreatePending: %v", err)
	}

	failed := ev.MarkFailed(ogp_failure_reason.Sanitize(errors.New("render failed: no cover decoded")), now)
	if err := repo.MarkFailed(context.Background(), failed); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	found, err := repo.FindByPhotobookID(context.Background(), pid)
	if err != nil {
		t.Fatalf("FindByPhotobookID: %v", err)
	}
	if !found.Status().IsFailed() {
		t.Errorf("status=%s want failed", found.Status().String())
	}
	if found.FailedAt() == nil {
		t.Errorf("failed_at must be set")
	}
	if found.FailureReason().IsZero() {
		t.Errorf("failure_reason must be set")
	}
}

func TestOgpRepository_FailureReason200CharCheck(t *testing.T) {
	pool := dbPool(t)
	pid := seedDraftPhotobook(t, pool)
	now := time.Now().UTC().Truncate(time.Second)

	// VO で sanitize すれば 200 char 以下になるが、ここは生 SQL で 201 char を渡して
	// CHECK 制約が効くことを確認する。
	pidPg := pgtype.UUID{Bytes: pid.UUID(), Valid: true}
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	id := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO photobook_ogp_images (
			id, photobook_id, status, version,
			failed_at, failure_reason, created_at, updated_at
		) VALUES (
			$1, $2, 'failed', 1,
			$3, $4, $3, $3
		)
	`, pgtype.UUID{Bytes: id, Valid: true}, pidPg,
		pgtype.Timestamptz{Time: now, Valid: true}, string(long))
	if err == nil {
		t.Errorf("expected CHECK violation for 201 char failure_reason, got nil")
	}
}
