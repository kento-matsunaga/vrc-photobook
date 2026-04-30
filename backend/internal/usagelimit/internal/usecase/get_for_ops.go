// Get / List for cmd/ops。読み取りのみ、redact 表示は呼び出し側で行う。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §10 / §13.1
package usecase

import (
	"context"
	"time"

	"vrcpb/backend/internal/usagelimit/domain/entity"
	"vrcpb/backend/internal/usagelimit/domain/vo/action"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_hash"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_type"
	"vrcpb/backend/internal/usagelimit/domain/vo/window"
	rdb "vrcpb/backend/internal/usagelimit/infrastructure/repository/rdb"
)

// ReadRepo は Get / List UseCase が依存する最小 interface。
// 本番では `*rdb.UsageCounterRepository` がこの interface を満たす。
type ReadRepo interface {
	GetByKey(
		ctx context.Context,
		st scope_type.ScopeType,
		hash scope_hash.ScopeHash,
		act action.Action,
		windowStart time.Time,
	) (entity.UsageCounter, error)

	ListByPrefix(ctx context.Context, f rdb.ListFilters) ([]entity.UsageCounter, error)
}

// =============================================================================
// GetUsageForOps
// =============================================================================

// GetUsageForOpsInput は GetUsageForOps の入力。
type GetUsageForOpsInput struct {
	ScopeType   scope_type.ScopeType
	ScopeHash   scope_hash.ScopeHash
	Action      action.Action
	Now         time.Time
	WindowSeconds int
}

// GetUsageForOpsOutput は単一窓の現状を返す。
type GetUsageForOpsOutput struct {
	Counter entity.UsageCounter
}

// GetUsageForOps は cmd/ops show 用の単一窓取得 UseCase。
type GetUsageForOps struct {
	repo ReadRepo
}

// NewGetUsageForOps は UseCase を組み立てる。
func NewGetUsageForOps(repo ReadRepo) *GetUsageForOps {
	return &GetUsageForOps{repo: repo}
}

// Execute は now が属する固定窓の counter を返す。見つからなければ Repository が
// ErrNotFound を返す（呼び出し側で「(none)」表示）。
func (u *GetUsageForOps) Execute(ctx context.Context, in GetUsageForOpsInput) (GetUsageForOpsOutput, error) {
	w, err := window.New(in.WindowSeconds)
	if err != nil {
		return GetUsageForOpsOutput{}, err
	}
	c, err := u.repo.GetByKey(ctx, in.ScopeType, in.ScopeHash, in.Action, w.StartFor(in.Now))
	if err != nil {
		return GetUsageForOpsOutput{}, err
	}
	return GetUsageForOpsOutput{Counter: c}, nil
}

// =============================================================================
// ListUsageForOps
// =============================================================================

// ListUsageForOpsInput は ListUsageForOps の入力。
type ListUsageForOpsInput struct {
	ScopeType       string // 空文字なら全 scope_type
	ScopeHashPrefix string // 空文字なら全 hash
	Action          string // 空文字なら全 action
	Limit           int32
	Offset          int32
}

// ListUsageForOpsOutput は一覧結果。
type ListUsageForOpsOutput struct {
	Counters []entity.UsageCounter
}

// ListUsageForOps は cmd/ops list 用の一覧取得 UseCase。
type ListUsageForOps struct {
	repo ReadRepo
}

// NewListUsageForOps は UseCase を組み立てる。
func NewListUsageForOps(repo ReadRepo) *ListUsageForOps {
	return &ListUsageForOps{repo: repo}
}

// Execute は filter / offset / limit に従って一覧を返す。
func (u *ListUsageForOps) Execute(ctx context.Context, in ListUsageForOpsInput) (ListUsageForOpsOutput, error) {
	if in.Limit <= 0 || in.Limit > 200 {
		in.Limit = 50
	}
	rows, err := u.repo.ListByPrefix(ctx, rdb.ListFilters{
		ScopeType:       in.ScopeType,
		ScopeHashPrefix: in.ScopeHashPrefix,
		Action:          in.Action,
		Limit:           in.Limit,
		Offset:          in.Offset,
	})
	if err != nil {
		return ListUsageForOpsOutput{}, err
	}
	return ListUsageForOpsOutput{Counters: rows}, nil
}
