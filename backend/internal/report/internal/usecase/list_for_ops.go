package usecase

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	reportrdb "vrcpb/backend/internal/report/infrastructure/repository/rdb"
)

// ListReportsForOpsInput は cmd/ops report list の入力。
//
// Status / Reason は空文字で「全件」。Limit はクランプ（≤ 200、既定 20）、Offset は >= 0。
type ListReportsForOpsInput struct {
	Status string
	Reason string
	Limit  int32
	Offset int32
}

// ListReportsForOpsOutput は cmd/ops に返す view 一覧。
type ListReportsForOpsOutput struct {
	Reports []reportrdb.View
}

// ListReportsForOps は通報一覧を取得する参照系 UseCase。
type ListReportsForOps struct {
	pool *pgxpool.Pool
}

// NewListReportsForOps は UseCase を組み立てる。
func NewListReportsForOps(pool *pgxpool.Pool) *ListReportsForOps {
	return &ListReportsForOps{pool: pool}
}

// Execute は filter 適用 + minor_safety_concern 優先 sort で list を返す。
func (u *ListReportsForOps) Execute(ctx context.Context, in ListReportsForOpsInput) (ListReportsForOpsOutput, error) {
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
	repo := reportrdb.NewReportRepository(u.pool)
	views, err := repo.List(ctx, reportrdb.ListFilter{
		Status: in.Status,
		Reason: in.Reason,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return ListReportsForOpsOutput{}, fmt.Errorf("list reports: %w", err)
	}
	return ListReportsForOpsOutput{Reports: views}, nil
}
