// Package wireup は upload verification の HTTP handler を組み立てる。
//
// 配置の理由（Go internal ルール）:
//   - usecase / repository は internal/uploadverification/internal/usecase / .../repository 配下
//   - cmd/api/main.go から直接 import できないため、本パッケージで Handler を組み立てる
package wireup

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/uploadverification/infrastructure/repository/rdb"
	"vrcpb/backend/internal/turnstile"
	uvhttp "vrcpb/backend/internal/uploadverification/interface/http"
	"vrcpb/backend/internal/uploadverification/internal/usecase"
	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

// エラー sentinel re-export（PR36）。
var (
	ErrRateLimited            = usecase.ErrRateLimited
	ErrRateLimiterUnavailable = usecase.ErrRateLimiterUnavailable
)

// RateLimited は HTTP layer 用 wrapper の type alias。
type RateLimited = usecase.RateLimited

// AsRateLimited は err が RateLimited wrapper 由来か判定し、Retry-After 秒を取り出す。
func AsRateLimited(err error) (*RateLimited, bool) {
	var rl *RateLimited
	if errors.As(err, &rl) {
		return rl, true
	}
	return nil, false
}

// Config は BuildHandlers 引数。
type Config struct {
	Hostname string // Turnstile siteverify の hostname 期待値
	Action   string // Turnstile widget action 期待値
	// Usage は IssueUploadVerificationSession 内で呼び出す UsageLimit UseCase。
	// nil なら UsageLimit を skip（PR36 commit 3 以前の互換維持用）。本番では非 nil。
	Usage *usagelimitwireup.Check
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
	issue := usecase.NewIssueUploadVerificationSession(verifier, repo, cfg.Usage)
	return uvhttp.NewHandlers(issue, cfg.Hostname, cfg.Action, clock)
}
