package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/moderation/domain/entity"
	"vrcpb/backend/internal/moderation/domain/vo/action_detail"
	"vrcpb/backend/internal/moderation/domain/vo/action_id"
	"vrcpb/backend/internal/moderation/domain/vo/action_kind"
	"vrcpb/backend/internal/moderation/domain/vo/action_reason"
	"vrcpb/backend/internal/moderation/domain/vo/operator_label"
	moderationrdb "vrcpb/backend/internal/moderation/infrastructure/repository/rdb"
	outboxdomain "vrcpb/backend/internal/outbox/domain"
	"vrcpb/backend/internal/outbox/domain/vo/aggregate_type"
	"vrcpb/backend/internal/outbox/domain/vo/event_type"
	outboxrdb "vrcpb/backend/internal/outbox/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// UnhideInput は UnhidePhotobookByOperator の入力。
type UnhideInput struct {
	PhotobookID   photobook_id.PhotobookID
	ActorLabel    operator_label.OperatorLabel
	Reason        action_reason.ActionReason
	Detail        action_detail.ActionDetail
	CorrelationID *action_id.ActionID // 任意（直前の hide action id）
	Now           time.Time
}

// UnhideOutput は CLI / caller への戻り。
type UnhideOutput struct {
	ActionID    action_id.ActionID
	PhotobookID photobook_id.PhotobookID
	UnhiddenAt  time.Time
}

// UnhidePhotobookByOperator は status='published' AND hidden_by_operator=true な
// photobook を hidden=false に戻す。**同一 TX 4 要素**:
//
//	1. SELECT photobooks（現状確認）
//	2. UPDATE photobooks SET hidden=false（hidden=true 前提で WHERE）
//	3. INSERT moderation_actions（kind='unhide'）
//	4. INSERT outbox_events（event_type='photobook.unhidden'、no-op handler）
type UnhidePhotobookByOperator struct {
	pool *pgxpool.Pool
}

// NewUnhidePhotobookByOperator は UseCase を組み立てる。
func NewUnhidePhotobookByOperator(pool *pgxpool.Pool) *UnhidePhotobookByOperator {
	return &UnhidePhotobookByOperator{pool: pool}
}

// Execute は同一 TX で unhide 操作を完遂する。
func (u *UnhidePhotobookByOperator) Execute(ctx context.Context, in UnhideInput) (UnhideOutput, error) {
	aid, err := action_id.New()
	if err != nil {
		return UnhideOutput{}, fmt.Errorf("action_id gen: %w", err)
	}
	var out UnhideOutput
	err = database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		photobookRepo := photobookrdb.NewPhotobookRepository(tx)
		moderationRepo := moderationrdb.NewModerationActionRepository(tx)
		outboxRepo := outboxrdb.NewOutboxRepository(tx)

		view, err := photobookRepo.GetForOps(ctx, in.PhotobookID)
		if err != nil {
			if errors.Is(err, photobookrdb.ErrNotFound) {
				return ErrPhotobookNotFound
			}
			return fmt.Errorf("get for ops: %w", err)
		}
		if view.Status != "published" {
			// hide / unhide とも published のみ受け付ける（計画書 §13 #4）
			return ErrInvalidStatusForHide
		}
		if !view.HiddenByOperator {
			return ErrAlreadyUnhidden
		}

		updated, err := photobookRepo.SetHiddenByOperator(ctx, in.PhotobookID, false, true, in.Now)
		if err != nil {
			return fmt.Errorf("set hidden=false: %w", err)
		}
		if !updated {
			return ErrAlreadyUnhidden
		}

		ma, err := entity.New(entity.NewParams{
			ID:            aid,
			Kind:          action_kind.Unhide(),
			TargetID:      in.PhotobookID,
			ActorLabel:    in.ActorLabel,
			Reason:        in.Reason,
			Detail:        in.Detail,
			CorrelationID: in.CorrelationID,
			ExecutedAt:    in.Now,
		})
		if err != nil {
			return fmt.Errorf("build moderation action: %w", err)
		}
		if err := moderationRepo.Insert(ctx, ma); err != nil {
			return fmt.Errorf("insert moderation action: %w", err)
		}

		var corrStr *string
		if in.CorrelationID != nil {
			s := in.CorrelationID.String()
			corrStr = &s
		}
		ev, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Photobook(),
			AggregateID:   in.PhotobookID.UUID(),
			EventType:     event_type.PhotobookUnhidden(),
			Payload: outboxdomain.PhotobookUnhiddenPayload{
				EventVersion:  outboxdomain.EventVersion,
				OccurredAt:    in.Now.UTC(),
				PhotobookID:   in.PhotobookID.String(),
				ActionID:      aid.String(),
				Reason:        in.Reason.String(),
				ActorLabel:    in.ActorLabel.String(),
				CorrelationID: corrStr,
			},
			Now: in.Now.UTC(),
		})
		if err != nil {
			return fmt.Errorf("build photobook.unhidden event: %w", err)
		}
		if err := outboxRepo.Create(ctx, ev); err != nil {
			return fmt.Errorf("outbox create photobook.unhidden: %w", err)
		}

		out = UnhideOutput{ActionID: aid, PhotobookID: in.PhotobookID, UnhiddenAt: in.Now}
		return nil
	})
	if err != nil {
		return UnhideOutput{}, err
	}
	return out, nil
}
