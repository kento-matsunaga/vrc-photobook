// Package wireup は UsageLimit UseCase の facade（cmd/ops / Backend HTTP / 既存集約用）。
//
// `internal/usagelimit/internal/usecase` は usagelimit サブツリーからのみ import 可能なため、
// 入出力型 / sentinel エラーを re-export する。
package wireup

import (
	"github.com/jackc/pgx/v5/pgxpool"

	rdb "vrcpb/backend/internal/usagelimit/infrastructure/repository/rdb"
	usecase "vrcpb/backend/internal/usagelimit/internal/usecase"
)

// 入出力型 re-export。
type (
	// Check は CheckAndConsumeUsage の alias。
	Check = usecase.CheckAndConsumeUsage

	// CheckInput / CheckOutput は usecase pkg のものをそのまま再公開。
	CheckInput  = usecase.CheckInput
	CheckOutput = usecase.CheckOutput

	// Get / List for ops も re-export。
	GetForOps        = usecase.GetUsageForOps
	GetForOpsInput   = usecase.GetUsageForOpsInput
	GetForOpsOutput  = usecase.GetUsageForOpsOutput
	ListForOps       = usecase.ListUsageForOps
	ListForOpsInput  = usecase.ListUsageForOpsInput
	ListForOpsOutput = usecase.ListUsageForOpsOutput
)

// エラー sentinel re-export。
var (
	// ErrRateLimited は閾値超過。HTTP 429 にマップ。
	ErrRateLimited = usecase.ErrRateLimited

	// ErrUsageRepositoryFailed は Repository 失敗。fail-closed で HTTP 429 にマップ。
	ErrUsageRepositoryFailed = usecase.ErrUsageRepositoryFailed
)

// NewCheck は pool から CheckAndConsumeUsage UseCase を組み立てる。
func NewCheck(pool *pgxpool.Pool) *Check {
	return usecase.NewCheckAndConsumeUsage(rdb.NewUsageCounterRepository(pool))
}

// NewGetForOps は pool から GetUsageForOps UseCase を組み立てる。
func NewGetForOps(pool *pgxpool.Pool) *GetForOps {
	return usecase.NewGetUsageForOps(rdb.NewUsageCounterRepository(pool))
}

// NewListForOps は pool から ListUsageForOps UseCase を組み立てる。
func NewListForOps(pool *pgxpool.Pool) *ListForOps {
	return usecase.NewListUsageForOps(rdb.NewUsageCounterRepository(pool))
}
