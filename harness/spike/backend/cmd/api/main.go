// Package main は M1 PoC 用 Backend API の起動エントリ。
// Cloud Run + Go chi + pgx の最小構成が成立するかを確認する。
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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"vrcpb/spike-backend/internal/config"
	"vrcpb/spike-backend/internal/db"
	"vrcpb/spike-backend/internal/health"
	"vrcpb/spike-backend/internal/sandbox"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "error", err)
		os.Exit(1)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(rootCtx, cfg.DatabaseURL)
	if err != nil {
		// DSN が解決できない等の起動時 DB エラーは警告のみ。サーバは起動して /readyz で 503 を返す方針。
		logger.Warn("db pool init failed; server still starts but /readyz will fail",
			"error", err.Error())
	}
	if pool != nil {
		defer pool.Close()
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Readyz(pool))
	r.Get("/sandbox/db-ping", sandbox.DBPing(pool))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("server starting", "port", cfg.Port, "env", cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err.Error())
			stop()
		}
	}()

	<-rootCtx.Done()
	logger.Info("shutdown initiated")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err.Error())
		os.Exit(1)
	}
	logger.Info("shutdown complete")
}
