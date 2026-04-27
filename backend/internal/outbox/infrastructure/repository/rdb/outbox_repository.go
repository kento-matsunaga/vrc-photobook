// Package rdb は Outbox events の RDB Repository。
//
// 設計参照:
//   - docs/plan/m2-outbox-plan.md §6 / §7
//   - docs/design/cross-cutting/outbox.md
//
// 公開する操作:
//   - Create: 新規 pending event を 1 行 INSERT（producer 側、TX-bound 必須）
//   - ListPendingForUpdate / MarkProcessingByIDs: worker の claim TX 用
//   - MarkProcessed / MarkFailedRetry / MarkDead: worker の handler 後 TX 用
//   - ReleaseStaleLocks: stuck row の救出
//   - FindByID: test / inspector
//
// Create は **必ず TX-bound で呼ぶ**（集約状態更新と同 TX）。worker 系は claim TX と
// handler 後 TX を別々に張る運用なので、Repository は pgx.Tx でも pool でも受け取れる
// （sqlcgen.DBTX）。
package rdb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/outbox/domain"
	"vrcpb/backend/internal/outbox/infrastructure/repository/rdb/sqlcgen"
)

// OutboxRepository は outbox_events への永続化を提供する。
type OutboxRepository struct {
	q *sqlcgen.Queries
}

// NewOutboxRepository は pgx pool / Tx (sqlcgen.DBTX を満たすもの) から Repository を作る。
//
// 集約状態更新と同一 TX で INSERT する用途では **呼び出し側は pgx.Tx を渡す**こと。
// worker 系は claim TX / handler 後 TX をそれぞれ Repository を作り直して使う。
func NewOutboxRepository(db sqlcgen.DBTX) *OutboxRepository {
	return &OutboxRepository{q: sqlcgen.New(db)}
}

// Create は新規 pending event を 1 行 INSERT する。
//
// status='pending' / available_at=event.AvailableAt() / attempts=0 を default 設定。
// Failure (e.g. CHECK 違反、payload 形式不正) は呼び出し側 TX を rollback させる。
func (r *OutboxRepository) Create(ctx context.Context, ev domain.Event) error {
	return r.q.CreateOutboxEvent(ctx, sqlcgen.CreateOutboxEventParams{
		ID:            pgtype.UUID{Bytes: ev.ID(), Valid: true},
		AggregateType: ev.AggregateType().String(),
		AggregateID:   pgtype.UUID{Bytes: ev.AggregateID(), Valid: true},
		EventType:     ev.EventType().String(),
		Payload:       ev.PayloadJSON(),
		AvailableAt:   pgtype.Timestamptz{Time: ev.AvailableAt(), Valid: true},
		CreatedAt:     pgtype.Timestamptz{Time: ev.CreatedAt(), Valid: true},
	})
}

// PendingEventRow は claim 後に worker が扱う最小プロジェクション。
//
// domain.Event は producer 用（payload struct を持つ）で、worker は jsonb の生バイトと
// 識別子だけ扱えば良いため、worker 専用の struct として用意する。
type PendingEventRow struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       []byte
	Status        string
	AvailableAt   time.Time
	Attempts      int
	LastError     *string
	LockedAt      *time.Time
	LockedBy      *string
	ProcessedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ListPendingForUpdate は claim TX 内で pickup 候補を FOR UPDATE SKIP LOCKED で取り出す。
//
// 呼び出し側は **必ず pgx.Tx で本 Repository を作る**こと（claim TX で他 worker の
// 行を skip するために行 lock が必要）。
func (r *OutboxRepository) ListPendingForUpdate(ctx context.Context, now time.Time, limit int) ([]PendingEventRow, error) {
	rows, err := r.q.ListPendingOutboxEventsForUpdate(ctx, sqlcgen.ListPendingOutboxEventsForUpdateParams{
		AvailableAt: pgtype.Timestamptz{Time: now, Valid: true},
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]PendingEventRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPendingEventRow(row))
	}
	return out, nil
}

