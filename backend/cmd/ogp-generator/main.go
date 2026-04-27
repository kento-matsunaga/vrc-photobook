// Package main は ogp-generator CLI のエントリ。
//
// 用途:
//   --photobook-id <uuid>   指定 photobook 1 件の OGP を生成 + R2 PUT
//   --all-pending           pending / stale / failed の photobook を最大 --max-events 件処理
//   --max-events            既定 50
//   --timeout               既定 60s
//   --dry-run               R2 PUT / DB 更新せず、render 結果のみ log
//
// 注意:
//   PR33b では UseCase は **renderer + R2 PUT までで停止**し、images table の
//   usage_kind='ogp' 行作成 / photobook_ogp_images.MarkGenerated は行わない（CHECK 制約
//   image_id NOT NULL を満たせないため、PR33c で images row 作成と組で完了させる）。
//
// セキュリティ:
//   - DATABASE_URL / R2 credentials は env 経由（値はログに出さない）
//   - storage_key 完全値 / token / Cookie / R2 credentials はログに出さない
package main

import (
	"context"
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
	ogpwireup "vrcpb/backend/internal/ogp/wireup"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/shared"
)

const (
	defaultMaxEvents = 50
	defaultTimeout   = 60 * time.Second
)

func main() {
	var (
		photobookID = flag.String("photobook-id", "", "photobook UUID to generate OGP for")
		allPending  = flag.Bool("all-pending", false, "process pending/stale/failed photobooks (capped by --max-events)")
		maxEvents   = flag.Int("max-events", defaultMaxEvents, "max photobooks to process in one run")
		timeoutFlag = flag.Duration("timeout", defaultTimeout, "overall context timeout")
		dryRun      = flag.Bool("dry-run", false, "render but do not PUT to R2 / update DB")
	)
	flag.Parse()

	cfg := config.Load()
	logger := shared.NewLogger(cfg.AppEnv)
	slog.SetDefault(logger)

	if cfg.DatabaseURL == "" {
		logger.Error("DATABASE_URL not configured")
		os.Exit(1)
	}
	if !cfg.IsR2Configured() {
		logger.Error("R2 secrets not configured")
		os.Exit(1)
	}

	if *photobookID == "" && !*allPending {
		fmt.Fprintln(os.Stderr, "must specify --photobook-id <uuid> or --all-pending")
		os.Exit(2)
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

	runner, err := ogpwireup.NewRunner(pool, r2Client, cfg.R2BucketName, logger)
	if err != nil {
		logger.Error("ogp runner init failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("ogp-generator starting",
		slog.String("photobook_id", *photobookID),
		slog.Bool("all_pending", *allPending),
		slog.Bool("dry_run", *dryRun),
		slog.Int("max_events", *maxEvents),
		slog.Duration("timeout", *timeoutFlag),
	)

	out, runErr := runner.Run(ctx, ogpwireup.RunInput{
		PhotobookID: *photobookID,
		AllPending:  *allPending,
		MaxEvents:   *maxEvents,
		DryRun:      *dryRun,
	})

	logger.Info("ogp-generator finished",
		slog.Int("picked", out.Picked),
		slog.Int("rendered", out.Rendered),
		slog.Int("uploaded", out.Uploaded),
		slog.Int("failed", out.Failed),
		slog.Int("skipped", out.Skipped),
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
