// Package main は outbox-worker CLI のエントリ。
//
// 用途:
//   --once             1 件だけ処理して終了
//   --all-pending      pending / failed が無くなるまで処理（max-events / timeout 上限を尊重）
//   --max-events       1 起動で処理する最大件数（既定 50）
//   --timeout          context timeout（既定 60s）
//   --worker-id        locked_by に書く識別子（未指定なら hostname-pid-randomhex）
//   --dry-run          claim 結果を log するだけで status を変えない
//   --release-stale-locks=<duration>
//                      processing で stuck した行を pending に戻して終了。timeout は
//                      stale 判定閾値（locked_at < now - timeout）。本 flag のときは
//                      他の処理は行わない。
//
// 起動形態:
//   現状は CLI として image に同梱し、ローカル CLI / Cloud SQL Auth Proxy 経由で実行する。
//   Cloud Run Jobs / Scheduler 化は未実施（運用判断待ち、新正典ロードマップ参照）。
//
// セキュリティ:
//   - DATABASE_URL 等の Secret は env 経由（値そのものはログに出さない、shared/logging.go
//     の禁止リストで制御）。
//   - payload 全文 / token / Cookie / presigned URL / storage_key 完全値 / R2 credentials
//     はログに出さない（handler 側 + sanitize で制御）。
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vrcpb/backend/internal/config"
	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	ogpwireup "vrcpb/backend/internal/ogp/wireup"
	outboxwireup "vrcpb/backend/internal/outbox/wireup"
	"vrcpb/backend/internal/shared"
)

const (
	defaultMaxEvents = 50
	defaultTimeout   = 60 * time.Second
)

func main() {
	var (
		once             = flag.Bool("once", false, "exit after processing a single event")
		allPending       = flag.Bool("all-pending", false, "process until no pending events (capped by --max-events)")
		dryRun           = flag.Bool("dry-run", false, "log claim results without changing status")
		maxEvents        = flag.Int("max-events", defaultMaxEvents, "max events to process in one run")
		timeoutFlag      = flag.Duration("timeout", defaultTimeout, "overall context timeout")
		workerIDFlag     = flag.String("worker-id", "", "locked_by identifier (auto-generated if empty)")
		releaseStaleFlag = flag.Duration("release-stale-locks", 0, "if > 0, release processing rows whose locked_at < now - <duration> and exit")
	)
	flag.Parse()

	cfg := config.Load()
	logger := shared.NewLogger(cfg.AppEnv)
	slog.SetDefault(logger)

	if cfg.DatabaseURL == "" {
		logger.Error("DATABASE_URL not configured")
		os.Exit(1)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(rootCtx, *timeoutFlag)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("db pool init failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	workerID := *workerIDFlag
	if workerID == "" {
		workerID = generateWorkerID()
	}

	// OGP generator 組み立て（photobook.published handler に渡す）。
	// R2 secrets が無い場合は OgpGenerator=nil で wireup に渡し、handler 登録を skip
	// する（pending event を意図せず processed に進めないための安全側）。
	cfgRunner := outboxwireup.Config{
		WorkerID: workerID,
	}
	if cfg.IsR2Configured() {
		r2Client, err := r2.NewAWSClient(r2.AWSConfig{
			AccountID:       cfg.R2AccountID,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			BucketName:      cfg.R2BucketName,
			Endpoint:        cfg.R2Endpoint,
		})
		if err != nil {
			logger.Error("r2 client init failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		gen, err := ogpwireup.BuildOutboxOgpAdapter(pool, r2Client, cfg.R2BucketName, logger)
		if err != nil {
			logger.Error("ogp adapter init failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		cfgRunner.OgpGenerator = gen
		logger.Info("outbox-worker: ogp generator wired (photobook.published will trigger OGP generation)")
	} else {
		logger.Warn("outbox-worker: R2 not configured; photobook.published handler is not registered (events stay pending)")
	}

	runner := outboxwireup.NewRunner(pool, cfgRunner, logger)

	// release-stale-locks モード: 救出だけ実施して終了。
	if *releaseStaleFlag > 0 {
		released, err := runner.ReleaseStaleLocks(ctx, *releaseStaleFlag)
		if err != nil {
			logger.Error("release stale locks failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Info("release stale locks completed",
			slog.Int64("released", released),
			slog.Duration("threshold", *releaseStaleFlag),
		)
		return
	}

	if !*once && !*allPending && !*dryRun {
		fmt.Fprintln(os.Stderr, "must specify one of --once / --all-pending / --dry-run / --release-stale-locks")
		os.Exit(2)
	}

	max := *maxEvents
	if *once {
		max = 1
	}

	logger.Info("outbox-worker starting",
		slog.String("worker_id", workerID),
		slog.Bool("once", *once),
		slog.Bool("all_pending", *allPending),
		slog.Bool("dry_run", *dryRun),
		slog.Int("max_events", max),
		slog.Duration("timeout", *timeoutFlag),
	)

	out, runErr := runner.Run(ctx, outboxwireup.RunInput{
		MaxEvents: max,
		DryRun:    *dryRun,
	})

	logger.Info("outbox-worker finished",
		slog.String("worker_id", workerID),
		slog.Int("picked", out.Picked),
		slog.Int("processed", out.Processed),
		slog.Int("failed_retry", out.FailedRetry),
		slog.Int("dead", out.Dead),
		slog.Int("skipped_unknown", out.Skipped),
	)

	if runErr != nil {
		if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
			logger.Warn("aborted by context", slog.String("error", runErr.Error()))
			os.Exit(1)
		}
		logger.Error("run failed", slog.String("error", runErr.Error()))
		os.Exit(1)
	}
}

// generateWorkerID は hostname-pid-randomhex 形式の識別子を作る。
//
// hostname 取得失敗時は "unknown" を使う。random hex で同一 host 同時起動の衝突を避ける。
func generateWorkerID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	var rnd [4]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	return fmt.Sprintf("%s-%d-%s", host, os.Getpid(), hex.EncodeToString(rnd[:]))
}
