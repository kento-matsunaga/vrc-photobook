package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	outboxdomain "vrcpb/backend/internal/outbox/domain"
	"vrcpb/backend/internal/outbox/domain/vo/aggregate_type"
	"vrcpb/backend/internal/outbox/domain/vo/event_type"
	outboxrdb "vrcpb/backend/internal/outbox/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// ErrPublishConflict は publish 楽観ロック失敗（version 不一致 / status≠draft）。
var ErrPublishConflict = errors.New("publish conflict (version mismatch or not in draft state)")

// PublishFromDraftInput は publish の入力。
type PublishFromDraftInput struct {
	PhotobookID     photobook_id.PhotobookID
	ExpectedVersion int
	Now             time.Time
}

// PublishFromDraftOutput は publish 結果。RawManageToken はログ禁止。
type PublishFromDraftOutput struct {
	Photobook       domain.Photobook
	RawManageToken  manage_url_token.ManageUrlToken
}

// PublishFromDraft は draft → published を実行する UseCase。
//
// **同一 TX 内で**:
//  1. photobook を取得・状態検証
//  2. public_url_slug 生成
//  3. ManageUrlToken / ManageUrlTokenHash 生成
//  4. photobookRepo.PublishFromDraft（status='draft' AND version=$expected で UPDATE）
//  5. session の RevokeAllDrafts（同 tx）
//
// session revoke 失敗時は photobook 側 UPDATE もロールバックされる（I-D7 / I-S9 整合）。
//
// Outbox INSERT は本 PR では行わない（Outbox table は後続 PR、計画 §7.1）。
type PublishFromDraft struct {
	pool                 *pgxpool.Pool
	photobookRepoFactory PhotobookTxRepositoryFactory
	revokerFactory       DraftSessionRevokerFactory
	slugGen              SlugGenerator
}

// NewPublishFromDraft は UseCase を組み立てる。
func NewPublishFromDraft(
	pool *pgxpool.Pool,
	photobookRepoFactory PhotobookTxRepositoryFactory,
	revokerFactory DraftSessionRevokerFactory,
	slugGen SlugGenerator,
) *PublishFromDraft {
	return &PublishFromDraft{
		pool:                 pool,
		photobookRepoFactory: photobookRepoFactory,
		revokerFactory:       revokerFactory,
		slugGen:              slugGen,
	}
}

// Execute は WithTx 内で publish + RevokeAllDrafts を原子的に実行する。
func (u *PublishFromDraft) Execute(
	ctx context.Context,
	in PublishFromDraftInput,
) (PublishFromDraftOutput, error) {
	publicSlug, err := u.slugGen.Generate(ctx)
	if err != nil {
		return PublishFromDraftOutput{}, fmt.Errorf("slug gen: %w", err)
	}
	rawManage, err := manage_url_token.Generate()
	if err != nil {
		return PublishFromDraftOutput{}, fmt.Errorf("manage token gen: %w", err)
	}
	manageHash := manage_url_token_hash.Of(rawManage)

	var resultPB domain.Photobook
	err = database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		repo := u.photobookRepoFactory(tx)
		revoker := u.revokerFactory(tx)

		pb, err := repo.FindByID(ctx, in.PhotobookID)
		if err != nil {
			if errors.Is(err, rdb.ErrNotFound) {
				return ErrPublishConflict
			}
			return err
		}
		if err := pb.CanPublish(); err != nil {
			return err
		}
		if pb.Version() != in.ExpectedVersion {
			return ErrPublishConflict
		}
		// domain 側の状態遷移（DB UPDATE は次の repo 呼び出し）
		next, err := pb.Publish(publicSlug, manageHash, in.Now)
		if err != nil {
			return err
		}
		if err := repo.PublishFromDraft(ctx, pb.ID(), publicSlug, manageHash, in.Now, in.ExpectedVersion); err != nil {
			if errors.Is(err, rdb.ErrOptimisticLockConflict) {
				return ErrPublishConflict
			}
			return err
		}
		if _, err := revoker.RevokeAllDrafts(ctx, pb.ID()); err != nil {
			return fmt.Errorf("revoke all drafts: %w", err)
		}
		// PR30: PhotobookPublished event を同一 TX で Outbox に INSERT。
		// payload は worker（PR31）が DB 再 fetch できる最小値のみ含める。
		// raw manage token / draft token / Cookie / presigned URL は入れない。
		var coverIDStr *string
		if cid := next.CoverImageID(); cid != nil {
			s := cid.String()
			coverIDStr = &s
		}
		ev, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Photobook(),
			AggregateID:   next.ID().UUID(),
			EventType:     event_type.PhotobookPublished(),
			Payload: outboxdomain.PhotobookPublishedPayload{
				EventVersion: outboxdomain.EventVersion,
				OccurredAt:   in.Now.UTC(),
				PhotobookID:  next.ID().String(),
				Slug:         publicSlug.String(),
				Visibility:   next.Visibility().String(),
				Type:         next.Type().String(),
				CoverImageID: coverIDStr,
			},
			Now: in.Now.UTC(),
		})
		if err != nil {
			return fmt.Errorf("build photobook.published event: %w", err)
		}
		if err := outboxrdb.NewOutboxRepository(tx).Create(ctx, ev); err != nil {
			return fmt.Errorf("outbox create photobook.published: %w", err)
		}
		resultPB = next
		return nil
	})
	if err != nil {
		return PublishFromDraftOutput{}, err
	}
	return PublishFromDraftOutput{Photobook: resultPB, RawManageToken: rawManage}, nil
}
