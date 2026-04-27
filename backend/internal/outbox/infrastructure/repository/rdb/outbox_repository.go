// Package rdb は Outbox events の RDB Repository。
//
// 設計参照:
//   - docs/plan/m2-outbox-plan.md §6 / §7
//   - docs/design/cross-cutting/outbox.md
//
// PR30 で公開する操作:
//   - Create: 新規 pending event を 1 行 INSERT
//
// **必ず TX-bound で呼ぶ**。pool 直で Create を呼ぶと「集約状態更新と同一 TX で保証」
// という Outbox pattern の不変条件が崩れる。NewOutboxRepository は pgx.Tx を受ける。
package rdb

import (
	"context"

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
// 集約状態更新と同一 TX で INSERT する用途のため、**呼び出し側は pgx.Tx を渡す**こと。
// pool を渡すと別 TX で動く（誤用、Outbox pattern が壊れる）。
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
