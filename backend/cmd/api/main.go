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

	"vrcpb/backend/internal/auth/session/sessionintegration"
	"vrcpb/backend/internal/config"
	"vrcpb/backend/internal/database"
	internalhttp "vrcpb/backend/internal/http"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	imageuploadhttp "vrcpb/backend/internal/imageupload/interface/http"
	imageuploadwireup "vrcpb/backend/internal/imageupload/wireup"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/wireup"
	"vrcpb/backend/internal/shared"
	"vrcpb/backend/internal/uploadverification/infrastructure/turnstile"
	uvhttp "vrcpb/backend/internal/uploadverification/interface/http"
	uvwireup "vrcpb/backend/internal/uploadverification/wireup"
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
	var photobookManageHandlers *photobookhttp.ManageHandlers
	if pool != nil {
		photobookHandlers = wireup.BuildHandlers(pool, manageSessionTTL, photobookhttp.SystemClock{})
		// PR25a: 管理ページ read endpoint（pool だけで完結）
		photobookManageHandlers = wireup.BuildManageReadHandlers(pool)
	}

	// PR21: R2 が configured かつ pool 利用可能なときに imageupload endpoint を組み立てる。
	// PR21 Step A 段階では R2 Secret 未注入（IsR2Configured()=false）のため、handler は
	// nil で渡され endpoint は登録されない。Step D で Secret を注入後に有効化される。
	var imageUploadHandlers *imageuploadhttp.Handlers
	var photobookPublicHandlers *photobookhttp.PublicHandlers
	if pool != nil && cfg.IsR2Configured() {
		r2Client, err := r2.NewAWSClient(r2.AWSConfig{
			AccountID:       cfg.R2AccountID,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			BucketName:      cfg.R2BucketName,
			Endpoint:        cfg.R2Endpoint,
		})
		if err != nil {
			logger.Warn("r2 client init failed; image upload endpoints will be disabled",
				slog.String("error", err.Error()))
		} else {
			imageUploadHandlers = imageuploadwireup.BuildHandlers(pool, r2Client, imageuploadhttp.SystemClock{})
			// PR25a: 公開 Viewer は presigned GET URL を返すため r2Client を必要とする
			photobookPublicHandlers = wireup.BuildPublicHandlers(pool, r2Client)
			logger.Info("r2 configured; image upload endpoints enabled")
		}
	} else if pool != nil {
		logger.Info("r2 not configured; image upload endpoints disabled (PR21 Step A or earlier)")
	}

	// PR22: Turnstile が configured かつ pool 利用可能なときに upload-verifications endpoint を
	// 組み立てる。Turnstile secret は cfg.TurnstileSecretKey で渡す。
	var uvHandlers *uvhttp.Handlers
	if pool != nil && cfg.TurnstileSecretKey != "" {
		verifier := turnstile.NewCloudflareVerifier(turnstile.CloudflareConfig{
			Secret: cfg.TurnstileSecretKey,
		})
		uvHandlers = uvwireup.BuildHandlers(pool, verifier, uvwireup.Config{
			Hostname: cfg.TurnstileHostname,
			Action:   cfg.TurnstileAction,
		}, uvhttp.SystemClock{})
		logger.Info("turnstile configured; upload-verifications endpoint enabled")
	} else if pool != nil {
		logger.Info("turnstile not configured; upload-verifications endpoint disabled")
	}

	routerCfg := internalhttp.RouterConfig{
		Pool:                       pool,
		PhotobookHandlers:          photobookHandlers,
		PhotobookPublicHandlers:    photobookPublicHandlers,
		PhotobookManageHandlers:    photobookManageHandlers,
		ImageUploadHandlers:        imageUploadHandlers,
		UploadVerificationHandlers: uvHandlers,
		AllowedOrigins:             cfg.AllowedOrigins,
	}
	// session validator は draft / manage 共通（session_type は middleware が渡す）。
	if imageUploadHandlers != nil || uvHandlers != nil || photobookManageHandlers != nil {
		validator := sessionintegration.NewSessionValidator(pool)
		routerCfg.DraftSessionValidator = validator
		routerCfg.ManageSessionValidator = validator
	}
	router := internalhttp.NewRouter(routerCfg)
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
