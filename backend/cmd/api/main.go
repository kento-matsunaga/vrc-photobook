// Package main は M2 本実装 Backend API の起動エントリ。
//
// PR1: 最小骨格として `/health` のみ。
// PR2: config / slog JSON logger / graceful shutdown / `/readyz`（DB 未実装時 503 固定）。
// PR3: pgx pool 接続を追加。DATABASE_URL 空時は pool nil で起動継続、`/readyz` で 503。
// PR4 以降で middleware / 認証 / 各集約のルートを順次追加する。
//
// 起動:   `go run ./cmd/api`（PORT 環境変数を優先、未設定時 8080）
// 終了:   SIGINT / SIGTERM を受け取り、10 秒以内に graceful shutdown する
//
// PoC との関係:
//   - `harness/spike/backend/cmd/api/main.go` は M1 PoC であり、本実装には流用しない
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
	"vrcpb/backend/internal/database"
	internalhttp "vrcpb/backend/internal/http"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/wireup"
	"vrcpb/backend/internal/shared"
)

// manageSessionTTL は manage session の有効期限。
//
// 業務知識 v4 §6.15 / 計画 m2-photobook-session-integration-plan.md §14.3 で 7 日確定。
// 将来 env 化したくなった場合は config.Config に追加する。
const manageSessionTTL = 7 * 24 * time.Hour

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

	// DB pool 初期化。DATABASE_URL 空のときは pool nil（/readyz で 503 db_not_configured）。
	// DSN の値そのものはログに出さず、設定の有無だけ出す。
	pool, err := database.NewPool(rootCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Warn("db pool init failed; server still starts and /readyz will return 503",
			slog.String("error", err.Error()))
	}
	if pool != nil {
		defer pool.Close()
		logger.Info("db pool configured")
	} else {
		logger.Info("db not configured; /readyz will return 503 db_not_configured")
	}

	// PR9c: pool が利用可能なときだけ Photobook の token 交換 endpoint を組み立てる。
	// pool 未設定（DATABASE_URL 空）時は handler nil で渡して endpoint 自体を作らない。
	var photobookHandlers *photobookhttp.Handlers
	if pool != nil {
		photobookHandlers = wireup.BuildHandlers(pool, manageSessionTTL, photobookhttp.SystemClock{})
	}

	router := internalhttp.NewRouter(pool, photobookHandlers)
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
