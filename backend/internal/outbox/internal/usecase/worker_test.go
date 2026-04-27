// Worker の実 DB 統合テスト。
//
// 実行方法:
//   docker compose -f backend/docker-compose.yaml up -d postgres
//   export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//   go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//   go -C backend test ./internal/outbox/internal/usecase/...
package usecase_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	outboxdomain "vrcpb/backend/internal/outbox/domain"
	"vrcpb/backend/internal/outbox/domain/vo/aggregate_type"
	"vrcpb/backend/internal/outbox/domain/vo/event_type"
	outboxrdb "vrcpb/backend/internal/outbox/infrastructure/repository/rdb"
	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
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
	if _, err := pool.Exec(context.Background(), "TRUNCATE TABLE outbox_events"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// recordingHandler は呼び出された event を記録するテスト用 handler。
type recordingHandler struct {
	mu     sync.Mutex
	events []outboxusecase.EventTarget
	err    error
}

func (h *recordingHandler) Handle(_ context.Context, ev outboxusecase.EventTarget) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, ev)
	return h.err
}

// seedPending は与えた event_type の pending row を 1 件 INSERT する（available_at = now）。
func seedPending(t *testing.T, pool *pgxpool.Pool, et event_type.EventType, now time.Time) uuid.UUID {
	t.Helper()
	pid := uuid.New()
	ev, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
		AggregateType: aggregate_type.Photobook(),
		AggregateID:   pid,
		EventType:     et,
		Payload: outboxdomain.PhotobookPublishedPayload{
			EventVersion: 1, OccurredAt: now,
			PhotobookID: pid.String(), Slug: "abcd1234efgh5678",
			Visibility: "unlisted", Type: "memory",
		},
		Now:         now,
		AvailableAt: now,
	})
	if err != nil {
		t.Fatalf("NewPendingEvent: %v", err)
	}
	repo := outboxrdb.NewOutboxRepository(pool)
	if err := repo.Create(context.Background(), ev); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return ev.ID()
}

func TestWorkerRunProcessesPendingEvents(t *testing.T) {
	pool := dbPool(t)
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	id := seedPending(t, pool, event_type.PhotobookPublished(), now.Add(-time.Minute))

	registry := outboxusecase.NewHandlerRegistry()
	rec := &recordingHandler{}
	registry.Register(event_type.PhotobookPublished().String(), rec)

	w := outboxusecase.NewWorker(pool, registry, outboxusecase.WorkerConfig{
		WorkerID: "test-worker-1",
	}, discardLogger())

	out, err := w.Run(context.Background(), outboxusecase.RunInput{MaxEvents: 5, Now: now})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Picked != 1 || out.Processed != 1 {
		t.Fatalf("counts: picked=%d processed=%d (want 1/1)", out.Picked, out.Processed)
	}
	if len(rec.events) != 1 || rec.events[0].ID != id {
		t.Fatalf("handler not called with expected event: %+v", rec.events)
	}

	// status='processed' を確認
	row, err := outboxrdb.NewOutboxRepository(pool).FindByID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if row.Status != "processed" {
		t.Errorf("status=%s want processed", row.Status)
	}
	if row.ProcessedAt == nil {
		t.Errorf("processed_at must be set")
	}
	if row.LockedAt != nil || row.LockedBy != nil {
		t.Errorf("locked_at/by should be cleared after processed: %+v / %v", row.LockedAt, row.LockedBy)
	}
}

func TestWorkerRunFailedRetry(t *testing.T) {
	pool := dbPool(t)
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	id := seedPending(t, pool, event_type.ImageBecameAvailable(), now.Add(-time.Minute))

	registry := outboxusecase.NewHandlerRegistry()
	registry.Register(event_type.ImageBecameAvailable().String(), &recordingHandler{err: errors.New("transient failure")})

	w := outboxusecase.NewWorker(pool, registry, outboxusecase.WorkerConfig{
		WorkerID:    "test-worker-2",
		MaxAttempts: 3,
		Backoff:     time.Minute,
		MaxBackoff:  time.Hour,
	}, discardLogger())

	out, err := w.Run(context.Background(), outboxusecase.RunInput{MaxEvents: 5, Now: now})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Picked != 1 || out.FailedRetry != 1 {
		t.Fatalf("counts: picked=%d failed_retry=%d", out.Picked, out.FailedRetry)
	}

	row, err := outboxrdb.NewOutboxRepository(pool).FindByID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if row.Status != "failed" {
		t.Errorf("status=%s want failed", row.Status)
	}
	if row.Attempts != 1 {
		t.Errorf("attempts=%d want 1", row.Attempts)
	}
	if row.LastError == nil || *row.LastError == "" {
		t.Errorf("last_error must be set: %v", row.LastError)
	}
	if !row.AvailableAt.After(now) {
		t.Errorf("available_at should be pushed back, got %v (now %v)", row.AvailableAt, now)
	}
}

