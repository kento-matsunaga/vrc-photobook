// http_wireup: cmd/api からの HTTP 配線用 facade。
//
// `internal/ogp/internal/usecase` は ogp 配下からのみ import 可能なため、cmd/api は
// 本 facade を経由して PublicHandlers を取得する。
package wireup

import (
	"github.com/jackc/pgx/v5/pgxpool"

	ogphttp "vrcpb/backend/internal/ogp/interface/http"
	ogpusecase "vrcpb/backend/internal/ogp/internal/usecase"
)

// BuildPublicHandlers は OGP の public lookup endpoint 用 handler を組み立てる。
func BuildPublicHandlers(pool *pgxpool.Pool) *ogphttp.PublicHandlers {
	return ogphttp.NewPublicHandlers(ogpusecase.NewGetPublicOgp(pool))
}
