package usecase

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// ListHiddenInput は ListHiddenPhotobooks の入力。
type ListHiddenInput struct {
	Limit  int32 // 0 / 負値は呼び出し側で 20 にデフォルト
	Offset int32
}

// ListHiddenOutput は cmd/ops list-hidden の戻り。
type ListHiddenOutput struct {
	Items []photobookrdb.OpsHiddenSummary
}

// ListHiddenPhotobooks は hidden_by_operator=true な photobook を最新更新順で返す。
type ListHiddenPhotobooks struct {
	pool *pgxpool.Pool
}

// NewListHiddenPhotobooks は UseCase を組み立てる。
func NewListHiddenPhotobooks(pool *pgxpool.Pool) *ListHiddenPhotobooks {
	return &ListHiddenPhotobooks{pool: pool}
}

// Execute は単純 SELECT。
func (u *ListHiddenPhotobooks) Execute(ctx context.Context, in ListHiddenInput) (ListHiddenOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}
	repo := photobookrdb.NewPhotobookRepository(u.pool)
	items, err := repo.ListHiddenForOps(ctx, limit, offset)
	if err != nil {
		return ListHiddenOutput{}, fmt.Errorf("list hidden: %w", err)
	}
	return ListHiddenOutput{Items: items}, nil
}
