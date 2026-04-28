package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	moderationrdb "vrcpb/backend/internal/moderation/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// GetForOpsInput は GetPhotobookForOps の入力（id 優先、無ければ slug）。
type GetForOpsInput struct {
	PhotobookID *photobook_id.PhotobookID
	Slug        *slug.Slug
}

// GetForOpsOutput は cmd/ops show に返すスリムビュー（raw token / hash 系を含めない）。
type GetForOpsOutput struct {
	Photobook      photobookrdb.OpsView
	RecentActions  []moderationrdb.ActionSummary // 直近 ≤ 5 件
}

// GetPhotobookForOps は cmd/ops show の参照系 UseCase。
type GetPhotobookForOps struct {
	pool *pgxpool.Pool
}

// NewGetPhotobookForOps は UseCase を組み立てる。
func NewGetPhotobookForOps(pool *pgxpool.Pool) *GetPhotobookForOps {
	return &GetPhotobookForOps{pool: pool}
}

// Execute は単純 SELECT。本 UseCase は TX を張らない（参照のみ）。
func (u *GetPhotobookForOps) Execute(ctx context.Context, in GetForOpsInput) (GetForOpsOutput, error) {
	if in.PhotobookID == nil && in.Slug == nil {
		return GetForOpsOutput{}, fmt.Errorf("get for ops: either PhotobookID or Slug is required")
	}
	photobookRepo := photobookrdb.NewPhotobookRepository(u.pool)

	pid := in.PhotobookID
	if pid == nil {
		pb, err := photobookRepo.FindAnyBySlug(ctx, *in.Slug)
		if err != nil {
			if errors.Is(err, photobookrdb.ErrNotFound) {
				return GetForOpsOutput{}, ErrPhotobookNotFound
			}
			return GetForOpsOutput{}, fmt.Errorf("find by slug: %w", err)
		}
		id := pb.ID()
		pid = &id
	}
	view, err := photobookRepo.GetForOps(ctx, *pid)
	if err != nil {
		if errors.Is(err, photobookrdb.ErrNotFound) {
			return GetForOpsOutput{}, ErrPhotobookNotFound
		}
		return GetForOpsOutput{}, fmt.Errorf("get for ops: %w", err)
	}

	moderationRepo := moderationrdb.NewModerationActionRepository(u.pool)
	actions, err := moderationRepo.ListRecentByPhotobook(ctx, view.ID, 5)
	if err != nil {
		return GetForOpsOutput{}, fmt.Errorf("list recent moderation actions: %w", err)
	}
	return GetForOpsOutput{Photobook: view, RecentActions: actions}, nil
}
