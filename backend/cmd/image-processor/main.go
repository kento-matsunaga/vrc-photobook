// Package main は image-processor CLI のエントリ。
//
// 用途（plan §15.1 実行形態）:
//   - --once       : 1 件だけ処理して終了
//   - --all-pending: 処理対象が無くなるまで（または --max-images まで）処理
//   - --dry-run    : claim 結果を log に出すのみで処理しない
//   - --max-images : 1 回の起動で処理する最大枚数（既定 100）
//   - --timeout    : context timeout（既定 5m）
//
// 起動形態:
//   CLI として image に同梱（`backend/Dockerfile` の同梱バイナリ群、entrypoint を切り替えて
//   起動する想定）。現状は Cloud Run Job 未作成で、ローカル CLI / Cloud SQL Auth Proxy
//   経由で実行する。Cloud Run Job 化は実運用で processing 詰まりが顕在化した時点で再判断。
//
// セキュリティ:
//   - Secret（DATABASE_URL / R2_*）は環境変数経由で受け取り、値そのものはログに出さない
//   - storage_key / R2 credentials / file 内容はログに出さない（plan §10B.2）
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
	processorwireup "vrcpb/backend/internal/imageprocessor/wireup"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/shared"
)

const (
	defaultMaxImages = 100
	defaultTimeout   = 5 * time.Minute
)

func main() {
	var (
		once        = flag.Bool("once", false, "exit after processing a single image")
		allPending  = flag.Bool("all-pending", false, "process until no pending images (capped by --max-images)")
		dryRun      = flag.Bool("dry-run", false, "log claim results without processing")
		maxImages   = flag.Int("max-images", defaultMaxImages, "max images to process in one run")
		timeoutFlag = flag.Duration("timeout", defaultTimeout, "overall context timeout")
	)
	flag.Parse()

	if !*once && !*allPending && !*dryRun {
		fmt.Fprintln(os.Stderr, "must specify one of --once / --all-pending / --dry-run")
		os.Exit(2)
	}

	cfg := config.Load()
	logger := shared.NewLogger(cfg.AppEnv)
	slog.SetDefault(logger)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(rootCtx, *timeoutFlag)
	defer cancel()

	if cfg.DatabaseURL == "" {
		logger.Error("DATABASE_URL not configured")
		os.Exit(1)
	}
	if !cfg.IsR2Configured() {
		logger.Error("R2 secrets not configured")
		os.Exit(1)
	}

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

	runner := processorwireup.NewRunner(pool, r2Client, logger)

	max := *maxImages
	if *once {
		max = 1
	}
	in := processorwireup.RunInput{
		MaxImages: max,
		DryRun:    *dryRun,
	}

	logger.Info("image-processor starting",
		slog.Bool("once", *once),
		slog.Bool("all_pending", *allPending),
		slog.Bool("dry_run", *dryRun),
		slog.Int("max_images", max),
		slog.Duration("timeout", *timeoutFlag),
	)

	out, err := runner.Run(ctx, in)
	logger.Info("image-processor finished",
		slog.Int("picked", out.Picked),
		slog.Int("success", out.Success),
		slog.Int("failed", out.Failed),
	)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			logger.Warn("aborted by context", slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Error("execute failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
