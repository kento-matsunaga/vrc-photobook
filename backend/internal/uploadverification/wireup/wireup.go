// Package wireup は upload verification の HTTP handler を組み立てる。
//
// 配置の理由（Go internal ルール）:
//   - usecase / repository は internal/uploadverification/internal/usecase / .../repository 配下
//   - cmd/api/main.go から直接 import できないため、本パッケージで Handler を組み立てる
package wireup

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/uploadverification/infrastructure/repository/rdb"
	"vrcpb/backend/internal/turnstile"
	uvhttp "vrcpb/backend/internal/uploadverification/interface/http"
	"vrcpb/backend/internal/uploadverification/internal/usecase"
)

// Config は BuildHandlers 引数。
type Config struct {
	Hostname string // Turnstile siteverify の hostname 期待値
	Action   string // Turnstile widget action 期待値
}

// BuildHandlers は pool / Turnstile verifier / config から uploadverification の
// HTTP Handlers を組み立てる。
func BuildHandlers(
	pool *pgxpool.Pool,
	verifier turnstile.Verifier,
	cfg Config,
	clock uvhttp.Clock,
) *uvhttp.Handlers {
	repo := rdb.NewUploadVerificationSessionRepository(pool)
	issue := usecase.NewIssueUploadVerificationSession(verifier, repo)
	return uvhttp.NewHandlers(issue, cfg.Hostname, cfg.Action, clock)
}
