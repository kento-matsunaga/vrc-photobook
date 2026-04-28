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

// HideInput は HidePhotobookByOperator の入力。
type HideInput struct {
	PhotobookID photobook_id.PhotobookID
	ActorLabel  operator_label.OperatorLabel
	Reason      action_reason.ActionReason
	Detail      action_detail.ActionDetail
	Now         time.Time
}

// HideOutput は CLI / caller への戻り。raw token / hash 系は返さない。
type HideOutput struct {
	ActionID    action_id.ActionID
	PhotobookID photobook_id.PhotobookID
	HiddenAt    time.Time
}

// HidePhotobookByOperator は status='published' な photobook を hidden_by_operator=true
// に変更する。**同一 TX 4 要素**:
//
//	1. SELECT photobooks FOR UPDATE（FOR UPDATE は SetHiddenByOperator UPDATE に内在）
//	2. UPDATE photobooks SET hidden_by_operator=true（status='published' AND hidden=false）
//	3. INSERT moderation_actions（kind='hide'、append-only）
//	4. INSERT outbox_events（event_type='photobook.hidden'、handler は no-op）
//
// 失敗時は全体 rollback。version は上げない（編集 OCC を壊さない、計画書 §5.6）。
type HidePhotobookByOperator struct {
	pool *pgxpool.Pool
}

// NewHidePhotobookByOperator は UseCase を組み立てる。
func NewHidePhotobookByOperator(pool *pgxpool.Pool) *HidePhotobookByOperator {
	return &HidePhotobookByOperator{pool: pool}
}

// Execute は同一 TX で hide 操作を完遂する。
func (u *HidePhotobookByOperator) Execute(ctx context.Context, in HideInput) (HideOutput, error) {
	aid, err := action_id.New()
	if err != nil {
		return HideOutput{}, fmt.Errorf("action_id gen: %w", err)
	}
	var out HideOutput
	err = database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		photobookRepo := photobookrdb.NewPhotobookRepository(tx)
		moderationRepo := moderationrdb.NewModerationActionRepository(tx)
		outboxRepo := outboxrdb.NewOutboxRepository(tx)

		// 1) 現状確認
		view, err := photobookRepo.GetForOps(ctx, in.PhotobookID)
		if err != nil {
			if errors.Is(err, photobookrdb.ErrNotFound) {
				return ErrPhotobookNotFound
			}
			return fmt.Errorf("get for ops: %w", err)
		}
		if view.Status != "published" {
			return ErrInvalidStatusForHide
		}
		if view.HiddenByOperator {
			return ErrAlreadyHidden
		}

		// 2) UPDATE photobooks（hidden=false → true）
		updated, err := photobookRepo.SetHiddenByOperator(ctx, in.PhotobookID, true, false, in.Now)
		if err != nil {
			return fmt.Errorf("set hidden: %w", err)
		}
		if !updated {
			// 並行 hide / status 変動で 0 行になった場合
			return ErrAlreadyHidden
		}

		// 3) moderation_actions append
		ma, err := entity.New(entity.NewParams{
			ID:         aid,
			Kind:       action_kind.Hide(),
			TargetID:   in.PhotobookID,
			ActorLabel: in.ActorLabel,
			Reason:     in.Reason,
			Detail:     in.Detail,
			ExecutedAt: in.Now,
		})
		if err != nil {
			return fmt.Errorf("build moderation action: %w", err)
		}
		if err := moderationRepo.Insert(ctx, ma); err != nil {
			return fmt.Errorf("insert moderation action: %w", err)
		}

		// 4) Outbox INSERT (PhotobookHidden, no-op handler)
		ev, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Photobook(),
			AggregateID:   in.PhotobookID.UUID(),
			EventType:     event_type.PhotobookHidden(),
			Payload: outboxdomain.PhotobookHiddenPayload{
				EventVersion: outboxdomain.EventVersion,
				OccurredAt:   in.Now.UTC(),
				PhotobookID:  in.PhotobookID.String(),
				ActionID:     aid.String(),
				Reason:       in.Reason.String(),
				ActorLabel:   in.ActorLabel.String(),
			},
			Now: in.Now.UTC(),
		})
		if err != nil {
			return fmt.Errorf("build photobook.hidden event: %w", err)
		}
		if err := outboxRepo.Create(ctx, ev); err != nil {
			return fmt.Errorf("outbox create photobook.hidden: %w", err)
		}

		out = HideOutput{ActionID: aid, PhotobookID: in.PhotobookID, HiddenAt: in.Now}
		return nil
	})
	if err != nil {
		return HideOutput{}, err
	}
	return out, nil
}
