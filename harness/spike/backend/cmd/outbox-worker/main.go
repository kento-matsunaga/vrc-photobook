// Package main は Outbox ワーカーの最小 CLI（M1 priority 7 PoC）。
//
// 想定起動経路:
//   - PoC ローカル: `go run ./cmd/outbox-worker --once`
//   - 本実装: Cloud Run Jobs + Cloud Scheduler（cross-cutting/reconcile-scripts.md §3.7.5、U11）
//
// セキュリティ方針:
//   - payload 全文はログに出さない（event_type / attempts / status のみ表示）
//   - last_error 詳細は slog に残すが、CLI 出力では概略のみ
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"vrcpb/spike-backend/internal/config"
	"vrcpb/spike-backend/internal/db"
	"vrcpb/spike-backend/internal/db/sqlcgen"
)

func main() {
	once := flag.Bool("once", false, "1 回だけ pending を最大 limit 件 claim & 処理して終了する")
	limit := flag.Int("limit", 50, "1 回の claim で取得する最大件数（FOR UPDATE SKIP LOCKED）")
	retryFailed := flag.Bool("retry-failed", false,
		"failed イベントを pending に戻す（自動 reconciler outbox_failed_retry 相当）")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "error", err.Error())
		os.Exit(2)
	}

	rootCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(rootCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("db pool init failed", "error", err.Error())
		os.Exit(2)
	}
	if pool == nil {
		logger.Error("db pool is nil; DATABASE_URL not configured")
		os.Exit(2)
	}
	defer pool.Close()

	queries := sqlcgen.New(pool)

	if *retryFailed {
		n, err := queries.RetryFailedOutboxEvents(rootCtx)
		if err != nil {
			logger.Error("retry-failed exec failed", "error", err.Error())
			os.Exit(2)
		}
		logger.Info("outbox failed events requeued", "requeued", n)
		return
	}

	if !*once {
		logger.Error("daemon mode is not implemented in PoC; use --once or --retry-failed")
		os.Exit(2)
	}

	processed, failed, err := processOnce(rootCtx, queries, int32(*limit))
	if err != nil {
		logger.Error("process-once failed", "error", err.Error())
		os.Exit(2)
	}
	logger.Info("outbox process-once done",
		"claimed", processed+failed,
		"processed", processed,
		"failed", failed)
	// 最後に exit 0 を明示
	fmt.Fprintln(os.Stderr, "ok")
}

// processOnce は pending を claim して mock ハンドラに通す。
// PoC のハンドラ規則は sandbox.OutboxProcessOnce と同一に揃える:
//   - event_type が "ForceFail" を含む → MarkOutboxFailed
//   - それ以外 → MarkOutboxProcessed
//
// 本実装では event_type ごとのハンドラルーティングが入る（cross-cutting/outbox.md §6.2）。
func processOnce(ctx context.Context, q *sqlcgen.Queries, limit int32) (processed, failed int, err error) {
	rows, err := q.ClaimPendingOutboxEvents(ctx, limit)
	if err != nil {
		return 0, 0, fmt.Errorf("claim: %w", err)
	}
	for _, ev := range rows {
		eventID := uuid.UUID(ev.ID.Bytes).String()
		if strings.Contains(ev.EventType, "ForceFail") {
			errMsg := "mock_forced_fail"
			if err := q.MarkOutboxFailed(ctx, sqlcgen.MarkOutboxFailedParams{
				ID:        ev.ID,
				LastError: &errMsg,
			}); err != nil {
				slog.Warn("mark failed exec error",
					"event_id", eventID, "error", err.Error())
				continue
			}
			failed++
			continue
		}
		if err := q.MarkOutboxProcessed(ctx, ev.ID); err != nil {
			slog.Warn("mark processed exec error",
				"event_id", eventID, "error", err.Error())
			continue
		}
		processed++
	}
	return processed, failed, nil
}