func TestWorkerRunDeadAfterMaxAttempts(t *testing.T) {
	pool := dbPool(t)
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	id := seedPending(t, pool, event_type.ImageFailed(), now.Add(-time.Minute))

	// attempts=2 に手動更新（次回失敗で attempts=3 となり MaxAttempts=3 で dead）
	if _, err := pool.Exec(context.Background(),
		"UPDATE outbox_events SET attempts = 2 WHERE id = $1",
		pgtype.UUID{Bytes: id, Valid: true}); err != nil {
		t.Fatalf("update attempts: %v", err)
	}

	registry := outboxusecase.NewHandlerRegistry()
	registry.Register(event_type.ImageFailed().String(), &recordingHandler{err: errors.New("permanent failure")})

	w := outboxusecase.NewWorker(pool, registry, outboxusecase.WorkerConfig{
		WorkerID:    "test-worker-3",
		MaxAttempts: 3,
		Backoff:     time.Minute,
		MaxBackoff:  time.Hour,
	}, discardLogger())

	out, err := w.Run(context.Background(), outboxusecase.RunInput{MaxEvents: 1, Now: now})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Dead != 1 {
		t.Fatalf("dead=%d want 1; out=%+v", out.Dead, out)
	}

	row, _ := outboxrdb.NewOutboxRepository(pool).FindByID(context.Background(), id)
	if row.Status != "dead" {
		t.Errorf("status=%s want dead", row.Status)
	}
	if row.Attempts != 3 {
		t.Errorf("attempts=%d want 3", row.Attempts)
	}
}

func TestWorkerRunUnknownEventTypeFallsBackToFailedRetry(t *testing.T) {
	pool := dbPool(t)
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	id := seedPending(t, pool, event_type.PhotobookPublished(), now.Add(-time.Minute))

	// registry に handler を登録しない
	registry := outboxusecase.NewHandlerRegistry()

	w := outboxusecase.NewWorker(pool, registry, outboxusecase.WorkerConfig{
		WorkerID:    "test-worker-4",
		MaxAttempts: 5,
		Backoff:     time.Minute,
	}, discardLogger())

	out, err := w.Run(context.Background(), outboxusecase.RunInput{MaxEvents: 1, Now: now})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Skipped != 1 {
		t.Errorf("skipped=%d want 1; out=%+v", out.Skipped, out)
	}

	row, _ := outboxrdb.NewOutboxRepository(pool).FindByID(context.Background(), id)
	if row.Status != "failed" {
		t.Errorf("status=%s want failed", row.Status)
	}
}

