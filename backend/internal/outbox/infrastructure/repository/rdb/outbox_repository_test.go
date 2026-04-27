// OutboxRepository.Create の実 DB 統合テスト。
//
// 実行方法:
//   docker compose -f backend/docker-compose.yaml up -d postgres
//   export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//   go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//   go -C backend test ./internal/outbox/...
package rdb_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	outboxdomain "vrcpb/backend/internal/outbox/domain"
	"vrcpb/backend/internal/outbox/domain/vo/aggregate_type"
	"vrcpb/backend/internal/outbox/domain/vo/event_type"
	outboxrdb "vrcpb/backend/internal/outbox/infrastructure/repository/rdb"
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

func TestOutboxRepositoryCreate(t *testing.T) {
	pool := dbPool(t)
	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)

	t.Run("正常_PhotobookPublished_INSERT", func(t *testing.T) {
		_, _ = pool.Exec(context.Background(), "TRUNCATE TABLE outbox_events")
		pid := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		ev, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Photobook(),
			AggregateID:   pid,
			EventType:     event_type.PhotobookPublished(),
			Payload: outboxdomain.PhotobookPublishedPayload{
				EventVersion: 1, OccurredAt: now,
				PhotobookID: pid.String(), Slug: "ab12cd34ef56gh78",
				Visibility: "unlisted", Type: "memory",
			},
			Now: now,
		})
		if err != nil {
			t.Fatalf("NewPendingEvent: %v", err)
		}
		// TX で実行（実運用も TX-bound のため）
		err = withTx(t, pool, func(tx pgx.Tx) error {
			return outboxrdb.NewOutboxRepository(tx).Create(context.Background(), ev)
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		var (
			gotStatus  string
			gotPayload []byte
			gotAttempt int
		)
		row := pool.QueryRow(context.Background(),
			"SELECT status, payload, attempts FROM outbox_events WHERE id = $1",
			pgtype.UUID{Bytes: ev.ID(), Valid: true})
		if err := row.Scan(&gotStatus, &gotPayload, &gotAttempt); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if gotStatus != "pending" {
			t.Errorf("status=%s want pending", gotStatus)
		}
		if gotAttempt != 0 {
			t.Errorf("attempts=%d want 0", gotAttempt)
		}
		// payload JSON が object として保存されている
		var parsed map[string]any
		if err := json.Unmarshal(gotPayload, &parsed); err != nil {
			t.Fatalf("payload unmarshal: %v", err)
		}
		if parsed["slug"] != "ab12cd34ef56gh78" {
			t.Errorf("slug mismatch: %v", parsed["slug"])
		}
	})

	t.Run("異常_event_type_未対応値はCHECK違反", func(t *testing.T) {
		_, _ = pool.Exec(context.Background(), "TRUNCATE TABLE outbox_events")
		// 直接 SQL で禁止 event_type を入れる（VO 経由では到達不能なので）
		_, err := pool.Exec(context.Background(), `
			INSERT INTO outbox_events
			    (id, aggregate_type, aggregate_id, event_type, payload,
			     status, available_at, attempts, created_at, updated_at)
			VALUES
			    ($1, 'photobook', $2, 'photobook.deleted', '{}'::jsonb,
			     'pending', now(), 0, now(), now())
		`,
			pgtype.UUID{Bytes: uuid.New(), Valid: true},
			pgtype.UUID{Bytes: uuid.New(), Valid: true})
		if err == nil {
			t.Errorf("CHECK violation expected for unknown event_type")
		}
	})

	t.Run("異常_payload_array_はCHECK違反", func(t *testing.T) {
		_, _ = pool.Exec(context.Background(), "TRUNCATE TABLE outbox_events")
		_, err := pool.Exec(context.Background(), `
			INSERT INTO outbox_events
			    (id, aggregate_type, aggregate_id, event_type, payload,
			     status, available_at, attempts, created_at, updated_at)
			VALUES
			    ($1, 'photobook', $2, 'photobook.published', '[]'::jsonb,
			     'pending', now(), 0, now(), now())
		`,
			pgtype.UUID{Bytes: uuid.New(), Valid: true},
			pgtype.UUID{Bytes: uuid.New(), Valid: true})
		if err == nil {
			t.Errorf("CHECK violation expected for non-object payload")
		}
	})

	t.Run("rollback_時にoutbox行が残らない", func(t *testing.T) {
		_, _ = pool.Exec(context.Background(), "TRUNCATE TABLE outbox_events")
		ctx := context.Background()
		pid := uuid.New()
		ev, _ := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Photobook(), AggregateID: pid,
			EventType: event_type.PhotobookPublished(),
			Payload: outboxdomain.PhotobookPublishedPayload{
				EventVersion: 1, OccurredAt: now, PhotobookID: pid.String(),
				Slug: "rb12cd34ef56gh78", Visibility: "unlisted", Type: "memory",
			},
			Now: now,
		})
		// 意図的にエラーを inject して rollback する
		injectErr := errors.New("intentional")
		err := withTx(t, pool, func(tx pgx.Tx) error {
			if err := outboxrdb.NewOutboxRepository(tx).Create(ctx, ev); err != nil {
				return err
			}
			return injectErr
		})
		if !errors.Is(err, injectErr) {
			t.Fatalf("expected injectErr, got %v", err)
		}
		// rollback されたので 0 行
		var count int
		row := pool.QueryRow(ctx, "SELECT count(*)::int FROM outbox_events WHERE id = $1",
			pgtype.UUID{Bytes: ev.ID(), Valid: true})
		if err := row.Scan(&count); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 rows after rollback, got %d", count)
		}
	})
}

// withTx は test 内で TX を 1 つ開始して fn を実行する小さい helper。
func withTx(t *testing.T, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(context.Background())
}

// 静的アサーション: 共通 import の sql package を使わなくてもよい
var _ = sql.ErrNoRows
