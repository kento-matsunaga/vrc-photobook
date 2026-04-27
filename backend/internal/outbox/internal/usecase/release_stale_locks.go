// release_stale_locks.go: worker crash 等で processing のまま残った行を救出する。
//
// processing で長時間放置された行（locked_at < threshold）を pending に戻す。
// 現状は CLI flag (`--release-stale-locks=<duration>`) 経由で起動する想定。Cloud Run
// Jobs / Scheduler 化は未実施。
package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	outboxrdb "vrcpb/backend/internal/outbox/infrastructure/repository/rdb"
)

// ReleaseStaleLocks は locked_at < (now - timeout) の processing 行を pending に戻す。
//
// 戻り値は救出された行数。失敗時は error を返し、worker は続行可否を呼び出し側で判断する。
func ReleaseStaleLocks(ctx context.Context, pool *pgxpool.Pool, now time.Time, timeout time.Duration, logger *slog.Logger) (int64, error) {
	if logger == nil {
		logger = slog.Default()
	}
	threshold := now.Add(-timeout)
	repo := outboxrdb.NewOutboxRepository(pool)
	released, err := repo.ReleaseStaleLocks(ctx, threshold, now)
	if err != nil {
		return 0, err
	}
	if released > 0 {
		logger.WarnContext(ctx, "outbox stale locks released",
			slog.Int64("released", released),
			slog.Duration("timeout", timeout),
		)
	}
	return released, nil
}