func TestWorkerRunDryRunDoesNotChangeStatus(t *testing.T) {
	pool := dbPool(t)
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	id := seedPending(t, pool, event_type.PhotobookPublished(), now.Add(-time.Minute))

	rec := &recordingHandler{}
	registry := outboxusecase.NewHandlerRegistry()
	registry.Register(event_type.PhotobookPublished().String(), rec)

	w := outboxusecase.NewWorker(pool, registry, outboxusecase.WorkerConfig{WorkerID: "dry"}, discardLogger())
	out, err := w.Run(context.Background(), outboxusecase.RunInput{MaxEvents: 1, Now: now, DryRun: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Picked != 1 || out.Processed != 0 {
		t.Errorf("dry-run counts: %+v", out)
	}
	if len(rec.events) != 0 {
		t.Errorf("handler must not be called in dry-run")
	}

	row, _ := outboxrdb.NewOutboxRepository(pool).FindByID(context.Background(), id)
	if row.Status != "pending" {
		t.Errorf("status=%s want pending (dry-run)", row.Status)
	}
}

func TestWorkerRunSkipsEventsBeforeAvailableAt(t *testing.T) {
	pool := dbPool(t)
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	// available_at を未来に設定
	id := seedPending(t, pool, event_type.PhotobookPublished(), now.Add(time.Hour))

	rec := &recordingHandler{}
	registry := outboxusecase.NewHandlerRegistry()
	registry.Register(event_type.PhotobookPublished().String(), rec)

	w := outboxusecase.NewWorker(pool, registry, outboxusecase.WorkerConfig{WorkerID: "future"}, discardLogger())
	out, err := w.Run(context.Background(), outboxusecase.RunInput{MaxEvents: 5, Now: now})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Picked != 0 {
		t.Errorf("picked=%d want 0 (event is in the future)", out.Picked)
	}
	row, _ := outboxrdb.NewOutboxRepository(pool).FindByID(context.Background(), id)
	if row.Status != "pending" {
		t.Errorf("status=%s want pending", row.Status)
	}
}

func TestWorkerConcurrentClaimsDoNotDouble(t *testing.T) {
	pool := dbPool(t)
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	// 1 件だけ pending を入れて並列 worker 2 つで pickup させる
	id := seedPending(t, pool, event_type.ImageBecameAvailable(), now.Add(-time.Minute))

	registry1 := outboxusecase.NewHandlerRegistry()
	rec1 := &recordingHandler{}
	registry1.Register(event_type.ImageBecameAvailable().String(), rec1)

	registry2 := outboxusecase.NewHandlerRegistry()
	rec2 := &recordingHandler{}
	registry2.Register(event_type.ImageBecameAvailable().String(), rec2)

	w1 := outboxusecase.NewWorker(pool, registry1, outboxusecase.WorkerConfig{WorkerID: "worker-A"}, discardLogger())
	w2 := outboxusecase.NewWorker(pool, registry2, outboxusecase.WorkerConfig{WorkerID: "worker-B"}, discardLogger())

	var wg sync.WaitGroup
	wg.Add(2)
	results := make([]outboxusecase.RunOutput, 2)

	go func() {
		defer wg.Done()
		out, _ := w1.Run(context.Background(), outboxusecase.RunInput{MaxEvents: 5, Now: now})
		results[0] = out
	}()
	go func() {
		defer wg.Done()
		out, _ := w2.Run(context.Background(), outboxusecase.RunInput{MaxEvents: 5, Now: now})
		results[1] = out
	}()
	wg.Wait()

	totalPicked := results[0].Picked + results[1].Picked
	if totalPicked != 1 {
		t.Errorf("total picked=%d want 1 (FOR UPDATE SKIP LOCKED should prevent double claim)", totalPicked)
	}

	row, _ := outboxrdb.NewOutboxRepository(pool).FindByID(context.Background(), id)
	if row.Status != "processed" {
		t.Errorf("status=%s want processed", row.Status)
	}
}

func TestReleaseStaleLocks(t *testing.T) {
	pool := dbPool(t)
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	id := seedPending(t, pool, event_type.PhotobookPublished(), now.Add(-time.Hour))

	// 直接 SQL で processing 化（locked_at = 1 時間前）
	if _, err := pool.Exec(context.Background(), `
		UPDATE outbox_events
		SET status='processing', locked_at=$2, locked_by='dead-worker', updated_at=$2
		WHERE id=$1`,
		pgtype.UUID{Bytes: id, Valid: true},
		pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
	); err != nil {
		t.Fatalf("seed processing: %v", err)
	}

	released, err := outboxusecase.ReleaseStaleLocks(context.Background(), pool, now, 30*time.Minute, discardLogger())
	if err != nil {
		t.Fatalf("ReleaseStaleLocks: %v", err)
	}
	if released != 1 {
		t.Errorf("released=%d want 1", released)
	}

	row, _ := outboxrdb.NewOutboxRepository(pool).FindByID(context.Background(), id)
	if row.Status != "pending" {
		t.Errorf("status=%s want pending after release", row.Status)
	}
	if row.LockedAt != nil || row.LockedBy != nil {
		t.Errorf("locks should be cleared, got %v / %v", row.LockedAt, row.LockedBy)
	}
}
