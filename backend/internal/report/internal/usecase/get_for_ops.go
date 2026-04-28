package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/report/domain/vo/report_id"
	reportrdb "vrcpb/backend/internal/report/infrastructure/repository/rdb"
)

// GetReportForOpsInput は cmd/ops report show の入力。
type GetReportForOpsInput struct {
	ReportID report_id.ReportID
}

// GetReportForOpsOutput は cmd/ops 表示用の View（reporter_contact / detail を含む完全版）。
//
// 出力ホワイトリストは cmd/ops 側で制御する（source_ip_hash 完全値ではなく先頭 4 byte のみ表示）。
type GetReportForOpsOutput struct {
	Report reportrdb.View
}

// GetReportForOps は単一 Report を View で取得する参照系 UseCase。
type GetReportForOps struct {
	pool *pgxpool.Pool
}

// NewGetReportForOps は UseCase を組み立てる。
func NewGetReportForOps(pool *pgxpool.Pool) *GetReportForOps {
	return &GetReportForOps{pool: pool}
}

// ErrReportNotFound は report_id 不在。
var ErrReportNotFound = errors.New("report: not found")

// Execute は報告を読み取って返す。
func (u *GetReportForOps) Execute(ctx context.Context, in GetReportForOpsInput) (GetReportForOpsOutput, error) {
	repo := reportrdb.NewReportRepository(u.pool)
	v, err := repo.GetByID(ctx, in.ReportID)
	if err != nil {
		if errors.Is(err, reportrdb.ErrNotFound) {
			return GetReportForOpsOutput{}, ErrReportNotFound
		}
		return GetReportForOpsOutput{}, fmt.Errorf("get report: %w", err)
	}
	return GetReportForOpsOutput{Report: v}, nil
}
