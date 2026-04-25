// Package main は M2 本実装 Backend API の起動エントリ。
//
// PR1: 最小骨格として `/health` のみ。
// PR2: config / slog JSON logger / graceful shutdown / `/readyz` を追加。
// PR3 以降で DB 接続 / migration / sqlc / 認証 middleware / 各集約のルートを順次追加する。
//
// 起動:   `go run ./cmd/api`（PORT 環境変数を優先、未設定時 8080）
// 終了:   SIGINT / SIGTERM を受け取り、10 秒以内に graceful shutdown する
//
// PoC との関係:
//   - `harness/spike/backend/cmd/api/main.go` は M1 PoC であり、本実装には流用しない
//   - 本ファイルは `docs/plan/m2-implementation-bootstrap-plan.md` §3 / §4 に基づく新規作成
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vrcpb/backend/internal/config"
	internalhttp "vrcpb/backend/internal/http"
	"vrcpb/backend/internal/shared"
)

const (
	// shutdownTimeout は SIGINT / SIGTERM 受信後の graceful shutdown 上限。
	// Cloud Run の SIGTERM → process kill は 10 秒の grace period が標準。
	shutdownTimeout = 10 * time.Second

	// readHeaderTimeout は Slowloris 攻撃を抑止するためのヘッダ読取上限。
	readHeaderTimeout = 10 * time.Second
)

func main() {
	cfg := config.Load()

	logger := shared.NewLogger(cfg.AppEnv)
	slog.SetDefault(logger)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	router := internalhttp.NewRouter()
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	go func() {
		logger.Info("server starting", slog.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			stop()
		}
	}()

	<-rootCtx.Done()
	logger.Info("shutdown initiated")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("shutdown complete")
}