// MarkProcessingByIDs は claim TX で複数行を processing に遷移させる。
//
// claim TX 内で ListPendingForUpdate と組で呼ばれ、commit すると lock が解放されるが、
// status='processing' が論理 lock として機能する（pickup query は pending/failed のみ）。
func (r *OutboxRepository) MarkProcessingByIDs(ctx context.Context, ids []uuid.UUID, lockedAt time.Time, lockedBy string) error {
	pgIDs := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		pgIDs = append(pgIDs, pgtype.UUID{Bytes: id, Valid: true})
	}
	by := lockedBy
	return r.q.MarkOutboxProcessingByIDs(ctx, sqlcgen.MarkOutboxProcessingByIDsParams{
		Column1:  pgIDs,
		LockedAt: pgtype.Timestamptz{Time: lockedAt, Valid: true},
		LockedBy: &by,
	})
}

// MarkProcessed は handler 成功後に呼ぶ（pool 直で十分、単行更新）。
func (r *OutboxRepository) MarkProcessed(ctx context.Context, id uuid.UUID, processedAt time.Time) error {
	return r.q.MarkOutboxProcessed(ctx, sqlcgen.MarkOutboxProcessedParams{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		ProcessedAt: pgtype.Timestamptz{Time: processedAt, Valid: true},
	})
}

// MarkFailedRetry は handler 失敗後 + retry 余地あり。available_at は呼び出し側が
// backoff 計算した値を渡す。lastError は sanitize 済（200 char 以内）。
func (r *OutboxRepository) MarkFailedRetry(ctx context.Context, id uuid.UUID, lastError string, availableAt, updatedAt time.Time) error {
	le := lastError
	return r.q.MarkOutboxFailedRetry(ctx, sqlcgen.MarkOutboxFailedRetryParams{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		LastError:   &le,
		AvailableAt: pgtype.Timestamptz{Time: availableAt, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: updatedAt, Valid: true},
	})
}

// MarkDead は retry 上限到達 or 致命的エラー時の最終遷移。
func (r *OutboxRepository) MarkDead(ctx context.Context, id uuid.UUID, lastError string, updatedAt time.Time) error {
	le := lastError
	return r.q.MarkOutboxDead(ctx, sqlcgen.MarkOutboxDeadParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		LastError: &le,
		UpdatedAt: pgtype.Timestamptz{Time: updatedAt, Valid: true},
	})
}

// ReleaseStaleLocks は processing のまま残った行（locked_at < threshold）を pending に戻す。
// 戻り値は影響行数。
func (r *OutboxRepository) ReleaseStaleLocks(ctx context.Context, threshold, now time.Time) (int64, error) {
	return r.q.ReleaseStaleOutboxLocks(ctx, sqlcgen.ReleaseStaleOutboxLocksParams{
		LockedAt:  pgtype.Timestamptz{Time: threshold, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
}

// FindByID は test / inspector 用に 1 行を取り出す。
func (r *OutboxRepository) FindByID(ctx context.Context, id uuid.UUID) (PendingEventRow, error) {
	row, err := r.q.FindOutboxEventByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return PendingEventRow{}, err
	}
	return toPendingEventRow(row), nil
}

// toPendingEventRow は sqlcgen 表現を application 表現に変換する。
func toPendingEventRow(row sqlcgen.OutboxEvent) PendingEventRow {
	out := PendingEventRow{
		ID:            row.ID.Bytes,
		AggregateType: row.AggregateType,
		AggregateID:   row.AggregateID.Bytes,
		EventType:     row.EventType,
		Payload:       row.Payload,
		Status:        row.Status,
		Attempts:      int(row.Attempts),
		LastError:     row.LastError,
		LockedBy:      row.LockedBy,
	}
	if row.AvailableAt.Valid {
		out.AvailableAt = row.AvailableAt.Time
	}
	if row.LockedAt.Valid {
		t := row.LockedAt.Time
		out.LockedAt = &t
	}
	if row.ProcessedAt.Valid {
		t := row.ProcessedAt.Time
		out.ProcessedAt = &t
	}
	if row.CreatedAt.Valid {
		out.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		out.UpdatedAt = row.UpdatedAt.Time
	}
	return out
}
