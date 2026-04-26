// Package wireup は Photobook 集約の HTTP handler を組み立てるための facade。
//
// 配置の理由（Go internal ルール）:
//   - cmd/api/main.go は internal/photobook/internal/usecase を直接 import できない
//   - 本パッケージは photobook サブツリー内に居住し、internal/usecase + repository +
//     session_adapter を組み合わせて Handlers を返す
//   - main.go は本パッケージを 1 つ呼ぶだけで token 交換 endpoint の依存ツリーが揃う
//
// 拡張時の指針:
//   - publish / reissue / その他の Photobook UseCase 用 handler が増えたら、本パッケージで
//     一括して組み立てる
package wireup

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/infrastructure/session_adapter"
	photobookhttp "vrcpb/backend/internal/photobook/interface/http"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// BuildHandlers は pool / TTL から Photobook 集約の HTTP Handlers を組み立てる。
//
// pool は本番では *pgxpool.Pool。manageSessionTTL は manage session の TTL（7 日想定）。
// clock は nil で SystemClock が使われる。
func BuildHandlers(
	pool *pgxpool.Pool,
	manageSessionTTL time.Duration,
	clock photobookhttp.Clock,
) *photobookhttp.Handlers {
	repo := photobookrdb.NewPhotobookRepository(pool)
	draftIssuer := session_adapter.NewDraftIssuer(pool)
	manageIssuer := session_adapter.NewManageIssuer(pool)

	draftExchange := usecase.NewExchangeDraftTokenForSession(repo, draftIssuer)
	manageExchange := usecase.NewExchangeManageTokenForSession(repo, manageIssuer)

	return photobookhttp.NewHandlers(draftExchange, manageExchange, manageSessionTTL, clock)
}
