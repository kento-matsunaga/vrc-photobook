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
	"vrcpb/spike-backend/internal/db/sqlcgen"
	"vrcpb/spike-backend/internal/health"
	"vrcpb/spike-backend/internal/httpx"
	r2pkg "vrcpb/spike-backend/internal/r2"
	"vrcpb/spike-backend/internal/sandbox"
	"vrcpb/spike-backend/internal/turnstile"
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

	// R2 クライアントは設定未注入なら nil。ハンドラ側で nil 判定して 503 を返す。
	r2Client, err := r2pkg.NewClient(rootCtx, cfg.R2)
	if err != nil && !errors.Is(err, r2pkg.ErrNotConfigured) {
		// 認証情報は出さず、エラー種別だけ警告として出す
		logger.Warn("r2 client init failed; r2 sandbox endpoints will fail",
			"error", err.Error())
		r2Client = nil
	}
	if errors.Is(err, r2pkg.ErrNotConfigured) {
		logger.Info("r2 not configured; r2 sandbox endpoints will return 503")
		r2Client = nil
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	// CORS: 許可オリジンのみに credentials 付きクロスオリジンリクエストを許可
	r.Use(httpx.CORS(cfg.AllowedOrigins))

	if len(cfg.AllowedOrigins) == 0 {
		logger.Info("ALLOWED_ORIGINS not set; cross-origin requests will not receive CORS headers")
	} else {
		logger.Info("CORS allowed origins configured", "count", len(cfg.AllowedOrigins))
	}

	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Readyz(pool))
	r.Get("/sandbox/db-ping", sandbox.DBPing(pool))

	// R2 接続検証用 sandbox エンドポイント（M1 priority 4）
	r.Get("/sandbox/r2-headbucket", sandbox.R2HeadBucket(r2Client))
	r.Get("/sandbox/r2-list", sandbox.R2List(r2Client))
	r.Post("/sandbox/r2-presign-put", sandbox.R2PresignPut(r2Client))
	r.Get("/sandbox/r2-headobject", sandbox.R2HeadObject(r2Client))

	// Frontend / Backend 結合検証用 sandbox エンドポイント
	r.Get("/sandbox/session-check", sandbox.SessionCheck)
	r.Post("/sandbox/origin-check", sandbox.OriginCheck(cfg.AllowedOrigins))

	// Turnstile + upload_verification_session sandbox（M1 priority 5）
	turnstileClient := turnstile.NewClient(cfg.TurnstileSecretKey)
	if turnstileClient.IsMock() {
		logger.Warn("turnstile secret not configured; running in MOCK mode (PoC only)")
	} else {
		logger.Info("turnstile secret configured; siteverify will be called")
	}
	var queries *sqlcgen.Queries
	if pool != nil {
		queries = sqlcgen.New(pool)
	} else {
		logger.Warn("db pool nil; turnstile/consume sandbox endpoints will return 503")
	}
	r.Post("/sandbox/turnstile/verify",
		sandbox.TurnstileVerify(turnstileClient, queries, cfg.UploadVerification))
	r.Post("/sandbox/upload-intent/consume",
		sandbox.UploadIntentConsume(queries))

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
